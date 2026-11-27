// Package view builds the app's window: the project switcher, the file tree,
// the outline and the preview. The state belongs to the window's thread:
// anything another goroutine learns (a file changed, a project is gone)
// arrives through Post, which the next frame applies before it builds.
package view

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
	"mdpreview/internal/notices"
	"mdpreview/internal/session"
	"mdpreview/internal/watch"
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
	// open is the open state of each heading of the document's outline, by the
	// key its title makes: the outline's directories start open, and a click on
	// one's arrow writes here, so that a reloaded outline shows what the user
	// left open.
	open map[string]*bool
	// treeOpen is the sidebar's tree: which of its rows are open, and how it
	// lays them out. Its keys are the entries' paths, so that a tree read again
	// keeps what the user folded; treeSeen are the directories the window has
	// shown, so that a new one opens by itself and a folded one stays folded.
	// treeClosed are the rows the user folded — the ones a restart brings back;
	// the search opens rows of its own and never touches it.
	treeOpen   ui.OutlineState[treeKey]
	treeSeen   map[treeKey]bool
	treeClosed map[treeKey]bool
	// tabs are the documents the window has open, in the order they were
	// opened: the tab bar shows them, and sel is always one of them.
	tabs []docSel
	// scrolls keeps each tab's body scroll position, so that switching back
	// shows the document where the user left it.
	scrolls map[docSel]*ui.ScrollState
	sel     docSel
	// notes are the messages the window reported, the newest first: the toasts
	// pop up from them and the bell's history lists them. They survive a
	// restart in the user data directory.
	notes *notices.Store
	// noticeShown is the ID of the last message handed to a toast, so that a
	// message toasts once — a click builds the view twice — and so that the
	// messages of the last run do not toast again on this one.
	noticeShown int
	// noticeOpen is whether the history panel shows.
	noticeOpen bool
	// tabsOpen is whether the panel listing every tab shows.
	tabsOpen bool
	// tabScroll is how far the strip of tabs scrolls sideways; tabShown is the
	// tab it was last scrolled to show, and tabView the room it had as it did.
	tabScroll ui.ScrollState
	tabShown  docSel
	tabView   float32
	// tabTotal is how wide all the tabs are, as the strip was last scrolled to
	// show one: a tab opened or closed changes it, and the place is asked for
	// again.
	tabTotal float32
	// tabPending says the strip has yet to take the place asked for, because it
	// had not laid the tabs out: the ask is repeated until it does.
	tabPending bool
	// tabAsk is the place the strip was last asked to slide to.
	tabAsk float32
	// zoom is how much larger or smaller the body's text shows than the
	// theme's: one while it was never changed.
	zoom float32
	// find is the search over the document shown, which the floating panel
	// shows.
	find findState
	// treeSearch is the search over the tree's file names, which the sidebar
	// shows in place of the whole tree. treeFocused says the last press or key
	// was the sidebar's: Cmd+F searches files when it was, and the document when
	// it was not.
	treeSearch  treeSearchState
	treeFocused bool
	// treeSel is the rows of the tree the user picked with the pointer, in the
	// order they were picked: Cmd+C copies their paths, and a context menu
	// opened on one of them acts on all of them. Apart from the document shown
	// and never saved.
	treeSel []treeKey
	// panelDown sinks the project panel into the sidebar's body as its first
	// row, leaving the shell's header slot to the collapse toggle, which then
	// trails at the header's end (main.go): the macOS arrangement, so that
	// the traffic lights keep the header's leading corner. Elsewhere the
	// panel stays in the header. MDPREVIEW_PANEL_DOWN=1 previews it on any
	// machine; the tests set the field directly.
	panelDown bool

	// The document shown: its source, its parse, the outline built from it,
	// and what the native markdown view shows it with.
	src      string
	doc      *parser.Document
	headings []workspace.Heading
	docErr   error
	md       markdown.Options
	openURL  func(string)
	// anchor is the heading the outline marks as current: the one last
	// jumped to. The native body keeps its place by jump, not by this.
	anchor string
	// jump is the block of the document shown that the native body brings to
	// the top of its pane, 0 for none. The outline, a link to a heading and a
	// search hit ask for one; the markdown view builds that block with a
	// marker that scrolls the body there (native.go's jumpMarker).
	jump int
	// jumpSeen is when the frame that built the marker built it. The jump is
	// kept for the rest of that frame — a click builds the view twice, and
	// only the last build's ask to scroll survives it — and dropped on the
	// next one.
	jumpSeen time.Time

	// renderMode is the rendering backend the user chose for a document, by
	// its identity; documents without one use the configuration's.
	renderMode map[docSel]config.RenderMode
	// The web view, made the first time a document asks for it; rev makes
	// each change of the document a new address for it to load.
	wv    *mygo.WebView
	wvURL string
	webOK bool
	rev   int

	// gone remembers the projects whose directory was removed, so that a
	// project found gone is dropped once and not every frame.
	gone map[string]bool

	// scan reads a project's tree: on a goroutine unless syncScan, which the
	// tests set so that a tree they write is there in the frame that asks.
	// scanning says which projects are being read now, scanAgain which were
	// asked for again while they were, and scanGen the scan whose result
	// counts. tooLarge are the projects the window refused to read, until a
	// reload asks again.
	scan     func(workspace.Project) (*workspace.Node, error)
	syncScan bool
	// watchProject starts one project's watch. It is a field so that a test
	// can hold a watch, as it holds a scan; the app's own starts mygo's.
	watchProject func(context.Context, workspace.Project) (watch.Source, error)
	scanning     map[string]bool
	scanAgain    map[string]bool
	scanGen      map[string]int
	tooLarge     map[string]bool

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

	// projectsOpen is whether the project panel shows.
	projectsOpen bool
	// sys is the desktop the window acts through; nil without one, as in the
	// tests that do not care.
	sys System

	sidebarW float32
	tocW     float32
	// tocScroll is how far the document's outline scrolls, both ways.
	tocScroll ui.ScrollState
	// contentWidth, when set, says how wide the window's content is on either
	// side of the sidebar; contentHeight how tall it is. The app sets them,
	// since only it sees the shell; tests set their own.
	contentWidth  func() float32
	contentHeight func() float32

	inbox chan func()
}

