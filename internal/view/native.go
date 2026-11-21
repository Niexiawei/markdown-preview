package view

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/workspace"
)

// loadDoc reads and parses the document the window shows, and points the
// markdown view at its directory, so that its images and links resolve.
func (s *State) loadDoc() {
	s.doc, s.headings, s.src, s.docErr = nil, nil, "", nil
	if s.selected == "" {
		return
	}
	p, ok := s.ActiveProject()
	if !ok {
		return
	}
	doc, headings, err := s.ws.Parse(p.ID, s.selected)
	if err != nil {
		s.docErr = err
		s.status = err.Error()
		return
	}
	src, err := s.ws.ReadMarkdown(p.ID, s.selected)
	if err != nil {
		s.docErr = err
		s.status = err.Error()
		return
	}
	s.doc, s.headings, s.src = doc, headings, src
	if abs, err := s.ws.AbsOf(p.ID, s.selected); err == nil {
		s.md.LoadImage = markdown.FileImages(filepath.Dir(abs))
	}
	s.md.Extensions = parser.Default
	s.md.OnLink = s.followLink
	s.md.Anchor = s.anchor
	s.md.AnchorVersion = s.anchorVer
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
	target := path.Clean(path.Join(path.Dir(s.selected), url))
	if target == "." || target == ".." || strings.HasPrefix(target, "../") || !workspace.IsMarkdown(target) {
		return
	}
	p, ok := s.ActiveProject()
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

// toc builds the document's outline, whose entries scroll the body to their
// heading.
func (s *State) toc(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(8).Children(func() {
		ui.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
		if len(s.headings) == 0 {
			return
		}
		ui.Tree(c, func() {
			s.tocItems(c, buildTOC(s.headings))
		})
	})
}

func (s *State) tocItems(c *ui.Context, nodes []*tocNode) {
	for _, n := range nodes {
		n := n
		if len(n.children) > 0 {
			item := ui.TreeItem(c, n.h.Text, s.openOf("toc:"+n.h.Slug), func() {
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
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(28).Gap(12).MaxWidth(820).Children(func() {
			ui.Text(c, s.selected).FontSize(11).TextColor(t.TextMuted)
			markdown.View(c, s.src, &s.md)
		})
	})
}
