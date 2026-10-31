// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// A loopback-only acceptance composition, NOT a production login mode or image.
// Real PostgreSQL/admission/evidence; synthetic identities, runtime and provider.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/landing"
	"github.com/wunderforge/agenova/internal/management"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"github.com/wunderforge/agenova/internal/workstore"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var address = "127.0.0.1:5210"

type localAuth struct{ store identitydb.Store }

func (a localAuth) Authenticate(r *http.Request) (v0.Principal, error) {
	c, err := r.Cookie("agenova_local_acceptance")
	if err != nil || c.Value != "owner" && c.Value != "member" && c.Value != "viewer" {
		return v0.Principal{}, authentication.ErrUnauthenticated
	}
	p := v0.Principal{Subject: "local-" + c.Value, Team: "team-a", AuthenticationContext: "oidc"}
	var active bool
	if a.store.DB.QueryRowContext(r.Context(), `SELECT active FROM agenova_members WHERE principal=$1 AND team=$2`, p.Subject, p.Team).Scan(&active) != nil || !active {
		return v0.Principal{}, authentication.ErrForbidden
	}
	return p, nil
}
func (a localAuth) AuthenticateSession(r *http.Request) (v0.Principal, func(context.Context) error, error) {
	p, err := a.Authenticate(r)
	return p, func(ctx context.Context) error { _, e := a.Authenticate(r.WithContext(ctx)); return e }, err
}

type source struct{ p v0.Principal }

func (s source) Principal() v0.Principal { return s.p }

type previewRuntime struct {
	mu        sync.Mutex
	claims    map[string]string
	released  map[string]bool
	authority map[string]runtime.NetworkAuthority
	templates map[string]string
}

func (b *previewRuntime) Allocate(r runtime.AllocateRequest) (runtime.Allocation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := v0.SandboxClaimBackendIdentity{Backend: "local-synthetic-no-infra-proof", WorkerID: r.ClaimID}
	b.claims[id.WorkerID] = r.ClaimID
	b.authority[id.WorkerID] = r.Network
	if b.templates == nil {
		b.templates = map[string]string{}
	}
	b.templates[id.WorkerID] = r.TemplateRef
	return runtime.Allocation{ClaimID: r.ClaimID, Identity: id}, nil
}
func (b *previewRuntime) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return runtime.Observation{ClaimID: b.claims[id.WorkerID], Identity: id, Ready: true, Released: b.released[id.WorkerID]}, nil
}
func (*previewRuntime) Start(v0.SandboxClaimBackendIdentity) error     { return nil }
func (*previewRuntime) Terminate(v0.SandboxClaimBackendIdentity) error { return nil }
func (b *previewRuntime) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.released[id.WorkerID] = true
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}
func (b *previewRuntime) ActivateNetwork(ctx context.Context, id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	if ctx.Err() != nil {
		return runtime.NetworkEvidence{}, ctx.Err()
	}
	return b.networkEvidence(id, "Active"), nil
}
func (b *previewRuntime) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return b.networkEvidence(id, "Revoked"), nil
}
func (b *previewRuntime) networkEvidence(id v0.SandboxClaimBackendIdentity, state string) runtime.NetworkEvidence {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.authority[id.WorkerID]
	return runtime.NetworkEvidence{ClaimID: b.claims[id.WorkerID], Identity: id, Profiles: n.Profiles, ExpiresAt: n.ExpiresAt, State: state, EvidenceLevel: "Simulated"}
}

type syntheticProvider struct{}