// New returns the state of a window showing the projects in cfg, whose
// configuration lives in dir, as do the messages it reported.
func New(ws *workspace.Workspace, cfg config.Config, dir string) *State {
	// A file this app cannot read leaves an empty history, which the window
	// shows as no messages at all: it must open all the same.
	book, _ := notices.Load(dir)
	s := &State{
		ws:           ws,
		cfgDir:       dir,
		cfg:          cfg,
		activeID:     cfg.ActiveID,
		open:         map[string]*bool{},
		treeOpen:     ui.OutlineState[treeKey]{Generation: 1},
		treeSeen:     map[treeKey]bool{},
		treeClosed:   map[treeKey]bool{},
		scrolls:      map[docSel]*ui.ScrollState{},
		gone:         map[string]bool{},
		watches:      map[string]context.CancelFunc{},
		scan:         readTree,
		watchProject: startProjectWatch,
		scanning:     map[string]bool{},
		scanAgain:    map[string]bool{},
		scanGen:      map[string]int{},
		tooLarge:     map[string]bool{},
		renderMode:   map[docSel]config.RenderMode{},
		notes:        book,
		// The messages the last run left are history, not news: they show in
		// the bell's list without popping up again.
		noticeShown: book.Last(),
		sidebarW:    300,
		tocW:        240,
		inbox:       make(chan func(), 1024),
	}
	// macOS puts the project panel in the sidebar's body and the collapse
	// toggle at the header's trailing end; elsewhere the panel stays in the
	// header. The env override previews either from any machine.
	s.panelDown = runtime.GOOS == "darwin" || os.Getenv("MDPREVIEW_PANEL_DOWN") == "1"
	if !cfg.IsOpen(s.activeID) {
		s.activeID = ""
		if open := cfg.OpenProjects(); len(open) > 0 {
			s.activeID = open[0].ID
		}
	}
	// The last session comes back before the trees are read: the directories
	// it marks folded have to be seen already, or the scan would open them all.
	sess, _ := session.Load(dir)
	s.restoreSession(sess)
	s.reloadTree()
	if !s.sel.empty() {
		s.onDocChanged()
	}
	return s
}

