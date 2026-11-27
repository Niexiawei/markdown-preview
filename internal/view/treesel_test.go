package view

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// rowOfPath is a document's or a directory's row in the window's only project.
func rowOfPath(s *State, path string) treeKey {
	return treeKey{project: s.trees[0].project.ID, path: path}
}

// treeKeyOfProject is a project's own row, by the name it shows.
func treeKeyOfProject(s *State, name string) treeKey {
	for _, tr := range s.trees {
		if tr.project.Name == name {
			return treeKey{project: tr.project.ID}
		}
	}
	return treeKey{}
}

// TestAPlainPickMakesTheRowTheOnlyOnePicked pins that picking rows without the
// modifier keeps a single row: the last one.
func TestAPlainPickMakesTheRowTheOnlyOnePicked(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	a, b := rowOfPath(s, "docs/a.md"), rowOfPath(s, "b.md")

	s.selectRow(a, 0)
	if want := []treeKey{a}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
	s.selectRow(b, 0)
	if want := []treeKey{b}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

// TestCtrlPicksRowsAndTakesThemOutAgain pins the modifier: adding on top, and
// the same row twice taking it out.
func TestCtrlPicksRowsAndTakesThemOutAgain(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	a, b := rowOfPath(s, "docs/a.md"), rowOfPath(s, "b.md")

	s.selectRow(a, ui.Ctrl)
	s.selectRow(b, ui.Ctrl)
	if want := []treeKey{a, b}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want both rows in the order picked", s.treeSel)
	}
	s.selectRow(a, ui.Ctrl)
	if want := []treeKey{b}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

// TestAPlainPickAfterModifierPicksDropsTheOthers pins that a plain click starts
// a new selection rather than adding to the old one.
func TestAPlainPickAfterModifierPicksDropsTheOthers(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	a, b := rowOfPath(s, "docs/a.md"), rowOfPath(s, "b.md")

	s.selectRow(a, ui.Ctrl)
	s.selectRow(b, ui.Ctrl)
	s.selectRow(a, 0)
	if want := []treeKey{a}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want only %v", s.treeSel, want)
	}
}

// TestOpeningADocumentPicksItsRow pins that showing a document is picking it:
// the selection follows, so that Cmd+C copies what the user just opened rather
// than the folder picked before.
func TestOpeningADocumentPicksItsRow(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectRow(rowOfPath(s, "docs"), 0)

	s.selectDoc("docs/a.md")
	if want := []treeKey{rowOfPath(s, "docs/a.md")}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

// TestClosingThePickedDocumentPicksTheTabThatTakesOver pins that the selection
// does not stay on a tab the window closed.
func TestClosingThePickedDocumentPicksTheTabThatTakesOver(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()
	s.selectDoc("docs/a.md")
	s.selectDoc("docs/b.md")

	s.closeTab(s.sel)
	if want := []treeKey{rowOfPath(s, "docs/a.md")}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the tab that took over", s.treeSel)
	}
}

// TestPickedRowsGoWhenTheirProjectGoes pins the pruning of a closed project.
func TestPickedRowsGoWhenTheirProjectGoes(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	writeDoc(t, betaRoot, "docs/a.md", "# A\n")
	s.Refresh()
	key := treeKeyOfProject(s, "beta")
	s.selectRow(key, 0)
	if len(s.treeSel) != 1 {
		t.Fatalf("treeSel = %v, want the project node picked", s.treeSel)
	}

	s.dropTreeSelOf(key.project)
	if len(s.treeSel) != 0 {
		t.Errorf("treeSel = %v, want nothing picked", s.treeSel)
	}
}

// TestPickedRowsGoWhenTheFileGoes pins the pruning of a file that went away
// under a scan: a picked row the tree no longer has cannot be copied.
func TestPickedRowsGoWhenTheFileGoes(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectRow(rowOfPath(s, "docs/a.md"), 0)

	if err := os.Remove(filepath.Join(root, "docs", "a.md")); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if len(s.treeSel) != 0 {
		t.Errorf("treeSel = %v, want the row of the gone file dropped", s.treeSel)
	}
}

// TestPickedRowsSurviveACollapsedDirectory pins the other side: a folded row is
// still picked, so its path can be copied.
func TestPickedRowsSurviveACollapsedDirectory(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	a := rowOfPath(s, "docs/a.md")
	s.selectRow(a, 0)

	s.setTreeKeyOpen(rowOfPath(s, "docs"), false)
	if want := []treeKey{a}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the folded row still picked", s.treeSel)
	}
}

