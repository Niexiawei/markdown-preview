package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"mdpreview/internal/config"
	"mdpreview/internal/watch"
)

// looseDoc writes a document outside every project of s and returns its path.
func looseDoc(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	writeDoc(t, root, "loose.md", body)
	// A temp directory may be named with a short 8.3 name on Windows, which
	// the window resolves: the path the test opens is the resolved one.
	path := filepath.Join(root, "loose.md")
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func TestOpeningAFileAddsItAsItsOwnEntry(t *testing.T) {
	s, _ := newState(t)
	file := looseDoc(t, "# Loose\n")
	s.OpenDocument(file)
	projects := s.Projects()
	if len(projects) != 2 {
		t.Fatalf("Projects = %v, want the directory project and the file", projects)
	}
	p := projects[1]
	switch {
	case p.File != file:
		t.Errorf("File = %q, want %q", p.File, file)
	case p.Root != filepath.Dir(file):
		t.Errorf("Root = %q, want %q", p.Root, filepath.Dir(file))
	case p.Name != "loose.md":
		t.Errorf("Name = %q, want loose.md", p.Name)
	}
	if got := s.Selected(); got != "loose.md" {
		t.Errorf("Selected = %q, want loose.md", got)
	}
	if got, ok := s.SelectedProject(); !ok || got.ID != p.ID {
		t.Errorf("SelectedProject = %v, %v, want the file's own project", got, ok)
	}
	if !strings.Contains(s.src, "Loose") {
		t.Errorf("src = %q, want the file's body", s.src)
	}
}

func TestTheTitleRowShowsALooseFilesPath(t *testing.T) {
	s, _ := newState(t)
	file := looseDoc(t, "# Loose\n")
	s.OpenDocument(file)
	if got := s.shortPath(); got != file {
		t.Errorf("shortPath = %q, want the file's path %q", got, file)
	}
}

func TestOpeningTheSameFileTwiceIsOneEntry(t *testing.T) {
	s, _ := newState(t)
	file := looseDoc(t, "# Loose\n")
	s.OpenDocument(file)
	first := s.Status()
	s.OpenDocument(file)
	if got := len(s.Projects()); got != 2 {
		t.Errorf("Projects = %d, want the directory project and the file", got)
	}
	if got := len(s.tabs); got != 1 {
		t.Errorf("tabs = %v, want the one", s.tabs)
	}
	if got := s.Status(); got != first {
		t.Errorf("Status = %q, want the first message %q: one file, one note", got, first)
	}
}

func TestTheTreeShowsProjectsAndLooseFilesTogether(t *testing.T) {
	s, _ := newState(t)
	s.OpenDocument(looseDoc(t, "# Loose\n"))
	tt := newTester(s)
	if !tt.HasText("alpha") || !tt.HasText("loose.md") {
		t.Errorf("texts = %q, want the project's node and the file's row", tt.Texts())
	}
}

func TestOpeningAFileOfAnOpenProjectSelectsItThere(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "docs/a.md", "# A\n")
	s.Refresh()
	s.OpenDocument(filepath.Join(root, "docs", "a.md"))
	if got := len(s.Projects()); got != 1 {
		t.Errorf("Projects = %d, want the one: the file is in it", got)
	}
	if got := s.Selected(); got != "docs/a.md" {
		t.Errorf("Selected = %q, want docs/a.md", got)
	}
}

func TestOpeningAFileOfAClosedProjectOpensIt(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "docs/a.md", "# A\n")
	var cfg config.Config
	p, _ := cfg.AddProject(dir, "closed")
	cfg.SetOpen(p.ID, false)
	s := newStateWith(cfg)
	if s.IsOpen(p.ID) {
		t.Fatal("the project is open, want it closed")
	}
	s.OpenDocument(filepath.Join(dir, "docs", "a.md"))
	if !s.IsOpen(p.ID) {
		t.Error("the project is still closed, want it open around the file")
	}
	if got := s.Selected(); got != "docs/a.md" {
		t.Errorf("Selected = %q, want docs/a.md", got)
	}
	if got := len(s.Projects()); got != 1 {
		t.Errorf("Projects = %d, want the one project", got)
	}
}

func TestOpeningAFileThatIsNotMarkdownIsRefused(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes.txt", "text\n")
	for _, path := range []string{
		filepath.Join(root, "notes.txt"),
		filepath.Join(root, "gone.md"),
		root,
	} {
		s.OpenDocument(path)
		if got := len(s.Projects()); got != 1 {
			t.Fatalf("OpenDocument(%q) added a project: %v", path, s.Projects())
		}
		if got := s.Status(); !strings.Contains(got, "无法打开：") {
			t.Errorf("Status = %q, want the refusal of %q", got, path)
		}
	}
}

func TestAGoneFileProjectIsDropped(t *testing.T) {
	s, _ := newState(t)
	file := looseDoc(t, "# Loose\n")
	s.OpenDocument(file)
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if got := len(s.Projects()); got != 1 {
		t.Errorf("Projects = %d, want the file's project gone", got)
	}
	if got := s.Status(); !strings.Contains(got, "文件不可用：") {
		t.Errorf("Status = %q, want the file named as gone", got)
	}
}

func TestAWriteToALooseFileRefreshesTheBody(t *testing.T) {
	s, _ := newState(t)
	file := looseDoc(t, "# Loose\n")
	s.OpenDocument(file)
	id := s.Projects()[1].ID
	writeDoc(t, filepath.Dir(file), "loose.md", "# Looser\n")
	s.applyChanges(id, watch.ChangeSet{Docs: []watch.Change{{Op: mygo.FileWrite, Path: "loose.md"}}})
	if !strings.Contains(s.src, "Looser") {
		t.Errorf("src = %q, want the new body", s.src)
	}
}

func TestTheSameFileInAProjectAndOnItsOwn(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "a.md", "# A\n")
	s, _ := newState(t)
	file := filepath.Join(dir, "a.md")
	s.OpenDocument(file)
	loose := s.Projects()[1].ID
	s.AddProject(dir)
	dirID := s.Projects()[2].ID
	newTester(s)
	rows := 0
	for i := range s.treeOpen.Rows() {
		if row := s.treeOpen.Item(i); row.path == "a.md" {
			rows++
		}
	}
	if rows != 2 {
		t.Errorf("a.md shows %d times, want twice: the project's and its own", rows)
	}
	// Closing the directory project leaves the document shown, through its own
	// entry.
	s.ToggleProject(dirID)
	if got := s.Selected(); got != "a.md" {
		t.Errorf("Selected = %q, want a.md still shown", got)
	}
	if p, ok := s.SelectedProject(); !ok || p.ID != loose {
		t.Errorf("SelectedProject = %v, %v, want the file's own project", p, ok)
	}
}
