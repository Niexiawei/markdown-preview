package view

import (
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// viewScheme is the URL scheme the web view loads documents through.
const viewScheme = "mdview"

// RegisterMDView serves the web view's documents: the scheme must answer
// before the window that shows them is made, so main registers it once the
// app runs, with the projects the window has open.
func RegisterMDView(roots func() []string) {
	mygo.Protocol.Handle(viewScheme, newMDViewHandler(roots))
}

// mdViewHandler answers the web view's requests: /doc?p=<path>&rev=<n> for a
// document as HTML, and /img?p=<path> for an image beside it.
type mdViewHandler struct {
	roots func() []string
}

func newMDViewHandler(roots func() []string) *mdViewHandler {
	return &mdViewHandler{roots: roots}
}

func (h *mdViewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("p")
	if path == "" {
		http.Error(w, "missing p", http.StatusBadRequest)
		return
	}
	abs, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil || !h.allowed(abs) {
		http.Error(w, "outside the projects", http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/img":
		h.image(w, r, abs)
	case "/doc":
		h.document(w, r, abs, r.URL.Query().Get("rev"))
	default:
		http.NotFound(w, r)
	}
}

func (h *mdViewHandler) image(w http.ResponseWriter, r *http.Request, abs string) {
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(abs)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeFile(w, r, abs)
}

func (h *mdViewHandler) document(w http.ResponseWriter, r *http.Request, abs, rev string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, "cannot read the document", http.StatusNotFound)
		return
	}
	zoom, _ := strconv.ParseFloat(r.URL.Query().Get("z"), 32)
	html := docHTML(filepath.Dir(abs), string(data), rev, float32(zoom))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

// allowed reports whether abs is inside one of the projects the window has
// open, resolving links so that a path pointing out of a project through one
// does not slip through.
func (h *mdViewHandler) allowed(abs string) bool {
	real := abs
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		real = resolved
	}
	for _, root := range h.roots() {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		resolvedRoot := absRoot
		if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
			resolvedRoot = resolved
		}
		if within(resolvedRoot, real) {
			return true
		}
	}
	return false
}

// within reports whether path is root or below it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

var (
	headingTag = regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
	tagPattern = regexp.MustCompile(`<[^>]*>`)
	imgSource  = regexp.MustCompile(`(?s)(<img\b[^>]*?\bsrc=")([^"]+)(")`)
)

// docHTML renders src as the page the web view shows: the markdown as HTML,
// with an id on every heading so that its outline scrolls there, images
// pointed at the handler, and a style of its own.
func docHTML(baseDir, src, rev string, zoom float32) string {
	body := parser.Parse(src).HTML()
	body = addHeadingIDs(body)
	body = rewriteImages(body, baseDir)

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<meta name=\"rev\" content=\"" + escapeAttr(rev) + "\">")
	b.WriteString("<style>:root { --md-scale: " + formatScale(zoom) + "; }" + docCSS + "</style></head><body><article>")
	b.WriteString(body)
	b.WriteString("</article></body></html>\n")
	return b.String()
}

// addHeadingIDs gives every heading the anchor its slug names, the way the
// native view's outline finds it.
func addHeadingIDs(html string) string {
	return headingTag.ReplaceAllStringFunc(html, func(tag string) string {
		m := headingTag.FindStringSubmatch(tag)
		if m == nil {
			return tag
		}
		text := unescapeText(tagPattern.ReplaceAllString(m[2], ""))
		slug := workspace.Slug(text)
		if slug == "" {
			return tag
		}
		return "<h" + m[1] + ` id="` + escapeAttr(slug) + `">` + m[2] + "</h" + m[1] + ">"
	})
}

