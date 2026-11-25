package view

import (
	"strings"

	"mdpreview/internal/workspace"
)

// locateDoc brings the document shown into the tree's view: the search folds
// away, the directories above it unfold, and the tree slides to its row.
func (s *State) locateDoc() {
	if s.sel.empty() {
		return
	}
	if s.treeSearch.open {
		// The search restored the folds it opened; what the user asked for
		// here is the whole tree, with the document in it.
		s.closeTreeSearch()
	}
	if !s.revealDocInTree(s.sel) {
		s.notifyWarn("项目树里没有这个文件：" + s.sel.path)
		return
	}
	// The ancestors it unfolded stay open from now on, so the fold memory the
	// restart restores changes with them: one write for the whole reveal.
	s.saveSession()
	if row := s.rowOf(s.roots(), treeKey{project: s.sel.project, path: s.sel.path}); row >= 0 {
		s.treeOpen.List.ScrollIntoView(row)
	}
}

// revealDocInTree unfolds what sits above the document sel in the tree. It
// reports whether the tree holds the document at all.
func (s *State) revealDocInTree(sel docSel) bool {
	tr, ok := s.treeOf(sel.project)
	if !ok || tr.root == nil {
		return false
	}
	if tr.project.File != "" {
		// A project of one document is that document's row.
		return sel.path == tr.project.Name || sel.path == tr.root.Path
	}
	if len(s.trees) > 1 {
		s.setTreeKeyOpen(treeKey{project: sel.project}, true)
	}
	node := tr.root
	parts := strings.Split(sel.path, "/")
	for i, part := range parts {
		child := childNamed(node, part)
		if child == nil {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		s.setTreeKeyOpen(treeKey{project: sel.project, path: child.Path}, true)
		node = child
	}
	return false
}

// childNamed returns n's entry of that name.
func childNamed(n *workspace.Node, name string) *workspace.Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}
