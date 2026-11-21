package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// newState returns a window showing one project rooted at a temp dir.
func newState(t *testing.T) (*State, string) {
	t.Helper()
	root := t.TempDir()
	var cfg config.Config
	p, _ := cfg.AddProject(root, "alpha")
	cfg.ActiveID = p.ID
	return New(workspace.New(cfg), cfg, t.TempDir()), root
}

// newStateWith returns a window showing the projects of cfg.
func newStateWith(cfg config.Config) *State {
	return New(workspace.New(cfg), cfg, "")
}

func writeDoc(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestViewShowsEmptyState(t *testing.T) {
	s := newStateWith(config.Config{Version: config.Version})
	tt := ui.NewTester(s.View, 1000, 700)
	if !tt.HasText("打开一个目录开始预览 markdown") {
		t.Errorf("texts = %q, want the empty state", tt.Texts())
	}
	if !tt.HasText("打开项目…") {
		t.Errorf("texts = %q, want the open button", tt.Texts())
	}
}

func TestTreeListsMarkdown(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	tt := ui.NewTester(s.View, 1000, 700)
	if !tt.HasText("notes") {
		t.Errorf("texts = %q, want the notes directory", tt.Texts())
	}
	if !tt.HasText("a.md") {
		t.Errorf("texts = %q, want a.md", tt.Texts())
	}
}

func TestTreeKeepsOpenStateAcrossRefresh(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	tt := ui.NewTester(s.View, 1000, 700)
	if !tt.HasText("a.md") {
		t.Fatalf("texts = %q, want a.md before collapsing", tt.Texts())
	}

	*s.openOf("notes") = false
	tt = ui.NewTester(s.View, 1000, 700)
	if tt.HasText("a.md") {
		t.Errorf("texts = %q, want notes collapsed", tt.Texts())
	}

	s.Refresh()
	tt = ui.NewTester(s.View, 1000, 700)
	if tt.HasText("a.md") {
		t.Errorf("texts = %q, want notes still collapsed after a reload", tt.Texts())
	}
}

func TestClickingDocShowsIt(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	tt := ui.NewTester(s.View, 1000, 700)
	if err := tt.Click("a.md"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.Selected(); got != "notes/a.md" {
		t.Errorf("Selected = %q, want notes/a.md", got)
	}
	if !tt.HasText("alpha/notes/a.md") {
		t.Errorf("texts = %q, want the path in the toolbar", tt.Texts())
	}
}

func TestToolbarShowsProjectNames(t *testing.T) {
	var cfg config.Config
	dir := t.TempDir()
	alpha, _ := cfg.AddProject(filepath.Join(dir, "alpha"), "alpha")
	cfg.AddProject(filepath.Join(dir, "beta"), "beta")
	cfg.ActiveID = alpha.ID
	os.MkdirAll(filepath.Join(dir, "alpha"), 0o755)
	os.MkdirAll(filepath.Join(dir, "beta"), 0o755)
	s := newStateWith(cfg)
	tt := ui.NewTester(s.View, 1000, 700)
	if !tt.HasText("alpha ▾") {
		t.Fatalf("texts = %q, want the active project's name", tt.Texts())
	}
	if err := tt.Click("alpha ▾"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !tt.HasText("beta") {
		t.Errorf("texts = %q, want beta in the menu", tt.Texts())
	}
}

func TestChoosingProjectSwitchesIt(t *testing.T) {
	var cfg config.Config
	dir := t.TempDir()
	alphaRoot := filepath.Join(dir, "alpha")
	betaRoot := filepath.Join(dir, "beta")
	alpha, _ := cfg.AddProject(alphaRoot, "alpha")
	beta, _ := cfg.AddProject(betaRoot, "beta")
	cfg.ActiveID = alpha.ID
	if err := os.MkdirAll(alphaRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s := newStateWith(cfg)
	tt := ui.NewTester(s.View, 1000, 700)
	if err := tt.Click("alpha ▾"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if err := tt.ChooseMenuItem("beta"); err != nil {
		t.Fatalf("ChooseMenuItem: %v", err)
	}
	tt = ui.NewTester(s.View, 1000, 700)
	if p, ok := s.ActiveProject(); !ok || p.ID != beta.ID {
		t.Errorf("ActiveProject = %v, %v, want beta", p, ok)
	}
	if !tt.HasText("only.md") {
		t.Errorf("texts = %q, want beta's document", tt.Texts())
	}
}

func TestRemoveProjectFallsBackToNext(t *testing.T) {
	var cfg config.Config
	dir := t.TempDir()
	alpha, _ := cfg.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := cfg.AddProject(filepath.Join(dir, "beta"), "beta")
	cfg.ActiveID = alpha.ID
	os.MkdirAll(filepath.Join(dir, "alpha"), 0o755)
	os.MkdirAll(filepath.Join(dir, "beta"), 0o755)
	s := newStateWith(cfg)
	s.RemoveProject(alpha.ID)
	if p, ok := s.ActiveProject(); !ok || p.ID != beta.ID {
		t.Errorf("ActiveProject = %v, %v, want beta", p, ok)
	}
	if len(s.Projects()) != 1 {
		t.Errorf("Projects = %v, want 1", s.Projects())
	}
}

func TestGoneProjectIsDropped(t *testing.T) {
	var cfg config.Config
	dir := t.TempDir()
	alphaRoot := filepath.Join(dir, "alpha")
	betaRoot := filepath.Join(dir, "beta")
	alpha, _ := cfg.AddProject(alphaRoot, "alpha")
	beta, _ := cfg.AddProject(betaRoot, "beta")
	cfg.ActiveID = alpha.ID
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s := newStateWith(cfg)
	if err := os.RemoveAll(alphaRoot); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if p, ok := s.ActiveProject(); !ok || p.ID != beta.ID {
		t.Errorf("ActiveProject = %v, %v, want beta", p, ok)
	}
	if len(s.Projects()) != 1 {
		t.Errorf("Projects = %v, want alpha dropped", s.Projects())
	}
	if !strings.Contains(s.Status(), "") {
		t.Logf("status = %q", s.Status())
	}
}
