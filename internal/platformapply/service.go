// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package platformapply validates, plans, and reconciles one declarative
// Platform without knowing which provider implements its deployment target.
package platformapply

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
)

const (
	ReferencePolicyID      = "reference-default-deny"
	ReferencePolicyVersion = "1"
)

type Change struct {
	Component string `json:"component"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
}

type ComponentStatus struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	State     string `json:"state"`
	Reference string `json:"reference,omitempty"`
}

type Plan struct {
	PlatformName  string            `json:"platformName"`
	Revision      string            `json:"revision"`
	Target        string            `json:"target"`
	Changes       []Change          `json:"changes"`
	Components    []ComponentStatus `json:"components"`
	targetChanged bool
}

func (p Plan) Changed() bool { return len(p.Changes) > 0 }

type ApplyResult struct {
	Plan       Plan              `json:"plan"`
	Applied    bool              `json:"applied"`
	Ready      bool              `json:"ready"`
	Components []ComponentStatus `json:"components"`
}

type DeploymentRequest struct {
	Platform *platform.ResolvedPlatform
	Lock     *platform.PlatformLock
	Config   map[string]any
}

// DeploymentAdapter owns target vocabulary and mutations. The application
// layer sees only a stable target label, plan changes, and safe status.
type DeploymentAdapter interface {
	Plan(context.Context, DeploymentRequest) (target string, changes []Change, status []ComponentStatus, err error)
	Preflight(context.Context, DeploymentRequest) error
	Apply(context.Context, DeploymentRequest) ([]ComponentStatus, error)
}

type Service struct {
	Adapters *adapterregistry.Lifecycle
}

func (s Service) ValidateFile(path string) (*platform.ResolvedPlatform, *platform.PlatformLock, error) {
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read Platform file: %w", err)
	}
	parsed, validationErr := v1alpha1.ParsePlatformYAML(document)
	if validationErr != nil {
		return nil, nil, fmt.Errorf("validate Platform: %w", validationErr)
	}
	return s.Validate(parsed)
}

func (s Service) Validate(input *v1alpha1.Platform) (*platform.ResolvedPlatform, *platform.PlatformLock, error) {
	if s.Adapters == nil {
		return nil, nil, fmt.Errorf("adapter lifecycle is required")
	}
	resolved, lock, resolveErr := platform.Resolve(input, s.Adapters)
	if resolveErr != nil {
		return nil, nil, fmt.Errorf("resolve Platform: %w", resolveErr)
	}
	if resolved.InitialPolicyRef.ID != ReferencePolicyID || resolved.InitialPolicyRef.Version != ReferencePolicyVersion {
		return nil, nil, fmt.Errorf("initial policy %s@%s is not available; reference install requires %s@%s", resolved.InitialPolicyRef.ID, resolved.InitialPolicyRef.Version, ReferencePolicyID, ReferencePolicyVersion)
	}
	return resolved, lock, nil
}

func (s Service) PlanFile(ctx context.Context, path string) (Plan, error) {
	resolved, lock, err := s.ValidateFile(path)
	if err != nil {
		return Plan{}, err
	}
	return s.Plan(ctx, resolved, lock)
}

func (s Service) Plan(ctx context.Context, resolved *platform.ResolvedPlatform, lock *platform.PlatformLock) (Plan, error) {
	request, adapter, err := s.deployment(resolved, lock)
	if err != nil {
		return Plan{}, err
	}
	target, targetChanges, targetStatus, err := adapter.Plan(ctx, request)
	if err != nil {
		return Plan{}, fmt.Errorf("plan deployment target %s: %w", target, err)
	}
	changes, statuses, err := s.activationPlan(resolved.Adapters)
	if err != nil {
		return Plan{}, err
	}
	changes = append(changes, targetChanges...)
	statuses = append(statuses, targetStatus...)
	if changes == nil {
		changes = []Change{}
	}
	if statuses == nil {
		statuses = []ComponentStatus{}
	}
	sortPlan(changes, statuses)
	return Plan{PlatformName: resolved.PlatformName, Revision: resolved.Revision, Target: target, Changes: changes, Components: statuses, targetChanged: len(targetChanges) > 0}, nil
}

func (s Service) ApplyFile(ctx context.Context, path string) (ApplyResult, error) {
	resolved, lock, err := s.ValidateFile(path)
	if err != nil {
		return ApplyResult{}, err
	}
	return s.Apply(ctx, resolved, lock)
}

func (s Service) Apply(ctx context.Context, resolved *platform.ResolvedPlatform, lock *platform.PlatformLock) (ApplyResult, error) {
	plan, err := s.Plan(ctx, resolved, lock)
	if err != nil {
		return ApplyResult{}, err
	}
	return s.applyWithPlan(ctx, resolved, lock, plan)
}

// ApplyPlanned applies only the exact target plan previously shown to the
// operator. A concurrent target change requires a fresh plan and confirmation.
func (s Service) ApplyPlanned(ctx context.Context, resolved *platform.ResolvedPlatform, lock *platform.PlatformLock, confirmed Plan) (ApplyResult, error) {
	current, err := s.Plan(ctx, resolved, lock)
	if err != nil {
		return ApplyResult{}, err
	}
	if !reflect.DeepEqual(current, confirmed) {
		return ApplyResult{Plan: current, Components: current.Components}, fmt.Errorf("Platform plan changed before apply; review the new plan and retry")
	}
	return s.applyWithPlan(ctx, resolved, lock, current)
}

func (s Service) applyWithPlan(ctx context.Context, resolved *platform.ResolvedPlatform, lock *platform.PlatformLock, plan Plan) (ApplyResult, error) {
	if !plan.Changed() {
		return ApplyResult{Plan: plan, Ready: allReady(plan.Components), Components: plan.Components}, nil
	}
	var request DeploymentRequest
	var adapter DeploymentAdapter
	var err error
	if plan.targetChanged {
		request, adapter, err = s.deployment(resolved, lock)
		if err != nil {
			return ApplyResult{Plan: plan}, err
		}
		// Target authority is checked before even the local adapter lock changes.
		// The deployment adapter still rechecks at mutation time to narrow TOCTOU.
		if err := adapter.Preflight(ctx, request); err != nil {
			return ApplyResult{Plan: plan}, fmt.Errorf("preflight target %s: %w", plan.Target, err)
		}
	}
	activated := false
	for _, requirement := range resolved.Adapters {
		installation, err := s.Adapters.Install(requirement.ID + "@" + requirement.Version)
		if err != nil {
			_, statuses, statusErr := s.activationPlan(resolved.Adapters)
			if statusErr != nil {
				return ApplyResult{Plan: plan, Applied: activated}, fmt.Errorf("activate adapter %s@%s: %w; inspect partial adapter state: %v", requirement.ID, requirement.Version, err, statusErr)
			}
			for index := range statuses {
				if statuses[index].Name == requirement.Name && statuses[index].Category == "adapter" {
					statuses[index].State = "failed"
				}
			}
			sortPlan(nil, statuses)
			return ApplyResult{Plan: plan, Applied: activated, Components: statuses}, fmt.Errorf("activate adapter %s@%s: %w", requirement.ID, requirement.Version, err)
		}
		activated = activated || installation.Changed
	}
	_, adapterStatuses, statusErr := s.activationPlan(resolved.Adapters)
	if statusErr != nil {
		return ApplyResult{Plan: plan, Applied: true}, statusErr
	}
	if !plan.targetChanged {
		statuses := append(adapterStatuses, targetStatuses(plan.Components)...)
		sortPlan(nil, statuses)
		return ApplyResult{Plan: plan, Applied: activated, Ready: allReady(statuses), Components: statuses}, nil
	}
	statuses, err := adapter.Apply(ctx, request)
	statuses = append(adapterStatuses, statuses...)
	sortPlan(nil, statuses)
	if err != nil {
		return ApplyResult{Plan: plan, Applied: true, Components: statuses}, fmt.Errorf("apply Platform revision %s: %w", resolved.Revision, err)
	}
	return ApplyResult{Plan: plan, Applied: true, Ready: allReady(statuses), Components: statuses}, nil
}

func targetStatuses(components []ComponentStatus) []ComponentStatus {
	statuses := make([]ComponentStatus, 0, len(components))
	for _, component := range components {
		if component.Category != "adapter" {
			statuses = append(statuses, component)
		}
	}
	return statuses
}

func (s Service) deployment(resolved *platform.ResolvedPlatform, lock *platform.PlatformLock) (DeploymentRequest, DeploymentAdapter, error) {
	if resolved == nil || lock == nil {
		return DeploymentRequest{}, nil, fmt.Errorf("resolved Platform and lock are required")
	}
	var deployment platform.ResolvedInstance
	found := false
	for _, instance := range resolved.Instances {
		if instance.Category == platform.CapabilityDeployment {
			deployment, found = instance, true
			break
		}
	}
	if !found {
		return DeploymentRequest{}, nil, fmt.Errorf("resolved Platform has no deployment instance")
	}
	var requirement platform.ResolvedAdapter
	for _, candidate := range resolved.Adapters {
		if candidate.Name == deployment.AdapterRef {
			requirement = candidate
			break
		}
	}
	if requirement.ID == "" {
		return DeploymentRequest{}, nil, fmt.Errorf("deployment adapter requirement %q is unavailable", deployment.AdapterRef)
	}
	implementation, err := s.Adapters.Construct(requirement.ID, requirement.Version, platform.CapabilityDeployment)
	if err != nil {
		return DeploymentRequest{}, nil, err
	}
	adapter, ok := implementation.(DeploymentAdapter)
	if !ok {
		return DeploymentRequest{}, nil, fmt.Errorf("deployment adapter %s@%s does not implement the Platform deployment protocol", requirement.ID, requirement.Version)
	}
	request := DeploymentRequest{Platform: resolved, Lock: lock, Config: cloneMap(deployment.Config)}
	return request, adapter, nil
}

func (s Service) activationPlan(requirements []platform.ResolvedAdapter) ([]Change, []ComponentStatus, error) {
	lock, err := s.Adapters.List()
	if err != nil {
		return nil, nil, fmt.Errorf("read active adapters: %w", err)
	}
	installed := make(map[string]struct{}, len(lock.Adapters))
	versions := make(map[string]string, len(lock.Adapters))
	for _, item := range lock.Adapters {
		installed[item.ID+"@"+item.Version] = struct{}{}
		versions[item.ID] = item.Version
	}
	var changes []Change
	var statuses []ComponentStatus
	for _, requirement := range requirements {
		if version, ok := versions[requirement.ID]; ok && version != requirement.Version {
			return nil, nil, fmt.Errorf("adapter %s already active at version %s; requested version %s requires an explicit upgrade", requirement.ID, version, requirement.Version)
		}
		ref := requirement.ID + "@" + requirement.Version
		state := "available"
		if _, ok := installed[ref]; ok {
			if _, err := s.Adapters.Inspect(ref); err != nil {
				return nil, nil, fmt.Errorf("inspect active adapter %s: %w", ref, err)
			}
			state = "configured"
			if hasCapability(requirement.Capabilities, platform.CapabilityDeployment) {
				state = "used"
			}
		} else {
			changes = append(changes, Change{Component: ref, Action: "activate", Detail: "activate exact bundled adapter version"})
		}
		statuses = append(statuses, ComponentStatus{Name: requirement.Name, Category: "adapter", State: state, Reference: ref})
	}
	return changes, statuses, nil
}

func hasCapability(values []platform.Capability, wanted platform.Capability) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sortPlan(changes []Change, statuses []ComponentStatus) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Component == changes[j].Component {
			return changes[i].Action < changes[j].Action
		}
		return changes[i].Component < changes[j].Component
	})
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Category == statuses[j].Category {
			return statuses[i].Name < statuses[j].Name
		}
		return statuses[i].Category < statuses[j].Category
	})
}

func allReady(statuses []ComponentStatus) bool {
	if len(statuses) == 0 {
		return false
	}
	for _, status := range statuses {
		switch status.State {
		case "available", "configured", "used":
		default:
			return false
		}
	}
	return true
}

func EncodeLock(lock *platform.PlatformLock) (string, error) {
	data, err := json.Marshal(lock)
	if err != nil {
		return "", fmt.Errorf("encode Platform lock: %w", err)
	}
	return string(data), nil
}

func cloneMap(input map[string]any) map[string]any {
	data, _ := json.Marshal(input)
	var output map[string]any
	_ = json.Unmarshal(data, &output)
	return output
}

func SafeTarget(contextName, namespace string) string {
	return strings.TrimSpace(contextName) + "/" + strings.TrimSpace(namespace)
}
