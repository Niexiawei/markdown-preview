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
