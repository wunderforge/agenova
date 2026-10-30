// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/accessadvisor"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type volatileAuth struct{}

type volatileTemplates struct{ store *registration.MemoryStore }

func (t volatileTemplates) Lookup(name string) (*v0.AgentTemplate, error) {
	return t.store.Template(name)
}

func (volatileAuth) Authenticate(r *http.Request) (v0.Principal, error) {
	c, e := r.Cookie("agenova_local_advisor")
	if e != nil || c.Value != "local-owner" {
		return v0.Principal{}, authentication.ErrUnauthenticated
	}
	return v0.Principal{Subject: "local-owner", Team: "team-a", AuthenticationContext: "oidc"}, nil
}
func (a volatileAuth) AuthenticateSession(r *http.Request) (v0.Principal, func(context.Context) error, error) {
	p, e := a.Authenticate(r)
	return p, func(ctx context.Context) error { _, e := a.Authenticate(r.WithContext(ctx)); return e }, e
}
func (volatileAuth) Authorize(_ context.Context, _ v0.Principal, action, project string) error {
	if project == "research" && (action == "work.submit" || action == "work.read-own") {
		return nil
	}
	return authentication.ErrForbidden
}

type syntheticAdviceTool struct{}

func (syntheticAdviceTool) Invoke(_ context.Context, in toolbackend.Invocation) (toolbackend.Result, error) {
	text := "SYNTHETIC local public-page fixture: ephemeral meeting rooms and protecting room names. No live page was fetched."
	if in.Operation == "crm.read" {
		b, e := os.ReadFile("harness/demo/incident-mcp/data/crm/customers.json")
		if e != nil {
			return toolbackend.Result{}, e
		}
		text = "SYNTHETIC customer records: " + string(b)
	}
	return toolbackend.Result{Text: text, Untrusted: true}, nil
}

func serveVolatile(ctx context.Context, planner *accessadvisor.Service) error {
	data, e := os.ReadFile("harness/examples/access-advisor/researcher.yaml")
	if e != nil {
		return e
	}
	template, invalid := v0.ParseAgentTemplateYAML(data)
	if invalid != nil {
		return invalid
	}
	guide, e := os.ReadFile("harness/examples/access-advisor/ACCESS.md")
	if e != nil {
		return e
	}
	template.Spec.AccessGuide, e = v0.ParseAccessGuideMarkdown(guide)
	if e != nil {
		return e
	}
	if e := v0.ValidateAgentTemplate(template); e != nil {
		return e
	}
	registry := registration.NewMemoryStore()
	if _, e := registry.PutTemplate(template); e != nil {
		return e
	}
	bundle := policy.PolicyBundle{ID: "local-advisor-policy", Version: "1", Rules: []policy.Rule{{Team: "team-a", Action: "claim.create", Project: "research", TemplateRef: template.Metadata.Name, Constraints: &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{}}}}}}
	loader := &policy.Loader{}
	if e := loader.Load(bundle); e != nil {
		return e
	}
	auth := volatileAuth{}
	setup := func(_ context.Context, p v0.Principal, project, name string) (console.Setup, error) {
		if project != "" && project != "research" || name != "" && name != template.Metadata.Name {
			return console.Setup{}, authentication.ErrForbidden
		}
		// Reparse a snapshot so the server registry cannot be mutated by consumers.
		copy, e := registry.Template(template.Metadata.Name)
		if e != nil {
			return console.Setup{}, e
		}
		return console.Setup{Principal: p, Project: "research", Projects: []string{"research"}, Templates: []string{template.Metadata.Name}, Template: copy, Policy: bundle, Installation: console.InstallationIdentity{Kind: "installed", Platform: "LOCAL ADVISOR ACCEPTANCE · volatile Work history", Revision: "sha256:" + strings.Repeat("b", 64)}, Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "synthetic", "model": "synthetic", "tool": "mock", "memory": "notConnected"}}, nil
	}
	bindings := []toolbackend.Binding{}
	for _, d := range []toolbackend.Descriptor{{Operation: "web.read", Description: "Synthetic public-page fixture for local Advisor UI acceptance", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}, {Operation: "crm.read", Description: "Synthetic customer retention fixture", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", MaxBytes: 64, AllowedValues: []string{"customer-retention"}}} {
		bindings = append(bindings, toolbackend.Binding{Descriptor: d, Provider: syntheticAdviceTool{}, Backend: "synthetic-local-fixtures", MaxObservationBytes: 8192, MaxConcurrentCalls: 1})
	}
	tools, e := toolbackend.NewSet(bindings)
	if e != nil {
		return e
	}
	backend := &previewRuntime{claims: map[string]string{}, released: map[string]bool{}, authority: map[string]runtime.NetworkAuthority{}}
	service, e := console.NewServiceWithOptions(backend, previewExecutor{templateFor: backend.templateFor}, syntheticProvider{}, "", console.Options{Advisor: planner, ToolBackend: tools, Access: auth, Setup: func() (console.Setup, error) { return setup(ctx, v0.Principal{}, "", "") }, SetupFor: setup, Configure: func(t *v0.AgentTemplate) (app.ResolvedLaunch, error) {
		return app.ResolvedLaunch{ProfileRef: "standard-isolated", TemplateRef: t.Metadata.Name}, nil
	}, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, source{p}, loader, volatileTemplates{registry})
	}})
	if e != nil {
		return e
	}
	defer service.Close()
	api, e := console.AuthenticatedHandler(service, auth)
	if e != nil {
		return e
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("local-advisor-ready"))
	})
	mux.Handle("/api/", api)
	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		p, e := auth.Authenticate(r)
		if e != nil {
			http.Error(w, "Local acceptance sign-in required", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"subject": p.Subject, "team": p.Team})
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "agenova_local_advisor", Value: "local-owner", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/?mode=connected&project=research&template="+template.Metadata.Name+"#/work/new", 303)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "agenova_local_advisor", Value: "", Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/login", 303)
	})
	dir, _ := filepath.Abs("ui/dist")
	assets := http.FileServer(http.Dir(dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			assets.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if _, e := auth.Authenticate(r); e != nil {
			http.Redirect(w, r, "/login", 303)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != address || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Loopback only", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Printf("Local Advisor acceptance: http://%s/login ; real configured Advisor, synthetic task execution, in-memory Work history. No AWS.\n", address)
	if e := server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		return e
	}
	return nil
}
