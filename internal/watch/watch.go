// Package watch turns a project's file events into the changes the window
// applies: which documents changed, and whether the tree did.
package watch

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/egoist/mygo"

	"mdpreview/internal/workspace"
)

// Change is one document the file system reported.
type Change struct {
	Op   mygo.FileOp
	Path string
	Old  string
}

// ChangeSet is what one batch of file events means to the window.
type ChangeSet struct {
	// Docs are the markdown files that appeared, changed, went away or moved.
	Docs []Change
	// TreeDirty says the tree gained, lost or renamed an entry.
	TreeDirty bool
}

// Fold turns a batch of file events into what the window should do about
// them.
func Fold(events []mygo.FileEvent) ChangeSet {
	var set ChangeSet
	for _, e := range events {
		rel := path.Clean(strings.TrimPrefix(e.Path, "./"))
		if e.Op == mygo.FileHint {
			// The watch only says something under rel changed.
			if isDirPath(rel) || e.IsDir || rel == "." {
				set.TreeDirty = true
			}
			continue
		}
		if e.IsDir {
			if e.Op == mygo.FileCreate || e.Op == mygo.FileRemove || e.Op == mygo.FileRename {
				set.TreeDirty = true
			}
			continue
		}
		if rel == "." {
			set.TreeDirty = true
			continue
		}
		if workspace.Ignored(rel, false) || !workspace.IsMarkdown(rel) {
			// A file the tree does not show changes nothing in it.
			continue
		}
		set.Docs = append(set.Docs, Change{Op: e.Op, Path: rel, Old: cleanOld(e.OldPath)})
		if e.Op == mygo.FileCreate || e.Op == mygo.FileRemove || e.Op == mygo.FileRename {
			set.TreeDirty = true
		}
	}
	return set
}

// cleanOld turns a rename's source into the path the tree uses.
func cleanOld(old string) string {
	if old == "" {
		return ""
	}
	return path.Clean(strings.TrimPrefix(old, "./"))
}

// isDirPath reports whether path names a directory, judging by its last
// element: the watch reports a directory's own events, and a hint names one.
func isDirPath(rel string) bool {
	return path.Ext(rel) == "" || !strings.Contains(path.Base(rel), ".")
}

// Source is a project's file watch, as the window reads it.
type Source interface {
	Next() (ChangeSet, error)
	Close() error
}

// watcher is a Source over a mygo file watch.
type watcher struct {
	fw  *mygo.FileWatcher
	err error
}

// Watch watches the project at root, and reports what changes in it as
// ChangeSets. It must be called once the app runs.
func Watch(ctx context.Context, root string) (Source, error) {
	fw, err := mygo.WatchFiles(ctx, root, mygo.FileWatchOptions{
		Recursive: true,
		Ignore:    workspace.Ignored,
	})
	if err != nil {
		return nil, err
	}
	return &watcher{fw: fw}, nil
}

// Next waits for the next batch of changes.
func (w *watcher) Next() (ChangeSet, error) {
	for {
		events, err := w.fw.Next(context.Background())
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ChangeSet{}, err
			}
			return ChangeSet{}, err
		}
		set := Fold(events)
		if len(set.Docs) == 0 && !set.TreeDirty {
			continue
		}
		return set, nil
	}
}

// Close stops watching.
func (w *watcher) Close() error { return w.fw.Close() }
