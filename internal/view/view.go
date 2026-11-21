package view

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// View builds the window as one frame, for the tests: the shell lays these
// same pieces out in its slots (Left, LeftHeader, LeftToolbar, ContentTitle
// and Content), and this is what they look like side by side.
func (s *State) View(c *ui.Context) {
	s.drain()
	s.openURL = func(url string) { c.Services().OpenURL(url) }
	t := c.Theme()
	ui.Row(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Width(300).Background(t.Surface).Children(func() {
			s.leftHeader(c)
			ui.Scroll(c).Grow(1).Children(func() { s.fileTree(c) })
			s.leftToolbar(c)
		})
		ui.Column(c).Grow(1).Children(func() {
			s.contentTitle(c)
			s.preview(c)
			s.statusBar(c)
		})
	})
}

// Left is the shell's sidebar: the file tree, which the shell scrolls and
// collapses.
func (s *State) Left(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(sidebar.HeaderHeight(c), t.Space(1), 0, t.Space(1)).Children(func() {
		ui.Scroll(c).Grow(1).Children(func() { s.fileTree(c) })
	})
}

// LeftHeader is the shell's sidebar header, beside its collapse control: the
// project the tree belongs to.
func (s *State) LeftHeader(c *ui.Context) { s.leftHeader(c) }

// LeftToolbar is the shell's fixed footer of the sidebar: the actions that
// open, reload and serve documents.
func (s *State) LeftToolbar(c *ui.Context) { s.leftToolbar(c) }

// ContentTitle is the shell's titlebar area: the document shown.
func (s *State) ContentTitle(c *ui.Context) { s.contentTitle(c) }

// Content is the shell's page: the outline and the body, with the status bar
// under them.
func (s *State) Content(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(sidebar.HeaderHeight(c))
		ui.Column(c).Grow(1).Children(func() {
			s.preview(c)
			s.statusBar(c)
		})
	})
}

// leftHeader builds the project switcher: the active project's name, with the
// projects to choose from under it.
func (s *State) leftHeader(c *ui.Context) {
	t := c.Theme()
	label := "项目"
	if p, ok := s.ActiveProject(); ok {
		label = trimEllipsis(p.Name, 18)
	}
	button := components.Button(c, "", components.ButtonGhost).Label("切换项目")
	button.Gap(t.Space(0.75)).Children(func() {
		components.Icon(c, lucide.Folder()).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
		components.Text(c, label).SingleLine().Grow(1)
		components.Icon(c, lucide.ChevronsUpDown()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
	})
	if button.Clicked() {
		s.projectsOpen = !s.projectsOpen
	}
	ui.Popover(c, button, &s.projectsOpen, func() {
		ui.Column(c).Width(280).Padding(t.Space(1)).Gap(t.Space(0.25)).Children(func() {
			if len(s.Projects()) == 0 {
				components.Text(c, "还没有项目").FontSize(12).TextColor(t.TextMuted)
				return
			}
			for _, p := range s.Projects() {
				p := p
				active := p.ID == s.activeID
				row := components.Button(c, "", components.ButtonGhost).Label(p.Name)
				row.Gap(t.Space(0.75)).Children(func() {
					components.Icon(c, lucide.Folder()).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
					components.Text(c, p.Name).SingleLine().Grow(1)
					if active {
						components.Icon(c, lucide.Check()).Size(t.Rem(1), t.Rem(1)).TextColor(t.Accent)
					}
				})
				if active {
					row.Background(selectedRowTint(t))
				}
				if row.Clicked() {
					s.SelectProject(p.ID)
					s.projectsOpen = false
				}
			}
			ui.Row(c).Height(1).Background(t.Border)
			remove := components.Button(c, "", components.ButtonGhost).Label("移除当前项目")
			remove.Gap(t.Space(0.75)).Children(func() {
				components.Icon(c, lucide.Trash()).Size(t.Rem(1), t.Rem(1)).TextColor(t.Warning)
				components.Text(c, "移除当前项目").SingleLine().TextColor(t.Warning)
			})
			if remove.Clicked() {
				s.RemoveProject(s.activeID)
				s.projectsOpen = false
			}
		})
	})
}

// leftToolbar builds the actions under the tree: open a project, reload it,
// and the local MCP server.
func (s *State) leftToolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(t.Space(1)).Gap(t.Space(0.5)).AlignItems(ui.Center).Children(func() {
		if components.IconButton(c, lucide.FolderOpen(), "打开项目", components.IconButtonCompact).Clicked() {
			s.OpenProject()
		}
		if components.IconButton(c, lucide.RefreshCw(), "刷新", components.IconButtonCompact).
			Disabled(s.activeID == "").Clicked() {
			s.Refresh()
		}
		ui.Spacer(c)
		s.mcpSettings(c)
	})
}

// contentTitle builds the titlebar's leading area: the document the window
// shows.
func (s *State) contentTitle(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(sidebar.HeaderHeight(c)).Padding(0, t.Space(2)).AlignItems(ui.Center).Gap(t.Space(0.75)).Children(func() {
		if path := s.shortPath(); path != "" {
			components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
			components.Text(c, trimEllipsis(path, 90)).FontSize(12).TextColor(t.TextMuted).SingleLine()
		}
	})
}

// tree builds the markdown tree, or what to do when there is nothing to show.
func (s *State) fileTree(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(t.Space(0.5)).Gap(t.Space(0.25)).Children(func() {
		if _, ok := s.ActiveProject(); !ok {
			components.Text(c, "打开一个目录作为项目").FontSize(12).TextColor(t.TextMuted)
			return
		}
		if s.tree == nil || len(s.tree.Children) == 0 {
			components.Text(c, "这个目录下没有 markdown 文件").FontSize(12).TextColor(t.TextMuted)
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

// statusBar builds the row at the bottom: the rendering mode of the document
// shown, and what the window is doing.
func (s *State) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(34).PaddingX(t.Space(1)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		if s.selected != "" {
			s.modeIdx = s.modeIndex()
			labels := []string{"原生", "WebView"}
			if !s.webOK && s.win != nil && s.renderMode[s.selected] == config.RenderWebView {
				labels[1] = "WebView（不可用）"
			}
			ui.Segmented(c, &s.modeIdx, labels...).Disabled(s.selected == "")
			if s.modeIdx != s.modeIndex() {
				s.setMode([]config.RenderMode{config.RenderNative, config.RenderWebView}[max(0, min(1, s.modeIdx))])
				s.onDocChanged()
			}
		}
		components.Text(c, trimEllipsis(s.status, 120)).FontSize(12).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
	})
}
