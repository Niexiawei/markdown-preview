package view

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// loadDoc reads and parses the document the window shows, and points the
// markdown view at its directory, so that its images and links resolve.
func (s *State) loadDoc() {
	s.doc, s.headings, s.src, s.docErr = nil, nil, "", nil
	if s.sel.empty() {
		return
	}
	p, ok := s.SelectedProject()
	if !ok {
		return
	}
	doc, headings, err := s.ws.Parse(p.ID, s.sel.path)
	if err != nil {
		s.docErr = err
		s.status = err.Error()
		return
	}
	src, err := s.ws.ReadMarkdown(p.ID, s.sel.path)
	if err != nil {
		s.docErr = err
		s.status = err.Error()
		return
	}
	s.doc, s.headings, s.src = doc, headings, src
	// Another document is another search: the matches of the last one name
	// lines that no longer mean the same thing.
	s.find.matches, s.find.current, s.find.truncated = nil, 0, false
	s.find.lines = strings.Split(src, "\n")
	if s.find.open && s.find.query != "" {
		s.refreshMatches()
	}
	if abs, err := s.ws.AbsOf(p.ID, s.sel.path); err == nil {
		s.md.LoadImage = markdown.FileImages(filepath.Dir(abs))
	}
	s.md.Extensions = parser.Default
	s.md.OnLink = s.followLink
	// A jump names a block of the document it was asked in: another document
	// opens where its own tab was left.
	s.jump = 0
	s.rev++
	s.loadWebDoc()
}

// gotoHeading brings the heading of slug to the top of the body, and marks the
// outline's entry for it as the current one.
func (s *State) gotoHeading(slug string) {
	slug = strings.TrimPrefix(strings.TrimSpace(slug), "#")
	if slug == "" || s.doc == nil {
		return
	}
	s.anchor = slug
	if s.mode() == config.RenderWebView && s.wv != nil {
		// The page has no block to jump to: webview.go scrolls it by id.
		s.scrollWebTo(slug)
		return
	}
	s.jumpToBlock(blockOfHeading(s.doc, slug))
}

// jumpToBlock brings block b of the document shown to the top of the body's
// pane, gliding there. Where the document ends before the block reaches the
// top, the body stops at its end: the pane keeps the place within what it has.
func (s *State) jumpToBlock(b int) {
	d := s.doc
	if d == nil || b <= 0 || b >= d.Len() {
		return
	}
	// A table shows its rows and cells itself: the table is what scrolls.
	for b > 0 && b < d.Len() {
		k := d.Block(b).Kind
		if k != parser.TableRow && k != parser.TableCell {
			break
		}
		p := d.Block(b).Parent
		if p <= 0 || p >= b {
			return
		}
		b = p
	}
	// A new ask starts without the stamp of the last one's frame, which
	// would otherwise read as "the frame is over" and drop it.
	s.jump, s.jumpSeen = b, time.Time{}
}

// blockOfHeading returns the block of d that holds the heading of slug, 0 when
// the document has none. It is how a heading, which the outline names by its
// slug, becomes the block the body scrolls to.
func blockOfHeading(d *parser.Document, slug string) int {
	if d == nil {
		return 0
	}
	slug = strings.ToLower(strings.TrimPrefix(slug, "#"))
	for i := 1; i < d.Len(); i++ {
		if d.Block(i).Kind != parser.Heading {
			continue
		}
		if markdown.Slug(parser.PlainText(d.Inlines(i))) == slug {
			return i
		}
	}
	return 0
}

// jumpMarker is the markdown view's Block hook: for the block the body is to
// bring to the top it builds a marker of no size at that block's own start,
// which asks the pane to line its top up with the block's, gliding. The block
// itself is the view's to build, so it reports false.
//
// The marker is how a jump lines a block up with the pane's top rather than
// merely showing it: Options.Anchor, which the view follows itself, scrolls as
// little as shows the block, at once.
func (s *State) jumpMarker(c *ui.Context, d *parser.Document, i int) bool {
	if s.jump <= 0 || i != s.jump {
		return false
	}
	s.jumpSeen = c.Now()
	ui.Box(c).Width(0).Height(0).ScrollIntoView(ui.ScrollIntoViewOptions{
		Block:     ui.ScrollStart,
		Animation: &ui.Smooth,
	})
	return false
}

