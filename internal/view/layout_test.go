package view

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/shells/sidebar"

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
		if sx := s.docScroll().MaxX; sx != 0 {
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
	if err := tt.Click("WebView"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
	if err := tt.Click("原生"); err != nil {
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

// TestModeToggleIsOneSegmentedControl pins the switch as the theme's segmented
// control: the two backends are segments of one track, not two loose buttons
// that name themselves.
func TestModeToggleIsOneSegmentedControl(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 900, 48)
	if tt.HasText("渲染方式：原生") {
		t.Errorf("texts = %q, want no per-segment label: the segmented control names itself", tt.Texts())
	}
	if err := tt.Click("WebView"); err != nil {
		t.Fatalf("Click WebView: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
	if err := tt.Click("原生"); err != nil {
		t.Fatalf("Click 原生: %v", err)
	}
	if got := s.mode(); got != config.RenderNative {
		t.Errorf("mode = %q, want native", got)
	}
}

// TestTitleFitsNarrowWindow pins that a long path gives way instead of pushing
// the mode switch out of the window: at the narrowest useful title bar both
// segments show at their whole width, and still work.
func TestTitleFitsNarrowWindow(t *testing.T) {
	s, root := newState(t)
	doc := "docs/2026-11-21-a-really-long-document-name-for-the-title-bar.md"
	writeDoc(t, root, doc, "# A\n")
	s.Refresh()
	s.selectDoc(doc)

	build := func(c *ui.Context) { s.ContentTitle(c) }
	wide := ui.NewTester(build, 900, 48)
	natural, ok := wide.Find("WebView")
	if !ok {
		t.Fatalf("texts = %q, want the mode switch", wide.Texts())
	}

	narrow := ui.NewTester(build, 280, 48)
	cut, ok := narrow.Find("WebView")
	if !ok {
		t.Fatalf("texts = %q, want the mode switch", narrow.Texts())
	}
	if cut.W < natural.W {
		t.Errorf("the WebView segment shows %v of its %v: the title bar cut it", cut.W, natural.W)
	}
	if err := narrow.Click("WebView"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.mode(); got != config.RenderWebView {
		t.Errorf("mode = %q, want webview", got)
	}
}

// TestTitleCopiesTheDocumentPath pins the button after the document's name:
// one click puts the document's absolute path in the clipboard and says so.
func TestTitleCopiesTheDocumentPath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := ui.NewTester(func(c *ui.Context) { s.ContentTitle(c) }, 900, 48)
	if err := tt.Click("复制文档路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes", "a.md"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if !strings.Contains(s.Status(), "已复制文件路径") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestTOCScrollSurvivesAReloadButNotADocumentChange pins that the outline
// keeps its place while the same document reloads (an agent saved it, or the
// backend switched), and starts at the top for another document.
func TestTOCScrollSurvivesAReloadButNotADocumentChange(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## 一\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")

	s.tocScroll.Y = 120
	s.onDocChanged()
	if s.tocScroll.Y != 120 {
		t.Errorf("Y = %v after the same document reloaded, want 120", s.tocScroll.Y)
	}

	s.selectDoc("b.md")
	if s.tocScroll.X != 0 || s.tocScroll.Y != 0 {
		t.Errorf("scroll = (%v, %v) for another document, want (0, 0)", s.tocScroll.X, s.tocScroll.Y)
	}
}

// TestTitleSitsInsideTheHeaderSlot pins the title bar to the room the shell
// gives it: the shell pads its header by 6 DIP top and bottom, clips what
// overflows, and the mode switch must stay inside that slot rather than being
// cut by it.
func TestTitleSitsInsideTheHeaderSlot(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")

	const pad float32 = 6
	var header, slotTop, slotBottom float32
	tt := ui.NewTester(func(c *ui.Context) {
		header = sidebar.HeaderHeight(c)
		slotTop, slotBottom = pad, header-pad
		ui.Row(c).Height(header).Padding(pad, 8).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).FillHeight().Clip().Children(func() {
				s.ContentTitle(c)
			})
		})
	}, 900, 48)

	for _, label := range []string{"原生", "WebView"} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("texts = %q, want %q", tt.Texts(), label)
		}
		if rect.Y < slotTop+6 || rect.Y+rect.H > slotBottom-6 {
			t.Errorf("%q shows at %v, outside the header's slot %v..%v",
				label, rect, slotTop+6, slotBottom-6)
		}
	}
}

// TestExpandAllOnlyTouchesTheCurrentDocument pins the outline's two bulk
// actions: expanding every heading of the document shown unfolds its outline,
// and not another document's headings of the same name.
func TestExpandAllOnlyTouchesTheCurrentDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 总览\n\n## 范围\n\n### 细节\n")
	writeDoc(t, root, "b.md", "# 总览\n\n## 范围\n\n### 细节\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md") // b.md shows; a.md keeps its own open state

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("折叠全部目录"); err != nil {
		t.Fatalf("Click collapse: %v", err)
	}
	tt.Frame()
	if tt.HasText("范围") {
		t.Errorf("outline texts = %q, want only the top level after folding all", tt.Texts())
	}
	if err := tt.Click("展开全部目录"); err != nil {
		t.Fatalf("Click expand: %v", err)
	}
	tt.Frame()
	for _, want := range []string{"范围", "细节"} {
		if !tt.HasText(want) {
			t.Errorf("outline texts = %q, want %q after expanding all", tt.Texts(), want)
		}
	}

	s.selectDoc("a.md")
	tt.Frame()
	if !tt.HasText("范围") {
		t.Errorf("outline texts = %q, want a.md untouched by b.md's bulk action", tt.Texts())
	}
}

// TestCollapseAllFoldsTheOutline pins the other direction: folding every
// heading leaves only the top level, and a click still jumps.
func TestCollapseAllFoldsTheOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n### 三\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("折叠全部目录"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	if tt.HasText("二") || tt.HasText("三") {
		t.Errorf("outline texts = %q, want only the top level", tt.Texts())
	}
	if err := tt.Click("一"); err != nil {
		t.Fatalf("Click heading: %v", err)
	}
	if s.anchor != "一" {
		t.Errorf("anchor = %q, want the top-level heading", s.anchor)
	}
}

// TestTOCBulkActionsAreDisabledWithoutHeadings pins that the two bulk actions
// do nothing on a document without headings, rather than erroring.
func TestTOCBulkActionsAreDisabledWithoutHeadings(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "plain.md", "没有标题的正文\n")
	s.Refresh()
	s.selectDoc("plain.md")

	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	for _, label := range []string{"展开全部目录", "折叠全部目录"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("outline texts = %q, want %q", tt.Texts(), label)
		}
	}
	s.setTOC(true)
	if len(s.open) != 0 {
		t.Errorf("open = %v, want nothing written without headings", s.open)
	}
}
