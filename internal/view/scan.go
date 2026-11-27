package view

import (
	"errors"
	"slices"

	"github.com/egoist/mygo"

	"mdpreview/internal/workspace"
)

// The trees of the open projects are read on goroutines: a directory of
// thousands of documents takes long enough that reading it in the frame that
// asked would freeze the window. This file is the bookkeeping of that: which
// project is being read, which result is the current one, and what the window
// does with each outcome.

// readTree reads a project's tree, as the app does.
func readTree(p workspace.Project) (*workspace.Node, error) { return workspace.Tree(p) }

// SetSyncScan makes the state read the trees in the frame that asks for them,
// rather than on a goroutine, and reads them now. The tests build frames
// without a window, and one that arrives later would make every test wait for
// it; the app leaves this off.
func (s *State) SetSyncScan() {
	s.syncScan = true
	// A scan the state started on a goroutine would land after the tree the
	// test writes: its result is made stale, so that nothing arrives later.
	for id := range s.scanning {
		s.scanGen[id]++
	}
	s.scanning = map[string]bool{}
	s.scanAgain = map[string]bool{}
	s.Refresh()
}

// loadTree starts reading a project's tree. A scan already running is not
// restarted: the ask is remembered and made again when the scan ends, since
// the tree it read may be older than the ask.
func (s *State) loadTree(p workspace.Project) {
	s.showPlaceholder(p)
	if s.scanning[p.ID] {
		s.scanAgain[p.ID] = true
		return
	}
	s.scanGen[p.ID]++
	gen := s.scanGen[p.ID]
	s.scanning[p.ID] = true
	scan := s.scan
	if s.syncScan {
		tree, err := scan(p)
		s.applyTree(p, gen, tree, err)
		return
	}
	go func() {
		tree, err := scan(p)
		s.Post(func() { s.applyTree(p, gen, tree, err) })
	}()
}

// showPlaceholder puts a project's row in the tree before its scan lands, so
// that opening a directory of thousands of entries shows what was opened at
// once, marked as being read.
func (s *State) showPlaceholder(p workspace.Project) {
	if _, ok := s.treeOf(p.ID); ok {
		return
	}
	s.trees = append(s.trees, projectTree{project: p})
	s.sortTrees()
}

// rescanTree reads one project's tree again, as its files changed in a way the
// window cannot make to the tree it has.
func (s *State) rescanTree(id string) {
	p, ok := s.project(id)
	if !ok || !s.cfg.IsOpen(id) {
		return
	}
	s.loadTree(p)
}

// applyTree takes what a scan read. It runs on the window's thread, and drops
// a result a newer scan already replaced, or one whose project the user closed
// or removed while it ran.
func (s *State) applyTree(p workspace.Project, gen int, tree *workspace.Node, err error) {
	if gen != s.scanGen[p.ID] {
		// A newer scan owns the flag, and this result is older than it.
		return
	}
	delete(s.scanning, p.ID)
	if !s.cfg.IsOpen(p.ID) {
		delete(s.scanAgain, p.ID)
		return
	}
	defer func() {
		if s.scanAgain[p.ID] && s.cfg.IsOpen(p.ID) {
			delete(s.scanAgain, p.ID)
			s.loadTree(p)
		}
	}()
	switch {
	case err == nil:
		s.putTree(p, tree)
		s.afterTreesChanged()
	case errors.Is(err, workspace.ErrTreeTooLarge):
		// The project is refused rather than half shown, and not watched
		// either: a watch of a million entries would read the whole tree
		// again. A reload is the way back.
		s.tooLarge[p.ID] = true
		s.dropTree(p.ID)
		s.stopWatch(p.ID)
		s.notifyWarn("目录过大无法打开，请缩小目录范围：" + p.Root)
		s.afterTreesChanged()
	case errors.Is(err, workspace.ErrProjectGone):
		if s.gone[p.ID] {
			return
		}
		s.gone[p.ID] = true
		s.notifyErr(goneText(p))
		s.removeProject(p.ID)
	default:
		s.notifyErr("无法读取项目：" + p.Name + "：" + err.Error())
	}
}

