package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAppThemeUsesPalette(t *testing.T) {
	var got *ui.Theme
	ui.NewTester(func(c *ui.Context) {
		got = AppTheme(c)
	}, 100, 100)
	if got == nil {
		t.Fatal("AppTheme = nil")
	}
	if got.Accent != ui.Hex(accentHex) {
		t.Errorf("Accent = %v, want %s", got.Accent, accentHex)
	}
	if got.AccentHover != ui.Hex(accentHoverHex) {
		t.Errorf("AccentHover = %v, want %s", got.AccentHover, accentHoverHex)
	}
	if got.AccentPressed != ui.Hex(accentPressedHex) {
		t.Errorf("AccentPressed = %v, want %s", got.AccentPressed, accentPressedHex)
	}
	if got.Warning != ui.Hex(warningHex) {
		t.Errorf("Warning = %v, want %s", got.Warning, warningHex)
	}
	if got.Text == (ui.Color{}) || got.Background == (ui.Color{}) {
		t.Error("theme lost Onevra's text or background")
	}
}

// TestShellSlotsBuild builds each region the sidebar shell asks for, so that a
// panic in the layout the shell uses shows up without a window.
func TestShellSlotsBuild(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n\n## 二\n\n正文\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	slots := []struct {
		name  string
		build func(c *ui.Context)
		w, h  int
		want  []string
	}{
		{"Left", s.Left, 280, 600, []string{"notes", "a.md"}},
		{"LeftHeader", s.LeftHeader, 280, 40, []string{"切换项目", "alpha"}},
		{"LeftToolbar", s.LeftToolbar, 280, 40, []string{"打开项目", "刷新", "MCP"}},
		{"ContentTitle", s.ContentTitle, 800, 40, []string{"alpha/notes/a.md"}},
		{"Content", s.Content, 800, 600, []string{"目录", "正文"}},
	}
	for _, slot := range slots {
		slot := slot
		build := slot.build
		tt := ui.NewTester(func(c *ui.Context) { build(c) }, slot.w, slot.h)
		for _, want := range slot.want {
			if !tt.HasText(want) {
				t.Errorf("%s texts = %q, want %q", slot.name, tt.Texts(), want)
			}
		}
	}
}
