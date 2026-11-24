package view

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// The floating panel's size, in DIPs, and how many lines of context its preview
// shows on either side of a match.
const (
	findPaneWidth float32 = 420
	// findListHeight is how tall the matches' island is until its band is
	// dragged; findListMin and findListMax are how short and how tall a drag
	// may leave it.
	findListHeight float32 = 132
	findListMin    float32 = 72
	findListMax    float32 = 320
	// The preview's island, likewise: findPreviewMin is its caption and a
	// couple of lines, and findPreviewMax is as much context as is worth
	// showing at once.
	findPreviewHeight float32 = 148
	findPreviewMin    float32 = 96
	findPreviewMax    float32 = 420
	// findGrip is the band under each island that drags it taller or shorter,
	// in DIPs. It is wider than the room between the islands was, since it has
	// to be caught with a pointer.
	findGrip    float32 = 10
	findContext int     = 2
	// findIslandPad is the room inside each pane of the panel, in DIPs.
	findIslandPad float32 = 6
	// findPaneInner is how wide what sits in a pane is: the panel less a
	// pane's room on either side and its border.
	findPaneInner = findPaneWidth - 2*findIslandPad - 2
)

// findBand names which of the panel's two data islands a band resizes.
type findBand int

const (
	findListBand findBand = iota
	findPreviewBand
)

// findBandLabel is what a band is called for the reader and for the tests.
func findBandLabel(which findBand) string {
	if which == findPreviewBand {
		return "调整命中上下文高度"
	}
	return "调整匹配列表高度"
}

// findBandMin is the shortest a band's island may be left.
func findBandMin(which findBand) float32 {
	if which == findPreviewBand {
		return findPreviewMin
	}
	return findListMin
}

// findBandMax is the tallest a band's island may be dragged on its own, before
// the window has its say.
func findBandMax(which findBand) float32 {
	if which == findPreviewBand {
		return findPreviewMax
	}
	return findListMax
}

// findBandDefault is the height a band's island shows at until it is dragged.
func findBandDefault(which findBand) float32 {
	if which == findPreviewBand {
		return findPreviewHeight
	}
	return findListHeight
}

// findState is the window's search over the document shown: whether the panel
// floats, the phrase, where it matches, and which match the body shows.
type findState struct {
	open      bool
	needFocus bool
	query     string
	matches   []workspace.Match
	current   int // 0-based into matches
	truncated bool
	// listH and previewH are how tall the two data islands are, as the user
	// dragged them: 0 until then, and for as long as the window lives
	// (findHeightOf reads them, within what the panel allows).
	listH, previewH float32
	// shownList and shownPreview are how tall they show this frame, which the
	// window may leave less than the user asked for.
	shownList, shownPreview float32
	// fieldH is the field's island as the last frame laid it out: the room the
	// two islands have is measured from it.
	fieldH float32
	// listOver and previewOver are how far past a limit the pointer dragged a
	// band, so that coming back moves it at once rather than once the pointer is
	// over it again, as ui.Split keeps its own (splitOvershoot).
	listOver, previewOver float32
	// lines are the source's lines, kept from the last load: the preview
	// reads them every frame, and splitting the source per frame is the kind
	// of work the window's thread must not do.
	lines []string
}

// findable reports whether there is a document to search.
func (s *State) findable() bool { return !s.sel.empty() && s.src != "" }

// findListHeightOf returns how tall the matches' island is: the height the user
// dragged it to, within what the panel allows, and the default until its band is
// dragged.
func (s *State) findListHeightOf() float32 { return s.findHeightOf(findListBand) }

// findPreviewHeightOf is findListHeightOf for the preview's island.
func (s *State) findPreviewHeightOf() float32 { return s.findHeightOf(findPreviewBand) }

// findHeightOf returns how tall one of the panel's data islands is: the height
// the user dragged it to, within what the panel allows, and the default until
// its band is dragged.
func (s *State) findHeightOf(which findBand) float32 {
	h := s.find.listH
	if which == findPreviewBand {
		h = s.find.previewH
	}
	if h == 0 {
		return findBandDefault(which)
	}
	return min(max(h, findBandMin(which)), findBandMax(which))
}

