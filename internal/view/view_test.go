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

// newTester builds the window in memory, without a window.
func newTester(s *State) *ui.Tester { return ui.NewTester(s.View, 1000, 700) }

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
	if !tt.HasText("打开项目") {
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

	*s.openOf(s.Trees()[0].project.ID + ":notes") = false
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
	if !tt.HasText("alpha") {
		t.Fatalf("texts = %q, want the active project's name", tt.Texts())
	}
	if err := tt.Click("切换项目"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !tt.HasText("beta") {
		t.Errorf("texts = %q, want beta in the menu", tt.Texts())
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

func TestMCPSettingsShowsAddressAndToken(t *testing.T) {
	s, _ := newState(t)
	s.SetMCPStatus(true, "http://127.0.0.1:8765/mcp", "secret-token")
	s.mcpOpen = true
	tt := newTester(s)
	for _, want := range []string{"本地 MCP server", "http://127.0.0.1:8765/mcp", "secret-token", "复制 token"} {
		if !tt.HasText(want) {
			t.Errorf("texts = %q, want %q", tt.Texts(), want)
		}
	}
}

func TestMCPSettingsOffByDefault(t *testing.T) {
	s, _ := newState(t)
	s.mcpOpen = true
	tt := newTester(s)
	if !tt.HasText("关闭中。打开后，agent 可以列出和检索当前打开的项目的 markdown 文档。") {
		t.Errorf("texts = %q, want the off state", tt.Texts())
	}
}

func TestMCPControlCalledOnToggle(t *testing.T) {
	s, _ := newState(t)
	var got []bool
	s.SetMCPControl(func(on bool) { got = append(got, on) })
	s.mcpOpen = true
	tt := newTester(s)
	if err := tt.Click("MCP server"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if len(got) != 1 || got[0] != true {
		t.Errorf("control called with %v, want [true]", got)
	}
}

// twoProjects returns a window with alpha and beta open, and their roots.
func twoProjects(t *testing.T) (*State, string, string) {
	t.Helper()
	dir := t.TempDir()
	alphaRoot, betaRoot := filepath.Join(dir, "alpha"), filepath.Join(dir, "beta")
	var cfg config.Config
	alpha, _ := cfg.AddProject(alphaRoot, "alpha")
	cfg.AddProject(betaRoot, "beta")
	cfg.SetActive(alpha.ID)
	for _, root := range []string{alphaRoot, betaRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return New(workspace.New(cfg), cfg, t.TempDir()), alphaRoot, betaRoot
}

func TestTreeGroupsByProjectWhenSeveralAreOpen(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s.Refresh()
	if got := len(s.Trees()); got != 2 {
		t.Fatalf("Trees = %d, want 2", got)
	}
	tt := newTester(s)
	for _, want := range []string{"alpha", "beta", "a.md", "only.md"} {
		if !tt.HasText(want) {
			t.Errorf("texts = %q, want %q", tt.Texts(), want)
		}
	}
}

func TestSingleOpenProjectHasNoProjectLevel(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	trees := s.Trees()
	if len(trees) != 1 {
		t.Fatalf("Trees = %d, want 1", len(trees))
	}
	var names []string
	for _, child := range trees[0].root.Children {
		names = append(names, child.Name)
	}
	if len(names) != 1 || names[0] != "notes" {
		t.Errorf("top level = %v, want [notes]: one project adds no project node", names)
	}
}

func TestNoOpenProjectShowsTheEmptyState(t *testing.T) {
	dir := t.TempDir()
	alphaRoot, betaRoot := filepath.Join(dir, "alpha"), filepath.Join(dir, "beta")
	var cfg config.Config
	alpha, _ := cfg.AddProject(alphaRoot, "alpha")
	beta, _ := cfg.AddProject(betaRoot, "beta")
	cfg.SetOpen(alpha.ID, false)
	cfg.SetOpen(beta.ID, false)
	for _, root := range []string{alphaRoot, betaRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := newStateWith(cfg)
	tt := newTester(s)
	if !tt.HasText("打开一个目录开始预览 markdown") {
		t.Errorf("texts = %q, want the empty state", tt.Texts())
	}
	if got := s.Selected(); got != "" {
		t.Errorf("Selected = %q, want none", got)
	}
}

func TestSamePathInTwoProjectsLoadsItsOwnBody(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# ALPHA body\n")
	writeDoc(t, betaRoot, "docs/a.md", "# BETA body\n")
	s.Refresh()
	trees := s.Trees()
	if len(trees) != 2 {
		t.Fatalf("Trees = %d, want 2", len(trees))
	}
	s.selectDocIn(trees[1].project.ID, "docs/a.md")
	if got := s.Selected(); got != "docs/a.md" {
		t.Errorf("Selected = %q, want docs/a.md", got)
	}
	if !strings.Contains(s.src, "BETA body") {
		t.Errorf("src = %q, want the body of beta's document", s.src)
	}
	if p, ok := s.SelectedProject(); !ok || p.ID != trees[1].project.ID {
		t.Errorf("SelectedProject = %v, %v, want beta", p, ok)
	}
	if p, ok := s.ActiveProject(); !ok || p.ID != trees[1].project.ID {
		t.Errorf("ActiveProject = %v, %v, want beta: the MCP default follows", p, ok)
	}
	tt := newTester(s)
	if !tt.HasText("beta/docs/a.md") {
		t.Errorf("texts = %q, want beta's path in the title", tt.Texts())
	}
	s.selectDocIn(trees[0].project.ID, "docs/a.md")
	if !strings.Contains(s.src, "ALPHA body") {
		t.Errorf("src = %q, want the body of alpha's document", s.src)
	}
}

func TestClickingDocInAnotherProjectShowsIt(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s.Refresh()
	tt := newTester(s)
	if err := tt.Click("only.md"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if got := s.Selected(); got != "only.md" {
		t.Fatalf("Selected = %q, want only.md", got)
	}
	if p, ok := s.SelectedProject(); !ok || p.Root != betaRoot {
		t.Errorf("SelectedProject = %v, %v, want beta", p, ok)
	}
	if p, ok := s.ws.Active(); !ok || p.Root != betaRoot {
		t.Errorf("workspace active = %v, %v, want beta: the MCP tools read it", p, ok)
	}
}

func TestDirectoriesOfTwoProjectsOpenApart(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "docs/b.md", "# B\n")
	s.Refresh()
	trees := s.Trees()
	if len(trees) != 2 {
		t.Fatalf("Trees = %d, want 2", len(trees))
	}
	*s.openOf(trees[0].project.ID + ":docs") = false
	tt := newTester(s)
	if tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's docs collapsed", tt.Texts())
	}
	if !tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta's docs still open", tt.Texts())
	}
}

func TestGoneProjectKeepsTheOthers(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "a.md", "# A\n")
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s.Refresh()
	trees := s.Trees()
	s.selectDocIn(trees[1].project.ID, "only.md")
	if err := os.RemoveAll(alphaRoot); err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if got := len(s.Projects()); got != 1 {
		t.Fatalf("Projects = %v, want alpha dropped", s.Projects())
	}
	if got := len(s.Trees()); got != 1 || s.Trees()[0].project.Root != betaRoot {
		t.Fatalf("Trees = %v, want only beta", s.Trees())
	}
	if got := s.Selected(); got != "only.md" {
		t.Errorf("Selected = %q, want only.md: another project's document stays", got)
	}
	tt := newTester(s)
	if !tt.HasText("only.md") {
		t.Errorf("texts = %q, want beta's document still in the tree", tt.Texts())
	}
	if !strings.Contains(s.Status(), "不可用") {
		t.Errorf("status = %q, want the gone project named", s.Status())
	}
}

func TestPanelTogglesProjectOpen(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s.Refresh()
	beta := s.Projects()[1].ID

	tt := newTester(s)
	if err := tt.Click("切换项目"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if err := tt.Click("beta"); err != nil {
		t.Fatalf("Click beta: %v", err)
	}
	if s.IsOpen(beta) {
		t.Error("beta is open, want it closed by the click")
	}
	tt = newTester(s)
	if tt.HasText("only.md") {
		t.Errorf("texts = %q, want beta's document gone from the tree", tt.Texts())
	}
	if !s.projectsOpen {
		t.Error("panel closed, want it to stay open while projects are toggled")
	}
	if err := tt.Click("beta"); err != nil {
		t.Fatalf("Click beta again: %v", err)
	}
	if !s.IsOpen(beta) {
		t.Error("beta is closed, want the second click to open it")
	}
	if len(s.Trees()) != 2 {
		t.Errorf("Trees = %d, want both projects again", len(s.Trees()))
	}
}

func TestPanelListsClosedProjectsToo(t *testing.T) {
	dir := t.TempDir()
	alphaRoot, betaRoot := filepath.Join(dir, "alpha"), filepath.Join(dir, "beta")
	var cfg config.Config
	cfg.AddProject(alphaRoot, "alpha")
	beta, _ := cfg.AddProject(betaRoot, "beta")
	cfg.SetOpen(beta.ID, false)
	for _, root := range []string{alphaRoot, betaRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := newStateWith(cfg)
	tt := newTester(s)
	if err := tt.Click("切换项目"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !tt.HasText("beta") {
		t.Errorf("texts = %q, want the closed project listed so it can be opened", tt.Texts())
	}
}

func TestClosingProjectStopsWatching(t *testing.T) {
	s, _, _ := twoProjects(t)
	beta := s.Projects()[1].ID
	cancelled := false
	s.watches[beta] = func() { cancelled = true }
	s.ToggleProject(beta)
	if !cancelled {
		t.Error("beta's watch was not cancelled")
	}
	if _, ok := s.watches[beta]; ok {
		t.Error("beta is still watched after closing it")
	}
}

// fakeSystem records what the window asked of the desktop.
type fakeSystem struct {
	revealed, copied string
	asked            bool
	confirm          bool
	revealErr        error
	confirmOptions   ConfirmOptions
}

func (f *fakeSystem) Reveal(path string) error { f.revealed = path; return f.revealErr }

func (f *fakeSystem) CopyText(text string) { f.copied = text }

func (f *fakeSystem) Confirm(options ConfirmOptions) bool {
	f.asked, f.confirmOptions = true, options
	return f.confirm
}

// openPanel clicks the project panel open.
func openPanel(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("切换项目"); err != nil {
		t.Fatalf("Click the project panel: %v", err)
	}
}

func TestProjectMenuCopiesThePath(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	sys := &fakeSystem{}
	s.SetSystem(sys)
	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制项目路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.copied != betaRoot {
		t.Errorf("copied = %q, want %q", sys.copied, betaRoot)
	}
	if !strings.Contains(s.Status(), "已复制") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

func TestProjectMenuRevealsTheFolder(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	sys := &fakeSystem{}
	s.SetSystem(sys)
	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.revealed != betaRoot {
		t.Errorf("revealed = %q, want %q", sys.revealed, betaRoot)
	}
}

func TestProjectMenuActsOnTheClickedProject(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	sys := &fakeSystem{}
	s.SetSystem(sys)
	if p, _ := s.ActiveProject(); p.Root != alphaRoot {
		t.Fatalf("active = %q, want alpha before the click", p.Root)
	}
	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制项目路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.copied != betaRoot {
		t.Errorf("copied = %q, want beta's root, not the active project's", sys.copied)
	}
}

func TestProjectMenuRevealFailureIsReported(t *testing.T) {
	s, _, _ := twoProjects(t)
	sys := &fakeSystem{revealErr: os.ErrNotExist}
	s.SetSystem(sys)
	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !strings.Contains(s.Status(), "文件管理器") {
		t.Errorf("status = %q, want the failure reported", s.Status())
	}
}
