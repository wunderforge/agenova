// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command mcpfixture serves one read-only MCP tool over Streamable HTTP for
// the E16 reference acceptance. It is an independent module: the official SDK
// and its Go version never enter the Agenova root module.
//
// Every POST is logged as a "receipt" before the SDK dispatches it, so a
// complete pod log proves which tools/call requests reached the server.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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
)

type config struct {
	addr         string
	dataDir      string
	pod          string
	maxFileBytes int64
	slowFile     string
	slowDelay    time.Duration
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
	return cfg, nil
}

type readInput struct {
	File string `json:"file" jsonschema:"relative path of one file inside the fixture dataset"`
}

func newHandler(root *os.Root, cfg config, log *logger) http.Handler {
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
		record := entry{Event: "tool", Tool: toolName, File: bounded(in.File), Correlation: correlation, Outcome: "ok", Bytes: len(text)}
		if code != "" {
			record.Outcome, record.Error, record.Bytes = "error", code, 0
			log.write(record)
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "read_file failed: " + code}}}, nil, nil
		}
		log.write(record)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		SessionTimeout:      5 * time.Minute,
		MaxRequestBodyBytes: maxRequestBytes,
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", receipts(stream, log))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
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
// size and status after it returns.
func receipts(next http.Handler, log *logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		in := entry{Event: "receipt", HTTPMethod: r.Method, Correlation: bounded(r.Header.Get(correlationHeader)), Session: sessionHash(r.Header.Get("Mcp-Session-Id"))}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
			if err != nil || len(body) > maxRequestBytes {
				in.Error = "request-too-large"
				if err != nil {
					in.Error = "request-read-failed"
				}
				log.write(in)
				http.Error(w, "request rejected", http.StatusRequestEntityTooLarge)
				return
			}
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
		log.write(in)
		counter := &countingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(counter, r)
		log.write(entry{Event: "response", HTTPMethod: r.Method, RPCMethod: in.RPCMethod, RPCID: in.RPCID, Correlation: in.Correlation, Status: counter.status, Bytes: counter.bytes, DurationMS: time.Since(start).Milliseconds()})
	})
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
// headers or raw session IDs.
type entry struct {
	Time        string `json:"time"`
	Pod         string `json:"pod,omitempty"`
	Event       string `json:"event"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	RPCMethod   string `json:"rpcMethod,omitempty"`
	RPCID       string `json:"rpcId,omitempty"`
	Tool        string `json:"tool,omitempty"`
	File        string `json:"file,omitempty"`
	Correlation string `json:"correlation,omitempty"`
	Session     string `json:"session,omitempty"`
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