// restoreSession puts the snapshot of the last run back in place: the tabs it
// names, the active one among them, and the directories the user had folded.
// It is silent — no "opened" message — and drops what no longer explains
// itself: tabs of projects the window does not show, and folded rows of them.
// The active one is only taken when it is one of the tabs; the pruning of
// files that went away is afterTreesChanged's, which reloadTree calls next.
func (s *State) restoreSession(sess session.Session) {
	for _, ref := range sess.TreeClosed {
		if ref.Project == "" || !s.cfg.IsOpen(ref.Project) {
			continue
		}
		k := treeKey{project: ref.Project, path: ref.Path}
		s.treeSeen[k] = true
		s.treeClosed[k] = true
	}
	for _, ref := range sess.Tabs {
		if ref.Project == "" || ref.Path == "" || !s.cfg.IsOpen(ref.Project) {
			continue
		}
		if sel := (docSel{project: ref.Project, path: ref.Path}); !s.hasTab(sel) {
			s.tabs = append(s.tabs, sel)
		}
	}
	if active := (docSel{project: sess.Active.Project, path: sess.Active.Path}); s.hasTab(active) {
		s.sel = active
		if s.sel.project != s.cfg.ActiveID && s.cfg.SetActive(s.sel.project) {
			s.activeID = s.sel.project
		}
	}
}

// SetSystem gives the window the desktop it acts through.
func (s *State) SetSystem(sys System) { s.sys = sys }

// revealProject shows a project's directory in the file manager.
func (s *State) revealProject(p workspace.Project) {
	if s.sys == nil {
		return
	}
	if err := s.sys.Reveal(p.Root); err != nil {
		s.notifyErr("无法打开文件管理器：" + err.Error())
		return
	}
	s.notify("已在文件管理器中打开：" + p.Root)
}

// copyProjectPath puts a project's directory in the clipboard.
func (s *State) copyProjectPath(p workspace.Project) {
	if s.sys == nil {
		return
	}
	s.sys.CopyText(p.Root)
	s.notifyDone("已复制项目路径：" + p.Root)
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

// ToggleProject opens or closes a project: the open ones are the projects the
// tree shows, the window watches, and the web view serves.
func (s *State) ToggleProject(id string) {
	p, ok := s.project(id)
	if !ok {
		return
	}
	open := !s.cfg.IsOpen(id)
	s.cfg.SetOpen(id, open)
	if open {
		s.startWatch(p)
	} else {
		s.stopWatch(p.ID)
		s.dropTabsOf(id)
	}
	s.activeID = s.cfg.ActiveID
	s.ws.SetConfig(s.cfg)
	s.save()
	s.reloadTree()
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

// Status returns the text of the newest message the window reported, "" when
// it reported none. The history keeps them all; this is the one a reader would
// care about most.
func (s *State) Status() string {
	if len(s.Messages()) == 0 {
		return ""
	}
	return s.Messages()[0].Text
}

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

// treeOf returns the tree of the project with id, if the window has read it.
func (s *State) treeOf(id string) (projectTree, bool) {
	for _, tr := range s.trees {
		if tr.project.ID == id {
			return tr, true
		}
	}
	return projectTree{}, false
}

// hasDocs reports whether any open project has a markdown document.
func (s *State) hasDocs() bool {
	for _, tr := range s.trees {
		switch {
		case tr.root == nil:
		case tr.project.File != "":
			// A project of one document is that document.
			return true
		case len(tr.root.Children) > 0:
			return true
		}
	}
	return false
}

// OpenProject asks for directories and opens each of them as a project.
func (s *State) OpenProject() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:     "打开项目目录",
		Directory: true,
		Multiple:  true,
		Parent:    s.win,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	s.AddProjects(paths)
}

// OpenDocumentFile asks for markdown files and opens each of them, as a
// document of a project the window knows or of its own.
func (s *State) OpenDocumentFile() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:    "打开 Markdown 文件",
		Multiple: true,
		Filters:  []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md", "markdown"}}},
		Parent:   s.win,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	for _, path := range paths {
		s.OpenDocument(path)
	}
}

