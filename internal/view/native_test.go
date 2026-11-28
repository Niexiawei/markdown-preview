package view

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
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

// TestTOCJumpWithoutAWebViewFallsBackToTheBody pins the degraded path: in
// webview mode without a page (headless tests, a machine without one) the body
// shown is the native one, so an outline click still brings the chapter to the
// top there.
func TestTOCJumpWithoutAWebViewFallsBackToTheBody(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	s.setMode(config.RenderWebView)
	if s.wv != nil {
		t.Fatal("a web view was made without a window")
	}
	s.gotoHeading("第-6-节")
	if want := blockOfHeading(s.doc, "第-6-节"); want == 0 || s.jump != want {
		t.Errorf("jump = %d, want the heading's block %d", s.jump, want)
	}
}

// TestJumpAfterAnotherDocumentStillJumps pins that a jump asked for later is not
// dropped by the frame stamp of an earlier one: switching documents, and coming
// back to the outline, still glides the body to the chapter clicked.
func TestJumpAfterAnotherDocumentStillJumps(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	writeDoc(t, root, "b.md", "另一个文档\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	first := s.docScroll().Y
	if first <= 0 {
		t.Fatalf("Y = %v, want the first jump to have scrolled", first)
	}

	s.selectDoc("b.md")
	s.selectDoc("a.md")
	tt.Frame()
	if err := tt.Click("第 2 节"); err != nil {
		t.Fatalf("Click after coming back: %v", err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if got := s.docScroll().Y; got >= first-100 {
		t.Errorf("Y = %v after jumping to the second section, want well above %v", got, first)
	}
}

// TestJumpAskResetsTheFrameStamp pins the stamp's meaning: it says when the
// current ask was built into a marker, so a new ask must start without one,
// as after another document replaced the one an earlier ask was made in.
func TestJumpAskResetsTheFrameStamp(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")
	s.jumpSeen = time.Unix(1, 0) // as the frame that built a marker left it
	s.jumpToBlock(blockOfHeading(s.doc, "第-2-节"))
	if s.jump == 0 {
		t.Fatal("no jump was asked for")
	}
	if !s.jumpSeen.IsZero() {
		t.Errorf("jumpSeen = %v, want a new ask to start without a stamp", s.jumpSeen)
	}
}

// TestTOCJumpLeavesRoomAboveTheHeading pins the room a jump leaves: the chapter
// lands 16 DIP under the pane's top edge, not flush against it, so that the
// heading reads as a heading rather than as the first line of the page.
func TestTOCJumpLeavesRoomAboveTheHeading(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	// The body alone, so that the heading's text names its own box.
	tt := ui.NewTester(func(c *ui.Context) { s.nativePreview(c) }, 700, 600)
	tt.Frame()
	s.gotoHeading("第-6-节")
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	rect, ok := tt.Find("第 6 节")
	if !ok {
		t.Fatalf("texts = %q, want the heading", tt.Texts())
	}
	if rect.W <= 0 || rect.H <= 0 {
		t.Fatalf("heading at %v is clipped: the jump left it out of the pane", rect)
	}
	if rect.Y < 12 || rect.Y > 20 {
		t.Errorf("heading at Y=%v, want the 16 DIP a jump leaves above it", rect.Y)
	}
}

// hrs is a document whose sections are parted by rules: a rule's margins stack
// with the room the markdown view leaves between blocks once it wraps them, so
// this is the shape that a jump must still land right on.
func hrs(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## 第 %d 节\n\n", i)
		for line := 1; line <= 8; line++ {
			fmt.Fprintf(&b, "P%d 正文，第 %d 行。\n\n", i, line)
		}
		b.WriteString("---\n\n")
	}
	return b.String()
}

// TestTOCJumpLandsRightAfterRules pins the landing where the document parts its
// sections with rules: the room the view leaves between blocks with a wrapper is
// not the room it leaves without one, so a jump measured in one and spent in the
// other lands off — the heading hidden above the pane.
func TestTOCJumpLandsRightAfterRules(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", hrs(12))
	s.Refresh()
	s.selectDoc("a.md")

	tt := ui.NewTester(func(c *ui.Context) { s.nativePreview(c) }, 700, 600)
	tt.Frame()
	s.gotoHeading("第-8-节")
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	rect, ok := tt.Find("第 8 节")
	if !ok {
		t.Fatalf("texts = %q, want the heading", tt.Texts())
	}
	if rect.W <= 0 || rect.H <= 0 {
		t.Fatalf("heading at %v is clipped: the jump left it out of the pane", rect)
	}
	if rect.Y < 12 || rect.Y > 24 {
		t.Errorf("heading at Y=%v, want the 16 DIP a jump leaves above it", rect.Y)
	}
}

// TestTheOutlineFollowsTheBodyScroll pins the follow: the section at the top
// of the body is the outline's current one, without a click.
func TestTheOutlineFollowsTheBodyScroll(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Frame() // 一帧之后标记与正文容器才有位置
	pane := s.bodyBox
	if pane.H == 0 || len(s.headMarks) < 5 {
		t.Fatalf("pane %v, marks %d: the body did not lay out", pane, len(s.headMarks))
	}
	// 把第 5 节的标记顶到窗格顶边（bodyJumpGap 之下）。
	s.docScroll().Y = s.headMarks[4].y - (pane.Y + bodyJumpGap) + 1
	tt.Frame()
	tt.Frame()

	if want := workspace.Slug("第 5 节"); s.anchor != want {
		t.Errorf("anchor = %q, want %q", s.anchor, want)
	}
}

func TestScrollingBackToTheTopLeavesTheFirstHeadingCurrent(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Frame()
	s.docScroll().Y = 2000
	tt.Frame()
	tt.Frame()
	first := workspace.Slug("第 1 节")
	if s.anchor == first || s.anchor == "" {
		t.Fatalf("anchor = %q, want a later section while scrolled", s.anchor)
	}
	s.docScroll().Y = 0
	tt.Frame()
	tt.Frame()
	if want := workspace.Slug("第 1 节"); s.anchor != want {
		t.Errorf("anchor = %q back at the top, want the follow to take over, %q", s.anchor, want)
	}
}

// TestAJumpKeepsItsHighlightWhileItGlides pins the hold: the frames between
// the click and the landing must not repaint the outline with the sections the
// body is passing. It also pins that the follow resumes once the glide is over.
func TestAJumpKeepsItsHighlightWhileItGlides(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	if want := workspace.Slug("第 6 节"); s.anchor != want {
		t.Errorf("anchor = %q right after the click, want %q", s.anchor, want)
	}
	tt.Advance(time.Second)
	tt.Frame()
	s.docScroll().Y = 0
	tt.Frame()
	tt.Frame()
	if want := workspace.Slug("第 1 节"); s.anchor != want {
		t.Errorf("anchor = %q back at the top, want the follow to take over, %q", s.anchor, want)
	}
}

// TestTheOutlineKeepsTheCurrentHeadingInSight pins that the outline scrolls to
// the current row of a long document, rather than marking a row nobody sees.
func TestTheOutlineKeepsTheCurrentHeadingInSight(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(40))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s) // 1000x700
	tt.Frame()
	pane := s.bodyBox
	if len(s.headMarks) < 30 {
		t.Fatalf("marks = %d, want the whole outline measured", len(s.headMarks))
	}
	s.docScroll().Y = s.headMarks[29].y - (pane.Y + bodyJumpGap) + 1
	for range 3 {
		tt.Frame()
	}
	if want := workspace.Slug("第 30 节"); s.anchor != want {
		t.Fatalf("anchor = %q, want %q", s.anchor, want)
	}
	if s.tocScroll.Y <= 0 {
		t.Fatal("the outline did not scroll to the current row")
	}
	list, ok := tt.Find("目录列表")
	if !ok {
		t.Fatalf("texts = %q, want the outline's scroll container", tt.Texts())
	}
	row, ok := tt.Find("第 30 节")
	if !ok {
		t.Fatalf("texts = %q, want the current row built", tt.Texts())
	}
	if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
		t.Errorf("row at %v, outline %v: the current row is not in sight", row, list)
	}
}

// TestTheHoldOutlastsTheGlide pins that the outline is not repainted while the
// body is still moving: the hold has to cover MyGo's smooth scroll, or the last
// frames of a jump mark the section the body is passing rather than the one it
// was sent to.
func TestTheHoldOutlastsTheGlide(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", sections(8))
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	if err := tt.Click("第 6 节"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	if got := s.followUntil.Sub(s.frameNow); got < ui.Smooth.Spring.Duration {
		t.Errorf("hold = %v, want at least the %v the glide takes", got, ui.Smooth.Spring.Duration)
	}
}
