package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Tree returns a project's tree: the directories and documents of a directory
// project, or the one document of a single-file project.
func Tree(p Project) (*Node, error) {
	if p.File == "" {
		return BuildTree(p.Root)
	}
	docs, err := fileDocs(p)
	if err != nil {
		return nil, err
	}
	return &Node{Name: docs[0].Path, Path: docs[0].Path}, nil
}

// fileDocs is the one document of a single-file project.
func fileDocs(p Project) ([]Doc, error) {
	info, err := os.Stat(p.File)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrProjectGone
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrProjectGone
	}
	name := filepath.Base(p.File)
	return []Doc{{
		Path:    name,
		Title:   strings.TrimSuffix(name, filepath.Ext(name)),
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}}, nil
}

// Locate returns the project the document at abs belongs to, and its path in
// it: the single-file project that is that document, else the deepest known
// project that holds it and whose tree would show it. It reports false for a
// path no project holds, which a new single-file project takes.
func Locate(projects []Project, abs string) (id, rel string, ok bool) {
	abs = filepath.Clean(abs)
	best := -1
	for i, p := range projects {
		if p.File != "" {
			if filepath.Clean(p.File) == abs {
				return p.ID, filepath.Base(p.File), true
			}
			continue
		}
		if !within(p.Root, abs) || !IsMarkdown(abs) {
			continue
		}
		in, err := filepath.Rel(p.Root, abs)
		if err != nil {
			continue
		}
		if Ignored(filepath.ToSlash(in), false) {
			continue
		}
		if best < 0 || len(p.Root) > len(projects[best].Root) {
			best = i
		}
	}
	if best < 0 {
		return "", "", false
	}
	p := projects[best]
	in, err := filepath.Rel(p.Root, abs)
	if err != nil {
		return "", "", false
	}
	return p.ID, filepath.ToSlash(in), true
}
