// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platformapply

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
)

type fakeDeployment struct {
	applied       bool
	fail          error
	denied        error
	preflightRuns int
	applyRuns     int
}

func (f *fakeDeployment) Preflight(context.Context, DeploymentRequest) error {
	f.preflightRuns++
	return f.denied
}

func (f *fakeDeployment) Plan(_ context.Context, request DeploymentRequest) (string, []Change, []ComponentStatus, error) {
	if f.applied {
		return "fake/reference", nil, []ComponentStatus{{Name: "control-plane", Category: "deployment", State: "available", Reference: request.Platform.Revision}}, nil
	}
	return "fake/reference", []Change{{Component: "control-plane", Action: "create", Detail: "test"}}, []ComponentStatus{{Name: "control-plane", Category: "deployment", State: "unavailable"}}, nil
}

func (f *fakeDeployment) Apply(_ context.Context, request DeploymentRequest) ([]ComponentStatus, bool, error) {
	f.applyRuns++
	if f.fail != nil {
		return []ComponentStatus{{Name: "control-plane", Category: "deployment", State: "failed"}}, false, f.fail
	}
	f.applied = true
	return []ComponentStatus{{Name: "control-plane", Category: "deployment", State: "available", Reference: request.Platform.Revision}}, true, nil
}

func TestServicePlansActivatesAndAppliesIdempotently(t *testing.T) {
	deployment := &fakeDeployment{}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Plan(context.Background(), resolved, lock)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Target != "fake/reference" || len(plan.Changes) != 4 {
		t.Fatalf("first plan = %#v, want three adapter activations and one deployment change", plan)
	}
	result, err := service.Apply(context.Background(), resolved, lock)
	if err != nil || !result.Applied || !result.Ready {
		t.Fatalf("first apply = %#v, %v", result, err)
	}
	installed, err := service.Adapters.List()
	if err != nil || len(installed.Adapters) != 3 {
		t.Fatalf("installed adapters = %#v, %v", installed, err)
	}
	second, err := service.Apply(context.Background(), resolved, lock)
	if err != nil || second.Applied || !second.Ready || len(second.Plan.Changes) != 0 {
		t.Fatalf("second apply = %#v, %v", second, err)
	}
}

func TestServiceReturnsBoundedPartialFailure(t *testing.T) {
	service := newTestService(t, &fakeDeployment{fail: errors.New("target rejected deployment")})
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), resolved, lock)
	failed := false
	for _, component := range result.Components {
		failed = failed || component.State == "failed"
	}
	if err == nil || !result.Applied || !failed {
		t.Fatalf("Apply() = %#v, %v", result, err)
	}
}

func TestServiceKeepsAppliedFalseWhenTargetFailsBeforeMutation(t *testing.T) {
	deployment := &fakeDeployment{applied: true}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(context.Background(), resolved, lock); err != nil {
		t.Fatal(err)
	}
	deployment.applied = false
	deployment.fail = errors.New("target denied before mutation")
	result, err := service.Apply(context.Background(), resolved, lock)
	if err == nil || result.Applied {
		t.Fatalf("Apply() = %#v, %v, want no mutation reported", result, err)
	}
}

func TestServicePreflightsBeforeAdapterActivation(t *testing.T) {
	service := newTestService(t, &fakeDeployment{denied: errors.New("RBAC denied")})
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), resolved, lock)
	if err == nil || !strings.Contains(err.Error(), "RBAC denied") || result.Applied {
		t.Fatalf("Apply() = %#v, %v", result, err)
	}
	installed, err := service.Adapters.List()
	if err != nil || len(installed.Adapters) != 0 {
		t.Fatalf("preflight denial changed local adapter state: %#v, %v", installed, err)
	}
}

func TestServiceRejectsUnavailableInitialPolicy(t *testing.T) {
	service := newTestService(t, &fakeDeployment{})
	input := testPlatform()
	input.Spec.InitialPolicyRef.Version = "2"
	if _, _, err := service.Validate(input); err == nil || !strings.Contains(err.Error(), "is not available") {
		t.Fatalf("Validate() error = %v", err)
	}
}

type failSecondInstallStore struct {
	base  *adapterregistry.MemoryStore
	calls int
}

func (s *failSecondInstallStore) List() (adapterregistry.InstallationLock, error) {
	return s.base.List()
}