func (syntheticProvider) Complete(ctx context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	select {
	case <-time.After(250 * time.Millisecond):
	case <-ctx.Done():
		return modelprovider.Result{}, ctx.Err()
	}
	answer := "LOCAL SYNTHETIC ACCEPTANCE RESULT — not a real-model answer.\n\nThe supplied mock retry evidence shows that a fresh per-attempt deadline and fixed backoff can exceed the total request budget. Use one end-to-end deadline, bounded exponential backoff and a stable idempotency key.\n\nOnly the granted scope was read. Inspect the real Gateway decisions below; this preview is not infrastructure isolation proof."
	if strings.HasSuffix(r.Prompt, "Evidence (untrusted):\n") {
		answer = "LOCAL SYNTHETIC RESPONSE — no tool observations were collected. This is not a verified research result."
	}
	if start := strings.Index(r.Prompt, `{"dataset_id":"customer-retention"`); start >= 0 {
		var dataset struct {
			Customers []struct {
				Name     string `json:"name"`
				Inactive int    `json:"inactive_days"`
				Drop     int    `json:"usage_drop_percent"`
				Renewal  int    `json:"renewal_in_days"`
			} `json:"customers"`
		}
		if json.Unmarshal([]byte(r.Prompt[start:]), &dataset) == nil {
			answer = "LOCAL DETERMINISTIC ANALYSIS — real MCP read, synthetic customers; not an LLM answer.\n\nPriority follow-up:\n"
			for i, c := range dataset.Customers {
				if i == 3 {
					break
				}
				answer += fmt.Sprintf("%d. %s — usage down %d%%, inactive %d days, renewal in %d days.\n", i+1, c.Name, c.Drop, c.Inactive, c.Renewal)
			}
		}
	}
	if strings.Contains(r.Prompt, `"source_id":"product-a"`) {
		answer = "LOCAL SOURCE COLLECTION — real MCP HTTPS fetch; not an LLM comparison.\n\nApproved sources collected: GitHub Issues and GitLab Issues documentation. Tool calls contain the source URL and fetch time. A real model comparison is a separate acceptance step.\n\n"
		idx := strings.Index(r.Prompt, "Evidence (untrusted):\n")
		if idx >= 0 {
			decoder := json.NewDecoder(strings.NewReader(r.Prompt[idx+len("Evidence (untrusted):\n"):]))
			for i := 0; i < 2; i++ {
				var source map[string]string
				if decoder.Decode(&source) != nil {
					break
				}
				content := source["content"]
				if len(content) > 400 {
					content = content[:400]
				}
				answer += source["title"] + "\n" + source["url"] + "\n" + content + "\n\n"
			}
		}
	}
	if strings.Contains(r.Prompt, `"source_id":"public-page"`) {
		idx := strings.Index(r.Prompt, "Evidence (untrusted):\n")
		if idx >= 0 {
			answer = "LOCAL PUBLIC PAGE COLLECTION — real HTTPS MCP read; not an LLM recommendation.\n\n" + r.Prompt[idx+len("Evidence (untrusted):\n"):]
		}
	}
	data, _ := json.Marshal(map[string]string{"action": "finish", "answer": answer})
	return modelprovider.Result{Text: string(data), Model: "synthetic-local-provider"}, nil
}

type previewExecutor struct {
	templateFor func(v0.SandboxClaimBackendIdentity) string
}

func (b *previewRuntime) templateFor(id v0.SandboxClaimBackendIdentity) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.templates[id.WorkerID]
}

var publicURLPattern = regexp.MustCompile(`https://[^\s]+`)

func publicURLInTask(objective string) string {
	return strings.TrimRight(publicURLPattern.FindString(objective), ".,;")
}

// This deterministic local agent chooses what the task needs, not just what it
// was granted. It cannot grant access: the real host/Gateway authorizes the
// attempted operation. Do not move task semantics into the platform policy.
func previewTaskTool(t workerprotocol.Task) (workerprotocol.Tool, bool) {
	if publicURLInTask(t.Objective) != "" {
		return workerprotocol.Tool{Operation: "web.read", ResourceScope: "web:public-pages", InputMode: "text", MaxBytes: 1024}, true
	}
	if strings.Contains(strings.ToLower(t.Objective), "customer") {
		return workerprotocol.Tool{Operation: "crm.read", ResourceScope: "crm:customer-retention", AllowedValues: []string{"customer-retention"}}, true
	}
	if len(t.Tools) > 0 {
		return t.Tools[0], true
	}
	return workerprotocol.Tool{}, false
}

