// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
	"github.com/wunderforge/agenova/internal/connectedclient"
	"github.com/wunderforge/agenova/internal/runtime"
)

func main() {
	os.Exit(cli.MainWithServices(os.Args, os.Stdout, os.Stderr, cli.Services{
		NewRuntime:      app.NewRuntime,
		Run:             submitClaimRequest,
		RunConnected:    submitConnected,
		NewAdapters:     app.NewAdapterLifecycle,
		NewPlatform:     app.NewPlatformService,
		NewRegistration: app.NewRegistrationService,
		Input:           os.Stdin,
	}))
}

func submitConnected(path, stateDirectory string) (cli.RunReport, error) {
	state, err := app.AppliedPlatform(stateDirectory)
	if err != nil {
		return cli.RunReport{}, err
	}
	contextName, namespace, err := app.DeploymentCoordinates(&state.Platform)
	if err != nil {
		return cli.RunReport{}, err
	}
	view, err := (connectedclient.Client{Context: contextName, Namespace: namespace}).RunFile(path)
	if view.RequestRef == "" {
		return cli.RunReport{}, err
	}
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
	return report, err
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