// followLink opens what a link in the document points at: a heading of it, a
// document of the project beside it, or an address in the system.
func (s *State) followLink(url string) {
	url = strings.TrimSpace(url)
	switch {
	case url == "":
		return
	case strings.HasPrefix(url, "#"):
		s.gotoHeading(url)
		return
	case ui.SafeToOpen(url):
		if s.openURL != nil {
			s.openURL(url)
		}
		return
	}
	// A path in the project, relative to the document that links to it.
	target := path.Clean(path.Join(path.Dir(s.sel.path), url))
	if target == "." || target == ".." || strings.HasPrefix(target, "../") || !workspace.IsMarkdown(target) {
		return
	}
	p, ok := s.SelectedProject()
	if !ok {
		return
	}
	if _, err := s.ws.Stat(p.ID, target); err != nil {
		s.status = "链接的文档不存在：" + target
		return
	}
	s.selectDoc(target)
}

// tocNode is one entry of the outline, holding the headings under it.
type tocNode struct {
	h        workspace.Heading
	children []*tocNode
}

// buildTOC turns a document's headings into a tree by their levels, so that
// the outline indents the way the document nests.
func buildTOC(headings []workspace.Heading) []*tocNode {
	var roots []*tocNode
	var stack []*tocNode
	for _, h := range headings {
		n := &tocNode{h: h}
		for len(stack) > 0 && stack[len(stack)-1].h.Level >= h.Level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
		}
		stack = append(stack, n)
	}
	return roots
}

// tocPadding is the room around the outline, in DIPs.
const tocPadding float32 = 8

// tocKey is where the outline's open state lives for a heading of the document
// shown: by project and path first, so that two documents whose headings share
// a name do not unfold each other.
func (s *State) tocKey(slug string) string {
	return "toc:" + s.sel.project + ":" + s.sel.path + ":" + slug
}

// setTOC unfolds or folds every heading of the document shown.
func (s *State) setTOC(open bool) {
	var walk func([]*tocNode)
	walk = func(nodes []*tocNode) {
		for _, n := range nodes {
			if len(n.children) == 0 {
				continue
			}
			b := open
			s.open[s.tocKey(n.h.Slug)] = &b
			walk(n.children)
		}
	}
	walk(buildTOC(s.headings))
}

// toc builds the document's outline, whose entries scroll the body to their
// heading. The outline scrolls both ways, and keeps its rows as wide as the
// pane, so that a click on one lands wherever on it the pointer is.
func (s *State) toc(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(tocPadding).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, "目录").FontSize(13).TextColor(t.TextMuted)
			ui.Spacer(c)
			s.tocActions(c)
		})
		if len(s.headings) == 0 {
			return
		}
		// Both ways: an outline deeper than the pane scrolls down, and a
		// heading wider than it scrolls sideways instead of being cut. The
		// column takes at least the pane's own width, so that a short
		// heading's row still stretches across it.
		ui.ScrollBoth(c).Grow(1).TrackScroll(&s.tocScroll).Children(func() {
			ui.Column(c).MinWidth(max(0, s.tocW-2*tocPadding)).Gap(t.Space(0.25)).Children(func() {
				ui.Tree(c, func() {
					s.tocItems(c, buildTOC(s.headings))
				})
			})
		})
	})
}

