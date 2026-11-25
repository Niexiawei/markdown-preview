package view

import (
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"

	"mdpreview/internal/watch"
	"mdpreview/internal/workspace"
)

// countingScan replaces the scan with one that counts, and reads the tree as
// the app does.
func countingScan(s *State) *atomic.Int64 {
	scans := &atomic.Int64{}
	s.scan = func(p workspace.Project) (*workspace.Node, error) {
		scans.Add(1)
		return workspace.Tree(p)
	}
	return scans
}

func TestPatchingTheTreeMatchesARescan(t *testing.T) {
	patched, patchedRoot := newState(t)
	scanned, scannedRoot := newState(t)
	for _, root := range []string{patchedRoot, scannedRoot} {
		writeDoc(t, root, "docs/a.md", "# A\n")
		writeDoc(t, root, "docs/b.md", "# B\n")
		writeDoc(t, root, "top.md", "# Top\n")
	}
	patched.Refresh()
	scanned.Refresh()
	// Both directories end up the same; one hears the changes one by one, the
	// other is simply read again.
	for _, root := range []string{patchedRoot, scannedRoot} {
		writeDoc(t, root, "docs/c.md", "# C\n")
		if err := os.Rename(filepath.Join(root, "docs", "b.md"), filepath.Join(root, "docs", "d.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, "top.md")); err != nil {
			t.Fatal(err)
		}
	}
	sets := []watch.ChangeSet{
		{Docs: []watch.Change{{Op: mygo.FileCreate, Path: "docs/c.md"}}, TreeDirty: true},
		{Docs: []watch.Change{{Op: mygo.FileRename, Path: "docs/d.md", Old: "docs/b.md"}}, TreeDirty: true},
		{Docs: []watch.Change{{Op: mygo.FileRemove, Path: "top.md"}}, TreeDirty: true},
	}
	for _, set := range sets {
		patched.applyChanges(patched.activeID, set)
	}
	scanned.Refresh()
	// The roots' names are their two directories, which differ by design.
	if got, want := patched.Trees()[0].root.Children, scanned.Trees()[0].root.Children; !reflect.DeepEqual(got, want) {
		t.Errorf("patched tree = %+v, want the scanned one %+v", got, want)
	}
}

func TestAWriteDoesNotRescanTheTree(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	scans := countingScan(s)
	writeDoc(t, root, "a.md", "# A again\n")
	s.applyChanges(s.activeID, watch.ChangeSet{Docs: []watch.Change{{Op: mygo.FileWrite, Path: "a.md"}}})
	if got := scans.Load(); got != 0 {
		t.Errorf("scans = %d, want none: a write changes no entry", got)
	}
}

func TestANewFileAddsOneNode(t *testing.T) {
	s, root := newState(t)
	s.Refresh()
	scans := countingScan(s)
	writeDoc(t, root, "docs/new.md", "# New\n")
	s.applyChanges(s.activeID, watch.ChangeSet{
		Docs:      []watch.Change{{Op: mygo.FileCreate, Path: "docs/new.md"}},
		TreeDirty: true,
	})
	if got := scans.Load(); got != 0 {
		t.Errorf("scans = %d, want none: the change says what appeared", got)
	}
	tt := newTester(s)
	if !tt.HasText("docs") || !tt.HasText("new.md") {
		t.Errorf("texts = %q, want the new document in the tree", tt.Texts())
	}
}

func TestADeadDirectoryRescans(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	scans := countingScan(s)
	if err := os.RemoveAll(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	// The watch of a directory that went away says only that the tree changed.
	s.applyChanges(s.activeID, watch.ChangeSet{TreeDirty: true})
	if got := scans.Load(); got != 1 {
		t.Errorf("scans = %d, want 1: a change without a document is read again", got)
	}
	tt := newTester(s)
	if tt.HasText("docs") {
		t.Errorf("texts = %q, want the directory gone", tt.Texts())
	}
}

// TestAChangeWithNoTreeYetIsReadAgain pins the case of a file event that
// arrives before the first scan of its project did: there is no tree to patch,
// so the project is read again rather than the change being lost.
func TestAChangeWithNoTreeYetIsReadAgain(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.trees = nil
	scans := countingScan(s)
	s.applyChanges(s.activeID, watch.ChangeSet{
		Docs:      []watch.Change{{Op: mygo.FileCreate, Path: "a.md"}},
		TreeDirty: true,
	})
	if got := scans.Load(); got != 1 {
		t.Errorf("scans = %d, want 1: without a tree the change cannot be made to one", got)
	}
	if trees := s.Trees(); len(trees) != 1 || !hasChild(trees[0].root, "a.md") {
		t.Errorf("Trees = %v, want the project read again", trees)
	}
}
