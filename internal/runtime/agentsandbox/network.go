// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

const claimUIDLabel = "agents.x-k8s.io/claim-uid"
const networkIncarnationLabel = "agenova.io/network-incarnation"

// A successful grant must remain tied to its activation lifetime even while
// application persistence is blocked. AfterFunc installs no parked goroutine;
// explicit trusted release unregisters every retained callback. Guard identity
// includes AllocationID, so a same-name replacement is never a cleanup target.
type networkLifetime struct {
	identity v1alpha1.SandboxClaimBackendIdentity
	stop     func() bool
	complete chan struct{}
	once     sync.Once
	err      error // protected by a.mu; retained when trusted revocation is unconfirmed
}

func (g *networkLifetime) finish() { g.once.Do(func() { close(g.complete) }) }

func (a *ControlledAdapter) armNetworkLifetimeLocked(ctx context.Context, id v1alpha1.SandboxClaimBackendIdentity) {
	key := ctx.Done()
	if key == nil {
		return
	} // Background has no cancellable lifetime to retain.
	guards := a.networkLifetimes[id.WorkerID]
	if guards == nil {
		guards = make(map[<-chan struct{}]*networkLifetime)
		a.networkLifetimes[id.WorkerID] = guards
	}
	if guards[key] != nil {
		return
	} // repeated activation keeps the original guard
	g := &networkLifetime{identity: id, complete: make(chan struct{})}
	guards[key] = g
	g.stop = context.AfterFunc(ctx, func() {
		defer g.finish()
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.networkLifetimes[id.WorkerID][key] != g {
			return
		}
		entry, err := a.entryFor(g.identity)
		if err == nil {
			_, err = a.revokeNetworkLocked(entry, g.identity)
		}
		g.err = err
		if err != nil && entry != nil {
			a.SpikeAdapter.mu.Lock()
			entry.detail = "activation context ended; trusted network revocation remains unconfirmed"
			a.SpikeAdapter.mu.Unlock()
		}
	})
}

func (a *ControlledAdapter) stopNetworkLifetimesLocked(id v1alpha1.SandboxClaimBackendIdentity) {
	guards := a.networkLifetimes[id.WorkerID]
	for key, g := range guards {
		if g.identity != id {
			continue
		}
		delete(guards, key)
		if g.stop != nil && g.stop() {
			g.finish()
		}
	}
	if len(guards) == 0 {
		delete(a.networkLifetimes, id.WorkerID)
	}
}

func (a *ControlledAdapter) canceledNetworkLifetimeLocked(id v1alpha1.SandboxClaimBackendIdentity) bool {
	for done, g := range a.networkLifetimes[id.WorkerID] {
		if g.identity != id {
			continue
		}
		select {
		case <-done:
			return true
		default:
		}
	}
	return false
}

type pinnedNetworkClient interface {
	createBytes([]byte) error
	createBytesContext(context.Context, []byte) error
	createNetworkPolicyBefore(context.Context, []byte, time.Time) error
	getContext(context.Context, string, string, any) error
	existsContext(context.Context, string, string) (bool, error)
	patchPodIncarnationContext(context.Context, string, string, string, string) error
	deletePinned(resource, name, uid, version string) error
	patchPodIncarnation(name, podUID, claimUID, value string) error
}

func networkIncarnation(claimUID, podUID string) string {
	digest := sha256.Sum256([]byte("agenova-work-network-v1\x00" + claimUID + "\x00" + podUID))
	return hex.EncodeToString(digest[:20])
}

type networkPolicyObject struct {
	Metadata struct {
		upstreamMeta
		Owners []struct{ APIVersion, Kind, Name, UID string } `json:"ownerReferences"`
	} `json:"metadata"`
	Spec map[string]any `json:"spec"`
}

// ConfigureNetworkIsolation declares an operator-verified startup-isolating
// substrate. This declaration is NOT packet enforcement evidence. It must be
// validated independently on the real cluster before advertising protection.
func (a *ControlledAdapter) ConfigureNetworkIsolation(mode string) error {
	if mode != "strict" {
		return fmt.Errorf("%w: strict startup network isolation is required", runtime.ErrUnsupported)
	}
	a.SpikeAdapter.mu.Lock()
	defer a.SpikeAdapter.mu.Unlock()
	a.networkMode = mode
	return nil
}