// AddProject opens the directory at root as a project.
func (s *State) AddProject(root string) { s.AddProjects([]string{root}) }

// AddProjects opens the directories at roots as projects, the last one active,
// and reports how many were new. The document the window shows stays shown:
// another project beside it changes nothing about it.
func (s *State) AddProjects(roots []string) int {
	added := 0
	for _, root := range roots {
		p, isNew := s.cfg.AddProject(root, filepath.Base(filepath.Clean(root)))
		if isNew {
			s.gone = map[string]bool{}
			added++
		}
		s.cfg.SetOpen(p.ID, true)
		s.cfg.ActiveID = p.ID
		s.startWatch(p)
	}
	if len(roots) == 0 {
		return 0
	}
	s.afterProjectsChanged()
	return added
}

// afterProjectsChanged applies a change to the projects to everything that
// reads them: the workspace the app asks, the file it keeps, and the trees.
func (s *State) afterProjectsChanged() {
	s.activeID = s.cfg.ActiveID
	s.ws.SetConfig(s.cfg)
	s.save()
	s.reloadTree()
}

// OpenDocument shows the document at path, as the command line, a second
// launch or the system opening a file with the app asks: a document of a
// project the window knows, or a file of its own, which makes a project of one
// document. Opening the document shown again does nothing.
func (s *State) OpenDocument(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		s.notifyErr("无法打开：" + path)
		return
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() || !workspace.IsMarkdown(abs) {
		s.notifyErr("无法打开：" + abs)
		return
	}
	sel := docSel{project: "", path: filepath.Base(abs)}
	if id, rel, ok := workspace.Locate(s.cfg.Projects, abs); ok {
		sel = docSel{project: id, path: rel}
		if sel == s.sel && s.cfg.IsOpen(id) {
			return
		}
		if !s.cfg.IsOpen(id) {
			p, _ := s.project(id)
			s.cfg.SetOpen(id, true)
			s.startWatch(p)
			s.afterProjectsChanged()
		}
	} else {
		p, _ := s.cfg.AddFileProject(abs)
		sel = docSel{project: p.ID, path: filepath.Base(abs)}
		if sel == s.sel {
			return
		}
		s.startWatch(p)
		s.afterProjectsChanged()
	}
	s.selectDocIn(sel.project, sel.path)
	s.notifyDone("已打开 " + filepath.Base(abs))
	s.invalidate()
}

// invalidate asks the window for a frame, for the work that arrives outside
// one, as a second launch's command line does.
func (s *State) invalidate() {
	if s.win != nil {
		s.win.Invalidate()
	}
}

// RemoveProject removes a project from the window, asking first: the window
// cannot undo it. Without a desktop to ask through, as in the tests that do
// not supply one, nothing is removed.
func (s *State) RemoveProject(id string) {
	p, ok := s.project(id)
	if !ok || s.sys == nil {
		return
	}
	if !s.sys.Confirm(ConfirmOptions{
		Title:        "移除项目？",
		Message:      "从列表中移除 “" + p.Name + "”？项目里的文件不会被删除。",
		ConfirmLabel: "移除",
	}) {
		s.notify("已取消移除")
		return
	}
	s.removeProject(id)
}

// removeProject removes a project from the window, without asking.
func (s *State) removeProject(id string) {
	s.stopWatch(id)
	s.cfg.RemoveProject(id)
	s.activeID = s.cfg.ActiveID
	s.dropTabsOf(id)
	delete(s.gone, id)
	s.ws.SetConfig(s.cfg)
	s.save()
	s.reloadTree()
}

