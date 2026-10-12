// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPSMemberWithoutKubernetes(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer member-token" {
			t.Error("missing auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	client := Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "member-token", nil }}
	views, err := client.List()
	if err != nil || len(views) != 0 || calls != 1 {
		t.Fatal("member HTTPS path failed", err, calls)
	}
	client.AccessToken = func(context.Context) (string, error) { return "", errors.New("credential unavailable with secret") }
	if _, err = client.List(); err == nil || strings.Contains(err.Error(), "secret") || calls != 1 {
		t.Fatal("credential failure not local and redacted", err, calls)
	}
}
func TestRemoteRedirectNeverForwardsToken(t *testing.T) {
	leaked := 0
	sink := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++; w.WriteHeader(200) }))
	defer sink.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	c := Client{Server: origin.URL, HTTPClient: origin.Client(), AccessToken: func(context.Context) (string, error) { return "private-token", nil }}
	if _, err := c.List(); err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("redirect accepted or token leaked")
	}
	if leaked != 0 {
		t.Fatal("redirect followed")
	}
}
func TestRemoteConfigurationFailsClosed(t *testing.T) {
	token := func(context.Context) (string, error) { return "token", nil }
	for _, server := range []string{"http://localhost:80", "https://user:pass@api.example", "https://api.example/path", "https://api.example?", "https://api.example#fragment", "https://api.example/"} {
		c := Client{Server: server, AccessToken: token}
		if _, _, err := c.openTunnel(context.Background()); err == nil {
			t.Fatal("invalid remote origin accepted", server)
		}
	}
	for _, c := range []Client{{Server: "https://api.example"}, {AccessToken: token}, {Server: "https://api.example", AccessToken: token, Context: "cluster"}} {
		if _, _, err := c.openTunnel(context.Background()); err == nil {
			t.Fatal("ambiguous/missing authority transport accepted")
		}
	}
}