// setFindHeight asks for one island to be h tall, within its own floor and the
// ceiling the window leaves it. The ceiling is passed in rather than measured
// here: only a build may ask the theme or the shell how much room there is.
func (s *State) setFindHeight(which findBand, h, room float32) {
	h = min(max(h, findBandMin(which)), room)
	if which == findPreviewBand {
		s.find.previewH = h
		return
	}
	s.find.listH = h
}

// findHeightMaxOf returns how tall one island may be: its own ceiling, or what
// the window has left once the other island is placed, whichever is less. The
// room the other takes is what it shows, not what it asked for: in a window too
// short for both, dragging one must not shrink the other to its floor.
func (s *State) findHeightMaxOf(c *ui.Context, which findBand) float32 {
	other := s.find.shownPreview
	if which == findPreviewBand {
		other = s.find.shownList
	}
	if other == 0 {
		// Before the panel has laid the two of them out once.
		if which == findPreviewBand {
			other = s.findListHeightOf()
		} else {
			other = s.findPreviewHeightOf()
		}
	}
	return max(min(findBandMax(which), s.findRoom(c)-other), findBandMin(which))
}

// findTop is where the panel starts, below the header which holds the tabs and
// below the title row the page shows first.
func (s *State) findTop(c *ui.Context) float32 {
	return sidebar.HeaderHeight(c) + titleRowHeight + c.Theme().Space(2)
}

// findRoom is what the panel's data islands and their bands may take: the
// window's height, less the room above the panel, the room below it, the field's
// island and the bands.
func (s *State) findRoom(c *ui.Context) float32 {
	room := s.windowHeight() - s.findTop(c) - c.Theme().Space(2) - s.find.fieldH - 2*findGrip
	return max(room, findListMin+findPreviewMin)
}

// fitFindHeights decides how tall the two islands show: the heights the user
// asked for, less what the window cannot hold. The preview gives way first, since
// it is the context around a hit rather than the hits themselves; what was asked
// for is kept, so that a window made taller again shows it.
func (s *State) fitFindHeights(c *ui.Context) {
	room := s.findRoom(c)
	list, preview := s.findListHeightOf(), s.findPreviewHeightOf()
	if over := list + preview - room; over > 0 {
		preview = max(preview-over, min(findPreviewMin, preview))
		if over = list + preview - room; over > 0 {
			list = max(list-over, min(findListMin, list))
		}
	}
	s.find.shownList, s.find.shownPreview = list, preview
}

// openFind shows the panel over the document shown, over what was searched for
// before: searching for the same phrase again is the common case.
func (s *State) openFind() {
	if !s.findable() {
		return
	}
	// One panel at a time holds the keys, and the document's search is the one
	// the body answers to: the tree's folds away, keeping its phrase.
	s.treeFocused, s.treeSearch.open = false, false
	s.find.open = true
	// The field takes the keyboard as the panel opens, even when the last
	// phrase is still in it: the user asked to search, not to read.
	s.find.needFocus = true
	if s.find.query != "" {
		s.refreshMatches()
	}
}

// closeFind hides the panel, and clears the page's highlights with it.
func (s *State) closeFind() {
	s.find.open = false
	if s.wv != nil {
		s.wv.Page().StopFindInPage()
	}
}

// refreshMatches recomputes where the phrase appears in the document shown. It
// runs when the phrase changes and when the document does, not every frame.
func (s *State) refreshMatches() {
	s.find.matches, s.find.truncated = workspace.FindIn(s.src, s.find.query)
	s.find.current = min(s.find.current, max(0, len(s.find.matches)-1))
}

// findNext moves to the match n places on, round the ends, showing it in the
// body.
func (s *State) findNext(n int) {
	total := len(s.find.matches)
	if total == 0 {
		return
	}
	s.gotoMatch(((s.find.current+n)%total + total) % total)
}

// gotoMatch shows the match at i in the body: the native view scrolls to the
// block holding it, the web view to the same place in its page.
func (s *State) gotoMatch(i int) {
	if i < 0 || i >= len(s.find.matches) {
		return
	}
	s.find.current = i
	if s.mode() == config.RenderWebView {
		s.findInWeb()
		return
	}
	if s.doc == nil {
		return
	}
	// The block, not the match: a scroll is all the native view does, and
	// one block is as close as it gets.
	s.jumpToBlock(blockAt(s.doc, s.find.matches[i].Offset))
}