func (s *failSecondInstallStore) Install(adapter adapterregistry.InstalledAdapter) (bool, error) {
	s.calls++
	if s.calls == 2 {
		return false, errors.New("injected adapter store failure")
	}
	return s.base.Install(adapter)
}

func TestServiceReportsAdaptersActivatedBeforeLaterFailure(t *testing.T) {
	store := &failSecondInstallStore{base: adapterregistry.NewMemoryStore()}
	service := newTestServiceWithStore(t, &fakeDeployment{}, store)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Apply(context.Background(), resolved, lock)
	if err == nil || !result.Applied {
		t.Fatalf("Apply() = %#v, %v, want recorded partial activation", result, err)
	}
	states := map[string]string{}
	for _, status := range result.Components {
		states[status.Name] = status.State
	}
	if states["deployment"] != "used" || states["model"] != "failed" || states["runtime"] != "available" {
		t.Fatalf("partial adapter states = %#v", result.Components)
	}
}

func TestServiceRejectsApplyWhenConfirmedPlanChanges(t *testing.T) {
	deployment := &fakeDeployment{applied: true}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.Plan(context.Background(), resolved, lock)
	if err != nil {
		t.Fatal(err)
	}
	deployment.applied = false
	result, err := service.ApplyPlanned(context.Background(), resolved, lock, confirmed)
	if err == nil || !strings.Contains(err.Error(), "plan changed") || result.Applied || deployment.applied {
		t.Fatalf("ApplyPlanned() = %#v, %v, want new confirmation before mutation", result, err)
	}
}

func TestServiceActivatesAdaptersWithoutTouchingReadyTarget(t *testing.T) {
	deployment := &fakeDeployment{applied: true, denied: errors.New("no target mutation permission")}
	service := newTestService(t, deployment)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Plan(context.Background(), resolved, lock)
	if err != nil || !plan.Changed() || plan.targetChanged {
		t.Fatalf("adapter-only plan = %#v, %v", plan, err)
	}
	result, err := service.ApplyPlanned(context.Background(), resolved, lock, plan)
	if err != nil || !result.Applied || !result.Ready || deployment.preflightRuns != 0 || deployment.applyRuns != 0 {
		t.Fatalf("adapter-only apply = %#v, %v; preflight=%d apply=%d", result, err, deployment.preflightRuns, deployment.applyRuns)
	}
}

type tamperingStore struct {
	base   *adapterregistry.MemoryStore
	tamper bool
}

func (s *tamperingStore) List() (adapterregistry.InstallationLock, error) {
	lock, err := s.base.List()
	if err == nil && s.tamper && len(lock.Adapters) > 0 {
		lock.Adapters[0].Capabilities = []platform.Capability{platform.CapabilityModel}
	}
	return lock, err
}

func (s *tamperingStore) Install(adapter adapterregistry.InstalledAdapter) (bool, error) {
	return s.base.Install(adapter)
}

