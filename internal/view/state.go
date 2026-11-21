// Package view builds the app's window: the project switcher, the file tree,
// the outline and the preview. The state belongs to the window's thread:
// anything another goroutine learns (a file changed, a project is gone)
// arrives through Post, which the next frame applies before it builds.
package view

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// State is what the window shows, and what the app keeps of it.
type State struct {
	ws     *workspace.Workspace
	cfgDir string
	win    *mygo.Window

	cfg      config.Config
	activeID string
	tree     *workspace.Node
	// open is the open state of each directory of the tree, by its project
	// path: directories start open, and a click on one's arrow writes here,
	// so that a reloaded tree shows what the user left open.
	open     map[string]*bool
	selected string
	status   string

	// The document shown: its source, its parse, the outline built from it,
	// and what the native markdown view shows it with.
	src       string
	doc       *parser.Document
	headings  []workspace.Heading
	docErr    error
	md        markdown.Options
	anchor    string
	anchorVer uint64
	openURL   func(string)

	// renderMode is the rendering backend the user chose for a document, by
	// its project path; documents without one use the configuration's.
	renderMode map[string]config.RenderMode
	// modeIdx is the segment the status bar shows, which a click writes.
	modeIdx int
	// The web view, made the first time a document asks for it; rev makes
	// each change of the document a new address for it to load.
	wv    *mygo.WebView
	wvURL string
	webOK bool
	rev   int

	// gone remembers the projects whose directory was removed, so that a
	// project found gone is dropped once and not every frame.
	gone map[string]bool

	// watchReady is set once the app runs, when the projects can be
	// watched; watches holds each project's cancel function.
	watchReady bool
	watches    map[string]context.CancelFunc

	// The local MCP server, as the settings panel shows it.
	mcpEnabled bool
	mcpAddr    string
	mcpToken   string
	mcpOpen    bool
	mcpControl func(on bool)

	sidebarW float32
	tocW     float32

	inbox chan func()
}

// New returns the state of a window showing the projects in cfg, whose
// configuration lives in dir.
func New(ws *workspace.Workspace, cfg config.Config, dir string) *State {
	s := &State{
		ws:         ws,
		cfgDir:     dir,
		cfg:        cfg,
		activeID:   cfg.ActiveID,
		open:       map[string]*bool{},
		gone:       map[string]bool{},
		watches:    map[string]context.CancelFunc{},
		renderMode: map[string]config.RenderMode{},
		sidebarW:   300,
		tocW:       240,
		inbox:      make(chan func(), 1024),
	}
	if s.activeID == "" && len(cfg.Projects) > 0 {
		s.activeID = cfg.Projects[0].ID
	}
	s.reloadTree()
	return s
}

// SetWindow gives the state the window it draws in, which it asks for a frame
// when something changed outside the view.
func (s *State) SetWindow(win *mygo.Window) { s.win = win }

// Post runs fn on the window's thread before the next frame. It is how a
// goroutine watching files reaches the state.
func (s *State) Post(fn func()) {
	s.inbox <- fn
	if s.win != nil {
		s.win.Invalidate()
	}
}

// drain applies what other goroutines posted.
func (s *State) drain() {
	for {
		select {
		case fn := <-s.inbox:
			fn()
		default:
			return
		}
	}
}

// ActiveProject returns the project the window shows.
func (s *State) ActiveProject() (workspace.Project, bool) {
	for _, p := range s.cfg.Projects {
		if p.ID == s.activeID {
			return p, true
		}
	}
	return workspace.Project{}, false
}

// Projects returns the projects the window has open.
func (s *State) Projects() []workspace.Project { return s.cfg.Projects }

// Selected returns the document the window shows, as its project names it.
func (s *State) Selected() string { return s.selected }

// Status returns the line the status bar shows besides the rendering mode.
func (s *State) Status() string { return s.status }

// openOf returns the open state of the directory at path, which starts open.
func (s *State) openOf(path string) *bool {
	if p, ok := s.open[path]; ok {
		return p
	}
	b := true
	s.open[path] = &b
	return &b
}

