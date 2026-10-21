// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/wunderforge/agenova/internal/runtime"
)

// defaultCommandTimeout bounds every single kubectl invocation so a hung
// subprocess cannot outlive the adapter's polling deadlines.
const defaultCommandTimeout = 30 * time.Second

// A selected image can write arbitrary stdout/stderr through exec. A deadline
// alone does not bound the memory captured before that deadline.
const commandOutputLimit = 1 << 20

type boundedCommandOutput struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *boundedCommandOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := commandOutputLimit - b.data.Len()
	if len(p) > remaining {
		n, _ := b.data.Write(p[:remaining])
		return n, io.ErrShortWrite
	}
	return b.data.Write(p)
}

// commandFunc runs one kubectl invocation. It exists so tests can substitute
// a fake executable while the real argument construction and result
// classification in kubectlRunner stay under test.
type commandFunc func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)

// kubeClient is the adapter's view of the cluster. kubectlRunner is the real
// implementation; tests may provide fakes at this seam as well.
type kubeClient interface {
	applyBytes(manifest []byte) error
	get(resource, name string, dst any) error
	delete(resource, name string) error
	exists(resource, name string) (bool, error)
}

// kubectlRunner executes kubectl commands against a specific context and
// namespace. It is the only mechanism this package uses to talk to Kubernetes,
// keeping the adapter free of k8s.io/client-go transitive dependencies during
// the spike phase.
type kubectlRunner struct {
	context   string
	namespace string
	timeout   time.Duration
	command   commandFunc
}

func newKubectlRunner(kubeContext, namespace string) *kubectlRunner {
	return &kubectlRunner{
		context:   kubeContext,
		namespace: namespace,
		timeout:   defaultCommandTimeout,
		command:   execKubectl,
	}
}

// killWaitDelay bounds how long we wait for output pipes to close after the
// deadline kills kubectl. Without it a child process (for example a credential
// plugin) holding the pipe would keep CombinedOutput blocked past the deadline.
const killWaitDelay = time.Second

// execKubectl is the production commandFunc. The context deadline is enforced
// by exec.CommandContext, which kills the subprocess when it expires.
func execKubectl(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.WaitDelay = killWaitDelay
	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}
	var output boundedCommandOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	out := output.data.Bytes() // All process/copy goroutines have joined.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out, fmt.Errorf("%w (command deadline %s)", ctxErr, deadlineString(ctx))
	}
	return out, err
}

func deadlineString(ctx context.Context) string {
	if d, ok := ctx.Deadline(); ok {
		return d.Format(time.RFC3339)
	}
	return "none"
}

// applyBytes applies a manifest provided as raw bytes via stdin.
func (r *kubectlRunner) applyBytes(manifest []byte) error {
	raw, err := r.runWithStdin(manifest, "apply", "-f", "-")
	if err != nil {
		return fmt.Errorf("kubectl apply: %w\noutput: %s", err, raw)
	}
	return nil
}

func (r *kubectlRunner) createBytes(manifest []byte) error {
	_, err := r.runWithStdin(manifest, "create", "-f", "-")
	return err
}

