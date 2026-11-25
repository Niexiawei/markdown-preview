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

// TestClickingTheArrowOfADirectoryOpensItOnce pins the arrow in the box before
// a row's name: one press there opens or closes the directory, and does not
// also press the row, which would close it again.
func TestClickingTheArrowOfADirectoryOpensItOnce(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	tt := newTester(s)
	text, ok := tt.Find("docs")
	if !ok {
		t.Fatalf("no docs row; texts = %q", tt.Texts())
	}
	// The arrow sits in the square before the name, past the row's padding.
	tt.ClickAt(text.X-10, text.Y+text.H/2)
	tt.Frame()
	if s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("docs is still open, want the arrow to close it")
	}
	tt.ClickAt(text.X-10, text.Y+text.H/2)
	tt.Frame()
	if !s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("docs is still closed, want a second press to open it")
	}
}

// TestFoldingEveryTreeClosesTheProjects pins the whole-tree fold with several
// projects open: each project's own node folds with its directories, so that
// folding everything leaves the names, as a window of one project folds down to
// its top level.
func TestFoldingEveryTreeClosesTheProjects(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only/b.md", "# B\n")
	s.Refresh()
	beta := s.Trees()[1].project.ID
	s.setTreeOpen("", false)
	tt := newTester(s)
	for _, gone := range []string{"docs", "a.md", "only", "b.md"} {
		if tt.HasText(gone) {
			t.Errorf("texts = %q, want %q folded away with the node", tt.Texts(), gone)
		}
	}
	if !tt.HasText("alpha") || !tt.HasText("beta") {
		t.Errorf("texts = %q, want the project names left", tt.Texts())
	}
	if s.treeOpen.Open.Has(treeKeyOf(beta, "")) {
		t.Error("beta's node is open, want it closed with the rest")
	}
	s.setTreeOpen("", true)
	if tt := newTester(s); !tt.HasText("a.md") || !tt.HasText("b.md") {
		t.Errorf("texts = %q, want every entry back", tt.Texts())
	}
}
