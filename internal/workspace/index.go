package workspace

import "github.com/egoist/mygo/ui/markdown/parser"

// Index is what the MCP server reads a workspace through: the projects it has
// open, their documents, one document's source and parse, and where a phrase
// appears. It keeps no UI, so a tool call never needs a window.
type Index interface {
	Projects() []Project
	ListDocs(projectID string) ([]Doc, error)
	ReadMarkdown(projectID, path string) (string, error)
	Parse(projectID, path string) (*parser.Document, []Heading, error)
	Search(projectID, query string, limit int) ([]Hit, error)
	Active() (Project, bool)
}

var _ Index = (*Workspace)(nil)
