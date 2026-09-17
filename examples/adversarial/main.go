// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command adversarial is the E9-T3 denial demo: it submits one canonical
// ClaimRequest (YAML or JSON) via --task under an out-of-band, unauthorized
// Team B principal and evaluates it through the merged E2 authorization path
// (internal/authorization with a fixture PolicyBundle). It prints [DENIED]
// with the real Decision evidence — principal subject, policy ID/version, and
// denial reason — before any claim is created, and exits non-zero on denial
// and on malformed input.
//
// This runs at fixture depth: the policy bundle and principal are composed
// locally, with no live backend and no credentials. It is not live Agenova
// policy enforcement.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/policy"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// fixtureModeLabel makes it impossible to mistake this demo for a live policy
// enforcement path.
const fixtureModeLabel = "[FIXTURE MODE] policy evaluated against a static reference bundle — not live Agenova governance"

// claimStore records the claims a demo would create so tests can assert that a
// denied submission never reaches claim creation. The adversarial continuation
// is only ever invoked on an Allow decision.
type claimStore struct {
	created []string
}

func (s *claimStore) create(requestRef string) {
	s.created = append(s.created, requestRef)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, &claimStore{}))
}

// run keeps the whole demo testable in-process: tests inject their own claim
// store and buffers, then assert on the exit code, the printed evidence, and
// that no claim was created. The store's create is the only claim path; run
// never fabricates a claim any other way.
func run(args []string, stdout, stderr io.Writer, store *claimStore) int {
	flags := flag.NewFlagSet("adversarial", flag.ContinueOnError)
	flags.SetOutput(stderr)
	taskPath := flags.String("task", "", "path to a ClaimRequest YAML or JSON file")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *taskPath == "" {
		fmt.Fprintln(stderr, "adversarial: --task is required: path to a ClaimRequest YAML or JSON file")
		return exitUsage
	}

	data, err := os.ReadFile(*taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "adversarial: read task file %s: %v\n", *taskPath, err)
		return exitFailure
	}

	// The trusted principal is composed out-of-band as the unauthorized Team B
	// identity; the reference bundle only allows Team A, so this request is
	// denied by default-deny before any claim exists.
	loader := &policy.Loader{}
	if err := loader.Load(policy.ReferenceBundle()); err != nil {
		fmt.Fprintf(stderr, "adversarial: load reference policy bundle: %v\n", err)
		return exitFailure
	}
	service, err := app.NewReferenceAssignmentService(app.ReferencePrincipalTeamB, authorization.Authorizer{Policies: loader})
	if err != nil {
		fmt.Fprintf(stderr, "adversarial: construct reference assignment service: %v\n", err)
		return exitFailure
	}

	fmt.Fprintln(stdout, fixtureModeLabel)

	result, err := service.AdmitYAML(data, func(admission authorization.Admission) error {
		// Reached only on Allow. For the Team B adversarial fixture this must
		// never run; if it did, a claim would be created here.
		store.create(admission.Decision().PrincipalRef)
		return nil
	})
	if err != nil {
		fmt.Fprintf(stderr, "adversarial: %v\n", err)
		return exitFailure
	}

	decision := result.Decision
	switch decision.Result {
	case v1alpha1.DecisionResultDeny:
		fmt.Fprintf(stdout, "[DENIED] request=%s principal=%s team=%s\n",
			result.RequestRef, result.Principal.Subject, result.Principal.Team)
		fmt.Fprintf(stdout, "         policy=%s@%s decisionID=%s\n",
			decision.PolicyRef.ID, decision.PolicyRef.Version, decision.ID)
		fmt.Fprintf(stdout, "         reason=%s\n", decision.Reason)
		fmt.Fprintf(stdout, "         claimsCreated=%d (no SandboxClaim issued)\n", len(store.created))
		return exitFailure
	case v1alpha1.DecisionResultAllow:
		fmt.Fprintf(stdout, "[ALLOWED] request=%s principal=%s claimsCreated=%d\n",
			result.RequestRef, result.Principal.Subject, len(store.created))
		return exitOK
	default:
		fmt.Fprintf(stderr, "adversarial: unexpected decision result %q for request %s\n",
			decision.Result, result.RequestRef)
		return exitFailure
	}
}
