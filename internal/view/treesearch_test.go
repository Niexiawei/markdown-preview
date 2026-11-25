package view

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
)

// openTreeSearchOn opens the tree's search on the window's trees and types
// query into it, as a user does.
func openTreeSearchOn(s *State, query string) {
	s.treeSearch.open = true
	s.setTreeQuery(query)
}

// TestCandidatesComeInTreeOrder pins the order of what the tree's search
// matches: the projects as they were opened, directories before the files
// beside them, each by name — the order the tree itself shows.
func TestCandidatesComeInTreeOrder(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a_plan.md", "# A\n")
	writeDoc(t, alphaRoot, "b_plan.md", "# B\n")
	writeDoc(t, betaRoot, "plan_c.md", "# C\n")
	s.Refresh()

	alpha, beta := s.Projects()[0].ID, s.Projects()[1].ID
	openTreeSearchOn(s, "plan")
	want := []docSel{
		{project: alpha, path: "docs/a_plan.md"},
		{project: alpha, path: "b_plan.md"},
		{project: beta, path: "plan_c.md"},
	}
	if !slices.Equal(s.treeSearch.candidates, want) {
		t.Errorf("candidates = %v, want %v", s.treeSearch.candidates, want)
	}
}

// TestCandidatesMarkTheirDirectories pins the directories a search keeps: the
// ones holding a candidate, on every level above it, and no others.
func TestCandidatesMarkTheirDirectories(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/deep/PLAN.md", "# Plan\n")
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	writeDoc(t, root, "other/x.md", "# X\n")
	writeDoc(t, root, "readme.md", "# Readme\n")
	s.Refresh()

	id := s.Projects()[0].ID
	openTreeSearchOn(s, "plan")
	want := []string{id + ":docs", id + ":docs/deep"}
	got := make([]string, 0, len(s.treeSearch.dirs))
	for key := range s.treeSearch.dirs {
		got = append(got, key.project+":"+key.path)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("dirs = %v, want %v", got, want)
	}
}

// TestClearingTheQueryShowsTheWholeTree pins that an emptied phrase is no
// search at all: the tree shows every file again.
func TestClearingTheQueryShowsTheWholeTree(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes.md", "# Notes\n")
	s.Refresh()

	openTreeSearchOn(s, "n")
	if len(s.treeSearch.candidates) == 0 {
		t.Fatalf("candidates = %v, want the match before clearing", s.treeSearch.candidates)
	}
	s.setTreeQuery("")
	if s.searching() {
		t.Error("searching = true, want false once the phrase is empty")
	}
	if len(s.treeSearch.candidates) != 0 || len(s.treeSearch.dirs) != 0 {
		t.Errorf("candidates = %v, dirs = %v, want no search at all", s.treeSearch.candidates, s.treeSearch.dirs)
	}
}

// TestSearchingUsesTheTreeInMemory pins that typing searches the tree the
// window has, not the disk: a file removed behind the window's back is still
// found until the tree is reloaded.
func TestSearchingUsesTheTreeInMemory(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	if err := os.Remove(filepath.Join(root, "a.md")); err != nil {
		t.Fatal(err)
	}

	openTreeSearchOn(s, "a")
	want := docSel{project: s.Projects()[0].ID, path: "a.md"}
	if !slices.Contains(s.treeSearch.candidates, want) {
		t.Errorf("candidates = %v, want a.md found in the tree in memory", s.treeSearch.candidates)
	}
}

// TestClosingAProjectDropsItsCandidates pins that a closed project takes its
// candidates with it: the tree no longer shows them, so the search must not
// keep offering them, or Enter would open a document of a project that is
// closed.
func TestClosingAProjectDropsItsCandidates(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "a_plan.md", "# A\n")
	writeDoc(t, betaRoot, "b_plan.md", "# B\n")
	s.Refresh()
	beta := s.Projects()[1].ID

	openTreeSearchOn(s, "plan")
	if len(s.treeSearch.candidates) != 2 {
		t.Fatalf("candidates = %v, want both projects' matches", s.treeSearch.candidates)
	}
	s.ToggleProject(beta)
	if len(s.treeSearch.candidates) != 1 {
		t.Fatalf("candidates = %v, want beta's match dropped", s.treeSearch.candidates)
	}
	if got := s.treeSearch.candidates[0].project; got == beta {
		t.Errorf("candidate project = %q, want alpha's", got)
	}
}