// ConfigureAgentTemplate refuses to start even an idle arbitrary image when
// the operator has not established a startup-isolating substrate.
func (a *ControlledAdapter) ConfigureAgentTemplate(template *v1alpha1.AgentTemplate) (string, error) {
	a.SpikeAdapter.mu.Lock()
	mode := a.networkMode
	a.SpikeAdapter.mu.Unlock()
	if mode != "strict" {
		return "", fmt.Errorf("%w: strict startup network isolation is required before configuring workers", runtime.ErrUnsupported)
	}
	// Controlled workers must not share warm-pool prefixes: some supported
	// CNIs cache policy by Pod name with the final hyphen segment removed.
	return a.SpikeAdapter.configureAgentTemplate(template, false)
}

func (a *SpikeAdapter) validateNetworkAllocation(n runtime.NetworkAuthority) error {
	if n.ExpiresAt.IsZero() && len(n.Profiles) == 0 {
		return nil // direct legacy adapter probes, not a governed Work allocation
	}
	a.mu.Lock()
	mode := a.networkMode
	a.mu.Unlock()
	if mode != "strict" {
		return fmt.Errorf("%w: startup-isolating network substrate is not configured", runtime.ErrUnsupported)
	}
	if n.ExpiresAt.IsZero() || !time.Now().Before(n.ExpiresAt) {
		return errors.New("network authority requires a future backend expiry")
	}
	if len(n.Profiles) > 1 || (len(n.Profiles) == 1 && n.Profiles[0] != "public-web") {
		return errors.New("unsupported network authority profile")
	}
	return nil
}

func networkRuleName(claimID, workerID string) string {
	return resourceName("net", claimID+":"+workerID)
}

func (a *ControlledAdapter) networkEvidence(entry *allocationEntry, state string) runtime.NetworkEvidence {
	a.SpikeAdapter.mu.Lock()
	defer a.SpikeAdapter.mu.Unlock()
	return runtime.NetworkEvidence{ClaimID: entry.claimID, Identity: entry.identity(), Profiles: append([]string(nil), entry.network.Profiles...), ExpiresAt: entry.network.ExpiresAt, State: state, EvidenceLevel: "Configured"}
}

// boundNetworkClaim verifies the current upstream binding plus controller-
// supplied Pod label. Neither worker-supplied values nor shared template names
// select a network grant. A changed binding fails without touching its peer.
func (a *ControlledAdapter) boundNetworkClaim(entry *allocationEntry) (upstreamSandboxClaim, error) {
	return a.boundNetworkClaimContext(context.Background(), entry)
}