// putTree puts a project's tree in place of the one it had, keeping the trees
// in the order of the configuration rather than the order the scans finished.
func (s *State) putTree(p workspace.Project, tree *workspace.Node) {
	delete(s.tooLarge, p.ID)
	for i := range s.trees {
		if s.trees[i].project.ID == p.ID {
			s.trees[i] = projectTree{project: p, root: tree}
			return
		}
	}
	s.trees = append(s.trees, projectTree{project: p, root: tree})
	s.sortTrees()
}

// dropTree takes a project's tree out of the sidebar, as a project the window
// refuses or no longer has.
func (s *State) dropTree(id string) {
	kept := s.trees[:0]
	for _, tr := range s.trees {
		if tr.project.ID != id {
			kept = append(kept, tr)
		}
	}
	s.trees = kept
	s.dropTreeSelOf(id)
	for k := range s.treeSeen {
		if k.project == id {
			delete(s.treeSeen, k)
			delete(s.treeClosed, k)
		}
	}
}

// sortTrees keeps the trees in the order of the configuration's open projects,
// which is the order the sidebar shows them in.
func (s *State) sortTrees() {
	rank := make(map[string]int, len(s.trees))
	for i, p := range s.cfg.OpenProjects() {
		rank[p.ID] = i
	}
	slices.SortStableFunc(s.trees, func(a, b projectTree) int {
		return rank[a.project.ID] - rank[b.project.ID]
	})
}

// bumpTree says the trees changed under the window: what a search over them
// offers is read again. The view of a tree is a function of the tree, so a
// patched one shows without anything else being done here.
func (s *State) bumpTree() {
	// A patch can add a directory, which the window shows open, as a read tree
	// does; the outline then walks its open items again, told its children
	// changed.
	s.noteNewDirectories()
	s.treeOpen.Generation++
	if s.treeSearch.open {
		s.refreshCandidates()
	}
}

// afterTreesChanged tells the view and the tabs what the new trees mean: the
// outline walks the trees again, and a tab whose file is gone goes.
func (s *State) afterTreesChanged() {
	// The outline keeps the rows it walked until it is told its children may
	// differ: a tree read for a project that had none, as the scan of a big
	// directory does, changes them without changing what is open.
	s.bumpTree()
	s.pruneTreeSel()
	if s.sel.empty() {
		return
	}
	// A tab pointing at a file that is no longer there shows nothing: drop
	// those the window does not show first, then the one it does, so that its
	// neighbour in the bar takes over.
	for _, tab := range append([]docSel(nil), s.tabs...) {
		if tab == s.sel {
			continue
		}
		if _, err := s.ws.Stat(tab.project, tab.path); err != nil {
			s.dropTab(tab)
		}
	}
	if _, err := s.ws.Stat(s.sel.project, s.sel.path); err != nil {
		s.closeTab(s.sel)
		s.notifyWarn("文件已被删除")
	}
}

// scanningName is the name of the first open project whose tree is being read,
// which the tree shows while it has nothing to show yet.
func (s *State) scanningName() string {
	for _, p := range s.cfg.OpenProjects() {
		if s.scanning[p.ID] {
			return p.Name
		}
	}
	return ""
}

// goneText is what the window says of a project that is no longer there: a
// document of its own, or a directory.
func goneText(p workspace.Project) string {
	if p.File != "" {
		return "文件不可用：" + p.File
	}
	return "项目目录不可用：" + p.Root
}

// watchFailure is what the window says when a project could not be watched.
// The entry limit reads as what it is — a project too large to follow — rather
// than as an error of the watch.
func watchFailure(p workspace.Project, err error) string {
	if errors.Is(err, mygo.ErrWatchLimit) {
		return "目录过大，已停止监听：" + p.Name + "（刷新仍可用）"
	}
	return "无法监听这个项目：" + err.Error()
}
