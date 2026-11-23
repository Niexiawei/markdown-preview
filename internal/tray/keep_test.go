package tray

import (
	"testing"

	"github.com/egoist/mygo"
)

// 点窗口的关闭按钮只是把窗口藏起来：进程、监听、MCP 都还在。
func TestClosingTheWindowHidesIt(t *testing.T) {
	k := &keep{}
	e := &mygo.CloseEvent{}
	k.close(e)
	if !e.DefaultPrevented() {
		t.Fatal("closing the window went through; it should have been prevented and hidden")
	}
}

// 托盘里的「退出」走 App.Quit → prepareQuit → 每个窗口 close()，这条路径必须放行，
// 否则应用永远退不掉。
func TestQuittingLetsTheWindowClose(t *testing.T) {
	k := &keep{quitting: true}
	e := &mygo.CloseEvent{}
	k.close(e)
	if e.DefaultPrevented() {
		t.Fatal("quitting was blocked by the close handler; the app could never exit")
	}
}
