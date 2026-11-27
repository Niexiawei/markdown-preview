package view

import "github.com/egoist/mygo/ui"

// The sidebar's picked rows: what Cmd+C copies, and what a context menu opened
// on one of them acts on. Picking is apart from showing — the document shown
// keeps its own accent fill — and it is not saved: it is what the user is
// pointing at now, not part of the window's session.
//
// A row is named by its treeKey rather than by its absolute path: a scan, a
// fold or a rename keeps the key, and a key the tree no longer has is dropped
// by pruneTreeSel.

// multiClick is the modifier that picks a row instead of acting on it: Control
// everywhere, and Command too on macOS, where ui.Cmd is Super. It is a
// variable, not a constant: ui.Cmd is decided at start, from the platform.
var multiClick = ui.Ctrl | ui.Cmd

// selectRow is what a click on a tree row does to the picked rows. A plain
// click leaves the row as the only one picked; the row's own action runs after
// it, in the row's handler. A click with the modifier adds the row, or takes it
// out again, and does nothing else: folding a directory or opening a document
// while the user is picking several rows would undo the pick.
func (s *State) selectRow(k treeKey, mods ui.Modifiers) {
	if mods&multiClick == 0 {
		s.treeSel = []treeKey{k}
		return
	}
	for i, picked := range s.treeSel {
		if picked != k {
			continue
		}
		s.treeSel = append(s.treeSel[:i], s.treeSel[i+1:]...)
		return
	}
	s.treeSel = append(s.treeSel, k)
}

// isRowSelected reports whether the row is one of the picked ones.
func (s *State) isRowSelected(k treeKey) bool {
	for _, picked := range s.treeSel {
		if picked == k {
			return true
		}
	}
	return false
}

// clearTreeSel forgets every picked row.
func (s *State) clearTreeSel() { s.treeSel = nil }

// syncTreeSelToDoc makes the document's row the picked one: showing a document
// is picking it, so that a later Cmd+C copies what the user just opened rather
// than the folder picked before it.
func (s *State) syncTreeSelToDoc(sel docSel) {
	if sel.empty() {
		s.clearTreeSel()
		return
	}
	s.treeSel = []treeKey{{project: sel.project, path: sel.path}}
}

// pruneTreeSel drops the picked rows the tree no longer has: a file removed
// under a scan cannot be copied. A project still being read keeps its node
// picked — it is there on screen, with the loading mark after its name.
func (s *State) pruneTreeSel() {
	kept := s.treeSel[:0]
	for _, k := range s.treeSel {
		if _, ok := s.treeOf(k.project); !ok {
			continue
		}
		if k.path != "" {
			if _, ok := s.treeNode(k); !ok {
				continue
			}
		}
		kept = append(kept, k)
	}
	s.treeSel = kept
}

// dropTreeSelOf drops the picked rows of a project the window closed or
// removed.
func (s *State) dropTreeSelOf(projectID string) {
	kept := s.treeSel[:0]
	for _, k := range s.treeSel {
		if k.project != projectID {
			kept = append(kept, k)
		}
	}
	s.treeSel = kept
}