// findInWeb highlights the phrase in the page and scrolls to the match chosen:
// the web view's own find does both. The page counts its own matches, and markup
// can split one, so its count is not always the source's: the walk stops at
// whichever of the two runs out first.
func (s *State) findInWeb() {
	if s.wv == nil {
		s.notifyErr("WebView 不可用，未在页面内高亮")
		return
	}
	res, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{})
	if err != nil {
		s.notifyErr("无法在页面中查找：" + err.Error())
		return
	}
	for n := 1; n <= min(s.find.current, res.Matches-1); n++ {
		if _, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{FindNext: true}); err != nil {
			s.notifyErr("无法在页面中查找：" + err.Error())
			return
		}
	}
}

// blockAt returns the deepest block of d whose source holds the byte offset at.
func blockAt(d *parser.Document, at int) int {
	if d.Len() <= 1 {
		return 0
	}
	best := 1
	for i := 1; i < d.Len(); i++ {
		b := d.Block(i)
		if b.Start <= at && at < b.End && b.Start >= d.Block(best).Start {
			best = i
		}
	}
	return best
}

// findShortcuts handles the panel's keys: Escape empties the field before it
// closes the panel, and Cmd+G walks the matches. Cmd+F is the one the two
// searches share, and searchShortcuts routes it.
func (s *State) findShortcuts(c *ui.Context) {
	switch {
	case s.find.open && c.Shortcut(0, ui.KeyEscape):
		if s.find.query != "" {
			s.find.query = ""
			s.refreshMatches()
			return
		}
		s.closeFind()
	case s.find.open && c.Shortcut(ui.Cmd, ui.KeyG):
		if c.Modifiers()&ui.Shift != 0 {
			s.findNext(-1)
			return
		}
		s.findNext(1)
	}
}

// findPanel floats the search over the body's top right, as an editor's find bar
// does: the field, how many matches there are, the lines they sit on with the
// one chosen mixed in, and the lines around it to see it in context.
//
// Each of the three is a pane of its own (findIsland), apart from the others and
// from the page: in one card, the last row of the matches and the match's own
// line read as one run of rows.
func (s *State) findPanel(c *ui.Context) {
	if !s.find.open || s.sel.empty() {
		return
	}
	t := c.Theme()
	// What the window has room for, before any island asks for its height.
	s.fitFindHeights(c)
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().
			Right(t.Space(2)).
			Top(s.findTop(c)).
			Width(findPaneWidth).
			Children(func() {
				s.findField(c)
				s.findList(c)
				s.findGripBand(c, findListBand)
				s.findPreview(c)
				s.findGripBand(c, findPreviewBand)
			})
	})
}

// findIsland fills one pane of the search panel: a face of its own, rounded,
// bordered and lifted off the page. The room inside is the content's own, so
// that a pane whose content scrolls keeps the padding for its rows and leaves
// the face to the bars (findScrollPane).
func findIsland(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).FillWidth().Radius(t.Radius*1.5).
		Background(t.Surface).Border(1, t.Border).
		Shadow(0, t.Space(0.5), t.Space(1.5), 0, t.Text.Alpha(0.10))
}

// findScrollPane fills an island with a scroll whose bars lie against the
// island's border rather than inside its padding: a press near the edge drags
// the bar instead of landing on a row under the thumb. The padding is the
// rows' own, inside the scroll, so what they show is what they showed.
func findScrollPane(c *ui.Context, label string, build func()) {
	ui.ScrollBoth(c).Grow(1).Label(label).Children(func() {
		ui.Column(c).MinWidth(findPaneInner + 2*findIslandPad).Gap(c.Theme().Space(0.25)).
			Padding(findIslandPad).Children(build)
	})
}

// findCaption names what a pane of the panel holds.
func findCaption(c *ui.Context, text string) {
	components.Text(c, text).FontSize(10).TextColor(c.Theme().TextMuted)
}

