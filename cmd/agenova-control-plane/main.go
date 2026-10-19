// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// agenova-control-plane is the minimum internal reference-install process. It
// intentionally exposes only readiness and secret-free installation status.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"github.com/wunderforge/agenova/internal/workstore"
)

type status struct {
	Platform         string `json:"platform"`
	Revision         string `json:"revision"`
	InitialPolicyRef string `json:"initialPolicyRef"`
	State            string `json:"state"`
	ReadinessScope   string `json:"readinessScope"`
	ProviderHealth   string `json:"providerHealth"`
}

func main() {
	if len(os.Args) > 1 {
		if err := localCommand(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := serve(); err != nil {
		log.Fatal(err)
	}
}

func serve() error {
	configPath, err := memberConfigPath()
	if err != nil {
		return err
	}
	var members *memberDependencies
	if configPath != "" {
		members, err = openMemberDependencies(context.Background(), configPath)
		if err != nil {
			return err
		}
		defer members.db.Close()
	}
	var access console.AccessAuthorizer
	var durable console.DurableWork
	if members != nil {
		access = members.store
		writer, err := workstore.Acquire(context.Background(), members.db)
		if err != nil {
			return fmt.Errorf("acquire durable Work writer: %w", err)
		}
		defer writer.Close()
		durable = writer
	}
	configured, err := configuredServiceWithDurable("/etc/agenova/effective-platform.json", access, durable)
	if err != nil {
		return err
	}
	defer configured.Close()
	var workHandler http.Handler = console.Handler(configured)
	statusHandler := handler()
	if members != nil {
		workHandler, err = members.handler(configured)
		if err != nil {
			return err
		}
		statusHandler = memberStatusHandler() // Kubelet readiness stays reachable; metadata is loopback-only.
	}
	// The authenticated listener is behind a same-host TLS reverse proxy. Neither
	// the reference API nor an unauthenticated private-command bypass is started.
	private := &http.Server{Addr: "127.0.0.1:8081", Handler: workHandler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 3 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	listener, err := net.Listen("tcp", private.Addr)
	if err != nil {
		return fmt.Errorf("start private Work service: %w", err)
	}
	defer listener.Close()
	server := &http.Server{Addr: ":8080", Handler: statusHandler, ReadHeaderTimeout: 5 * time.Second}
	statusListener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("start private status service: %w", err)
	}
	defer statusListener.Close()
	failures := make(chan error, 2)
	go func() { failures <- private.Serve(listener) }()
	go func() { failures <- server.Serve(statusListener) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("agenova control plane listening; member authentication enabled=%t", members != nil)
	select {
	case err = <-failures:
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = private.Shutdown(shutdown)
	_ = server.Shutdown(shutdown)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func memberConfigPath() (string, error) {
	path, present := os.LookupEnv("AGENOVA_MEMBER_CONFIG")
	mode := os.Getenv("AGENOVA_AUTH_MODE")
	switch mode {
	case "", "reference", "member":
	default:
		return "", errors.New("unknown authentication mode")
	}
	if present && strings.TrimSpace(path) == "" {
		return "", errors.New("member configuration path is empty")
	}
	if mode == "member" && !present {
		return "", errors.New("member configuration is required")
	}
	if mode == "reference" && present {
		return "", errors.New("reference and member authentication cannot be combined")
	}
	return path, nil
}

func configuredService(path string) (*console.Service, error) {
	return configuredServiceWithAccess(path, nil)
}

func configuredServiceWithAccess(path string, access console.AccessAuthorizer) (*console.Service, error) {
	return configuredServiceWithDurable(path, access, nil)
}
func configuredServiceWithDurable(path string, access console.AccessAuthorizer, durable console.DurableWork) (*console.Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read installed Platform: %w", err)
	}
	var resolved platform.ResolvedPlatform
	if err := json.Unmarshal(data, &resolved); err != nil {
		return nil, fmt.Errorf("decode installed Platform: %w", err)
	}
	if resolved.Revision == "" {
		return nil, fmt.Errorf("installed Platform revision is missing")
	}
	registry, err := bundled.NewRegistry()
	if err != nil {
		return nil, err
	}
	_, namespace, err := app.DeploymentCoordinates(&resolved)
	if err != nil {
		return nil, err
	}
	// Token references resolve in the install namespace only, on each call.
	tools, err := buildInstalledTools(&resolved, registry, kubectlSecretReader{namespace: namespace})
	if err != nil {
		return nil, err
	}
	// "configured" reports installed configuration, not a successful call.
	toolStatus := "mock"
	if tools != nil {
		toolStatus = "configured"
	}
	compatibleWorkerImage := strings.TrimSpace(os.Getenv("AGENOVA_ALLOWED_WORKER_IMAGE"))
	if compatibleWorkerImage == "" {
		return nil, fmt.Errorf("installed runtime compatible worker image is missing")
	}
	if err := setInClusterKubeconfig(); err != nil {
		return nil, err
	}
	store := registration.KubernetesStore{Namespace: namespace}
	runtimeNamespace := ""
	adapterIDs := map[string]string{}
	for _, adapter := range resolved.Adapters {
		adapterIDs[adapter.Name] = adapter.ID
	}
	runtimeCount := 0
	for _, instance := range resolved.Instances {
		if instance.Category != platform.CapabilityRuntime {
			continue
		}
		runtimeCount++
		if adapterIDs[instance.AdapterRef] != bundled.AgentSandboxRuntimeID {
			return nil, fmt.Errorf("reference Control Plane does not support the selected runtime adapter")
		}
		connection, _ := instance.Config["connection"].(map[string]any)
		if connection["mode"] != "in-cluster" {
			return nil, fmt.Errorf("unsupported runtime connection")
		}
		runtimeNamespace, _ = connection["namespace"].(string)
	}
	if runtimeCount != 1 || runtimeNamespace == "" || runtimeNamespace == "default" {
		return nil, fmt.Errorf("installed runtime namespace is invalid")
	}
	if runtimeNamespace != namespace {
		return nil, fmt.Errorf("reference runtime namespace must match the installed Control Plane namespace")
	}
	adapter := agentsandbox.NewControlled("", runtimeNamespace)
	modelProfiles := map[string]string{}
	for _, profile := range resolved.Profiles {
		if profile.Capability == platform.CapabilityModel {
			modelProfiles[profile.Name], _ = profile.Config["model"].(string)
		}
	}
	runtimeProfiles := map[string]bool{}
	for _, profile := range resolved.Profiles {
		if profile.Capability == platform.CapabilityRuntime {
			runtimeProfiles[profile.Name] = true
		}
	}
	// The service sends each Work's schema with every model request; this one is only the fallback.
	provider, err := bundled.NewInstalledModelClient(context.Background(), &resolved, []byte(workerprotocol.FinishSchema))
	if err != nil {
		return nil, err
	}
	preset := app.ReferencePrincipalTeamA
	var setupPrincipal v0.Principal
	if access == nil {
		principal, err := app.NewReferencePrincipalSource(preset)
		if err != nil {
			return nil, err
		}
		setupPrincipal = principal.Principal()
	} else {
		preset = "" // Authenticated mode cannot select a fixed reference identity.
	}
	options := console.Options{
		ToolBackend: tools,
		Setup: func() (console.Setup, error) {
			bundle, err := store.ActivePolicy()
			if err != nil {
				return console.Setup{}, err
			}
			templates, err := store.Templates()
			if err != nil {
				return console.Setup{}, err
			}
			if len(templates) != 1 {
				return console.Setup{}, fmt.Errorf("reference Portal requires exactly one registered AgentTemplate")
			}
			return console.Setup{Principal: setupPrincipal, Template: templates[0], Policy: bundle,
				Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "configured", "model": "configured", "tool": toolStatus, "memory": "notConnected"},
				Installation: console.InstallationIdentity{Kind: "installed", Platform: resolved.PlatformName, Revision: resolved.Revision}}, nil
		},
		PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
			var source interface {
				ActivePolicy() (policy.PolicyBundle, error)
				Template(string) (*v0.AgentTemplate, error)
				Lookup(string) (*v0.AgentTemplate, error)
			} = store
			if memberStore, ok := access.(identitydb.Store); ok {
				request, err := v0.ParseClaimRequestJSON(data)
				if err != nil {
					return app.PreparedAssignment{}, err
				}
				source = identitydb.ProjectRegistry{Store: memberStore, Team: p.Team, Project: request.Spec.ProjectRef, Fallback: store}
			}
			bundle, err := source.ActivePolicy()
			if err != nil {
				return app.PreparedAssignment{}, &console.SubmissionError{Code: "active_policy_unavailable", Message: "Active PolicyBundle is unavailable; register or repair the active policy.", Cause: err}
			}
			loader := &policy.Loader{}
			if err := loader.Load(bundle); err != nil {
				return app.PreparedAssignment{}, &console.SubmissionError{Code: "active_policy_invalid", Message: "Active PolicyBundle is invalid; register a valid policy version.", Cause: err}
			}
			prepared, err := app.PrepareAssignment(data, requestPrincipal{p}, loader, source)
			if err != nil {
				request, parseErr := v0.ParseClaimRequestJSON(data)
				if parseErr == nil {
					if _, lookupErr := source.Template(request.Spec.TemplateRef); lookupErr != nil {
						return prepared, &console.SubmissionError{Code: "agent_template_unavailable", Message: "AgentTemplate is unavailable; register the requested template.", Cause: err}
					}
				}
				return prepared, &console.SubmissionError{Code: "assignment_unavailable", Message: "Assignment could not be resolved; check the registered template and active policy.", Cause: err}
			}
			if prepared.Issued != nil && prepared.Issued.Claim != nil {
				if err := validateInstalledAuthority(prepared.Issued.EffectiveAuthority, modelProfiles, runtimeProfiles, tools); err != nil {
					return app.PreparedAssignment{}, err
				}
			}
			return prepared, nil
		},
		Configure: func(template *v0.AgentTemplate) (app.ResolvedLaunch, error) {
			if template == nil || template.Spec.Artifact == nil || template.Spec.Entrypoint == nil {
				return app.ResolvedLaunch{}, fmt.Errorf("registered template is incomplete")
			}
			if err := requireCompatibleWorkerImage(template.Spec.Artifact.Image, compatibleWorkerImage); err != nil {
				return app.ResolvedLaunch{}, err
			}
			if len(template.Spec.Entrypoint.Command) != 2 || template.Spec.Entrypoint.Command[0] != "/agenova-workerctl" || template.Spec.Entrypoint.Command[1] != "serve" {
				return app.ResolvedLaunch{}, fmt.Errorf("reference runtime requires a controlled-worker entrypoint")
			}
			name := template.Metadata.Name
			if err := adapter.AddTemplate(v0.AgentSandboxTemplate{Metadata: v0.ObjectMeta{Name: name}, Spec: v0.AgentSandboxTemplateSpec{Image: template.Spec.Artifact.Image, Command: template.Spec.Entrypoint.Command}}); err != nil {
				return app.ResolvedLaunch{}, err
			}
			if err := adapter.AddWarmPool(v0.SandboxWarmPool{Metadata: v0.ObjectMeta{Name: "pool-" + name}, Spec: v0.SandboxWarmPoolSpec{TemplateRef: name, Replicas: 1}}); err != nil {
				return app.ResolvedLaunch{}, err
			}
			return app.ResolvedLaunch{TemplateRef: name}, nil
		},
	}
	if memberStore, ok := access.(identitydb.Store); ok {
		options.SetupFor = func(ctx context.Context, p v0.Principal, project, template string) (console.Setup, error) {
			view, err := options.Setup()
			if err != nil {
				return view, err
			}
			projects, err := memberStore.ConfigurationProjects(ctx, p)
			if err != nil {
				return console.Setup{}, err
			}
			view.Principal = p
			view.Projects = projects
			if project == "" && len(projects) > 0 {
				project = projects[0]
			}
			if project == "" {
				view.Template = nil
				view.Policy = policy.PolicyBundle{ID: "no-project-access", Version: "1", Rules: []policy.Rule{}}
				return view, nil
			}
			if !slices.Contains(projects, project) {
				return console.Setup{}, fmt.Errorf("project configuration access denied")
			}
			registry := identitydb.ProjectRegistry{Store: memberStore, Team: p.Team, Project: project, Fallback: store}
			view.Project = project
			view.Policy, err = registry.ActivePolicy()
			if err != nil {
				return console.Setup{}, err
			}
			visible := []policy.Rule{}
			for _, rule := range view.Policy.Rules {
				if rule.Team == p.Team && rule.Project == project {
					visible = append(visible, rule)
				}
			}
			view.Policy.Rules = visible
			defaults := []string{}
			if view.Template != nil {
				defaults = append(defaults, view.Template.Metadata.Name)
			}
			view.Templates, err = registry.TemplateNames(defaults)
			if err != nil {
				return console.Setup{}, err
			}
			if template == "" && len(view.Templates) > 0 {
				template = view.Templates[0]
			}
			if !slices.Contains(view.Templates, template) {
				return console.Setup{}, fmt.Errorf("template unavailable")
			}
			view.Template, err = registry.Template(template)
			return view, err
		}
	}
	options.Access = access
	options.Durable = durable
	if durable != nil {
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		_, recoveryErr := console.RecoverDurableWork(recoveryCtx, durable, adapter)
		cancel()
		if recoveryErr != nil {
			return nil, fmt.Errorf("durable Work recovery incomplete; reconcile the recorded allocation before admitting new Work: %w", recoveryErr)
		}
	}
	if access == nil {
		prepare := options.PrepareFor
		options.PrepareFor = nil
		options.Prepare = func(data []byte) (app.PreparedAssignment, error) { return prepare(data, setupPrincipal) }
	}
	return console.NewServiceWithOptions(adapter, adapter, provider, preset, options)
}

type requestPrincipal struct{ value v0.Principal }

func (p requestPrincipal) Principal() v0.Principal { return p.value }

// This reference composition currently provides only a synthetic git.read
// adapter and no Memory Interface. Check the issued effective grant before
// the console journals a claim or configures a worker: a broader template
// ceiling must never be mistaken for an implemented gateway capability.
func validateInstalledAuthority(authority *v0.EffectiveAuthority, models map[string]string, runtimeProfiles map[string]bool, tools *toolbackend.Set) error {
	if authority == nil {
		return &console.SubmissionError{Code: "authority_missing", Message: "Issued effective authority is missing; inspect policy and template configuration."}
	}
	if _, ok := models[authority.ModelProfile]; !ok {
		return &console.SubmissionError{Code: "model_profile_unavailable", Message: "Granted model profile is not installed; update the Platform model configuration."}
	}
	if !runtimeProfiles[authority.Runtime.ProfileRef] {
		return &console.SubmissionError{Code: "runtime_profile_unavailable", Message: "Granted runtime profile is not installed; update the Platform runtime configuration."}
	}
	for _, tool := range authority.Tools {
		if (tools == nil && tool != "git.read") || (tools != nil && !tools.Catalog().Supports(tool)) {
			return &console.SubmissionError{Code: "tool_unsupported", Message: "Granted tool is not supported by the installed Tool Gateway; narrow the template or install a compatible gateway."}
		}
		// An installed operation with no route for any granted scope would leave
		// the Work's catalog empty, and the Work would run without the tool.
		if tools != nil && len(tools.Catalog().Intersect([]string{tool}, authority.ResourceScopes).Entries()) == 0 {
			return &console.SubmissionError{Code: "tool_route_unavailable", Message: "Granted tool has no installed route for the granted resource scopes; narrow the template or configure a tool profile for those scopes."}
		}
	}
	if len(authority.MemoryScopes) != 0 {
		return &console.SubmissionError{Code: "memory_unsupported", Message: "Granted memory scope is not supported by the installed Memory Interface; narrow the template or install a compatible interface."}
	}
	return nil
}

// The installed runtime declares one worker artifact implementing the
// controlled-worker protocol. A template cannot redirect that protocol to an
// arbitrary image merely by declaring the expected entrypoint command.
func requireCompatibleWorkerImage(image, allowed string) error {
	if allowed == "" {
		return fmt.Errorf("installed runtime compatible worker image is missing")
	}
	if image != allowed {
		return fmt.Errorf("registered template image does not match the installed runtime compatible worker image")
	}
	return nil
}

func localCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing private command")
	}
	if args[0] == "identity-bootstrap" {
		return bootstrapOwnerCommand(args)
	}
	if args[0] == "identity-migrate" {
		if len(args) != 3 || args[1] != "--database-url-file" {
			return errors.New("identity-migrate requires --database-url-file <operator credential file>")
		}
		db, err := openMemberDatabase(context.Background(), args[2])
		if err != nil {
			return err
		}
		defer db.Close()
		return identitydb.Migrate(context.Background(), db)
	}
	client := &http.Client{Timeout: 4 * time.Minute}
	var method, path string
	var body io.Reader
	switch args[0] {
	case "submit":
		if len(args) != 1 {
			return fmt.Errorf("submit accepts no arguments")
		}
		data, err := io.ReadAll(io.LimitReader(os.Stdin, (128<<10)+1))
		if err != nil || len(data) > 128<<10 {
			return fmt.Errorf("submission is too large")
		}
		method, path, body = http.MethodPost, "/api/requests", bytes.NewReader(data)
	case "evidence":
		if len(args) != 2 || args[1] == "" || strings.ContainsAny(args[1], "/\\?&#") {
			return fmt.Errorf("provide one bounded request reference")
		}
		method, path = http.MethodGet, "/api/requests/"+args[1]+"/evidence"
	default:
		return fmt.Errorf("unknown private command")
	}
	req, err := http.NewRequestWithContext(context.Background(), method, "http://127.0.0.1:8081"+path, body)
	if err != nil {
		return err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("installed Work service is unavailable")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Work service rejected the request (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	_, err = os.Stdout.Write(data)
	return err
}

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ready\n"))
	})
	mux.HandleFunc("GET /v1/status", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(status{Platform: os.Getenv("AGENOVA_PLATFORM_NAME"), Revision: os.Getenv("AGENOVA_PLATFORM_REVISION"), InitialPolicyRef: os.Getenv("AGENOVA_POLICY_REF"), State: "installation-ready", ReadinessScope: "installation-components", ProviderHealth: "not-checked"})
	})
	return mux
}

// Existing Kubernetes probes use the Pod IP. Expose only the bounded readiness
// response there; detailed installation metadata still requires local access.
func memberStatusHandler() http.Handler {
	next := handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || !net.ParseIP(host).IsLoopback() {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
