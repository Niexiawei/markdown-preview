package view

import (
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/workspace"
)

// The window's file actions: copying a document's path, and from the file
// tree, an entry's name or path, and showing a directory in the file manager.
// They ask the desktop through System, so that the tests need no clipboard
// and no window.

// copyDocPath puts the absolute path of the document shown in the clipboard,
// as the title bar's button and Cmd+C do.
func (s *State) copyDocPath() {
	p, ok := s.SelectedProject()
	if !ok || s.sel.empty() || s.sys == nil {
		return
	}
	abs, err := s.ws.AbsOf(p.ID, s.sel.path)
	if err != nil {
		s.status = "无法复制文件路径：" + err.Error()
		return
	}
	s.sys.CopyText(abs)
	s.status = "已复制文件路径：" + abs
}

// copyNodeFile puts a tree entry itself on the clipboard, as the file manager
// copies it: another application pastes the file, not a path to it.
func (s *State) copyNodeFile(tr projectTree, n *workspace.Node) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOf(tr, n)
	if err != nil {
		s.status = "无法复制：" + err.Error()
		return
	}
	if err := s.sys.CopyFiles(abs); err != nil {
		s.status = "无法复制：" + err.Error()
		return
	}
	if n.IsDir {
		s.status = "已复制文件夹：" + abs
		return
	}
	s.status = "已复制文件：" + abs
}

// copyNodeName puts a tree entry's name in the clipboard.
func (s *State) copyNodeName(n *workspace.Node) {
	if s.sys == nil {
		return
	}
	s.sys.CopyText(n.Name)
	s.status = "已复制文件名称：" + n.Name
}

// copyNodePath puts a tree entry's absolute path in the clipboard.
func (s *State) copyNodePath(tr projectTree, n *workspace.Node) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOf(tr, n)
	if err != nil {
		s.status = "无法复制文件路径：" + err.Error()
		return
	}
	s.sys.CopyText(abs)
	s.status = "已复制文件路径：" + abs
}

// revealNode shows a directory of the tree in the file manager.
func (s *State) revealNode(tr projectTree, n *workspace.Node) {
	if s.sys == nil {
		return
	}
	abs, err := s.absOf(tr, n)
	if err != nil {
		s.status = "无法打开文件管理器：" + err.Error()
		return
	}
	if err := s.sys.Reveal(abs); err != nil {
		s.status = "无法打开文件管理器：" + err.Error()
		return
	}
	s.status = "已在文件管理器中打开：" + abs
}

// absOf is a tree entry's absolute path on disk.
func (s *State) absOf(tr projectTree, n *workspace.Node) (string, error) {
	return s.ws.AbsOf(tr.project.ID, n.Path)
}

// handleShortcuts copies the document shown for the platform's Cmd+C, which
// the title bar's button does too.
func (s *State) handleShortcuts(c *ui.Context) {
	s.findShortcuts(c)
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyC):
		s.copyDocPath()
	case c.Shortcut(ui.Cmd, ui.KeyEqual):
		s.zoomBy(zoomStep)
	case c.Shortcut(ui.Cmd, ui.KeyMinus):
		s.zoomBy(-zoomStep)
	case c.Shortcut(ui.Cmd, ui.Key0):
		s.resetZoom()
	}
}