// rewriteImages points every relative image at the handler, which serves the
// files of the projects alone.
func rewriteImages(html, baseDir string) string {
	return imgSource.ReplaceAllStringFunc(html, func(tag string) string {
		m := imgSource.FindStringSubmatch(tag)
		if m == nil {
			return tag
		}
		src := m[2]
		if strings.Contains(src, "://") || strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "/") {
			return tag
		}
		abs := filepath.Join(baseDir, filepath.FromSlash(src))
		return m[1] + "/img?p=" + url.QueryEscape(abs) + m[3]
	})
}

// unescapeText turns the entities an inline's HTML holds back into text.
func unescapeText(text string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return replacer.Replace(text)
}

func escapeAttr(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(text)
}

// Roots returns the directories of the projects the window has open, which
// the web view's handler serves and nothing else.
func (s *State) Roots() []string {
	open := s.cfg.OpenProjects()
	roots := make([]string, 0, len(open))
	for _, p := range open {
		roots = append(roots, p.Root)
	}
	return roots
}

// mode is the rendering backend the document shown uses.
func (s *State) mode() config.RenderMode {
	if m, ok := s.renderMode[s.sel]; ok {
		return m
	}
	if s.cfg.Render == config.RenderWebView {
		return config.RenderWebView
	}
	return config.RenderNative
}

// setMode makes the document shown use a rendering backend.
func (s *State) setMode(m config.RenderMode) {
	if s.renderMode == nil {
		s.renderMode = map[docSel]config.RenderMode{}
	}
	s.renderMode[s.sel] = m
}

// modeIndex is the segment the document shown chooses: 0 native, 1 web view.
func (s *State) modeIndex() int {
	if s.mode() == config.RenderWebView {
		return 1
	}
	return 0
}

// webPreview shows the document with the web view, making it the first time
// one is asked for: mygo makes a web view once, and the view only shows it.
func (s *State) webPreview(c *ui.Context) {
	if !s.ensureWebView() {
		s.nativePreview(c)
		return
	}
	s.loadWebDoc()
	ui.WebView(c, s.wv).Grow(1)
}

// ensureWebView makes the window's web view, reporting whether it could: a
// machine without the system's web view keeps the native one.
func (s *State) ensureWebView() bool {
	if s.wv != nil {
		return true
	}
	if s.win == nil {
		return false
	}
	wv, err := s.win.NewWebView(mygo.WebViewOptions{URL: s.docURL()})
	if err != nil {
		s.webOK = false
		s.status = "WebView 不可用：" + err.Error()
		return false
	}
	s.wv = wv
	s.webOK = true
	s.wvURL = s.docURL()
	return true
}

// docURL is the address the web view loads the document shown from. The
// revision changes with the document, so that a changed file reloads.
func (s *State) docURL() string {
	abs := ""
	if p, ok := s.SelectedProject(); ok && !s.sel.empty() {
		if path, err := s.ws.AbsOf(p.ID, s.sel.path); err == nil {
			abs = path
		}
	}
	return viewScheme + "://localhost/doc?p=" + url.QueryEscape(abs) + "&rev=" + itoa(int(s.rev)) +
		"&z=" + formatScale(s.zoomOrOne())
}

// applyWebZoom rescales the page the web view shows without loading it again: a
// load would lose the scroll position, which the app's own zoom buttons must
// not do.
func (s *State) applyWebZoom() {
	if s.wv == nil {
		return
	}
	_, _ = s.wv.Page().Eval("document.documentElement.style.setProperty('--md-scale', '" + formatScale(s.zoomOrOne()) + "')")
}

// loadWebDoc reloads the web view when the document or its source changed.
func (s *State) loadWebDoc() {
	if s.wv == nil {
		return
	}
	// The address names the document, not where in it the body shows: the
	// outline scrolls the page in place (scrollWebTo), since a hash would
	// load the page again and lose where it was.
	url := s.docURL()
	if url == s.wvURL {
		return
	}
	s.wvURL = url
	_ = s.wv.Page().LoadURL(url)
}

