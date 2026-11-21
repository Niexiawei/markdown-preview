package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/markdown/parser"

	"mdpreview/internal/config"
	"mdpreview/internal/workspace"
)

// fakeIndex is a workspace with two documents, for the tools' tests.
type fakeIndex struct {
	docs map[string]string
}

func (f fakeIndex) Projects() []workspace.Project {
	return []workspace.Project{{ID: "p1", Name: "notes", Root: "/notes"}}
}

func (f fakeIndex) Active() (workspace.Project, bool) {
	return workspace.Project{ID: "p1", Name: "notes", Root: "/notes"}, true
}

func (f fakeIndex) ListDocs(projectID string) ([]workspace.Doc, error) {
	if projectID != "p1" {
		return nil, workspace.ErrProjectGone
	}
	docs := make([]workspace.Doc, 0, len(f.docs))
	for path, body := range f.docs {
		docs = append(docs, workspace.Doc{Path: path, Title: "t", Size: int64(len(body)), ModTime: time.Unix(1700000000, 0)})
	}
	return docs, nil
}

func (f fakeIndex) ReadMarkdown(projectID, path string) (string, error) {
	body, ok := f.docs[path]
	if !ok {
		return "", workspace.ErrOutsideProject
	}
	return body, nil
}

func (f fakeIndex) Parse(projectID, path string) (*parser.Document, []workspace.Heading, error) {
	body, err := f.ReadMarkdown(projectID, path)
	if err != nil {
		return nil, nil, err
	}
	doc := parser.Parse(body)
	return doc, []workspace.Heading{{Level: 1, Text: "t", Slug: "t"}}, nil
}

func (f fakeIndex) Search(projectID, query string, limit int) ([]workspace.Hit, error) {
	return []workspace.Hit{{Path: "a.md", Line: 1, Text: "hit: " + query}}, nil
}

// newTestServer returns a server with a token, without listening.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Options{Index: fakeIndex{docs: map[string]string{"a.md": "# A\n"}}, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// call posts one JSON-RPC request and returns the decoded answer.
func call(t *testing.T, s *Server, body string, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var out map[string]any
	if rec.Code == http.StatusOK && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("answer is not JSON: %v (%s)", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestInitializeHandshake(t *testing.T) {
	s := newTestServer(t)
	code, out := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, "secret")
	if code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
	result, _ := out["result"].(map[string]any)
	if result["protocolVersion"] == "" {
		t.Errorf("result = %v, want a protocol version", result)
	}
	if _, ok := result["capabilities"].(map[string]any)["tools"]; !ok {
		t.Errorf("result = %v, want tools capabilities", result)
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "markdown-preview" {
		t.Errorf("serverInfo = %v, want markdown-preview", info)
	}
}

func TestToolsListHasFourReadOnlyTools(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "secret")
	result, _ := out["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	names := map[string]bool{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = true
		schema, _ := tool["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("tool %q schema = %v, want an object", name, schema)
		}
		annotations, _ := tool["annotations"].(map[string]any)
		if annotations["readOnlyHint"] != true || annotations["destructiveHint"] == true {
			t.Errorf("tool %q annotations = %v, want read-only", name, annotations)
		}
	}
	if len(tools) != 4 {
		t.Errorf("tools = %v, want 4", names)
	}
	for _, want := range []string{"list_projects", "list_docs", "read_doc", "search_docs"} {
		if !names[want] {
			t.Errorf("tools = %v, want %q", names, want)
		}
	}
}

func TestToolsCallListDocs(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_docs","arguments":{}}}`, "secret")
	result, _ := out["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("result = %v, want no error", result)
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content = %v, want one item", content)
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "a.md") {
		t.Errorf("text = %q, want the document", text)
	}
}

func TestToolsCallReadDocAndSearch(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_doc","arguments":{"path":"a.md"}}}`, "secret")
	text := firstText(t, out)
	if !strings.Contains(text, "# A") {
		t.Errorf("text = %q, want the document's source", text)
	}

	_, out = call(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"search_docs","arguments":{"query":"导出"}}}`, "secret")
	text = firstText(t, out)
	if !strings.Contains(text, "hit: 导出") {
		t.Errorf("text = %q, want the hit", text)
	}

	_, out = call(t, s, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}`, "secret")
	text = firstText(t, out)
	if !strings.Contains(text, "/notes") {
		t.Errorf("text = %q, want the project root", text)
	}
}

func TestToolErrorIsContent(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"read_doc","arguments":{}}}`, "secret")
	result, _ := out["result"].(map[string]any)
	if result["isError"] != true {
		t.Errorf("result = %v, want isError", result)
	}
}

func TestUnknownMethodIsMethodNotFound(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":8,"method":"resources/list"}`, "secret")
	errObj, _ := out["error"].(map[string]any)
	if code, _ := errObj["code"].(float64); int(code) != codeMethodNotFound {
		t.Errorf("error = %v, want %d", errObj, codeMethodNotFound)
	}
}

func TestUnknownToolIsInvalidParams(t *testing.T) {
	s := newTestServer(t)
	_, out := call(t, s, `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"delete_doc"}}`, "secret")
	errObj, _ := out["error"].(map[string]any)
	if code, _ := errObj["code"].(float64); int(code) != codeInvalidParams {
		t.Errorf("error = %v, want %d", errObj, codeInvalidParams)
	}
}

func TestMissingTokenIs401(t *testing.T) {
	s := newTestServer(t)
	code, _ := call(t, s, `{"jsonrpc":"2.0","id":10,"method":"ping"}`, "")
	if code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", code)
	}
	code, _ = call(t, s, `{"jsonrpc":"2.0","id":10,"method":"ping"}`, "wrong")
	if code != http.StatusUnauthorized {
		t.Errorf("code = %d with a wrong token, want 401", code)
	}
}

func TestGetOnEndpointIs405(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
}

func TestNotificationHasNoAnswer(t *testing.T) {
	s := newTestServer(t)
	code, out := call(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "secret")
	if code != http.StatusAccepted {
		t.Errorf("code = %d, want 202", code)
	}
	if len(out) != 0 {
		t.Errorf("answer = %v, want none", out)
	}
}

func TestServerListensOnLoopbackOnly(t *testing.T) {
	s, err := New(Options{Index: fakeIndex{}, Addr: "0.0.0.0:0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(); err == nil {
		s.Close()
		t.Error("Start(0.0.0.0) = nil, want a refusal")
	}
}

func TestServerStartAndClose(t *testing.T) {
	s, err := New(Options{Index: fakeIndex{}, Addr: "127.0.0.1:0", Log: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	addr, err := s.Start()
	if err != nil {
		t.Skipf("cannot listen here: %v", err)
	}
	defer s.Close()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("addr = %q, want loopback", addr)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func firstText(t *testing.T, out map[string]any) string {
	t.Helper()
	result, _ := out["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("answer = %v, want content", out)
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	return text
}

func TestServerServesRealWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\n## Install\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.AddProject(root, "notes")
	ws := workspace.New(cfg)

	srv, err := New(Options{Index: ws, Addr: "127.0.0.1:0", Token: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	addr, err := srv.Start()
	if err != nil {
		t.Skipf("cannot listen here: %v", err)
	}
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_docs","arguments":{}}}`
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(data), "guide.md") {
		t.Errorf("answer = %s, want guide.md", data)
	}
	if !strings.Contains(string(data), "Guide") {
		t.Errorf("answer = %s, want the document's title", data)
	}
}
