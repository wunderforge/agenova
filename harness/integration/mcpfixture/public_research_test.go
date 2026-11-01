// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPublicExtractionAndSpecificFailureStages(t *testing.T) {
	for _, test := range []struct{ kind, body, code, want string }{
		{"text/html", "<nav>junk</nav><article><h1>Real evidence</h1><p>Useful public source facts here.</p><script>bad()</script></article>", "", "Real evidence"},
		{"application/json", `{"current":{"temperature":21.5},"source":"actual fixture"}`, "", "temperature"},
		{"application/json", "{oops", "parse-failed", ""},
		{"text/html", "<main>Please enable JavaScript.</main>", "javascript-required", ""},
		{"image/png", "not a page", "unsupported-content", ""},
		{"text/plain", strings.Repeat("x", (2<<20)+1), "response-too-large", ""},
	} {
		client := &http.Client{Transport: responseTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {test.kind}}, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}
		text, code := readDocument(context.Background(), "x", publicSource{Title: "fixture", URL: "https://example.org"}, client)
		if code != test.code || test.want != "" && !strings.Contains(text, test.want) || strings.Contains(text, "junk") || strings.Contains(text, "bad()") {
			t.Fatalf("extraction: %s %s", code, text)
		}
	}
}
func TestSearchReturnsRealResultFieldsAndRejectsChallenges(t *testing.T) {
	body := `<rss><channel><item><title>Festival</title><link>https://example.org/date</link><description>Observed date.</description></item><item><title>private</title><link>https://127.0.0.1/</link></item></channel></rss>`
	p := rssSearchProvider{endpoint: "https://www.bing.com/search", client: &http.Client{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("q") != "festival" {
			t.Error("query lost")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}}
	rows, code := p.Search(context.Background(), "festival")
	if code != "" || len(rows) != 1 || rows[0].URL != "https://example.org/date" {
		t.Fatal(rows, code)
	}
	body = "<html>captcha</html>"
	if _, code = p.Search(context.Background(), "festival"); code != "search-challenge" {
		t.Fatal(code)
	}
	client := publicClient()
	for _, address := range []string{"http://example.org", "https://127.0.0.1/", "https://169.254.169.254/"} {
		r, _ := http.NewRequest("GET", address, nil)
		if client.CheckRedirect(r, nil) == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
	if !utf8.ValidString(utf8Prefix(strings.Repeat("中", 100), 181)) {
		t.Fatal("split UTF-8")
	}
}
