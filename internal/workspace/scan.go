package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrProjectGone is what reading a project that is no longer a directory
// returns: its root was deleted, moved or replaced.
var ErrProjectGone = errors.New("workspace: project directory is gone")

// ErrTreeTooLarge is what reading a project with more than MaxEntries entries
// returns: the window refuses it rather than read a tree it cannot show.
var ErrTreeTooLarge = errors.New("workspace: project has too many entries")

// MaxEntries is the most entries a project may hold: files and directories,
// the ones the scan leaves out not counted. The file watch reads the same
// number, so a project that scans watches too.
const MaxEntries = 1_000_000

// Doc is a markdown document of a project.
type Doc struct {
	ProjectID string    `json:"project_id"`
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	ModTime   time.Time `json:"mod_time"`
	Size      int64     `json:"size"`
}

// Node is one entry of a project's tree: a directory with markdown in it, or
// a markdown file.
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	IsDir    bool    `json:"is_dir"`
	Children []*Node `json:"children,omitempty"`
}

// ignoredDirs are the directories a project's scan leaves out: build output,
// dependencies and the app's own scratch directories.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".mygo":        true,
	".superpowers": true,
}

// Ignored reports whether the scan and the file watch leave out the entry at
// rel, relative to the project root with slashes. Hidden entries are left
// out, whatever they are, as are the files in a directory whose name one of
// these has.
func Ignored(rel string, isDir bool) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
		if ignoredDirs[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

// IsMarkdown reports whether name is a markdown file: .md or .markdown, in
// any case.
func IsMarkdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// ListDocs returns the markdown documents under root, sorted by path. A root
// with more than MaxEntries entries is refused, so that a project the window
// could never show is not read whole.
func ListDocs(root string) ([]Doc, error) {
	return listDocs(root, MaxEntries)
}

// listDocs is ListDocs with the ceiling named, which its test holds it to.
func listDocs(root string, max int) ([]Doc, error) {
	info, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrProjectGone
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrProjectGone
	}
	docs := []Doc{}
	seen := map[string]bool{}
	entries := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if Ignored(rel, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		entries++
		if entries > max {
			return ErrTreeTooLarge
		}
		if d.IsDir() {
			if key, ok := realPath(path); ok {
				if seen[key] {
					return fs.SkipDir
				}
				seen[key] = true
			}
			return nil
		}
		if !IsMarkdown(d.Name()) || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		docs = append(docs, Doc{
			Path:    rel,
			Title:   strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, nil
}

// BuildTree returns root as a tree of the directories that hold markdown and
// the markdown files, directories first, each sorted by name. A directory
// without markdown in it is left out.
func BuildTree(root string) (*Node, error) {
	docs, err := ListDocs(root)
	if err != nil {
		return nil, err
	}
	tree := &Node{Name: filepath.Base(root), IsDir: true}
	for _, doc := range docs {
		parts := strings.Split(doc.Path, "/")
		node := tree
		for i, part := range parts {
			last := i == len(parts)-1
			child := node.child(part)
			if child == nil {
				child = &Node{
					Name:  part,
					Path:  strings.Join(parts[:i+1], "/"),
					IsDir: !last,
				}
				node.Children = append(node.Children, child)
			}
			node = child
		}
		node.Path = doc.Path
	}
	sortTree(tree)
	return tree, nil
}

func (n *Node) child(name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// lessNode is the order the tree keeps: directories first, then documents,
// each by name ignoring case.
func lessNode(a, b *Node) bool {
	if a.IsDir != b.IsDir {
		return a.IsDir
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func sortTree(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool { return lessNode(n.Children[i], n.Children[j]) })
	for _, c := range n.Children {
		sortTree(c)
	}
}

// realPath returns the path a symlink or junction resolves to, so that a loop
// back into a directory already walked is caught instead of walked again.
func realPath(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	return resolved, true
}
