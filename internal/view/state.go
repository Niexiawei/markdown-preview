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
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// docSel names the document the window shows: the project it belongs to, and
// its path in it.
type docSel struct {
	project string
	path    string
}

// empty reports whether no document is selected.
func (d docSel) empty() bool { return d.path == "" }

// projectTree is one open project's tree, as the sidebar shows it.
type projectTree struct {
	project workspace.Project
	root    *workspace.Node
}

// State is what the window shows, and what the app keeps of it.
type State struct {
	ws     *workspace.Workspace
	cfgDir string
	win    *mygo.Window

	cfg      config.Config
	activeID string
	// trees are the trees of the open projects, in the order they were added:
	// with more than one project the sidebar puts a node of each on top.
	trees []projectTree
	// open is the open state of each directory of the trees, by the key
	// "projectID:path": directories start open, and a click on one's arrow
	// writes here, so that a reloaded tree shows what the user left open.
	open   map[string]*bool
	sel    docSel
	status string

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
	// its identity; documents without one use the configuration's.
	renderMode map[docSel]config.RenderMode
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

	// projectsOpen is whether the project switcher's panel shows.
	projectsOpen bool

	sidebarW float32
	tocW     float32
	// docScroll is how far the document's body is scrolled, both ways.
	docScroll ui.ScrollState
	// contentWidth, when set, says how wide the window's content is on either
	// side of the sidebar. The app sets it, since only it sees the shell;
	// tests set their own.
	contentWidth func() float32

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
		renderMode: map[docSel]config.RenderMode{},
		sidebarW:   300,
		tocW:       240,
		inbox:      make(chan func(), 1024),
	}
	if !cfg.IsOpen(s.activeID) {
		s.activeID = ""
		if open := cfg.OpenProjects(); len(open) > 0 {
			s.activeID = open[0].ID
		}
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

// Projects returns every project the window knows, open or not: the project
// panel lists them all.
func (s *State) Projects() []workspace.Project { return s.cfg.Projects }

// project returns the project with id.
func (s *State) project(id string) (workspace.Project, bool) {
	for _, p := range s.cfg.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return workspace.Project{}, false
}

// activate shows the project with id as the active one.
func (s *State) activate(id string) {
	if !s.cfg.SetActive(id) {
		return
	}
	s.activeID = id
	s.ws.SetConfig(s.cfg)
	s.save()
}

// Selected returns the document the window shows, as its project names it.
func (s *State) Selected() string { return s.sel.path }

// SelectedProject returns the project of the document the window shows.
func (s *State) SelectedProject() (workspace.Project, bool) {
	if s.sel.empty() {
		return workspace.Project{}, false
	}
	return s.project(s.sel.project)
}

// IsOpen reports whether the window shows the project with id.
func (s *State) IsOpen(id string) bool { return s.cfg.IsOpen(id) }

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

// Trees returns the trees of the open projects, in the order they were added.
func (s *State) Trees() []projectTree { return s.trees }

// hasDocs reports whether any open project has a markdown document.
func (s *State) hasDocs() bool {
	for _, tr := range s.trees {
		if tr.root != nil && len(tr.root.Children) > 0 {
			return true
		}
	}
	return false
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
		s.sel = docSel{}
	}
	s.ws.SetConfig(s.cfg)
	s.save()
	s.startWatch(p.ID, p.Root)
	s.reloadTree()
}

// RemoveProject removes a project from the window.
func (s *State) RemoveProject(id string) {
	s.stopWatch(id)
	s.cfg.RemoveProject(id)
	if s.cfg.ActiveID == "" && len(s.cfg.Projects) > 0 {
		s.cfg.ActiveID = s.cfg.Projects[0].ID
	}
	s.activeID = s.cfg.ActiveID
	if s.sel.project == id {
		s.sel = docSel{}
	}
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

// reloadTree reads the trees of the open projects, dropping the projects whose
// directory is gone and the document that is no longer one of them.
func (s *State) reloadTree() {
	s.trees = s.trees[:0]
	for _, p := range s.cfg.OpenProjects() {
		tree, err := workspace.BuildTree(p.Root)
		if err == nil {
			s.trees = append(s.trees, projectTree{project: p, root: tree})
			continue
		}
		if s.gone[p.ID] {
			s.status = "项目目录不可用：" + p.Root
			continue
		}
		s.gone[p.ID] = true
		s.status = "项目目录不可用：" + p.Root
		s.cfg.RemoveProject(p.ID)
		s.activeID = s.cfg.ActiveID
		if s.sel.project == p.ID {
			s.sel = docSel{}
		}
		s.ws.SetConfig(s.cfg)
		s.save()
	}
	if s.sel.empty() {
		return
	}
	if _, err := s.ws.Stat(s.sel.project, s.sel.path); err != nil {
		s.sel = docSel{}
		s.status = "文件已被删除"
		s.onDocChanged()
	}
}

// selectDoc shows a document of the active project.
func (s *State) selectDoc(rel string) {
	p, ok := s.ActiveProject()
	if !ok {
		return
	}
	s.selectDocIn(p.ID, rel)
}

// selectDocIn shows a document of the project with projectID, which becomes
// the active project: the MCP tools read with it when no project is named.
func (s *State) selectDocIn(projectID, rel string) {
	sel := docSel{project: projectID, path: rel}
	if sel == s.sel {
		return
	}
	s.sel = sel
	s.status = ""
	if projectID != "" && s.cfg.SetActive(projectID) {
		s.activeID = projectID
		s.ws.SetConfig(s.cfg)
		s.save()
	}
	s.onDocChanged()
}

// onDocChanged reloads the parse and the outline of the document shown.
func (s *State) onDocChanged() { s.loadDoc() }

// SetContentWidth tells the window how wide its content is. The shell owns the
// window, so the app measures it and hands it over.
func (s *State) SetContentWidth(fn func() float32) { s.contentWidth = fn }

// windowWidth is how wide the window's content is: what the app measured, or a
// default for the tests that build the view without a window.
func (s *State) windowWidth() float32 {
	if s.contentWidth != nil {
		return max(320, s.contentWidth())
	}
	return 840
}

// bodyPaneWidth is how wide the document's body is: the window's content less
// the outline beside it.
func (s *State) bodyPaneWidth() float32 {
	return max(200, s.windowWidth()-s.tocW)
}

// shortPath is the path shown in the status bar: the project's name in front
// of the document's.
func (s *State) shortPath() string {
	if s.sel.empty() {
		return ""
	}
	if p, ok := s.SelectedProject(); ok {
		return p.Name + "/" + s.sel.path
	}
	return s.sel.path
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
