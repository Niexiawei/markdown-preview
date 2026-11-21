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

// leftHeader builds the project panel: the active project's name, and under it
// every project the window knows, checked when open. A click opens or closes
// one; the right click has the project's actions.
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
				open := s.cfg.IsOpen(p.ID)
				row := components.Button(c.Key("project:"+p.ID), "", components.ButtonGhost).Label("项目：" + p.Name)
				// The row opens and closes the project: a check box, so that a
				// screen reader says which.
				row.Role(ui.RoleCheckBox).Checked(open)
				row.Gap(t.Space(0.75)).Children(func() {
					components.Icon(c, lucide.Folder()).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
					components.Text(c, p.Name).SingleLine().Grow(1)
					if open {
						components.Icon(c, lucide.Check()).Size(t.Rem(1), t.Rem(1)).TextColor(t.Accent)
					}
				})
				if open {
					row.Background(selectedRowTint(t))
				}
				if row.Clicked() {
					s.ToggleProject(p.ID)
				}
				components.ContextMenu(c, row, func(m *components.Menu) {
					if m.Item("在文件管理器中打开").Chosen() {
						s.revealProject(p)
					}
					if m.Item("复制项目路径").Chosen() {
						s.copyProjectPath(p)
					}
					m.Separator()
					if m.Item("移除项目…").Chosen() {
						s.RemoveProject(p.ID)
					}
				})
			}
		})
	})
}

// leftToolbar builds the actions under the tree: open a project, reload it,
// and the local MCP server.
func (s *State) leftToolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(t.Space(1)).Gap(t.Space(0.5)).AlignItems(ui.Center).Children(func() {
		if components.IconButton(c, lucide.FolderOpen(), "打开项目", components.IconButtonPlain).Clicked() {
			s.OpenProject()
		}
		if components.IconButton(c, lucide.RefreshCw(), "刷新", components.IconButtonPlain).
			Disabled(s.activeID == "").Clicked() {
			s.Refresh()
		}
		ui.Spacer(c)
		s.mcpSettings(c)
	})
}

// contentTitle builds the titlebar's leading area: the document the window
// shows, and the rendering mode it shows with.
func (s *State) contentTitle(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(sidebar.HeaderHeight(c)).Padding(0, t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		if path := s.shortPath(); path != "" {
			components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
			// The path gives the rest of the bar its room: a long one ends in
			// an ellipsis in its middle, keeping the file's name, rather than
			// pushing the mode switch out of the window.
			components.Text(c, path).FontSize(12).TextColor(t.TextMuted).SingleLine().TruncateMiddle().Shrink(1)
			s.modeToggle(c)
		}
	})
}

// modeToggle is the rendering mode of the document shown, right after its
// name: the two backends as the segments of the theme's control.
func (s *State) modeToggle(c *ui.Context) {
	idx := s.modeIndex()
	parts := components.Tabs(c, &idx, components.TabsSegmented, "原生", "WebView")
	// A window that could not make a web view keeps that segment out of the
	// pointer's way while the document asks for it.
	if !s.webOK && s.win != nil && s.mode() == config.RenderWebView {
		parts.Tab(1).Disabled(true)
	}
	if idx != s.modeIndex() {
		modes := []config.RenderMode{config.RenderNative, config.RenderWebView}
		s.setMode(modes[idx])
		s.onDocChanged()
	}
}

// fileTree builds the markdown tree, or what to do when there is nothing to
// show. With one project open its entries are the top level; with more, each
// project gets a node of its own name.
func (s *State) fileTree(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(t.Space(0.5)).Gap(t.Space(0.25)).Children(func() {
		if len(s.trees) == 0 {
			components.Text(c, "打开一个目录作为项目").FontSize(12).TextColor(t.TextMuted)
			return
		}
		if !s.hasDocs() {
			components.Text(c, "这个目录下没有 markdown 文件").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Tree(c, func() {
			if len(s.trees) == 1 {
				s.treeItems(c, s.trees[0])
				return
			}
			for _, tr := range s.trees {
				tr := tr
				ui.TreeItem(c, tr.project.Name, s.openOf("project:"+tr.project.ID), func() {
					s.treeItems(c, tr)
				})
			}
		})
	})
}

// treeItems builds one project's entries, and its directories' in turn.
func (s *State) treeItems(c *ui.Context, tr projectTree) {
	for _, child := range tr.root.Children {
		child := child
		if child.IsDir {
			ui.TreeItem(c, child.Name, s.openOf(tr.project.ID+":"+child.Path), func() {
				s.treeItems(c, tr.childOf(child))
			})
			continue
		}
		selected := s.sel.project == tr.project.ID && child.Path == s.sel.path
		item := ui.TreeItem(c, child.Name, nil, nil).Selected(selected)
		if item.Clicked() {
			s.selectDocIn(tr.project.ID, child.Path)
		}
	}
}

// childOf is the tree of tr from node down, which the tree builds as the user
// opens its directories.
func (tr projectTree) childOf(node *workspace.Node) projectTree {
	return projectTree{project: tr.project, root: node}
}

// statusBar builds the row at the bottom: the rendering mode of the document
// shown, and what the window is doing.
func (s *State) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(24).PaddingX(t.Space(1)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		components.Text(c, trimEllipsis(s.status, 140)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		ui.Spacer(c)
	})
}
