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
		s.startWatch(p.ID, p.Root)
	}
}

// watchedProjects are the projects the window watches: the open ones, which are
// the ones the tree shows.
func (s *State) watchedProjects() []workspace.Project { return s.cfg.OpenProjects() }

// startWatch begins watching one project, unless it is watched already.
func (s *State) startWatch(id, root string) {
	if !s.watchReady {
		return
	}
	if _, ok := s.watches[id]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	src, err := watch.Watch(ctx, root)
	if err != nil {
		cancel()
		s.status = "无法监听这个项目：" + err.Error()
		return
	}
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
	for _, change := range set.Docs {
		switch {
		case change.Old != "" && id == s.sel.project && change.Old == s.sel.path:
			// The document the window shows was renamed: follow it.
			s.sel.path = change.Path
			s.onDocChanged()
		case id == s.sel.project && change.Path == s.sel.path:
			if change.Op == mygo.FileRemove {
				s.sel = docSel{}
				s.status = "文件已被删除"
				s.reloadTree()
				continue
			}
			s.onDocChanged()
		}
	}
	if set.TreeDirty {
		s.reloadTree()
	}
}
