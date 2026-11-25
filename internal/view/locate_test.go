package view

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTheCrosshairBringsTheDocumentIntoView(t *testing.T) {
	s, root := newState(t)
	for i := range 40 {
		writeDoc(t, root, "docs/f"+strconv.Itoa(i)+".md", "# F\n")
	}
	s.Refresh()
	id := s.Trees()[0].project.ID
	s.selectDoc("docs/f39.md")
	s.setTreeOpen("", false)
	tt := ui.NewTester(s.View, 1000, 300)
	tt.Frame()
	if err := tt.Click("在项目树中定位当前文档"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if !s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("docs is folded, want the crosshair to unfold the way to the document")
	}
	first, last := s.treeOpen.List.Visible()
	want := s.rowOf(s.roots(), treeKeyOf(id, "docs/f39.md"))
	if want < 0 {
		t.Fatalf("the tree does not show docs/f39.md")
	}
	if want < first || want > last {
		t.Errorf("rows %d..%d visible, want row %d: the tree slid to the document", first, last, want)
	}
}

func TestTheCrosshairIsDisabledWithoutADocument(t *testing.T) {
	s, _ := newState(t)
	tt := ui.NewTester(func(c *ui.Context) { s.LeftHeader(c) }, 300, 48)
	a, ok := tt.Access("在项目树中定位当前文档")
	if !ok {
		t.Fatalf("no crosshair; access = %v", accessLabels(tt))
	}
	if !a.Disabled {
		t.Error("the crosshair is enabled with no document shown, want it disabled")
	}
}

func TestLocatingClosesTheTreeSearch(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	writeDoc(t, root, "top.md", "# Top\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	s.selectDoc("top.md")
	openTreeSearchOn(s, "a")
	if !s.treeSearch.open {
		t.Fatal("the search is closed, want it open")
	}
	s.locateDoc()
	if s.treeSearch.open {
		t.Error("the search is still open, want it folded away")
	}
	if got := s.treeSearch.query; got != "a" {
		t.Errorf("query = %q, want it kept", got)
	}
	if !s.treeOpen.Open.Has(treeKeyOf(id, "docs")) {
		t.Error("docs is folded, want the document's directory unfolded")
	}
}

func TestLocatingAFileTheTreeDoesNotShowIsReported(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()
	id := s.Trees()[0].project.ID
	s.sel = docSel{project: id, path: "gone.md"}
	s.locateDoc()
	if got := s.Status(); !strings.Contains(got, "项目树里没有这个文件") {
		t.Errorf("Status = %q, want the file named as not in the tree", got)
	}
}

func TestLocatingALooseFileWorks(t *testing.T) {
	s, _ := newState(t)
	s.OpenDocument(looseDoc(t, "# Loose\n"))
	tt := ui.NewTester(s.View, 1000, 300)
	tt.Frame()
	if err := tt.Click("在项目树中定位当前文档"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt.Frame()
	first, last := s.treeOpen.List.Visible()
	loose := s.Projects()[1].ID
	want := s.rowOf(s.roots(), treeKey{project: loose, path: "loose.md"})
	if want < first || want > last {
		t.Errorf("rows %d..%d visible, want row %d: the file's own row", first, last, want)
	}
	if got := s.Selected(); got != "loose.md" {
		t.Errorf("Selected = %q, want the file still shown", got)
	}
}
