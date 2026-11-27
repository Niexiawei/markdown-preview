package view

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/workspace"
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

// menuTargets are the rows a context menu acts on: every picked row when the
// row it opened on is one of them, else the row alone, as a file manager's
// menu does.
func (s *State) menuTargets(k treeKey) []treeKey {
	if len(s.treeSel) > 1 && s.isRowSelected(k) {
		return slices.Clone(s.treeSel)
	}
	return []treeKey{k}
}

// projectRowKey is a project's own row: a project of one document is that
// document's row, as the tree builds it.
func projectRowKey(p workspace.Project) treeKey {
	k := treeKey{project: p.ID}
	if p.File != "" {
		k.path = p.Name
	}
	return k
}

// rowName is the name on disk of what a row shows: a renamed project's node
// still says the directory's name, not the name it shows in the window.
func (s *State) rowName(k treeKey) (string, bool) {
	tr, ok := s.treeOf(k.project)
	if !ok {
		return "", false
	}
	if k.path == "" {
		return filepath.Base(tr.project.Root), true
	}
	n, ok := s.treeNode(k)
	if !ok {
		return "", false
	}
	return n.Name, true
}

// isFileKey reports whether a row is a file rather than a directory: a project
// of one document is that document.
func (s *State) isFileKey(k treeKey) bool {
	tr, ok := s.treeOf(k.project)
	if !ok {
		return false
	}
	if tr.project.File != "" {
		return true
	}
	if k.path == "" {
		return false
	}
	n, ok := s.treeNode(k)
	return ok && !n.IsDir
}

// copyRows puts the rows themselves on the clipboard, as the file manager
// copies them: another application pastes the files, not paths to them.
func (s *State) copyRows(keys []treeKey) {
	if s.sys == nil || len(keys) == 0 {
		return
	}
	paths := make([]string, 0, len(keys))
	for _, k := range keys {
		abs, err := s.absOfKey(k)
		if err != nil {
			s.notifyErr("无法复制：" + err.Error())
			return
		}
		paths = append(paths, abs)
	}
	if err := s.sys.CopyFiles(paths...); err != nil {
		s.notifyErr("无法复制：" + err.Error())
		return
	}
	if len(paths) > 1 {
		s.notifyDone(fmt.Sprintf("已复制 %d 个文件", len(paths)))
		return
	}
	if s.isFileKey(keys[0]) {
		s.notifyDone("已复制文件：" + paths[0])
		return
	}
	s.notifyDone("已复制文件夹：" + paths[0])
}

// copyRowNames puts the rows' names on the clipboard, one per line.
func (s *State) copyRowNames(keys []treeKey) {
	if s.sys == nil || len(keys) == 0 {
		return
	}
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		name, ok := s.rowName(k)
		if !ok {
			s.notifyErr("无法复制文件名称：文件不存在")
			return
		}
		names = append(names, name)
	}
	s.sys.CopyText(strings.Join(names, "\n"))
	if len(names) > 1 {
		s.notifyDone(fmt.Sprintf("已复制 %d 个文件名称", len(names)))
		return
	}
	s.notifyDone("已复制文件名称：" + names[0])
}

// copyRowPaths puts the rows' absolute paths on the clipboard, one per line.
func (s *State) copyRowPaths(keys []treeKey) { s.copyPaths(keys) }

// revealRow shows a row in the file manager.
func (s *State) revealRow(k treeKey) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOfKey(k)
	if err != nil {
		s.notifyErr("无法打开文件管理器：" + err.Error())
		return
	}
	if err := s.sys.Reveal(abs); err != nil {
		s.notifyErr("无法打开文件管理器：" + err.Error())
		return
	}
	s.notify("已在文件管理器中打开：" + abs)
}

// absOfKey is a row's absolute path on disk: the project's own directory for a
// project node, the file itself for a document.
func (s *State) absOfKey(k treeKey) (string, error) { return s.ws.AbsOf(k.project, k.path) }

// copyTreeSelection puts the picked rows' absolute paths on the clipboard, one
// per line, and reports whether there was a row to copy: Cmd+C falls back to
// the document shown when there was none.
func (s *State) copyTreeSelection() bool { return s.copyPaths(s.treeSel) }

// copyPaths puts the rows' absolute paths on the clipboard, one per line, and
// reports whether there was a row to copy.
func (s *State) copyPaths(keys []treeKey) bool {
	if s.sys == nil || len(keys) == 0 {
		return false
	}
	paths := make([]string, 0, len(keys))
	for _, k := range keys {
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
