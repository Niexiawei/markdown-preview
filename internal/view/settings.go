package view

import (
	"github.com/egoist/mygo/ui"
)

// mcpButtonLabel is the toolbar button that opens the MCP settings.
const mcpButtonLabel = "MCP"

// SetMCPControl tells the window who turns the local MCP server on and off.
// The app provides it, since only it can start and stop the server.
func (s *State) SetMCPControl(fn func(on bool)) { s.mcpControl = fn }

// SetMCPStatus tells the window where the MCP server listens, and with which
// token, once it started or stopped.
func (s *State) SetMCPStatus(enabled bool, addr, token string) {
	s.mcpEnabled, s.mcpAddr, s.mcpToken = enabled, addr, token
}

// setMCP asks the app to turn the server on or off.
func (s *State) setMCP(on bool) {
	if s.mcpControl == nil {
		return
	}
	s.mcpControl(on)
}

// mcpSettings builds the button and the panel that shows the local MCP
// server: whether it runs, where, and with which token.
func (s *State) mcpSettings(c *ui.Context) {
	t := c.Theme()
	button := ui.Button(c, mcpButtonLabel)
	if button.Clicked() {
		s.mcpOpen = !s.mcpOpen
	}
	ui.Popover(c, button, &s.mcpOpen, func() {
		ui.Column(c).Width(360).Padding(14).Gap(10).Children(func() {
			ui.Text(c, "本地 MCP server").FontSize(13).Bold()
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				on := s.mcpEnabled
				ui.Text(c, "允许 agent 读取文档").FontSize(13).Grow(1)
				if ui.Switch(c, &on).Label("MCP server").Changed() {
					s.setMCP(on)
				}
			})
			if !s.mcpEnabled {
				ui.Text(c, "关闭中。打开后，agent 可以列出和检索当前打开的项目的 markdown 文档。").
					FontSize(12).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, "地址").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, s.mcpAddr).FontSize(12)
			ui.Text(c, "Token").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, trimEllipsis(s.mcpToken, 60)).FontSize(12)
			if ui.Button(c, "复制 token").Clicked() {
				c.Services().WriteClipboard(s.mcpToken)
				s.status = "token 已复制"
			}
			ui.Text(c, "在 agent 的 MCP 配置里填上面的地址，请求头带 Authorization: Bearer <token>。").
				FontSize(12).TextColor(t.TextMuted)
		})
	})
}
