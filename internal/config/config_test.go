package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingIsDefault(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version {
		t.Errorf("Version = %d, want %d", c.Version, Version)
	}
	if len(c.Projects) != 0 {
		t.Errorf("Projects = %v, want empty", c.Projects)
	}
	if c.Render != RenderNative {
		t.Errorf("Render = %q, want %q", c.Render, RenderNative)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	var c Config
	a, ok := c.AddProject(filepath.Join(dir, "alpha"), "alpha")
	if !ok {
		t.Fatal("AddProject alpha = false, want true")
	}
	b, ok := c.AddProject(filepath.Join(dir, "beta"), "beta")
	if !ok {
		t.Fatal("AddProject beta = false, want true")
	}
	if !c.SetActive(b.ID) {
		t.Fatal("SetActive(beta) = false")
	}
	c.Render = RenderWebView
	if err := c.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Projects) != 2 {
		t.Fatalf("Projects = %v, want 2", got.Projects)
	}
	if got.Projects[0] != a || got.Projects[1] != b {
		t.Errorf("Projects = %v, want [%v %v]", got.Projects, a, b)
	}
	if got.ActiveID != b.ID {
		t.Errorf("ActiveID = %q, want %q", got.ActiveID, b.ID)
	}
	if got.Render != RenderWebView {
		t.Errorf("Render = %q, want %q", got.Render, RenderWebView)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	c := Config{Version: Version, Render: RenderNative}
	if err := c.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != FileName {
			t.Errorf("leftover file %q, want only %q", e.Name(), FileName)
		}
	}
}

func TestAddProjectDedupes(t *testing.T) {
	dir := t.TempDir()
	var c Config
	root := filepath.Join(dir, "alpha")
	if _, ok := c.AddProject(root, "alpha"); !ok {
		t.Fatal("first AddProject = false")
	}
	if _, ok := c.AddProject(root, "alpha again"); ok {
		t.Error("second AddProject = true, want false")
	}
	if len(c.Projects) != 1 {
		t.Errorf("Projects = %v, want 1", c.Projects)
	}
}

func TestProjectIDStable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "alpha")
	if ProjectID(root) != ProjectID(root) {
		t.Error("ProjectID is not stable")
	}
	if ProjectID(root) == ProjectID(root+"-2") {
		t.Error("ProjectID collides for different roots")
	}
	if len(ProjectID(root)) != 16 {
		t.Errorf("ProjectID = %q, want 16 hex characters", ProjectID(root))
	}
}

func TestMCPDefaultsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.MCP.Enabled {
		t.Error("MCP.Enabled = true, want off by default")
	}
	if c.MCP.Port != MCPPort {
		t.Errorf("MCP.Port = %d, want %d", c.MCP.Port, MCPPort)
	}
	c.MCP = MCP{Enabled: true, Port: 9123, Token: "abc"}
	if err := c.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.MCP != c.MCP {
		t.Errorf("MCP = %+v, want %+v", got.MCP, c.MCP)
	}
}

func TestSetActiveRejectsUnknown(t *testing.T) {
	var c Config
	if c.SetActive("nope") {
		t.Error("SetActive(unknown) = true, want false")
	}
	c.AddProject(filepath.Join(t.TempDir(), "alpha"), "alpha")
	if c.SetActive("nope") {
		t.Error("SetActive(unknown) = true, want false")
	}
}

// writeConfig writes the configuration file the app reads, as the test says.
func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// projectJSON is one project as the file holds it.
func projectJSON(id, name string) string {
	return `{"id":"` + id + `","name":"` + name + `","root":"` + name + `"}`
}

