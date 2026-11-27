package view

import "github.com/egoist/mygo/ui"

// copyDocPath puts the absolute path of the document shown in the clipboard,
// as the title bar's button and Cmd+C do when no tree row is picked. The tree's
// rows are copied by treesel.go, and ask the desktop through System, so that
// the tests need no clipboard and no window.
func (s *State) copyDocPath() {
	p, ok := s.SelectedProject()
	if !ok || s.sel.empty() || s.sys == nil {
		return
	}
	abs, err := s.ws.AbsOf(p.ID, s.sel.path)
	if err != nil {
		s.notifyErr("无法复制文件路径：" + err.Error())
		return
	}
	s.sys.CopyText(abs)
	s.notifyDone("已复制文件路径：" + abs)
}

// handleShortcuts copies what the user is pointing at for the platform's
// Cmd+C: the rows picked in the tree, and the document shown when none is, as
// the title bar's button copies it too.
func (s *State) handleShortcuts(c *ui.Context) {
	s.searchShortcuts(c)
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyC):
		if !s.copyTreeSelection() {
			s.copyDocPath()
		}
	case c.Shortcut(ui.Cmd, ui.KeyW):
		s.closeTab(s.sel)
	case c.Shortcut(ui.Cmd, ui.KeyEqual):
		s.zoomBy(zoomStep)
	case c.Shortcut(ui.Cmd, ui.KeyMinus):
		s.zoomBy(-zoomStep)
	case c.Shortcut(ui.Cmd, ui.Key0):
		s.resetZoom()
	}
}
