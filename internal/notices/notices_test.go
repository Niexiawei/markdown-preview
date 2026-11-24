package notices

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// moment is when a message was reported: a fixed time, so that a saved message
// reads back exactly as it was written.
func moment(hour, minute int) time.Time {
	return time.Date(2026, 11, 24, hour, minute, 0, 0, time.UTC)
}

// load opens the store in dir, failing the test when it cannot.
func load(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%q): %v", dir, err)
	}
	return s
}

// add records one message, failing the test when it cannot be saved.
func add(t *testing.T, s *Store, kind, text string, at time.Time) Message {
	t.Helper()
	m, err := s.Add(kind, text, at)
	if err != nil {
		t.Fatalf("Add(%q): %v", text, err)
	}
	return m
}

func TestAnEmptyStoreHasNoMessages(t *testing.T) {
	s := load(t, t.TempDir())
	if len(s.Messages()) != 0 {
		t.Errorf("Messages = %v, want none", s.Messages())
	}
	if s.Last() != 0 {
		t.Errorf("Last = %d, want 0", s.Last())
	}
	if s.Unseen() != 0 {
		t.Errorf("Unseen = %d, want 0", s.Unseen())
	}
}

func TestNewMessagesComeFirst(t *testing.T) {
	s := load(t, t.TempDir())
	add(t, s, "info", "一", moment(9, 0))
	add(t, s, "info", "二", moment(9, 1))
	add(t, s, "info", "三", moment(9, 2))
	got := s.Messages()
	if len(got) != 3 || got[0].Text != "三" || got[2].Text != "一" {
		t.Errorf("Messages = %v, want 三, 二, 一", got)
	}
}

func TestMessagesSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	s := load(t, dir)
	add(t, s, "success", "已复制文件", moment(9, 41))
	add(t, s, "error", "无法打开文件管理器", moment(9, 42))

	again := load(t, dir)
	got := again.Messages()
	if len(got) != 2 {
		t.Fatalf("Messages = %v, want the two that were saved", got)
	}
	if got[0].Text != "无法打开文件管理器" || got[0].Kind != "error" {
		t.Errorf("Messages[0] = %+v, want the newest error first", got[0])
	}
	if !got[1].At.Equal(moment(9, 41)) {
		t.Errorf("At = %v, want %v", got[1].At, moment(9, 41))
	}
	if got[0].Seen || got[1].Seen {
		t.Errorf("Seen = %v, %v, want both unseen", got[0].Seen, got[1].Seen)
	}
}

func TestMessagesHaveNoLimit(t *testing.T) {
	dir := t.TempDir()
	s := load(t, dir)
	for i := range 250 {
		add(t, s, "info", "消息", moment(9, i%60))
	}
	if len(s.Messages()) != 250 {
		t.Fatalf("Messages = %d, want 250 kept", len(s.Messages()))
	}
	if len(load(t, dir).Messages()) != 250 {
		t.Errorf("after a restart: %d, want 250 kept", len(load(t, dir).Messages()))
	}
}

func TestIDsKeepGrowingPastAClear(t *testing.T) {
	s := load(t, t.TempDir())
	last := add(t, s, "info", "一", moment(9, 0)).ID
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	next := add(t, s, "info", "二", moment(9, 1)).ID
	if next <= last {
		t.Errorf("ID = %d, want past the cleared %d", next, last)
	}
	if s.Last() != next {
		t.Errorf("Last = %d, want %d", s.Last(), next)
	}
}

func TestRemovingOneMessage(t *testing.T) {
	dir := t.TempDir()
	s := load(t, dir)
	add(t, s, "info", "一", moment(9, 0))
	middle := add(t, s, "info", "二", moment(9, 1))
	add(t, s, "info", "三", moment(9, 2))
	if err := s.Remove(middle.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got := s.Messages()
	if len(got) != 2 || got[0].Text != "三" || got[1].Text != "一" {
		t.Errorf("Messages = %v, want 三, 一", got)
	}
	if err := s.Remove(999); err != nil {
		t.Errorf("Remove(999): %v, want it ignored", err)
	}
	if len(s.Messages()) != 2 {
		t.Errorf("Messages = %v, want the unknown id to leave the two", s.Messages())
	}
	if len(load(t, dir).Messages()) != 2 {
		t.Errorf("after a restart: %v, want the removal saved", load(t, dir).Messages())
	}
}

func TestClearingEveryMessage(t *testing.T) {
	dir := t.TempDir()
	s := load(t, dir)
	add(t, s, "info", "一", moment(9, 0))
	add(t, s, "info", "二", moment(9, 1))
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if len(s.Messages()) != 0 {
		t.Errorf("Messages = %v, want none", s.Messages())
	}
	if len(load(t, dir).Messages()) != 0 {
		t.Errorf("after a restart: %v, want the clear saved", load(t, dir).Messages())
	}
}

func TestUnseenCountsWhatWasNotSeen(t *testing.T) {
	s := load(t, t.TempDir())
	add(t, s, "info", "一", moment(9, 0))
	add(t, s, "info", "二", moment(9, 1))
	add(t, s, "info", "三", moment(9, 2))
	if got := s.Unseen(); got != 3 {
		t.Errorf("Unseen = %d, want 3", got)
	}
}

func TestMarkSeenMakesThemAllSeen(t *testing.T) {
	dir := t.TempDir()
	s := load(t, dir)
	add(t, s, "info", "一", moment(9, 0))
	add(t, s, "info", "二", moment(9, 1))
	if err := s.MarkSeen(); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	if got := s.Unseen(); got != 0 {
		t.Errorf("Unseen = %d, want 0", got)
	}
	if got := load(t, dir).Unseen(); got != 0 {
		t.Errorf("after a restart: Unseen = %d, want 0", got)
	}
}

func TestAnEmptyTextIsNotAdded(t *testing.T) {
	s := load(t, t.TempDir())
	if _, err := s.Add("info", "", moment(9, 0)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(s.Messages()) != 0 {
		t.Errorf("Messages = %v, want the empty text dropped", s.Messages())
	}
}

func TestAMemoryOnlyStoreKeepsWorking(t *testing.T) {
	s := load(t, "")
	m := add(t, s, "info", "一", moment(9, 0))
	if len(s.Messages()) != 1 || s.Last() != m.ID {
		t.Errorf("Messages = %v, Last = %d, want the message in memory", s.Messages(), s.Last())
	}
	if err := s.Save(); err != nil {
		t.Errorf("Save: %v, want a store without a directory to save nothing", err)
	}
}

func TestABrokenFileLeavesAnEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err == nil {
		t.Error("Load = nil error, want the broken file reported")
	}
	if len(s.Messages()) != 0 {
		t.Errorf("Messages = %v, want none from a broken file", s.Messages())
	}
}
