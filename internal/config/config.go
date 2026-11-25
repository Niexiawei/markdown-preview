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

// Version is the schema version of the file Save writes. Version 2 added
// OpenIDs, the projects the window shows.
const Version = 2

// FileName is the name of the file Load reads and Save writes.
const FileName = "config.json"

// RenderMode is how a document's body renders.
type RenderMode string

// The rendering backends a document's body may use.
const (
	RenderNative  RenderMode = "native"
	RenderWebView RenderMode = "webview"
)

// Project is a directory of markdown documents, or one markdown file: File is
// the document and Root the directory it sits in, so a single file is a
// project of its own and can sit in the tree beside the directory projects.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
	File string `json:"file,omitempty"`
}

// MCP is the app's local Model Context Protocol server: whether it runs, the
// port it listens on, and the token callers authenticate with.
type MCP struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

// MCPPort is the port the server listens on by default.
const MCPPort = 8765

// Config is what the app keeps between runs.
type Config struct {
	Version  int        `json:"version"`
	Projects []Project  `json:"projects"`
	OpenIDs  []string   `json:"open_ids"`
	ActiveID string     `json:"active_id"`
	Render   RenderMode `json:"render"`
	MCP      MCP        `json:"mcp"`
}

// Load reads the configuration in dir. A directory without one gives the
// defaults rather than an error, as does a file the app does not know yet.
func Load(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return defaults(), nil
	}
	if err != nil {
		return defaults(), err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return defaults(), err
	}
	c.normalize()
	return c, nil
}

// defaults is the configuration of a directory without one yet.
func defaults() Config {
	return Config{Version: Version, Render: RenderNative, MCP: MCP{Port: MCPPort}, OpenIDs: []string{}}
}

// normalize applies the defaults and drops what the file no longer explains: a
// file without open_ids, which every file before version 2 is, has every
// project open, and OpenIDs never names a project that is gone. It also keeps
// ActiveID on an open project, since the MCP server falls back to it.
func (c *Config) normalize() {
	if c.Version < Version {
		c.Version = Version
	}
	if c.Render == "" {
		c.Render = RenderNative
	}
	if c.MCP.Port == 0 {
		c.MCP.Port = MCPPort
	}
	known := make(map[string]bool, len(c.Projects))
	for _, p := range c.Projects {
		known[p.ID] = true
	}
	if c.OpenIDs == nil {
		c.OpenIDs = make([]string, 0, len(c.Projects))
		for _, p := range c.Projects {
			c.OpenIDs = append(c.OpenIDs, p.ID)
		}
	} else {
		kept := c.OpenIDs[:0]
		for _, id := range c.OpenIDs {
			if known[id] {
				kept = append(kept, id)
			}
		}
		c.OpenIDs = kept
	}
	if !c.IsOpen(c.ActiveID) {
		c.ActiveID = ""
		if len(c.OpenIDs) > 0 {
			c.ActiveID = c.OpenIDs[0]
		}
	}
}

// Save writes the configuration into dir, replacing the file in one step so
// that a crash leaves either the old file or the new one.
func (c Config) Save(dir string) error {
	c.normalize()
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
	c.OpenIDs = append(c.OpenIDs, id)
	if c.ActiveID == "" {
		c.ActiveID = id
	}
	return p, true
}

// AddFileProject adds the document at path as a project of its own, unless it is one
// already. It reports whether it added it.
func (c *Config) AddFileProject(path string) (Project, bool) {
	id := ProjectID(path)
	for _, p := range c.Projects {
		if p.ID == id {
			return p, false
		}
	}
	p := Project{ID: id, Name: filepath.Base(path), Root: filepath.Dir(path), File: path}
	c.Projects = append(c.Projects, p)
	c.OpenIDs = append(c.OpenIDs, id)
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
		open := c.OpenIDs[:0]
		for _, o := range c.OpenIDs {
			if o != id {
				open = append(open, o)
			}
		}
		c.OpenIDs = open
		if c.ActiveID == id {
			c.ActiveID = ""
			if len(c.OpenIDs) > 0 {
				c.ActiveID = c.OpenIDs[0]
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

// IsOpen reports whether the project with id is open: one of the projects the
// window shows, and watches for changes.
func (c Config) IsOpen(id string) bool {
	for _, open := range c.OpenIDs {
		if open == id {
			return true
		}
	}
	return false
}

// SetOpen opens or closes the project with id, keeping ActiveID on an open
// project. It reports whether there is such a project.
func (c *Config) SetOpen(id string, open bool) bool {
	known := false
	for _, p := range c.Projects {
		if p.ID == id {
			known = true
			break
		}
	}
	if !known {
		return false
	}
	if open {
		if !c.IsOpen(id) {
			c.OpenIDs = append(c.OpenIDs, id)
		}
	} else {
		kept := c.OpenIDs[:0]
		for _, o := range c.OpenIDs {
			if o != id {
				kept = append(kept, o)
			}
		}
		c.OpenIDs = kept
	}
	if !c.IsOpen(c.ActiveID) {
		c.ActiveID = ""
		if len(c.OpenIDs) > 0 {
			c.ActiveID = c.OpenIDs[0]
		}
	}
	return true
}

// OpenProjects returns the projects that are open, in the order they were
// added.
func (c Config) OpenProjects() []Project {
	out := make([]Project, 0, len(c.OpenIDs))
	for _, p := range c.Projects {
		if c.IsOpen(p.ID) {
			out = append(out, p)
		}
	}
	return out
}
