// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workstore"
)

// CleanupRestorer is an operator recovery capability, not an admission or worker
// capability. Reattachment must never make Start available on an old allocation.
type CleanupRestorer interface {
	runtime.RuntimeBackend
	RestoreForCleanup(string, v0.SandboxClaimBackendIdentity) error
}

// RecoverDurableWork runs only under the store's exclusive writer lease, before
// admission. It reconciles known allocations; it never calls Allocate/Start or a
// provider. Unobserved allocation identities and unconfirmed release stop recovery.
func RecoverDurableWork(ctx context.Context, store DurableWork, backend CleanupRestorer) (int, error) {
	if store == nil || backend == nil {
		return 0, ErrRecoveryRequired
	}
	pending, err := store.Pending(ctx)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, snapshot := range pending {
		if err := ctx.Err(); err != nil {
			return recovered, err
		}
		if view, ok := completedEvidence(snapshot); ok {
			if err := store.Finish(ctx, view); err != nil {
				return recovered, err
			}
			recovered++
			continue
		}
		if err := recoverAllocation(ctx, store, backend, snapshot); err != nil {
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}
func recoverAllocation(ctx context.Context, store DurableWork, backend CleanupRestorer, snapshot workstore.Snapshot) error {
	view := evidence.Clone(snapshot.View)
	if view.State == nil || v0.ValidateIssuedState(view.State) != nil || view.State.Claim == nil || view.State.Claim.BackendIdentity == nil {
		return ErrRecoveryRequired
	}
	claim := view.State.Claim
	if claim.Phase == v0.ClaimPhasePending {
		return ErrRecoveryRequired
	}
	terminal, terminated, cleaned := false, false, false
	terminalKind := ""
	for _, f := range view.Facts {
		if f.RequestRef != view.RequestRef || f.Kind == "RunOutcome" {
			return ErrRecoveryRequired
		}
		if f.Kind != "Runtime" {
			continue
		}
		switch f.Operation {
		case "Succeeded", "Failed", "Expired", "Cancelled", "StartFailed":
			terminal = true
			terminalKind = f.Operation
		case "TerminateSucceeded", "TerminateFailed":
			terminated = true
		case "CleanupSucceeded":
			cleaned = true
		case "CleanupFailed":
			return ErrRecoveryRequired // do not rewrite failed evidence
		}
	}
	if (claim.Phase == v0.ClaimPhaseSucceeded || claim.Phase == v0.ClaimPhaseFailed || claim.Phase == v0.ClaimPhaseExpired) != terminal {
		return ErrRecoveryRequired
	}
	if claim.Phase == v0.ClaimPhaseSucceeded && (snapshot.StagedResult == nil || snapshot.StagedResult.Model == nil || snapshot.StagedResult.Text == "") {
		return ErrRecoveryRequired
	}
	if err := store.CheckWriter(ctx); err != nil {
		return err
	}
	if !cleaned {
		if err := backend.RestoreForCleanup(claim.ID, *claim.BackendIdentity); err != nil {
			return ErrRecoveryRequired
		}
	}
	last, err := store.LastSequence(ctx)
	if err != nil {
		return err
	}
	journal, err := facts.NewDurableJournal(last, func(f facts.Fact) error { return store.Append(ctx, f, nil) })
	if err != nil {
		return err
	}
	if err := journal.RegisterRequest(view.RequestRef, view.State.Principal); err != nil {
		return err
	}
	if err := journal.BindClaim(view.RequestRef, *claim); err != nil {
		return err
	}
	networkLevel := ""
	runtimeFact := func(kind string) error {
		view.State.Evidence.RuntimeEvents = append(view.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: kind})
		fact := facts.Fact{Kind: "Runtime", RequestRef: view.RequestRef, ClaimID: claim.ID, BackendIdentity: claim.BackendIdentity, Operation: kind, ReasonCode: "recovery-" + kind, PolicyRef: &view.State.PolicyRef}
		if kind == "NetworkRevocationAttempt" || kind == "NetworkRevocationFailed" || kind == "NetworkRevoked" {
			fact.Target = "work-network"
			fact.Reason = "Recovery network release; original grant parameters are not reconstructed. This is not a packet-level enforcement test."
			if kind == "NetworkRevoked" {
				fact.Reason = "Network evidence level: " + networkLevel + ". " + fact.Reason
			}
		}
		f, err := journal.AppendWithCommit(fact, func(f facts.Fact) error { return store.Append(ctx, f, view.State) })
		if err == nil {
			view.Facts = append(view.Facts, f)
		}
		return err
	}
	if !terminal {
		claim.Phase = v0.ClaimPhaseFailed
		terminalKind = "Failed"
		if err := runtimeFact("Failed"); err != nil {
			// Failed persistence cannot leave a restored, verified allocation
			// online. A lost writer lease still forbids acting on another writer.
			if !cleaned {
				if writerErr := store.CheckWriter(ctx); writerErr != nil {
					return errors.Join(err, writerErr)
				}
				if network, ok := backend.(runtime.NetworkBackend); ok {
					_, revokeErr := network.RevokeNetwork(*claim.BackendIdentity)
					return errors.Join(err, revokeErr)
				}
				stopErr := backend.Terminate(*claim.BackendIdentity)
				_, cleanupErr := backend.Cleanup(*claim.BackendIdentity)
				return errors.Join(err, stopErr, cleanupErr)
			}
			return err
		}
	}
	if !cleaned {
		if err := store.CheckWriter(ctx); err != nil {
			return err
		}
		if network, ok := backend.(runtime.NetworkBackend); ok {
			attemptErr := runtimeFact("NetworkRevocationAttempt")
			result, err := network.RevokeNetwork(*claim.BackendIdentity)
			if attemptErr != nil {
				return errors.Join(attemptErr, err)
			}
			if err != nil || !validNetworkRelease(result, claim.ID, *claim.BackendIdentity) {
				if recordErr := runtimeFact("NetworkRevocationFailed"); recordErr != nil {
					return recordErr
				}
				return ErrRecoveryRequired
			}
			networkLevel = result.EvidenceLevel
			if err := runtimeFact("NetworkRevoked"); err != nil {
				return err
			}
		}
		stopErr := backend.Terminate(*claim.BackendIdentity)
		if stopErr != nil && !errors.Is(stopErr, runtime.ErrReleased) {
			return ErrRecoveryRequired
		}
		if !terminated {
			kind := "TerminateSucceeded"
			if stopErr != nil {
				kind = "TerminateFailed"
			}
			if err := runtimeFact(kind); err != nil {
				return err
			}
		}
		if err := store.CheckWriter(ctx); err != nil {
			return err
		}
		result, err := backend.Cleanup(*claim.BackendIdentity)
		if err != nil || !result.Released || result.Identity != *claim.BackendIdentity {
			return ErrRecoveryRequired
		}
		if err := runtimeFact("CleanupSucceeded"); err != nil {
			return err
		}
	}
	status := string(claim.Phase)
	if terminalKind == "Cancelled" {
		status = "Cancelled"
	}
	view.Outcome = &evidence.Outcome{Status: status}
	if claim.Phase == v0.ClaimPhaseSucceeded {
		view.Outcome.Text = snapshot.StagedResult.Text
		view.Outcome.Model = snapshot.StagedResult.Model
	} else {
		view.Outcome.Failure = "The service was interrupted. Runtime release was verified; uncertain external operations were not replayed."
	}
	f, err := journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: view.RequestRef, ClaimID: claim.ID, Operation: status, ReasonCode: "run-recovered-interruption", Reason: view.Outcome.Failure})
	if err != nil {
		return err
	}
	view.Facts = append(view.Facts, f)
	return store.Finish(ctx, view)
}

func validNetworkRelease(result runtime.NetworkEvidence, claim string, identity v0.SandboxClaimBackendIdentity) bool {
	return result.ClaimID == claim && result.Identity == identity && result.State == "Revoked" &&
		(result.EvidenceLevel == "Configured" || result.EvidenceLevel == "Simulated")
}
