package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"mdpreview/internal/watch"
)

func writeBody(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyChangesRefreshesTree(t *testing.T) {
	s, root := newState(t)
	writeBody(t, root, "a.md", "# A\n")
	s.Refresh()
	writeBody(t, root, "new.md", "# New\n")
	s.applyChanges(s.activeID, watch.ChangeSet{TreeDirty: true})
	tt := newTester(s)
	if !tt.HasText("new.md") {
		t.Errorf("texts = %q, want the new file in the tree", tt.Texts())
	}
}

func TestApplyChangesReloadsSelectedDoc(t *testing.T) {
	s, root := newState(t)
	writeBody(t, root, "a.md", "# Old\n")
	s.Refresh()
	s.selectDoc("a.md")
	writeBody(t, root, "a.md", "# Fresh\n\n## Added\n")
	s.applyChanges(s.activeID, watch.ChangeSet{Docs: []watch.Change{{Op: mygo.FileWrite, Path: "a.md"}}})
	if !strings.Contains(s.src, "Fresh") {
		t.Errorf("src = %q, want the new body", s.src)
	}
	if len(s.headings) != 2 || s.headings[1].Text != "Added" {
		t.Errorf("headings = %v, want the new outline", s.headings)
	}
	if s.mode() == "" {
		t.Error("mode lost")
	}
}

func TestApplyChangesFollowsRename(t *testing.T) {
	s, root := newState(t)
	writeBody(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	if err := os.Rename(filepath.Join(root, "a.md"), filepath.Join(root, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	s.applyChanges(s.activeID, watch.ChangeSet{
		TreeDirty: true,
		Docs:      []watch.Change{{Op: mygo.FileRename, Path: "renamed.md", Old: "a.md"}},
	})
	if got := s.Selected(); got != "renamed.md" {
		t.Errorf("Selected = %q, want renamed.md", got)
	}
	if !strings.Contains(s.src, "# A") {
		t.Errorf("src = %q, want the document still shown", s.src)
	}
}

func TestApplyChangesClearsRemovedDoc(t *testing.T) {
	s, root := newState(t)
	writeBody(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}
	s.applyChanges(s.activeID, watch.ChangeSet{
		TreeDirty: true,
		Docs:      []watch.Change{{Op: mygo.FileRemove, Path: "a.md"}},
	})
	if got := s.Selected(); got != "" {
		t.Errorf("Selected = %q, want none", got)
	}
	if !strings.Contains(s.Status(), "删除") {
		t.Errorf("status = %q, want the file gone", s.Status())
	}
}

func TestApplyChangesIgnoresOtherProjects(t *testing.T) {
	s, root := newState(t)
	writeBody(t, root, "a.md", "# A\n")
	s.Refresh()
	writeBody(t, root, "new.md", "# New\n")
	s.applyChanges("another-project", watch.ChangeSet{TreeDirty: true})
	tt := newTester(s)
	if tt.HasText("new.md") {
		t.Errorf("texts = %q, want nothing applied for another project", tt.Texts())
	}
}

func TestApplyChangesOnlyTouchesItsOwnProjectsDoc(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "docs/a.md", "# B\n")
	s.Refresh()
	trees := s.Trees()
	s.selectDocIn(trees[0].project.ID, "docs/a.md")
	// The other project reports the same path removed.
	s.applyChanges(trees[1].project.ID, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRemove, Path: "docs/a.md"}},
	})
	if got := s.Selected(); got != "docs/a.md" {
		t.Errorf("Selected = %q, want alpha's document untouched", got)
	}
	s.applyChanges(trees[0].project.ID, watch.ChangeSet{
		Docs: []watch.Change{{Op: mygo.FileRemove, Path: "docs/a.md"}},
	})
	if got := s.Selected(); got != "" {
		t.Errorf("Selected = %q, want the removed document cleared", got)
	}
}
