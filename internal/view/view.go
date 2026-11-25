package view

import (
	"time"

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
	s.handleShortcuts(c)
	s.updateToasts(c)
	s.openURL = func(url string) { c.Services().OpenURL(url) }
	t := c.Theme()
	ui.Row(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Width(300).FillHeight().Background(t.Surface).Children(func() {
			s.leftHeader(c)
			if s.treeSearch.open {
				s.treeSearchRow(c)
			}
			s.fileTree(c)
			s.leftToolbar(c)
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Clip().Children(func() {
			// As the shell's header row: the tabs sit in it, and the title
			// row is the first thing the page shows under it.
			ui.Row(c).Height(sidebar.HeaderHeight(c)).Children(func() { s.ContentTitle(c) })
			s.titleRow(c)
			s.preview(c)
		})
	})
}

// Left is the shell's sidebar: the file tree, which the shell scrolls and
// collapses.
func (s *State) Left(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(sidebar.HeaderHeight(c), t.Space(1), 0, t.Space(1)).Children(func() {
		if s.treeSearch.open {
			s.treeSearchRow(c)
		}
		s.fileTree(c)
	})
}

// LeftHeader is the shell's sidebar header, beside its collapse control: the
// project the tree belongs to.
func (s *State) LeftHeader(c *ui.Context) { s.leftHeader(c) }

// LeftToolbar is the shell's fixed footer of the sidebar: the actions that
// open, reload and serve documents.
func (s *State) LeftToolbar(c *ui.Context) { s.leftToolbar(c) }

// titleInsetGap is the room onevra's sidebar leaves between its sidebar header
// and this slot, and titleInsetWanted the room this app wants there: the number
// is hardcoded in shells/sidebar (`titleInset = 24`) and no option exposes it,
// so the strip is pulled back by the difference. 8 read as flush against the
// sidebar; 12 leaves the first tab's corner visible.
const (
	titleInsetGap    float32 = 24
	titleInsetWanted float32 = 12
)

// ContentTitle is the shell's titlebar area: the tabs of the documents the
// window has open. The title row under them is the page's own.
func (s *State) ContentTitle(c *ui.Context) {
	s.tabBarAt(c, titleInsetWanted-titleInsetGap)
}

// Content is the shell's page: the outline and the body, filling the page.
func (s *State) Content(c *ui.Context) {
	s.drain()
	s.handleShortcuts(c)
	s.updateToasts(c)
	ui.Column(c).Fill().Children(func() {
		// The header, which holds the tabs, floats over the top of the page.
		ui.Row(c).Height(sidebar.HeaderHeight(c))
		s.titleRow(c)
		s.preview(c)
	})
}

// titleRow is the room the document's title and mode switch take, between the
// tabs in the header and the page's own content, in DIPs.
const titleRowHeight float32 = 34

// titleRow builds the row under the tabs: the document shown, the button that
// copies its path, and the rendering mode it shows with.
func (s *State) titleRow(c *ui.Context) {
	ui.Row(c).Height(titleRowHeight).Children(func() { s.contentTitle(c) })
	ui.Divider(c)
}

// leftHeader builds the project panel: the tree's bulk actions, then the active
// project's name, and under it every project the window knows, checked when
// open. A click opens or closes one; the right click has the project's actions.
func (s *State) leftHeader(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0.5)).Children(func() {
		s.treeBulkActions(c)
		s.projectSwitcher(c)
	})
}

// treeBulkActions builds the tree's bulk actions: unfold or fold every directory
// of every open project. They sit before the project panel's button; the
// outline's own pair sits in its header.
func (s *State) treeBulkActions(c *ui.Context) {
	none := len(s.trees) == 0
	if components.IconButton(c, lucide.UnfoldVertical(), "展开全部目录树", components.IconButtonPlain).
		Disabled(none).Clicked() {
		s.setTreeOpen("", true)
	}
	if components.IconButton(c, lucide.FoldVertical(), "折叠全部目录树", components.IconButtonPlain).
		Disabled(none).Clicked() {
		s.setTreeOpen("", false)
	}
}

