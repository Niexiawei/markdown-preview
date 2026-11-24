package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

func TestAMessageIsToastedOnce(t *testing.T) {
	s, _ := newState(t)
	s.notify("已复制文件路径：/tmp/a.md")
	ui.NewTester(s.View, 1000, 700)
	newest := s.Messages()[0].ID
	if s.noticeShown != newest {
		t.Errorf("noticeShown = %d, want %d toasted", s.noticeShown, newest)
	}
	ui.NewTester(s.View, 1000, 700)
	if s.noticeShown != newest {
		t.Errorf("noticeShown = %d, want the message toasted a single time", s.noticeShown)
	}
	s.notifyDone("已复制文件名称：a.md")
	ui.NewTester(s.View, 1000, 700)
	if got := s.Messages()[0].ID; s.noticeShown != got {
		t.Errorf("noticeShown = %d, want the new message %d toasted", s.noticeShown, got)
	}
}

func TestThePageToastsWhatItReports(t *testing.T) {
	s, _ := newState(t)
	s.notifyErr("无法打开文件管理器：boom")
	ui.NewTester(s.Content, 800, 600)
	if got := s.Messages()[0].ID; s.noticeShown != got {
		t.Errorf("noticeShown = %d, want %d toasted by the page", s.noticeShown, got)
	}
}

func TestOldMessagesAreNotToastedOnStartup(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	again := restart(t, s)
	if got := again.Messages()[0].ID; again.noticeShown != got {
		t.Errorf("noticeShown = %d, want the last run's %d to stay history", again.noticeShown, got)
	}
	ui.NewTester(again.View, 1000, 700)
	if got := again.Messages()[0].ID; again.noticeShown != got {
		t.Errorf("noticeShown = %d, want no toast for the last run's messages", again.noticeShown)
	}
}

func TestTheToastKeepsLongMessagesShort(t *testing.T) {
	s, _ := newState(t)
	long := "已复制文件：D:\\" + strings.Repeat("很长的目录\\", 60) + "a.md"
	s.notifyDone(long)
	ui.NewTester(s.View, 1000, 700)
	if got := s.Messages()[0].ID; s.noticeShown != got {
		t.Errorf("noticeShown = %d, want the long message toasted", s.noticeShown)
	}
	if s.Messages()[0].Text != long {
		t.Error("the history lost the long message, want it kept whole")
	}
}

// newToolbar builds the sidebar's own footer, where the bell lives.
func newToolbar(s *State) *ui.Tester { return ui.NewTester(s.LeftToolbar, 280, 40) }

// textIndex is where texts first holds want, or -1.
func textIndex(texts []string, want string) int {
	for i, got := range texts {
		if got == want {
			return i
		}
	}
	return -1
}

func TestTheToolbarShowsTheBell(t *testing.T) {
	s, _ := newState(t)
	if tt := newToolbar(s); !tt.HasText("消息") {
		t.Errorf("texts = %q, want the bell", tt.Texts())
	}
}

