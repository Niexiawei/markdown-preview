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
	// The list's own accessible name: the row's text is what fits, the label
	// is the whole path.
	if err := tt.Click("打开：" + names[0]); err != nil {
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
	rect, ok := tt.Find("文档：" + names[0])
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

// TestTabsSitAboveTheTitle pins the order the user asked for: the tabs are the
// top row of the preview (in the shell's header slot), the document's path and
// mode switch come under them, and the body under that.
func TestTabsSitAboveTheTitle(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n\n正文标记\n")
	s.Refresh()
	s.selectDoc("alpha.md")

	tt := ui.NewTester(s.View, 900, 700)
	// The tab's own accessible name, the title row's mode switch, and a
	// paragraph the body alone shows.
	tab, ok := tt.Find("文档：alpha.md")
	if !ok {
		t.Fatalf("texts = %q, want the tab", tt.Texts())
	}
	title, ok := tt.Find("WebView")
	if !ok {
		t.Fatalf("texts = %q, want the title row's mode switch", tt.Texts())
	}
	body, ok := tt.Find("正文标记")
	if !ok {
		t.Fatalf("texts = %q, want the body's paragraph", tt.Texts())
	}
	if tab.Y+tab.H > title.Y {
		t.Errorf("tab at %v..%v, title at %v: the tabs are not above the title", tab.Y, tab.Y+tab.H, title.Y)
	}
	if title.Y+title.H > body.Y {
		t.Errorf("title at %v..%v, body at %v: the title is not above the body", title.Y, title.Y+title.H, body.Y)
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

// TestCmdWClosesTheTab pins the keyboard's close: Cmd/Ctrl+W closes the tab
// shown and shows its neighbour.
func TestCmdWClosesTheTab(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n")
	writeDoc(t, root, "beta.md", "# B\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	s.selectDoc("beta.md")

	tt := ui.NewTester(func(c *ui.Context) { s.Content(c) }, 900, 700)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	if s.hasTab(docSel{project: s.sel.project, path: "beta.md"}) {
		t.Errorf("tabs = %v, want beta.md closed", s.tabs)
	}
	if got := s.Selected(); got != "alpha.md" {
		t.Errorf("selected = %q, want the neighbour", got)
	}
}

// TestTabBarScrollsAndKeepsTheListInsideItsSlot pins the two halves of an
// overflowing bar: the strip scrolls sideways rather than dropping tabs, while
// the button listing them all stays inside the room the bar is given — the shell
// reserves part of its header for the window's caption buttons.
func TestTabBarScrollsAndKeepsTheListInsideItsSlot(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 8 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}
	// The window's content is wide, the header slot the bar builds in is not.
	s.SetContentWidth(func() float32 { return 1400 })

	const slot float32 = 520
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Width(slot).Height(48).Children(func() { s.tabBar(c) })
	}, int(slot), 48)
	tt.Frame()
	tt.Frame()

	if s.tabScroll.MaxX <= 0 {
		t.Errorf("MaxX = %v, want a strip that scrolls sideways", s.tabScroll.MaxX)
	}
	if got := s.tabScroll.X; got <= 0 {
		t.Errorf("X = %v, want the strip scrolled to the tab shown", got)
	}
	over, ok := tt.Find("全部已打开文件")
	if !ok {
		t.Fatalf("tab texts = %q, want the overflow button", tt.Texts())
	}
	if over.X+over.W > slot {
		t.Errorf("the overflow button ends at %v, past the slot's %v", over.X+over.W, slot)
	}
}

// TestTabScrollFollowsTheTabShown pins the strip's two directions: choosing a
// tab at the far end scrolls it into view, and choosing the first brings the
// strip back to its start.
func TestTabScrollFollowsTheTabShown(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 10 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	s.SetContentWidth(func() float32 { return 600 })
	build := func(c *ui.Context) {
		ui.Row(c).Width(600).Height(48).Children(func() { s.tabBar(c) })
	}
	for _, name := range names {
		s.selectDoc(name)
	}

	s.selectDoc(names[9])
	tt := ui.NewTester(build, 600, 48)
	tt.Frame()
	tt.Frame()
	if s.tabScroll.X <= 0 {
		t.Errorf("X = %v, want the strip scrolled to the tenth tab", s.tabScroll.X)
	}

	s.selectDoc(names[0])
	tt.Frame()
	tt.Frame()
	if got := s.tabScroll.X; got != 0 {
		t.Errorf("X = %v, want the strip back at its start for the first tab", got)
	}
}

// TestTabStripScrollsWithTheWheel pins the wheel over the strip: it moves the
// tabs sideways, since the strip has no other axis for the wheel to scroll.
func TestTabStripScrollsWithTheWheel(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 10 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}
	s.selectDoc(names[0]) // the strip rests at its start
	s.SetContentWidth(func() float32 { return 600 })
	build := func(c *ui.Context) {
		ui.Row(c).Width(600).Height(48).Children(func() { s.tabBar(c) })
	}
	tt := ui.NewTester(build, 600, 48)
	tt.Frame()
	tt.Frame()
	if s.tabScroll.X != 0 {
		t.Fatalf("X = %v, want the strip at its start", s.tabScroll.X)
	}

	tt.Wheel(300, 20, 0, -120)
	tt.Frame()
	tt.Frame()
	if s.tabScroll.X <= 0 {
		t.Errorf("X = %v, want the wheel to scroll the strip sideways", s.tabScroll.X)
	}
}

