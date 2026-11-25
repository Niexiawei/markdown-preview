package workspace

import (
	"slices"
	"strings"
)

// The window keeps its trees in memory and updates them from what the file
// watch reports, rather than reading a project again for every write: these
// are the three changes a watch can mean to a tree.

// InsertDoc adds the document at rel to the tree, making the directories above
// it. It reports whether the tree gained a document.
func InsertDoc(tree *Node, rel string) bool {
	parts := strings.Split(rel, "/")
	node := tree
	created := false
	for i, part := range parts {
		last := i == len(parts)-1
		var child *Node
		for _, c := range node.Children {
			if c.Name == part && c.IsDir != last {
				child = c
				break
			}
		}
		if child != nil {
			node = child
			continue
		}
		child = &Node{Name: part, Path: strings.Join(parts[:i+1], "/"), IsDir: !last}
		if last {
			child.Path = rel
		}
		if !insertChild(node, child) {
			// The tree holds something else by that name, as a document where
			// a directory goes: only a scan can say which of them is real.
			return false
		}
		node = child
		created = true
	}
	return created
}

// insertChild puts c among n's children where the tree's order has it, and
// reports whether it was not there already.
func insertChild(n *Node, c *Node) bool {
	i := 0
	for i < len(n.Children) && lessNode(n.Children[i], c) {
		i++
	}
	if i < len(n.Children) {
		other := n.Children[i]
		if other.Name == c.Name && other.IsDir == c.IsDir {
			return false
		}
	}
	n.Children = slices.Insert(n.Children, i, c)
	return true
}

// RemoveDoc takes the document at rel out of the tree, and the directories it
// leaves empty. It reports whether the tree lost a document.
func RemoveDoc(tree *Node, rel string) bool {
	parts := strings.Split(rel, "/")
	node := tree
	for i, part := range parts {
		last := i == len(parts)-1
		found := -1
		for j, c := range node.Children {
			if c.Name == part && c.IsDir != last {
				found = j
				break
			}
		}
		if found < 0 {
			return false
		}
		if last {
			node.Children = slices.Delete(node.Children, found, found+1)
			pruneEmpty(tree)
			return true
		}
		node = node.Children[found]
	}
	return false
}

// pruneEmpty drops the directories without anything in them, which a removed
// document leaves behind.
func pruneEmpty(n *Node) {
	kept := n.Children[:0]
	for _, c := range n.Children {
		if c.IsDir {
			pruneEmpty(c)
			if len(c.Children) == 0 {
				continue
			}
		}
		kept = append(kept, c)
	}
	n.Children = kept
}

// RenameDoc moves the document at from to to, keeping the tree's order. It
// reports whether the tree changed.
func RenameDoc(tree *Node, from, to string) bool {
	removed := RemoveDoc(tree, from)
	added := InsertDoc(tree, to)
	return removed || added
}