// Refresh reloads the file tree and the document shown. It is the user's ask,
// so it reads again what the window refused as too large, and watches again a
// project whose watch stopped at the entry limit.
func (s *State) Refresh() {
	s.tooLarge = map[string]bool{}
	for _, p := range s.cfg.OpenProjects() {
		s.startWatch(p)
		s.loadTree(p)
	}
	s.afterTreesChanged()
	s.onDocChanged()
}

func (s *State) save() {
	_ = s.cfg.Save(s.cfgDir)
}

// saveSession writes the snapshot of what the window shows now: the tabs in
// their order, the active one, and the directories the user folded. It runs
// on the window's thread after the user acted — once per action, never inside
// a loop — and when the scans prune a tab of a file that went away.
func (s *State) saveSession() {
	closed := make([]session.Ref, 0, len(s.treeClosed))
	for k := range s.treeClosed {
		if s.cfg.IsOpen(k.project) {
			closed = append(closed, session.Ref{Project: k.project, Path: k.path})
		}
	}
	slices.SortFunc(closed, func(a, b session.Ref) int {
		if a.Project != b.Project {
			return strings.Compare(a.Project, b.Project)
		}
		return strings.Compare(a.Path, b.Path)
	})
	tabs := make([]session.Ref, len(s.tabs))
	for i, tab := range s.tabs {
		tabs[i] = session.Ref{Project: tab.project, Path: tab.path}
	}
	_ = session.Save(s.cfgDir, session.Session{
		Tabs:       tabs,
		Active:     session.Ref{Project: s.sel.project, Path: s.sel.path},
		TreeClosed: closed,
	})
}

// reloadTree lines the trees up with the open projects: the trees of the ones
// the window no longer shows go, and the ones it shows but has not read yet are
// read. What a scan makes of a project is applyTree's business.
func (s *State) reloadTree() {
	open := s.cfg.OpenProjects()
	rank := make(map[string]bool, len(open))
	for _, p := range open {
		rank[p.ID] = true
	}
	kept := s.trees[:0]
	for _, tr := range s.trees {
		if rank[tr.project.ID] {
			kept = append(kept, tr)
		}
	}
	s.trees = kept
	for _, p := range open {
		if s.tooLarge[p.ID] {
			// The window refused this one: the user's reload reads it again,
			// not every frame that lines the trees up.
			continue
		}
		if _, ok := s.treeOf(p.ID); ok {
			continue
		}
		s.loadTree(p)
	}
	s.afterTreesChanged()
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
// The document gets a tab, as every document the window shows does.
func (s *State) selectDocIn(projectID, rel string) {
	sel := docSel{project: projectID, path: rel}
	if sel == s.sel {
		return
	}
	if projectID != "" && s.cfg.SetActive(projectID) {
		s.activeID = projectID
		s.ws.SetConfig(s.cfg)
		s.save()
	}
	s.openTab(sel)
}

// docScroll is how far the document shown is scrolled, both ways: each tab
// keeps its own, so that switching back does not jump to the top.
func (s *State) docScroll() *ui.ScrollState {
	if st, ok := s.scrolls[s.sel]; ok {
		return st
	}
	if s.scrolls == nil {
		s.scrolls = map[docSel]*ui.ScrollState{}
	}
	st := &ui.ScrollState{}
	s.scrolls[s.sel] = st
	return st
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

// SetContentHeight tells the window how tall its content is, as SetContentWidth
// does how wide: the outline of the document and the search panel fit in it.
func (s *State) SetContentHeight(fn func() float32) { s.contentHeight = fn }

// windowHeight is how tall the window's content is: what the app measured, or a
// default for the tests that build the view without a window.
func (s *State) windowHeight() float32 {
	if s.contentHeight != nil {
		return max(320, s.contentHeight())
	}
	return 700
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
		if p.File != "" {
			// A document of its own is named by the path it has, not by a
			// project's name in front of the same name.
			return p.File
		}
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