func (e previewExecutor) Execute(ctx context.Context, identity v0.SandboxClaimBackendIdentity, t workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	evidenceText := ""
	tool, needed := previewTaskTool(t)
	if e.templateFor != nil {
		// Only the local Researcher fixture interprets web/customer task intent.
		// Existing coding fixtures retain their original catalog-first behavior.
		name := e.templateFor(identity)
		if name != "researcher" && name != "researcher-v2" {
			needed = len(t.Tools) > 0
			if needed {
				tool = t.Tools[0]
			}
		}
	}
	if needed {
		if len(tool.AllowedValues) == 0 && tool.InputMode != "text" {
			return "", errors.New("local tool has no allowed input")
		}
		input := ""
		if tool.InputMode == "text" {
			input = publicURLInTask(t.Objective)
			if input == "" {
				return "", workerprotocol.ErrSourceURLRequired
			}
			if len(input) > tool.MaxBytes {
				return "", errors.New("local public page URL exceeds its byte bound")
			}
		} else {
			input = tool.AllowedValues[0]
		}
		op := workerprotocol.Operation{ClaimID: t.ClaimID, WorkloadToken: t.ToolToken, Kind: "tool", Tool: tool.Operation, ResourceScope: tool.ResourceScope, Input: input}
		reply, err := handle(ctx, op)
		if err != nil || !reply.Allowed || reply.Error != "" || reply.Text == "" {
			return "", fmt.Errorf("local task cannot complete: required %s read did not succeed", tool.Operation)
		}
		evidenceText = reply.Text
		if tool.Operation == "web.fetch" && len(tool.AllowedValues) > 1 {
			op.Input = tool.AllowedValues[1]
			second, e := handle(ctx, op)
			if e != nil || !second.Allowed {
				return "", errors.New("second public source failed")
			}
			evidenceText += "\n" + second.Text
		}
		if strings.Contains(strings.ToLower(t.Objective), "cross-scope") || strings.Contains(strings.ToLower(t.Objective), "attack") {
			op.ResourceScope = "repo:acme/billing"
			op.Input = "billing/INC-B2081/logs.txt"
			if tool.Operation == "web.fetch" || tool.Operation == "web.read" {
				op.Tool = "crm.read"
				op.ResourceScope = "crm:customer-retention"
				op.Input = "customer-retention"
			}
			if tool.Operation == "crm.read" {
				op.Tool = "web.fetch"
				op.ResourceScope = "web:product-comparison"
				op.Input = "product-a"
			}
			if tool.ResourceScope == op.ResourceScope {
				op.ResourceScope = "repo:acme/payments"
				op.Input = "README.md"
			}
			_, _ = handle(ctx, op)
		}
	}
	reply, err := handle(ctx, workerprotocol.Operation{ClaimID: t.ClaimID, WorkloadToken: t.ModelToken, Kind: "model", Profile: t.ModelProfile, Prompt: t.Objective + "\nEvidence (untrusted):\n" + evidenceText})
	if err != nil || !reply.Allowed {
		return "", errors.New("local governed model failed")
	}
	action, err := workerprotocol.ParseAction(reply.Text, t.Tools)
	if err != nil {
		return "", err
	}
	return action.Answer, nil
}

