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
	// one's scroll was left, and no heading of it is the one last jumped
	// to. A reload of the same document keeps both.
	s.tocScroll.X, s.tocScroll.Y = 0, 0
	s.anchor = ""
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

// tabsMenuWidth is how wide the list of all tabs is, in DIPs.
const tabsMenuWidth float32 = 260

// tabBarHeight is the room the tab bar takes above the body, in DIPs.
const tabBarHeight float32 = 36

// tabIcon is the room the document icon takes in a tab, in DIPs.
const tabIcon float32 = 14

// tabClose is the room the close button takes, and tabPad the room on either
// side of a tab's name: the same numbers the fit and the layout use, so that a
// tab is never wider than the room the bar made for it.
func tabClose(c *ui.Context) float32 { return max(20, c.Theme().Rem(1.25)) }

// tabOverflow is the room the button listing every tab takes: an icon button of
// the plain size, which reads as a control rather than as decoration.
func tabOverflow(c *ui.Context) float32 { return max(32, c.Theme().Rem(2)) }

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

// tabsWidth is how wide all the tabs are together, gaps included: more than the
// room the bar has means the strip scrolls, and that the list of all of them is
// worth showing.
func (s *State) tabsWidth(c *ui.Context, widths []float32) float32 {
	total := float32(0)
	for _, w := range widths {
		total += w
	}
	if len(widths) > 1 {
		total += float32(len(widths)-1) * c.Theme().Space(0.5)
	}
	return total
}

// revealTab scrolls the strip so that the tab shown shows, reporting whether
// the place it set is the place the strip can rest at: the strip's own range
// comes from the frame before, and a tab opened this frame has not grown it yet,
// so a place clamped to that range would stop one tab short and leave the tab
// shown cut.
func (s *State) revealTab(c *ui.Context, widths []float32, view float32) bool {
	if view <= 0 {
		return true
	}
	gap := c.Theme().Space(0.5)
	starts := make([]float32, len(widths))
	x := float32(0)
	for i, w := range widths {
		starts[i] = x
		x += w + gap
	}
	active := s.tabIndex(s.sel)
	if active < 0 {
		return true
	}
	// Where the tab shown has to begin for its end to come into view, and the
	// first tab start at or after it: the strip comes to rest where a tab
	// begins, never in the middle of one, where a tab cut down to its close
	// button reads as a stray ✕ in blank space.
	target := starts[active] + widths[active] - view
	desired := starts[active]
	for i := 0; i <= active; i++ {
		if starts[i] >= target {
			desired = starts[i]
			break
		}
	}
	// A tab can sit too near the end for its start to be within reach: then the
	// end of the strip shows it, which is as far as the tabs go.
	end := max(0, s.tabsWidth(c, widths)-view)
	desired = min(desired, end)
	switch {
	case s.tabScroll.MaxX <= 0 && end > 0, s.tabScroll.MaxX+1 < end:
		// The strip has not laid the tabs out yet, or has not grown to hold one
		// opened this frame: keep asking, so that the place is set again once it
		// has, and the tab shown is not left cut.
		return false
	}
	desired = min(desired, s.tabScroll.MaxX)
	// It slides there rather than jumping: the tabs moving under a click that
	// chose a tab out of sight is what shows the strip moved at all. Asked again
	// on the way, the glide turns toward the new place.
	if desired != s.tabAsk {
		s.tabAsk = desired
		s.tabScroll.ScrollTo(desired, s.tabScroll.Y, ui.Smooth)
	}
	return true
}

// tabFade fades the ends of the strip that have more tabs beyond them.
func (s *State) tabFade(p *ui.Painter, r ui.Rect, bg ui.Color) {
	const fade float32 = 20
	if s.tabScroll.X > 0 {
		p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: fade, H: r.H},
			ui.LinearGradient{From: bg, To: bg.Alpha(0), Angle: 90}, 0)
	}
	if s.tabScroll.MaxX > 0 && s.tabScroll.X < s.tabScroll.MaxX {
		p.FillGradient(ui.Rect{X: r.X + r.W - fade, Y: r.Y, W: fade, H: r.H},
			ui.LinearGradient{From: bg.Alpha(0), To: bg, Angle: 90}, 0)
	}
}

// tabBar builds the header's row: the tabs of the open documents, scrolling
// sideways when they do not all fit, and the button listing all of them beside
// the strip. A click on a tab shows it, and its close button closes it.
func (s *State) tabBar(c *ui.Context) { s.tabBarAt(c, 0) }

