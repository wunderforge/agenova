package operator

import (
	"testing"

	"github.com/donozhang1992/agenova/api/v1alpha1"
	"github.com/donozhang1992/agenova/internal/runtime"
	"github.com/donozhang1992/agenova/internal/runtime/contracttest"
)

func TestRuntimeBackendContract(t *testing.T) {
	contracttest.Run(t, newBackend)
}

func newBackend(t *testing.T) runtime.RuntimeBackend {
	t.Helper()

	r := NewRuntime()
	template := v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "python-agent-v1"},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image:   "example.local/agenova/python-agent:dev",
			Command: []string{"python", "/app/agent.py"},
		},
	}
	if err := r.AddTemplate(template); err != nil {
		t.Fatalf("add template: %v", err)
	}

	pool := v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "python-agent-pool"},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: "python-agent-v1",
			Replicas:    1,
		},
	}
	if err := r.AddWarmPool(pool); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	return r
}