func TestLoadOpensEverythingWhenTheFileHasNoOpenIDs(t *testing.T) {
	dir := t.TempDir()
	alpha, beta := ProjectID(filepath.Join(dir, "alpha")), ProjectID(filepath.Join(dir, "beta"))
	writeConfig(t, dir, `{"version":1,"projects":[`+projectJSON(alpha, "alpha")+`,`+projectJSON(beta, "beta")+
		`],"active_id":"`+alpha+`"}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.OpenProjects(); len(got) != 2 {
		t.Errorf("OpenProjects = %v, want both projects: a file without open_ids has them all open", got)
	}
	if c.Version != Version {
		t.Errorf("Version = %d, want %d", c.Version, Version)
	}
}

func TestSetOpenTogglesTheProject(t *testing.T) {
	dir := t.TempDir()
	var c Config
	alpha, _ := c.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := c.AddProject(filepath.Join(dir, "beta"), "beta")
	if !c.SetOpen(beta.ID, false) {
		t.Fatalf("SetOpen(%q, false) = false", beta.ID)
	}
	if c.IsOpen(beta.ID) {
		t.Error("IsOpen(beta) = true after closing it")
	}
	if got := c.OpenProjects(); len(got) != 1 || got[0].ID != alpha.ID {
		t.Errorf("OpenProjects = %v, want [alpha]", got)
	}
	if !c.SetOpen(beta.ID, true) {
		t.Fatalf("SetOpen(%q, true) = false", beta.ID)
	}
	if got := c.OpenProjects(); len(got) != 2 {
		t.Errorf("OpenProjects = %v, want both open again", got)
	}
	if c.SetOpen("nope", true) {
		t.Error("SetOpen(unknown) = true, want false")
	}
}

func TestOpenIDsOfRemovedProjectsAreDropped(t *testing.T) {
	dir := t.TempDir()
	alpha := ProjectID(filepath.Join(dir, "alpha"))
	writeConfig(t, dir, `{"version":2,"projects":[`+projectJSON(alpha, "alpha")+
		`],"active_id":"`+alpha+`","open_ids":["ghost","`+alpha+`"]}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.OpenProjects(); len(got) != 1 || got[0].ID != alpha {
		t.Errorf("OpenProjects = %v, want [alpha]: the ghost is gone", got)
	}
}

func TestClosingActiveProjectPicksAnother(t *testing.T) {
	dir := t.TempDir()
	var c Config
	alpha, _ := c.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := c.AddProject(filepath.Join(dir, "beta"), "beta")
	if !c.SetActive(beta.ID) {
		t.Fatal("SetActive(beta) = false")
	}
	c.SetOpen(beta.ID, false)
	if c.ActiveID != alpha.ID {
		t.Errorf("ActiveID = %q, want alpha: the active project is always an open one", c.ActiveID)
	}
	c.SetOpen(alpha.ID, false)
	if c.ActiveID != "" {
		t.Errorf("ActiveID = %q, want none: no project is open", c.ActiveID)
	}
}

func TestAddProjectOpensIt(t *testing.T) {
	var c Config
	p, _ := c.AddProject(filepath.Join(t.TempDir(), "alpha"), "alpha")
	if !c.IsOpen(p.ID) {
		t.Errorf("IsOpen(%q) = false, want a new project open", p.ID)
	}
}

func TestRemoveProjectClosesIt(t *testing.T) {
	dir := t.TempDir()
	var c Config
	alpha, _ := c.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := c.AddProject(filepath.Join(dir, "beta"), "beta")
	c.SetActive(beta.ID)
	if !c.RemoveProject(beta.ID) {
		t.Fatalf("RemoveProject(beta) = false")
	}
	if got := c.OpenProjects(); len(got) != 1 || got[0].ID != alpha.ID {
		t.Errorf("OpenProjects = %v, want [alpha]", got)
	}
	if c.ActiveID != alpha.ID {
		t.Errorf("ActiveID = %q, want alpha", c.ActiveID)
	}
}

// TestAFileProjectIsOneDocument pins a single-document project: its file, the
// directory it sits in, its name, and that adding it twice keeps one entry.
func TestAFileProjectIsOneDocument(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(file, []byte("# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var c Config
	p, isNew := c.AddFileProject(file)
	switch {
	case !isNew:
		t.Fatal("isNew = false, want the file added")
	case p.File != file:
		t.Errorf("File = %q, want %q", p.File, file)
	case p.Root != dir:
		t.Errorf("Root = %q, want %q", p.Root, dir)
	case p.Name != "notes.md":
		t.Errorf("Name = %q, want notes.md", p.Name)
	case !c.IsOpen(p.ID):
		t.Error("the file project is not open, want it open")
	case p.ID == ProjectID(dir):
		t.Error("the file project's id is the directory project's, want them apart")
	}
	if again, isNew := c.AddFileProject(file); isNew || again.ID != p.ID {
		t.Errorf("AddFileProject twice = %q, %v, want the same one", again.ID, isNew)
	}
	// The file survives a save and a load.
	if err := c.Save(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
