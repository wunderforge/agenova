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
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/workerprotocol"
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
	configured, err := configuredService("/etc/agenova/effective-platform.json")
	if err != nil {
		return err
	}
	defer configured.Close()
	// Admission can perform three bounded registration reads (20s each) and
	// two bounded runtime setup calls (30s each) before returning Accepted.
	// Keep the server budget above their sum; clients wait longer still.
	private := &http.Server{Addr: "127.0.0.1:8081", Handler: console.Handler(configured), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 3 * time.Minute}
	privateListener, err := net.Listen("tcp", private.Addr)
	if err != nil {
		return fmt.Errorf("start private Work service: %w", err)
	}
	go func() {
		if err := private.Serve(privateListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("private work service stopped: %v", err)
		}
	}()
	server := &http.Server{Addr: ":8080", Handler: handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("agenova reference control plane listening on %s", server.Addr)
	return server.ListenAndServe()
}

func configuredService(path string) (*console.Service, error) {
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
	compatibleWorkerImage := strings.TrimSpace(os.Getenv("AGENOVA_ALLOWED_WORKER_IMAGE"))
	if compatibleWorkerImage == "" {
		return nil, fmt.Errorf("installed runtime compatible worker image is missing")
	}
	if err := setInClusterKubeconfig(); err != nil {
		return nil, err
	}
	_, namespace, err := app.DeploymentCoordinates(&resolved)
	if err != nil {
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
	modelConfig := modelprovider.Config{Models: map[string]string{}, MaxTokens: 512, OutputSchema: []byte(workerprotocol.ActionSchema), Timeout: 2 * time.Minute}
	backends := map[string]string{}
	for _, instance := range resolved.Instances {
		if instance.Category != platform.CapabilityModel {
			continue
		}
		if adapterIDs[instance.AdapterRef] != bundled.OpenAICompatibleModelID {
			return nil, fmt.Errorf("reference Control Plane does not support the selected model adapter")
		}
		endpoint, _ := instance.Config["endpoint"].(string)
		backends[instance.Name] = endpoint
	}
	for _, profile := range resolved.Profiles {
		if profile.Capability != platform.CapabilityModel {
			continue
		}
		model, _ := profile.Config["model"].(string)
		if modelConfig.Endpoint == "" {
			modelConfig.Endpoint = backends[profile.BackendRef]
		}
		if modelConfig.Endpoint != backends[profile.BackendRef] {
			return nil, fmt.Errorf("reference model composition supports one endpoint")
		}
		modelConfig.Models[profile.Name] = model
	}
	runtimeProfiles := map[string]bool{}
	for _, profile := range resolved.Profiles {
		if profile.Capability == platform.CapabilityRuntime {
			runtimeProfiles[profile.Name] = true
		}
	}
	modelConfig.AllowDockerHostHTTP = strings.HasPrefix(modelConfig.Endpoint, "http://host.docker.internal:")
	provider, err := modelprovider.New(modelConfig)
	if err != nil {
		return nil, err
	}
	preset := app.ReferencePrincipalTeamA // Fixed local reference identity, never request/CLI supplied.
	principal, err := app.NewReferencePrincipalSource(preset)
	if err != nil {
		return nil, err
	}
	return console.NewServiceWithOptions(adapter, adapter, provider, preset, console.Options{
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
			return console.Setup{Principal: principal.Principal(), Template: templates[0], Policy: bundle,
				Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "configured", "model": "configured", "tool": "mock", "memory": "notConnected"},
				Installation: console.InstallationIdentity{Kind: "installed", Platform: resolved.PlatformName, Revision: resolved.Revision}}, nil
		},
		Prepare: func(data []byte) (app.PreparedAssignment, error) {
			bundle, err := store.ActivePolicy()
			if err != nil {
				return app.PreparedAssignment{}, &console.SubmissionError{Code: "active_policy_unavailable", Message: "Active PolicyBundle is unavailable; register or repair the active policy.", Cause: err}
			}
			loader := &policy.Loader{}
			if err := loader.Load(bundle); err != nil {
				return app.PreparedAssignment{}, &console.SubmissionError{Code: "active_policy_invalid", Message: "Active PolicyBundle is invalid; register a valid policy version.", Cause: err}
			}
			prepared, err := app.PrepareAssignment(data, principal, loader, store)
			if err != nil {
				request, parseErr := v0.ParseClaimRequestJSON(data)
				if parseErr == nil {
					if _, lookupErr := store.Template(request.Spec.TemplateRef); lookupErr != nil {
						return prepared, &console.SubmissionError{Code: "agent_template_unavailable", Message: "AgentTemplate is unavailable; register the requested template.", Cause: err}
					}
				}
				return prepared, &console.SubmissionError{Code: "assignment_unavailable", Message: "Assignment could not be resolved; check the registered template and active policy.", Cause: err}
			}
			if prepared.Issued != nil && prepared.Issued.Claim != nil {
				if err := validateInstalledAuthority(prepared.Issued.EffectiveAuthority, modelConfig.Models, runtimeProfiles); err != nil {
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
	})
}

// This reference composition currently provides only a synthetic git.read
// adapter and no Memory Interface. Check the issued effective grant before
// the console journals a claim or configures a worker: a broader template
// ceiling must never be mistaken for an implemented gateway capability.
func validateInstalledAuthority(authority *v0.EffectiveAuthority, models map[string]string, runtimeProfiles map[string]bool) error {
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
		if tool != "git.read" {
			return &console.SubmissionError{Code: "tool_unsupported", Message: "Granted tool is not supported by the installed Tool Gateway; narrow the template or install a compatible gateway."}
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