// tabBarAt is tabBar with the strip pulled left by pull DIPs, as the shell's
// title slot pads its content by a gap onevra hardcodes and exposes no option
// for.
func (s *State) tabBarAt(c *ui.Context, pull float32) {
	t := c.Theme()
	row := ui.Row(c).Key("tab-bar").FillWidth().Height(tabBarHeight).
		AlignItems(ui.Center).Gap(t.Space(0.5))
	if len(s.tabs) == 0 {
		return
	}
	// The bar's own width, from the frame before: the shell pads its header for
	// the window's caption buttons, so the content's width is not the room the
	// bar has, and a strip laid out for more room than it has runs under them.
	// The room is the bar's whole box: the padding around the tabs is the
	// shell's, not this row's.
	known := row.Bounds().W > 0
	avail := row.Bounds().W
	if !known {
		// The first frame has no box yet: the content's width is the closest
		// guess, and the frame after measures the bar itself.
		avail = s.windowWidth()
	}
	widths := make([]float32, len(s.tabs))
	for i, tab := range s.tabs {
		widths[i] = tabWidth(c, path.Base(tab.path))
	}
	overflow := s.tabsWidth(c, widths) > avail
	strip := avail
	if overflow {
		strip -= tabOverflow(c) + t.Space(0.5)
	}
	// The strip is scrolled before it is built: a ScrollState set after its
	// container built this frame is a place it never takes.
	// The place is asked for again when the tabs, their room or the tab shown
	// change: closing a tab before the one shown moves it, too.
	total := s.tabsWidth(c, widths)
	if known && (s.tabPending || s.tabShown != s.sel || s.tabView != strip || s.tabTotal != total) {
		s.tabPending = !s.revealTab(c, widths, strip)
		s.tabShown, s.tabView, s.tabTotal = s.sel, strip, total
	}
	row.Children(func() {
		// The strip is pulled to the inset the app asks for, and the list
		// button keeps the shell's trailing one: pulling the whole row would
		// leave that room doubled at the far end.
		ui.Row(c).Grow(1).MinWidth(0).Translate(pull, 0).Children(func() {
			// The strip scrolls sideways rather than dropping tabs: the list
			// beside it is a shortcut to a tab, not the only way to reach one.
			ui.ScrollHorizontal(c).Grow(1).MinWidth(0).TrackScroll(&s.tabScroll).
				// A hairline, and as narrow a place to press: a bar as wide as
				// the theme's would take clicks meant for the tabs' bottoms.
				ScrollbarStyle(ui.ScrollbarStyle{ThumbWidth: 2, TrackWidth: 2, Flush: true}).
				// Where more tabs lie beyond an edge, it fades: a tab cut there
				// reads as one to scroll to, rather than as a stray close button
				// in blank space.
				DrawOver(func(p *ui.Painter, r ui.Rect) { s.tabFade(p, r, t.Background) }).
				// A plain wheel scrolls the strip: it only goes sideways, where
				// the wheel's own axis would scroll the page under it instead.
				HandleInput(func(ev ui.InputEvent) bool {
					if ev.Kind != ui.InputScroll || ev.Mods&ui.Shift != 0 || ev.DY == 0 {
						return false
					}
					// A notch down moves the strip on, as it scrolls a column down.
					// The user takes the strip from here: a place still asked for
					// by a tab chosen a frame ago gives way to their scrolling.
					s.tabPending = false
					s.tabScroll.X -= ev.DY
					return true
				}).Children(func() {
				ui.Row(c).Height(tabBarHeight - 2*t.Space(1)).Gap(t.Space(0.5)).
					AlignItems(ui.Center).Children(func() {
					for i, tab := range s.tabs {
						s.tab(c, tab, widths[i])
					}
				})
			})
		})
		if !overflow {
			return
		}
		button := components.IconButton(c, lucide.ChevronDown(), "全部已打开文件", components.IconButtonPlain)
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
			// The rows keep clear of the bar the list scrolls by, which lies
			// over its right edge: a close button under it takes presses meant
			// for neither.
			ui.Column(c).Gap(t.Space(0.25)).Padding(0, t.Space(3), 0, 0).Children(func() {
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
	// The name's room: the menu, less its padding, the bar the list scrolls by,
	// the close button, and the room inside the button.
	room := tabsMenuWidth - 2*t.Space(1) - t.Space(3) - tabClose(c) - 2*tabPad(c) - tabIcon - t.Space(0.5)
	row.Children(func() {
		name := components.Button(c, "", components.ButtonGhost).Label("打开："+tab.path).
			Padding(t.Space(0.75), tabPad(c))
		name.Grow(1).Children(func() {
			// As a tab of the bar: a row of its own, grown to the button's
			// width, so that a short name still starts at the left.
			ui.Row(c).Grow(1).Gap(t.Space(0.5)).AlignItems(ui.Center).Children(func() {
				components.Icon(c, lucide.FileText()).Size(tabIcon, tabIcon).TextColor(t.TextMuted)
				components.Text(c, fitText(c, path.Base(tab.path), room)).
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