// TestTabStripFollowsTheRoomItHas pins that the tab shown is brought fully into
// view when the strip's room changes too, not only when another tab is chosen: a
// narrower window must not leave the tab shown half under the list button.
func TestTabStripFollowsTheRoomItHas(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 10 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	for _, name := range names {
		s.selectDoc(name)
	}
	s.selectDoc(names[7])

	build := func(width float32) func(c *ui.Context) {
		return func(c *ui.Context) {
			ui.Row(c).Width(width).Height(48).Children(func() { s.tabBar(c) })
		}
	}
	wide := ui.NewTester(build(900), 900, 48)
	wide.Frame()
	wide.Frame()
	wideX := s.tabScroll.X

	narrow := ui.NewTester(build(520), 520, 48)
	narrow.Frame()
	narrow.Frame()
	if s.tabScroll.X <= wideX {
		t.Errorf("X = %v in the narrow strip, want more than the %v it had room for", s.tabScroll.X, wideX)
	}
}

// TestTabClicksReachTheTabBottom pins the scroll bar's room: it is a hairline,
// so a click on the bottom of a tab is a click on the tab rather than on the
// bar lying over it.
func TestTabClicksReachTheTabBottom(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "alpha.md", "# A\n")
	writeDoc(t, root, "beta.md", "# B\n")
	s.Refresh()
	s.selectDoc("alpha.md")
	s.selectDoc("beta.md")
	s.selectDoc("alpha.md")

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Width(600).Height(tabBarHeight).Children(func() { s.tabBar(c) })
	}, 600, int(tabBarHeight))
	tt.Frame()
	rect, ok := tt.Find("文档：beta.md")
	if !ok {
		t.Fatalf("texts = %q, want beta.md's tab", tt.Texts())
	}
	// A finger's width above the bottom, inside the tab's own box.
	tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H-2)
	tt.Frame()
	if got := s.Selected(); got != "beta.md" {
		t.Errorf("selected = %q, want beta.md: the click reached the tab, not a bar over it", got)
	}
}

// TestTabStripShowsTheWholeTabShown pins where the strip comes to rest: the tab
// shown is wholly inside it, and its left edge cuts no tab — it sits at a tab's
// start, or, where a strip of wide tabs cannot reach that far, at its end.
func TestTabStripShowsTheWholeTabShown(t *testing.T) {
	s, root := newState(t)
	var names []string
	for i := range 10 {
		name := fmt.Sprintf("文档-%02d-很长很长的名字.md", i)
		names = append(names, name)
		writeDoc(t, root, name, "# x\n")
	}
	s.Refresh()
	s.SetContentWidth(func() float32 { return 600 })

	// The starts and widths of the tabs, measured as the bar measures them.
	var starts, widths []float32
	build := func(c *ui.Context) {
		starts, widths = starts[:0], widths[:0]
		x := float32(0)
		for _, name := range names {
			w := tabWidth(c, name)
			starts = append(starts, x)
			widths = append(widths, w)
			x += w + c.Theme().Space(0.5)
		}
		ui.Row(c).Width(600).Height(48).Children(func() { s.tabBar(c) })
	}

	for i, name := range names {
		s.selectDoc(name)
		tt := ui.NewTester(build, 600, 48)
		tt.Frame()
		tt.Frame()
		rest, view := s.tabScroll.X, s.tabView
		if starts[i] < rest {
			t.Errorf("with %s shown the strip rests at %v, past the tab's own start %v", name, rest, starts[i])
		}
		if starts[i]+widths[i] > rest+view {
			t.Errorf("with %s shown the tab ends at %v, past the strip's %v", name, starts[i]+widths[i], rest+view)
		}
		if rest == 0 || rest == s.tabScroll.MaxX {
			continue
		}
		onStart := false
		for _, start := range starts {
			if rest == start {
				onStart = true
				break
			}
		}
		if !onStart {
			t.Errorf("with %s shown the strip rests at %v, which is neither the end nor the start of a tab: %v",
				name, rest, starts)
		}
	}
}
