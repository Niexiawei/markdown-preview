package view

import (
	"strconv"
	"strings"
	"testing"
)

// treeKeyOf names a row of the tree, for a test that reaches into its state.
func treeKeyOf(projectID, path string) treeKey { return treeKey{project: projectID, path: path} }

func TestTheTreeOnlyBuildsWhatShows(t *testing.T) {
	s, root := newState(t)
	for i := range 2000 {
		writeDoc(t, root, "docs/f"+strconv.Itoa(i)+".md", "# F\n")
	}
	s.Refresh()
	tt := newTester(s)
	if got := s.treeOpen.Rows(); got != 2001 {
		t.Fatalf("Rows = %d, want the 2001 entries all open", got)
	}
	shown := 0
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "f") && strings.HasSuffix(text, ".md") {
			shown++
		}
	}
	if shown == 0 {
		t.Fatal("no document row was built")
	}
	if shown > 200 {
		t.Errorf("built %d rows of 2001, want only what the pane shows", shown)
	}
}

func TestUnfoldingFoldingAndOpeningTheWholeTree(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/deep/a.md", "# A\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	newTester(s)
	if got := s.treeOpen.Rows(); got != 3 {
		t.Fatalf("Rows = %d, want the tree open as it was read", got)
	}
	s.setTreeOpen("", false)
	newTester(s)
	if got := s.treeOpen.Rows(); got != 1 {
		t.Errorf("Rows after folding = %d, want the top level only", got)
	}
	s.setTreeOpen("", true)
	newTester(s)
	if got := s.treeOpen.Rows(); got != 3 {
		t.Errorf("Rows after unfolding = %d, want every directory open", got)
	}
	if !s.treeOpen.Open.Has(treeKeyOf(id, "docs")) || !s.treeOpen.Open.Has(treeKeyOf(id, "docs/deep")) {
		t.Error("unfolding left a directory closed")
	}
}

func TestTheTreeKeepsWhatTheUserFoldedAcrossAReload(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "top.md", "# Top\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	s.setTreeKeyOpen(treeKeyOf(id, "docs"), false)
	s.Refresh()
	if s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("the reload unfolded the directory the user folded")
	}
	if tt := newTester(s); tt.HasText("a.md") {
		t.Errorf("texts = %q, want the folded directory to stay folded", tt.Texts())
	}
}

func TestTheNewDirectoriesOpenByThemselves(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	writeDoc(t, root, "fresh/b.md", "# B\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	if !s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("the directory read before is closed, want it as the user left it")
	}
	if !s.treeOpen.Open.Has(treeKeyOf(id, "fresh")) {
		t.Error("a directory the window just read is closed, want it open")
	}
	if tt := newTester(s); !tt.HasText("b.md") {
		t.Errorf("texts = %q, want the new directory's document", tt.Texts())
	}
}

func TestAProjectNodeOpensAndClosesItsWholeProject(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "docs/b.md", "# B\n")
	s.Refresh()
	alpha := s.Trees()[0].project.ID
	s.setTreeKeyOpen(treeKeyOf(alpha, ""), false)
	tt := newTester(s)
	if tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's entries closed with its node", tt.Texts())
	}
	if !tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta's entries alone", tt.Texts())
	}
	s.setTreeKeyOpen(treeKeyOf(alpha, ""), true)
	if tt := newTester(s); !tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's entries back", tt.Texts())
	}
}

func TestTheTreeRowsAreWhatTheTreeHolds(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "top.md", "# Top\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	newTester(s)
	rows := make([]treeKey, 0, s.treeOpen.Rows())
	for i := range s.treeOpen.Rows() {
		rows = append(rows, s.treeOpen.Item(i))
	}
	want := []treeKey{
		{id, "docs"},
		{id, "docs/a.md"},
		{id, "top.md"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows = %v, want %v", rows, want)
		}
	}
	// The tree's own lookup agrees with the rows it built.
	for i, k := range want {
		if got := s.rowOf(s.roots(), k); got != i {
			t.Errorf("rowOf(%v) = %d, want %d", k, got, i)
		}
	}
	if got := s.rowOf(s.roots(), treeKeyOf(id, "docs/gone.md")); got != -1 {
		t.Errorf("rowOf(a file the tree does not have) = %d, want -1", got)
	}
}