func TestServiceRejectsInstalledAdapterMetadataDrift(t *testing.T) {
	store := &tamperingStore{base: adapterregistry.NewMemoryStore()}
	service := newTestServiceWithStore(t, &fakeDeployment{applied: true}, store)
	resolved, lock, err := service.Validate(testPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(context.Background(), resolved, lock); err != nil {
		t.Fatal(err)
	}
	store.tamper = true
	if _, err := service.Plan(context.Background(), resolved, lock); err == nil || !strings.Contains(err.Error(), "does not match the bundled manifest") {
		t.Fatalf("Plan() error = %v, want exact installed metadata validation", err)
	}
}

func TestAllReadyFailsClosed(t *testing.T) {
	for _, state := range []string{"pending", "", "unknown", "failed", "unavailable"} {
		if allReady([]ComponentStatus{{State: state}}) {
			t.Fatalf("state %q reported Ready", state)
		}
	}
	for _, state := range []string{"available", "configured", "used"} {
		if !allReady([]ComponentStatus{{State: state}}) {
			t.Fatalf("state %q did not report Ready", state)
		}
	}
}

func TestServiceDeduplicatesAdapterAliasActivation(t *testing.T) {
	service := newTestService(t, &fakeDeployment{})
	input := testPlatform()
	input.Spec.Adapters = append(input.Spec.Adapters, v1alpha1.PlatformAdapterRequirement{Name: "model-alias", ID: "example.com/model/fake", Version: "1.0.0"})
	resolved, lock, err := service.Validate(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Plan(context.Background(), resolved, lock)
	if err != nil {
		t.Fatal(err)
	}
	activations, modelStatuses := 0, 0
	for _, change := range plan.Changes {
		if change.Component == "example.com/model/fake@1.0.0" {
			activations++
		}
	}
	for _, status := range plan.Components {
		if status.Name == "model" || status.Name == "model-alias" {
			modelStatuses++
		}
	}
	if activations != 1 || modelStatuses != 2 {
		t.Fatalf("alias plan = %#v, want one activation and two statuses", plan)
	}
}

func newTestService(t *testing.T, deployment DeploymentAdapter) Service {
	return newTestServiceWithStore(t, deployment, adapterregistry.NewMemoryStore())
}

func newTestServiceWithStore(t *testing.T, deployment DeploymentAdapter, store adapterregistry.Store) Service {
	t.Helper()
	registrations := []adapterregistry.Registration{
		testRegistration("example.com/deployment/fake", platform.CapabilityDeployment, func() (any, error) { return deployment, nil }),
		testRegistration("example.com/runtime/fake", platform.CapabilityRuntime, func() (any, error) { return &struct{}{}, nil }),
		testRegistration("example.com/model/fake", platform.CapabilityModel, func() (any, error) { return &struct{}{}, nil }),
	}
	registry, err := adapterregistry.New(registrations...)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, store)
	if err != nil {
		t.Fatal(err)
	}
	return Service{Adapters: lifecycle}
}

func testRegistration(id string, capability platform.Capability, factory adapterregistry.Factory) adapterregistry.Registration {
	descriptor := platform.Descriptor{ID: id, Version: "1.0.0", Capabilities: []platform.Capability{capability}, CanonicalizeInstance: func(_ platform.Capability, config map[string]any) (map[string]any, error) { return config, nil }}
	manifest := adapterregistry.Manifest{ID: id, Version: "1.0.0", Protocol: adapterregistry.ProtocolVersion, Capabilities: []platform.Capability{capability}, InstanceSchema: adapterregistry.ConfigSchema{}}
	if capability == platform.CapabilityRuntime || capability == platform.CapabilityModel {
		descriptor.CanonicalizeProfile = func(_ platform.Capability, _, config map[string]any) (map[string]any, error) { return config, nil }
		manifest.ProfileSchema = adapterregistry.ConfigSchema{}
	}
	return adapterregistry.Registration{Manifest: manifest, Descriptor: descriptor, Factories: map[platform.Capability]adapterregistry.Factory{capability: factory}}
}

func testPlatform() *v1alpha1.Platform {
	return &v1alpha1.Platform{
		APIVersion: v1alpha1.PlatformAPIVersion, Kind: v1alpha1.PlatformKind, Metadata: v1alpha1.ObjectMeta{Name: "reference"},
		Spec: v1alpha1.PlatformSpec{
			Adapters:         []v1alpha1.PlatformAdapterRequirement{{Name: "deployment", ID: "example.com/deployment/fake", Version: "1.0.0"}, {Name: "runtime", ID: "example.com/runtime/fake", Version: "1.0.0"}, {Name: "model", ID: "example.com/model/fake", Version: "1.0.0"}},
			Infrastructure:   v1alpha1.PlatformInfrastructure{Deployment: &v1alpha1.PlatformInstance{Name: "control", AdapterRef: "deployment", Config: map[string]any{"target": "reference"}}, RuntimeBackends: []v1alpha1.PlatformInstance{{Name: "runtime", AdapterRef: "runtime"}}, RuntimeProfiles: []v1alpha1.PlatformProfile{{Name: "standard", BackendRef: "runtime"}}},
			Services:         v1alpha1.PlatformServices{ModelBackends: []v1alpha1.PlatformInstance{{Name: "model", AdapterRef: "model"}}, ModelProfiles: []v1alpha1.PlatformProfile{{Name: "coding", BackendRef: "model"}}},
			InitialPolicyRef: &v1alpha1.PlatformPolicyReference{ID: ReferencePolicyID, Version: ReferencePolicyVersion},
		},
	}
}
