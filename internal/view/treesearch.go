package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"mdpreview/internal/config"
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
	// shows them, and index names where each of them sits: the tree asks about
	// every file it builds, so the lookup cannot be a walk of the candidates.
	candidates []docSel
	index      map[docSel]int
	// dirs names the directories holding a candidate: a search shows those,
	// the candidates, and the project nodes above them.
	dirs map[treeKey]bool
	// opened are the rows the search opened to show its candidates, so that
	// closing it shows the tree as the user folded it.
	opened []treeKey
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
	t.candidates, t.dirs, t.index = nil, map[treeKey]bool{}, map[docSel]int{}
	if !s.searching() {
		t.current = 0
		s.closeSearchOpen()
		return
	}
	for _, tr := range s.trees {
		s.matchTree(tr, tr.root)
	}
	t.current = min(t.current, max(0, len(t.candidates)-1))
	s.openSearchDirs()
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
				s.treeSearch.dirs[treeKey{project: tr.project.ID, path: child.Path}] = true
				found = true
			}
			continue
		}
		if !workspace.FuzzyMatch(child.Name, s.treeSearch.query) {
			continue
		}
		cell := docSel{project: tr.project.ID, path: child.Path}
		s.treeSearch.index[cell] = len(s.treeSearch.candidates)
		s.treeSearch.candidates = append(s.treeSearch.candidates, cell)
		found = true
	}
	return found
}

// openTreeSearch shows the tree's search, over the phrase searched before:
// searching for the same name again is the common case. It takes the keyboard,
// and the document's search folds away: one panel at a time holds the keys.
func (s *State) openTreeSearch() {
	s.treeSearch.open = true
	s.treeSearch.needFocus = true
	s.treeSearch.current = 0
	s.treeFocused = true
	s.closeFind()
	s.refreshCandidates()
}

// searchShortcuts handles the keys the two searches share: Cmd+F searches the
// file names when the sidebar has the focus and the document when it has not,
// and Cmd+Shift+F searches the file names wherever the focus is.
//
// The two are registered apart: a shortcut matches the modifiers held exactly,
// so Cmd+F's registration never sees the key while Shift is down.
func (s *State) searchShortcuts(c *ui.Context) {
	switch {
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF):
		s.openTreeSearch()
		return
	case c.Shortcut(ui.Cmd, ui.KeyF):
		// A press in the page never reaches an element, so the focus cannot say
		// which pane was used: while the page shows, the key searches the
		// document, and Shift+Cmd+F searches the file names.
		if s.treeFocused && s.mode() != config.RenderWebView {
			s.openTreeSearch()
			return
		}
		s.openFind()
		return
	case s.treeSearch.open && c.Shortcut(0, ui.KeyUp):
		// The arrows walk the candidates rather than the field's caret: the
		// window's shortcut takes them before the field's editor does, while the
		// field holds the keyboard.
		s.moveCandidate(-1)
		return
	case s.treeSearch.open && c.Shortcut(0, ui.KeyDown):
		s.moveCandidate(1)
		return
	case s.treeSearch.open && c.Shortcut(0, ui.KeyEscape):
		// The phrase first, as the document's search does: clearing it shows the
		// whole tree again, and the row itself goes on the next press.
		if s.treeSearch.query != "" {
			s.setTreeQuery("")
			return
		}
		s.closeTreeSearch()
		return
	}
	s.findShortcuts(c)
}

// closeTreeSearch hides the tree's search, and shows every file again. The
// phrase is kept, as the document's search keeps its own.
func (s *State) closeTreeSearch() {
	s.treeSearch.open = false
	s.closeSearchOpen()
}

// openSearchDirs unfolds the rows holding a candidate, for the length of the
// search: what it unfolds is remembered, so that closing it folds them back.
func (s *State) openSearchDirs() {
	for k := range s.treeSearch.dirs {
		s.openForSearch(k)
	}
	if len(s.trees) > 1 {
		for _, tr := range s.trees {
			if s.projectHasCandidate(tr.project.ID) {
				s.openForSearch(treeKey{project: tr.project.ID})
			}
		}
	}
	s.treeOpen.Generation++
}

// openForSearch unfolds one row for the length of the search, remembering it
// if the user had it folded.
func (s *State) openForSearch(k treeKey) {
	if !s.treeOpen.Open.Has(k) {
		s.treeSearch.opened = append(s.treeSearch.opened, k)
	}
	s.treeOpen.Open.Add(k)
}

// closeSearchOpen folds back the rows the search unfolded.
func (s *State) closeSearchOpen() {
	if len(s.treeSearch.opened) == 0 {
		return
	}
	for _, k := range s.treeSearch.opened {
		s.treeOpen.Open.Remove(k)
	}
	s.treeSearch.opened = nil
	s.treeOpen.Generation++
}

// candidateIndex is where a file of the tree sits among the candidates, or -1
// when the search does not offer it.
func (s *State) candidateIndex(projectID, path string) int {
	if i, ok := s.treeSearch.index[docSel{project: projectID, path: path}]; ok {
		return i
	}
	return -1
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

// isCurrentCandidate reports whether the file is the candidate the keyboard is
// on: the one row the search fills, since the tree already shows nothing but
// matches.
func (s *State) isCurrentCandidate(projectID, path string) bool {
	return s.searching() && s.candidateIndex(projectID, path) == s.treeSearch.current
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
		field.Grow(1).FontSize(12).Placeholder("搜索文件名")
		if s.treeSearch.needFocus {
			// Asked for until the field has it: the tree below is a focus group
			// of its own, and takes the keyboard on the frame the search opens
			// when nothing else holds it, so a single AutoFocus loses the race.
			field.Focus()
			s.treeSearch.needFocus = !field.Focused()
		}
		switch {
		case field.Changed():
			s.setTreeQuery(s.treeSearch.query)
		case field.Submitted():
			s.openCandidate()
		}
		components.Text(c, s.treeSearchCount()).FontSize(11).TextColor(t.TextMuted)
		if components.IconButton(c, lucide.X(), "关闭树搜索", components.IconButtonCompact).Clicked() {
			s.closeTreeSearch()
		}
	})
}

// treePress is the sidebar's pane seeing the pointer: a press there is what
// makes Cmd+F search the file names.
func (s *State) treePress(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputPointerDown {
		s.treeFocused = true
	}
	return false
}

// bodyPress is the body's pane seeing the pointer: a press there is what makes
// Cmd+F search the document again.
func (s *State) bodyPress(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputPointerDown {
		s.treeFocused = false
	}
	return false
}

// moveCandidate makes the candidate n places on the one the keyboard is on, round
// the ends, and asks the tree to bring it into view. It opens nothing: the
// documents are stepped through, and Enter or a click shows one.
func (s *State) moveCandidate(n int) {
	total := len(s.treeSearch.candidates)
	if total == 0 {
		return
	}
	s.gotoCandidate(((s.treeSearch.current+n)%total + total) % total)
}