// projectSwitcher builds the project panel's button: the active project's name,
// and the list of every project the window knows.
func (s *State) projectSwitcher(c *ui.Context) {
	t := c.Theme()
	label, reading := "项目", ""
	if p, ok := s.ActiveProject(); ok {
		label = trimEllipsis(p.Name, 18)
		if s.scanning[p.ID] {
			// The tree below still shows what the window read last time: the
			// name says that it is being read again.
			reading = p.Name
		}
	}
	button := components.Button(c, "", components.ButtonGhost).Label("切换项目")
	button.Grow(1).Gap(t.Space(0.75)).Busy(reading != "").Children(func() {
		icon := lucide.Folder()
		if p, ok := s.ActiveProject(); ok && p.File != "" {
			icon = lucide.FileText()
		}
		components.Icon(c, icon).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
		components.Text(c, label).SingleLine().Grow(1)
		if reading != "" {
			components.Spinner(c, components.SpinnerSpokes).Label("正在扫描 " + reading)
		}
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
				kind := "项目："
				icon := lucide.Folder()
				if p.File != "" {
					kind, icon = "文件：", lucide.FileText()
				}
				row := components.Button(c.Key("project:"+p.ID), "", components.ButtonGhost).Label(kind + p.Name)
				// The row opens and closes the project: a check box, so that a
				// screen reader says which.
				row.Role(ui.RoleCheckBox).Checked(open)
				row.Gap(t.Space(0.75)).Children(func() {
					components.Icon(c, icon).Size(t.Rem(1), t.Rem(1)).TextColor(t.TextMuted)
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
					if m.Item("展开此项目的全部目录").Chosen() {
						s.setTreeOpen(p.ID, true)
					}
					if m.Item("折叠此项目的全部目录").Chosen() {
						s.setTreeOpen(p.ID, false)
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
		if components.IconButton(c, lucide.FilePlus(), "打开 Markdown 文件", components.IconButtonPlain).Clicked() {
			s.OpenDocumentFile()
		}
		if components.IconButton(c, lucide.RefreshCw(), "刷新", components.IconButtonPlain).
			Disabled(s.activeID == "").Clicked() {
			s.Refresh()
		}
		ui.Spacer(c)
		s.bellButton(c)
		s.mcpSettings(c)
	})
}

// contentTitle builds the titlebar's leading area: the document the window
// shows, the button that copies its path, and the rendering mode it shows
// with.
func (s *State) contentTitle(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().FillHeight().Padding(0, t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		if path := s.shortPath(); path != "" {
			components.Icon(c, lucide.FileText()).Size(t.Rem(0.875), t.Rem(0.875)).TextColor(t.TextMuted)
			// The path gives the rest of the bar its room: a long one ends in
			// an ellipsis in its middle, keeping the file's name, rather than
			// pushing the mode switch out of the window.
			components.Text(c, path).FontSize(12).TextColor(t.TextMuted).SingleLine().TruncateMiddle().Shrink(1)
			if components.IconButton(c, lucide.Copy(), "复制文档路径", components.IconButtonCompact).Clicked() {
				s.copyDocPath()
			}
			s.modeToggle(c)
		}
	})
}

// modeToggle is the rendering mode of the document shown, right after its
// name: the two backends as the segments of the theme's control.
func (s *State) modeToggle(c *ui.Context) {
	t := c.Theme()
	idx := s.modeIndex()
	parts := components.Tabs(c, &idx, components.TabsSegmented, "原生", "WebView")
	// The header is shorter than a page: a smaller label and the tabs' own
	// padding brought in keep the whole control inside the slot the shell
	// clips to, rather than centring it in a row taller than that slot.
	parts.List.FontSize(11)
	for i := range 2 {
		parts.Tab(i).Padding(t.Space(1), t.Space(2))
	}
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
// project gets a node of its own name. While the search is open it shows what
// the phrase matches instead, and the tree is the list of candidates.
func (s *State) fileTree(c *ui.Context) {
	t := c.Theme()
	if s.treeSearch.reveal && !s.treeSearch.revealSeen.IsZero() && !s.treeSearch.revealSeen.Equal(c.Now()) {
		// The frame that asked the tree to slide is over: what it asked for is
		// on its way, or done.
		s.treeSearch.reveal, s.treeSearch.revealSeen = false, time.Time{}
	}
	ui.Column(c).Grow(1).FillWidth().Padding(t.Space(0.5), 0, t.Space(0.5), 0).
		Gap(t.Space(0.25)).Children(func() {
		if len(s.trees) == 0 {
			if name := s.scanningName(); name != "" {
				components.Text(c, "正在扫描 "+name+"…").FontSize(12).TextColor(t.TextMuted)
				return
			}
			components.Text(c, "打开一个目录或 Markdown 文件").FontSize(12).TextColor(t.TextMuted)
			return
		}
		if !s.hasDocs() {
			components.Text(c, "这个目录下没有 markdown 文件").FontSize(12).TextColor(t.TextMuted)
			return
		}
		if s.searching() && len(s.treeSearch.candidates) == 0 {
			components.Text(c, "没有匹配的文件").FontSize(12).TextColor(t.TextMuted)
			return
		}
		s.treeBody(c)
	})
}

// projectActions adds a project node's two bulk actions after its name: the
// directories of that project alone, unfolded or folded, without the other
// projects'. They are compact because the row is, and their names carry the
// project so that a screen reader and a test can tell two of them apart.
func (s *State) projectActions(c *ui.Context, item ui.Element, tr projectTree) {
	item.Children(func() {
		ui.Spacer(c)
		if components.IconButton(c, lucide.UnfoldVertical(), "展开项目 "+tr.project.Name+" 的全部目录",
			components.IconButtonCompact).Clicked() {
			s.setTreeOpen(tr.project.ID, true)
		}
		if components.IconButton(c, lucide.FoldVertical(), "折叠项目 "+tr.project.Name+" 的全部目录",
			components.IconButtonCompact).Clicked() {
			s.setTreeOpen(tr.project.ID, false)
		}
	})
}

// projectMenu gives a project node its right click: folding and unfolding every
// directory of that project alone. A window with one project has no such node,
// so the project panel's rows carry the same two entries.
func (s *State) projectMenu(c *ui.Context, item ui.Element, tr projectTree) {
	components.ContextMenu(c, item, func(m *components.Menu) {
		if m.Item("展开此项目的全部目录").Chosen() {
			s.setTreeOpen(tr.project.ID, true)
		}
		if m.Item("折叠此项目的全部目录").Chosen() {
			s.setTreeOpen(tr.project.ID, false)
		}
	})
}

// nodeMenu gives a tree entry its right click: the entry itself on the
// clipboard for another application to paste, its name or its path as text,
// and for a directory, showing it in the file manager.
func (s *State) nodeMenu(c *ui.Context, item ui.Element, tr projectTree, n *workspace.Node) {
	components.ContextMenu(c, item, func(m *components.Menu) {
		copyFile := "复制文件"
		if n.IsDir {
			copyFile = "复制文件夹"
		}
		if m.Item(copyFile).Chosen() {
			s.copyNodeFile(tr, n)
		}
		m.Separator()
		if m.Item("复制文件名称").Chosen() {
			s.copyNodeName(n)
		}
		if m.Item("复制文件路径").Chosen() {
			s.copyNodePath(tr, n)
		}
		if !n.IsDir {
			return
		}
		m.Separator()
		if m.Item("在文件管理器中打开").Chosen() {
			s.revealNode(tr, n)
		}
	})
}
