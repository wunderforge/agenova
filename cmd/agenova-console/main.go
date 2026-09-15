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
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
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
	principal := flag.String("principal", "team-a", "fixed operator identity: team-a or team-b")
	flag.Parse()
	if strings.TrimSpace(*kube) == "" || strings.TrimSpace(*namespace) == "" || *namespace == "default" {
		return fmt.Errorf("explicit kube-context and non-default namespace are required")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("console must listen on a literal loopback address; no public authentication is provided")
	}
	provider, err := modelprovider.New(modelprovider.Config{Endpoint: *endpoint, Models: map[string]string{"approved-coding-model": *model}, MaxTokens: 256, Timeout: 2 * time.Minute})
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
	service, err := console.NewService(adapter, adapter, provider, app.ReferencePrincipalPreset(*principal))
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
	fmt.Printf("Internal demo listening at http://%s; fixed principal=%s. Tools/memory are not connected.\n", *listen, *principal)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("console server failed")
	}
	return nil
}

func consoleSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
