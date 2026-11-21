package view

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mdpreview/internal/config"
)

func TestDocHTMLEscapesAndAnchors(t *testing.T) {
	html := docHTML(t.TempDir(), "# A\n\n`<b>` and <span>x</span>\n", "1")
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
	html := docHTML(t.TempDir(), "# 一\n\n## 二 Install 3\n", "1")
	if !strings.Contains(html, `<h2 id="二-install-3">`) {
		t.Errorf("html = %s, want the CJK heading's id", html)
	}
}

func TestDocHTMLTables(t *testing.T) {
	html := docHTML(t.TempDir(), "| a | b |\n| --- | --- |\n| 1 | 2 |\n", "1")
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td>1</td>") {
		t.Errorf("html = %s, want a table", html)
	}
}

func TestDocHTMLRewritesImages(t *testing.T) {
	dir := t.TempDir()
	html := docHTML(dir, "![pic](pic.png)\n", "1")
	want := "/img?p=" + url.QueryEscape(filepath.Join(dir, "pic.png"))
	if !strings.Contains(html, want) {
		t.Errorf("html = %s, want %s", html, want)
	}
}

func TestDocHTMLKeepsRemoteImages(t *testing.T) {
	html := docHTML(t.TempDir(), "![pic](https://example.com/p.png)\n", "1")
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
