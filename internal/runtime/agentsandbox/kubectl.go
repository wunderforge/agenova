package agentsandbox

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// kubectlRunner executes kubectl commands against a specific context and
// namespace. It is the only mechanism this package uses to talk to Kubernetes,
// keeping the adapter free of k8s.io/client-go transitive dependencies during
// the spike phase.
type kubectlRunner struct {
	context   string
	namespace string
}

// applyBytes applies a manifest provided as raw bytes via stdin.
func (r *kubectlRunner) applyBytes(manifest []byte) error {
	args := r.baseArgs("apply", "-f", "-")
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(string(manifest))
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply: %w\noutput: %s", err, raw)
	}
	return nil
}

// get fetches a resource by group/version/kind and name, unmarshaling into dst.
func (r *kubectlRunner) get(resource, name string, dst any) error {
	raw, err := r.run("get", resource, name, "-o", "json")
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// delete deletes a resource by type and name.
func (r *kubectlRunner) delete(resource, name string) error {
	_, err := r.run("delete", resource, name, "--ignore-not-found=true")
	return err
}

// exists returns true if the resource is present in the cluster.
func (r *kubectlRunner) exists(resource, name string) (bool, error) {
	raw, err := r.run("get", resource, name, "-o", "name")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "NotFound") {
			return false, nil
		}
		return false, err
	}
	return len(strings.TrimSpace(string(raw))) > 0, nil
}

func (r *kubectlRunner) run(verb string, extraArgs ...string) ([]byte, error) {
	args := r.baseArgs(append([]string{verb}, extraArgs...)...)
	out, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w\noutput: %s", verb, err, out)
	}
	return out, nil
}

func (r *kubectlRunner) baseArgs(args ...string) []string {
	base := []string{"--context", r.context, "--namespace", r.namespace}
	return append(base, args...)
}
