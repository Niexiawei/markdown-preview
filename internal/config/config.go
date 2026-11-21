// Package config loads and saves the app's configuration: the projects it
// has open, which one is active, and how documents render.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Version is the schema version of the file Save writes.
const Version = 1

// FileName is the name of the file Load reads and Save writes.
const FileName = "config.json"

// RenderMode is how a document's body renders.
type RenderMode string

// The rendering backends a document's body may use.
const (
	RenderNative  RenderMode = "native"
	RenderWebView RenderMode = "webview"
)

// Project is a directory of markdown documents.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
}

// Config is what the app keeps between runs.
type Config struct {
	Version  int        `json:"version"`
	Projects []Project  `json:"projects"`
	ActiveID string     `json:"active_id"`
	Render   RenderMode `json:"render"`
}

// Load reads the configuration in dir. A directory without one gives the
// defaults rather than an error, as does a file the app does not know yet.
func Load(dir string) (Config, error) {
	c := Config{Version: Version, Render: RenderNative}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{Version: Version, Render: RenderNative}, err
	}
	if c.Version == 0 {
		c.Version = Version
	}
	if c.Render == "" {
		c.Render = RenderNative
	}
	return c, nil
}

// Save writes the configuration into dir, replacing the file in one step so
// that a crash leaves either the old file or the new one.
func (c Config) Save(dir string) error {
	if c.Version == 0 {
		c.Version = Version
	}
	if c.Render == "" {
		c.Render = RenderNative
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
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

// ProjectID is the identity of the project at root: a short hash of its
// cleaned absolute path, so that a renamed directory keeps nothing stale and
// a moved one gets a fresh identity.
func ProjectID(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	return hex.EncodeToString(sum[:8])
}

// AddProject adds the directory at root as a project, unless it is one
// already. It reports whether it added it.
func (c *Config) AddProject(root, name string) (Project, bool) {
	id := ProjectID(root)
	for _, p := range c.Projects {
		if p.ID == id {
			return p, false
		}
	}
	if name == "" {
		name = filepath.Base(root)
	}
	p := Project{ID: id, Name: name, Root: root}
	c.Projects = append(c.Projects, p)
	if c.ActiveID == "" {
		c.ActiveID = id
	}
	return p, true
}

// RemoveProject removes the project with id, and makes the first one left
// active when it was the active one. It reports whether it removed it.
func (c *Config) RemoveProject(id string) bool {
	for i, p := range c.Projects {
		if p.ID != id {
			continue
		}
		c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
		if c.ActiveID == id {
			c.ActiveID = ""
			if len(c.Projects) > 0 {
				c.ActiveID = c.Projects[0].ID
			}
		}
		return true
	}
	return false
}

// SetActive makes the project with id the active one. It reports whether
// there is such a project.
func (c *Config) SetActive(id string) bool {
	for _, p := range c.Projects {
		if p.ID == id {
			c.ActiveID = id
			return true
		}
	}
	return false
}

// Active returns the active project, if there is one.
func (c Config) Active() (Project, bool) {
	for _, p := range c.Projects {
		if p.ID == c.ActiveID {
			return p, true
		}
	}
	return Project{}, false
}