func (a *ControlledAdapter) boundNetworkClaimContext(ctx context.Context, entry *allocationEntry) (upstreamSandboxClaim, error) {
	var claim upstreamSandboxClaim
	client, ok := a.kube.(pinnedNetworkClient)
	if !ok {
		return claim, runtime.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return claim, err
	}
	if err := client.getContext(ctx, "sandboxclaims", entry.upstreamClaimName, &claim); err != nil {
		return claim, errors.Join(err, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return claim, err
	}
	a.SpikeAdapter.mu.Lock()
	pinnedClaim, pinnedPod := entry.networkClaimUID, entry.networkPodUID
	a.SpikeAdapter.mu.Unlock()
	if claim.Metadata.Name != entry.upstreamClaimName || claim.Metadata.UID == "" || claim.Metadata.UID != pinnedClaim || claim.Status.Sandbox == nil || claim.Status.Sandbox.Name != entry.workerID {
		return claim, errIdentityMismatch
	}
	var pod struct {
		Metadata upstreamMeta `json:"metadata"`
		Spec     struct {
			HostNetwork bool `json:"hostNetwork"`
		} `json:"spec"`
	}
	if err := client.getContext(ctx, "pods", entry.workerID, &pod); err != nil {
		return claim, errors.Join(err, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return claim, err
	}
	if pod.Metadata.Name != entry.workerID || pod.Metadata.UID == "" || pod.Metadata.Labels[claimUIDLabel] != claim.Metadata.UID || pod.Spec.HostNetwork {
		return claim, errIdentityMismatch
	}
	if pinnedPod != "" && pod.Metadata.UID != pinnedPod {
		return claim, errIdentityMismatch
	}
	a.SpikeAdapter.mu.Lock()
	entry.networkPodUID = pod.Metadata.UID
	a.SpikeAdapter.mu.Unlock()
	return claim, nil
}

// ActivateNetwork is called after the application durably enters Running.
// NetworkPolicy API acceptance is configuration evidence, not CNI convergence.
func (a *ControlledAdapter) ActivateNetwork(ctx context.Context, id v1alpha1.SandboxClaimBackendIdentity) (evidence runtime.NetworkEvidence, activationErr error) {
	entry, err := a.entryFor(id)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.SpikeAdapter.mu.Lock()
	n := entry.network
	revoked := entry.networkRevoked
	a.SpikeAdapter.mu.Unlock()
	if revoked || a.phases[id.WorkerID] != controlRunning {
		return runtime.NetworkEvidence{}, runtime.ErrTerminated
	}
	if a.canceledNetworkLifetimeLocked(id) {
		return runtime.NetworkEvidence{}, runtime.ErrTerminated
	}
	if n.ExpiresAt.IsZero() {
		return runtime.NetworkEvidence{}, runtime.ErrUnsupported
	}
	// Bound every activation read/patch by both caller cancellation and the
	// allocation deadline. This per-call child is NOT the retained lifetime
	// signal; canceling it on return must not revoke a successfully active Work.
	readCtx, cancelReads := context.WithDeadline(ctx, n.ExpiresAt)
	defer cancelReads()
	deadline := n.ExpiresAt
	expired := func() bool { return !a.networkNow().Before(deadline) }
	compensate := false
	// Slow/ambiguous Kubernetes writes can complete after the authority deadline.
	// Never return an Active receipt in that case. Reuse the trusted UID-guarded
	// revocation path under the existing lock, not a worker-supplied stop ACK.
	defer func() {
		if expired() || compensate || ctx.Err() != nil {
			_, cleanupErr := a.revokeNetworkLocked(entry, id)
			evidence = runtime.NetworkEvidence{}
			if expired() {
				activationErr = errors.Join(activationErr, errors.New("network authority expired during activation"))
			}
			activationErr = errors.Join(activationErr, ctx.Err(), cleanupErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return runtime.NetworkEvidence{}, err
	}
	if err := a.validateNetworkAllocation(n); err != nil {
		return runtime.NetworkEvidence{}, err
	}
	if expired() {
		return runtime.NetworkEvidence{}, errors.New("network authority already expired")
	}
	claim, err := a.boundNetworkClaimContext(readCtx, entry)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	if claim.Spec.Lifecycle == nil || claim.Spec.Lifecycle.ShutdownTime == "" {
		return runtime.NetworkEvidence{}, errors.New("independent backend expiry is missing")
	}
	expiry, err := time.Parse(time.RFC3339Nano, claim.Spec.Lifecycle.ShutdownTime)
	if err != nil || expiry.After(n.ExpiresAt) {
		return runtime.NetworkEvidence{}, errors.New("independent backend expiry does not match authority")
	}
	deadline = expiry
	if expiry.Before(n.ExpiresAt) {
		var cancelEarlier context.CancelFunc
		readCtx, cancelEarlier = context.WithDeadline(readCtx, expiry)
		defer cancelEarlier()
	}
	if expired() {
		return runtime.NetworkEvidence{}, errors.New("independent backend expiry elapsed")
	}
	if len(n.Profiles) > 0 {
		client, ok := a.kube.(pinnedNetworkClient)
		if !ok {
			return runtime.NetworkEvidence{}, runtime.ErrUnsupported
		}
		// Stamp only the original Pod, guarded atomically by its UID and claim
		// label. Never place this label in a Sandbox/template: a replacement Pod
		// under the same Running claim must not inherit this network capability.
		if err := client.patchPodIncarnationContext(readCtx, entry.workerID, entry.networkPodUID, entry.networkClaimUID, networkIncarnation(entry.networkClaimUID, entry.networkPodUID)); err != nil {
			return runtime.NetworkEvidence{}, fmt.Errorf("pin network Pod incarnation: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		manifest, err := json.Marshal(a.publicWebPolicy(entry, claim.Metadata.UID))
		if err != nil {
			return runtime.NetworkEvidence{}, err
		}
		// An idempotent retry may inspect an existing rule, but never overwrite
		// it. A failed/ambiguous create is NOT converted into successful authority.
		present, queryErr := client.existsContext(readCtx, "networkpolicies", networkRuleName(entry.claimID, entry.workerID))
		if queryErr != nil {
			return runtime.NetworkEvidence{}, queryErr
		}
		if err := ctx.Err(); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		if !present {
			// Reads and patch are remote calls: check immediately before write.
			if expired() {
				return runtime.NetworkEvidence{}, errors.New("network authority expired before rule creation")
			}
			if err := ctx.Err(); err != nil {
				return runtime.NetworkEvidence{}, err
			}
			if createErr := client.createNetworkPolicyBefore(ctx, manifest, deadline); createErr != nil {
				compensate = true
				return runtime.NetworkEvidence{}, createErr
			}
			if err := ctx.Err(); err != nil {
				return runtime.NetworkEvidence{}, err
			}
			if expired() {
				return runtime.NetworkEvidence{}, errors.New("network authority expired during rule creation")
			}
		}
		compensate = true // any failed owned readback/binding must remove the grant
		policy, readErr := a.ownedNetworkPolicyContext(readCtx, entry)
		if readErr != nil {
			return runtime.NetworkEvidence{}, readErr
		}
		if err := ctx.Err(); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		var desired networkPolicyObject
		if err := json.Unmarshal(manifest, &desired); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		if !reflect.DeepEqual(policy.Spec, desired.Spec) {
			return runtime.NetworkEvidence{}, errors.New("existing network rule differs from resolved authority")
		}
		if _, err := a.boundNetworkClaimContext(readCtx, entry); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		compensate = false
	}
	if err := ctx.Err(); err != nil {
		return runtime.NetworkEvidence{}, err
	}
	a.SpikeAdapter.mu.Lock()
	entry.networkActive = true
	a.SpikeAdapter.mu.Unlock()
	a.armNetworkLifetimeLocked(ctx, id)
	return a.networkEvidence(entry, "Active"), nil
}

func (a *ControlledAdapter) publicWebPolicy(entry *allocationEntry, uid string) map[string]any {
	return map[string]any{
		"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy",
		"metadata": map[string]any{"name": networkRuleName(entry.claimID, entry.workerID), "namespace": a.namespace,
			"annotations":     map[string]string{"agenova.io/claim-id": entry.claimID, "agenova.io/worker-id": entry.workerID},
			"labels":          map[string]string{"app.kubernetes.io/managed-by": "agenova"},
			"ownerReferences": []any{map[string]any{"apiVersion": apiVersionExtensions, "kind": kindSandboxClaim, "name": entry.upstreamClaimName, "uid": uid}}},
		"spec": map[string]any{
			"podSelector": map[string]any{"matchLabels": map[string]string{claimUIDLabel: uid, networkIncarnationLabel: networkIncarnation(uid, entry.networkPodUID)}},
			"policyTypes": []string{"Egress"},
			"egress": []any{
				map[string]any{"to": []any{map[string]any{"ipBlock": map[string]any{"cidr": "0.0.0.0/0", "except": []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}}}}, "ports": []any{map[string]any{"protocol": "TCP", "port": 443}}},
				map[string]any{"to": []any{map[string]any{"namespaceSelector": map[string]any{"matchLabels": map[string]string{"kubernetes.io/metadata.name": "kube-system"}}, "podSelector": map[string]any{"matchLabels": map[string]string{"k8s-app": "kube-dns"}}}}, "ports": []any{map[string]any{"protocol": "UDP", "port": 53}, map[string]any{"protocol": "TCP", "port": 53}}},
			},
		},
	}
}

func (a *ControlledAdapter) ownedNetworkPolicy(entry *allocationEntry) (networkPolicyObject, error) {
	var policy networkPolicyObject
	if err := a.kube.get("networkpolicies", networkRuleName(entry.claimID, entry.workerID), &policy); err != nil {
		return policy, err
	}
	return a.validateOwnedNetworkPolicy(entry, policy)
}

func (a *ControlledAdapter) ownedNetworkPolicyContext(ctx context.Context, entry *allocationEntry) (networkPolicyObject, error) {
	var policy networkPolicyObject
	client, ok := a.kube.(pinnedNetworkClient)
	if !ok {
		return policy, runtime.ErrUnsupported
	}
	if err := client.getContext(ctx, "networkpolicies", networkRuleName(entry.claimID, entry.workerID), &policy); err != nil {
		return policy, err
	}
	return a.validateOwnedNetworkPolicy(entry, policy)
}

func (a *ControlledAdapter) validateOwnedNetworkPolicy(entry *allocationEntry, policy networkPolicyObject) (networkPolicyObject, error) {
	a.SpikeAdapter.mu.Lock()
	claimUID := entry.networkClaimUID
	a.SpikeAdapter.mu.Unlock()
	m := policy.Metadata
	if m.Name != networkRuleName(entry.claimID, entry.workerID) || m.Namespace != a.namespace || m.UID == "" || m.ResourceVersion == "" || m.Labels["app.kubernetes.io/managed-by"] != "agenova" || m.Annotations["agenova.io/claim-id"] != entry.claimID || m.Annotations["agenova.io/worker-id"] != entry.workerID {
		return policy, errIdentityMismatch
	}
	for _, owner := range m.Owners {
		if owner.APIVersion == apiVersionExtensions && owner.Kind == kindSandboxClaim && owner.Name == entry.upstreamClaimName && owner.UID == claimUID && claimUID != "" {
			return policy, nil
		}
	}
	return policy, errIdentityMismatch
}

// RevokeNetwork removes the grant and destroys the worker through the trusted
// substrate, closing existing connections even when an arbitrary image lies
// about stop acknowledgements. No success is reported until Pod absence.
func (a *ControlledAdapter) RevokeNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	entry, err := a.entryFor(id)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.revokeNetworkLocked(entry, id)
}

// revokeNetworkLocked is shared with activation's expiry compensation. Caller
// holds a.mu, preserving serialization with activation and terminal teardown.
func (a *ControlledAdapter) revokeNetworkLocked(entry *allocationEntry, id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	a.SpikeAdapter.mu.Lock()
	revoked := entry.networkRevoked
	a.SpikeAdapter.mu.Unlock()
	if revoked {
		a.stopNetworkLifetimesLocked(id)
		return a.networkEvidence(entry, "Revoked"), nil
	}
	present, err := a.kube.exists("sandboxclaims", entry.upstreamClaimName)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	var claim upstreamSandboxClaim
	if present {
		if err := a.kube.get("sandboxclaims", entry.upstreamClaimName, &claim); err != nil {
			return runtime.NetworkEvidence{}, err
		}
		if claim.Metadata.Name != entry.upstreamClaimName || claim.Metadata.UID != entry.networkClaimUID || claim.Metadata.UID == "" || claim.Metadata.ResourceVersion == "" || claim.Status.Sandbox == nil || claim.Status.Sandbox.Name != id.WorkerID {
			return runtime.NetworkEvidence{}, errIdentityMismatch
		}
	}
	client, ok := a.kube.(pinnedNetworkClient)
	if !ok {
		return runtime.NetworkEvidence{}, runtime.ErrUnsupported
	}
	var errs []error
	ruleName := networkRuleName(entry.claimID, id.WorkerID)
	rulePresent, queryErr := a.kube.exists("networkpolicies", ruleName)
	if queryErr != nil {
		errs = append(errs, queryErr)
	}
	if rulePresent {
		policy, err := a.ownedNetworkPolicy(entry)
		if err == nil {
			err = client.deletePinned("networkpolicies", ruleName, policy.Metadata.UID, policy.Metadata.ResourceVersion)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	// Always attempt trusted worker destruction even when rule removal failed.
	if present {
		if err := client.deletePinned("sandboxclaims", entry.upstreamClaimName, claim.Metadata.UID, claim.Metadata.ResourceVersion); err != nil {
			return runtime.NetworkEvidence{}, errors.Join(append(errs, err)...)
		}
	}
	if err := a.confirmReleased(entry); err != nil {
		return runtime.NetworkEvidence{}, err
	}
	deadline := time.Now().Add(a.cleanupTimeout)
	for {
		present, err := a.kube.exists("pods", id.WorkerID)
		if err != nil {
			return runtime.NetworkEvidence{}, err
		}
		if !present {
			break
		}
		if !time.Now().Before(deadline) {
			return runtime.NetworkEvidence{}, errCleanupUnconfirmed
		}
		time.Sleep(a.pollInterval)
	}
	if len(errs) > 0 {
		return runtime.NetworkEvidence{}, errors.Join(errs...)
	}
	a.SpikeAdapter.mu.Lock()
	entry.networkActive = false
	entry.networkRevoked = true
	entry.state = allocReleased
	entry.detail = "network grant removed; claim, sandbox and Pod confirmed absent"
	a.SpikeAdapter.mu.Unlock()
	a.phases[id.WorkerID] = controlTerminated
	a.stopNetworkLifetimesLocked(id)
	a.changed.Broadcast()
	return a.networkEvidence(entry, "Revoked"), nil
}
