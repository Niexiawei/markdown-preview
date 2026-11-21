package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTOCListsHeadings(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n### 三\n\n正文一\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := ui.NewTester(s.View, 1000, 700)
	for _, want := range []string{"目录", "一", "二", "三", "正文一"} {
		if !tt.HasText(want) {
			t.Errorf("texts = %q, want %q", tt.Texts(), want)
		}
	}
}

func TestTOCClickSetsAnchor(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文一\n")
	s.Refresh()
	s.selectDoc("a.md")
	tt := ui.NewTester(s.View, 1000, 700)
	if err := tt.Click("二"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.anchor != "二" {
		t.Errorf("anchor = %q, want 二", s.anchor)
	}
	if s.anchorVer != 1 {
		t.Errorf("anchorVer = %d, want 1", s.anchorVer)
	}
	if s.md.Anchor != "二" || s.md.AnchorVersion != 1 {
		t.Errorf("md options = %q/%d, want 二/1", s.md.Anchor, s.md.AnchorVersion)
	}
}

func TestEmptyTOCShowsBody(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "plain.md", "没有标题的正文\n")
	s.Refresh()
	s.selectDoc("plain.md")
	tt := ui.NewTester(s.View, 1000, 700)
	if !tt.HasText("目录") {
		t.Errorf("texts = %q, want the outline heading", tt.Texts())
	}
	if !tt.HasText("没有标题的正文") {
		t.Errorf("texts = %q, want the body", tt.Texts())
	}
}

func TestFollowLinkOpensSiblingDoc(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n[b](sub/b.md)\n")
	writeDoc(t, root, "sub/b.md", "# B\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.followLink("sub/b.md")
	if got := s.Selected(); got != "sub/b.md" {
		t.Errorf("Selected = %q, want sub/b.md", got)
	}
	if s.headings[0].Text != "B" {
		t.Errorf("headings = %v, want B", s.headings)
	}
}

func TestFollowLinkResolvesRelativePath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	writeDoc(t, root, "sub/b.md", "# B\n\n[up](../a.md)\n")
	s.Refresh()
	s.selectDoc("sub/b.md")
	s.followLink("../a.md")
	if got := s.Selected(); got != "a.md" {
		t.Errorf("Selected = %q, want a.md", got)
	}
}

func TestFollowLinkRefusesEscape(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.followLink("../../etc/passwd.md")
	if got := s.Selected(); got != "a.md" {
		t.Errorf("Selected = %q, want a.md to stay", got)
	}
}

func TestFollowLinkAnchor(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n\n## Install\n")
	s.Refresh()
	s.selectDoc("a.md")
	s.followLink("#install")
	if s.anchor != "install" || s.anchorVer != 1 {
		t.Errorf("anchor = %q/%d, want install/1", s.anchor, s.anchorVer)
	}
}

func TestDocChangeReloadsHeadingOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# Old\n")
	writeDoc(t, root, "b.md", "# New\n\n## Child\n")
	s.Refresh()
	s.selectDoc("a.md")
	if len(s.headings) != 1 || s.headings[0].Text != "Old" {
		t.Fatalf("headings = %v, want Old", s.headings)
	}
	s.selectDoc("b.md")
	if len(s.headings) != 2 || s.headings[1].Text != "Child" {
		t.Errorf("headings = %v, want Old gone and Child shown", s.headings)
	}
}
