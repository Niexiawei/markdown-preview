package view

import (
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/workspace"
)

// View builds the window: the toolbar, the sidebar and the preview between
// them, and the status bar under them.
func (s *State) View(c *ui.Context) {
	s.drain()
	ui.Column(c).Fill().Children(func() {
		s.toolbar(c)
		ui.Split(c, &s.sidebarW, func() { s.sidebar(c) }, func() { s.preview(c) }).
			Grow(1).SplitReset(300)
		s.statusbar(c)
	})
}

// toolbar builds the row at the top: the project switcher, the buttons that
// open and reload a project, and the document the window shows.
func (s *State) toolbar(c *ui.Context) {
	t := c.Theme()
	row := ui.Row(c).Height(46).PaddingX(12).AlignItems(ui.Center).Gap(8)
	row.Children(func() {
		label := "项目"
		if p, ok := s.ActiveProject(); ok {
			label = trimEllipsis(p.Name, 28) + " ▾"
		}
		ui.PopoverMenuButton(c, label, func(m *ui.Menu) {
			m.MaxWidth(360)
			for _, p := range s.Projects() {
				it := m.Item(p.Name).ID(p.ID).Subtitle(p.Root)
				if p.ID == s.activeID {
					it.Checked(true)
				}
				if it.Chosen() {
					s.SelectProject(p.ID)
				}
			}
			if len(s.Projects()) > 0 {
				m.Separator()
				if m.Item("移除当前项目").Danger().Chosen() {
					s.RemoveProject(s.activeID)
				}
			}
		})
		if ui.Button(c, "打开项目…").Clicked() {
			s.OpenProject()
		}
		if ui.Button(c, "刷新").Disabled(s.activeID == "").Clicked() {
			s.Refresh()
		}
		ui.Row(c).Grow(1)
		if path := s.shortPath(); path != "" {
			ui.Text(c, trimEllipsis(path, 90)).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

// sidebar builds the file tree, or what to do when there is nothing to show.
func (s *State) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(8).Gap(4).Background(t.Surface).Children(func() {
		if _, ok := s.ActiveProject(); !ok {
			ui.Text(c, "打开一个目录作为项目").FontSize(13).TextColor(t.TextMuted)
			return
		}
		if s.tree == nil || len(s.tree.Children) == 0 {
			ui.Text(c, "这个目录下没有 markdown 文件").FontSize(13).TextColor(t.TextMuted)
			return
		}
		ui.Tree(c, func() {
			s.treeItems(c, s.tree)
		})
	})
}

// treeItems builds one directory's entries, and its directories' in turn.
func (s *State) treeItems(c *ui.Context, node *workspace.Node) {
	for _, child := range node.Children {
		child := child
		if child.IsDir {
			ui.TreeItem(c, child.Name, s.openOf(child.Path), func() {
				s.treeItems(c, child)
			})
			continue
		}
		item := ui.TreeItem(c, child.Name, nil, nil).Selected(child.Path == s.selected)
		if item.Clicked() {
			s.selectDoc(child.Path)
		}
	}
}

// statusbar builds the row at the bottom: what the window is doing, and the
// rendering mode of the document shown.
func (s *State) statusbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(32).PaddingX(12).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Text(c, trimEllipsis(s.status, 120)).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Grow(1)
	})
}
