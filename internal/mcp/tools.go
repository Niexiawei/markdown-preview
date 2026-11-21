package mcp

import (
	"encoding/json"
	"time"
)

// tool is one read-only tool the server offers.
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	run         func(args map[string]any) (any, *rpcError)
}

// toolSchemas returns the tools the server lists, in a fixed order.
func (s *Server) tools() []tool {
	return []tool{
		{
			Name:        "list_projects",
			Description: "List the markdown projects this app has open, with their root directories and which one is active.",
			InputSchema: objectSchema(nil),
			Annotations: readOnly(),
			run: func(map[string]any) (any, *rpcError) {
				projects := s.index.Projects()
				active, hasActive := s.index.Active()
				out := make([]map[string]any, 0, len(projects))
				for _, p := range projects {
					out = append(out, map[string]any{
						"id":     p.ID,
						"name":   p.Name,
						"root":   p.Root,
						"active": hasActive && active.ID == p.ID,
					})
				}
				return out, nil
			},
		},
		{
			Name:        "list_docs",
			Description: "List the markdown documents of a project, with their paths relative to the project root.",
			InputSchema: objectSchema(map[string]any{
				"project": map[string]any{"type": "string", "description": "Project id. The active project when omitted."},
			}),
			Annotations: readOnly(),
			run: func(args map[string]any) (any, *rpcError) {
				projectID, err := s.projectID(args)
				if err != nil {
					return nil, err
				}
				docs, err2 := s.index.ListDocs(projectID)
				if err2 != nil {
					return nil, errorf(codeInternalError, "cannot list documents: %v", err2)
				}
				out := make([]map[string]any, 0, len(docs))
				for _, d := range docs {
					out = append(out, map[string]any{
						"path":     d.Path,
						"title":    d.Title,
						"size":     d.Size,
						"mod_time": d.ModTime.UTC().Format(time.RFC3339),
					})
				}
				return out, nil
			},
		},
		{
			Name:        "read_doc",
			Description: "Read one markdown document, whole, as its source.",
			InputSchema: objectSchema(map[string]any{
				"path":    map[string]any{"type": "string", "description": "Path relative to the project root, as list_docs gives it."},
				"project": map[string]any{"type": "string", "description": "Project id. The active project when omitted."},
			}, "path"),
			Annotations: readOnly(),
			run: func(args map[string]any) (any, *rpcError) {
				path, _ := args["path"].(string)
				if path == "" {
					return nil, errorf(codeInvalidParams, "path is required")
				}
				projectID, err := s.projectID(args)
				if err != nil {
					return nil, err
				}
				src, readErr := s.index.ReadMarkdown(projectID, path)
				if readErr != nil {
					return nil, errorf(codeInternalError, "cannot read %q: %v", path, readErr)
				}
				return map[string]any{"path": path, "content": src}, nil
			},
		},
		{
			Name:        "search_docs",
			Description: "Find the lines of a project's documents that hold a phrase, ignoring case.",
			InputSchema: objectSchema(map[string]any{
				"query":   map[string]any{"type": "string", "description": "The phrase to look for."},
				"project": map[string]any{"type": "string", "description": "Project id. The active project when omitted."},
				"limit":   map[string]any{"type": "integer", "description": "How many lines to return at most; 50 when omitted."},
			}, "query"),
			Annotations: readOnly(),
			run: func(args map[string]any) (any, *rpcError) {
				query, _ := args["query"].(string)
				if query == "" {
					return nil, errorf(codeInvalidParams, "query is required")
				}
				projectID, err := s.projectID(args)
				if err != nil {
					return nil, err
				}
				limit := 0
				if v, ok := args["limit"].(float64); ok {
					limit = int(v)
				}
				hits, searchErr := s.index.Search(projectID, query, limit)
				if searchErr != nil {
					return nil, errorf(codeInternalError, "cannot search: %v", searchErr)
				}
				out := make([]map[string]any, 0, len(hits))
				for _, h := range hits {
					out = append(out, map[string]any{"path": h.Path, "line": h.Line, "text": h.Text})
				}
				return out, nil
			},
		},
	}
}

// toolSchemas returns what tools/list answers with.
func (s *Server) toolSchemas() []map[string]any {
	tools := s.tools()
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
			"annotations": t.Annotations,
		})
	}
	return out
}

// callTool runs the tool a tools/call names.
func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, errorf(codeInvalidParams, "invalid params: %v", err)
	}
	if call.Name == "" {
		return nil, errorf(codeInvalidParams, "name is required")
	}
	for _, t := range s.tools() {
		if t.Name != call.Name {
			continue
		}
		result, err := t.run(call.Arguments)
		if err != nil {
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			return nil, errorf(codeInternalError, "cannot encode the result: %v", marshalErr)
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": string(data)}},
			"isError": false,
		}, nil
	}
	return nil, errorf(codeInvalidParams, "unknown tool %q", call.Name)
}

// projectID is the project a call names, or the active one.
func (s *Server) projectID(args map[string]any) (string, *rpcError) {
	if id, ok := args["project"].(string); ok && id != "" {
		for _, p := range s.index.Projects() {
			if p.ID == id {
				return p.ID, nil
			}
		}
		return "", errorf(codeInvalidParams, "no project %q", id)
	}
	active, ok := s.index.Active()
	if !ok {
		return "", errorf(codeInvalidParams, "no active project: name one with project")
	}
	return active.ID, nil
}

// objectSchema is an input schema of properties, all optional unless named
// in required.
func objectSchema(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// readOnly marks a tool as changing nothing.
func readOnly() map[string]any {
	return map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}
}
