package view

import (
	"path"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// openTab shows the document sel, adding it to the tabs unless it is open
// there already. Every way of showing a document — the tree, a link in it, a
// search hit — comes through here, so that one document is one tab.
func (s *State) openTab(sel docSel) {
	if sel.empty() || sel == s.sel {
		return
	}
	if !s.hasTab(sel) {
		s.tabs = append(s.tabs, sel)
	}
	// Every tab keeps where its body was scrolled, so that switching back
	// shows the document where the user left it.
	if _, ok := s.scrolls[sel]; !ok {
		s.scrolls[sel] = &ui.ScrollState{}
	}
	s.activateTab(sel)
}

// activateTab makes sel the document shown, without adding a tab.
func (s *State) activateTab(sel docSel) {
	s.sel = sel
	s.status = ""
	// Another document's outline starts at its top, not where the last
	// one's scroll was left. A reload of the same document keeps it.
	s.tocScroll.X, s.tocScroll.Y = 0, 0
	s.onDocChanged()
}

// hasTab reports whether the document is open in a tab.
func (s *State) hasTab(sel docSel) bool { return s.tabIndex(sel) >= 0 }

// tabIndex is where the document's tab sits, or -1.
func (s *State) tabIndex(sel docSel) int {
	for i, tab := range s.tabs {
		if tab == sel {
			return i
		}
	}
	return -1
}

// closeTab closes the document's tab. Closing the document shown shows the
// tab to its right, else the one to its left, else nothing.
func (s *State) closeTab(sel docSel) {
	i := s.tabIndex(sel)
	if i < 0 {
		return
	}
	s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	delete(s.scrolls, sel)
	if sel != s.sel {
		return
	}
	switch {
	case i < len(s.tabs):
		s.activateTab(s.tabs[i])
	case i > 0:
		s.activateTab(s.tabs[i-1])
	default:
		s.sel = docSel{}
		s.status = ""
		s.onDocChanged()
	}
}

// dropTab forgets the document's tab without changing what the window shows:
// the caller is already clearing it, as when the file went away.
func (s *State) dropTab(sel docSel) {
	if i := s.tabIndex(sel); i >= 0 {
		s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	}
	delete(s.scrolls, sel)
}

// dropTabsOf closes every tab of one project: a project that is closed or
// removed takes its documents with it.
func (s *State) dropTabsOf(projectID string) {
	kept := s.tabs[:0]
	for _, tab := range s.tabs {
		if tab.project == projectID {
			delete(s.scrolls, tab)
			continue
		}
		kept = append(kept, tab)
	}
	s.tabs = kept
	if s.sel.project == projectID {
		s.sel = docSel{}
		s.status = ""
		s.onDocChanged()
	}
}

// renameTab moves a tab and its scroll position to its new path, keeping where
// it sat in the bar.
func (s *State) renameTab(from, to docSel) {
	i := s.tabIndex(from)
	if i < 0 {
		return
	}
	s.tabs[i] = to
	if st, ok := s.scrolls[from]; ok {
		delete(s.scrolls, from)
		s.scrolls[to] = st
	}
}

// tabsMenuWidth is how wide the list of all tabs is, in DIPs, and
// tabListTextWidth how much of a name its rows show.
const tabsMenuWidth float32 = 260

const tabListTextWidth float32 = tabsMenuWidth - 3*8 - 20 - tabIcon - 4

// tabBarHeight is the room the tab bar takes above the body, in DIPs.
const tabBarHeight float32 = 36

// tabIcon is the room the document icon takes in a tab, in DIPs.
const tabIcon float32 = 14

// tabClose is the room the close button takes, and tabPad the room on either
// side of a tab's name: the same numbers the fit and the layout use, so that a
// tab is never wider than the room the bar made for it.
func tabClose(c *ui.Context) float32 { return max(20, c.Theme().Rem(1.25)) }

func tabPad(c *ui.Context) float32 { return c.Theme().Space(1) }

// tabTextWidth is how wide the name of a tab can be inside a tab of width.
func tabTextWidth(c *ui.Context, width float32) float32 {
	return width - tabClose(c) - 2*tabPad(c) - tabIcon - c.Theme().Space(0.5)
}

// tabWidth is how wide the tab of name is: as wide as its name needs, held
// between the narrowest that still shows a name and the widest that leaves
// room for the next one.
func tabWidth(c *ui.Context, name string) float32 {
	t := c.Theme()
	w, _ := c.MeasureText(0, ui.Span{Text: name, Font: t.Font, Size: t.FontSize})
	return min(t.Rem(12), max(t.Rem(7), w+tabClose(c)+2*tabPad(c)+tabIcon+t.Space(0.5)))
}

// fitText trims text to the width it has, ending what is left with an ellipsis:
// a tab shows one line of a document's name, not the whole of it.
func fitText(c *ui.Context, text string, width float32) string {
	t := c.Theme()
	measure := func(s string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Font: t.Font, Size: t.FontSize})
		return w
	}
	if width <= 0 {
		return ""
	}
	if measure(text) <= width {
		return text
	}
	runes := []rune(text)
	for n := len(runes) - 1; n > 0; n-- {
		if s := string(runes[:n]) + "…"; measure(s) <= width {
			return s
		}
	}
	return "…"
}

