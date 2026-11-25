package view

import (
	"context"

	"github.com/egoist/mygo"

	"mdpreview/internal/watch"
	"mdpreview/internal/workspace"
)

// StartWatching watches every project the window has open, so that a file an
// agent or an editor writes shows up here. It must be called once the app
// runs, after the window is made.
func (s *State) StartWatching() {
	s.watchReady = true
	for _, p := range s.watchedProjects() {
		s.startWatch(p)
	}
}

// watchedProjects are the projects the window watches: the open ones, which are
// the ones the tree shows.
func (s *State) watchedProjects() []workspace.Project { return s.cfg.OpenProjects() }

// startWatch begins watching one project, unless it is watched already. A
// project of one document watches that document alone: its directory may be a
// home directory, or one the watch would never keep up with.
func (s *State) startWatch(p workspace.Project) {
	if !s.watchReady {
		return
	}
	if _, ok := s.watches[p.ID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	var (
		src watch.Source
		err error
	)
	if p.File != "" {
		src, err = watch.WatchFile(ctx, p.File)
	} else {
		src, err = watch.Watch(ctx, p.Root)
	}
	if err != nil {
		cancel()
		s.notifyErr(watchFailure(p, err))
		return
	}
	id := p.ID
	s.watches[id] = cancel
	go func() {
		defer src.Close()
		for {
			set, err := src.Next()
			if err != nil {
				return
			}
			id, set := id, set
			s.Post(func() { s.applyChanges(id, set) })
		}
	}()
}

// stopWatch ends one project's watch.
func (s *State) stopWatch(id string) {
	if cancel, ok := s.watches[id]; ok {
		cancel()
		delete(s.watches, id)
	}
}

// applyChanges applies what a project's watch reported, whichever open project
// it was: the tree shows them all. It runs on the window's thread, from the
// inbox.
func (s *State) applyChanges(id string, set watch.ChangeSet) {
	// The tree the window has takes the changes the watch reports, rather than
	// being read again: the events already say what appeared, went or moved.
	patched := false
	for _, change := range set.Docs {
		tr, ok := s.treeOf(id)
		if !ok || tr.root == nil {
			break
		}
		switch {
		case change.Old != "":
			patched = workspace.RenameDoc(tr.root, change.Old, change.Path) || patched
		case change.Op == mygo.FileRemove:
			patched = workspace.RemoveDoc(tr.root, change.Path) || patched
		default:
			patched = workspace.InsertDoc(tr.root, change.Path) || patched
		}
	}
	for _, change := range set.Docs {
		// A tab pointing at the old name would show nothing: follow the file
		// wherever it went, and forget it if it went away.
		if change.Old != "" {
			s.renameTab(docSel{project: id, path: change.Old}, docSel{project: id, path: change.Path})
		}
		switch {
		case change.Old != "" && id == s.sel.project && change.Old == s.sel.path:
			// The document the window shows was renamed: follow it.
			s.sel.path = change.Path
			s.onDocChanged()
		case id == s.sel.project && change.Path == s.sel.path:
			if change.Op == mygo.FileRemove {
				// The document shown went away: its neighbours take over.
				s.closeTab(s.sel)
				s.reloadTree()
				s.notifyWarn("文件已被删除")
				continue
			}
			s.onDocChanged()
		case change.Op == mygo.FileRemove:
			s.dropTab(docSel{project: id, path: change.Path})
		}
	}
	switch {
	case patched:
		s.bumpTree()
	case set.TreeDirty && len(set.Docs) == 0:
		// The watch says the tree changed without saying how, as a directory
		// that appeared or went: only a scan can tell.
		s.rescanTree(id)
	}
}
