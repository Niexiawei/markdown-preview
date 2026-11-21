package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mdpreview/internal/config"
)

// newWorkspace returns a workspace with one project rooted at a temp dir.
func newWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	var cfg config.Config
	cfg.AddProject(root, "test")
	return New(cfg), root
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Hello World":  "hello-world",
		"导出流程":         "导出流程",
		"a  b":         "a--b",
		"C++ Notes":    "c-notes",
		"a_b-c":        "a_b-c",
		"  Trailing  ": "--trailing--",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugMatchesMygoView(t *testing.T) {
	// ui/markdown.Slug: lower case, each space a hyphen, - _ and alphanumerics
	// kept, everything else dropped, with no collapsing and no trimming.
	if got := Slug("A  B!"); got != "a--b" {
		t.Errorf("Slug(%q) = %q, want %q", "A  B!", got, "a--b")
	}
}

func TestOutlineLevels(t *testing.T) {
	src := "# A\n\n### C\n\ntext\n\nSetext\n===\n"
	_, headings, err := parseSource(src)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	want := []Heading{{1, "A", "a"}, {3, "C", "c"}, {1, "Setext", "setext"}}
	if len(headings) != len(want) {
		t.Fatalf("headings = %v, want %v", headings, want)
	}
	for i := range want {
		if headings[i] != want[i] {
			t.Errorf("heading %d = %v, want %v", i, headings[i], want[i])
		}
	}
}

func TestOutlineSkipsCodeFence(t *testing.T) {
	src := "# Real\n\n```\n# not a heading\n```\n\n## Also\n"
	_, headings, err := parseSource(src)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	if len(headings) != 2 || headings[0].Text != "Real" || headings[1].Text != "Also" {
		t.Errorf("headings = %v, want [Real Also]", headings)
	}
}

func TestReadMarkdownTooLarge(t *testing.T) {
	ws, root := newWorkspace(t)
	big := make([]byte, MaxDocBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(root, "big.md"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	projectID := ws.Projects()[0].ID
	if _, err := ws.ReadMarkdown(projectID, "big.md"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("ReadMarkdown(big) = %v, want ErrTooLarge", err)
	}
}

func TestReadMarkdownInvalidUTF8(t *testing.T) {
	ws, root := newWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "bad.md"), []byte("a\x00b\xffc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectID := ws.Projects()[0].ID
	src, err := ws.ReadMarkdown(projectID, "bad.md")
	if err != nil {
		t.Fatalf("ReadMarkdown: %v", err)
	}
	if !strings.Contains(src, "\uFFFD") {
		t.Errorf("src = %q, want U+FFFD for invalid bytes", src)
	}
	if _, _, err := ws.Parse(projectID, "bad.md"); err != nil {
		t.Errorf("Parse(invalid UTF-8) = %v, want nil", err)
	}
}

func TestReadMarkdownRejectsEscape(t *testing.T) {
	ws, root := newWorkspace(t)
	projectID := ws.Projects()[0].ID
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	if err := os.WriteFile(outside, []byte("# outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.ReadMarkdown(projectID, "../outside.md"); !errors.Is(err, ErrOutsideProject) {
		t.Errorf("ReadMarkdown(../outside.md) = %v, want ErrOutsideProject", err)
	}
}

func TestParseCachesByModTime(t *testing.T) {
	ws, root := newWorkspace(t)
	write(t, root, "a.md")
	projectID := ws.Projects()[0].ID
	before := parseCount.Load()
	if _, _, err := ws.Parse(projectID, "a.md"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.Parse(projectID, "a.md"); err != nil {
		t.Fatal(err)
	}
	if got := parseCount.Load() - before; got != 1 {
		t.Errorf("parsed %d times, want 1", got)
	}

	later := time.Now().Add(2 * time.Second)
	path := filepath.Join(root, "a.md")
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.Parse(projectID, "a.md"); err != nil {
		t.Fatal(err)
	}
	if got := parseCount.Load() - before; got != 2 {
		t.Errorf("parsed %d times after a write, want 2", got)
	}
}

func TestSearchFindsLineNumbers(t *testing.T) {
	ws, root := newWorkspace(t)
	write(t, root, "a.md")
	write(t, root, "docs/b.md")
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\n\nfirst Export here\nsecond line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "b.md"), []byte("no match\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("Export\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectID := ws.Projects()[0].ID

	hits, err := ws.Search(projectID, "EXPORT", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %v, want 1", hits)
	}
	if hits[0].Path != "a.md" || hits[0].Line != 3 || !strings.Contains(hits[0].Text, "Export") {
		t.Errorf("hit = %+v, want a.md:3 with Export", hits[0])
	}
	if hits, _ := ws.Search(projectID, "export", 10); len(hits) != 1 {
		t.Errorf("Search(export) = %v, want 1 (case insensitive)", hits)
	}
	if hits, _ := ws.Search(projectID, "", 10); len(hits) != 0 {
		t.Errorf("Search(empty) = %v, want none", hits)
	}
	if hits, _ := ws.Search(projectID, "line", 1); len(hits) != 1 {
		t.Errorf("Search(line, limit 1) = %v, want 1", hits)
	}
}

func TestTitleFallsBackToFilename(t *testing.T) {
	ws, root := newWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "no-heading.md"), []byte("just text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "with-heading.md"), []byte("# Real Title\n\ntext\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectID := ws.Projects()[0].ID
	docs, err := ws.ListDocs(projectID)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	titles := map[string]string{}
	for _, d := range docs {
		titles[d.Path] = d.Title
	}
	if titles["no-heading.md"] != "no-heading" {
		t.Errorf("Title = %q, want %q", titles["no-heading.md"], "no-heading")
	}
	if titles["with-heading.md"] != "Real Title" {
		t.Errorf("Title = %q, want %q", titles["with-heading.md"], "Real Title")
	}
}

func TestTitlesAreCachedAndInvalidated(t *testing.T) {
	ws, root := newWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# First\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectID := ws.Projects()[0].ID
	before := titleScanCount.Load()
	for i := 0; i < 3; i++ {
		docs, err := ws.ListDocs(projectID)
		if err != nil {
			t.Fatalf("ListDocs: %v", err)
		}
		if docs[0].Title != "First" {
			t.Fatalf("Title = %q, want First", docs[0].Title)
		}
	}
	if got := titleScanCount.Load() - before; got != 1 {
		t.Errorf("scanned titles %d times, want 1", got)
	}

	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("# Second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	docs, err := ws.ListDocs(projectID)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if docs[0].Title != "Second" {
		t.Errorf("Title = %q, want Second after the write", docs[0].Title)
	}
}
