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

// findAnchorPrefix marks an anchor that names a search hit's block rather than
// a heading's slug: the markdown view scrolls to the block Options.AnchorBlock
// returns for it.
const findAnchorPrefix = "find:"

// The floating panel's size, in DIPs, and how many lines of context its preview
// shows on either side of a match.
const (
	findPaneWidth   float32 = 420
	findListHeight  float32 = 132
	findPreviewSize float32 = 148
	findContext     int     = 2
)

// findState is the window's search over the document shown: whether the panel
// floats, the phrase, where it matches, and which match the body shows.
type findState struct {
	open      bool
	needFocus bool
	query     string
	matches   []workspace.Match
	current   int // 0-based into matches
	truncated bool
	// lines are the source's lines, kept from the last load: the preview
	// reads them every frame, and splitting the source per frame is the kind
	// of work the window's thread must not do.
	lines []string
}

// findable reports whether there is a document to search.
func (s *State) findable() bool { return !s.sel.empty() && s.src != "" }

// openFind shows the panel over the document shown, over what was searched for
// before: searching for the same phrase again is the common case.
func (s *State) openFind() {
	if !s.findable() {
		return
	}
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
	// The anchor is the block, not the match: a scroll is all the native
	// view does, and one block is as close as it gets.
	block := blockAt(s.doc, s.find.matches[i].Offset)
	if block <= 0 {
		return
	}
	s.anchor = findAnchorPrefix + strconv.Itoa(block)
	s.anchorVer++
	s.md.Anchor, s.md.AnchorVersion = s.anchor, s.anchorVer
}

// findInWeb highlights the phrase in the page and scrolls to the match chosen:
// the web view's own find does both. The page counts its own matches, and markup
// can split one, so its count is not always the source's: the walk stops at
// whichever of the two runs out first.
func (s *State) findInWeb() {
	if s.wv == nil {
		s.status = "WebView 不可用，未在页面内高亮"
		return
	}
	res, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{})
	if err != nil {
		s.status = "无法在页面中查找：" + err.Error()
		return
	}
	for n := 1; n <= min(s.find.current, res.Matches-1); n++ {
		if _, err := s.wv.Page().FindInPage(s.find.query, mygo.FindOptions{FindNext: true}); err != nil {
			s.status = "无法在页面中查找：" + err.Error()
			return
		}
	}
}

// anchorBlock is the markdown view's block lookup: a find anchor names the block
// to scroll to, and anything else is a heading's slug, which the view finds
// itself, so -1 hands it back.
func (s *State) anchorBlock(d *parser.Document, name string) int {
	rest, ok := strings.CutPrefix(name, findAnchorPrefix)
	if !ok {
		return -1
	}
	i, err := strconv.Atoi(rest)
	if err != nil || i <= 0 || i >= d.Len() {
		return -1
	}
	return i
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

// findShortcuts handles the panel's keys: Cmd+F opens it, Escape empties the
// field before it closes the panel, and Cmd+G walks the matches.
func (s *State) findShortcuts(c *ui.Context) {
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyF):
		s.openFind()
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
func (s *State) findPanel(c *ui.Context) {
	if !s.find.open || s.sel.empty() {
		return
	}
	t := c.Theme()
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().
			Right(t.Space(2)).
			Top(sidebar.HeaderHeight(c)+tabBarHeight+t.Space(2)).
			Width(findPaneWidth).
			Radius(t.Radius*1.5).Background(t.Surface).
			Border(1, t.Border).
			Shadow(0, 8, 24, 0, t.Text.Alpha(0.12)).
			Padding(t.Space(1)).Gap(t.Space(1)).Children(func() {
			s.findField(c)
			s.findList(c)
			s.findPreview(c)
		})
	})
}

// findField builds the panel's first row: the field, the count, and the buttons
// walking the matches or closing the panel.
func (s *State) findField(c *ui.Context) {
	t := c.Theme()
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
	width := findPaneWidth - 2*t.Space(1)
	ui.ScrollBoth(c).MaxHeight(findListHeight).Children(func() {
		ui.Column(c).MinWidth(width).Gap(t.Space(0.25)).Children(func() {
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

// findPreview builds the lines around the match the body shows, the match itself
// picked out, so that a hit is recognized before the body is read.
func (s *State) findPreview(c *ui.Context) {
	t := c.Theme()
	if len(s.find.matches) == 0 {
		return
	}
	m := s.find.matches[s.find.current]
	lines := s.find.lines
	from := max(1, m.Line-findContext)
	to := min(len(lines), m.Line+findContext)
	// The context sits in a recessed card: without it the last list row and
	// the match's own line read as one run of rows.
	width := findPaneWidth - 2*t.Space(1)
	ui.Column(c).Padding(t.Space(0.5)).Radius(t.Radius).
		Background(t.Background).Children(func() {
		ui.ScrollBoth(c).MaxHeight(findPreviewSize).Children(func() {
			ui.Column(c).MinWidth(width).Gap(t.Space(0.25)).Children(func() {
				for n := from; n <= to; n++ {
					text := strings.TrimSuffix(lines[n-1], "\r")
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
