package workspace

import (
	"reflect"
	"slices"
	"testing"
)

// treeOf builds a tree from the documents written with write.
func treeOf(t *testing.T, root string) *Node {
	t.Helper()
	tree, err := BuildTree(root)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	return tree
}

func TestInsertDocCreatesTheDirectoriesAboveIt(t *testing.T) {
	tree := &Node{Name: "root", IsDir: true}
	if !InsertDoc(tree, "docs/a.md") {
		t.Fatal("InsertDoc = false, want the document added")
	}
	if len(tree.Children) != 1 || tree.Children[0].Name != "docs" || !tree.Children[0].IsDir {
		t.Fatalf("Children = %v, want the docs directory", names(tree))
	}
	docs := tree.Children[0]
	if docs.Path != "docs" {
		t.Errorf("docs.Path = %q, want docs", docs.Path)
	}
	if len(docs.Children) != 1 || docs.Children[0].Path != "docs/a.md" || docs.Children[0].IsDir {
		t.Fatalf("docs.Children = %v, want the document", names(docs))
	}
	if InsertDoc(tree, "docs/a.md") {
		t.Error("InsertDoc twice = true, want the tree unchanged")
	}
	if got := len(docs.Children); got != 1 {
		t.Errorf("docs holds %d entries, want 1: a document inserted twice is one", got)
	}
}

func TestInsertDocKeepsTheOrder(t *testing.T) {
	root := t.TempDir()
	write(t, root, "b.md")
	write(t, root, "docs/x.md")
	tree := treeOf(t, root)
	InsertDoc(tree, "a.md")
	InsertDoc(tree, "Aaa/y.md")
	want := []string{"Aaa", "docs", "a.md", "b.md"}
	if got := names(tree); !slices.Equal(got, want) {
		t.Fatalf("Children = %v, want %v", got, want)
	}
	// The tree the patches made is the tree a scan makes.
	write(t, root, "a.md")
	write(t, root, "Aaa/y.md")
	if again := treeOf(t, root); !reflect.DeepEqual(tree, again) {
		t.Errorf("patched tree = %+v, want the scanned one %+v", tree, again)
	}
}

func TestRemoveDocDropsTheEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/a.md")
	write(t, root, "docs/b.md")
	write(t, root, "top.md")
	tree := treeOf(t, root)
	if !RemoveDoc(tree, "docs/a.md") {
		t.Fatal("RemoveDoc = false, want the document removed")
	}
	if got := names(tree); !slices.Equal(got, []string{"docs", "top.md"}) {
		t.Fatalf("Children = %v, want the directory still there, with one document", got)
	}
	RemoveDoc(tree, "docs/b.md")
	if got := names(tree); !slices.Equal(got, []string{"top.md"}) {
		t.Fatalf("Children = %v, want the empty directory gone", got)
	}
	RemoveDoc(tree, "top.md")
	if got := len(tree.Children); got != 0 {
		t.Errorf("Children = %v, want none", names(tree))
	}
	if RemoveDoc(tree, "gone.md") {
		t.Error("RemoveDoc of a file the tree does not have = true, want false")
	}
}

func TestRenameDocMovesTheNode(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/a.md")
	write(t, root, "b.md")
	tree := treeOf(t, root)
	if !RenameDoc(tree, "docs/a.md", "renamed.md") {
		t.Fatal("RenameDoc = false, want the document moved")
	}
	want := []string{"b.md", "renamed.md"}
	if got := names(tree); !slices.Equal(got, want) {
		t.Fatalf("Children = %v, want %v", got, want)
	}
	// The directory it left is empty, so it goes with it.
	if !RenameDoc(tree, "b.md", "docs/b.md") {
		t.Fatal("RenameDoc into a new directory = false, want the document moved")
	}
	if got := names(tree); !slices.Equal(got, []string{"docs", "renamed.md"}) {
		t.Errorf("Children = %v, want the directory back with the moved document", got)
	}
}
