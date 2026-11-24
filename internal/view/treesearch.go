package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

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

// treeRowFill is how a row of the tree reads while the search is open.
type treeRowFill int

const (
	rowNone treeRowFill = iota
	rowCandidate
	rowCurrent
)

// treeRowFill is how the row of the file at path reads: the candidate the
// keyboard is on, a candidate the phrase matched, or nothing at all.
func (s *State) treeRowFill(projectID, path string) treeRowFill {
	if !s.searching() {
		return rowNone
	}
	switch i := s.candidateIndex(projectID, path); {
	case i < 0:
		return rowNone
	case i == s.treeSearch.current:
		return rowCurrent
	}
	return rowCandidate
}

// gotoCandidate makes the candidate at i the one the keyboard is on, and asks
// the tree to bring it into view.
func (s *State) gotoCandidate(i int) {
	if i < 0 || i >= len(s.treeSearch.candidates) {
		return
	}
	s.treeSearch.current, s.treeSearch.reveal = i, true
}

// openCandidate shows the candidate the keyboard is on. The search stays open:
// stepping through the files of a project is what it is for.
func (s *State) openCandidate() {
	if len(s.treeSearch.candidates) == 0 {
		return
	}
	cell := s.treeSearch.candidates[min(s.treeSearch.current, len(s.treeSearch.candidates)-1)]
	s.selectDocIn(cell.project, cell.path)
}

// treeSearchCount is what the row beside the field says: which candidate of how
// many shows, or that the phrase matches nothing.
func (s *State) treeSearchCount() string {
	if !s.searching() {
		return ""
	}
	if len(s.treeSearch.candidates) == 0 {
		return "无匹配"
	}
	return strconv.Itoa(s.treeSearch.current+1) + "/" + strconv.Itoa(len(s.treeSearch.candidates))
}

// treeSearchRow builds the row above the tree: the field searching file names,
// how many candidates it has, and the button closing it.
func (s *State) treeSearchRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0.5)).
		Padding(t.Space(0.5), t.Space(1)).Children(func() {
		field := ui.SearchField(c, &s.treeSearch.query).Label("在项目树中查找")
		field.Grow(1).FontSize(12)
		if s.treeSearch.needFocus {
			// AutoFocus gives the focus once, where Focus would take it back
			// every frame from the buttons beside the field.
			field.AutoFocus()
			s.treeSearch.needFocus = false
		}
		if field.Changed() {
			s.setTreeQuery(s.treeSearch.query)
		}
		components.Text(c, s.treeSearchCount()).FontSize(11).TextColor(t.TextMuted)
		if components.IconButton(c, lucide.X(), "关闭树搜索", components.IconButtonCompact).Clicked() {
			s.closeTreeSearch()
		}
	})
}

// treeMatched builds the tree while the phrase is typed: the project nodes, the
// directories holding a candidate, and the candidates themselves.
func (s *State) treeMatched(c *ui.Context) {
	multi := len(s.trees) > 1
	for _, tr := range s.trees {
		tr := tr
		if !s.projectHasCandidate(tr.project.ID) {
			continue
		}
		if !multi {
			s.matchedItems(c, tr, tr.root)
			continue
		}
		open := true
		ui.TreeItem(c, tr.project.Name, &open, func() { s.matchedItems(c, tr, tr.root) })
	}
}

// projectHasCandidate reports whether any candidate belongs to projectID, so
// that a project the phrase does not reach leaves no empty node behind.
func (s *State) projectHasCandidate(projectID string) bool {
	for _, cell := range s.treeSearch.candidates {
		if cell.project == projectID {
			return true
		}
	}
	return false
}

// matchedItems builds the directories of n that hold a candidate and the
// candidates themselves. The directories are unfolded for the length of the
// search with a flag of this build alone, never with s.open: closing the search
// must show the tree as the user folded it.
func (s *State) matchedItems(c *ui.Context, tr projectTree, n *workspace.Node) {
	if n == nil {
		return
	}
	for _, child := range n.Children {
		child := child
		if child.IsDir {
			if !s.treeSearch.dirs[tr.project.ID+":"+child.Path] {
				continue
			}
			open := true
			ui.TreeItem(c, child.Name, &open, func() { s.matchedItems(c, tr, child) })
			continue
		}
		if i := s.candidateIndex(tr.project.ID, child.Path); i >= 0 {
			s.matchedFile(c, tr, child, i)
		}
	}
}

// matchedFile builds one candidate: the fill saying the phrase matched it, the
// stronger one when it is the candidate the keyboard is on, which the tree also
// slides to when the search asks. A click opens it.
func (s *State) matchedFile(c *ui.Context, tr projectTree, n *workspace.Node, i int) {
	t := c.Theme()
	fill := s.treeRowFill(tr.project.ID, n.Path)
	item := ui.TreeItem(c, n.Name, nil, nil)
	switch fill {
	case rowCurrent:
		item.Background(currentCandidateTint(t))
	case rowCandidate:
		item.Background(candidateRowTint(t))
	}
	if fill == rowCurrent && s.treeSearch.reveal {
		// Kept for the rest of this frame, dropped on the next one: a click
		// builds the view twice, and the request must outlive the first build.
		s.treeSearch.revealSeen = c.Now()
		item.ScrollIntoView()
	}
	if item.Clicked() {
		s.gotoCandidate(i)
		s.openCandidate()
	}
}
