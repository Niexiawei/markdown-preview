package view

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

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

// pickRow picks the row a click hit, and reports whether the click carried the
// multi-select modifier: the row's own action runs only when it did not.
func (s *State) pickRow(k treeKey, el ui.Element) bool {
	mods := el.ClickModifiers()
	s.selectRow(k, mods)
	return mods&multiClick != 0
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

// absOfKey is a row's absolute path on disk: the project's own directory for a
// project node, the file itself for a document.
func (s *State) absOfKey(k treeKey) (string, error) { return s.ws.AbsOf(k.project, k.path) }

// copyTreeSelection puts the picked rows' absolute paths on the clipboard, one
// per line, and reports whether there was a row to copy: Cmd+C falls back to
// the document shown when there was none.
func (s *State) copyTreeSelection() bool {
	if s.sys == nil || len(s.treeSel) == 0 {
		return false
	}
	paths := make([]string, 0, len(s.treeSel))
	for _, k := range s.treeSel {
		abs, err := s.absOfKey(k)
		if err != nil {
			s.notifyErr("无法复制文件路径：" + err.Error())
			return true
		}
		paths = append(paths, abs)
	}
	s.sys.CopyText(strings.Join(paths, "\n"))
	if len(paths) == 1 {
		s.notifyDone("已复制文件路径：" + paths[0])
		return true
	}
	s.notifyDone(fmt.Sprintf("已复制 %d 个路径", len(paths)))
	return true
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
