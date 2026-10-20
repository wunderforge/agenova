// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// agenova-grafana-proxy consumes the existing E14 HTTPS identity and installation
// authorization endpoints. It owns no identity, credential or membership store.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/console"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type memberBoundary struct {
	origin, installation string
	client               *http.Client
}

func (b memberBoundary) call(r *http.Request, path string, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", b.origin+path, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	if body != nil {
		req.Method = "POST"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", b.origin)
	}
	// Only the member session goes to its original trusted HTTPS issuer. Never
	// forward caller identity, Authorization, arbitrary cookies, or redirect.
	var session *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name == "__Host-agenova-session" {
			if session != nil {
				cancel()
				return nil, errors.New("ambiguous session")
			}
			session = c
		}
	}
	if session == nil {
		cancel()
		return nil, errors.New("session required")
	}
	req.AddCookie(session)
	response, err := b.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	response.Body = &cancelBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }
func (b memberBoundary) Authenticate(r *http.Request) (v0.Principal, error) {
	var p v0.Principal
	response, err := b.call(r, "/auth/me", nil)
	if err != nil {
		return p, errors.New("member session unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return p, errors.New("member session denied")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4096))
	if decoder.Decode(&p) != nil || p.Subject == "" || p.Team == "" || p.AuthenticationContext != "oidc" {
		return v0.Principal{}, errors.New("invalid member identity")
	}
	return p, nil
}
func (b memberBoundary) authorize(r *http.Request, _ v0.Principal) error {
	body, _ := json.Marshal(map[string]string{"installation": b.installation, "action": "status"})
	response, err := b.call(r, "/api/platform/authorize", body)
	if err != nil {
		return errors.New("authorization unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 204 {
		return errors.New("diagnostics denied")
	}
	return nil
}
func main() {
	if err := serve(); err != nil {
		log.Fatal(err)
	}
}
func serve() error {
	origin, installation := os.Getenv("AGENOVA_MEMBER_ORIGIN"), os.Getenv("AGENOVA_INSTALLATION")
	if installation == "" {
		return errors.New("serving installation required")
	}
	boundary := memberBoundary{origin: origin, installation: installation, client: &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	handler, err := console.GrafanaProxy(os.Getenv("AGENOVA_GRAFANA_UPSTREAM"), origin, boundary, boundary.authorize)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	entry := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/grafana" || r.URL.Path == "/grafana/") && r.Method == "GET" {
			if _, err := r.Cookie("__Host-agenova-session"); err != nil {
				http.Redirect(w, r, "/login", 303)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
	mux.Handle("/grafana", entry)
	mux.Handle("/grafana/", entry)
	return (&http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10}).ListenAndServe()
}
