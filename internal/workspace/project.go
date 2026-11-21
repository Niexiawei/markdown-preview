// Package workspace reads a project's markdown documents: what files it has,
// what they say, their outline, and where a phrase appears. It keeps no UI
// and no window, so the MCP server and the views share it.
package workspace

import (
	"sync"

	"mdpreview/internal/config"
)

// Project is a directory of markdown documents.
type Project = config.Project

// docKey names a document: its project, and its path in it.
type docKey struct {
	project string
	path    string
}

// Workspace is the set of projects the app has open.
type Workspace struct {
	mu  sync.Mutex
	cfg config.Config

	cacheMu sync.Mutex
	cache   map[docKey]*docEntry
	titles  map[docKey]*titleEntry
}

// New returns a workspace of the projects in cfg.
func New(cfg config.Config) *Workspace {
	return &Workspace{cfg: cfg}
}

// SetConfig replaces the projects the workspace reads, as the app adds or
// removes one.
func (w *Workspace) SetConfig(cfg config.Config) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cfg = cfg
}

// Projects returns the projects the workspace has open, in the order they were
// added.
func (w *Workspace) Projects() []Project {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cfg.OpenProjects()
}

// Project returns the project with id.
func (w *Workspace) Project(id string) (Project, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.cfg.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

// Active returns the active project, which is always one the workspace has
// open.
func (w *Workspace) Active() (Project, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.cfg.Projects {
		if p.ID == w.cfg.ActiveID && w.cfg.IsOpen(p.ID) {
			return p, true
		}
	}
	return Project{}, false
}