// Tree returns the tree the sidebar shows.
func (s *State) Tree() *workspace.Node { return s.tree }

// SelectProject shows another project.
func (s *State) SelectProject(id string) {
	if !s.cfg.SetActive(id) {
		return
	}
	s.activeID = id
	s.selected = ""
	s.ws.SetConfig(s.cfg)
	s.save()
	s.reloadTree()
}

// OpenProject asks for a directory and shows it as a project.
func (s *State) OpenProject() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:     "打开项目目录",
		Directory: true,
		Parent:    s.win,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	s.AddProject(paths[0])
}

// AddProject adds the directory at root to the projects and shows it.
func (s *State) AddProject(root string) {
	s.gone = map[string]bool{}
	p, added := s.cfg.AddProject(root, filepath.Base(filepath.Clean(root)))
	s.activeID = p.ID
	s.cfg.ActiveID = p.ID
	if added {
		s.selected = ""
	}
	s.ws.SetConfig(s.cfg)
	s.save()
	s.startWatch(p.ID, p.Root)
	s.reloadTree()
}

// SetActive shows the project with id, saving the choice.
func (s *State) SetActive(id string) {
	if s.cfg.SetActive(id) {
		s.activeID = id
		s.save()
	}
}

// RemoveProject removes a project from the window.
func (s *State) RemoveProject(id string) {
	s.stopWatch(id)
	s.cfg.RemoveProject(id)
	if s.cfg.ActiveID == "" && len(s.cfg.Projects) > 0 {
		s.cfg.ActiveID = s.cfg.Projects[0].ID
	}
	s.activeID = s.cfg.ActiveID
	s.selected = ""
	delete(s.gone, id)
	s.ws.SetConfig(s.cfg)
	s.save()
	s.reloadTree()
}

// Refresh reloads the file tree and the document shown.
func (s *State) Refresh() {
	s.reloadTree()
	s.onDocChanged()
}

func (s *State) save() {
	_ = s.cfg.Save(s.cfgDir)
}

// reloadTree reads the active project's tree, dropping projects whose
// directory is gone.
func (s *State) reloadTree() {
	for {
		p, ok := s.ActiveProject()
		if !ok {
			s.tree, s.selected = nil, ""
			return
		}
		tree, err := workspace.BuildTree(p.Root)
		if err == nil {
			s.tree = tree
			if s.selected != "" {
				if _, statErr := s.ws.Stat(p.ID, s.selected); statErr != nil {
					s.selected = ""
					s.status = "文件已被删除"
					s.onDocChanged()
				}
			}
			return
		}
		if s.gone[p.ID] {
			s.tree, s.selected = nil, ""
			s.status = "项目目录不可用：" + p.Root
			return
		}
		s.gone[p.ID] = true
		s.cfg.RemoveProject(p.ID)
		s.activeID = s.cfg.ActiveID
		s.ws.SetConfig(s.cfg)
		s.save()
		if len(s.cfg.Projects) == 0 {
			s.tree, s.selected = nil, ""
			s.status = ""
			return
		}
	}
}

// selectDoc shows a document of the active project.
func (s *State) selectDoc(rel string) {
	if rel == s.selected {
		return
	}
	s.selected = rel
	s.status = ""
	s.onDocChanged()
}

// onDocChanged reloads the parse and the outline of the document shown.
func (s *State) onDocChanged() { s.loadDoc() }

// shortPath is the path shown in the status bar: the project's name in front
// of the document's.
func (s *State) shortPath() string {
	if s.selected == "" {
		return ""
	}
	if p, ok := s.ActiveProject(); ok {
		return p.Name + "/" + s.selected
	}
	return s.selected
}

// trimEllipsis shortens text to at most n runes, ending it with an ellipsis.
func trimEllipsis(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	if n <= 1 {
		return "…"
	}
	return strings.TrimRight(string(runes[:n-1]), " ") + "…"
}
