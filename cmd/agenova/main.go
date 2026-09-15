// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
	"github.com/wunderforge/agenova/internal/runtime"
)

func main() {
	os.Exit(cli.Main(os.Args, os.Stdout, os.Stderr, app.NewRuntime, submitClaimRequest))
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
