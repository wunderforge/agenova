// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"github.com/wunderforge/agenova/internal/login"
	"io"
	"os"
	"os/exec"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
	"github.com/wunderforge/agenova/internal/connectedclient"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/runtime"
)

func main() {
	os.Exit(cli.MainWithServices(os.Args, os.Stdout, os.Stderr, cli.Services{
		Login:           memberLogin,
		Logout:          memberLogout,
		HTTPS:           memberServices,
		NewRuntime:      app.NewRuntime,
		Run:             submitClaimRequest,
		RunConnected:    submitConnected,
		ShowConnected:   showConnected,
		ListConnected:   listConnected,
		ConnectAPI:      connectAPI,
		NewAdapters:     app.NewAdapterLifecycle,
		NewPlatform:     app.NewPlatformService,
		NewRegistration: app.NewRegistrationService,
		Input:           os.Stdin,
	}))
}

func installedClient(stateDirectory string) (connectedclient.Client, error) {
	state, err := app.AppliedPlatform(stateDirectory)
	if err != nil {
		return connectedclient.Client{}, fmt.Errorf("apply a Platform before connecting to Work: %w", err)
	}
	contextName, namespace, err := app.DeploymentCoordinates(&state.Platform)
	if err != nil {
		return connectedclient.Client{}, err
	}
	return connectedclient.Client{Context: contextName, Namespace: namespace}, nil
}

func showConnected(ref, stateDirectory string) (evidence.View, error) {
	client, err := installedClient(stateDirectory)
	if err != nil {
		return evidence.View{}, err
	}
	return client.Show(ref)
}

func listConnected(stateDirectory string) ([]evidence.View, error) {
	client, err := installedClient(stateDirectory)
	if err != nil {
		return nil, err
	}
	return client.List()
}

func connectAPI(stateDirectory string, port int, stdout, stderr io.Writer) error {
	client, err := installedClient(stateDirectory)
	if err != nil {
		return err
	}
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl is required to connect to the installed API")
	}
	cmd := exec.Command(path, "--context", client.Context, "--namespace", client.Namespace,
		"port-forward", "deployment/agenova-control-plane", fmt.Sprintf("%d:8081", port), "--address", "127.0.0.1")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	fmt.Fprint(stdout, apiConnectInstruction(port))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("local API connection stopped; check port availability, Platform status and Kubernetes access")
	}
	return nil
}

func apiConnectInstruction(port int) string {
	message := fmt.Sprintf("Connecting local Agenova API at http://127.0.0.1:%d. Wait for kubectl's Forwarding line, then run npm --prefix ui run dev in another terminal.\n", port)
	if port != 8088 {
		message += fmt.Sprintf("In the Vite terminal, set $env:AGENOVA_API_URL = 'http://127.0.0.1:%d' (PowerShell) or run AGENOVA_API_URL=http://127.0.0.1:%d npm --prefix ui run dev (POSIX shell).\n", port, port)
	}
	return message
}

func submitConnected(path, stateDirectory string) (cli.RunReport, error) {
	client, err := installedClient(stateDirectory)
	if err != nil {
		return cli.RunReport{}, err
	}
	view, err := client.RunFile(path)
	if view.RequestRef == "" {
		return cli.RunReport{}, err
	}
	return connectedReport(view), err
}

func connectedReport(view evidence.View) cli.RunReport {
	report := cli.RunReport{RequestRef: view.RequestRef, Evidence: &view}
	if view.State != nil {
		report.Decision = string(view.State.Decision.Result)
		report.Principal = view.State.Principal.Subject
		if view.State.Claim != nil {
			report.ClaimID = view.State.Claim.ID
			report.Phase = string(view.State.Claim.Phase)
			report.Allocated = view.State.Claim.BackendIdentity != nil
		}
	}
	if view.Outcome != nil {
		report.Phase = view.Outcome.Status
	}
	return report
}

func submitClaimRequest(path string, backend runtime.RuntimeBackend) (cli.RunReport, error) {
	preset, err := app.PrincipalPresetFromEnv()
	if err != nil {
		return cli.RunReport{}, err
	}
	result, err := app.SubmitClaimRequestFile(path, backend, preset)
	return cli.RunReport{
		RequestRef: result.RequestRef,
		Decision:   string(result.Decision),
		Principal:  result.Principal,
		Allocated:  result.Allocated,
		ClaimID:    result.ClaimID,
		Phase:      string(result.Phase),
		Evidence:   result.Evidence,
	}, err
}

func memberClient(server string) (*login.Client, error) {
	if !login.Origin(server) {
		return nil, fmt.Errorf("--server must be an HTTPS origin without a path, query or credentials")
	}
	store, err := login.NewOSStore()
	if err != nil {
		return nil, err
	}
	return &login.Client{Server: server, Store: store}, nil
}
func memberLogin(server string) error {
	client, err := memberClient(server)
	if err != nil {
		return err
	}
	return client.Login(context.Background())
}
func memberLogout(server string) error {
	client, err := memberClient(server)
	if err != nil {
		return err
	}
	return client.Logout(context.Background())
}
func memberServices(server string) (cli.Services, error) {
	auth, err := memberClient(server)
	if err != nil {
		return cli.Services{}, err
	}
	client := connectedclient.Client{Server: server, AccessToken: auth.AccessToken}
	return cli.Services{
		RunConnected: func(path, _ string) (cli.RunReport, error) {
			v, err := client.RunFile(path)
			if v.RequestRef == "" {
				return cli.RunReport{}, err
			}
			return connectedReport(v), err
		},
		ShowConnected: func(ref, _ string) (evidence.View, error) { return client.Show(ref) },
		ListConnected: func(_ string) ([]evidence.View, error) { return client.List() },
	}, nil
}