func TestTheBellOpensAndClosesTheHistory(t *testing.T) {
	s, _ := newState(t)
	if err := newToolbar(s).Click("消息"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !s.noticeOpen {
		t.Error("noticeOpen = false, want the history open")
	}
	if err := newToolbar(s).Click("消息"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.noticeOpen {
		t.Error("noticeOpen = true, want the history closed")
	}
}

func TestTheBadgeCountsTheUnreadOnes(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	tt := newToolbar(s)
	if textIndex(tt.Texts(), "2") < 0 {
		t.Errorf("texts = %q, want the badge to count two", tt.Texts())
	}
	if !tt.HasText("消息（2 条未读）") {
		t.Errorf("texts = %q, want the count in the bell's name too", tt.Texts())
	}
}

func TestNoBadgeWithoutUnread(t *testing.T) {
	s, _ := newState(t)
	tt := newToolbar(s)
	if textIndex(tt.Texts(), "0") >= 0 {
		t.Errorf("texts = %q, want no badge", tt.Texts())
	}
	if !tt.HasText("消息") {
		t.Errorf("texts = %q, want the bell", tt.Texts())
	}
}

func TestTheBadgeTextNeverGrows(t *testing.T) {
	for _, c := range []struct {
		unread int
		want   string
	}{
		{0, ""},
		{1, "1"},
		{99, "99"},
		{100, "99+"},
		{12345, "99+"},
	} {
		if got := noticeBadgeText(c.unread); got != c.want {
			t.Errorf("noticeBadgeText(%d) = %q, want %q", c.unread, got, c.want)
		}
	}
}

func TestTheBadgeIsCappedAt99(t *testing.T) {
	s, _ := newState(t)
	for range 120 {
		s.notify("消息")
	}
	if tt := newToolbar(s); textIndex(tt.Texts(), "99+") < 0 {
		t.Errorf("texts = %q, want the badge capped", tt.Texts())
	}
}

func TestOpeningTheHistoryMarksThemSeen(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	if err := newToolbar(s).Click("消息（2 条未读）"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.notes.Unseen(); got != 0 {
		t.Errorf("Unseen = %d, want the history read", got)
	}
	tt := newToolbar(s)
	if !tt.HasText("消息") || tt.HasText("消息（2 条未读）") {
		t.Errorf("texts = %q, want the count gone", tt.Texts())
	}
}

func TestUnreadSurvivesARestart(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	again := restart(t, s)
	if got := again.notes.Unseen(); got != 2 {
		t.Errorf("Unseen = %d, want the two kept", got)
	}
	if tt := newToolbar(again); !tt.HasText("消息（2 条未读）") {
		t.Errorf("texts = %q, want the count kept", tt.Texts())
	}
}

func TestTheHistoryListsTheNewestFirst(t *testing.T) {
	s, _ := newState(t)
	s.notify("旧的")
	s.notify("新的")
	s.noticeOpen = true
	texts := newTester(s).Texts()
	old, fresh := textIndex(texts, "旧的"), textIndex(texts, "新的")
	if old < 0 || fresh < 0 {
		t.Fatalf("texts = %q, want both messages listed", texts)
	}
	if fresh > old {
		t.Errorf("texts = %q, want the newest message first", texts)
	}
}

func TestLongMessagesWrapInTheHistory(t *testing.T) {
	s, _ := newState(t)
	long := "已复制文件：" + strings.Repeat("很长的目录\\", 60) + "a.md"
	s.notifyDone(long)
	s.noticeOpen = true
	if tt := newTester(s); !tt.HasText(long) {
		t.Errorf("texts = %q, want the whole message in the history", tt.Texts())
	}
	if got := s.Messages()[0].Text; got != long {
		t.Errorf("Text = %q, want the whole message kept", got)
	}
}

func TestTheHistoryDeletesOneMessage(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	s.notify("三")
	s.noticeOpen = true
	if err := newTester(s).Click("删除消息：二"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := len(s.Messages()); got != 2 {
		t.Errorf("Messages = %d, want the one removed", got)
	}
	if !s.noticeOpen {
		t.Error("noticeOpen = false, want the history still open")
	}
	if got := len(restart(t, s).Messages()); got != 2 {
		t.Errorf("after a restart: %d messages, want the removal saved", got)
	}
}

func TestDeletingAMessageKeepsTheRestInOrder(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notify("二")
	s.notify("三")
	s.noticeOpen = true
	if err := newTester(s).Click("删除消息：二"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	got := s.Messages()
	if len(got) != 2 || got[0].Text != "三" || got[1].Text != "一" {
		t.Errorf("Messages = %v, want 三, 一", got)
	}
}

func TestTheHistoryClearsEveryMessage(t *testing.T) {
	s, _ := newState(t)
	s.notify("一")
	s.notifyDone("二")
	s.noticeOpen = true
	tt := newTester(s)
	if err := tt.Click("清空全部"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := len(s.Messages()); got != 0 {
		t.Errorf("Messages = %d, want none", got)
	}
	texts := newTester(s).Texts()
	if !newTester(s).HasText("暂无消息") {
		t.Errorf("texts = %q, want the empty history to say so", texts)
	}
}

func TestTheHistorySaysWhenItIsEmpty(t *testing.T) {
	s, _ := newState(t)
	s.noticeOpen = true
	tt := newTester(s)
	if !tt.HasText("暂无消息") {
		t.Errorf("texts = %q, want the empty history to say so", tt.Texts())
	}
	if tt.HasText("清空全部") {
		t.Errorf("texts = %q, want no clear button over nothing", tt.Texts())
	}
}

func TestMessageTimesReadAcrossDays(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 11, 24, 9, 41, 0, 0, time.UTC), "09:41"},
		{time.Date(2026, 11, 23, 9, 41, 0, 0, time.UTC), "11-23 09:41"},
		{time.Date(2025, 11, 23, 9, 41, 0, 0, time.UTC), "2025-11-23 09:41"},
	} {
		if got := noticeTime(c.at, now); got != c.want {
			t.Errorf("noticeTime(%v) = %q, want %q", c.at, got, c.want)
		}
	}
}

// TestToastsSitAtTheBottomEnd pins where a message pops up: the bottom right
// corner, out of the way of the outline and of the body's first lines.
func TestToastsSitAtTheBottomEnd(t *testing.T) {
	if got := ToastOptions().Placement; got != components.ToastBottomEnd {
		t.Errorf("placement = %v, want the bottom right", got)
	}
}

// TestAMessageArrivingInTheOpenHistoryIsRead pins that the panel showing the
// history is the history read: a message that lands while it is open never
// raises the badge, which the user would see over a list already showing it.
func TestAMessageArrivingInTheOpenHistoryIsRead(t *testing.T) {
	s, _ := newState(t)
	s.noticeOpen = true
	s.notify("一")
	tt := newToolbar(s)
	if got := s.notes.Unseen(); got != 0 {
		t.Errorf("Unseen = %d, want it read as it lands", got)
	}
	if textIndex(tt.Texts(), "1") >= 0 || tt.HasText("消息（1 条未读）") {
		t.Errorf("texts = %q, want no badge over the open history", tt.Texts())
	}
}
