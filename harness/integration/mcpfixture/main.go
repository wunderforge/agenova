// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command mcpfixture serves one read-only MCP tool over Streamable HTTP for
// the E16 reference acceptance. It is an independent module: the official SDK
// and its Go version never enter the Agenova root module.
//
// Every POST is logged as a "receipt" before the SDK dispatches it, so a
// complete pod log proves which tools/call requests reached the server.
//
// /mcp is credential-free. When FIXTURE_TOKEN_FILE names a token file, the
// same process also serves /mcp-token through its own SDK handler and session
// table, and answers every request there without the configured bearer token
// with 401 before the SDK sees it. Receipts on both paths record how the
// Authorization header compares with the token, never the header itself.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	version           = "0.1.0"
	toolName          = "read_file"
	correlationHeader = "X-Agenova-Correlation"
	maxRequestBytes   = 16 << 10
	maxArgumentBytes  = 4096
	maxLoggedValue    = 256
	maxTokenBytes     = 4096
	openPath          = "/mcp"
	tokenPath         = "/mcp-token"
	bearerChallenge   = `Bearer realm="agenova-e16-fixture"`
)

type config struct {
	addr         string
	dataDir      string
	pod          string
	maxFileBytes int64
	slowFile     string
	slowDelay    time.Duration
	// tokenDigest is the SHA-256 of the token file, nil when none is set. The
	// token itself is never kept.
	tokenDigest []byte
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcpfixture:", err)
		os.Exit(2)
	}
	root, err := os.OpenRoot(cfg.dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcpfixture: data directory is unavailable")
		os.Exit(2)
	}
	defer root.Close()
	server := &http.Server{Addr: cfg.addr, Handler: newHandler(root, cfg, newLogger(os.Stdout, cfg.pod)), ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "mcpfixture: server stopped")
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{addr: ":8080", dataDir: "/data", pod: getenv("POD_NAME"), maxFileBytes: 1 << 20}
	if v := getenv("FIXTURE_ADDR"); v != "" {
		cfg.addr = v
	}
	if v := getenv("FIXTURE_DATA_DIR"); v != "" {
		cfg.dataDir = v
	}
	if v := getenv("FIXTURE_MAX_FILE_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > 4<<20 {
			return config{}, errors.New("FIXTURE_MAX_FILE_BYTES must be 1..4194304")
		}
		cfg.maxFileBytes = n
	}
	// The slow path exists only in this fixture so timeout evidence needs no
	// fault-injection flag in Agenova itself.
	cfg.slowFile = getenv("FIXTURE_SLOW_FILE")
	if v := getenv("FIXTURE_SLOW_DELAY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 || d > 5*time.Minute {
			return config{}, errors.New("FIXTURE_SLOW_DELAY must be a duration up to 5m")
		}
		cfg.slowDelay = d
	}
	if (cfg.slowFile == "") != (cfg.slowDelay == 0) {
		return config{}, errors.New("FIXTURE_SLOW_FILE and FIXTURE_SLOW_DELAY must be set together")
	}
	if v := getenv("FIXTURE_TOKEN_FILE"); v != "" {
		digest, err := loadTokenDigest(v)
		if err != nil {
			return config{}, err
		}
		cfg.tokenDigest = digest
	}
	return cfg, nil
}

// loadTokenDigest reads the token file once and returns only its SHA-256.
// The bytes are taken as they are: a trailing newline is invalid, not
// trimmed. Errors are constant, so main never prints the file's content.
func loadTokenDigest(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, errors.New("FIXTURE_TOKEN_FILE is unreadable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxTokenBytes+1))
	if err != nil {
		return nil, errors.New("FIXTURE_TOKEN_FILE is unreadable")
	}
	if len(data) > maxTokenBytes || !visibleASCII(data) {
		return nil, errors.New("FIXTURE_TOKEN_FILE must hold 1..4096 visible ASCII bytes and no newline")
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

// visibleASCII reports whether value is non-empty and every byte is 0x21..0x7e.
func visibleASCII[T ~string | ~[]byte](value T) bool {
	if len(value) == 0 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

type readInput struct {
	File string `json:"file" jsonschema:"relative path of one file inside the fixture dataset"`
}

func newHandler(root *os.Root, cfg config, log *logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(openPath, receipts(newStream(root, cfg, log, openPath), log, openPath, cfg.tokenDigest, false))
	// Without a token file the path is not served at all, so no request can
	// ever be accepted on it.
	if cfg.tokenDigest != nil {
		mux.Handle(tokenPath, receipts(newStream(root, cfg, log, tokenPath), log, tokenPath, cfg.tokenDigest, true))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

// newStream builds one SDK server and Streamable HTTP handler for path. Each
// path gets its own, so a session opened on one is unknown on the other.
func newStream(root *os.Root, cfg config, log *logger, path string) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "agenova-e16-fixture", Version: version}, nil)
	closedWorld := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        toolName,
		Description: "Read one text file from the fixture dataset.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(ctx context.Context, req *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, any, error) {
		correlation := ""
		if req.Extra != nil {
			correlation = bounded(req.Extra.Header.Get(correlationHeader))
		}
		text, code := readFile(ctx, root, cfg, in.File)
		record := entry{Event: "tool", Path: path, Tool: toolName, File: bounded(in.File), Correlation: correlation, Outcome: "ok", Bytes: len(text)}
		if code != "" {
			record.Outcome, record.Error, record.Bytes = "error", code, 0
			log.write(record)
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "read_file failed: " + code}}}, nil, nil
		}
		log.write(record)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		SessionTimeout:      5 * time.Minute,
		MaxRequestBodyBytes: maxRequestBytes,
	})
}

