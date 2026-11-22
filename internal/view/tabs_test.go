package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"

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
