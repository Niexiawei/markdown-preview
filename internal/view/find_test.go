package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
)

// openFindOn opens the search panel on the document body and searches for
// query, as a user typing does.
func openFindOn(s *State, query string) {
	s.find.open = true
	s.find.query = query
	s.refreshMatches()
}

// TestFindPanelOpensWithCmdF pins the shortcut: it opens the floating panel
// over the body, and the panel carries what the requirements name — the
// matches' lines, and a preview of the chosen one's context.
func TestFindPanelOpensWithCmdF(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n导入期间禁用重复导入。\n\n导入完成后刷新。\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Fatal("the panel did not open")
	}
	s.find.query = "导入"
	s.refreshMatches()
	tt.Frame()

	if !tt.HasText("1/3") {
		t.Errorf("panel texts = %q, want the count 1/3", tt.Texts())
	}
	if !tt.HasText("导入期间禁用重复导入。") {
		t.Errorf("panel texts = %q, want the matched line listed", tt.Texts())
	}
	// The preview block names the lines around the match, from the line
	// numbers the matches sit on.
	if !tt.HasText("一") || !tt.HasText("导入完成后刷新。") {
		t.Errorf("panel texts = %q, want the context around the match", tt.Texts())
	}
}

// TestFindPanelStepsThroughMatches pins next/previous: they wrap, and each of
// them shows the match it lands on.
func TestFindPanelStepsThroughMatches(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "一 目标\n二 目标\n三 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	if len(s.find.matches) != 3 {
		t.Fatalf("matches = %v, want three", s.find.matches)
	}
	s.findNext(1)
	if s.find.current != 1 || s.find.matches[s.find.current].Line != 2 {
		t.Errorf("current = %d, want the second match", s.find.current)
	}
	s.findNext(1)
	s.findNext(1)
	if s.find.current != 0 {
		t.Errorf("current = %d, want 0: it wrapped", s.find.current)
	}
	s.findNext(-1)
	if s.find.current != 2 {
		t.Errorf("current = %d, want 2: it wrapped the other way", s.find.current)
	}
}

// TestFindJumpScrollsTheNativeBody pins the native jump: the block holding the
// match becomes the markdown view's anchor, which is what scrolls it, and the
// same match asked for twice scrolls again.
func TestFindJumpScrollsTheNativeBody(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n甲段\n\n乙段 目标\n\n丙段\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	s.gotoMatch(0)

	if !strings.HasPrefix(s.anchor, findAnchorPrefix) {
		t.Fatalf("anchor = %q, want the find prefix", s.anchor)
	}
	block := s.anchorBlock(s.doc, s.anchor)
	if block <= 0 {
		t.Fatalf("anchorBlock(%q) = %d, want the paragraph's block", s.anchor, block)
	}
	if got := s.doc.Block(block); got.Kind != parser.Paragraph {
		t.Errorf("block kind = %v, want the paragraph holding the match", got.Kind)
	}

	before := s.anchorVer
	s.gotoMatch(0)
	if s.anchorVer <= before {
		t.Errorf("anchorVer = %d, want more than %d", s.anchorVer, before)
	}
}

// TestAnchorBlockLeavesHeadingsAlone pins the fallback: a name that is not a
// find anchor is not a block, so the view finds the heading's slug itself.
func TestAnchorBlockLeavesHeadingsAlone(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n")
	s.Refresh()
	s.selectDoc("a.md")
	if got := s.anchorBlock(s.doc, "一"); got != -1 {
		t.Errorf("anchorBlock = %d, want -1 for a heading's slug", got)
	}
	if got := s.anchorBlock(s.doc, findAnchorPrefix+"99"); got != -1 {
		t.Errorf("anchorBlock = %d, want -1 for a block out of range", got)
	}
	if got := s.anchorBlock(s.doc, findAnchorPrefix+"x"); got != -1 {
		t.Errorf("anchorBlock = %d, want -1 for an anchor that is not a number", got)
	}
}

// TestFindMatchesFollowTheDocument pins that the panel searches what the window
// shows: another document means other matches, and a changed one too.
func TestFindMatchesFollowTheDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "甲 目标\n")
	writeDoc(t, root, "b.md", "乙 目标\n丙 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	if len(s.find.matches) != 1 {
		t.Fatalf("matches = %v, want one in a.md", s.find.matches)
	}
	s.selectDoc("b.md")
	if len(s.find.matches) != 2 {
		t.Errorf("matches = %v, want two in b.md", s.find.matches)
	}
	if s.find.current != 0 {
		t.Errorf("current = %d, want 0 on another document", s.find.current)
	}
}

// TestEscapeClearsThenCloses pins the panel's Escape: it empties the query
// first, and only an empty field closes the panel.
func TestEscapeClearsThenCloses(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "正文")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.query != "" {
		t.Errorf("query = %q, want it cleared", s.find.query)
	}
	if !s.find.open {
		t.Error("the panel closed while it only had to clear the query")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.open {
		t.Error("the panel stayed open on the second Escape")
	}
}

// TestFindPanelWithoutADocumentDoesNothing pins the guard: Cmd+F in the empty
// state opens nothing, so no panel floats over the empty state's text.
func TestFindPanelWithoutADocumentDoesNothing(t *testing.T) {
	s := newStateWith(config.Config{Version: config.Version})
	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if s.find.open {
		t.Error("the panel opened without a document")
	}
}

// TestFindPanelClosesOnTheCloseButton pins the button beside the count.
func TestFindPanelClosesOnTheCloseButton(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "正文")

	tt := ui.NewTester(func(c *ui.Context) { s.findPanel(c) }, 900, 700)
	if err := tt.Click("关闭搜索"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.find.open {
		t.Error("the panel stayed open")
	}
}

// TestFindListRowJumpsToItsMatch pins the list: a click on a matched line shows
// that match in the body.
func TestFindListRowJumpsToItsMatch(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "甲\n乙 目标\n丙 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")

	tt := ui.NewTester(func(c *ui.Context) { s.findPanel(c) }, 900, 700)
	if err := tt.Click("丙 目标"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.find.current != 1 {
		t.Errorf("current = %d, want the second match", s.find.current)
	}
}

// TestFindJumpWithoutAWebViewOnlyMovesTheList pins the degraded path: with no
// web view made (headless tests, a machine without one) jumping in webview mode
// still moves the list, and only says so in the status bar.
func TestFindJumpWithoutAWebViewOnlyMovesTheList(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "一 目标\n二 目标\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	openFindOn(s, "目标")

	s.gotoMatch(1)
	if s.find.current != 1 {
		t.Errorf("current = %d, want 1", s.find.current)
	}
	if s.wv != nil {
		t.Fatal("a web view was made without a window")
	}
	if !strings.Contains(s.status, "WebView") {
		t.Errorf("status = %q, want the web view's absence reported", s.status)
	}
}

// TestClosingTheFindPanelStopsThePageHighlight pins the panel's other half: it
// asks the page to stop highlighting as it closes.
func TestClosingTheFindPanelStopsThePageHighlight(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "A")

	s.closeFind()
	if s.find.open {
		t.Error("the panel stayed open")
	}
	if len(s.find.matches) == 0 {
		t.Error("closing the panel dropped the matches the next open would show")
	}
	s.openFind()
	if !s.find.open {
		t.Error("the panel did not open again")
	}
}
