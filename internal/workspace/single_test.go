package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mdpreview/internal/config"
)

// newFileWorkspace returns a workspace whose only project is the document at
// path.
func newFileWorkspace(t *testing.T, path string) (*Workspace, config.Project) {
	t.Helper()
	var cfg config.Config
	p, ok := cfg.AddFileProject(path)
	if !ok {
		t.Fatalf("AddFileProject(%q) = false, want the project added", path)
	}
	return New(cfg), p
}

// looseFile writes one document outside any directory project and returns it.
func looseFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheTreeOfAFileProjectIsOneNode(t *testing.T) {
	path := looseFile(t, "# Loose Notes\n")
	_, p := newFileWorkspace(t, path)
	tree, err := Tree(p)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	switch {
	case tree == nil:
		t.Fatal("Tree = nil, want the document")
	case tree.IsDir:
		t.Error("IsDir = true, want a document")
	case tree.Name != "notes.md":
		t.Errorf("Name = %q, want notes.md", tree.Name)
	case tree.Path != "notes.md":
		t.Errorf("Path = %q, want notes.md", tree.Path)
	case len(tree.Children) != 0:
		t.Errorf("Children = %v, want none", names(tree))
	}
}

func TestTheTreeOfAFileProjectWhoseFileIsGone(t *testing.T) {
	path := looseFile(t, "# Loose Notes\n")
	_, p := newFileWorkspace(t, path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Tree(p); !errors.Is(err, ErrProjectGone) {
		t.Fatalf("Tree(removed file) = %v, want ErrProjectGone", err)
	}
	// A directory where the document was is gone too, as far as the project
	// is concerned.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Tree(p); !errors.Is(err, ErrProjectGone) {
		t.Fatalf("Tree(file replaced by a directory) = %v, want ErrProjectGone", err)
	}
}

func TestListDocsOfAFileProject(t *testing.T) {
	path := looseFile(t, "# Loose Notes\n\nbody\n")
	ws, p := newFileWorkspace(t, path)
	docs, err := ws.ListDocs(p.ID)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("ListDocs = %v, want one document", docs)
	}
	doc := docs[0]
	switch {
	case doc.ProjectID != p.ID:
		t.Errorf("ProjectID = %q, want %q", doc.ProjectID, p.ID)
	case doc.Path != "notes.md":
		t.Errorf("Path = %q, want notes.md", doc.Path)
	case doc.Title != "Loose Notes":
		t.Errorf("Title = %q, want Loose Notes", doc.Title)
	case doc.Size == 0:
		t.Error("Size = 0, want the file's size")
	}
	// The document the workspace lists is the one it reads.
	src, err := ws.ReadMarkdown(p.ID, doc.Path)
	if err != nil {
		t.Fatalf("ReadMarkdown: %v", err)
	}
	if src != "# Loose Notes\n\nbody\n" {
		t.Errorf("ReadMarkdown = %q, want the file's source", src)
	}
}

func TestLocateFindsTheKnownProjectHoldingAFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.md")
	write(t, root, "sub/b.md")
	var cfg config.Config
	outer, _ := cfg.AddProject(root, "outer")
	inner, _ := cfg.AddProject(filepath.Join(root, "sub"), "inner")

	id, rel, ok := Locate(cfg.Projects, filepath.Join(root, "sub", "b.md"))
	if !ok || id != inner.ID || rel != "b.md" {
		t.Errorf("Locate(sub/b.md) = %q, %q, %v, want the deeper project", id, rel, ok)
	}
	if id, rel, ok := Locate(cfg.Projects, filepath.Join(root, "a.md")); !ok || id != outer.ID || rel != "a.md" {
		t.Errorf("Locate(a.md) = %q, %q, %v, want the outer project", id, rel, ok)
	}
}

func TestLocateFindsAFileProjectItself(t *testing.T) {
	path := looseFile(t, "# Loose Notes\n")
	_, p := newFileWorkspace(t, path)
	id, rel, ok := Locate([]Project{p}, path)
	if !ok || id != p.ID || rel != "notes.md" {
		t.Errorf("Locate(%q) = %q, %q, %v, want the file project", path, id, rel, ok)
	}
}

func TestLocateSkipsWhatTheTreeWouldNotShow(t *testing.T) {
	root := t.TempDir()
	write(t, root, "node_modules/x.md")
	write(t, root, "notes.txt")
	var cfg config.Config
	cfg.AddProject(root, "outer")
	cases := map[string]string{
		"hidden behind the ignore rules": filepath.Join(root, "node_modules", "x.md"),
		"not markdown":                   filepath.Join(root, "notes.txt"),
		"a directory":                    root,
		"nowhere":                        filepath.Join(t.TempDir(), "gone.md"),
	}
	for what, abs := range cases {
		if id, rel, ok := Locate(cfg.Projects, abs); ok {
			t.Errorf("Locate(%s) = %q, %q, true, want no project", what, id, rel)
		}
	}
}
