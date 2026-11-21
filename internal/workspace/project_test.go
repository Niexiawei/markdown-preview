package workspace

import (
	"path/filepath"
	"testing"

	"mdpreview/internal/config"
)

func TestProjectsListsOnlyOpenOnes(t *testing.T) {
	dir := t.TempDir()
	var cfg config.Config
	alpha, _ := cfg.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := cfg.AddProject(filepath.Join(dir, "beta"), "beta")
	cfg.SetOpen(beta.ID, false)
	ws := New(cfg)
	got := ws.Projects()
	if len(got) != 1 || got[0].ID != alpha.ID {
		t.Errorf("Projects = %v, want [alpha]: beta is closed", got)
	}
}

func TestActiveIsAnOpenProject(t *testing.T) {
	dir := t.TempDir()
	var cfg config.Config
	alpha, _ := cfg.AddProject(filepath.Join(dir, "alpha"), "alpha")
	beta, _ := cfg.AddProject(filepath.Join(dir, "beta"), "beta")
	cfg.SetOpen(beta.ID, false)
	// A hand-written file can still name a closed project; the workspace must
	// not offer it as the default the MCP tools read with.
	cfg.ActiveID = beta.ID
	ws := New(cfg)
	if p, ok := ws.Active(); ok {
		t.Errorf("Active = %v, %v, want none: %q is closed", p, ok, beta.ID)
	}
	cfg.ActiveID = alpha.ID
	ws.SetConfig(cfg)
	if p, ok := ws.Active(); !ok || p.ID != alpha.ID {
		t.Errorf("Active = %v, %v, want alpha", p, ok)
	}
}
