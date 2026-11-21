// Package mcp serves a workspace to agents over the Model Context Protocol:
// a JSON-RPC endpoint on the loopback interface, with read-only tools that
// list projects and documents, read one, and search them. It starts only
// when the app's settings ask for it.
package mcp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mdpreview/internal/workspace"
)

// protocolVersion is the MCP revision the server answers with.
const protocolVersion = "2025-06-18"

// Options configure a Server.
type Options struct {
	// Index is the workspace the tools read.
	Index workspace.Index
	// Addr is the address to listen on: "127.0.0.1:8765" when empty.
	Addr string
	// Token authenticates callers. A random one is made when empty.
	Token string
	// Log receives one line per call. Nothing is written when nil.
	Log io.Writer
}

// Server is the app's MCP endpoint.
type Server struct {
	index workspace.Index
	token string
	log   io.Writer
	addr  string

	mu  sync.Mutex
	srv *http.Server
	ln  net.Listener
}

// New returns a server for the workspace in o, without listening yet.
func New(o Options) (*Server, error) {
	if o.Index == nil {
		return nil, errors.New("mcp: no index")
	}
	token := o.Token
	if token == "" {
		token = newToken()
	}
	addr := o.Addr
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	return &Server{index: o.Index, token: token, log: o.Log, addr: addr}, nil
}

// newToken makes a token callers authenticate with.
func newToken() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// Token returns the token callers must send.
func (s *Server) Token() string { return s.token }

// Addr returns the address the server listens on, once started.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	return s.addr
}

// Start listens on its address and serves until Close. It reports the
// address it listens on, which differs from its configured one when the port
// was taken and Addr ended with :0.
func (s *Server) Start() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String(), nil
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return "", err
	}
	if !isLoopback(ln.Addr().String()) {
		ln.Close()
		return "", errors.New("mcp: the address is not on the loopback interface")
	}
	s.ln = ln
	s.srv = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go s.srv.Serve(ln)
	return ln.Addr().String(), nil
}

// isLoopback reports whether a listen address is on 127.0.0.0/8 or ::1.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Close stops the server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	err := s.srv.Close()
	s.srv, s.ln = nil, nil
	return err
}

// Handler returns the endpoint, which tests call without listening.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handle)
	return mux
}

// handle answers one JSON-RPC request or notification.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "cannot read the request", http.StatusBadRequest)
		return
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		s.write(w, http.StatusOK, newError(nil, errorf(codeParseError, "invalid JSON: %v", err)))
		s.trace(r, req, start, http.StatusOK)
		return
	}
	if resp, ok := s.dispatch(req); ok {
		s.write(w, http.StatusOK, resp)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
	s.trace(r, req, start, http.StatusOK)
}

// authorized checks the bearer token, in constant time.
func (s *Server) authorized(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimPrefix(header, prefix)
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

func (s *Server) write(w http.ResponseWriter, code int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}

// trace logs one call, so that a user can see what an agent asked.
func (s *Server) trace(r *http.Request, req request, start time.Time, code int) {
	if s.log == nil {
		return
	}
	fmt.Fprintf(s.log, "%s %s method=%s id=%s status=%d in %s\n",
		time.Now().Format(time.RFC3339), r.RemoteAddr, req.Method, string(req.ID), code, time.Since(start).Round(time.Millisecond))
}

// dispatch answers one request, reporting whether it has an answer at all: a
// notification has none.
func (s *Server) dispatch(req request) (response, bool) {
	switch req.Method {
	case "initialize":
		return newResponse(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "markdown-preview",
				"version": "0.1.0",
			},
			"instructions": "Read-only access to the markdown documents of the projects this app has open.",
		}), true
	case "notifications/initialized", "notifications/cancelled":
		return response{}, false
	case "ping":
		return newResponse(req.ID, map[string]any{}), true
	case "tools/list":
		return newResponse(req.ID, map[string]any{"tools": s.toolSchemas()}), true
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			return newError(req.ID, err), true
		}
		return newResponse(req.ID, result), true
	default:
		return newError(req.ID, errorf(codeMethodNotFound, "unknown method %q", req.Method)), true
	}
}

// WriteClientInfo writes the endpoint and its token into dir, as a note to
// whoever configures an agent.
func (s *Server) WriteClientInfo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	info := map[string]string{
		"url":   "http://" + s.Addr() + "/mcp",
		"token": s.token,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "mcp.json"), append(data, '\n'), 0o600)
}
