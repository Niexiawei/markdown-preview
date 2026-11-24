package view

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

// openTreeSearchOn opens the tree's search on the window's trees and types
// query into it, as a user does.
func openTreeSearchOn(s *State, query string) {
	s.treeSearch.open = true
	s.setTreeQuery(query)
}

// TestCandidatesComeInTreeOrder pins the order of what the tree's search
// matches: the projects as they were opened, directories before the files
// beside them, each by name — the order the tree itself shows.
func TestCandidatesComeInTreeOrder(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a_plan.md", "# A\n")
	writeDoc(t, alphaRoot, "b_plan.md", "# B\n")
	writeDoc(t, betaRoot, "plan_c.md", "# C\n")
	s.Refresh()

	alpha, beta := s.Projects()[0].ID, s.Projects()[1].ID
	openTreeSearchOn(s, "plan")
	want := []docSel{
		{project: alpha, path: "docs/a_plan.md"},
		{project: alpha, path: "b_plan.md"},
		{project: beta, path: "plan_c.md"},
	}
	if !slices.Equal(s.treeSearch.candidates, want) {
		t.Errorf("candidates = %v, want %v", s.treeSearch.candidates, want)
	}
}

// TestCandidatesMarkTheirDirectories pins the directories a search keeps: the
// ones holding a candidate, on every level above it, and no others.
func TestCandidatesMarkTheirDirectories(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/deep/PLAN.md", "# Plan\n")
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	writeDoc(t, root, "other/x.md", "# X\n")
	writeDoc(t, root, "readme.md", "# Readme\n")
	s.Refresh()

	id := s.Projects()[0].ID
	openTreeSearchOn(s, "plan")
	want := []string{id + ":docs", id + ":docs/deep"}
	got := make([]string, 0, len(s.treeSearch.dirs))
	for key := range s.treeSearch.dirs {
		got = append(got, key)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("dirs = %v, want %v", got, want)
	}
}

// TestClearingTheQueryShowsTheWholeTree pins that an emptied phrase is no
// search at all: the tree shows every file again.
func TestClearingTheQueryShowsTheWholeTree(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes.md", "# Notes\n")
	s.Refresh()

	openTreeSearchOn(s, "n")
	if len(s.treeSearch.candidates) == 0 {
		t.Fatalf("candidates = %v, want the match before clearing", s.treeSearch.candidates)
	}
	s.setTreeQuery("")
	if s.searching() {
		t.Error("searching = true, want false once the phrase is empty")
	}
	if len(s.treeSearch.candidates) != 0 || len(s.treeSearch.dirs) != 0 {
		t.Errorf("candidates = %v, dirs = %v, want no search at all", s.treeSearch.candidates, s.treeSearch.dirs)
	}
}

// TestSearchingUsesTheTreeInMemory pins that typing searches the tree the
// window has, not the disk: a file removed behind the window's back is still
// found until the tree is reloaded.
func TestSearchingUsesTheTreeInMemory(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}

	openTreeSearchOn(s, "a")
	want := docSel{project: s.Projects()[0].ID, path: "a.md"}
	if !slices.Contains(s.treeSearch.candidates, want) {
		t.Errorf("candidates = %v, want a.md found in the tree in memory", s.treeSearch.candidates)
	}
}

// TestClosingAProjectDropsItsCandidates pins that a closed project takes its
// candidates with it: the tree no longer shows them, so the search must not
// keep offering them, or Enter would open a document of a project that is
// closed.
func TestClosingAProjectDropsItsCandidates(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "a_plan.md", "# A\n")
	writeDoc(t, betaRoot, "b_plan.md", "# B\n")
	s.Refresh()
	beta := s.Projects()[1].ID

	openTreeSearchOn(s, "plan")
	if len(s.treeSearch.candidates) != 2 {
		t.Fatalf("candidates = %v, want both projects' matches", s.treeSearch.candidates)
	}
	s.ToggleProject(beta)
	if len(s.treeSearch.candidates) != 1 {
		t.Fatalf("candidates = %v, want beta's match dropped", s.treeSearch.candidates)
	}
	if got := s.treeSearch.candidates[0].project; got == beta {
		t.Errorf("candidate project = %q, want alpha's", got)
	}
}