// readFile returns file text or a stable error code. os.Root refuses paths
// and symlinks that leave the dataset; ConfigMap's internal ..data links stay
// inside it and resolve normally.
func readFile(ctx context.Context, root *os.Root, cfg config, name string) (string, string) {
	if name == "" || len(name) > maxArgumentBytes || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return "", "invalid-path"
	}
	if cfg.slowFile != "" && name == cfg.slowFile {
		timer := time.NewTimer(cfg.slowDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return "", "cancelled"
		}
	}
	file, err := root.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "not-found"
		}
		return "", "refused"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "not-a-file"
	}
	if info.Size() > cfg.maxFileBytes {
		return "", "too-large"
	}
	data, err := io.ReadAll(io.LimitReader(file, cfg.maxFileBytes+1))
	if err != nil || int64(len(data)) > cfg.maxFileBytes {
		return "", "read-failed"
	}
	if !utf8.Valid(data) {
		return "", "not-text"
	}
	return string(data), ""
}

type rpcEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"params"`
}

// receipts logs each request before the SDK sees it, then logs the response
// size and status after it returns. With requireToken, a request whose auth
// class is not ok gets its receipt and then 401 here; it never reaches next,
// so it can open no session. Without it the class is only logged.
func receipts(next http.Handler, log *logger, path string, digest []byte, requireToken bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		in := entry{Event: "receipt", Path: path, HTTPMethod: r.Method, Correlation: bounded(r.Header.Get(correlationHeader)), Session: sessionHash(r.Header.Get("Mcp-Session-Id")), Auth: authClass(r.Header, digest)}
		bodyRejected := false
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
			if err != nil || len(body) > maxRequestBytes {
				in.Error = "request-too-large"
				if err != nil {
					in.Error = "request-read-failed"
				}
				bodyRejected = true
			} else {
				var envelope rpcEnvelope
				if json.Unmarshal(body, &envelope) != nil {
					in.Error = "unparseable"
				}
				in.RPCMethod, in.RPCID = bounded(envelope.Method), bounded(string(envelope.ID))
				if envelope.Method == "tools/call" {
					in.Tool = bounded(envelope.Params.Name)
					if file, ok := envelope.Params.Arguments["file"].(string); ok {
						in.File = bounded(file)
					}
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		log.write(in)
		counter := &countingWriter{ResponseWriter: w, status: http.StatusOK}
		switch {
		case requireToken && in.Auth != "ok":
			// The token check wins over the body check, so every request
			// without the token on this path is answered 401.
			challenge(counter, in.Auth)
		case bodyRejected:
			http.Error(w, "request rejected", http.StatusRequestEntityTooLarge)
			return
		default:
			next.ServeHTTP(counter, r)
		}
		log.write(entry{Event: "response", Path: path, HTTPMethod: r.Method, RPCMethod: in.RPCMethod, RPCID: in.RPCID, Correlation: in.Correlation, Status: counter.status, Bytes: counter.bytes, DurationMS: time.Since(start).Milliseconds()})
	})
}

// authClass compares the Authorization header with the configured token by
// SHA-256 digest, in constant time over equal 32-byte values. The form is
// checked first, so a malformed header stays malformed with or without a
// token; with no token configured, a well-formed one is invalid, never ok.
func authClass(header http.Header, digest []byte) string {
	values := header.Values("Authorization")
	if len(values) == 0 {
		return "missing"
	}
	token, ok := strings.CutPrefix(values[0], "Bearer ")
	if len(values) > 1 || !ok || !visibleASCII(token) {
		return "malformed"
	}
	sum := sha256.Sum256([]byte(token))
	if digest == nil || subtle.ConstantTimeCompare(sum[:], digest) != 1 {
		return "invalid"
	}
	return "ok"
}

// challenge answers a token-path request whose auth class is not ok, with the
// RFC 6750 error code for a malformed or wrong token and none when missing.
func challenge(w http.ResponseWriter, class string) {
	value := bearerChallenge
	switch class {
	case "malformed":
		value += `, error="invalid_request"`
	case "invalid":
		value += `, error="invalid_token"`
	}
	w.Header().Set("WWW-Authenticate", value)
	http.Error(w, "bearer token required", http.StatusUnauthorized)
}

type countingWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (c *countingWriter) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.bytes += n
	return n, err
}

// Flush and Unwrap keep SSE streaming working through the wrapper.
func (c *countingWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *countingWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// entry is one JSON log line. It never carries file contents, authorization
// headers, tokens or any hash of them, or raw session IDs. Auth holds only
// the receipt's class (ok, missing, malformed or invalid); it is kept out of
// Error, which evidence checks read as a failure.
type entry struct {
	Time        string `json:"time"`
	Pod         string `json:"pod,omitempty"`
	Event       string `json:"event"`
	Path        string `json:"path,omitempty"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	RPCMethod   string `json:"rpcMethod,omitempty"`
	RPCID       string `json:"rpcId,omitempty"`
	Tool        string `json:"tool,omitempty"`
	File        string `json:"file,omitempty"`
	Correlation string `json:"correlation,omitempty"`
	Session     string `json:"session,omitempty"`
	Auth        string `json:"auth,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	Status      int    `json:"status,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	DurationMS  int64  `json:"durationMs,omitempty"`
	Error       string `json:"error,omitempty"`
}

type logger struct {
	mu  sync.Mutex
	out io.Writer
	pod string
	now func() time.Time
}

func newLogger(out io.Writer, pod string) *logger {
	return &logger{out: out, pod: bounded(pod), now: time.Now}
}

func (l *logger) write(e entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Time, e.Pod = l.now().UTC().Format(time.RFC3339Nano), l.pod
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = l.out.Write(append(line, '\n'))
}

func sessionHash(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:6])
}

func bounded(value string) string {
	if len(value) > maxLoggedValue {
		value = value[:maxLoggedValue]
	}
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
