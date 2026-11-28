package view

import (
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
)

// preview builds the right pane: the document's outline beside its body, or
// what to do when no document is shown.
func (s *State) preview(c *ui.Context) {
	switch {
	case len(s.trees) == 0:
		s.empty(c, "打开一个目录或 Markdown 文件开始预览")
		return
	case s.sel.empty() && s.hasDocs():
		s.empty(c, "在左侧选择一个文档")
		return
	case s.sel.empty():
		s.empty(c, "这个目录下没有 markdown 文件")
		return
	}
	body := func() { s.nativePreview(c) }
	if s.mode() == config.RenderWebView {
		body = func() { s.webPreview(c) }
	}
	ui.Split(c, &s.tocW, func() { s.toc(c) }, body).
		Grow(1).SplitReset(defaultTOCWidth)
	s.findPanel(c)
}

// empty centers one line of text in a pane.
func (s *State) empty(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(t.TextMuted)
	})
}
