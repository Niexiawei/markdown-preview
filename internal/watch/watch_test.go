package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"mdpreview/internal/workspace"
)

func TestFoldWriteMarksDoc(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileWrite, Path: "notes/a.md"}})
	if len(set.Docs) != 1 || set.Docs[0].Path != "notes/a.md" || set.Docs[0].Op != mygo.FileWrite {
		t.Fatalf("Docs = %v, want one write of notes/a.md", set.Docs)
	}
	if set.TreeDirty {
		t.Error("TreeDirty = true, want false: a write changes no entry")
	}
}

func TestFoldCreateMDMarksTreeDirty(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileCreate, Path: "notes/new.md"}})
	if !set.TreeDirty {
		t.Error("TreeDirty = false, want true: the tree gains an entry")
	}
	if len(set.Docs) != 1 {
		t.Errorf("Docs = %v, want one", set.Docs)
	}
}

func TestFoldRemoveDirMarksTreeDirty(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileRemove, Path: "notes", IsDir: true}})
	if !set.TreeDirty {
		t.Error("TreeDirty = false, want true")
	}
	if len(set.Docs) != 0 {
		t.Errorf("Docs = %v, want none: a directory is no document", set.Docs)
	}
}

func TestFoldRenameCarriesOld(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileRename, Path: "notes/b.md", OldPath: "notes/a.md"}})
	if len(set.Docs) != 1 || set.Docs[0].Old != "notes/a.md" {
		t.Fatalf("Docs = %v, want Old notes/a.md", set.Docs)
	}
	if !set.TreeDirty {
		t.Error("TreeDirty = false, want true")
	}
}

func TestFoldIgnoresNonMD(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileWrite, Path: "readme.txt"}})
	if len(set.Docs) != 0 || set.TreeDirty {
		t.Errorf("set = %+v, want an empty one", set)
	}
	set = Fold([]mygo.FileEvent{{Op: mygo.FileWrite, Path: "node_modules/x.md"}})
	if len(set.Docs) != 0 {
		t.Errorf("set = %+v, want ignored paths skipped", set)
	}
}

func TestFoldRootRemovalMarksTreeDirty(t *testing.T) {
	set := Fold([]mygo.FileEvent{{Op: mygo.FileRemove, Path: ".", IsDir: true}})
	if !set.TreeDirty {
		t.Error("TreeDirty = false, want true")
	}
}

func TestWatchSeesExternalWrite(t *testing.T) {
	root := t.TempDir()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("needs a running app: %v", r)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src, err := Watch(ctx, root)
	if err != nil {
		t.Skipf("cannot watch here: %v", err)
	}
	defer src.Close()

	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan ChangeSet, 1)
	go func() {
		set, err := src.Next()
		if err == nil {
			done <- set
		}
	}()
	select {
	case set := <-done:
		if len(set.Docs) != 1 || set.Docs[0].Path != "a.md" {
			t.Errorf("set = %+v, want a.md", set)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no change reported")
	}
}

var _ = workspace.Ignored
