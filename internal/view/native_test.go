package view

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

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

// sections is a document of n sections, each a heading "第 i 节" and a dozen
// lines that start with "P<i> 正文": the heading is what a test clicks in the
// outline, and the line is where the body shows it. A dozen lines a section
// makes the body several screens tall while the outline keeps only n rows, so
// that every heading stays within reach of a click.
func sections(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## 第 %d 节\n\n", i)
		for line := 1; line <= 12; line++ {
			fmt.Fprintf(&b, "P%d 正文，第 %d 行\n\n", i, line)
		}
	}
	return b.String()
}

// TestTOCClickSetsTheBlockToJumpTo pins what a click on the outline hands the
// body: the heading named, and the block the body is to bring to its top.
func TestTOCClickSetsTheBlockToJumpTo(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# 一\n\n## 二\n\n正文一\n")
	s.Refresh()
	s.selectDoc("a.md")
	// Only the outline: the body would build the block and take the jump.
	tt := ui.NewTester(func(c *ui.Context) { s.toc(c) }, 240, 400)
	if err := tt.Click("二"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.anchor != "二" {
		t.Errorf("anchor = %q, want 二", s.anchor)
	}
	want := blockOfHeading(s.doc, "二")
	if want == 0 {
		t.Fatal("blockOfHeading found no block for the heading")
	}
	if s.jump != want {
		t.Errorf("jump = %d, want the heading's block %d", s.jump, want)
	}
}

// TestTOCJumpPutsTheHeadingAtTheTop pins the place the outline brings a
// chapter to: the pane's top, not wherever it first shows.
func TestTOCJumpPutsTheHeadingAtTheTop(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if s.docScroll().Y <= 0 {
		t.Fatalf("Y = %v, want the body scrolled", s.docScroll().Y)
	}
	rect, ok := tt.Find("P6 正文，第 1 行")
	if !ok {
		t.Fatalf("texts = %q, want the body under the heading", tt.Texts())
	}
	if rect.Y > 200 {
		t.Errorf("body at Y=%v, want it near the pane's top", rect.Y)
	}
}

// TestTOCJumpNearTheEndLandsAtTheBottom pins the other end: a chapter the
// document cannot lift to the top stops with the body at its end, rather than
// partway.
func TestTOCJumpNearTheEndLandsAtTheBottom(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 8 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	sc := s.docScroll()
	if sc.MaxY <= 0 {
		t.Fatalf("MaxY = %v, want a body that scrolls", sc.MaxY)
	}
	if sc.Y < sc.MaxY-0.5 {
		t.Errorf("Y = %v, want the body at its end, %v", sc.Y, sc.MaxY)
	}
	if _, ok := tt.Find("P8 正文，第 1 行"); !ok {
		t.Errorf("texts = %q, want the last section still shown", tt.Texts())
	}
}

// TestTOCJumpGlides pins that the body glides there rather than jumping: the
// click leaves it where it was, and a frame after asks for another.
func TestTOCJumpGlides(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.docScroll().Y; got != 0 {
		t.Errorf("Y = %v right after the click, want the body still at its top", got)
	}
	if !tt.FramePending() {
		t.Error("no frame was asked for: the jump did not start gliding")
	}
	tt.Advance(time.Second)
	tt.Frame()
	at := s.docScroll().Y
	if at <= 0 {
		t.Fatalf("Y = %v after a second, want the body where the heading is", at)
	}
	// Where it went, it stays: the glide is all but over, not still creeping.
	tt.Advance(time.Second)
	tt.Frame()
	if got := s.docScroll().Y; math.Abs(float64(got-at)) > 1 {
		t.Errorf("Y = %v a second later, want %v", got, at)
	}
}

// TestJumpWithoutMotionWhenAsked pins the desktop's choice: asked for less
// motion, the body is there in the frame of the click.
func TestJumpWithoutMotionWhenAsked(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTesterWith(s.View, 1000, 700, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true},
	})
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	at := s.docScroll().Y
	if at <= 0 {
		t.Fatalf("Y = %v, want the body there at once", at)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if got := s.docScroll().Y; got != at {
		t.Errorf("Y = %v after a second, want %v: it should not have glided", got, at)
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
	if s.anchor != "install" {
		t.Errorf("anchor = %q, want install", s.anchor)
	}
	if want := blockOfHeading(s.doc, "install"); want == 0 || s.jump != want {
		t.Errorf("jump = %d, want the heading's block %d", s.jump, want)
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
