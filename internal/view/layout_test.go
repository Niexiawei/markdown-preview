package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
)

// TestBodyColumnFollowsPaneWidth pins that the body wraps inside the pane
// instead of overflowing it: a long paragraph scrolls neither way at either
// width, and the pane's own width follows what the app measured.
func TestBodyColumnFollowsPaneWidth(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# T\n\n这是一段很长很长的文字，用来确认它在不同宽度下都会在窗格内换行，"+
		"而不是溢出到可视区域外面去，这样用户就不需要横向滚动才能读完整段话。\n")
	s.Refresh()
	s.selectDoc("a.md")

	for _, width := range []float32{600, 1200} {
		width := width
		s.SetContentWidth(func() float32 { return width })
		_ = ui.NewTester(func(c *ui.Context) { s.Content(c) }, int(width), 700)
		if sx := s.docScroll.MaxX; sx != 0 {
			t.Errorf("width %v: MaxX = %v, want 0: the text wrapped instead of overflowing", width, sx)
		}
		want := width - s.tocW
		if got := s.bodyPaneWidth(); got != want {
			t.Errorf("width %v: bodyPaneWidth = %v, want %v", width, got, want)
		}
	}
}

// TestModeToggleSitsWithTheDocumentName pins the rendering switch in the
// title bar, after the document's name, and that a click still changes the
// document's backend.
func TestModeToggleSitsWithTheDocumentName(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 900, 40)
	for _, want := range []string{"alpha/notes/a.md", "原生", "WebView"} {
		if !tt.HasText(want) {
			t.Errorf("title texts = %q, want %q", tt.Texts(), want)
		}
	}
	if err := tt.Click("渲染方式：WebView"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
	if err := tt.Click("渲染方式：原生"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.mode(); got != config.RenderNative {
		t.Errorf("mode = %q, want native", got)
	}
}

// TestStatusBarDropsTheModeSwitch pins that the rendering switch left the
// bottom bar, so the body keeps that room.
func TestStatusBarDropsTheModeSwitch(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := ui.NewTester(func(c *ui.Context) { s.statusBar(c) }, 600, 24)
	for _, unwanted := range []string{"原生", "WebView"} {
		if tt.HasText(unwanted) {
			t.Errorf("status bar texts = %q, want no %q", tt.Texts(), unwanted)
		}
	}
}

// TestTOCScrollsBothWays pins that the outline scrolls both ways: a document
// with many headings and one long heading is cut neither at the bottom nor at
// the right edge.
func TestTOCScrollsBothWays(t *testing.T) {
	s, root := newState(t)
	var b strings.Builder
	b.WriteString("# " + strings.Repeat("很长", 60) + "\n\n")
	for i := range 60 {
		fmt.Fprintf(&b, "## 小节 %02d\n\n正文\n", i)
	}
	writeDoc(t, root, "a.md", b.String())
	s.Refresh()
	s.selectDoc("a.md")

	_ = ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 300)
	if s.tocScroll.MaxY <= 0 {
		t.Errorf("MaxY = %v, want the outline to scroll down", s.tocScroll.MaxY)
	}
	if s.tocScroll.MaxX <= 0 {
		t.Errorf("MaxX = %v, want the long heading to scroll sideways", s.tocScroll.MaxX)
	}
}

// TestTOCRowsFillThePane pins that a short outline's rows still take the whole
// pane: a click at the pane's right edge chooses the heading there.
func TestTOCRowsFillThePane(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 短标题\n\n## 范围\n\n## 执行顺序\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 300)
	rect, ok := tt.Find("范围")
	if !ok {
		t.Fatalf("texts = %q, want the heading", tt.Texts())
	}
	tt.ClickAt(220, rect.Y+rect.H/2)
	if s.anchor != "范围" {
		t.Errorf("anchor = %q, want the heading clicked at the pane's edge", s.anchor)
	}
}