// TestCandidatesClampTheCurrent pins the index of the candidate the keyboard is
// on: a reload that leaves fewer candidates keeps it inside them, so that
// nothing reads past the end.
func TestCandidatesClampTheCurrent(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "p1_plan.md", "# One\n")
	writeDoc(t, root, "p2_plan.md", "# Two\n")
	s.Refresh()

	openTreeSearchOn(s, "plan")
	s.treeSearch.current = 1
	if err := os.Remove(filepath.Join(root, "p2_plan.md")); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if len(s.treeSearch.candidates) != 1 {
		t.Fatalf("candidates = %v, want one left", s.treeSearch.candidates)
	}
	if s.treeSearch.current != 0 {
		t.Errorf("current = %d, want 0: the candidate it named is gone", s.treeSearch.current)
	}
}

// TestSearchingTheTreeShowsOnlyMatchingFiles pins the filtered tree: while a
// phrase is typed the sidebar shows the files it matches and the directories
// above them, and nothing else.
func TestSearchingTheTreeShowsOnlyMatchingFiles(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/PLAN.md", "# Plan\n")
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	writeDoc(t, root, "README.md", "# Readme\n")
	s.Refresh()
	openTreeSearchOn(s, "plan")

	tt := newTester(s)
	if !tt.HasText("PLAN.md") {
		t.Fatalf("texts = %q, want the matched file", tt.Texts())
	}
	for _, gone := range []string{"notes.md", "README.md"} {
		if tt.HasText(gone) {
			t.Errorf("texts = %q, want %q left out of the search", tt.Texts(), gone)
		}
	}
	if !tt.HasText("docs") {
		t.Errorf("texts = %q, want the directory holding the match", tt.Texts())
	}
}

// TestCandidatesAndTheOneChosenAreTwoDifferentFills pins the two washes the
// search puts on the tree: every candidate is filled, the one the keyboard is on
// is filled more, and the two are not the same fill.
func TestCandidatesAndTheOneChosenAreTwoDifferentFills(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md", "other.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")
	s.treeSearch.current = 1

	id := s.Projects()[0].ID
	switch {
	case s.treeRowFill(id, "a_plan.md") != rowCandidate:
		t.Errorf("fill of a_plan.md = %v, want rowCandidate", s.treeRowFill(id, "a_plan.md"))
	case s.treeRowFill(id, "b_plan.md") != rowCurrent:
		t.Errorf("fill of b_plan.md = %v, want rowCurrent", s.treeRowFill(id, "b_plan.md"))
	case s.treeRowFill(id, "other.md") != rowNone:
		t.Errorf("fill of other.md = %v, want rowNone", s.treeRowFill(id, "other.md"))
	}

	same := true
	_ = ui.NewTester(func(c *ui.Context) {
		same = candidateRowTint(c.Theme()) == currentCandidateTint(c.Theme())
	}, 100, 100)
	if same {
		t.Error("the two fills are the same colour, want the chosen one told apart")
	}
}

// TestSteppingTheCurrentCandidateScrollsTheTree pins the locating: the tree
// slides so that the candidate chosen shows, even when it is far below the fold.
func TestSteppingTheCurrentCandidateScrollsTheTree(t *testing.T) {
	s, root := newState(t)
	for i := range 40 {
		writeDoc(t, root, "plan_"+strconv.Itoa(i)+".md", "# P\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")
	s.treeSearch.current = len(s.treeSearch.candidates) - 1
	s.treeSearch.reveal = true

	tt := ui.NewTester(s.View, 1000, 300)
	tt.Frame()
	tt.Frame()
	if s.treeScroll.Y <= 0 {
		t.Errorf("treeScroll.Y = %v, want the tree scrolled to the candidate", s.treeScroll.Y)
	}
}

// TestTheSearchRowCountsItsCandidates pins the count beside the field, and what
// a phrase matching nothing shows.
func TestTheSearchRowCountsItsCandidates(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")

	tt := newTester(s)
	if !tt.HasText("1/3") {
		t.Errorf("texts = %q, want the count 1/3", tt.Texts())
	}
	s.setTreeQuery("zzz")
	tt = newTester(s)
	if !tt.HasText("无匹配") {
		t.Errorf("texts = %q, want 无匹配", tt.Texts())
	}
	if !tt.HasText("没有匹配的文件") {
		t.Errorf("texts = %q, want the empty search named in the tree", tt.Texts())
	}
}

// TestTheTreeScrollerIsNamed pins the tree's scrolling pane: a name for the
// reader, and the room it takes for the tests that measure it.
func TestTheTreeScrollerIsNamed(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()

	tt := newTester(s)
	r, ok := tt.Find("项目树")
	if !ok {
		t.Fatalf("no named tree scroller; texts = %q", tt.Texts())
	}
	if r.W < 200 {
		t.Errorf("tree width = %v, want the sidebar's own width", r.W)
	}
	if r.Y <= 0 {
		t.Errorf("tree Y = %v, want it below the header", r.Y)
	}
}
