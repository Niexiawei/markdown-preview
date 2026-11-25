package session

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	in := Session{Version: Version,
		Tabs:       []Ref{{Project: "a", Path: "docs/x.md"}, {Project: "a", Path: "y.md"}},
		Active:     Ref{Project: "a", Path: "y.md"},
		TreeClosed: []Ref{{Project: "a", Path: "docs"}, {Project: "b", Path: ""}},
	}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != Version || !slices.Equal(out.Tabs, in.Tabs) ||
		out.Active != in.Active || !slices.Equal(out.TreeClosed, in.TreeClosed) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestMissingEmptyOrBrokenFilesGiveAnEmptySession(t *testing.T) {
	dir := t.TempDir()
	if s, err := Load(dir); err != nil || s.Version != Version ||
		len(s.Tabs) != 0 || len(s.TreeClosed) != 0 {
		t.Errorf("missing file = %+v, %v; want empty session, no error", s, err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := Load(dir); err == nil || s.Version != Version {
		t.Errorf("broken file = %+v, %v; want empty session and an error", s, err)
	}
}

func TestEmptyDirSavesNothingAndLoadsNothing(t *testing.T) {
	if err := Save("", Session{Version: Version}); err != nil {
		t.Errorf(`Save("") = %v, want no-op nil`, err)
	}
	if s, err := Load(""); err != nil || s.Version != Version {
		t.Errorf(`Load("") = %+v, %v; want empty session`, s, err)
	}
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Session{Version: Version}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != FileName {
		t.Errorf("dir holds %v; want only %s", names, FileName)
	}
}
