package view

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mdpreview/internal/config"
)

func TestDocHTMLEscapesAndAnchors(t *testing.T) {
	html := docHTML(t.TempDir(), "# A\n\n`<b>` and <span>x</span>\n", "1", 1)
	if !strings.Contains(html, `<h1 id="a">`) {
		t.Errorf("html = %s, want a heading with an id", html)
	}
	if !strings.Contains(html, "<span>x</span>") {
		t.Errorf("html = %s, want raw HTML kept", html)
	}
	if !strings.Contains(html, "<code>&lt;b&gt;</code>") {
		t.Errorf("html = %s, want the code span escaped", html)
	}
}

func TestDocHTMLAnchorsFollowLevels(t *testing.T) {
	html := docHTML(t.TempDir(), "# 一\n\n## 二 Install 3\n", "1", 1)
	if !strings.Contains(html, `<h2 id="二-install-3">`) {
		t.Errorf("html = %s, want the CJK heading's id", html)
	}
}

func TestDocHTMLTables(t *testing.T) {
	html := docHTML(t.TempDir(), "| a | b |\n| --- | --- |\n| 1 | 2 |\n", "1", 1)
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td>1</td>") {
		t.Errorf("html = %s, want a table", html)
	}
}

func TestDocHTMLRewritesImages(t *testing.T) {
	dir := t.TempDir()
	html := docHTML(dir, "![pic](pic.png)\n", "1", 1)
	want := "/img?p=" + url.QueryEscape(filepath.Join(dir, "pic.png"))
	if !strings.Contains(html, want) {
		t.Errorf("html = %s, want %s", html, want)
	}
}

func TestDocHTMLKeepsRemoteImages(t *testing.T) {
	html := docHTML(t.TempDir(), "![pic](https://example.com/p.png)\n", "1", 1)
	if !strings.Contains(html, `src="https://example.com/p.png"`) {
		t.Errorf("html = %s, want the remote image kept", html)
	}
}

func TestMDViewForbidsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	h := newMDViewHandler(func() []string { return []string{root} })
	outside := filepath.Join(t.TempDir(), "x.md")
	os.WriteFile(outside, []byte("# x\n"), 0o644)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/doc?p="+url.QueryEscape(outside)+"&rev=1", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestMDViewServesImage(t *testing.T) {
	root := t.TempDir()
	png := filepath.Join(root, "p.png")
	os.WriteFile(png, []byte("\x89PNG\r\n\x1a\n"), 0o644)
	h := newMDViewHandler(func() []string { return []string{root} })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/img?p="+url.QueryEscape(png), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/png") {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestMDViewServesDocument(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "a.md")
	os.WriteFile(doc, []byte("# Hello\n"), 0o644)
	h := newMDViewHandler(func() []string { return []string{root} })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/doc?p="+url.QueryEscape(doc)+"&rev=3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<h1 id="hello">Hello</h1>`) {
		t.Errorf("body = %s, want the rendered heading", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestSegmentedSwitchRemembersPerDoc(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	if got := s.mode(); got != config.RenderNative {
		t.Fatalf("mode = %q, want native", got)
	}
	s.setMode(config.RenderWebView)
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
	s.selectDoc("b.md")
	if got := s.mode(); got != config.RenderNative {
		t.Errorf("mode of b.md = %q, want native", got)
	}
	s.selectDoc("a.md")
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode of a.md = %q, want webview", got)
	}
}

func TestSegmentedShowsBothModes(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	if !tt.HasText("原生") || !tt.HasText("WebView") {
		t.Errorf("texts = %q, want both segments", tt.Texts())
	}
}

func TestMDViewServesOddNames(t *testing.T) {
	root := t.TempDir()
	name := "方案 #1 50%+问号？.md"
	doc := filepath.Join(root, name)
	if err := os.WriteFile(doc, []byte("# 方案\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newMDViewHandler(func() []string { return []string{root} })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/doc?p="+url.QueryEscape(doc)+"&rev=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 for %q", rec.Code, name)
	}
	if !strings.Contains(rec.Body.String(), "方案") {
		t.Errorf("body = %s, want the document", rec.Body.String())
	}
}

func TestDocURLRoundTripsThroughHandler(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	u := s.docURL()
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("docURL = %q: %v", u, err)
	}
	if parsed.Scheme != viewScheme {
		t.Errorf("scheme = %q, want %q", parsed.Scheme, viewScheme)
	}
	abs := parsed.Query().Get("p")
	want, _ := filepath.Abs(filepath.Join(root, "a.md"))
	if abs != want {
		t.Errorf("p = %q, want %q", abs, want)
	}
}

func TestRootsAreTheOpenProjects(t *testing.T) {
	s, alphaRoot, _ := twoProjects(t)
	s.ToggleProject(s.Projects()[1].ID)
	roots := s.Roots()
	if len(roots) != 1 || roots[0] != alphaRoot {
		t.Errorf("Roots = %v, want only %q: the web view serves open projects", roots, alphaRoot)
	}
}

// TestDocHTMLCarriesTheZoom pins that the page is built at the zoom the app
// shows the body with, so that the first load needs no second pass.
func TestDocHTMLCarriesTheZoom(t *testing.T) {
	html := docHTML(t.TempDir(), "# A\n", "1", 1.25)
	if !strings.Contains(html, "--md-scale: 1.25") {
		t.Errorf("page = %q, want the zoom in it", html)
	}
	if !strings.Contains(html, "calc(15px * var(--md-scale))") {
		t.Errorf("page = %q, want the body to scale with it", html)
	}
}

// TestHeadingScrollJS pins the page's scroll to a heading: the browser's own
// smooth scroll that lines the heading up with the page's top, and stops at the
// page's end, as the native body does with a block.
func TestHeadingScrollJS(t *testing.T) {
	js := headingScrollJS("二-install-3")
	for _, want := range []string{
		`document.getElementById("二-install-3")`,
		"scrollIntoView",
		"behavior:'smooth'",
		"block:'start'",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("js = %s, want %q", js, want)
		}
	}
}

// TestDocURLHasNoHeading pins that the heading an outline click names never
// reaches the page's address: a hash there reloads the page, which both loses
// where the body was and skips the glide.
func TestDocURLHasNoHeading(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.anchor = "二"
	if got := s.docURL(); strings.Contains(got, "二") || strings.Contains(got, "#") {
		t.Errorf("address = %q, want no heading in it", got)
	}
}

// TestDocHTMLLeavesRoomAboveHeadings pins the page's half of the room a jump
// leaves: scrollIntoView counts scroll-margin, as the native body leaves it with
// its top border.
func TestDocHTMLLeavesRoomAboveHeadings(t *testing.T) {
	html := docHTML(t.TempDir(), "# A\n", "1", 1)
	if !strings.Contains(html, "scroll-margin-top") {
		t.Errorf("page = %q, want room above the headings", html)
	}
}

// TestThePageTellsTheOutlineWhereTheReaderIs pins the WebView path: the page
// reports the heading at its top, and that becomes the outline's current one.
func TestThePageTellsTheOutlineWhereTheReaderIs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)

	s.followTo("二", 0)
	if s.anchor != "二" {
		t.Errorf("anchor = %q, want the heading the page reported", s.anchor)
	}
}

// TestThePageCannotMoveTheNativeOutline pins the guard: the page of a web view
// nobody shows must not repaint the outline of the native body.
func TestThePageCannotMoveTheNativeOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n")
	s.Refresh()
	s.selectDoc("a.md") // 原生

	s.followTo("二", 0)
	if s.anchor != "" {
		t.Errorf("anchor = %q, want the native body's own state", s.anchor)
	}
}

// TestAPageReportDoesNotCutAJumpShort pins the hold for the page too.
func TestAPageReportDoesNotCutAJumpShort(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	s.frameNow = time.Now()
	s.followUntil = s.frameNow.Add(followHold)

	s.followTo("二", 0)
	if s.anchor != "" {
		t.Errorf("anchor = %q during the hold, want it left alone", s.anchor)
	}
}

// TestThePageCannotTakeTheOutlineOffAClammedJump pins the same for the page:
// while the page still sits where a jump left it, the heading it reports from
// the top edge does not replace the one the jump chose.
func TestThePageCannotTakeTheOutlineOffAClammedJump(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	tt := newTester(s)
	s.gotoHeading("二") // what a click on the outline does
	tt.Frame()
	tt.Advance(time.Second) // past the hold: the jump has landed
	tt.Frame()

	// The page cannot lift 二 to its top (the document ends first), so it
	// reports 一; it has not moved since the jump though.
	s.followTo("一", 120)
	if s.anchor != "二" {
		t.Fatalf("anchor = %q, want 二: the page did not move", s.anchor)
	}
	// The reader scrolls: the report is theirs now.
	s.followTo("一", 40)
	if s.anchor != "一" {
		t.Errorf("anchor = %q after the page moved, want 一", s.anchor)
	}
}
