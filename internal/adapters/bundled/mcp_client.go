// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/toolbackend"
)

// MCPCorrelationHeader carries the host-issued invocation ID on every request
// of one call so server logs can be matched to Agenova facts. Workers never
// set it; the value is non-secret.
const MCPCorrelationHeader = "X-Agenova-Correlation"

const mcpProtocolVersion = "2025-06-18"

type mcpRoute struct {
	tool      string
	parameter string
}

// mcpClient implements the documented Streamable HTTP subset: one short
// session per invocation (initialize, initialized, tools/call, DELETE). It
// never replays tools/call and advertises no client capabilities.
type mcpClient struct {
	endpoint    string
	timeout     time.Duration
	maxRequest  int
	maxResponse int
	concurrency int
	routes      map[string]mcpRoute
	http        *http.Client
}

// MaxConcurrentCalls exposes the configured ceiling; toolbackend.Set enforces it.
func (c *mcpClient) MaxConcurrentCalls() int { return c.concurrency }

func newMCPClient(endpoint string, timeout time.Duration, maxRequest, maxResponse int, routes map[string]mcpRoute) *mcpClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // The fixed endpoint must not be rerouted by environment proxies.
	transport.MaxResponseHeaderBytes = 16 << 10
	return &mcpClient{endpoint: endpoint, timeout: timeout, maxRequest: maxRequest, maxResponse: maxResponse, routes: routes, http: &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func mcpRouteKey(operation, scope string) string { return operation + "\x00" + scope }

func (c *mcpClient) Invoke(ctx context.Context, call toolbackend.Invocation) (toolbackend.Result, error) {
	route, ok := c.routes[mcpRouteKey(call.Operation, call.ResourceScope)]
	value, present := call.Parameters[route.parameter]
	if !ok || !present || len(call.Parameters) != 1 {
		return toolbackend.Result{}, toolbackend.ErrArguments
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	session := &mcpSession{client: c, correlation: call.ID}
	defer session.close()
	result, err := session.run(ctx, route.tool, route.parameter, value)
	if err != nil {
		return toolbackend.Result{}, classifyMCPError(ctx, err)
	}
	ref := call.ResourceScope + "/" + value
	if len(ref) > 256 {
		ref = ""
	}
	return toolbackend.Result{Text: result, ResultRef: ref}, nil
}

// classifyMCPError maps transport details to the neutral safe errors. Raw
// server text never leaves this package.
func classifyMCPError(ctx context.Context, err error) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return toolbackend.ErrTimeout
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, toolbackend.ErrResponseTooLarge), errors.Is(err, toolbackend.ErrProtocol),
		errors.Is(err, toolbackend.ErrUnavailable), errors.Is(err, toolbackend.ErrProvider), errors.Is(err, toolbackend.ErrArguments):
		return err
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return toolbackend.ErrTimeout
	}
	return toolbackend.ErrUnavailable
}

type mcpSession struct {
	client      *mcpClient
	correlation string
	id          string
	initialized bool
	nextID      int
}

func (s *mcpSession) run(ctx context.Context, tool, parameter, value string) (string, error) {
	initResult, err := s.request(ctx, "initialize", map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "agenova-control-plane", "version": ReferenceVersion},
	})
	if err != nil {
		return "", err
	}
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(initResult, &init) != nil || init.ProtocolVersion != mcpProtocolVersion {
		return "", toolbackend.ErrProtocol
	}
	s.initialized = true
	if err := s.notify(ctx, "notifications/initialized"); err != nil {
		return "", err
	}
	callResult, err := s.request(ctx, "tools/call", map[string]any{"name": tool, "arguments": map[string]string{parameter: value}})
	if err != nil {
		return "", err
	}
	var result struct {
		Content []struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if json.Unmarshal(callResult, &result) != nil || len(result.Content) == 0 {
		return "", toolbackend.ErrProtocol
	}
	if result.IsError {
		return "", toolbackend.ErrProvider
	}
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if item.Type != "text" || item.Text == nil {
			return "", toolbackend.ErrProtocol
		}
		parts = append(parts, *item.Text)
	}
	return strings.Join(parts, "\n"), nil
}

func (s *mcpSession) headers(req *http.Request) {
	req.Header.Set(MCPCorrelationHeader, s.correlation)
	if s.initialized {
		req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	}
	if s.id != "" {
		req.Header.Set("Mcp-Session-Id", s.id)
	}
}

