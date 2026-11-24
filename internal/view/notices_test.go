package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oneo-me/onevra/source/components"

	"mdpreview/internal/notices"
	"mdpreview/internal/workspace"
)

// restart builds the window again over the same user data directory, as the
// app does the next time it starts.
func restart(t *testing.T, s *State) *State {
	t.Helper()
	return New(workspace.New(s.cfg), s.cfg, s.cfgDir)
}

func TestTheNewestMessageIsTheStatus(t *testing.T) {
	s, _ := newState(t)
	if got := s.Status(); got != "" {
		t.Errorf("Status = %q, want nothing reported yet", got)
	}
	s.notify("一")
	s.notify("二")
	if got := s.Status(); got != "二" {
		t.Errorf("Status = %q, want the newest message", got)
	}
}

func TestMessagesKeepTheNewestFirst(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	s.notify("三")
	got := s.Messages()
	if len(got) != 3 {
		t.Fatalf("Messages = %v, want three", got)
	}
	if got[0].Text != "三" || got[2].Text != "一" {
		t.Errorf("Messages = %v, %v, %v, want 三, 二, 一", got[0].Text, got[1].Text, got[2].Text)
	}
}

func TestEmptyMessagesAreNotRecorded(t *testing.T) {
	s, _ := newState(t)
	s.notify("")
	if got := len(s.Messages()); got != 0 {
		t.Errorf("Messages = %d, want the empty text dropped", got)
	}
}

func TestErrorsAreMarkedAsSuch(t *testing.T) {
	s, _ := newState(t)
	s.notifyErr("坏了")
	s.notifyDone("好了")
	s.notifyWarn("小心")
	s.notify("知道了")
	want := []string{
		components.ToastInfo,
		components.ToastWarning,
		components.ToastSuccess,
		components.ToastError,
	}
	got := s.Messages()
	if len(got) != len(want) {
		t.Fatalf("Messages = %v, want %d", got, len(want))
	}
	for i, kind := range want {
		if got[i].Kind != kind {
			t.Errorf("Messages[%d].Kind = %q, want %q", i, got[i].Kind, kind)
		}
	}
}

func TestMessagesSurviveARestart(t *testing.T) {
	s, _ := newState(t)
	s.notifyDone("已复制文件")
	s.notifyErr("无法打开文件管理器")
	again := restart(t, s)
	if got := again.Status(); got != "无法打开文件管理器" {
		t.Errorf("Status = %q, want the newest message kept", got)
	}
	if got := len(again.Messages()); got != 2 {
		t.Errorf("Messages = %d, want both kept", got)
	}
}

func TestRemovingOneMessage(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	s.notify("三")
	middle := s.Messages()[1]
	s.RemoveMessage(middle.ID)
	got := s.Messages()
	if len(got) != 2 || got[0].Text != "三" || got[1].Text != "一" {
		t.Errorf("Messages = %v, want 三, 一", got)
	}
	if got := len(restart(t, s).Messages()); got != 2 {
		t.Errorf("after a restart: %d messages, want the removal saved", got)
	}
}

func TestClearingEveryMessage(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	s.ClearMessages()
	if got := len(s.Messages()); got != 0 {
		t.Errorf("Messages = %d, want none", got)
	}
	if got := s.Status(); got != "" {
		t.Errorf("Status = %q, want none", got)
	}
	if got := len(restart(t, s).Messages()); got != 0 {
		t.Errorf("after a restart: %d messages, want the clear saved", got)
	}
}

func TestAMessageWritesThroughToTheFile(t *testing.T) {
	s, _ := newState(t)
	s.notify("已复制文件名称：a.md")
	if _, err := os.Stat(filepath.Join(s.cfgDir, notices.FileName)); err != nil {
		t.Errorf("Stat notices.json: %v, want the message written through", err)
	}
}
