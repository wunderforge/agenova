// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// defaultCommandTimeout bounds every single kubectl invocation so a hung
// subprocess cannot outlive the adapter's polling deadlines.
const defaultCommandTimeout = 30 * time.Second

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
	out, err := cmd.CombinedOutput()
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
	base := []string{"--context", r.context, "--namespace", r.namespace}
	return append(base, args...)
}