// findField builds the panel's first row: the field, the count, and the buttons
// walking the matches or closing the panel.
func (s *State) findField(c *ui.Context) {
	t := c.Theme()
	island := findIsland(c).Label("搜索输入").Padding(findIslandPad)
	island.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0.5)).Children(func() {
			field := ui.SearchField(c, &s.find.query).Label("在文档中查找")
			field.Grow(1).FontSize(12)
			if s.find.needFocus {
				// AutoFocus gives the focus once, where Focus would take it back
				// every frame, from the list or the buttons beside the field.
				field.AutoFocus()
				s.find.needFocus = false
			}
			switch {
			case field.Changed():
				s.refreshMatches()
			case field.Submitted():
				s.findNext(1)
			}
			components.Text(c, s.findCount()).FontSize(11).TextColor(t.TextMuted)
			if components.IconButton(c, lucide.ArrowUp(), "上一个匹配", components.IconButtonCompact).Clicked() {
				s.findNext(-1)
			}
			if components.IconButton(c, lucide.ArrowDown(), "下一个匹配", components.IconButtonCompact).Clicked() {
				s.findNext(1)
			}
			if components.IconButton(c, lucide.X(), "关闭搜索", components.IconButtonCompact).Clicked() {
				s.closeFind()
			}
		})
	})
	// The island as the last frame laid it out: the room the two data islands
	// have is measured from it, and the field's height is the theme's, not the
	// view's to count.
	s.find.fieldH = island.Bounds().H
}

// findCount is the panel's "3/17": which match the body shows, of how many.
func (s *State) findCount() string {
	switch {
	case s.find.query == "":
		return ""
	case len(s.find.matches) == 0:
		return "无匹配"
	}
	count := strconv.Itoa(s.find.current+1) + "/" + strconv.Itoa(len(s.find.matches))
	if s.find.truncated {
		// Where the body is, and that the list stops before the document
		// does: the count alone would claim the document has 500 matches.
		return count + "（前 " + strconv.Itoa(len(s.find.matches)) + " 处）"
	}
	return count
}

// findList builds the matches: the number of the line and its text, the one the
// body shows tinted, a click showing another.
func (s *State) findList(c *ui.Context) {
	t := c.Theme()
	if len(s.find.matches) == 0 {
		return
	}
	findIsland(c).Label("匹配行").Height(s.find.shownList).Margin(t.Space(1.5), 0, 0, 0).Children(func() {
		findScrollPane(c, "匹配行列表", func() {
			for i, m := range s.find.matches {
				i, m := i, m
				row := ui.Row(c).Radius(t.Radius).AlignItems(ui.Center).
					Padding(t.Space(0.25), t.Space(0.5)).Gap(t.Space(0.75))
				if i == s.find.current {
					row.Background(selectedRowTint(t))
				}
				row.Children(func() {
					components.Text(c, strconv.Itoa(m.Line)).FontSize(10).
						TextColor(t.TextMuted).Width(t.Rem(2.5))
					components.Text(c, trimEllipsis(m.Text, 120)).FontSize(11).SingleLine()
				})
				if row.Clicked() {
					s.gotoMatch(i)
				}
			}
		})
	})
}

