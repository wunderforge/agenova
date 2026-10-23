// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPublicWebReaderAcceptsURLsWithoutKeysAndRefusesUnsafeTargets(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://example.org/meeting-features" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Body != nil {
			t.Fatal("unexpected URL or credential")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<main>Public meeting features</main>"))}, nil
	})}
	result, code := readPublicPage(context.Background(), "https://example.org/meeting-features", client)
	if code != "" || !strings.Contains(result, "Public meeting features") || !strings.Contains(result, `"source_id":"public-page"`) {
		t.Fatal(result, code)
	}
	for _, url := range []string{"", "http://example.org/", "https://user:pass@example.org/", "https://127.0.0.1/", "https://169.254.169.254/latest/meta-data", "https://10.1.2.3/", "https://[::1]/", "https://localhost/", "https://example.local/", "https://example.org:8443/", "https://example.org/#fragment", "https://example.org/\nx"} {
		if _, code := readPublicPage(context.Background(), url, client); code != "public-url-refused" {
			t.Fatal("unsafe URL dispatched", url, code)
		}
	}
	if calls != 1 {
		t.Fatal("unsafe URL reached transport")
	}
}
func TestPublicWebReaderActualNoKeyPage(t *testing.T) {
	if os.Getenv("AGENOVA_TEST_PUBLIC_RESEARCH") != "1" {
		t.Skip("explicit public network verification only")
	}
	for _, url := range []string{"https://example.org/", "https://jitsi.org/jitsi-meet/"} {
		result, code := readPublicPage(context.Background(), url, publicClient())
		if code != "" || !strings.Contains(result, "fetched_at") {
			t.Fatal("actual public page read failed", url, code)
		}
	}
}
