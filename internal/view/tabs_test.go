package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/watch"
)

// TestOpeningDocsMakesTabs pins that every document the window shows becomes a
// tab, in the order it was opened, and that opening one already open only
// switches to it.
func TestOpeningDocsMakesTabs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()

	s.selectDoc("a.md")
	s.selectDoc("b.md")
	s.selectDoc("a.md")

	if len(s.tabs) != 2 {
		t.Fatalf("tabs = %v, want two", s.tabs)
	}
	if s.tabs[0].path != "a.md" || s.tabs[1].path != "b.md" {
		t.Errorf("tabs = %v, want a.md then b.md in opening order", s.tabs)
	}
	if got := s.Selected(); got != "a.md" {
		t.Errorf("selected = %q, want a.md: reopening switched to it", got)
	}
}

// TestClosingTabs pins what closing a tab does to the document shown: the one
// to its right, else the one to its left, else nothing.
func TestClosingTabs(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")
	s.selectDoc("c.md")

	s.closeTab(s.sel) // c.md closes: nothing right, b.md takes over
	if got := s.Selected(); got != "b.md" {
		t.Errorf("selected = %q, want b.md after closing the last", got)
	}
	s.closeTab(s.tabs[0]) // a.md closes while b.md shows: still b.md
	if got := s.Selected(); got != "b.md" {
		t.Errorf("selected = %q, want b.md still", got)
	}
	s.closeTab(s.sel)
	if len(s.tabs) != 0 || !s.sel.empty() {
		t.Errorf("tabs = %v, selected = %q, want none", s.tabs, s.Selected())
	}
	if s.src != "" {
		t.Errorf("src = %q, want the body emptied", s.src)
	}
}

// TestTabsFollowRenamesAndDeletes pins that a tab never points at a document
// that moved or went away.
func TestTabsFollowRenamesAndDeletes(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.selectDoc("b.md")

	s.applyChanges(s.sel.project, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRename, Path: "z.md", Old: "a.md"}},
	})

	if len(s.tabs) != 2 {
		t.Fatalf("tabs = %v, want two", s.tabs)
	}
	if s.tabs[0].path != "z.md" {
		t.Errorf("tabs = %v, want the tab to follow the rename", s.tabs)
	}
	if _, ok := s.scrolls[docSel{project: s.sel.project, path: "z.md"}]; !ok {
		t.Errorf("scrolls = %v, want the position under the new path", s.scrolls)
	}

	// A renamed document keeps showing, under its new name.
	if err := os.Rename(filepath.Join(root, "b.md"), filepath.Join(root, "c.md")); err != nil {
		t.Fatal(err)
	}
	s.applyChanges(s.sel.project, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRename, Path: "c.md", Old: "b.md"}},
	})
	if got := s.Selected(); got != "c.md" {
		t.Errorf("selected = %q, want the document shown to follow its rename", got)
	}

	// A document that went away takes its tab with it, and the shown one
	// falls back to a neighbour.
	writeDoc(t, root, "d.md", "# D\n")
	s.selectDoc("d.md")
	if err := os.Remove(filepath.Join(root, "d.md")); err != nil {
		t.Fatal(err)
	}
	s.applyChanges(s.sel.project, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRemove, Path: "d.md"}},
	})
	if s.hasTab(docSel{project: s.sel.project, path: "d.md"}) {
		t.Errorf("tabs = %v, want d.md's tab gone", s.tabs)
	}
	if got := s.Selected(); got != "c.md" && got != "z.md" {
		t.Errorf("selected = %q, want a neighbour of the deleted document", got)
	}
}

// TestRemovingProjectDropsItsTabs pins that a removed project takes its tabs
// with it, and the document shown falls back to nothing.
func TestRemovingProjectDropsItsTabs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.removeProject(s.sel.project)
	if len(s.tabs) != 0 || !s.sel.empty() {
		t.Errorf("tabs = %v, selected = %q, want none", s.tabs, s.Selected())
	}
}

// TestScrollPositionsAreKeptPerTab pins that switching back to a tab shows it
// where the user left it.
func TestScrollPositionsAreKeptPerTab(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.docScroll().Y = 120
	s.selectDoc("b.md")
	if got := s.docScroll().Y; got != 0 {
		t.Errorf("b.md starts at %v, want 0", got)
	}
	s.selectDoc("a.md")
	if got := s.docScroll().Y; got != 120 {
		t.Errorf("a.md came back at %v, want 120", got)
	}
}

// TestClosingProjectDropsItsTabs pins the project panel's close action: a
// project that is closed leaves no tab behind.
func TestClosingProjectDropsItsTabs(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")

	s.ToggleProject(s.activeID)
	if len(s.tabs) != 0 {
		t.Errorf("tabs = %v, want none after closing the only project", s.tabs)
	}
	if !s.sel.empty() {
		t.Errorf("selected = %q, want none", s.Selected())
	}
}