// tocActions builds the outline's bulk actions: unfold or fold every heading of
// the document shown, and the body's zoom. The buttons are the plain size rather
// than the compact one: at the outline's density the compact ones read as
// decoration.
func (s *State) tocActions(c *ui.Context) {
	hasHeads := len(s.headings) > 0
	if components.IconButton(c, lucide.UnfoldVertical(), "展开全部目录", components.IconButtonPlain).
		Disabled(!hasHeads).Clicked() {
		s.setTOC(true)
	}
	if components.IconButton(c, lucide.FoldVertical(), "折叠全部目录", components.IconButtonPlain).
		Disabled(!hasHeads).Clicked() {
		s.setTOC(false)
	}
	t := c.Theme()
	if components.IconButton(c, lucide.ZoomOut(), "缩小正文", components.IconButtonPlain).Clicked() {
		s.zoomBy(-zoomStep)
	}
	components.Text(c, itoa(s.zoomPercent())+"%").FontSize(11).TextColor(t.TextMuted)
	if components.IconButton(c, lucide.ZoomIn(), "放大正文", components.IconButtonPlain).Clicked() {
		s.zoomBy(zoomStep)
	}
}

func (s *State) tocItems(c *ui.Context, nodes []*tocNode) {
	for _, n := range nodes {
		n := n
		if len(n.children) > 0 {
			item := ui.TreeItem(c, n.h.Text, s.openOf(s.tocKey(n.h.Slug)), func() {
				s.tocItems(c, n.children)
			}).Selected(n.h.Slug == s.anchor)
			if item.Clicked() {
				s.gotoHeading(n.h.Slug)
			}
			continue
		}
		item := ui.TreeItem(c, n.h.Text, nil, nil).Selected(n.h.Slug == s.anchor)
		if item.Clicked() {
			s.gotoHeading(n.h.Slug)
		}
	}
}

// bodyPadding is the room around a document's body, in DIPs.
const bodyPadding float32 = 28

// bodyJumpGap is the room a jump leaves between the pane's top edge and the
// block it brings there, in DIPs. It is the body's top border, because
// ScrollIntoView lines a block up with the container's border and counts none of
// its padding: a border is the only room it leaves. The padding above is less by
// as much, so that the body at rest looks as it did.
const bodyJumpGap float32 = 16

// nativePreview builds the document's body with the native markdown view.
func (s *State) nativePreview(c *ui.Context) {
	t := c.Theme()
	if s.docErr != nil {
		s.empty(c, s.docErr.Error())
		return
	}
	if s.src == "" {
		s.empty(c, "在左侧选择一个文档")
		return
	}
	// Both ways: a table, a code block or an image wider than the pane scrolls
	// here instead of being cut at its edge. The column takes the pane's own
	// width, so paragraphs wrap, and they re-wrap as the window resizes.
	// The body's text scales with the zoom: every length the theme gives in
	// ems — spacing, code, tables — follows its font size.
	s.md.Theme = &markdown.Theme{FontSize: max(9, t.FontSize*s.zoomOrOne())}
	// The hook wraps every block in a box of its own, so it is hung only on
	// the frames a jump is on its way.
	if s.jump > 0 && !s.jumpSeen.IsZero() && !s.jumpSeen.Equal(c.Now()) {
		// The frame that built the marker is over: what it asked for is on
		// its way, or done.
		s.jump, s.jumpSeen = 0, time.Time{}
	}
	s.md.Block = nil
	if s.jump > 0 {
		s.md.Block = s.jumpMarker
	}
	ui.ScrollBoth(c).TrackScroll(s.docScroll()).Fill().
		Padding(bodyPadding-bodyJumpGap, bodyPadding, bodyPadding, bodyPadding).
		BorderWidth(bodyJumpGap, 0, 0, 0).BorderColor(ui.Transparent).
		Children(func() {
			ui.Column(c).Width(max(200, s.bodyPaneWidth()-2*bodyPadding-2)).Gap(12).Children(func() {
				ui.Text(c, s.sel.path).FontSize(11).TextColor(t.TextMuted)
				markdown.View(c, s.src, &s.md)
			})
		})
}
