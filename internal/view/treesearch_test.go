package view

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
