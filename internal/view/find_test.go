package view

import (
	"strconv"
	"strings"
	"testing"
	"time"

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

// TestFindJumpSetsTheBlockToJumpTo pins what a click on a match hands the body:
// the block holding it, which the body brings to its top.
func TestFindJumpSetsTheBlockToJumpTo(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n甲段\n\n乙段 目标\n\n丙段\n")
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")
	s.gotoMatch(0)

	want := blockAt(s.doc, s.find.matches[0].Offset)
	if want <= 0 || s.jump != want {
		t.Fatalf("jump = %d, want the match's block %d", s.jump, want)
	}
	if got := s.doc.Block(want); got.Kind != parser.Paragraph {
		t.Errorf("block kind = %v, want the paragraph holding the match", got.Kind)
	}
}

// TestFindJumpPutsTheMatchAtTheTop pins that a hit scrolls the body the way the
// outline does: the matched block at the pane's top, gliding there.
func TestFindJumpPutsTheMatchAtTheTop(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "P5 正文，第 1 行")

	tt := newTester(s)
	s.gotoMatch(0)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if s.jump != 0 {
		t.Errorf("jump = %d, want it taken by the block's build", s.jump)
	}
	if s.docScroll().Y <= 0 {
		t.Fatalf("Y = %v, want the body scrolled to the hit", s.docScroll().Y)
	}
	rect, ok := tt.Find("P5 正文，第 1 行")
	if !ok {
		t.Fatalf("texts = %q, want the matched line", tt.Texts())
	}
	if rect.Y > 200 {
		t.Errorf("match at Y=%v, want it near the pane's top", rect.Y)
	}
}

// TestSecondJumpTurnsTowardTheNewTarget pins that walking the matches replaces
// the place asked for, rather than adding to it: the last one asked for wins.
func TestSecondJumpTurnsTowardTheNewTarget(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "第 1 行")
	if len(s.find.matches) < 6 {
		t.Fatalf("matches = %v, want one a section", s.find.matches)
	}
	s.gotoMatch(0)
	first := s.jump
	s.gotoMatch(5)
	want := blockAt(s.doc, s.find.matches[5].Offset)
	if s.jump != want || s.jump == first {
		t.Errorf("jump = %d, want %d (the last match asked for), not %d", s.jump, want, first)
	}
}

// TestJumpDoesNotCarryToAnotherDocument pins that the block asked for is the
// document's own: another document shows from its top, not at block 12 of the
// one searched.
func TestJumpDoesNotCarryToAnotherDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", sections(8))
	writeDoc(t, root, "beta.md", "丁\n\n戊\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	openFindOn(s, "P6 正文")
	s.gotoMatch(0)
	if s.jump == 0 {
		t.Fatal("no block was asked for")
	}
	s.selectDoc("beta.md")
	if s.jump != 0 {
		t.Errorf("jump = %d, want it left with its document", s.jump)
	}
	if s.anchor != "" {
		t.Errorf("anchor = %q, want none on another document", s.anchor)
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

// TestFindFieldTakesTypingWhenReopened pins that the panel's field takes the
// keyboard every time the panel opens, even with the last phrase still in it.
func TestFindFieldTakesTypingWhenReopened(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "导入目标\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("导入")
	tt.Frame()
	if s.find.query != "导入" {
		t.Fatalf("query = %q, want what was typed", s.find.query)
	}
	if len(s.find.matches) != 1 {
		t.Fatalf("matches = %v, want the typed phrase found", s.find.matches)
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.query != "" {
		t.Fatalf("query = %q, want Escape to empty it", s.find.query)
	}
	// The phrase a close keeps, as the panel's Escape leaves it: opening again
	// searches for it once more.
	s.find.query = "导入"
	s.closeFind()
	tt.Frame()

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("目标")
	tt.Frame()
	if s.find.query != "导入目标" {
		t.Errorf("query = %q, want the typing to land in the reopened field", s.find.query)
	}
}

// TestFindCountSaysWhenItCapped pins the panel's count once the source has more
// matches than the panel lists.
func TestFindCountSaysWhenItCapped(t *testing.T) {
	s, root := newState(t)
	var b strings.Builder
	for i := range 600 {
		b.WriteString("目标 " + strconv.Itoa(i) + "\n")
	}
	writeDoc(t, root, "a.md", b.String())
	s.Refresh()
	s.selectDoc("a.md")
	openFindOn(s, "目标")

	if !s.find.truncated {
		t.Fatal("truncated = false, want the source's extra matches reported")
	}
	if got := s.findCount(); !strings.Contains(got, "前") || !strings.Contains(got, "1/") {
		t.Errorf("count = %q, want both the position and the cap", got)
	}
}