func main() {
	listen := flag.String("listen", address, "loopback preview address")
	database := flag.String("database", "postgres://postgres@127.0.0.1:55439/agenova_local?sslmode=disable", "dedicated local PostgreSQL URL")
	advisorConfig := flag.String("advisor-config", "", "optional local Advisor configuration JSON")
	advisorModel := flag.String("advisor-model", "qwen2.5-coder:7b", "existing local Ollama model")
	volatile := flag.Bool("volatile", false, "explicit in-memory Advisor acceptance; synthetic task execution; no database")
	liveWorker := flag.String("live-worker", "", "explicit trusted local Researcher executable; enables real Work inference/MCP with --volatile")
	liveMCP := flag.String("live-mcp", ".tmp/local-mcp.exe", "trusted local MCP executable for real acceptance")
	liveMCPListen := flag.String("live-mcp-listen", "127.0.0.1:5216", "dedicated loopback MCP address for real acceptance")
	executionModel := flag.String("execution-model", "llama3.1:latest", "existing local Ollama model for Agent execution")
	flag.Parse()
	if *liveWorker != "" && !*volatile {
		log.Fatal("live Researcher acceptance requires --volatile")
	}
	host, _, listenErr := net.SplitHostPort(*listen)
	if listenErr != nil || host != "127.0.0.1" || !strings.HasPrefix(*database, "postgres://postgres@127.0.0.1:") {
		log.Fatal("local preview requires literal loopback endpoints")
	}
	address = *listen
	planner, plannerErr := localAdvisor(*advisorConfig, *advisorModel)
	if plannerErr != nil {
		log.Fatal("local Advisor configuration failed: ", plannerErr)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *volatile {
		var live *liveConfig
		if *liveWorker != "" {
			live = &liveConfig{Worker: *liveWorker, MCP: *liveMCP, MCPListen: *liveMCPListen, Model: *executionModel}
		}
		if e := serveVolatile(ctx, planner, live); e != nil {
			log.Fatal(e)
		}
		return
	}
	db, err := sql.Open("postgres", *database)
	if err != nil {
		log.Fatal("local database unavailable")
	}
	defer db.Close()
	if identitydb.Migrate(ctx, db) != nil {
		log.Fatal("local migration failed")
	}
	store := identitydb.Store{DB: db}
	for _, name := range []string{"owner", "member", "viewer"} {
		if err = store.PutMember(ctx, "local-preview-bootstrap", identitydb.Member{Issuer: "https://synthetic.local.invalid", Subject: name, Principal: "local-" + name, Team: "team-a", Active: true}); err != nil {
			log.Fatal("local seed failed")
		}
	}
	if _, err = db.Exec(`UPDATE agenova_members SET account_role='owner' WHERE principal='local-owner';INSERT INTO agenova_projects VALUES('team-a','payments'),('team-a','billing') ON CONFLICT DO NOTHING;INSERT INTO agenova_project_grants VALUES('local-member','team-a','payments','member'),('local-member','team-a','billing','member'),('local-viewer','team-a','payments','viewer') ON CONFLICT DO NOTHING;INSERT INTO agenova_scoped_roles VALUES('local-owner','team-a','governance-admin','account','team-a') ON CONFLICT DO NOTHING`); err != nil {
		log.Fatal("local grants failed")
	}
	principal := v0.Principal{Subject: "local-owner", Team: "team-a", AuthenticationContext: "oidc"}
	if err = seedResearch(ctx, store, principal); err != nil {
		log.Fatal("research project seed failed")
	}
	if planner != nil {
		if e := seedAdvisor(ctx, store, principal); e != nil {
			log.Fatal("local Advisor template seed failed: ", e)
		}
	}
	fallback := registration.NewMemoryStore()
	for _, name := range []string{"engineer", "incident-reader"} {
		t := app.ReferenceTemplate()
		t.Metadata.Name = name
		t.Spec.Artifact.Image = "example.invalid/agenova/local-synthetic:acceptance"
		t.Spec.CapabilityCeiling.Tools = []string{"git.read"}
		t.Spec.CapabilityCeiling.ResourceScopes = []string{"repo:acme/payments", "repo:acme/billing"}
		fallback.PutTemplate(t)
		data, _ := json.Marshal(t)
		for _, project := range []string{"payments", "billing"} {
			if _, err = store.ApplyGovernance(ctx, principal, project, "template", data, app.ValidateControlledTemplate); err != nil {
				log.Fatal("local template seed failed")
			}
		}
	}
	for _, project := range []string{"payments", "billing"} {
		var count int
		if db.QueryRow(`SELECT count(*) FROM agenova_project_policy_active WHERE team='team-a' AND project=$1`, project).Scan(&count) != nil {
			log.Fatal("local registry unavailable")
		}
		if count == 0 {
			doc := fmt.Sprintf(`{"apiVersion":"agenova.io/v1alpha1","kind":"PolicyBundle","metadata":{"name":"local-work-boundary","version":"1"},"spec":{"rules":[{"team":"team-a","project":"%s","templateRef":"engineer","action":"claim.create"},{"team":"team-a","project":"%s","templateRef":"incident-reader","action":"claim.create"}]}}`, project, project)
			if _, err = store.ApplyGovernance(ctx, principal, project, "policy", []byte(doc), nil); err != nil {
				log.Fatal("local policy seed failed")
			}
		}
	}
	durable, err := workstore.Acquire(ctx, db)
	if err != nil {
		log.Fatal("another preview writer is running")
	}
	defer durable.Close()
	mcpProcess, err := startMCP()
	if err != nil {
		log.Fatal("local MCP start failed; build .tmp/local-mcp.exe first")
	}
	defer func() { mcpProcess.Process.Kill(); mcpProcess.Wait() }()
	toolSet, err := researchTools()
	if err != nil {
		log.Fatal("real MCP catalog failed")
	}
	registry := func(p v0.Principal, project string) identitydb.ProjectRegistry {
		return identitydb.ProjectRegistry{Store: store, Team: p.Team, Project: project, Fallback: fallback}
	}
	base := func() (console.Setup, error) {
		return console.Setup{Installation: console.InstallationIdentity{Kind: "installed", Platform: "LOCAL ACCEPTANCE · synthetic identity/runtime/model · real Go API/PostgreSQL", Revision: "sha256:" + strings.Repeat("a", 64)}, Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "synthetic", "model": "synthetic", "tool": "mock", "memory": "notConnected"}, Observability: &console.ObservabilityInfo{Ingestion: "notVerified"}}, nil
	}
	setup := func(ctx context.Context, p v0.Principal, project, template string) (console.Setup, error) {
		view, _ := base()
		view.Principal = p
		view.Capabilities["tool"] = "MCP transport · synthetic CRM / live public sources"
		view.Capabilities["web.read"] = "publicHTTPS"
		view.PlatformInfo = localPlatformInfo()
		console.ScopePlatform(&view, store.IsOperator(ctx, p))
		projects, projectErr := store.ConfigurationProjects(ctx, p)
		if projectErr != nil {
			return view, projectErr
		}
		view.Projects = projects
		if project == "" && len(view.Projects) > 0 {
			project = view.Projects[0]
		}
		ok := false
		submission := []string{}
		for _, name := range view.Projects {
			ok = ok || name == project
			if store.Authorize(ctx, p, identitydb.SubmitWork, name) == nil {
				submission = append(submission, name)
			}
		}
		view.SubmissionProjects = &submission
		if !ok {
			return view, authentication.ErrForbidden
		}
		view.Project = project
		org, e := store.Organization(ctx, p)
		if e != nil {
			return view, e
		}
		view.Organization = &console.OrganizationInfo{ID: org.ID, DisplayName: org.DisplayName, Description: org.Description}
		r := registry(p, project)
		view.Policy, e = r.ActivePolicy()
		if e != nil {
			return view, e
		}
		view.Templates, e = r.TemplateNames(nil)
		if e != nil {
			return view, e
		}
		if template == "" && len(view.Templates) > 0 {
			template = view.Templates[0]
		}
		if template != "" {
			view.Template, e = r.Template(template)
		}
		return view, e
	}
	backend := &previewRuntime{claims: map[string]string{}, released: map[string]bool{}, authority: map[string]runtime.NetworkAuthority{}}
	service, err := console.NewServiceWithOptions(backend, previewExecutor{templateFor: backend.templateFor}, syntheticProvider{}, "", console.Options{Advisor: planner, ToolBackend: toolSet, Durable: durable, Access: store, Setup: base, SetupFor: setup, Configure: func(t *v0.AgentTemplate) (app.ResolvedLaunch, error) {
		return app.ResolvedLaunch{ProfileRef: "standard-isolated", TemplateRef: t.Metadata.Name}, nil
	}, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		req, parseErr := v0.ParseClaimRequestJSON(data)
		if parseErr != nil {
			return app.PreparedAssignment{}, parseErr
		}
		r := registry(p, req.Spec.ProjectRef)
		bundle, e := r.ActivePolicy()
		if e != nil {
			return app.PreparedAssignment{}, e
		}
		loader := &policy.Loader{}
		if e = loader.Load(bundle); e != nil {
			return app.PreparedAssignment{}, e
		}
		return app.PrepareAssignment(data, source{p}, loader, r)
	}})
	if err != nil {
		log.Fatal("local service composition failed")
	}
	defer service.Close()
	auth := localAuth{store}
	api, err := console.AuthenticatedHandler(service, auth)
	if err != nil {
		log.Fatal("local API unavailable")
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/api/governance", management.Governance(store, auth, app.ValidateControlledTemplate))
	mux.Handle("/api/organization", management.Organization(store, auth))
	mux.Handle("/api/members", invitations.MemberHandler(store, auth, "local-acceptance"))
	mux.Handle("/admin", invitations.Page(invitations.Service{Store: store}, auth))
	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		p, e := auth.Authenticate(r)
		if e != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"subject": p.Subject, "team": p.Team})
	})
	mux.HandleFunc("/api/access-requests", func(w http.ResponseWriter, r *http.Request) {
		p, e := auth.Authenticate(r)
		if e != nil || p.Subject != "local-owner" {
			http.Error(w, "Forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><meta name="color-scheme" content="only light"><title>Local acceptance sign-in</title><body style="font:16px system-ui;background:#faf9fc;color:#302840;max-width:650px;margin:90px auto"><h1>Local product acceptance</h1><p>This is not Google sign-in. Identity, runtime and model responses are synthetic. Authorization, Policy activation, PostgreSQL and the Go MCP service are real. CRM records are synthetic; public sources are fetched live.</p><form action="/local-session" method="post"><label>Test identity <select name="identity"><option>owner</option><option>member</option><option>viewer</option></select></label><button>Enter local workspace</button></form><p>No AWS or provider credentials are used.</p></body></html>`)
	})
	mux.HandleFunc("/local-session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Use POST", 405)
			return
		}
		name := r.FormValue("identity")
		if name != "owner" && name != "member" && name != "viewer" {
			http.Error(w, "Unknown synthetic identity", 400)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "agenova_local_acceptance", Value: name, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/?mode=connected&project=payments", 303)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Use POST", 405)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "agenova_local_acceptance", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/", 303)
	})
	directory, _ := filepath.Abs("ui/dist")
	public := landing.New(directory)
	assets := http.FileServer(http.Dir(directory))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/landing/") {
			public.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			assets.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if _, e := auth.Authenticate(r); e != nil {
			public.ServeHTTP(w, r)
			return
		}
		data, e := os.ReadFile(filepath.Join(directory, "index.html"))
		if e != nil {
			http.Error(w, "Build ui first", 503)
			return
		}
		p, _ := auth.Authenticate(r)
		navigation := "member"
		if store.IsOperator(r.Context(), p) {
			navigation = "admin"
		}
		html := strings.Replace(string(data), "<head>", `<head><meta name="agenova-account-navigation" content="`+navigation+`">`, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != address {
			http.Error(w, "Loopback preview only", 403)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+address {
				http.Error(w, "Origin rejected", 403)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
	log.Printf("Local acceptance only: http://%s — no AWS, synthetic execution", address)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("local preview stopped: %v", err)
	}
}