// findGripBand is the band under one of the panel's data islands: the pointer
// drags it to resize the island above it, the arrows move it once it has the
// keyboard, and a double click puts it back to its default. It is drawn only
// while the pointer is over it or dragging it, so that the panel stays three
// islands rather than one card with a rule through it.
//
// Dragged reports what the pointer moved since the frame before, not since the
// press: adding it to the height each frame is what makes the island follow the
// pointer. The overshoot kept beside the height is what makes coming back from a
// limit move the island at once. Without matches there is nothing to resize, so
// the bands are left out with them.
func (s *State) findGripBand(c *ui.Context, which findBand) {
	if len(s.find.matches) == 0 {
		return
	}
	t := c.Theme()
	band := ui.Box(c).FillWidth().Height(findGrip).Focusable().
		Cursor(ui.CursorResizeNS).Role(ui.RoleSplitter).Label(findBandLabel(which))
	over := &s.find.listOver
	if which == findPreviewBand {
		over = &s.find.previewOver
	}
	// What the window leaves this island, read now: the band's input comes
	// before the next build, where no theme or shell is at hand.
	room := s.findHeightMaxOf(c, which)
	// A press here takes the keyboard, so that the arrows that follow move the
	// edge rather than leaving the band with nothing to tab to.
	if band.Pressed() {
		band.Focus()
	}
	band.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown {
			return false
		}
		step := t.Space(2.5)
		switch ev.Key {
		case ui.KeyUp:
			s.setFindHeight(which, s.findHeightOf(which)-step, room)
		case ui.KeyDown:
			s.setFindHeight(which, s.findHeightOf(which)+step, room)
		default:
			return false
		}
		return true
	})
	_, dy, held := band.Dragged()
	switch {
	case !held:
		// The press ended, or never came: the next drag starts with the band
		// under the pointer.
		*over = 0
	case dy != 0:
		// A press that moves nothing is no drag: it must not pull the island
		// down to what the window shows of it.
		want := s.findHeightOf(which) + dy + *over
		s.setFindHeight(which, want, room)
		*over = want - s.findHeightOf(which)
	}
	if band.DoubleClicked() {
		s.setFindHeight(which, findBandDefault(which), room)
		*over = 0
	}
	band.Draw(func(p *ui.Painter, r ui.Rect) {
		if !held && !band.Hovered() {
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2, W: r.W, H: 1}, t.Accent, 0)
	})
}

// findPreview builds the lines around the match the body shows, the match itself
// picked out, so that a hit is recognized before the body is read. Its pane
// names the line it shows: told apart from the matches by the name alone, not by
// the colour of a wash.
func (s *State) findPreview(c *ui.Context) {
	t := c.Theme()
	if len(s.find.matches) == 0 {
		return
	}
	m := s.find.matches[s.find.current]
	lines := s.find.lines
	from := max(1, m.Line-findContext)
	to := min(len(lines), m.Line+findContext)
	findIsland(c).Label("命中上下文").Height(s.find.shownPreview).Children(func() {
		ui.Row(c).Padding(findIslandPad, findIslandPad, t.Space(0.25), findIslandPad).Children(func() {
			findCaption(c, "第 "+strconv.Itoa(m.Line)+" 行上下文")
		})
		findScrollPane(c, "命中上下文列表", func() {
			for n := from; n <= to; n++ {
				text := strings.TrimSuffix(lines[n-1], "")
				row := ui.Row(c).AlignItems(ui.Center).Gap(t.Space(0.75))
				if n == m.Line {
					row.Background(t.Accent.Alpha(0.08))
				}
				row.Children(func() {
					components.Text(c, strconv.Itoa(n)).FontSize(10).
						TextColor(t.TextMuted).Width(t.Rem(2.5))
					s.markedLine(c, text, n == m.Line)
				})
			}
		})
	})
}

// markedLine builds one line of the preview, the phrase picked out on the line of
// the match by the accent and a wash of it behind.
func (s *State) markedLine(c *ui.Context, text string, mark bool) {
	t := c.Theme()
	size := max(10, t.FontSize-2)
	style := ui.Span{Text: text, Font: t.Font, Size: size, Color: t.Text}
	if !mark {
		ui.RichText(c, style).SingleLine().FontSize(size)
		return
	}
	// The phrase on this line, as the source holds it: the same comparison the
	// matches came from, so that what is picked out is what was searched for.
	at := -1
	needle := []rune(s.find.query)
	runes := []rune(text)
	for i := 0; i+len(needle) <= len(runes) && len(needle) > 0; i++ {
		if strings.EqualFold(string(runes[i:i+len(needle)]), s.find.query) {
			at = i
			break
		}
	}
	if at < 0 {
		ui.RichText(c, style).SingleLine().FontSize(size)
		return
	}
	hit := ui.Span{Text: string(runes[at : at+len(needle)]), Font: t.Font, Size: size,
		Color: t.Accent, Background: t.Accent.Alpha(0.18)}
	ui.RichText(c,
		ui.Span{Text: string(runes[:at]), Font: t.Font, Size: size, Color: t.Text},
		hit,
		ui.Span{Text: string(runes[at+len(needle):]), Font: t.Font, Size: size, Color: t.Text},
	).SingleLine().FontSize(size)
}
