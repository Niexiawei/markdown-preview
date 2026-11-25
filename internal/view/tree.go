package view

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"mdpreview/internal/workspace"
)

// treeKey names a row of the sidebar's tree: the project, and the path in it.
// The path is "" for a project's own node.
type treeKey struct {
	project string
	path    string
}

// treeBody builds the tree itself: the entries of the window's one project, or
// a node of each when it shows several, and under them what is open. It is an
// Outline, so the window builds only the rows it shows, however many entries
// the projects have.
func (s *State) treeBody(c *ui.Context) {
	if s.searching() && s.treeSearch.reveal {
		s.revealCandidate(c)
	}
	ui.Outline(c, &s.treeOpen, s.roots(), s.children, func(k treeKey) { s.treeRow(c, k) }).
		Grow(1).Label("项目树").ObserveInput(func(ev ui.InputEvent) { s.treePress(ev) })
}

// revealCandidate slides the tree to the candidate the keyboard is on, for the
// rest of the frame that asked: a click builds the view twice, and the ask must
// outlive the first build (as State.jump does).
func (s *State) revealCandidate(c *ui.Context) {
	if len(s.treeSearch.candidates) == 0 {
		return
	}
	cell := s.treeSearch.candidates[min(s.treeSearch.current, len(s.treeSearch.candidates)-1)]
	if row := s.rowOf(s.roots(), treeKey{project: cell.project, path: cell.path}); row >= 0 {
		s.treeOpen.List.ScrollIntoView(row)
		s.treeSearch.revealSeen = c.Now()
	}
}

// roots are the tree's top level: the entries of the window's one project, or
// a node of each when it shows several.
func (s *State) roots() []treeKey {
	if len(s.trees) == 1 {
		if tr := s.trees[0]; tr.project.File != "" {
			// A project of one document is that document's row: a node above
			// it would show the same name twice.
			return []treeKey{{project: tr.project.ID, path: tr.project.Name}}
		}
		return s.children(treeKey{project: s.trees[0].project.ID})
	}
	roots := make([]treeKey, 0, len(s.trees))
	for _, tr := range s.trees {
		if s.searching() && !s.projectHasCandidate(tr.project.ID) {
			// The phrase reaches nothing in this project: no empty node.
			continue
		}
		key := treeKey{project: tr.project.ID}
		if tr.project.File != "" {
			key.path = tr.project.Name
		}
		roots = append(roots, key)
	}
	return roots
}

// children are the entries of an item of the tree, a project's or a
// directory's: nil for a document, and for an item the tree does not have.
func (s *State) children(k treeKey) []treeKey {
	n, ok := s.treeNode(k)
	if !ok || !n.IsDir {
		return nil
	}
	kids := make([]treeKey, 0, len(n.Children))
	for _, child := range n.Children {
		key := treeKey{project: k.project, path: child.Path}
		if s.searching() {
			// The search shows what it matches, and the directories above
			// what it matches.
			if child.IsDir {
				if !s.treeSearch.dirs[key] {
					continue
				}
			} else if s.candidateIndex(k.project, child.Path) < 0 {
				continue
			}
		}
		kids = append(kids, key)
	}
	return kids
}

// treeNode returns the entry a row names: the project's own node, or the entry
// at its path.
func (s *State) treeNode(k treeKey) (*workspace.Node, bool) {
	tr, ok := s.treeOf(k.project)
	if !ok || tr.root == nil {
		return nil, false
	}
	if k.path == "" || (!tr.root.IsDir && tr.root.Path == k.path) {
		return tr.root, true
	}
	return findNode(tr.root, k.path)
}

// findNode returns the entry of n at path, if the tree has it.
func findNode(n *workspace.Node, path string) (*workspace.Node, bool) {
	rest := path
	for {
		name, tail, _ := strings.Cut(rest, "/")
		found := childNamed(n, name)
		if found == nil {
			return nil, false
		}
		if tail == "" {
			return found, true
		}
		n, rest = found, tail
	}
}

// rowOf is the row k shows on, -1 when the tree does not show it. It walks the
// open items itself rather than reading Rows(), which is the last frame's: a
// click opens the directories above a row and scrolls to it in one frame, and
// the row it scrolls to must be the one this frame builds.
func (s *State) rowOf(roots []treeKey, want treeKey) int {
	type level struct {
		items []treeKey
		next  int
	}
	row := 0
	stack := []level{{items: roots}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next >= len(top.items) {
			stack = stack[:len(stack)-1]
			continue
		}
		k := top.items[top.next]
		top.next++
		if k == want {
			return row
		}
		row++
		if s.treeOpen.Open.Has(k) {
			if kids := s.children(k); len(kids) > 0 {
				stack = append(stack, level{items: kids})
			}
		}
	}
	return -1
}

