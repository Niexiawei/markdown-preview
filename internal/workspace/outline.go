package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Heading is one entry of a document's outline.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Slug  string `json:"slug"`
}

// Hit is one line of a document that holds a searched phrase.
type Hit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// slugSpace and slugKept describe how a heading becomes the anchor the
// markdown view scrolls to, which its own Slug builds the same way: a space
// is a hyphen, hyphens, underscores and letters and digits stay, everything
// else goes, and nothing is collapsed or trimmed.
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// outlineOf returns the headings of a parse, in order.
func outlineOf(doc *parser.Document) []Heading {
	headings := []Heading{}
	for i := range doc.Children(0) {
		// only top-level headings: one inside a quote or a list is text
		b := doc.Block(i)
		if b.Kind != parser.Heading {
			continue
		}
		text := strings.TrimSpace(parser.PlainText(doc.Inlines(i)))
		if text == "" {
			continue
		}
		headings = append(headings, Heading{Level: b.Level, Text: text, Slug: Slug(text)})
	}
	return headings
}

// parseSource parses src on its own, without a workspace's cache.
func parseSource(src string) (*parser.Document, []Heading, error) {
	doc := parser.Parse(src)
	return doc, outlineOf(doc), nil
}

// Search returns the lines of a project's documents that hold query, ignoring
// case, at most limit of them.
func (w *Workspace) Search(projectID, query string, limit int) ([]Hit, error) {
	if limit <= 0 {
		limit = 50
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	hits := []Hit{}
	if needle == "" {
		return hits, nil
	}
	docs, err := w.ListDocs(projectID)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		src, err := w.ReadMarkdown(projectID, doc.Path)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(src, "\n") {
			if !strings.Contains(strings.ToLower(line), needle) {
				continue
			}
			hits = append(hits, Hit{Path: doc.Path, Line: i + 1, Text: strings.TrimSpace(line)})
			if len(hits) >= limit {
				return hits, nil
			}
		}
	}
	return hits, nil
}

// RelOf returns path relative to the project root, with slashes, as the
// workspace names documents.
func (w *Workspace) RelOf(projectID, absPath string) (string, error) {
	root, ok := w.Root(projectID)
	if !ok {
		return "", ErrProjectGone
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// AbsOf returns the absolute path of a project document.
func (w *Workspace) AbsOf(projectID, path string) (string, error) {
	return w.resolve(projectID, path)
}

// Stat reports whether a project document still exists.
func (w *Workspace) Stat(projectID, path string) (os.FileInfo, error) {
	abs, err := w.resolve(projectID, path)
	if err != nil {
		return nil, err
	}
	return os.Stat(abs)
}
