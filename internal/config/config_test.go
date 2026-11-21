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