// treeRow builds one row of the tree: a project's node, a directory, or a
// document. The outline builds its indentation and arrow; what shows beside
// them, and what a click does, is here.
func (s *State) treeRow(c *ui.Context, k treeKey) {
	tr, ok := s.treeOf(k.project)
	if !ok {
		return
	}
	if k.path == "" || tr.root == nil {
		// The project's own node, and the row a project shows while the window
		// has not read it yet.
		s.projectRow(c, k, tr)
		return
	}
	n, ok := s.treeNode(k)
	if !ok {
		return
	}
	t := c.Theme()
	row := s.treeRowBox(c)
	switch {
	case s.isCurrentCandidate(k.project, k.path):
		// The one row that reads differently: everything else in the tree is
		// a match already.
		row.Background(currentCandidateTint(t))
	case !s.searching() && k.project == s.sel.project && k.path == s.sel.path:
		// The document shown, as the theme paints the row of a list chosen.
		row.Background(t.Accent)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	name := n.Name
	if k.project == s.sel.project && k.path == s.sel.path && !s.searching() {
		row.Children(func() {
			if !n.IsDir {
				components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.AccentText)
			}
			components.Text(c, name).SingleLine().Grow(1).TextColor(t.AccentText)
		})
	} else {
		row.Children(func() {
			if s.isLooseFile(k) {
				// A document of its own reads as one, where a project's node
				// reads as a directory.
				components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
			}
			components.Text(c, name).SingleLine().Grow(1)
		})
	}
	s.nodeMenu(c, row, tr, n)
	if n.IsDir {
		row.OnClick(func() { s.toggleTreeKey(k) })
		return
	}
	row.OnClick(func() {
		if i := s.candidateIndex(k.project, k.path); i >= 0 {
			s.gotoCandidate(i)
		}
		s.selectDocIn(k.project, k.path)
	})
}

// treeRowBox is the box of one row of the tree, sized and rounded as the theme
// paints an item of a tree: hover and the accent of the documet shown are what
// its rows read by.
func (s *State) treeRowBox(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).
		Gap(t.Space(0.5)).Padding(0, t.Space(1)).Radius(t.Radius)
}

// projectRow builds the row of a project: its name, the loading mark while the
// window reads it, and the actions of that project alone.
func (s *State) projectRow(c *ui.Context, k treeKey, tr projectTree) {
	t := c.Theme()
	row := s.treeRowBox(c)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		if tr.project.File == "" {
			components.Icon(c, lucide.Folder()).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
		} else {
			components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
		}
		components.Text(c, tr.project.Name).SingleLine().Grow(1)
		if s.scanning[tr.project.ID] {
			components.Spinner(c, components.SpinnerSpokes).Label("正在扫描 " + tr.project.Name)
		}
	})
	s.projectActions(c, row, tr)
	s.projectMenu(c, row, tr)
	row.OnClick(func() { s.toggleTreeKey(k) })
}

// isLooseFile reports whether the row is a document of its own, rather than an
// entry of a project's tree.
func (s *State) isLooseFile(k treeKey) bool {
	tr, ok := s.treeOf(k.project)
	return ok && tr.project.File != "" && !tr.rootIsDir()
}

// rootIsDir reports whether the project's tree has a directory at its top, as
// a directory project does.
func (tr projectTree) rootIsDir() bool { return tr.root == nil || tr.root.IsDir }

// setTreeKeyOpen opens or closes the children of a row, as a click on its arrow
// does.
func (s *State) setTreeKeyOpen(k treeKey, open bool) {
	if open {
		s.treeOpen.Open.Add(k)
		return
	}
	s.treeOpen.Open.Remove(k)
}

// toggleTreeKey opens the children of a row, or closes them.
func (s *State) toggleTreeKey(k treeKey) {
	s.setTreeKeyOpen(k, !s.treeOpen.Open.Has(k))
}

// setTreeOpen folds or unfolds every directory of the open projects, or of the
// project with projectID alone ("" for all of them). Folding closes the
// directories and leaves the project nodes as the user left them — a folded
// project still shows its top level, as a window of one project does; unfolding
// opens the nodes too, so that nothing stays hidden after it.
func (s *State) setTreeOpen(projectID string, open bool) {
	for _, tr := range s.trees {
		if projectID != "" && tr.project.ID != projectID {
			continue
		}
		if open && len(s.trees) > 1 {
			s.setTreeKeyOpen(treeKey{project: tr.project.ID}, true)
		}
		var walk func(n *workspace.Node)
		walk = func(n *workspace.Node) {
			for _, child := range n.Children {
				if !child.IsDir {
					continue
				}
				s.setTreeKeyOpen(treeKey{project: tr.project.ID, path: child.Path}, open)
				walk(child)
			}
		}
		if tr.root != nil {
			walk(tr.root)
		}
	}
	s.bumpTree()
}

// noteNewDirectories opens the directories the window has not shown before: the
// ones a project read for the first time brought, and the ones a file event
// added since. A directory the user folded is not new, so it stays folded.
func (s *State) noteNewDirectories() {
	for _, tr := range s.trees {
		s.noteNew(treeKey{project: tr.project.ID})
		var walk func(n *workspace.Node)
		walk = func(n *workspace.Node) {
			for _, child := range n.Children {
				if !child.IsDir {
					continue
				}
				s.noteNew(treeKey{project: tr.project.ID, path: child.Path})
				walk(child)
			}
		}
		if tr.root != nil {
			walk(tr.root)
		}
	}
}

// noteNew opens the row of the tree the window has not shown yet.
func (s *State) noteNew(k treeKey) {
	if s.treeSeen[k] {
		return
	}
	s.treeSeen[k] = true
	s.treeOpen.Open.Add(k)
}
