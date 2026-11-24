package view

import (
	"strings"
	"time"

	"mdpreview/internal/workspace"
)

// treeSearchState is the search over the file tree's names: whether it shows,
// the phrase, the files it matches, the directories those sit in, and the
// candidate the keyboard is on. The tree itself is the list of candidates —
// there is no panel beside it — so this state names what the tree keeps and
// which row reads as the one chosen.
type treeSearchState struct {
	open      bool
	needFocus bool
	query     string
	// candidates are the files the phrase matches, in the order the tree
	// shows them.
	candidates []docSel
	// dirs names the directories holding a candidate, by "projectID:path": a
	// search shows those, the candidates, and the project nodes above them.
	dirs map[string]bool
	// current is the index into candidates of the one the keyboard is on.
	current int
	// reveal asks the next build to bring the current candidate into view, and
	// revealSeen is when the frame that built it built it: a click builds the
	// view twice, and only a request kept for the rest of that frame survives it
	// (as with State.jump).
	reveal     bool
	revealSeen time.Time
}

// searching reports whether the tree shows what the phrase matches rather than
// the whole of every project. An empty phrase is no search: the tree shows
// everything.
func (s *State) searching() bool {
	return s.treeSearch.open && strings.TrimSpace(s.treeSearch.query) != ""
}

// setTreeQuery types a phrase into the tree's search. The candidates come from
// the trees the window holds, so nothing is read from the disk here.
func (s *State) setTreeQuery(q string) {
	s.treeSearch.query, s.treeSearch.current = q, 0
	s.refreshCandidates()
}

// refreshCandidates keeps the files of the open projects whose name the phrase
// matches, in the order the tree shows them, and marks the directories above
// them. It runs when the phrase changes and when the trees do — never per
// frame, and never with a scan of the disk.
func (s *State) refreshCandidates() {
	t := &s.treeSearch
	t.candidates, t.dirs = nil, map[string]bool{}
	if !s.searching() {
		t.current = 0
		return
	}
	for _, tr := range s.trees {
		s.matchTree(tr, tr.root)
	}
	t.current = min(t.current, max(0, len(t.candidates)-1))
}

// matchTree appends the candidates under n, and reports whether it found any,
// so that each directory above one learns it holds a candidate.
func (s *State) matchTree(tr projectTree, n *workspace.Node) bool {
	if n == nil {
		return false
	}
	found := false
	for _, child := range n.Children {
		if child.IsDir {
			if s.matchTree(tr, child) {
				s.treeSearch.dirs[tr.project.ID+":"+child.Path] = true
				found = true
			}
			continue
		}
		if !workspace.FuzzyMatch(child.Name, s.treeSearch.query) {
			continue
		}
		s.treeSearch.candidates = append(s.treeSearch.candidates,
			docSel{project: tr.project.ID, path: child.Path})
		found = true
	}
	return found
}

// openTreeSearch shows the tree's search, over the phrase searched before:
// searching for the same name again is the common case.
func (s *State) openTreeSearch() {
	s.treeSearch.open = true
	s.treeSearch.needFocus = true
	s.treeSearch.current = 0
	s.refreshCandidates()
}

// closeTreeSearch hides the tree's search, and shows every file again. The
// phrase is kept, as the document's search keeps its own.
func (s *State) closeTreeSearch() { s.treeSearch.open = false }

// candidateIndex is where a file of the tree sits among the candidates, or -1
// when the search does not offer it.
func (s *State) candidateIndex(projectID, path string) int {
	for i, c := range s.treeSearch.candidates {
		if c.project == projectID && c.path == path {
			return i
		}
	}
	return -1
}
