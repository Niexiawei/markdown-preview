// Package session keeps the snapshot of the window's last session in a file
// of its own beside the configuration: the tabs it had open, which one was
// active, and the directories of the sidebar's tree the user had folded.
// It reads and writes JSON alone, with no UI, so it tests without a window.
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Version is the schema version of the file Save writes.
const Version = 1

// FileName is the name of the file Load reads and Save writes.
const FileName = "session.json"

// Ref is the identity of one remembered thing: a tab is a document of a
// project, a folded row of the tree is a directory of one (a project's own
// node is the empty path).
type Ref struct {
	Project string `json:"project"`
	Path    string `json:"path"`
}

// Session is the snapshot of the window's last session: the tabs in the order
// they sat in the bar, the active one, and the directories the user folded.
type Session struct {
	Version    int   `json:"version"`
	Tabs       []Ref `json:"tabs"`
	Active     Ref   `json:"active"`
	TreeClosed []Ref `json:"tree_closed"`
}

// Load reads the session in dir. A directory without one, an empty dir, and a
// file that cannot be read all give an empty session — the window then starts
// as it would the first time — with the error reported for a broken file, so
// that writing it back does not spread the damage.
func Load(dir string) (Session, error) {
	if dir == "" {
		return Session{Version: Version}, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return Session{Version: Version}, nil
	}
	if err != nil {
		return Session{Version: Version}, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{Version: Version}, err
	}
	return s, nil
}

// Save writes the session into dir, replacing the file in one step so that a
// crash leaves either the old file or the new one. An empty dir keeps it in
// memory alone, as a window without a user data directory does, and saves
// nothing.
func Save(dir string, s Session) error {
	if dir == "" {
		return nil
	}
	s.Version = Version
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, FileName+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, FileName))
}
