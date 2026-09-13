// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package runtime defines the RuntimeBackend boundary between Agenova's stable
// runtime contract and any concrete sandbox substrate.
//
// Application-facing Agenova APIs must not change when the selected backend
// changes. The in-memory backend is the reference implementation and the
// contract test oracle.
package runtime

import (
	"errors"

	"github.com/wunderforge/agenova/api/v1alpha1"
)

// RuntimeBackend is the reduced, backend-neutral operation boundary one
// governed run needs from a sandbox substrate. It covers allocation/binding,
// observation, an explicit work-start boundary, termination and cleanup.
//
// It deliberately excludes pool administration and application work outcome.
// Pool setup is implementation-specific; Succeeded/Failed/Expired belong to
// the application lifecycle owner, never to the backend.
type RuntimeBackend interface {
	// Allocate binds one backend worker to the application claim identified
	// by req.ClaimID and returns the backend identity. It issues no public
	// claim and grants no authority. A ClaimID that already has a conflicting
	// allocation is rejected without disturbing the existing allocation.
	Allocate(req AllocateRequest) (Allocation, error)

	// Observe reports backend identity, readiness and resource evidence for a
	// known identity. Readiness is infrastructure evidence for Bound only; it
	// never establishes Running or Succeeded. Observe has no side effects.
	Observe(id v1alpha1.SandboxClaimBackendIdentity) (Observation, error)

	// Start explicitly acknowledges work start for a ready allocation. It is
	// distinct from Observe: readiness alone never implies start. Backends
	// that cannot establish actual work start return ErrUnsupported.
	Start(id v1alpha1.SandboxClaimBackendIdentity) error

	// Terminate stops worker execution, or cancels an allocation that never
	// started. It is idempotent and chooses no application outcome.
	Terminate(id v1alpha1.SandboxClaimBackendIdentity) error

	// Cleanup releases backend resources and reports release/replacement
	// evidence independently of any application outcome. It is idempotent.
	// The identity remains resolvable through Observe after release.
	Cleanup(id v1alpha1.SandboxClaimBackendIdentity) (CleanupResult, error)
}

// AllocateRequest is the resolved, backend-neutral launch input for one claim.
// Pool selection, if the backend uses pools, is implementation-internal.
type AllocateRequest struct {
	// ClaimID correlates the allocation with the application claim. Required.
	ClaimID string
	// TemplateRef names the runtime template to launch. Required.
	TemplateRef string
	// Input is the neutral launch input, using the same vocabulary as
	// BackendClaimSpec.Input.
	Input map[string]string
}

// FilesystemOutsideBoundary is the backend-neutral rule applied outside the
// worker's task directory. It describes supported configuration, not a host
// path policy or per-file authorization system.
type FilesystemOutsideBoundary string

const (
	// FilesystemOutsideRuntimeReadOnlyOtherUnavailable permits backend runtime
	// files to remain readable while all other task data outside the working
	// directory is unavailable through the supported worker configuration.
	FilesystemOutsideRuntimeReadOnlyOtherUnavailable FilesystemOutsideBoundary = "RuntimeReadOnlyOtherUnavailable"
)

// FilesystemEvidenceLevel states what an allocation has actually proved.
// Simulated is useful contract evidence but is never hostile-process
// isolation. BackendVerified requires separate real-backend evidence.
type FilesystemEvidenceLevel string

const (
	FilesystemEvidenceUnsupported     FilesystemEvidenceLevel = "Unsupported"
	FilesystemEvidenceSimulated       FilesystemEvidenceLevel = "Simulated"
	FilesystemEvidenceBackendVerified FilesystemEvidenceLevel = "BackendVerified"
)

// FilesystemBoundary describes the single task filesystem made available to
// one allocation. WorkingDirectory is worker-visible and backend-selected; it
// is never a caller-selected host path. An empty directory with Unsupported
// evidence reports a backend capability gap explicitly.
type FilesystemBoundary struct {
	WorkingDirectory string
	OutsideBoundary  FilesystemOutsideBoundary
	Ephemeral        bool
	EvidenceLevel    FilesystemEvidenceLevel
}

// Allocation is the result of a successful Allocate.
type Allocation struct {
	ClaimID    string
	Identity   v1alpha1.SandboxClaimBackendIdentity
	Filesystem FilesystemBoundary
}

// Observation is the backend's current resource evidence for one identity.
type Observation struct {
	ClaimID    string
	Identity   v1alpha1.SandboxClaimBackendIdentity
	Filesystem FilesystemBoundary
	// Ready is infrastructure readiness. It supports Bound and nothing more.
	Ready bool
	// Released reports that Cleanup confirmed resource release.
	Released bool
	// Replaced reports that the implementation replaced the worker slot. It
	// is false whenever replacement is not observable.
	Replaced bool
	// Detail is backend-specific human-readable context. It never carries
	// provider objects.
	Detail string
}

// CleanupResult is the resource evidence returned by Cleanup.
type CleanupResult struct {
	Identity v1alpha1.SandboxClaimBackendIdentity
	Released bool
	Replaced bool
}

// Sentinel errors shared by every backend. Implementations wrap them with
// context; callers match with errors.Is.
var (
	// ErrUnknownIdentity is returned when the full Backend/WorkerID pair does
	// not resolve to an allocation known to this backend.
	ErrUnknownIdentity = errors.New("runtime: unknown backend identity")
	// ErrNotReady is returned by Start when the allocation is not ready.
	ErrNotReady = errors.New("runtime: allocation not ready")
	// ErrAlreadyStarted is returned by a second Start.
	ErrAlreadyStarted = errors.New("runtime: work already started")
	// ErrTerminated is returned by Start after Terminate.
	ErrTerminated = errors.New("runtime: allocation terminated")
	// ErrReleased is returned by Start and Terminate after Cleanup.
	ErrReleased = errors.New("runtime: allocation released")
	// ErrUnsupported is returned when a backend cannot honestly provide the
	// operation's semantics. It is never a silent pass.
	ErrUnsupported = errors.New("runtime: operation unsupported by backend")
	// ErrFilesystemBoundary is returned by reference/model probes when an
	// operation targets data outside the allocation's task directory. Native
	// worker isolation remains a backend responsibility.
	ErrFilesystemBoundary = errors.New("runtime: outside task filesystem boundary")
)

// BackendClaim is the reference runtime's own state view of a claim
// assignment. It is not part of the reduced RuntimeBackend contract above and
// is distinct from the public, backend-neutral api/v1alpha1 authority/decision
// contract (SandboxClaim et al.). It remains here because the in-memory
// reference runtime still owns the application phase state machine until
// Ticket #31 delivers the run service, and gateways read it through
// ClaimReader for compatibility.
type BackendClaim struct {
	Metadata v1alpha1.ObjectMeta
	Spec     BackendClaimSpec
	Status   BackendClaimStatus
}

type BackendClaimSpec struct {
	PoolRef string
	Input   map[string]string
}

type BackendClaimStatus struct {
	Phase     v1alpha1.ClaimPhase
	SandboxID string
	Error     string

	// SandboxReplaced records the resource-side fact that the sandbox bound
	// to this claim was destroyed and replaced after the claim reached a
	// terminal phase. In the Kubernetes-facing shape this becomes a status
	// condition (SandboxReplaced=True), not a claim phase: claim phases keep
	// the business outcome (Succeeded/Failed/Expired); sandbox cleanup is a
	// resource fact.
	SandboxReplaced bool
}