// TestAPlainClickMakesTheRowTheOnlyOneSelected pins the pointer: a plain click
// picks the row it hits, whichever row was picked before.
func TestAPlainClickMakesTheRowTheOnlyOneSelected(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.Click("a.md"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if err := tt.Click("docs"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := []treeKey{rowOfPath(s, "docs")}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

// TestCtrlClickPicksSeveralRowsAndDoesNotOpenThem pins the modifier on the
// pointer: the row is picked, and the document it names is not shown.
func TestCtrlClickPicksSeveralRowsAndDoesNotOpenThem(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "b.md"); err != nil {
		t.Fatalf("ClickWith: %v", err)
	}
	if !s.sel.empty() {
		t.Errorf("sel = %+v, want no document shown", s.sel)
	}
	if want := []treeKey{rowOfPath(s, "docs/b.md")}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
	// The same click again takes it out.
	if err := tt.ClickWith(ui.Ctrl, "b.md"); err != nil {
		t.Fatalf("ClickWith: %v", err)
	}
	if len(s.treeSel) != 0 {
		t.Errorf("treeSel = %v, want nothing picked", s.treeSel)
	}
}

// TestCtrlClickAddsToTheDocumentShown pins that the document shown is already
// one of the picked rows, so that picking another adds to it.
func TestCtrlClickAddsToTheDocumentShown(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "docs/b.md", "# B\n")
	s.Refresh()
	s.selectDoc("docs/a.md")

	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "b.md"); err != nil {
		t.Fatalf("ClickWith: %v", err)
	}
	want := []treeKey{rowOfPath(s, "docs/a.md"), rowOfPath(s, "docs/b.md")}
	if !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the shown document and the picked row", s.treeSel)
	}
	if s.sel.path != "docs/a.md" {
		t.Errorf("sel = %+v, want the document shown unchanged", s.sel)
	}
}

// TestCtrlClickDoesNotFoldADirectory pins the same for a directory: picking it
// must not fold it, or the rows the user is picking would vanish.
func TestCtrlClickDoesNotFoldADirectory(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	key := rowOfPath(s, "docs")
	if !s.treeOpen.Open.Has(key) {
		t.Fatal("the directory should start open")
	}

	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "docs"); err != nil {
		t.Fatalf("ClickWith: %v", err)
	}
	if !s.treeOpen.Open.Has(key) {
		t.Error("a Ctrl-click folded the directory; it should only pick it")
	}
	if want := []treeKey{key}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}

// TestOpeningADocumentMovesTheSelection pins the selection following a click on
// a document: it leaves the directory picked before.
func TestOpeningADocumentMovesTheSelection(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.Click("docs"); err != nil { // picks the directory, folds it
		t.Fatalf("Click: %v", err)
	}
	if err := tt.Click("b.md"); err != nil { // opens the document
		t.Fatalf("Click: %v", err)
	}
	if want := []treeKey{rowOfPath(s, "b.md")}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the document just opened", s.treeSel)
	}
}

// TestCtrlClickPicksAProjectNodeToo pins the project's own row: it is a folder
// like any other, and can be picked to copy its path.
func TestCtrlClickPicksAProjectNodeToo(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	writeDoc(t, betaRoot, "docs/a.md", "# A\n")
	s.Refresh()
	key := treeKeyOfProject(s, "beta")

	tt := newTester(s)
	if err := tt.ClickWith(ui.Ctrl, "beta"); err != nil {
		t.Fatalf("ClickWith: %v", err)
	}
	if want := []treeKey{key}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want the project node picked", s.treeSel)
	}
}

// The project-node tests click "beta", never "alpha": the panel's button shows
// the *active* project's name, so "alpha" names two labels and the Tester would
// hit the button first. "beta" names one label in the window: the tree's row.

// TestAPlainClickOnAProjectNodeStillFoldsIt pins that picking did not take the
// row's own action away.
func TestAPlainClickOnAProjectNodeStillFoldsIt(t *testing.T) {
	s, alphaRoot, _ := twoProjects(t)
	writeDoc(t, alphaRoot, "a.md", "# A\n")
	s.Refresh()
	key := treeKeyOfProject(s, "beta")
	if !s.treeOpen.Open.Has(key) {
		t.Fatal("the project node should start open")
	}

	tt := newTester(s)
	if err := tt.Click("beta"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.treeOpen.Open.Has(key) {
		t.Error("a plain click should have folded the project node")
	}
	if want := []treeKey{key}; !slices.Equal(s.treeSel, want) {
		t.Errorf("treeSel = %v, want %v", s.treeSel, want)
	}
}
