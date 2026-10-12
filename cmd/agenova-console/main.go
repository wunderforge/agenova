// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// agenova-console is an opt-in loopback-only mid-term demo composition.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/demoactions"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	kube := flag.String("kube-context", "", "explicit local kind context")
	namespace := flag.String("namespace", "", "explicit non-default, operator-created demo namespace")
	listen := flag.String("listen", "127.0.0.1:8088", "literal loopback listen address")
	endpoint := flag.String("model-endpoint", "http://127.0.0.1:11434/v1", "trusted host-side OpenAI-compatible API base URL")
	model := flag.String("provider-model", "llama3.1:latest", "trusted model behind approved-coding-model")
	principal := flag.String("principal", "team-a", "fixed operator identity, never set by Work or UI")
	policyFile := flag.String("policy-file", "", "operator-selected policy document for the isolated real-action demo")
	templateFile := flag.String("template-file", "", "operator-selected shared AgentTemplate for the isolated real-action demo")
	toolRepo := flag.String("tool-repo", "", "host-side checkout of the dedicated synthetic payment repository")
	flag.Parse()
	if strings.TrimSpace(*kube) == "" || strings.TrimSpace(*namespace) == "" || *namespace == "default" {
		return fmt.Errorf("explicit kube-context and non-default namespace are required")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("console must listen on a literal loopback address; no public authentication is provided")
	}
	modelProfile := "approved-coding-model"
	if *policyFile != "" || *templateFile != "" || *toolRepo != "" {
		modelProfile = "coding-standard"
	}
	provider, err := modelprovider.New(modelprovider.Config{Endpoint: *endpoint, Models: map[string]string{modelProfile: *model}, MaxTokens: 2048, OutputSchema: []byte(workerprotocol.ActionSchema), Timeout: 2 * time.Minute})
	if err != nil {
		return err
	}
	adapter := agentsandbox.NewControlled(*kube, *namespace)
	if err = adapter.AddTemplate(v0.AgentSandboxTemplate{Metadata: v0.ObjectMeta{Name: app.ReferenceRuntimeTemplateRef}, Spec: v0.AgentSandboxTemplateSpec{Image: "agenova-testworker:kind", Command: []string{"/agenova-workerctl", "serve"}}}); err != nil {
		return fmt.Errorf("runtime template setup failed")
	}
	if err = adapter.AddWarmPool(v0.SandboxWarmPool{Metadata: v0.ObjectMeta{Name: "reference-engineer-pool"}, Spec: v0.SandboxWarmPoolSpec{TemplateRef: app.ReferenceRuntimeTemplateRef, Replicas: 1}}); err != nil {
		return fmt.Errorf("runtime pool setup failed")
	}
	options, err := roleDemoOptions(app.ReferencePrincipalPreset(*principal), *policyFile, *templateFile, *toolRepo)
	if err != nil {
		return err
	}
	service, err := console.NewServiceWithOptions(adapter, adapter, provider, app.ReferencePrincipalPreset(*principal), options)
	if err != nil {
		return err
	}
	defer service.Close()
	handler := console.Handler(service)
	server := &http.Server{Addr: *listen, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHost := r.Host
		if host, _, err := net.SplitHostPort(requestHost); err == nil {
			requestHost = host
		}
		if ip := net.ParseIP(requestHost); ip == nil || !ip.IsLoopback() {
			http.Error(w, "Loopback console host required.", http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	})}
	ctx, stop := consoleSignalContext(context.Background())
	defer stop()
	go func() {
		<-ctx.Done()
		bounded, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(bounded)
	}()
	fmt.Printf("Internal demo listening at http://%s; fixed operator principal=%s; real host tools=%t. Memory is not connected.\n", *listen, *principal, options.Tools != nil)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("console server failed")
	}
	return nil
}

type fixedTemplate struct{ document []byte }

func (s fixedTemplate) Lookup(name string) (*v0.AgentTemplate, error) {
	template, err := v0.ParseAgentTemplateYAML(s.document)
	if err != nil || template.Metadata.Name != name {
		return nil, fmt.Errorf("registered demo template %q unavailable", name)
	}
	return template, nil
}

func roleDemoOptions(preset app.ReferencePrincipalPreset, policyFile, templateFile, toolRepo string) (console.Options, error) {
	if policyFile == "" && templateFile == "" && toolRepo == "" {
		return console.Options{}, nil
	}
	if policyFile == "" || templateFile == "" || toolRepo == "" {
		return console.Options{}, fmt.Errorf("policy-file, template-file, and tool-repo are required together")
	}
	if preset != app.ReferencePrincipalPreset("payments-developer") && preset != app.ReferencePrincipalPreset("payments-sre") {
		return console.Options{}, fmt.Errorf("real-action demo requires an operator-selected payments principal")
	}
	policyData, err := os.ReadFile(policyFile)
	if err != nil {
		return console.Options{}, fmt.Errorf("demo policy file unavailable")
	}
	bundle, err := policy.ParseDocumentYAML(policyData)
	if err != nil {
		return console.Options{}, fmt.Errorf("demo policy document invalid: %w", err)
	}
	loader := &policy.Loader{}
	if err := loader.Load(bundle); err != nil {
		return console.Options{}, err
	}
	templateData, err := os.ReadFile(templateFile)
	if err != nil {
		return console.Options{}, fmt.Errorf("demo template file unavailable")
	}
	template, validationErr := v0.ParseAgentTemplateYAML(templateData)
	if validationErr != nil || template.Metadata.Name != "engineer" || template.Spec.Artifact == nil || template.Spec.Artifact.Image != "agenova-testworker:kind" {
		return console.Options{}, fmt.Errorf("demo worker template is incompatible")
	}
	principal, err := app.NewReferencePrincipalSource(preset)
	if err != nil {
		return console.Options{}, err
	}
	host, err := demoactions.NewHost(toolRepo, nil)
	if err != nil {
		return console.Options{}, err
	}
	return console.Options{
		Prepare: func(data []byte) (app.PreparedAssignment, error) {
			return app.PrepareAssignment(data, principal, loader, fixedTemplate{document: templateData})
		},
		Setup: func() (console.Setup, error) {
			return console.Setup{Principal: principal.Principal(), Template: template, Policy: bundle, Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "configured", "model": "configured", "tool": "real-host-demo", "memory": "notConnected"}, Installation: console.InstallationIdentity{Kind: "local-controlled-kind-demo"}}, nil
		},
		Tools:          host,
		CandidateTools: []string{"git.read", "github.pr.create", "kubernetes.rollback"},
		ToolScopes:     map[string]string{"git.read": demoactions.DemoRepoScope, "github.pr.create": demoactions.DemoRepoScope, "kubernetes.rollback": demoactions.DemoKubeScope},
	}, nil
}

func consoleSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
