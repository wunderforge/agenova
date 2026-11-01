// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package modelprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeOllamaUsesBoundedOptionsAndActualProviderContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected native request")
		}
		var p struct {
			Options  map[string]int `json:"options"`
			Messages []message      `json:"messages"`
			Stream   bool           `json:"stream"`
		}
		if json.NewDecoder(r.Body).Decode(&p) != nil {
			t.Error("invalid JSON")
		}
		if p.Options["num_ctx"] != 2048 || p.Options["num_predict"] != 100 || p.Stream || p.Messages[0].Content != "goal" {
			t.Error("request options changed")
		}
		w.Write([]byte(`{"done":true,"done_reason":"stop","message":{"content":"answer","thinking":"provider-visible text"},"prompt_eval_count":10,"eval_count":5,"load_duration":1000000,"eval_duration":2000000}`))
	}))
	defer server.Close()
	p, err := NewOllama(Config{Endpoint: server.URL, Models: map[string]string{"m": "local"}, MaxTokens: 100}, 2048)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Complete(context.Background(), Request{Profile: "m", Prompt: "goal"})
	if err != nil || r.Text != "answer" || r.Thinking != "provider-visible text" || r.FinishReason != "stop" || r.Timings.GenerateMS != 2 {
		t.Fatalf("lost actual response: %+v %v", r, err)
	}
	for _, url := range []string{"http://example.org", "http://127.0.0.1?key=x", "http://user:pass@127.0.0.1"} {
		if _, err := NewOllama(Config{Endpoint: url, Models: map[string]string{"m": "x"}}, 2048); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
