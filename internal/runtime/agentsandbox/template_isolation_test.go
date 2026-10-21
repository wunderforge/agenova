// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

type isolatedTemplateKube struct {
	templateCaptureKube
	mu      sync.Mutex
	objects map[string][]byte
}

func (k *isolatedTemplateKube) applyBytes(data []byte) error {
	var object struct {
		Kind     string
		Metadata struct{ Name string }
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.objects[object.Kind+"/"+object.Metadata.Name] = append([]byte(nil), data...)
	return nil
}

func imageTemplate(image string) *v0.AgentTemplate {
	limit := v0.Duration(time.Minute)
	return &v0.AgentTemplate{APIVersion: v0.AgentTemplateAPIVersion, Kind: v0.AgentTemplateKind, Metadata: v0.ObjectMeta{Name: "engineer"}, Spec: v0.AgentTemplateSpec{
		Artifact: &v0.AgentTemplateArtifact{Image: image}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agenova-workerctl", "serve"}},
		CapabilityCeiling: &v0.AgentTemplateCapabilityCeiling{MaxTimeout: &limit},
	}}
}

func TestSameNamedImmutableTemplatesNeverRepointEachOthersPool(t *testing.T) {
	k := &isolatedTemplateKube{objects: map[string][]byte{}}
	a := newSpikeAdapter(k, "isolated-projects")
	// Stand in for snapshots already resolved by two authorized project registries.
	first, second := imageTemplate("project-a/engineer:v1"), imageTemplate("project-b/engineer:v1")
	refA, err := a.ConfigureAgentTemplate(first)
	if err != nil {
		t.Fatal(err)
	}
	refB, err := a.ConfigureAgentTemplate(second)
	if err != nil {
		t.Fatal(err)
	}
	if refA == refB {
		t.Fatal("same display name collided across image snapshots")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		for _, pair := range []struct {
			template *v0.AgentTemplate
			ref      string
		}{{first, refA}, {second, refB}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ref, err := a.ConfigureAgentTemplate(pair.template)
				if err != nil || ref != pair.ref {
					t.Errorf("unstable content binding: ref=%s err=%v", ref, err)
				}
			}()
		}
	}
	wg.Wait()
	for ref, image := range map[string]string{refA: first.Spec.Artifact.Image, refB: second.Spec.Artifact.Image} {
		templateName, poolName := resourceName("tmpl", ref), resourceName("pool", "pool-"+ref)
		if len(templateName) > 63 || len(poolName)+6 > 63 {
			t.Fatal("invalid upstream resource name")
		}
		var template upstreamSandboxTemplate
		if err := json.Unmarshal(k.objects[kindSandboxTemplate+"/"+templateName], &template); err != nil {
			t.Fatal(err)
		}
		if template.Spec.PodTemplate.Spec.Containers[0].Image != image {
			t.Fatal("another project repointed the runtime image")
		}
		var pool upstreamSandboxWarmPool
		if err := json.Unmarshal(k.objects[kindSandboxWarmPool+"/"+poolName], &pool); err != nil {
			t.Fatal(err)
		}
		if pool.Spec.SandboxTemplateRef.Name != templateName {
			t.Fatal("warm pool points at another project's runtime template")
		}
	}
	if len(k.objects) != 4 {
		t.Fatalf("expected two immutable template/pool pairs, got %d", len(k.objects))
	}
}

func TestRuntimeTemplateIdentityCoversCommandAndCeilingNotJustImage(t *testing.T) {
	k := &isolatedTemplateKube{objects: map[string][]byte{}}
	a := newSpikeAdapter(k, "isolated-projects")
	template := imageTemplate("same/image:v1")
	base, err := a.ConfigureAgentTemplate(template)
	if err != nil {
		t.Fatal(err)
	}
	template.Spec.Entrypoint.Command = []string{"/custom-control", "serve"}
	commandRef, err := a.ConfigureAgentTemplate(template)
	if err != nil || commandRef == base {
		t.Fatalf("command not bound: %s %v", commandRef, err)
	}
	template.Spec.CapabilityCeiling.Tools = []string{"git.read"}
	ceilingRef, err := a.ConfigureAgentTemplate(template)
	if err != nil || ceilingRef == commandRef {
		t.Fatalf("immutable content not bound: %s %v", ceilingRef, err)
	}
	if _, err := a.ConfigureAgentTemplate(nil); err == nil {
		t.Fatal("missing template accepted")
	}
}