// visibleTabs returns the tabs the bar shows, and whether any are left out: as
// many as fit, counting the gap between them, the one shown first, so that it is
// never the one cut.
func (s *State) visibleTabs(c *ui.Context, avail float32) ([]docSel, bool) {
	gap := c.Theme().Space(0.5)
	widths := make([]float32, len(s.tabs))
	total := float32(0)
	for i, tab := range s.tabs {
		widths[i] = tabWidth(c, trimEllipsis(path.Base(tab.path), 40))
		total += widths[i] + gap
	}
	if total <= avail {
		return s.tabs, false
	}
	active := s.tabIndex(s.sel)
	if active < 0 {
		active = 0
	}
	// The tab shown first, then as many on either side as fit, right before
	// left: the next tab is likelier to be the one wanted. Each one costs its
	// width and the gap before it.
	room, from, to := avail-widths[active], active, active
	for {
		grew := false
		if to+1 < len(widths) && widths[to+1]+gap <= room {
			to++
			room -= widths[to] + gap
			grew = true
		}
		if from-1 >= 0 && widths[from-1]+gap <= room {
			from--
			room -= widths[from] + gap
			grew = true
		}
		if !grew {
			break
		}
	}
	return s.tabs[from : to+1], true
}

// tabBar builds the header's row: a tab per open document, which a click shows
// and its close button closes, and, when they do not fit, the button listing all
// of them.
func (s *State) tabBar(c *ui.Context) {
	t := c.Theme()
	row := ui.Row(c).Key("tab-bar").FillWidth().Height(tabBarHeight).AlignItems(ui.Center)
	if len(s.tabs) == 0 {
		return
	}
	// The bar's own width, from the frame before: the shell pads its header for
	// the window's caption buttons, so the content's width is not the room the
	// bar has, and a tab laid out for more room than it has runs under them.
	avail := row.Bounds().W - 2*t.Space(1)
	if avail <= 0 {
		avail = s.windowWidth() - 2*t.Space(1)
	}
	shown, cut := s.visibleTabs(c, avail)
	if cut {
		// The overflow button takes its own room, so one fewer tab fits.
		shown, _ = s.visibleTabs(c, avail-max(20, t.Rem(1.25))-t.Space(0.5))
	}
	row.Padding(0, t.Space(1)).Gap(t.Space(0.5)).Children(func() {
		for _, tab := range shown {
			s.tab(c, tab, tabWidth(c, path.Base(tab.path)))
		}
		if !cut {
			return
		}
		ui.Spacer(c)
		button := components.IconButton(c, lucide.ChevronDown(), "全部已打开文件", components.IconButtonCompact)
		if button.Clicked() {
			s.tabsOpen = !s.tabsOpen
		}
		ui.Popover(c, button, &s.tabsOpen, func() { s.tabsMenu(c) })
	})
}

// tab builds one tab of the bar, width wide: its name, which a click shows, and
// the button closing it, side by side rather than nested, so that a close is
// never also a switch.
func (s *State) tab(c *ui.Context, tab docSel, width float32) {
	t := c.Theme()
	inner := tabBarHeight - 2*t.Space(1)
	closeW := tabClose(c)
	row := ui.Row(c).Width(width).Height(inner).Radius(t.Radius).
		AlignItems(ui.Center).Clip()
	if tab == s.sel {
		row.Background(selectedRowTint(t))
	}
	row.Children(func() {
		name := components.Button(c, "", components.ButtonGhost).
			Label("文档："+tab.path).Height(inner).
			Padding(t.Space(0.75), tabPad(c))
		name.Width(max(0, width-closeW)).Children(func() {
			// A row of its own, grown to the button's width, keeps the icon
			// and the name at the tab's left edge: a button centres what it
			// holds, and the name is what the tab is read by.
			ui.Row(c).Grow(1).Gap(t.Space(0.5)).AlignItems(ui.Center).Children(func() {
				components.Icon(c, lucide.FileText()).Size(tabIcon, tabIcon).TextColor(t.TextMuted)
				components.Text(c, fitText(c, path.Base(tab.path), tabTextWidth(c, width))).
					FontSize(12).SingleLine()
			})
		})
		if name.Clicked() {
			s.openTab(tab)
		}
		if components.IconButton(c, lucide.X(), "关闭 "+path.Base(tab.path), components.IconButtonCompact).Clicked() {
			s.closeTab(tab)
		}
	})
}

// tabsMenu builds the list every open document shows in, for the tabs the bar
// has no room for.
func (s *State) tabsMenu(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(260).Padding(t.Space(1)).Children(func() {
		ui.Scroll(c).MaxHeight(400).Children(func() {
			ui.Column(c).Gap(t.Space(0.25)).Children(func() {
				for _, tab := range s.tabs {
					s.tabRow(c, tab)
				}
			})
		})
	})
}

// tabRow builds one row of the list of all tabs: the name from the row's left
// edge, and the button closing it at the right.
func (s *State) tabRow(c *ui.Context, tab docSel) {
	t := c.Theme()
	row := ui.Row(c).Radius(t.Radius).AlignItems(ui.Center)
	if tab == s.sel {
		row.Background(selectedRowTint(t))
	}
	row.Children(func() {
		name := components.Button(c, "", components.ButtonGhost).Label("打开：" + tab.path)
		name.Grow(1).Children(func() {
			// As a tab of the bar: a row of its own, grown to the button's
			// width, so that a short name still starts at the left.
			ui.Row(c).Grow(1).Gap(t.Space(0.5)).AlignItems(ui.Center).Children(func() {
				components.Icon(c, lucide.FileText()).Size(tabIcon, tabIcon).TextColor(t.TextMuted)
				components.Text(c, fitText(c, path.Base(tab.path), tabListTextWidth)).
					FontSize(12).SingleLine()
			})
		})
		if name.Clicked() {
			s.openTab(tab)
			s.tabsOpen = false
		}
		if components.IconButton(c, lucide.X(), "关闭 "+path.Base(tab.path), components.IconButtonCompact).Clicked() {
			s.closeTab(tab)
		}
	})
}