// scrollWebTo glides the page to the heading of slug, as the native body glides
// to the block: the page's own scrollIntoView lines the heading up with its top
// and stops at the page's end.
func (s *State) scrollWebTo(slug string) {
	if s.wv == nil {
		return
	}
	if _, err := s.wv.Page().Eval(headingScrollJS(slug)); err != nil {
		s.status = "无法在页面中滚动：" + err.Error()
	}
}

// headingScrollJS is the page's scroll to the heading of slug: the browser's
// smooth scroll, which lines the heading up with the page's top and stops at
// its end, as the native body stops at its own.
func headingScrollJS(slug string) string {
	return "(function(){var e=document.getElementById(" + strconv.Quote(slug) +
		");if(e)e.scrollIntoView({behavior:'smooth',block:'start'});})()"
}

// itoa is strconv.Itoa, kept small for the one place the view needs it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

// formatScale writes a zoom as the page's CSS wants it: two decimals, one for
// what was never changed.
func formatScale(zoom float32) string {
	if zoom <= 0 {
		zoom = zoomDefault
	}
	return strconv.FormatFloat(float64(zoom), 'f', 2, 64)
}

// docCSS is the page's look: a readable measure, the system's fonts, and the
// dark appearance the desktop asks for.
const docCSS = `
:root { color-scheme: light dark; }
* { box-sizing: border-box; }
body {
  margin: 0;
  padding: 28px 32px 64px;
  font: 15px/1.65 -apple-system, "Segoe UI", "Microsoft YaHei", system-ui, sans-serif;
  font-size: calc(15px * var(--md-scale));
  color: #1f2328;
  background: #ffffff;
  -webkit-font-smoothing: antialiased;
}
article { max-width: 820px; }
h1, h2, h3, h4, h5, h6 {
  line-height: 1.3; margin: 1.4em 0 0.55em; font-weight: 600;
  /* The room a jump leaves above the heading, as the native body's top border
     leaves it (native.go's bodyJumpGap): scrollIntoView counts it. */
  scroll-margin-top: calc(16px * var(--md-scale));
}
h1 { font-size: 1.9em; margin-top: 0.2em; }
h2 { font-size: 1.45em; border-bottom: 1px solid rgba(0,0,0,0.08); padding-bottom: 0.25em; }
h3 { font-size: 1.2em; }
p, ul, ol, blockquote, table, pre { margin: 0.75em 0; }
a { color: #0a6cff; text-decoration: none; }
a:hover { text-decoration: underline; }
code { font-family: ui-monospace, "Cascadia Code", Consolas, monospace; font-size: 0.9em;
  background: rgba(0,0,0,0.06); border-radius: 4px; padding: 0.15em 0.35em; }
pre { background: rgba(0,0,0,0.05); border-radius: 8px; padding: 12px 14px; overflow: auto; }
pre code { background: none; padding: 0; font-size: 0.88em; }
blockquote { margin-left: 0; padding: 0.1em 1em; border-left: 3px solid rgba(0,0,0,0.15); color: #57606a; }
table { border-collapse: collapse; }
th, td { border: 1px solid rgba(0,0,0,0.14); padding: 6px 10px; text-align: left; }
th { background: rgba(0,0,0,0.04); }
img { max-width: 100%; height: auto; }
hr { border: none; border-top: 1px solid rgba(0,0,0,0.12); margin: 1.6em 0; }
@media (prefers-color-scheme: dark) {
  body { color: #e6edf3; background: #161b22; }
  h2 { border-bottom-color: rgba(255,255,255,0.1); }
  a { color: #6cb0ff; }
  code, pre { background: rgba(255,255,255,0.07); }
  blockquote { border-left-color: rgba(255,255,255,0.18); color: #9aa4ae; }
  th, td { border-color: rgba(255,255,255,0.16); }
  th { background: rgba(255,255,255,0.05); }
  hr { border-top-color: rgba(255,255,255,0.14); }
}
`
