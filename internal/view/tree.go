package view

import (
	"strings"

	"github.com/egoist/mygo/plugins/loading"
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
	// An empty prefix takes the outline's indentation, arrow and padding away:
	// the row paints the whole width itself, so that the fill of the row
	// hovered or shown reaches both edges, as an item of a tree does.
	s.treeOpen.Prefix = func(ui.OutlineRow) {}
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
// document, on the fill of the row shown or hovered. The outline gives rows no
// padding and no arrow (treeBody sets an empty Prefix), so the row's own box
// paints the whole width, and the arrow — or the document's icon — sits in the
// box the arrows of a tree sit in.
func (s *State) treeRow(c *ui.Context, k treeKey) {
	tr, ok := s.treeOf(k.project)
	if !ok {
		return
	}
	t := c.Theme()
	project := k.path == "" || tr.root == nil
	var n *workspace.Node
	if !project {
		if n, ok = s.treeNode(k); !ok {
			return
		}
	}
	row := ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).
		Padding(0, t.Space(1)).Radius(t.Radius)
	shown := !s.searching() && k.path != "" && k.project == s.sel.project && k.path == s.sel.path
	switch {
	case !project && s.isCurrentCandidate(k.project, k.path):
		// The one row that reads differently while the search is open.
		row.Background(currentCandidateTint(t))
	case shown:
		// The document shown, as the theme paints the row of a list chosen.
		row.Background(t.Accent)
	case s.isRowSelected(k):
		// The rows picked for Cmd+C: the theme's wash, under the accent of the
		// document shown, so that the two never read as the same thing. The
		// fill stops short of the row's edges, so that several picked rows read
		// as rows rather than as one tall block.
		row.Draw(func(p *ui.Painter, r ui.Rect) {
			inset := t.Space(0.5)
			p.Fill(ui.Rect{X: r.X, Y: r.Y + inset, W: r.W, H: r.H - 2*inset}, t.Selection, t.Radius)
		})
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	text, mark := t.Text, t.TextMuted
	if shown {
		text, mark = t.AccentText, t.AccentText
	}
	name := tr.project.Name
	if !project {
		name = n.Name
	}
	// A project's node opens like a directory, and a document has no entries:
	// the arrow of one and the icon of the other share the box.
	branch := len(s.children(k)) > 0
	row.Children(func() {
		if depth := s.treeDepth(k); depth > 0 {
			ui.Box(c).Width(float32(depth) * t.Space(4)).Shrink(0)
		}
		s.treeMark(c, k, n, branch, mark)
		ui.Box(c).Width(t.Space(0.5)).Shrink(0)
		if project && tr.project.File == "" {
			components.Icon(c, lucide.Folder()).Size(t.Rem(1.25), t.Rem(1.25)).
				AlignSelf(ui.Center).MarginEnd(t.Space(1)).TextColor(mark)
		}
		// The name does not grow: what follows it — the loading mark while the
		// window reads the project — sits right after the name, and the rest of
		// the row is the spacer's.
		components.Text(c, name).SingleLine().TextColor(text)
		if project && s.scanning[tr.project.ID] {
			loading.Snake(c, loading.Options{Size: loading.DefaultSize, Color: t.TextMuted}).
				MarginStart(t.Space(0.5)).Label("正在扫描 " + tr.project.Name)
		}
		ui.Spacer(c)
	})
	if project {
		s.projectActions(c, row, tr)
		s.projectMenu(c, row, tr)
		row.OnClick(func() {
			if s.pickRow(k, row) {
				return
			}
			s.toggleTreeKey(k)
		})
		return
	}
	s.nodeMenu(c, row, tr, n)
	if n.IsDir {
		row.OnClick(func() {
			if s.pickRow(k, row) {
				return
			}
			s.toggleTreeKey(k)
		})
		return
	}
	row.OnClick(func() {
		if s.pickRow(k, row) {
			return
		}
		if i := s.candidateIndex(k.project, k.path); i >= 0 {
			s.gotoCandidate(i)
		}
		s.selectDocIn(k.project, k.path)
	})
}

// treeMark is the box before a row's name, where a tree keeps its arrows: the
// arrow of a directory, and the icon of a document, so that the two line up.
func (s *State) treeMark(c *ui.Context, k treeKey, n *workspace.Node, branch bool, color ui.Color) {
	t := c.Theme()
	box := ui.Box(c).Size(t.Space(4), t.Space(4)).Shrink(0).Role(ui.RoleNone)
	switch {
	case branch:
		open := s.treeOpen.Open.Has(k)
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			var path ui.Path
			if open {
				path.MoveTo(r.X+r.W/4, r.Y+r.H*3/8).LineTo(r.X+r.W/2, r.Y+r.H*5/8).
					LineTo(r.X+r.W*3/4, r.Y+r.H*3/8)
			} else {
				path.MoveTo(r.X+r.W*3/8, r.Y+r.H/4).LineTo(r.X+r.W*5/8, r.Y+r.H/2).
					LineTo(r.X+r.W*3/8, r.Y+r.H*3/4)
			}
			p.StrokePath(&path, 1.5, color)
		})
		box.OnClick(func() {
			if s.pickRow(k, box) {
				return
			}
			s.toggleTreeKey(k)
		})
	case n != nil && !n.IsDir:
		components.Icon(c, lucide.FileText()).Size(t.Space(4), t.Space(4)).TextColor(color)
	}
}

// treeDepth is how deep a row sits, for its indentation: a project's node and a
// document of its own sit at the top, and an entry a level for each directory
// above it, its own included.
func (s *State) treeDepth(k treeKey) int {
	tr, ok := s.treeOf(k.project)
	if !ok || k.path == "" || tr.project.File != "" {
		return 0
	}
	return strings.Count(k.path, "/") + 1
}

// setTreeKeyOpen opens or closes the children of a row, as a click on its arrow
// does. Closing a row the user chose to fold puts it in the set a restart
// restores; opening it takes it back out. It never saves: the entries that
// call it in a loop save once, at their own end.
func (s *State) setTreeKeyOpen(k treeKey, open bool) {
	if open {
		s.treeOpen.Open.Add(k)
		delete(s.treeClosed, k)
		return
	}
	s.treeOpen.Open.Remove(k)
	s.treeClosed[k] = true
}

// toggleTreeKey opens the children of a row, or closes them. It is one click,
// so the fold it leaves goes into the snapshot once, here.
func (s *State) toggleTreeKey(k treeKey) {
	s.setTreeKeyOpen(k, !s.treeOpen.Open.Has(k))
	s.saveSession()
}

// setTreeOpen folds or unfolds every directory of the open projects, or of the
// project with projectID alone ("" for all of them). With several projects open
// the node of each folds with them, so that folding everything leaves the
// projects themselves closed — as a window of one project, which has no node,
// folds down to its top level.
func (s *State) setTreeOpen(projectID string, open bool) {
	for _, tr := range s.trees {
		if projectID != "" && tr.project.ID != projectID {
			continue
		}
		if len(s.trees) > 1 {
			s.setTreeKeyOpen(treeKey{project: tr.project.ID}, open)
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
	// One write at the end, not one per row: folding a directory of thousands
	// of entries must not write the snapshot thousands of times.
	s.saveSession()
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
