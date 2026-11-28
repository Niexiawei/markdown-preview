package view

import (
	"testing"
	"time"

	"mdpreview/internal/session"
	"mdpreview/internal/workspace"
)

func TestTheSidebarWidthSurvivesARestart(t *testing.T) {
	s, _ := newState(t)
	if got := s.SidebarWidth(); got != DefaultSidebarWidth {
		t.Errorf("SidebarWidth = %v, want the default %v", got, DefaultSidebarWidth)
	}
	s.SetSidebarWidth(func() float32 { return 312 })
	tt := newTester(s)
	tt.Frame()
	tt.Advance(time.Second) // 停手满静默期
	tt.Frame()

	again := New(workspace.New(s.cfg), s.cfg, s.cfgDir)
	if got := again.SidebarWidth(); got != 312 {
		t.Errorf("after a restart SidebarWidth = %v, want 312", got)
	}
}

func TestAWidthUnderThePointerIsWrittenOnceItStops(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## 甲\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	tt.Frame()

	s.tocW = 300
	tt.Frame()
	tt.Advance(100 * time.Millisecond) // 还在动（没到静默期）
	tt.Frame()
	if got := readSession(t, s).TOC; len(got) != 0 {
		t.Errorf("toc widths = %+v, want nothing written yet", got)
	}
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if got := readSession(t, s).TOC; len(got) != 1 || got[0].Width != 300 {
		t.Errorf("toc widths = %+v, want the one 300", got)
	}
}

func TestTheOutlineWidthComesBackWithItsDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## 甲\n")
	writeDoc(t, root, "b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := newTester(s)
	s.tocW = 320
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	s.selectDoc("b.md")
	if got := s.tocW; got != defaultTOCWidth {
		t.Errorf("tocW = %v on the other document, want %v", got, defaultTOCWidth)
	}
	s.selectDoc("a.md")
	if got := s.tocW; got != 320 {
		t.Errorf("tocW = %v back on a.md, want 320", got)
	}
}

func TestAnOutlineWidthBelongsToItsProject(t *testing.T) {
	s, alpha, beta := twoProjects(t)
	writeDoc(t, alpha, "docs/a.md", "# A\n")
	writeDoc(t, beta, "docs/a.md", "# A\n")
	s.Refresh()
	s.selectDocIn(s.Trees()[0].project.ID, "docs/a.md")
	tt := newTester(s)
	s.tocW = 300
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()

	s.selectDocIn(s.Trees()[1].project.ID, "docs/a.md")
	if got := s.tocW; got != defaultTOCWidth {
		t.Errorf("tocW = %v in the other project, want %v", got, defaultTOCWidth)
	}
}

// readSession reads what the window wrote, for the tests about writing.
func readSession(t *testing.T, s *State) session.Session {
	t.Helper()
	sess, err := session.Load(s.cfgDir)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	return sess
}