// TestCandidatesClampTheCurrent pins the index of the candidate the keyboard is
// on: a reload that leaves fewer candidates keeps it inside them, so that
// nothing reads past the end.
func TestCandidatesClampTheCurrent(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "p1_plan.md", "# One\n")
	writeDoc(t, root, "p2_plan.md", "# Two\n")
	s.Refresh()

	openTreeSearchOn(s, "plan")
	s.treeSearch.current = 1
	if err := os.Remove(filepath.Join(root, "p2_plan.md")); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if len(s.treeSearch.candidates) != 1 {
		t.Fatalf("candidates = %v, want one left", s.treeSearch.candidates)
	}
	if s.treeSearch.current != 0 {
		t.Errorf("current = %d, want 0: the candidate it named is gone", s.treeSearch.current)
	}
}

// TestSearchingTheTreeShowsOnlyMatchingFiles pins the filtered tree: while a
// phrase is typed the sidebar shows the files it matches and the directories
// above them, and nothing else.
func TestSearchingTheTreeShowsOnlyMatchingFiles(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/PLAN.md", "# Plan\n")
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	writeDoc(t, root, "README.md", "# Readme\n")
	s.Refresh()
	openTreeSearchOn(s, "plan")

	tt := newTester(s)
	if !tt.HasText("PLAN.md") {
		t.Fatalf("texts = %q, want the matched file", tt.Texts())
	}
	for _, gone := range []string{"notes.md", "README.md"} {
		if tt.HasText(gone) {
			t.Errorf("texts = %q, want %q left out of the search", tt.Texts(), gone)
		}
	}
	if !tt.HasText("docs") {
		t.Errorf("texts = %q, want the directory holding the match", tt.Texts())
	}
}

// TestCandidatesAndTheOneChosenAreTwoDifferentFills pins the two washes the
// search puts on the tree: every candidate is filled, the one the keyboard is on
// is filled more, and the two are not the same fill.
func TestCandidatesAndTheOneChosenAreTwoDifferentFills(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md", "other.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")
	s.treeSearch.current = 1

	id := s.Projects()[0].ID
	switch {
	case s.treeRowFill(id, "a_plan.md") != rowCandidate:
		t.Errorf("fill of a_plan.md = %v, want rowCandidate", s.treeRowFill(id, "a_plan.md"))
	case s.treeRowFill(id, "b_plan.md") != rowCurrent:
		t.Errorf("fill of b_plan.md = %v, want rowCurrent", s.treeRowFill(id, "b_plan.md"))
	case s.treeRowFill(id, "other.md") != rowNone:
		t.Errorf("fill of other.md = %v, want rowNone", s.treeRowFill(id, "other.md"))
	}

	same := true
	_ = ui.NewTester(func(c *ui.Context) {
		same = candidateRowTint(c.Theme()) == currentCandidateTint(c.Theme())
	}, 100, 100)
	if same {
		t.Error("the two fills are the same colour, want the chosen one told apart")
	}
}

