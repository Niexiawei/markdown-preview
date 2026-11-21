package view

import (
	"context"

	"github.com/egoist/mygo"

	"mdpreview/internal/watch"
)

// StartWatching watches every project the window has open, so that a file an
// agent or an editor writes shows up here. It must be called once the app
// runs, after the window is made.
func (s *State) StartWatching() {
	s.watchReady = true
	for _, p := range s.Projects() {
		s.startWatch(p.ID, p.Root)
	}
}

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

// applyChanges applies what a project's watch reported. It runs on the
// window's thread, from the inbox.
func (s *State) applyChanges(id string, set watch.ChangeSet) {
	if id != s.activeID {
		return
	}
	for _, change := range set.Docs {
		switch {
		case change.Old != "" && change.Old == s.selected:
			// The document the window shows was renamed: follow it.
			s.selected = change.Path
			s.onDocChanged()
		case change.Path == s.selected:
			if change.Op == mygo.FileRemove {
				s.selected = ""
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
