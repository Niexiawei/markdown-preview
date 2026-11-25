package view

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// The macOS arrangement sinks the project panel into the sidebar's body and
// leaves the shell's header slot to the collapse toggle. The tests below set
// panelDown explicitly instead of trusting GOOS, so they pass on every
// platform.

func TestThePanelSinksIntoTheSidebarBodyOnMacOS(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	s.Refresh()
	s.panelDown = true
	tt := newTester(s)
	if !tt.HasText("切换项目") {
		t.Fatalf("sunk panel missing; texts = %v", tt.Texts())
	}
	panel, ok := tt.Find("切换项目")
	if !ok {
		t.Fatal("no 切换项目 button")
	}
	doc, ok := tt.Find("notes.md")
	if !ok {
		t.Fatalf("no tree row; texts = %v", tt.Texts())
	}
	if panel.Y+panel.H > doc.Y {
		t.Fatalf("panel row %v overlaps tree row %v", panel, doc)
	}
	if err := tt.Click("切换项目"); err != nil {
		t.Fatal(err)
	}
	if !s.projectsOpen {
		t.Fatal("project list did not open from the sunk row")
	}
}

func TestTheHeaderSlotStaysEmptyWhenThePanelSinks(t *testing.T) {
	s, _ := newState(t)
	s.panelDown = true
	tt := ui.NewTester(func(c *ui.Context) { s.LeftHeader(c) }, 280, 48)
	for _, want := range []string{"切换项目", "展开全部目录树"} {
		if tt.HasText(want) {
			t.Fatalf("header slot still builds the panel: texts = %v", tt.Texts())
		}
	}
}

func TestThePanelStaysInTheHeaderOutsideMacOS(t *testing.T) {
	s, _ := newState(t)
	s.panelDown = false
	header := ui.NewTester(func(c *ui.Context) { s.LeftHeader(c) }, 280, 48)
	if !header.HasText("切换项目") {
		t.Fatalf("header slot lost the panel; texts = %v", header.Texts())
	}
	body := ui.NewTester(func(c *ui.Context) { s.Left(c) }, 280, 600)
	if body.HasText("切换项目") {
		t.Fatalf("body grew a panel of its own; texts = %v", body.Texts())
	}
}

func TestTheSunkPanelFitsANarrowSidebar(t *testing.T) {
	s, _ := newState(t)
	s.panelDown = true
	tt := ui.NewTester(func(c *ui.Context) { s.Left(c) }, 200, 600)
	if !tt.HasText("切换项目") {
		t.Fatalf("sunk panel missing at 200 DIP; texts = %v", tt.Texts())
	}
	bulk, ok := tt.Find("展开全部目录树")
	if !ok {
		t.Fatal("no bulk action")
	}
	btn, ok := tt.Find("切换项目")
	if !ok {
		t.Fatal("no switcher")
	}
	if bulk.X+bulk.W > btn.X || btn.X+btn.W > 200 {
		t.Fatalf("panel overflows the sidebar: bulk %v, button %v", bulk, btn)
	}
}

func TestPanelDownFollowsTheOverride(t *testing.T) {
	t.Setenv("MDPREVIEW_PANEL_DOWN", "1")
	cfg := config.Config{Version: config.Version}
	s := New(workspace.New(cfg), cfg, t.TempDir())
	if !s.PanelDown() {
		t.Fatal("MDPREVIEW_PANEL_DOWN=1 did not sink the panel")
	}
}
