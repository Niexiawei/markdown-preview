package view

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

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
	if abs, err := s.ws.AbsOf(p.ID, s.sel.path); err == nil {
		s.md.LoadImage = markdown.FileImages(filepath.Dir(abs))
	}
	s.md.Extensions = parser.Default
	s.md.OnLink = s.followLink
	s.md.Anchor = s.anchor
	s.md.AnchorVersion = s.anchorVer
	s.rev++
	s.loadWebDoc()
}

// gotoHeading scrolls the document to the heading of slug.
func (s *State) gotoHeading(slug string) {
	slug = strings.TrimPrefix(strings.TrimSpace(slug), "#")
	if slug == "" {
		return
	}
	s.anchor = slug
	s.anchorVer++
	s.md.Anchor = slug
	s.md.AnchorVersion = s.anchorVer
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
			components.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
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
// the document shown. Without headings there is nothing to fold.
func (s *State) tocActions(c *ui.Context) {
	hasHeads := len(s.headings) > 0
	if components.IconButton(c, lucide.UnfoldVertical(), "展开全部目录", components.IconButtonCompact).
		Disabled(!hasHeads).Clicked() {
		s.setTOC(true)
	}
	if components.IconButton(c, lucide.FoldVertical(), "折叠全部目录", components.IconButtonCompact).
		Disabled(!hasHeads).Clicked() {
		s.setTOC(false)
	}
	t := c.Theme()
	if components.IconButton(c, lucide.ZoomOut(), "缩小正文", components.IconButtonCompact).Clicked() {
		s.zoomBy(-zoomStep)
	}
	components.Text(c, itoa(s.zoomPercent())+"%").FontSize(10).TextColor(t.TextMuted)
	if components.IconButton(c, lucide.ZoomIn(), "放大正文", components.IconButtonCompact).Clicked() {
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
	ui.ScrollBoth(c).TrackScroll(s.docScroll()).Fill().Padding(bodyPadding).Children(func() {
		ui.Column(c).Width(max(200, s.bodyPaneWidth()-2*bodyPadding-2)).Gap(12).Children(func() {
			ui.Text(c, s.sel.path).FontSize(11).TextColor(t.TextMuted)
			markdown.View(c, s.src, &s.md)
		})
	})
}
