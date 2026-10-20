// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// GrafanaProxy trusts only server-owned configuration and verified application
// sessions. It never passes application cookies/tokens or client-authored
// identity headers to Grafana. Each HTTP request rechecks live authorization.
func GrafanaProxy(target, publicOrigin string, auth Authenticator, authorize func(*http.Request, v0.Principal) (string, error)) (http.Handler, error) {
	upstream, err := url.Parse(target)
	origin, originErr := url.Parse(publicOrigin)
	if err != nil || originErr != nil || auth == nil || authorize == nil || upstream.Host == "" || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" || upstream.Path != "" || upstream.Scheme != "http" || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, errors.New("invalid private Grafana proxy configuration")
	}
	transport := &http.Transport{Proxy: nil, ResponseHeaderTimeout: 15 * time.Second, MaxIdleConnsPerHost: 4}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/grafana" && !strings.HasPrefix(r.URL.Path, "/grafana/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		// No long-lived websocket can outlive a membership/session check.
		if r.Header.Get("Upgrade") != "" {
			http.Error(w, "Streaming unavailable", 403)
			return
		}
		if r.Host != origin.Host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != publicOrigin) {
			http.Error(w, "Origin rejected", 403)
			return
		}
		if r.Method == "POST" && r.Header.Get("Origin") != publicOrigin {
			http.Error(w, "Origin required", 403)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		role, roleErr := authorize(r, p)
		if p.Subject == "" || p.Team == "" || p.AuthenticationContext != "oidc" || roleErr != nil || (role != "Viewer" && role != "Editor") {
			http.Error(w, "Diagnostics access denied", 403)
			return
		}
		if r.Method == "POST" && r.URL.Path != "/grafana/api/ds/query" && r.URL.Path != "/grafana/api/frontend-metrics" && !(role == "Editor" && r.URL.Path == "/grafana/api/dashboards/db") {
			http.Error(w, "Dashboard write denied", 403)
			return
		}
		if r.URL.Path == "/grafana" {
			http.Redirect(w, r, "/grafana/", 303)
			return
		}
		// Role-scoped logins avoid Grafana retaining cached permissions across a
		// live E14 role change; the proxy still checks authority on every request.
		sum := sha256.Sum256([]byte(p.Team + "\x00" + p.Subject + "\x00" + role))
		login := "agenova-" + hex.EncodeToString(sum[:])
		proxy := &httputil.ReverseProxy{Transport: transport, Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = origin.Host
			// Use an explicit allowlist: never forward cookies, bearer tokens, trace
			// baggage, trusted identity headers or arbitrary caller metadata.
			headers := http.Header{}
			for _, name := range []string{"Accept", "Accept-Encoding", "Accept-Language", "Content-Type", "If-None-Match", "If-Modified-Since", "Range", "X-Grafana-Org-Id"} {
				if value := pr.In.Header.Get(name); value != "" {
					headers.Set(name, value)
				}
			}
			headers.Set("X-WEBAUTH-USER", login)
			headers.Set("X-WEBAUTH-ROLE", role)
			headers.Set("X-Forwarded-Proto", "https")
			headers.Set("X-Forwarded-Host", origin.Host)
			if pr.In.Header.Get("Origin") != "" {
				headers.Set("Origin", publicOrigin)
			}
			pr.Out.Header = headers
		}, ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			response.Header.Set("Cache-Control", "no-store")
			return nil
		}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) { http.Error(w, "Grafana unavailable", 502) }}
		proxy.ServeHTTP(w, r)
	}), nil
}
