package view

import (
	"github.com/egoist/mygo/ui"
)

// preview builds the right pane: the document's outline beside its body,
// or what to do when no document is shown.
func (s *State) preview(c *ui.Context) {
	t := c.Theme()
	p, hasProject := s.ActiveProject()
	switch {
	case !hasProject:
		s.empty(c, "打开一个目录开始预览 markdown")
		return
	case s.selected == "" && s.tree != nil && len(s.tree.Children) > 0:
		s.empty(c, "在左侧选择一个文档")
		return
	case s.selected == "":
		s.empty(c, "这个目录下没有 markdown 文件")
		return
	}
	_ = p
	title := ""
	if s.tree != nil {
		title = s.selected
	}
	s.body(c, t, title)
}

// empty centers one line of text in a pane.
func (s *State) empty(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(t.TextMuted)
	})
}

// body shows the outline and the document. The outline and the rendered body
// come in the tasks that follow; until then the path is what shows.
func (s *State) body(c *ui.Context, t *ui.Theme, title string) {
	ui.Split(c, &s.tocW, func() {
		ui.Column(c).Fill().Padding(8).Children(func() {
			ui.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
		})
	}, func() {
		ui.Scroll(c).Fill().Padding(24).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
		})
	}).Fill().SplitReset(240)
}
