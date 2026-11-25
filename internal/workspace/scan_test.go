package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"mdpreview/internal/config"
)

func TestIgnoredRules(t *testing.T) {
	ignored := []string{".git", ".obsidian", "node_modules", "vendor", "dist", "build", ".mygo"}
	for _, name := range ignored {
		if !Ignored(name, true) {
			t.Errorf("Ignored(%q, dir) = false, want true", name)
		}
	}
	kept := []string{"docs", "notes", "src"}
	for _, name := range kept {
		if Ignored(name, true) {
			t.Errorf("Ignored(%q, dir) = true, want false", name)
		}
	}
	if Ignored("readme.txt", false) {
		t.Error("Ignored(file) = true, want false")
	}
	if !Ignored("docs/.hidden.md", false) {
		t.Error("Ignored(.hidden.md) = false, want true")
	}
}

func TestIsMarkdown(t *testing.T) {
	yes := []string{"a.md", "A.MD", "a.markdown", "a.MarkDown"}
	for _, name := range yes {
		if !IsMarkdown(name) {
			t.Errorf("IsMarkdown(%q) = false, want true", name)
		}
	}
	no := []string{"a.txt", "a.md.bak", "a", "md"}
	for _, name := range no {
		if IsMarkdown(name) {
			t.Errorf("IsMarkdown(%q) = true, want false", name)
		}
	}
}

func TestListDocsRecursive(t *testing.T) {
	root := t.TempDir()
	write(t, root, "notes/a.md")
	write(t, root, "notes/b.markdown")
	write(t, root, ".git/x.md")
	write(t, root, "node_modules/y.md")
	write(t, root, "readme.txt")

	docs, err := ListDocs(root)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("ListDocs = %v, want 2 docs", docs)
	}
	if docs[0].Path != "notes/a.md" || docs[1].Path != "notes/b.markdown" {
		t.Errorf("paths = %q, %q", docs[0].Path, docs[1].Path)
	}
	if docs[0].Title != "a" {
		t.Errorf("Title = %q, want %q", docs[0].Title, "a")
	}
	if docs[0].Size == 0 || docs[0].ModTime.IsZero() {
		t.Errorf("Size/ModTime not filled: %+v", docs[0])
	}
}

func TestBuildTreeSortsDirsFirst(t *testing.T) {
	root := t.TempDir()
	write(t, root, "readme.md")
	write(t, root, "docs/guide.md")
	write(t, root, "docs/deep/api.md")
	write(t, root, "empty/notes.txt")

	tree, err := BuildTree(root)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	if got := names(tree); !slices.Equal(got, []string{"docs", "readme.md"}) {
		t.Errorf("root children = %v, want [docs readme.md]", got)
	}
	docs := tree.Children[0]
	if got := names(docs); !slices.Equal(got, []string{"deep", "guide.md"}) {
		t.Errorf("docs children = %v, want [deep guide.md]", got)
	}
	if docs.Name != "docs" || !docs.IsDir {
		t.Errorf("first child = %+v, want docs as a dir", docs)
	}
}

func TestListDocsProjectGone(t *testing.T) {
	root := filepath.Join(t.TempDir(), "gone")
	write(t, root, "a.md")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ListDocs(root); !errors.Is(err, ErrProjectGone) {
		t.Errorf("ListDocs(gone) = %v, want ErrProjectGone", err)
	}
	if _, err := BuildTree(root); !errors.Is(err, ErrProjectGone) {
		t.Errorf("BuildTree(gone) = %v, want ErrProjectGone", err)
	}
}

func TestListDocsSymlinkLoop(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/one.md")
	if err := os.Symlink(root, filepath.Join(root, "a", "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	done := make(chan []Doc, 1)
	go func() {
		docs, _ := ListDocs(root)
		done <- docs
	}()
	select {
	case docs := <-done:
		if len(docs) != 1 {
			t.Errorf("ListDocs = %v, want 1 doc", docs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListDocs did not finish: symlink loop")
	}
}

func TestWorkspaceProjects(t *testing.T) {
	root := t.TempDir()
	var cfg config.Config
	p, _ := cfg.AddProject(root, "alpha")
	ws := New(cfg)
	if got := ws.Projects(); len(got) != 1 || got[0].ID != p.ID {
		t.Errorf("Projects() = %v, want [%v]", got, p)
	}
	if got, ok := ws.Project(p.ID); !ok || got.Root != root {
		t.Errorf("Project(%q) = %v, %v", p.ID, got, ok)
	}
	if _, ok := ws.Project("nope"); ok {
		t.Error("Project(nope) = true, want false")
	}
}

func write(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(n *Node) []string {
	out := make([]string, 0, len(n.Children))
	for _, c := range n.Children {
		out = append(out, c.Name)
	}
	return out
}

func TestListDocsWhenRootIsAFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ListDocs(root); !errors.Is(err, ErrProjectGone) {
		t.Errorf("ListDocs(file) = %v, want ErrProjectGone", err)
	}
}

func TestListDocsOddNames(t *testing.T) {
	root := t.TempDir()
	names := []string{"方案 #1 50%.md", "空格 与+加号.md", "问号？全角.md", "百分号%与会话#.md", "dash-and_under.md"}
	for _, name := range names {
		write(t, root, name)
	}
	docs, err := ListDocs(root)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	got := map[string]bool{}
	for _, d := range docs {
		got[d.Path] = true
	}
	for _, name := range names {
		if !got[name] {
			t.Errorf("docs = %v, want %q", got, name)
		}
	}
}

func TestParseEmptyAndCodeOnly(t *testing.T) {
	for _, src := range []string{"", "\n\n", "```go\nfunc main() {}\n```\n", "<div>x</div>\n"} {
		doc, headings, err := parseSource(src)
		if err != nil {
			t.Fatalf("parseSource(%q): %v", src, err)
		}
		if doc == nil {
			t.Errorf("parseSource(%q) = nil document", src)
		}
		if len(headings) != 0 {
			t.Errorf("parseSource(%q) headings = %v, want none", src, headings)
		}
	}
}

// TestTooManyEntriesStopsTheScan pins the ceiling: a directory with more entries
// than the limit is refused rather than read whole.
func TestTooManyEntriesStopsTheScan(t *testing.T) {
	root := t.TempDir()
	for i := range 10 {
		write(t, root, "d"+strconv.Itoa(i%3)+"/f"+strconv.Itoa(i)+".md")
	}
	if _, err := listDocs(root, 5); !errors.Is(err, ErrTreeTooLarge) {
		t.Fatalf("err = %v, want ErrTreeTooLarge", err)
	}
	if _, err := listDocs(root, 1000); err != nil {
		t.Fatalf("err = %v, want the tree read under the limit", err)
	}
	// The refused directories do not count: a tree of ignored files stays
	// under a limit of nothing.
	ignored := t.TempDir()
	write(t, ignored, "node_modules/a.md")
	if docs, err := listDocs(ignored, 1); err != nil || len(docs) != 0 {
		t.Errorf("listDocs(ignored) = %v, %v, want no documents and no error", docs, err)
	}
}

// TestTheEntryLimitIsAMillion pins the number the app refuses a project over.
func TestTheEntryLimitIsAMillion(t *testing.T) {
	if MaxEntries != 1_000_000 {
		t.Errorf("MaxEntries = %d, want 1000000", MaxEntries)
	}
}
