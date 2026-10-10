// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package modelprovider contains private, bounded OpenAI-compatible HTTP inference.
// It does not grant claim authority; callers must enforce governance before use.
package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
)

const (
	maxPromptBytes   = 64 << 10
	maxRequestBytes  = 128 << 10
	maxResponseBytes = 1 << 20
	maxTokens        = 2048
	maxTimeout       = 2 * time.Minute
)

type Request struct {
	Profile string
	Prompt  string
	// Optional trusted composition-edge format, not worker authority or policy.
	OutputSchema json.RawMessage
}

type Result struct {
	Text         string
	Model        string
	ResponseID   string
	InputTokens  int
	OutputTokens int
}

type Client interface {
	Complete(context.Context, Request) (Result, error)
}

// Config is trusted host-side configuration, never worker input. Endpoint is an
// API base URL (for example http://127.0.0.1:11434/v1). Zero token/timeout values
// default to 256 tokens and 60 seconds; the hard caps are 2048 tokens and 2 minutes.
// HTTPClient is copied and redirects are always disabled, including same-host ones.
type Config struct {
	Endpoint string
	// AllowDockerHostHTTP is only for an operator-selected, local kind/Ollama
	// reference install. It never accepts an arbitrary plaintext hostname.
	AllowDockerHostHTTP bool
	Models              map[string]string
	APIKey              string
	// Credential is an immutable operator-bound handle. It is resolved for each
	// admitted invocation; resolved values never populate APIKey or Adapter state.
	Credential *credentials.Binding
	MaxTokens  int
	// OutputSchema is trusted, private provider configuration, never worker authority.
	OutputSchema json.RawMessage
	Timeout      time.Duration
	HTTPClient   *http.Client
}

type Adapter struct {
	endpoint     string
	models       map[string]string
	apiKey       string
	credential   *credentials.Binding
	maxTokens    int
	outputSchema json.RawMessage
	timeout      time.Duration
	client       *http.Client
}

var _ Client = (*Adapter)(nil)

func New(cfg Config) (*Adapter, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u == nil || u.Opaque != "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(cfg.Endpoint, "#") {
		return nil, errors.New("model provider endpoint is invalid")
	}
	trustedDockerHost := cfg.AllowDockerHostHTTP && u.Hostname() == "host.docker.internal"
	if u.Scheme != "https" && (u.Scheme != "http" || (!loopback(u.Hostname()) && !trustedDockerHost)) {
		return nil, errors.New("model provider requires HTTPS or explicit loopback HTTP")
	}
	if strings.ContainsAny(cfg.APIKey, "\r\n") || (cfg.APIKey != "" && cfg.Credential != nil) {
		return nil, errors.New("model provider credentials are invalid")
	}
	if len(cfg.Models) == 0 {
		return nil, errors.New("model provider profiles are required")
	}
	models := make(map[string]string, len(cfg.Models))
	for profile, model := range cfg.Models {
		if strings.TrimSpace(profile) == "" || strings.TrimSpace(model) == "" || len(profile) > 256 || len(model) > 256 {
			return nil, errors.New("model provider profile configuration is invalid")
		}
		models[profile] = model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 256
	}
	if cfg.MaxTokens < 1 || cfg.MaxTokens > maxTokens {
		return nil, errors.New("model provider token limit is out of bounds")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = time.Minute
	}
	if cfg.Timeout < 0 || cfg.Timeout > maxTimeout {
		return nil, errors.New("model provider timeout is out of bounds")
	}
	client := &http.Client{}
	if cfg.HTTPClient != nil {
		*client = *cfg.HTTPClient
	}
	client.Timeout = cfg.Timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	u.Path = strings.TrimRight(u.Path, "/") + "/chat/completions"
	u.RawPath = ""
	if len(cfg.OutputSchema) > 8192 || (len(cfg.OutputSchema) > 0 && !json.Valid(cfg.OutputSchema)) {
		return nil, errors.New("model output schema is invalid or oversized")
	}
	return &Adapter{endpoint: u.String(), models: models, apiKey: cfg.APIKey, credential: cfg.Credential, maxTokens: cfg.MaxTokens, outputSchema: append(json.RawMessage(nil), cfg.OutputSchema...), timeout: cfg.Timeout, client: client}, nil
}

