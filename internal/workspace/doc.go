package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// MaxDocBytes is the largest document the app reads. Anything past it is
// refused rather than held in memory whole.
const MaxDocBytes = 8 << 20

// titleScanBytes is how much of a document ListDocs reads to find its title.
const titleScanBytes = 64 << 10

var (
	// ErrTooLarge is what reading a document over MaxDocBytes returns.
	ErrTooLarge = errors.New("workspace: document is too large")
	// ErrOutsideProject is what reading a path outside the project returns.
	ErrOutsideProject = errors.New("workspace: path is outside the project")
)

// parseCount and titleScanCount count the work the caches spare, for their
// tests.
var (
	parseCount     atomic.Int64
	titleScanCount atomic.Int64
)

// docEntry caches what parsing a document made, until its file changes.
type docEntry struct {
	modTime  time.Time
	size     int64
	src      string
	doc      *parser.Document
	headings []Heading
}

// ListDocs returns the markdown documents of a project, each with the title
// it shows: its first level-one heading, or its file name without the suffix.
func (w *Workspace) ListDocs(projectID string) ([]Doc, error) {
	p, ok := w.Project(projectID)
	if !ok {
		return nil, ErrProjectGone
	}
	var docs []Doc
	if p.File != "" {
		var err error
		docs, err = fileDocs(p)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		docs, err = ListDocs(p.Root)
		if err != nil {
			return nil, err
		}
	}
	for i := range docs {
		docs[i].ProjectID = projectID
		if title, ok := w.titleOf(projectID, p.Root, &docs[i]); ok {
			docs[i].Title = title
		}
	}
	return docs, nil
}

// ReadMarkdown returns the source of a document, with invalid UTF-8 replaced
// so that it can be shown and parsed.
func (w *Workspace) ReadMarkdown(projectID, path string) (string, error) {
	abs, err := w.resolve(projectID, path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.Size() > MaxDocBytes {
		return "", ErrTooLarge
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return toValidUTF8(string(data)), nil
}

// Parse returns a document's parse and its headings, reusing what it made
// last time unless the file changed.
func (w *Workspace) Parse(projectID, path string) (*parser.Document, []Heading, error) {
	abs, err := w.resolve(projectID, path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	key := docKey{project: projectID, path: path}
	w.cacheMu.Lock()
	entry, ok := w.cache[key]
	w.cacheMu.Unlock()
	if ok && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		return entry.doc, entry.headings, nil
	}

	src, err := w.ReadMarkdown(projectID, path)
	if err != nil {
		return nil, nil, err
	}
	parseCount.Add(1)
	doc := parser.Parse(src)
	headings := outlineOf(doc)
	w.cacheMu.Lock()
	if w.cache == nil {
		w.cache = map[docKey]*docEntry{}
	}
	w.cache[key] = &docEntry{
		modTime:  info.ModTime(),
		size:     info.Size(),
		src:      src,
		doc:      doc,
		headings: headings,
	}
	w.cacheMu.Unlock()
	return doc, headings, nil
}

// Forget drops what parsing cached for a document, as when its file is gone.
func (w *Workspace) Forget(projectID, path string) {
	w.cacheMu.Lock()
	defer w.cacheMu.Unlock()
	delete(w.cache, docKey{project: projectID, path: path})
}

// Root returns a project's directory.
func (w *Workspace) Root(projectID string) (string, bool) {
	p, ok := w.Project(projectID)
	if !ok {
		return "", false
	}
	return p.Root, true
}

func (w *Workspace) root(projectID string) (string, error) {
	p, ok := w.Project(projectID)
	if !ok {
		return "", ErrProjectGone
	}
	return p.Root, nil
}

// resolve joins path onto the project's root, and refuses a path that lands
// outside it, as a link of ../../etc does.
func (w *Workspace) resolve(projectID, path string) (string, error) {
	root, err := w.root(projectID)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(absRoot, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	if !within(absRoot, abs) {
		return "", ErrOutsideProject
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if resolvedRoot, err := filepath.EvalSymlinks(absRoot); err == nil && !within(resolvedRoot, resolved) {
			return "", ErrOutsideProject
		}
	}
	return abs, nil
}

// within reports whether path is root or below it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// titleOf returns the title a document shows: its first level-one heading,
// which reading the start of the file finds, kept until the file changes so
// that a tree reloaded on every file event does not read every document
// again.
func (w *Workspace) titleOf(projectID, root string, doc *Doc) (string, bool) {
	key := docKey{project: projectID, path: doc.Path}
	w.cacheMu.Lock()
	entry, ok := w.titles[key]
	w.cacheMu.Unlock()
	if ok && entry.modTime.Equal(doc.ModTime) && entry.size == doc.Size {
		return entry.title, entry.title != ""
	}
	titleScanCount.Add(1)
	title, found := firstHeading(filepath.Join(root, filepath.FromSlash(doc.Path)))
	w.cacheMu.Lock()
	if w.titles == nil {
		w.titles = map[docKey]*titleEntry{}
	}
	w.titles[key] = &titleEntry{modTime: doc.ModTime, size: doc.Size, title: title}
	w.cacheMu.Unlock()
	return title, found
}

// titleEntry is a document's title, until its file changes.
type titleEntry struct {
	modTime time.Time
	size    int64
	title   string
}

// firstHeading reads the start of a document for its first level-one
// heading, which a document keeps beyond titleScanBytes only by having an
// unusual amount of text before its title.
func firstHeading(abs string) (string, bool) {
	f, err := os.Open(abs)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := make([]byte, titleScanBytes)
	n, _ := f.Read(buf)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
		if title != "" {
			return title, true
		}
	}
	return "", false
}

// toValidUTF8 replaces invalid bytes with U+FFFD instead of failing, so that a
// document an editor half wrote still shows.
func toValidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}