// TestTabBarShowsEveryTab pins the bar above the body: every open document has
// a tab, and no overflow button shows while they fit.
func TestTabBarShowsEveryTab(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n")
	writeDoc(t, root, "beta.md", "# B\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	s.selectDoc("beta.md")

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 900, 40)
	for _, want := range []string{"alpha.md", "beta.md"} {
		if !tt.HasText(want) {
			t.Errorf("tab texts = %q, want %q", tt.Texts(), want)
		}
	}
	if tt.HasText("全部已打开文件") {
		t.Errorf("tab texts = %q, want no overflow button while they fit", tt.Texts())
	}
}

// TestTabBarOverflowsWithAButton pins the overflow: too many tabs and the
// rightmost control lists them all, and choosing one shows it.
func TestTabBarOverflowsWithAButton(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 12 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}

	// The list opens below the bar, so the window must have room for it.
	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 760, 460)
	if err := tt.Click("全部已打开文件"); err != nil {
		t.Fatalf("Click overflow: %v", err)
	}
	tt.Frame()
	if !s.tabsOpen {
		t.Fatal("the overflow button did not open the list")
	}
	if err := tt.Click(names[0]); err != nil {
		t.Fatalf("Click %s: %v", names[0], err)
	}
	if got := s.Selected(); got != names[0] {
		t.Errorf("selected = %q, want %q", got, names[0])
	}
	if s.tabsOpen {
		t.Error("the list stayed open after choosing a tab")
	}
}

// TestTabBarKeepsActiveTabVisible pins that the tab shown is never the one the
// bar leaves out.
func TestTabBarKeepsActiveTabVisible(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 12 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}
	s.selectDoc(names[0]) // the leftmost tab is the one shown

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 520, 400)
	rect, ok := tt.Find(names[0])
	if !ok {
		t.Fatalf("tab texts = %q, want the active tab's name", tt.Texts())
	}
	if rect.X+rect.W > 520 {
		t.Errorf("the active tab ends at %v, past the window's 520", rect.X+rect.W)
	}
	// The overflow button shares the room the tabs left: it must stay inside
	// the window, or nothing lists the tabs that were cut.
	over, ok := tt.Find("全部已打开文件")
	if !ok {
		t.Fatalf("tab texts = %q, want the overflow button", tt.Texts())
	}
	if over.X+over.W > 520 {
		t.Errorf("the overflow button ends at %v, past the window's 520", over.X+over.W)
	}
}

// TestClosingATabDoesNotActivateIt pins the close button's click: it closes the
// tab it sits in rather than switching to it first.
func TestClosingATabDoesNotActivateIt(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n")
	writeDoc(t, root, "beta.md", "# B\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	s.selectDoc("beta.md")

	tt := ui.NewTester(func(c *ui.Context) { s.tabBar(c) }, 900, 40)
	if err := tt.Click("关闭 alpha.md"); err != nil {
		t.Fatalf("Click close: %v", err)
	}
	if got := s.Selected(); got != "beta.md" {
		t.Errorf("selected = %q, want beta.md: closing a tab is not switching to it", got)
	}
	if s.hasTab(docSel{project: s.sel.project, path: "alpha.md"}) {
		t.Error("alpha.md still has a tab")
	}
}

// TestTabBarSitsAboveThePreview pins the bar's place in the shell's page: the
// tab row comes before the body, under the title.
func TestTabBarSitsAboveThePreview(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n\n正文标记\n")
	s.Refresh()
	s.selectDoc("alpha.md")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	// The tab's own accessible name, and a paragraph the body alone shows.
	tab, ok := tt.Find("文档：alpha.md")
	if !ok {
		t.Fatalf("texts = %q, want the tab", tt.Texts())
	}
	body, ok := tt.Find("正文标记")
	if !ok {
		t.Fatalf("texts = %q, want the body's paragraph", tt.Texts())
	}
	if tab.Y+tab.H > body.Y {
		t.Errorf("tab at %v..%v, body at %v: the bar is not above the body", tab.Y, tab.Y+tab.H, body.Y)
	}
}

// TestRefreshDropsTabsOfGoneFiles pins that a reload prunes tabs pointing at
// files that are no longer there, even when no change event reported them, and
// says so while switching to a neighbour.
func TestRefreshDropsTabsOfGoneFiles(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n")
	writeDoc(t, root, "beta.md", "# B\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	s.selectDoc("beta.md")

	if err := os.Remove(filepath.Join(root, "beta.md")); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if s.hasTab(docSel{project: s.sel.project, path: "beta.md"}) {
		t.Errorf("tabs = %v, want the deleted file's tab gone", s.tabs)
	}
	if got := s.Selected(); got != "alpha.md" {
		t.Errorf("selected = %q, want the neighbour", got)
	}
	if !strings.Contains(s.Status(), "删除") {
		t.Errorf("status = %q, want the deletion reported", s.Status())
	}

	// The same for a tab the window does not show: it is pruned too, and the
	// document shown stays.
	writeDoc(t, root, "gamma.md", "# G\n")
	s.Refresh()
	s.selectDoc("gamma.md")
	if err := os.Remove(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if s.hasTab(docSel{project: s.sel.project, path: "alpha.md"}) {
		t.Errorf("tabs = %v, want the tab of the file gone from disk dropped", s.tabs)
	}
	if got := s.Selected(); got != "gamma.md" {
		t.Errorf("selected = %q, want the document shown to stay", got)
	}
}
