// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Ollama is an explicitly configured local provider. Context options are per
// request; no global daemon settings or model downloads are performed.
type Ollama struct {
	adapter       *Adapter
	contextTokens int
	endpoint      string
}

func NewOllama(cfg Config, contextTokens int) (*Ollama, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u == nil || !loopback(u.Hostname()) || cfg.APIKey != "" {
		return nil, errors.New("Ollama requires an explicit loopback endpoint without a credential")
	}
	if contextTokens < 1024 || contextTokens > 8192 {
		return nil, errors.New("invalid local context bound")
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/") + "/v1"
	a, err := New(cfg)
	if err != nil {
		return nil, err
	}
	return &Ollama{adapter: a, contextTokens: contextTokens, endpoint: strings.TrimRight(u.String(), "/") + "/api/chat"}, nil
}

func (p *Ollama) ConfiguredModel(profile string) string { return p.adapter.ConfiguredModel(profile) }
func (p *Ollama) Complete(ctx context.Context, req Request) (Result, error) {
	a := p.adapter
	model, ok := a.models[req.Profile]
	if ctx == nil || !ok || strings.TrimSpace(req.Prompt) == "" || len(req.Prompt) > maxPromptBytes {
		return Result{}, errors.New("invalid local model request")
	}
	schema := a.outputSchema
	if len(req.OutputSchema) > 0 {
		schema = req.OutputSchema
	}
	if len(schema) > 8192 || (len(schema) > 0 && !json.Valid(schema)) {
		return Result{}, errors.New("invalid local model format")
	}
	payload := struct {
		Model     string          `json:"model"`
		Messages  []message       `json:"messages"`
		Stream    bool            `json:"stream"`
		Format    json.RawMessage `json:"format,omitempty"`
		KeepAlive string          `json:"keep_alive"`
		Options   map[string]any  `json:"options"`
	}{model, []message{{Role: "user", Content: req.Prompt}}, false, schema, "10m", map[string]any{"num_ctx": p.contextTokens, "num_predict": a.maxTokens, "temperature": 0}}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > maxRequestBytes {
		return Result{}, errors.New("invalid local model payload")
	}
	bounded, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(bounded, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, errors.New("invalid local model endpoint")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return Result{}, requestFailure(bounded)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return Result{}, errors.New("local model provider returned an unsuccessful status")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Result{}, requestFailure(bounded)
	}
	if len(raw) > maxResponseBytes {
		return Result{}, errors.New("local model response exceeds the byte limit")
	}
	var result struct {
		Message    message `json:"message"`
		Done       bool    `json:"done"`
		Reason     string  `json:"done_reason"`
		Prompt     int     `json:"prompt_eval_count"`
		Tokens     int     `json:"eval_count"`
		Total      int64   `json:"total_duration"`
		Load       int64   `json:"load_duration"`
		EvalPrompt int64   `json:"prompt_eval_duration"`
		Eval       int64   `json:"eval_duration"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Done || strings.TrimSpace(result.Message.Content) == "" || result.Prompt < 0 || result.Tokens < 0 {
		return Result{}, errors.New("local model returned an invalid completion")
	}
	if err := bounded.Err(); err != nil {
		return Result{}, err
	}
	ms := func(n int64) int64 {
		if n < 0 {
			return 0
		}
		return int64(time.Duration(n) / time.Millisecond)
	}
	return Result{Text: result.Message.Content, Thinking: string(result.Message.Thinking), FinishReason: result.Reason, Model: model, InputTokens: result.Prompt, OutputTokens: result.Tokens, Timings: Timings{TotalMS: ms(result.Total), LoadMS: ms(result.Load), PromptMS: ms(result.EvalPrompt), GenerateMS: ms(result.Eval)}}, nil
}
