// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// agenova-mcp-kubernetes exposes one bounded, read-only Kubernetes list tool
// over the Streamable HTTP subset used by the Agenova control plane.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	protocolVersion   = "2025-06-18"
	correlationHeader = "X-Agenova-Correlation"
	maxRequestBytes   = 8192
	maxResultBytes    = 60 << 10
)

type config struct {
	listen, namespace, tokenFile, tlsCertFile, tlsKeyFile string
	allowed                                               map[string]bool
}

type commandRunner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type kubectlRunner struct{}

func (kubectlRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.New("kubernetes read failed")
	}
	return stdout.Bytes(), nil
}

type server struct {
	config config
	runner commandRunner
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	s := &server{config: cfg, runner: kubectlRunner{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.Handle("POST /mcp", s)
	httpServer := &http.Server{Addr: cfg.listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("Agenova Kubernetes MCP listening on %s", cfg.listen)
	log.Fatal(httpServer.ListenAndServeTLS(cfg.tlsCertFile, cfg.tlsKeyFile))
}

func loadConfig() (config, error) {
	cfg := config{
		listen:      strings.TrimSpace(os.Getenv("AGENOVA_MCP_LISTEN")),
		namespace:   strings.TrimSpace(os.Getenv("AGENOVA_MCP_NAMESPACE")),
		tokenFile:   strings.TrimSpace(os.Getenv("AGENOVA_MCP_BEARER_TOKEN_FILE")),
		tlsCertFile: strings.TrimSpace(os.Getenv("AGENOVA_MCP_TLS_CERT_FILE")),
		tlsKeyFile:  strings.TrimSpace(os.Getenv("AGENOVA_MCP_TLS_KEY_FILE")),
		allowed:     map[string]bool{},
	}
	if cfg.listen == "" {
		cfg.listen = ":8080"
	}
	if cfg.namespace == "" || cfg.tokenFile == "" || cfg.tlsCertFile == "" || cfg.tlsKeyFile == "" {
		return config{}, errors.New("namespace, bearer token file and TLS certificate/key files are required")
	}
	for _, kind := range strings.Split(os.Getenv("AGENOVA_MCP_ALLOWED_KINDS"), ",") {
		kind = strings.TrimSpace(kind)
		if kind != "" {
			cfg.allowed[kind] = true
		}
	}
	if len(cfg.allowed) == 0 {
		return config{}, errors.New("AGENOVA_MCP_ALLOWED_KINDS requires at least one exact resource kind")
	}
	return cfg, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("Content-Type") != "application/json" || !strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	correlation := r.Header.Get(correlationHeader)
	if !visibleASCII(correlation, 256) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
	if err != nil || len(body) > maxRequestBytes {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	var req rpcRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF || req.JSONRPC != "2.0" || req.Method == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch req.Method {
	case "initialize":
		s.initialize(w, req)
	case "notifications/initialized":
		if r.Header.Get("MCP-Protocol-Version") != protocolVersion || len(req.ID) != 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	case "tools/call":
		if r.Header.Get("MCP-Protocol-Version") != protocolVersion {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.call(w, r, req)
	default:
		s.rpcError(w, req.ID, -32601)
	}
}

func (s *server) initialize(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(req.Params, &params) != nil || params.ProtocolVersion != protocolVersion || len(req.ID) == 0 {
		s.rpcError(w, req.ID, -32602)
		return
	}
	s.rpcResult(w, req.ID, map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "agenova-mcp-kubernetes", "version": "0.1.0"},
	})
}

func (s *server) call(w http.ResponseWriter, r *http.Request, req rpcRequest) {
	if len(req.ID) == 0 {
		s.rpcError(w, req.ID, -32600)
		return
	}
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if json.Unmarshal(req.Params, &params) != nil || params.Name != "list_resources" || len(params.Arguments) != 1 {
		s.rpcError(w, req.ID, -32602)
		return
	}
	kind, ok := params.Arguments["kind"]
	if !ok || !s.config.allowed[kind] {
		s.rpcError(w, req.ID, -32602)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	data, err := s.runner.Run(ctx, "get", kind, "--namespace", s.config.namespace, "--output=json", "--request-timeout=5s")
	if err != nil {
		s.rpcResult(w, req.ID, map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": "Kubernetes read failed"}}})
		return
	}
	text, err := boundedResourceNames(data)
	if err != nil {
		s.rpcResult(w, req.ID, map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": "Kubernetes response was invalid"}}})
		return
	}
	s.rpcResult(w, req.ID, map[string]any{"content": []map[string]string{{"type": "text", "text": text}}})
}

func boundedResourceNames(data []byte) (string, error) {
	var list struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Items      []struct {
			Metadata struct{ Name, Namespace string } `json:"metadata"`
		} `json:"items"`
	}
	if len(data) > 2<<20 || json.Unmarshal(data, &list) != nil || list.Kind == "" || list.Items == nil {
		return "", errors.New("invalid Kubernetes list")
	}
	rows := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		if item.Metadata.Name == "" || strings.ContainsAny(item.Metadata.Name, "\r\n\t") {
			return "", errors.New("invalid resource name")
		}
		rows = append(rows, item.Metadata.Namespace+"/"+item.Metadata.Name)
	}
	sort.Strings(rows)
	result, err := json.Marshal(map[string]any{"apiVersion": list.APIVersion, "kind": list.Kind, "resources": rows})
	if err != nil || len(result) > maxResultBytes {
		return "", errors.New("result too large")
	}
	return string(result), nil
}

func (s *server) authorized(r *http.Request) bool {
	want, err := os.ReadFile(s.config.tokenFile)
	if err != nil || len(want) == 0 || len(want) > 4096 {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !visibleASCII(got, 4096) || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), want) == 1
}

func (s *server) rpcResult(w http.ResponseWriter, id json.RawMessage, result any) {
	s.writeRPC(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (s *server) rpcError(w http.ResponseWriter, id json.RawMessage, code int) {
	s.writeRPC(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": "request rejected"}})
}

func (s *server) writeRPC(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write MCP response: %v", err)
	}
}

func visibleASCII(value string, max int) bool {
	if value == "" || len(value) > max {
		return false
	}
	for _, c := range []byte(value) {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