// TestSteppingTheCurrentCandidateScrollsTheTree pins the locating: the tree
// slides so that the candidate chosen shows, even when it is far below the fold.
func TestSteppingTheCurrentCandidateScrollsTheTree(t *testing.T) {
	s, root := newState(t)
	for i := range 40 {
		writeDoc(t, root, "plan_"+strconv.Itoa(i)+".md", "# P\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")
	s.treeSearch.current = len(s.treeSearch.candidates) - 1
	s.treeSearch.reveal = true

	tt := ui.NewTester(s.View, 1000, 300)
	tt.Frame()
	tt.Frame()
	first, last := s.treeOpen.List.Visible()
	if want := s.rowOf(s.roots(), treeKey{project: s.Trees()[0].project.ID, path: "plan_39.md"}); want < first || want > last {
		t.Errorf("rows %d..%d visible, want row %d: the tree slid to the candidate", first, last, want)
	}
}

// TestTheSearchShowsOnlyWhatMatchesInTheOutline pins the filtered tree: the
// directories holding a match unfold for the length of the search, and closing
// it shows the tree as the user folded it.
func TestTheSearchShowsOnlyWhatMatchesInTheOutline(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/plan.md", "# P\n")
	writeDoc(t, root, "docs/other.md", "# O\n")
	writeDoc(t, root, "top.md", "# T\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	s.setTreeKeyOpen(treeKey{project: id, path: "docs"}, false)
	openTreeSearchOn(s, "plan")
	tt := newTester(s)
	if tt.HasText("other.md") {
		t.Errorf("texts = %q, want only what the phrase matches", tt.Texts())
	}
	if !tt.HasText("plan.md") {
		t.Errorf("texts = %q, want the match under the directory it sits in", tt.Texts())
	}
	s.closeTreeSearch()
	tt = newTester(s)
	if tt.HasText("plan.md") {
		t.Errorf("texts = %q, want the directory as the user folded it", tt.Texts())
	}
	if !tt.HasText("top.md") {
		t.Errorf("texts = %q, want the rest of the tree back", tt.Texts())
	}
}

// TestTheSearchRevealsTheCurrentCandidate pins the slide to the candidate the
// keyboard is on, of a list longer than the pane.
func TestTheSearchRevealsTheCurrentCandidate(t *testing.T) {
	s, root := newState(t)
	for i := range 60 {
		writeDoc(t, root, "docs/a"+strconv.Itoa(100 + i)[1:]+".md", "# A\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "a")
	if got := len(s.treeSearch.candidates); got != 60 {
		t.Fatalf("candidates = %d, want 60", got)
	}
	s.moveCandidate(40)
	tt := ui.NewTester(s.View, 1000, 300)
	tt.Frame()
	tt.Frame()
	cell := s.treeSearch.candidates[40]
	want := s.rowOf(s.roots(), treeKey{project: cell.project, path: cell.path})
	first, last := s.treeOpen.List.Visible()
	if want < first || want > last {
		t.Errorf("rows %d..%d visible, want row %d (%s)", first, last, want, cell.path)
	}
}

// TestTheSearchRowCountsItsCandidates pins the count beside the field, and what
// a phrase matching nothing shows.
func TestTheSearchRowCountsItsCandidates(t *testing.T) {
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	openTreeSearchOn(s, "plan")

	tt := newTester(s)
	if !tt.HasText("1/3") {
		t.Errorf("texts = %q, want the count 1/3", tt.Texts())
	}
	s.setTreeQuery("zzz")
	tt = newTester(s)
	if !tt.HasText("无匹配") {
		t.Errorf("texts = %q, want 无匹配", tt.Texts())
	}
	if !tt.HasText("没有匹配的文件") {
		t.Errorf("texts = %q, want the empty search named in the tree", tt.Texts())
	}
}

// TestTheTreeScrollerIsNamed pins the tree's scrolling pane: a name for the
// reader, and the room it takes for the tests that measure it.
func TestTheTreeScrollerIsNamed(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()

	tt := newTester(s)
	r, ok := tt.Find("项目树")
	if !ok {
		t.Fatalf("no named tree scroller; texts = %q", tt.Texts())
	}
	if r.W < 200 {
		t.Errorf("tree width = %v, want the sidebar's own width", r.W)
	}
	if r.Y <= 0 {
		t.Errorf("tree Y = %v, want it below the header", r.Y)
	}
}

// TestCmdFInTheTreeSearchesFiles pins the routing of the shortcut: a press in
// the sidebar makes Cmd+F search the file names, not the document.
func TestCmdFInTheTreeSearchesFiles(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := newTester(s)
	if err := tt.Click("a.md"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.treeSearch.open {
		t.Error("the tree's search did not open after a press in the tree")
	}
	if s.find.open {
		t.Error("the document's search opened, want the tree's")
	}
}

// TestCmdFInTheBodySearchesTheDocument pins the other side of the routing: a
// press in the body makes Cmd+F search the document.
func TestCmdFInTheBodySearchesTheDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := newTester(s)
	tt.ClickAt(700, 400)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Error("the document's search did not open after a press in the body")
	}
	if s.treeSearch.open {
		t.Error("the tree's search opened, want the document's")
	}
}

// TestShiftCmdFOpensTheTreeSearchFromAnywhere pins the shortcut that needs no
// routing: it searches file names wherever the focus is.
func TestShiftCmdFOpensTheTreeSearchFromAnywhere(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Frame()
	if !s.treeSearch.open {
		t.Fatal("the tree's search did not open")
	}
	if s.find.open {
		t.Error("the document's search opened, want only the tree's")
	}
}

// TestOpeningOnePanelClosesTheOther pins that the two searches never share the
// keyboard: opening either folds the other away.
func TestOpeningOnePanelClosesTheOther(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n\n正文\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Fatalf("the document's search did not open; texts = %q", tt.Texts())
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Frame()
	if s.find.open {
		t.Error("the document's search is still open beside the tree's")
	}
	if !s.treeSearch.open {
		t.Fatalf("the tree's search did not open")
	}
	tt.ClickAt(700, 400)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Error("the document's search did not reopen after a press in the body")
	}
	if s.treeSearch.open {
		t.Error("the tree's search is still open beside the document's")
	}
}

// typeTreeSearch opens the tree's search with the shortcut and types query into
// it, as a user does: the field takes the keyboard as the search opens.
func typeTreeSearch(t *testing.T, tt *ui.Tester, query string) {
	t.Helper()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Frame()
	tt.Type(query)
	tt.Frame()
}

// searchableTree is a project of three files a phrase matches, and one it does
// not, with the window built.
func searchableTree(t *testing.T) (*State, *ui.Tester) {
	t.Helper()
	s, root := newState(t)
	for _, name := range []string{"a_plan.md", "b_plan.md", "c_plan.md", "other.md"} {
		writeDoc(t, root, name, "# "+name+"\n")
	}
	s.Refresh()
	return s, newTester(s)
}

// TestArrowsStepTheCandidatesAndOpenNothing pins the keys that walk the
// candidates: the arrows move the one the tree marks as current, and nothing
// opens until it is asked for.
func TestArrowsStepTheCandidatesAndOpenNothing(t *testing.T) {
	s, tt := searchableTree(t)
	typeTreeSearch(t, tt, "plan")

	before := s.Selected()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if s.treeSearch.current != 2 {
		t.Errorf("current = %d, want 2 after two downs", s.treeSearch.current)
	}
	if got := s.Selected(); got != before {
		t.Errorf("Selected = %q, want %q: stepping must not open a document", got, before)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if s.treeSearch.current != 1 {
		t.Errorf("current = %d, want 1 after an up", s.treeSearch.current)
	}
}

// TestEnterOpensTheCurrentCandidate pins the key that opens: Enter shows the
// candidate the arrows are on, and leaves the search open to walk further.
func TestEnterOpensTheCurrentCandidate(t *testing.T) {
	s, tt := searchableTree(t)
	typeTreeSearch(t, tt, "plan")

	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := s.Selected(); got != "b_plan.md" {
		t.Errorf("Selected = %q, want b_plan.md", got)
	}
	if !s.treeSearch.open {
		t.Error("the tree's search closed, want it left open")
	}
}

// TestEnterWithNoCandidatesDoesNothing pins the guarded Enter: a phrase that
// matches nothing opens nothing, and reads nothing past the end of the list.
func TestEnterWithNoCandidatesDoesNothing(t *testing.T) {
	s, tt := searchableTree(t)
	s.selectDoc("other.md")
	typeTreeSearch(t, tt, "zzz")
	if len(s.treeSearch.candidates) != 0 {
		t.Fatalf("candidates = %v, want none", s.treeSearch.candidates)
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := s.Selected(); got != "other.md" {
		t.Errorf("Selected = %q, want other.md still shown", got)
	}
}

// TestEscapeClearsTheQueryThenClosesTheTreeSearch pins the order Escape goes in:
// the phrase first, so that the tree shows everything again, and the search row
// itself on the second press.
func TestEscapeClearsTheQueryThenClosesTheTreeSearch(t *testing.T) {
	s, tt := searchableTree(t)
	typeTreeSearch(t, tt, "plan")
	if !s.treeSearch.open {
		t.Fatalf("the tree's search is not open")
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !s.treeSearch.open {
		t.Fatal("the first Escape closed the search, want the phrase cleared first")
	}
	if s.treeSearch.query != "" || s.searching() {
		t.Errorf("query = %q, want it cleared", s.treeSearch.query)
	}
	if !tt.HasText("other.md") {
		t.Errorf("texts = %q, want the whole tree back", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.treeSearch.open {
		t.Error("the second Escape left the search open")
	}
}

// TestEscapeClosesOnlyTheTreeSearch pins that Escape belongs to one search at a
// time: the document's phrase is left alone while the tree's is open.
func TestEscapeClosesOnlyTheTreeSearch(t *testing.T) {
	s, tt := searchableTree(t)
	s.selectDoc("a_plan.md")
	typeTreeSearch(t, tt, "plan")
	// The document's search is shut while the tree's is open: put it back, as a
	// frame that opened it before the routing changed it would leave it, so that
	// Escape has both panels to choose between.
	s.find.open, s.find.query = true, "plan"
	s.refreshMatches()

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.find.query != "plan" {
		t.Errorf("find.query = %q, want the document's phrase kept", s.find.query)
	}
	if s.treeSearch.query != "" {
		t.Errorf("tree query = %q, want it cleared", s.treeSearch.query)
	}
}

// TestCandidateIndexFindsEveryCandidate pins the lookup the filtered tree asks
// for on every row it builds: each candidate is found where the list has it, and
// nothing else is a candidate.
func TestCandidateIndexFindsEveryCandidate(t *testing.T) {
	s, root := newState(t)
	for i := range 30 {
		writeDoc(t, root, "plan_"+strconv.Itoa(i)+".md", "# P\n")
	}
	writeDoc(t, root, "other.md", "# O\n")
	s.Refresh()
	openTreeSearchOn(s, "plan")

	id := s.Projects()[0].ID
	for i, cell := range s.treeSearch.candidates {
		if got := s.candidateIndex(cell.project, cell.path); got != i {
			t.Errorf("candidateIndex(%q) = %d, want %d", cell.path, got, i)
		}
	}
	if got := s.candidateIndex(id, "other.md"); got != -1 {
		t.Errorf("candidateIndex(other.md) = %d, want -1", got)
	}
}

// TestCmdFInTheWebViewSearchesTheDocument pins the one place the focus cannot
// say which pane was used: a press in the page goes to the page itself and never
// to an element, so while the page shows, Cmd+F searches the document even after
// a press in the tree. Shift+Cmd+F still searches the file names.
func TestCmdFInTheWebViewSearchesTheDocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")

	tt := newTester(s)
	if err := tt.Click("a.md"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	s.setMode(config.RenderWebView)
	tt.Frame()

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if !s.find.open {
		t.Error("the document's search did not open while the page shows")
	}
	if s.treeSearch.open {
		t.Error("the tree's search opened, want the document's")
	}
}

// TestTheSearchFieldsSpeakChinese pins what an empty field shows: a Chinese
// placeholder in both searches, rather than the framework's own "Search".
// The placeholder is painted, not a text of its own, so assistive technology is
// where a test reads it.
func TestTheSearchFieldsSpeakChinese(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A 正文")
	s.Refresh()
	s.selectDoc("a.md")

	tt := newTester(s)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Frame()
	if a, ok := tt.Access("在项目树中查找"); !ok || a.Placeholder != "搜索文件名" {
		t.Errorf("tree search placeholder = %q (found %v), want 搜索文件名", a.Placeholder, ok)
	}
	tt.ClickAt(700, 400)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if a, ok := tt.Access("在文档中查找"); !ok || a.Placeholder != "在文档中查找" {
		t.Errorf("document search placeholder = %q (found %v), want 在文档中查找", a.Placeholder, ok)
	}
}
