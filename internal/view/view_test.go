package view

import (
	"os"
	"path/filepath"
	"slices"
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
	s := New(workspace.New(cfg), cfg, t.TempDir())
	s.SetSyncScan()
	return s, root
}

// newStateWith returns a window showing the projects of cfg.
func newStateWith(cfg config.Config) *State {
	s := New(workspace.New(cfg), cfg, "")
	s.SetSyncScan()
	return s
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
	if !tt.HasText("打开一个目录或 Markdown 文件开始预览") {
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

	s.setTreeKeyOpen(treeKeyOf(s.Trees()[0].project.ID, "notes"), false)
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
	s.SetSystem(&fakeSystem{confirm: true})
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
	s := New(workspace.New(cfg), cfg, t.TempDir())
	s.SetSyncScan()
	return s, alphaRoot, betaRoot
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
	if !tt.HasText("打开一个目录或 Markdown 文件开始预览") {
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
	s.setTreeKeyOpen(treeKeyOf(trees[0].project.ID, "docs"), false)
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
	if err := tt.Click("项目：beta"); err != nil {
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
	if err := tt.Click("项目：beta"); err != nil {
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
	// files is what the window put on the clipboard as files, and filesErr
	// what that write reports.
	files          []string
	filesErr       error
	asked          bool
	confirm        bool
	revealErr      error
	confirmOptions ConfirmOptions
}

func (f *fakeSystem) Reveal(path string) error { f.revealed = path; return f.revealErr }

func (f *fakeSystem) CopyText(text string) { f.copied = text }

func (f *fakeSystem) CopyFiles(paths ...string) error {
	f.files = append([]string(nil), paths...)
	return f.filesErr
}

func (f *fakeSystem) Confirm(options ConfirmOptions) bool {
	f.asked, f.confirmOptions = true, options
	return f.confirm
}

// openPanel clicks the project panel open, unless it is open already.
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
	if err := tt.RightClick("项目：beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制项目路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.copied != betaRoot {
		t.Errorf("copied = %q, want %q", sys.copied, betaRoot)
	}
	if !s.IsOpen(s.Projects()[1].ID) {
		t.Error("the right click closed the project, want it left open")
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
	if err := tt.RightClick("项目：beta"); err != nil {
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
	if err := tt.RightClick("项目：beta"); err != nil {
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
	if err := tt.RightClick("项目：beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !strings.Contains(s.Status(), "文件管理器") {
		t.Errorf("status = %q, want the failure reported", s.Status())
	}
}

// removeFromPanel right clicks a project in the panel and chooses to remove it.
func removeFromPanel(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	openPanel(t, tt)
	if err := tt.RightClick("项目：" + name); err != nil {
		t.Fatalf("RightClick %q: %v", name, err)
	}
	if err := tt.Click("移除项目…"); err != nil {
		t.Fatalf("Click remove: %v", err)
	}
}

func TestRemovingAProjectAsksFirst(t *testing.T) {
	s, _, _ := twoProjects(t)
	sys := &fakeSystem{}
	s.SetSystem(sys)
	beta := s.Projects()[1]
	tt := newTester(s)
	removeFromPanel(t, tt, "beta")
	if !sys.asked {
		t.Fatal("removal went ahead without asking")
	}
	if sys.confirmOptions.ConfirmLabel != "移除" || !strings.Contains(sys.confirmOptions.Message, beta.Name) {
		t.Errorf("confirm options = %+v, want the project named and a 移除 button", sys.confirmOptions)
	}
	if len(s.Projects()) != 2 {
		t.Errorf("Projects = %v, want beta kept when the question is open", s.Projects())
	}
	tt = newTester(s)
	if !tt.HasText("beta") {
		t.Errorf("texts = %q, want beta still listed", tt.Texts())
	}
}

func TestCancelledRemovalKeepsTheProject(t *testing.T) {
	s, _, betaRoot := twoProjects(t)
	sys := &fakeSystem{confirm: false}
	s.SetSystem(sys)
	trees := s.Trees()
	s.selectDocIn(trees[1].project.ID, "")
	writeDoc(t, betaRoot, "only.md", "# Only\n")
	s.Refresh()
	s.selectDocIn(trees[1].project.ID, "only.md")
	tt := newTester(s)
	removeFromPanel(t, tt, "beta")
	if len(s.Projects()) != 2 {
		t.Fatalf("Projects = %v, want beta kept", s.Projects())
	}
	if got := s.Selected(); got != "only.md" {
		t.Errorf("Selected = %q, want beta's document still shown", got)
	}
	if !strings.Contains(s.Status(), "取消") {
		t.Errorf("status = %q, want the cancellation reported", s.Status())
	}
}

func TestConfirmedRemovalDropsTheProject(t *testing.T) {
	s, _, _ := twoProjects(t)
	sys := &fakeSystem{confirm: true}
	s.SetSystem(sys)
	beta := s.Projects()[1]
	tt := newTester(s)
	removeFromPanel(t, tt, "beta")
	if len(s.Projects()) != 1 {
		t.Fatalf("Projects = %v, want beta removed", s.Projects())
	}
	if s.IsOpen(beta.ID) {
		t.Error("beta is open after being removed")
	}
	tt = newTester(s)
	if tt.HasText("beta") {
		t.Errorf("texts = %q, want beta gone from the panel", tt.Texts())
	}
}

func TestRemovalWithoutSystemDoesNothing(t *testing.T) {
	s, _, _ := twoProjects(t)
	beta := s.Projects()[1]
	s.RemoveProject(beta.ID)
	if len(s.Projects()) != 2 {
		t.Errorf("Projects = %v, want beta kept: nothing can confirm a removal", s.Projects())
	}
}

func TestAddProjectsOpensAll(t *testing.T) {
	dir := t.TempDir()
	alphaRoot, betaRoot := filepath.Join(dir, "alpha"), filepath.Join(dir, "beta")
	for _, root := range []string{alphaRoot, betaRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeDoc(t, alphaRoot, "a.md", "# A\n")
	s := newStateWith(config.Config{Version: config.Version, Render: config.RenderNative})
	if got := s.AddProjects([]string{alphaRoot, betaRoot}); got != 2 {
		t.Errorf("AddProjects = %d, want 2", got)
	}
	if got := len(s.Trees()); got != 2 {
		t.Fatalf("Trees = %d, want both projects open", got)
	}
	if p, ok := s.ActiveProject(); !ok || p.Root != betaRoot {
		t.Errorf("ActiveProject = %v, %v, want beta: the last one added", p, ok)
	}
	tt := newTester(s)
	if !tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's document in the tree", tt.Texts())
	}
}

func TestRemovingTheLastOpenProjectLeavesNoneActive(t *testing.T) {
	s, _, _ := twoProjects(t)
	s.SetSystem(&fakeSystem{confirm: true})
	projects := s.Projects()
	s.ToggleProject(projects[1].ID) // beta closed; alpha is the only open one
	s.RemoveProject(projects[0].ID)
	if p, ok := s.ActiveProject(); ok {
		t.Errorf("ActiveProject = %v, %v, want none: the project left is closed", p, ok)
	}
	if got := len(s.Projects()); got != 1 {
		t.Fatalf("Projects = %v, want the closed project kept", s.Projects())
	}
}

func TestProjectRowReportsWhetherItIsOpen(t *testing.T) {
	s, _, _ := twoProjects(t)
	tt := newTester(s)
	openPanel(t, tt)
	info, ok := tt.Access("项目：alpha")
	if !ok {
		t.Fatalf("no accessible row for alpha; texts = %q", tt.Texts())
	}
	if info.Role != ui.RoleCheckBox {
		t.Errorf("role = %v, want a check box: the row opens and closes the project", info.Role)
	}
	if !info.Checked {
		t.Error("alpha is open, want the row reported as checked")
	}

	if err := tt.Click("项目：alpha"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tt = newTester(s)
	info, ok = tt.Access("项目：alpha")
	if !ok {
		t.Fatalf("no accessible row for alpha; texts = %q", tt.Texts())
	}
	if info.Checked {
		t.Error("alpha is closed, want the row reported as unchecked")
	}
}

// TestTreeFileMenuCopiesTheName pins the file's two entries: its name, and its
// absolute path.
func TestTreeFileMenuCopiesTheName(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	for _, want := range []string{"复制文件名称", "复制文件路径"} {
		if !tt.HasText(want) {
			t.Fatalf("texts = %q, want %q in the file's menu", tt.Texts(), want)
		}
	}
	if tt.HasText("在文件管理器中打开") {
		t.Errorf("texts = %q, want no reveal: a file has no folder to show", tt.Texts())
	}
	if err := tt.Click("复制文件名称"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if sys.copied != "a.md" {
		t.Errorf("copied = %q, want a.md", sys.copied)
	}
	if !strings.Contains(s.Status(), "已复制文件名称") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestTreeDirMenuCopiesThePathAndRevealsTheFolder pins the directory's three
// entries, the path being the directory's own.
func TestTreeDirMenuCopiesThePathAndRevealsTheFolder(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	for _, want := range []string{"复制文件名称", "复制文件路径", "在文件管理器中打开"} {
		if !tt.HasText(want) {
			t.Fatalf("texts = %q, want %q in the directory's menu", tt.Texts(), want)
		}
	}
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick again: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes"); sys.revealed != want {
		t.Errorf("revealed = %q, want %q", sys.revealed, want)
	}
}

// TestTreeDirMenuReportsARevealFailure pins that a file manager that cannot
// open the directory says so in the status bar instead of failing silently.
func TestTreeDirMenuReportsARevealFailure(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.SetSystem(&fakeSystem{revealErr: os.ErrNotExist})

	tt := newTester(s)
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("在文件管理器中打开"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !strings.Contains(s.Status(), "文件管理器") {
		t.Errorf("status = %q, want the failure reported", s.Status())
	}
}

// TestTreeMenuWithoutDesktopDoesNothing pins that a tree without a System to
// copy through is a no-op, not a panic.
func TestTreeMenuWithoutDesktopDoesNothing(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if s.Status() != "" {
		t.Errorf("status = %q, want nothing claimed to have been copied", s.Status())
	}
}

// TestCmdCCopiesTheDocumentPath pins Cmd/Ctrl+C: the document shown, its
// absolute path.
func TestCmdCCopiesTheDocumentPath(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.selectDoc("notes/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyC)
	if want := filepath.Join(root, "notes", "a.md"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
	if !strings.Contains(s.Status(), "已复制文件路径") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestCmdCCopiesNothingWithoutADocument pins that the shortcut does not claim
// to copy a path when no document is shown.
func TestCmdCCopiesNothingWithoutADocument(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	tt.Key(ui.Cmd, ui.KeyC)
	if sys.copied != "" {
		t.Errorf("copied = %q, want nothing copied", sys.copied)
	}
	if s.Status() != "" {
		t.Errorf("status = %q, want nothing claimed to have been copied", s.Status())
	}
}

// TestTreeMenuCopiesTheClickedFileWhileADocumentShows pins the tree's path
// entry apart from the title bar's button: with a document shown, the menu
// still copies the file it was opened on.
func TestTreeMenuCopiesTheClickedFileWhileADocumentShows(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	writeDoc(t, root, "notes/b.md", "# B\n")
	s.Refresh()
	s.selectDoc("notes/a.md")
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("b.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制文件路径"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if want := filepath.Join(root, "notes", "b.md"); sys.copied != want {
		t.Errorf("copied = %q, want %q", sys.copied, want)
	}
}

// TestTreeFileMenuCopiesTheFile pins the entry that puts the file itself on the
// clipboard, so that another application can paste it.
func TestTreeFileMenuCopiesTheFile(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if !tt.HasText("复制文件") {
		t.Fatalf("texts = %q, want 复制文件 in the file's menu", tt.Texts())
	}
	if err := tt.Click("复制文件"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	want := []string{filepath.Join(root, "notes", "a.md")}
	if !slices.Equal(sys.files, want) {
		t.Errorf("files = %q, want %q", sys.files, want)
	}
	if !strings.Contains(s.Status(), "已复制文件") {
		t.Errorf("status = %q, want the copy reported", s.Status())
	}
}

// TestTreeDirMenuCopiesTheFolder pins the same entry for a directory, which is
// what pasting a folder into another application expects.
func TestTreeDirMenuCopiesTheFolder(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	sys := &fakeSystem{}
	s.SetSystem(sys)

	tt := newTester(s)
	if err := tt.RightClick("notes"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if !tt.HasText("复制文件夹") {
		t.Fatalf("texts = %q, want 复制文件夹 in the directory's menu", tt.Texts())
	}
	if err := tt.Click("复制文件夹"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	want := []string{filepath.Join(root, "notes")}
	if !slices.Equal(sys.files, want) {
		t.Errorf("files = %q, want %q", sys.files, want)
	}
}

// TestTreeMenuReportsACopyFailure pins that a clipboard write that fails says
// so in the status bar instead of leaving the user waiting for a paste that
// will not come.
func TestTreeMenuReportsACopyFailure(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()
	s.SetSystem(&fakeSystem{filesErr: os.ErrPermission})

	tt := newTester(s)
	if err := tt.RightClick("a.md"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("复制文件"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !strings.Contains(s.Status(), "无法复制") {
		t.Errorf("status = %q, want the failure reported", s.Status())
	}
}

// TestUnfoldingAndFoldingEveryDirectoryOfTheTree pins the sidebar's two bulk
// actions: folding every directory of every open project at once, and
// unfolding them all again.
func TestUnfoldingAndFoldingEveryDirectoryOfTheTree(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	writeDoc(t, root, "draft/deep/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if !tt.HasText("a.md") || !tt.HasText("b.md") {
		t.Fatalf("texts = %q, want both documents before folding", tt.Texts())
	}
	if err := tt.Click("折叠全部目录树"); err != nil {
		t.Fatalf("Click fold: %v", err)
	}
	tt.Frame()
	for _, gone := range []string{"a.md", "b.md"} {
		if tt.HasText(gone) {
			t.Errorf("texts = %q, want %q folded away", tt.Texts(), gone)
		}
	}
	if err := tt.Click("展开全部目录树"); err != nil {
		t.Fatalf("Click unfold: %v", err)
	}
	tt.Frame()
	for _, want := range []string{"a.md", "b.md"} {
		if !tt.HasText(want) {
			t.Errorf("texts = %q, want %q back after unfolding", tt.Texts(), want)
		}
	}
}

// TestBulkActionsSitLeftOfTheProjectSwitcher pins where the tree's bulk actions
// live: in the sidebar header, before the project panel's button.
func TestBulkActionsSitLeftOfTheProjectSwitcher(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	s.Refresh()

	tt := ui.NewTester(func(c *ui.Context) { s.LeftHeader(c) }, 300, 48)
	switch_, ok := tt.Find("切换项目")
	if !ok {
		t.Fatalf("no project switcher; texts = %q", tt.Texts())
	}
	right := switch_.X
	for _, label := range []string{"展开全部目录树", "折叠全部目录树", "在项目树中定位当前文档"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q; texts = %q", label, tt.Texts())
		}
		if r.X+r.W > right+1 {
			t.Errorf("%q sits at X %v..%v, want it left of the switcher at %v", label, r.X, r.X+r.W, right)
		}
		// The two are one row: the icon's own label is shorter than the
		// switcher's, so their boxes only have to overlap vertically.
		if r.Y+r.H < switch_.Y || r.Y > switch_.Y+switch_.H {
			t.Errorf("%q sits at Y %v..%v, want it in the switcher's row %v..%v",
				label, r.Y, r.Y+r.H, switch_.Y, switch_.Y+switch_.H)
		}
	}
}

// TestTheProjectMenuFoldsOneProjectOnly pins the tree's project node: its
// right click folds that project's directories, and leaves the other project's
// alone.
func TestTheProjectMenuFoldsOneProjectOnly(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	if !tt.HasText("b.md") {
		t.Fatalf("texts = %q, want beta's document before folding", tt.Texts())
	}
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("折叠此项目的全部目录"); err != nil {
		t.Fatalf("Click fold: %v", err)
	}
	tt.Frame()
	if tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta folded away", tt.Texts())
	}
	if !tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's document kept", tt.Texts())
	}
	if !tt.HasText("only") {
		t.Errorf("texts = %q, want beta's own directory still shown: folding closed the folders, not the node", tt.Texts())
	}
	if err := tt.RightClick("beta"); err != nil {
		t.Fatalf("RightClick again: %v", err)
	}
	if err := tt.Click("展开此项目的全部目录"); err != nil {
		t.Fatalf("Click unfold: %v", err)
	}
	tt.Frame()
	if !tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta's document back", tt.Texts())
	}
}

// TestTheProjectPanelFoldsOneProject pins the same action where a window with
// one project can reach it: the project panel's own right click, since the tree
// puts no project node on top of a single project.
func TestTheProjectPanelFoldsOneProject(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "notes/a.md", "# A\n")
	s.Refresh()

	tt := newTester(s)
	openPanel(t, tt)
	if err := tt.RightClick("项目：alpha"); err != nil {
		t.Fatalf("RightClick: %v", err)
	}
	if err := tt.Click("折叠此项目的全部目录"); err != nil {
		t.Fatalf("Click fold: %v", err)
	}
	tt.Frame()
	if tt.HasText("a.md") {
		t.Errorf("texts = %q, want the project's directories folded", tt.Texts())
	}
}

// TestTheProjectRowFoldsOnlyItsOwnProject pins the two actions on a project's
// own row, after its name: they fold and unfold that project's directories
// alone, without the other projects'.
func TestTheProjectRowFoldsOnlyItsOwnProject(t *testing.T) {
	s, alphaRoot, betaRoot := twoProjects(t)
	writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
	writeDoc(t, betaRoot, "only/b.md", "# B\n")
	s.Refresh()

	tt := newTester(s)
	name, ok := tt.Find("beta")
	if !ok {
		t.Fatalf("no project node for beta; texts = %q", tt.Texts())
	}
	unfold, ok := tt.Find("展开项目 beta 的全部目录")
	if !ok {
		t.Fatalf("no unfold action on beta's row; texts = %q", tt.Texts())
	}
	if unfold.X <= name.X {
		t.Errorf("unfold sits at X %v, want it after beta's name at %v", unfold.X, name.X)
	}
	if err := tt.Click("折叠项目 beta 的全部目录"); err != nil {
		t.Fatalf("Click fold: %v", err)
	}
	tt.Frame()
	if tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta's document folded away", tt.Texts())
	}
	if !tt.HasText("a.md") {
		t.Errorf("texts = %q, want alpha's document kept", tt.Texts())
	}
	if !tt.HasText("only") {
		t.Errorf("texts = %q, want beta's own directory still shown: the button folded it, not closed the node", tt.Texts())
	}
	if err := tt.Click("展开项目 beta 的全部目录"); err != nil {
		t.Fatalf("Click unfold: %v", err)
	}
	tt.Frame()
	if !tt.HasText("b.md") {
		t.Errorf("texts = %q, want beta's document back", tt.Texts())
	}
}

// TestTheToolbarOpensAProjectOrADocument pins the two entries of the sidebar's
// footer, side by side, before the reload button.
func TestTheToolbarOpensAProjectOrADocument(t *testing.T) {
	s, _ := newState(t)
	tt := ui.NewTester(func(c *ui.Context) { s.LeftToolbar(c) }, 300, 48)
	open, ok := tt.Find("打开项目")
	if !ok {
		t.Fatalf("no 打开项目; texts = %q", tt.Texts())
	}
	doc, ok := tt.Find("打开 Markdown 文件")
	if !ok {
		t.Fatalf("no 打开 Markdown 文件; texts = %q", tt.Texts())
	}
	reload, ok := tt.Find("刷新")
	if !ok {
		t.Fatalf("no 刷新; texts = %q", tt.Texts())
	}
	if doc.X < open.X || doc.X+doc.W > reload.X+1 {
		t.Errorf("打开 Markdown 文件 at X %v..%v, want it between %v and %v", doc.X, doc.X+doc.W, open.X, reload.X)
	}
}