func loopback(host string) bool {
	// Literal addresses avoid treating a worker-selected DNS name as local.
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *Adapter) Complete(ctx context.Context, req Request) (Result, error) {
	if a == nil || a.client == nil {
		return Result{}, errors.New("model provider is unavailable")
	}
	model, ok := a.models[req.Profile]
	if !ok {
		return Result{}, errors.New("model provider profile is not configured")
	}
	if ctx == nil {
		return Result{}, errors.New("model provider context is required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(req.Prompt) == "" || len(req.Prompt) > maxPromptBytes {
		return Result{}, errors.New("model provider prompt is empty or oversized")
	}
	schema := a.outputSchema
	if len(req.OutputSchema) > 0 {
		if len(req.OutputSchema) > 8192 || !json.Valid(req.OutputSchema) {
			return Result{}, errors.New("model output schema is invalid or oversized")
		}
		schema = req.OutputSchema
	}
	payload := struct {
		Model          string         `json:"model"`
		Messages       []message      `json:"messages"`
		MaxTokens      int            `json:"max_tokens"`
		Stream         bool           `json:"stream"`
		ResponseFormat map[string]any `json:"response_format,omitempty"`
	}{Model: model, Messages: []message{{Role: "user", Content: req.Prompt}}, MaxTokens: a.maxTokens, Stream: false}
	if len(schema) > 0 {
		payload.ResponseFormat = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "agent_action", "strict": true, "schema": schema}}
	}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > maxRequestBytes {
		return Result{}, errors.New("model provider request is oversized or invalid")
	}
	if a.credential == nil {
		legacy := []byte(a.apiKey)
		defer clear(legacy)
		return a.completeHTTP(ctx, model, body, legacy)
	}
	var result Result
	var providerErr error
	useErr := a.credential.Use(ctx, func(invocation context.Context, value []byte) error {
		if !validBearerMaterial(value) {
			providerErr = errors.New("model provider credentials are invalid")
			return providerErr
		}
		result, providerErr = a.completeHTTP(invocation, model, body, value)
		// A provider response is untrusted. Reject an exact resolved value in
		// any returned field rather than exposing it as ordinary business data.
		if providerErr == nil && (bytes.Contains([]byte(result.Text), value) || bytes.Contains([]byte(result.ResponseID), value) || bytes.Contains([]byte(result.Model), value)) {
			result = Result{}
			providerErr = errors.New("model provider response contains private credential material")
		}
		return providerErr
	})
	if useErr != nil {
		// The scoped boundary sanitizes callback errors. Preserve this adapter's
		// already sanitized provider outcome, including its own child deadline;
		// caller termination and resolver errors remain owned by the boundary.
		if errors.Is(useErr, credentials.ErrUse) && providerErr != nil {
			return Result{}, providerErr
		}
		return Result{}, useErr
	}
	return result, providerErr
}

// validBearerMaterial enforces the RFC 6750 token grammar before HTTP dispatch.
// Resolved material remains callback-local; net/http may make internal copies.
func validBearerMaterial(value []byte) bool {
	if len(value) == 0 || len(value) > credentials.MaxValueBytes {
		return false
	}
	padding := false
	for i, b := range value {
		if b == '=' {
			if i == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("-._~+/", rune(b))) {
			return false
		}
	}
	return true
}

func (a *Adapter) completeHTTP(ctx context.Context, model string, body, token []byte) (Result, error) {
	boundedCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(boundedCtx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, errors.New("model provider request could not be created")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if len(token) > 0 {
		httpReq.Header.Set("Authorization", "Bearer "+string(token))
		defer httpReq.Header.Del("Authorization")
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return Result{}, requestFailure(boundedCtx)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, errors.New("model provider returned an unsuccessful status")
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Result{}, requestFailure(boundedCtx)
	}
	if len(responseBody) > maxResponseBytes {
		return Result{}, errors.New("model provider response is oversized")
	}
	if err := boundedCtx.Err(); err != nil {
		return Result{}, err
	}
	var response struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(responseBody, &response) != nil {
		return Result{}, errors.New("model provider response is invalid")
	}
	if len(response.Choices) != 1 || strings.TrimSpace(response.Choices[0].Message.Content) == "" || response.Usage.PromptTokens < 0 || response.Usage.CompletionTokens < 0 {
		return Result{}, errors.New("model provider response has no valid completion")
	}
	// Model identity is the private configured target, not an upstream-selected
	// replacement. Response IDs and usage remain optional provider metadata.
	return Result{Text: response.Choices[0].Message.Content, Model: model, ResponseID: response.ID, InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens}, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func requestFailure(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Never return transport errors: they may contain endpoint credentials,
	// response bodies, or details from a host-configured RoundTripper.
	return errors.New("model provider request failed")
}