// Bound the grant write itself, not just the checks preceding it. A canceled
// API request may still have committed; activation always performs owned
// readback/revocation compensation rather than treating cancellation as absence.
func (r *kubectlRunner) createNetworkPolicyBefore(parent context.Context, manifest []byte, expiry time.Time) error {
	deadline := time.Now().Add(r.timeout)
	if expiry.Before(deadline) {
		deadline = expiry
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := r.command(ctx, manifest, r.baseArgs("create", "-f", "-")...)
	return err
}

// patchPodIncarnation atomically guards the original Pod's immutable identity.
// Patching a reused name without these JSONPatch tests could authorize a peer.
func (r *kubectlRunner) patchPodIncarnation(name, podUID, claimUID, value string) error {
	if !isDNSLabel(name) || podUID == "" || claimUID == "" || value != networkIncarnation(claimUID, podUID) {
		return errors.New("invalid Pod network incarnation")
	}
	patch, err := json.Marshal([]map[string]string{
		{"op": "test", "path": "/metadata/uid", "value": podUID},
		{"op": "test", "path": "/metadata/labels/agents.x-k8s.io~1claim-uid", "value": claimUID},
		{"op": "add", "path": "/metadata/labels/agenova.io~1network-incarnation", "value": value},
	})
	if err != nil {
		return err
	}
	_, err = r.runWithStdin(patch, "patch", "pods", name, "--type=json", "--patch-file=-")
	return err
}

// deletePinned uses API DeleteOptions preconditions. kubectl's ordinary delete
// can otherwise destroy a same-name replacement between verification/deletion.
func (r *kubectlRunner) deletePinned(resource, name, uid, version string) error {
	if !isDNSLabel(name) || uid == "" || version == "" {
		return errors.New("pinned delete requires full resource identity")
	}
	var prefix string
	switch resource {
	case "sandboxclaims":
		prefix = "/apis/extensions.agents.x-k8s.io/v1alpha1/namespaces/"
	case "networkpolicies":
		prefix = "/apis/networking.k8s.io/v1/namespaces/"
	default:
		return errors.New("unsupported pinned deletion resource")
	}
	if !isDNSLabel(r.namespace) {
		return errors.New("invalid pinned deletion namespace")
	}
	payload, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": map[string]string{"uid": uid, "resourceVersion": version}})
	_, err := r.runWithStdin(payload, "delete", "--raw", prefix+r.namespace+"/"+resource+"/"+name, "-f", "-")
	return err
}

// get fetches a resource by type and name, unmarshaling into dst.
func (r *kubectlRunner) get(resource, name string, dst any) error {
	raw, err := r.run("get", resource, name, "-o", "json")
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// delete deletes a resource by type and name. A missing resource is not an
// error; delete says nothing about whether dependants are gone.
func (r *kubectlRunner) delete(resource, name string) error {
	_, err := r.run("delete", resource, name, "--ignore-not-found=true")
	return err
}

// exists reports whether the named resource is present.
//
// Classification is by exit status and output, never by error text:
// --ignore-not-found makes a missing object exit 0 with empty output, so
// exit 0 + empty ⇒ absent, exit 0 + name ⇒ present, and every non-zero exit
// (auth, connectivity, missing plugin, timeout) is returned as an error. A
// query failure is therefore never mistaken for release evidence.
func (r *kubectlRunner) exists(resource, name string) (bool, error) {
	raw, err := r.run("get", resource, name, "--ignore-not-found=true", "-o", "name")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(raw))) > 0, nil
}

// execWorker invokes the explicit, image-provided Agenova control protocol
// inside the bound Pod. kubectl execution is not an upstream SandboxClaim
// operation; a different worker image must opt in to this protocol.
func (r *kubectlRunner) execWorker(workerID string, args ...string) (string, error) {
	if workerID == "" || strings.ContainsAny(workerID, "/ \t\r\n") {
		return "", fmt.Errorf("invalid worker id %q", workerID)
	}
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, "\r\n") {
			return "", fmt.Errorf("invalid worker control argument %q", arg)
		}
	}
	argv := append([]string{"exec", "pod/" + workerID, "-c", "agent", "--", "/agenova-workerctl"}, args...)
	out, err := r.run(argv[0], argv[1:]...)
	if err != nil {
		return "", runtime.ErrWorkerControlUnavailable
	}
	return string(out), nil
}

// Inspect structured container state, never raw messages or logs.
func (r *kubectlRunner) workerReadiness(workerID string) error {
	var pod struct {
		Status struct {
			Containers []struct {
				Name  string `json:"name"`
				State struct {
					Waiting *struct {
						Reason string `json:"reason"`
					} `json:"waiting"`
				} `json:"state"`
			} `json:"containerStatuses"`
		} `json:"status"`
	}
	raw, err := r.runWithStdin(nil, "get", "pods", workerID, "--ignore-not-found=true", "-o", "json")
	if err != nil {
		return errors.New("worker readiness inspection unavailable")
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	} // Pod creation can follow assignment.
	if err := json.Unmarshal(raw, &pod); err != nil {
		return errors.New("worker readiness inspection unavailable")
	}
	for _, c := range pod.Status.Containers {
		if c.Name == "agent" && c.State.Waiting != nil {
			switch c.State.Waiting.Reason {
			case "ImagePullBackOff", "ErrImagePull", "InvalidImageName":
				return runtime.ErrWorkerImageUnavailable
			}
		}
	}
	return nil
}

func (r *kubectlRunner) run(verb string, extraArgs ...string) ([]byte, error) {
	out, err := r.runWithStdin(nil, append([]string{verb}, extraArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w\noutput: %s", verb, err, out)
	}
	return out, nil
}

func (r *kubectlRunner) runWithStdin(stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.command(ctx, stdin, r.baseArgs(args...)...)
}

func (r *kubectlRunner) baseArgs(args ...string) []string {
	base := []string{}
	if r.context != "" {
		base = append(base, "--context", r.context)
	}
	base = append(base, "--namespace", r.namespace)
	return append(base, args...)
}