func (s *mcpSession) post(ctx context.Context, message map[string]any) (*http.Response, error) {
	body, err := json.Marshal(message)
	if err != nil || len(body) > s.client.maxRequest {
		return nil, toolbackend.ErrArguments
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.client.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, toolbackend.ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	s.headers(req)
	return s.client.http.Do(req)
}

func (s *mcpSession) notify(ctx context.Context, method string) error {
	resp, err := s.post(ctx, map[string]any{"jsonrpc": "2.0", "method": method})
	if err != nil {
		return err
	}
	defer drain(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return statusError(resp.StatusCode)
	}
	return nil
}

func (s *mcpSession) request(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	s.nextID++
	id := s.nextID
	resp, err := s.post(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	defer drain(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode)
	}
	if method == "initialize" {
		session := resp.Header.Get("Mcp-Session-Id")
		if session != "" && !visibleASCII(session, 256) {
			return nil, toolbackend.ErrProtocol
		}
		s.id = session
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, toolbackend.ErrProtocol
	}
	body := &limitedBody{r: resp.Body, remaining: s.client.maxResponse}
	switch mediaType {
	case "application/json":
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		return matchResponse(data, id)
	case "text/event-stream":
		return readEventStream(body, id)
	}
	return nil, toolbackend.ErrProtocol
}

// close ends the session best-effort. It uses its own short deadline so a
// cancelled invocation still releases server state, and never retries.
func (s *mcpSession) close() {
	if s.id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.client.endpoint, nil)
	if err != nil {
		return
	}
	s.headers(req)
	if resp, err := s.client.http.Do(req); err == nil {
		drain(resp.Body)
	}
}

func statusError(status int) error {
	if status >= 500 {
		return toolbackend.ErrUnavailable
	}
	return toolbackend.ErrProtocol // Includes 3xx redirects, 404 session expiry and 4xx rejections.
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// matchResponse accepts exactly one JSON-RPC response with the expected ID.
func matchResponse(data []byte, id int) (json.RawMessage, error) {
	var message rpcMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&message) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, toolbackend.ErrProtocol
	}
	result, matched, err := message.response(id)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, toolbackend.ErrProtocol
	}
	return result, nil
}

func (m rpcMessage) response(id int) (json.RawMessage, bool, error) {
	if m.JSONRPC != "2.0" {
		return nil, false, toolbackend.ErrProtocol
	}
	if len(m.ID) == 0 || string(m.ID) == "null" {
		if m.Method == "" {
			return nil, false, toolbackend.ErrProtocol
		}
		return nil, false, nil // Server notification: ignored.
	}
	if m.Method != "" {
		// A server request would need a client capability we never advertise.
		return nil, false, toolbackend.ErrProtocol
	}
	var got int
	if json.Unmarshal(m.ID, &got) != nil || got != id {
		return nil, false, toolbackend.ErrProtocol
	}
	if m.Error != nil {
		return nil, false, toolbackend.ErrProvider
	}
	if len(m.Result) == 0 {
		return nil, false, toolbackend.ErrProtocol
	}
	return m.Result, true, nil
}

// readEventStream reads SSE events until the response for id arrives, then
// reads the rest of the stream to its end. The whole stream, including
// anything after the response, shares one byte budget, so a valid result
// followed by an oversized tail is still rejected. Streamable HTTP servers
// close the stream after the response; one that keeps it open runs into the
// invocation deadline.
func readEventStream(body io.Reader, id int) (json.RawMessage, error) {
	reader := bufio.NewReaderSize(body, 4096)
	var data strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// A partial last line cannot complete an event.
				return nil, toolbackend.ErrProtocol
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if data.Len() == 0 {
				continue
			}
			var message rpcMessage
			if json.Unmarshal([]byte(data.String()), &message) != nil {
				return nil, toolbackend.ErrProtocol
			}
			data.Reset()
			result, matched, err := message.response(id)
			if err != nil {
				return nil, err
			}
			if matched {
				if _, err := io.Copy(io.Discard, reader); err != nil {
					return nil, err
				}
				return result, nil
			}
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		// event:, id:, retry: and comment lines carry nothing this subset uses.
	}
}

type limitedBody struct {
	r         io.Reader
	remaining int
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		// Probe one byte so a body of exactly the limit is still accepted.
		var probe [1]byte
		n, err := l.r.Read(probe[:])
		if n > 0 {
			return 0, toolbackend.ErrResponseTooLarge
		}
		return 0, err
	}
	if len(p) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= n
	return n, err
}

func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
	_ = body.Close()
}

func visibleASCII(value string, limit int) bool {
	if len(value) > limit {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}
