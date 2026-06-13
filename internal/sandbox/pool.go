package sandbox

import (
	"errors"
	"fmt"

	"github.com/donozhang1992/agenova/api/v1alpha1"
)

type Phase string

const (
	PhaseIdle      Phase = "Idle"
	PhaseBound     Phase = "Bound"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
)

type Sandbox struct {
	ID          string
	TemplateRef string
	ClaimID     string
	Phase       Phase
}

type WarmPool struct {
	Name        string
	TemplateRef string

	nextOrdinal   int
	replacedCount int
	sandboxes     []Sandbox
}

func NewWarmPool(pool v1alpha1.SandboxWarmPool) (*WarmPool, error) {
	if pool.Metadata.Name == "" {
		return nil, errors.New("warm pool name is required")
	}
	if pool.Spec.TemplateRef == "" {
		return nil, errors.New("warm pool template ref is required")
	}
	if pool.Spec.Replicas < 1 {
		return nil, errors.New("warm pool replicas must be at least 1")
	}

	warmPool := &WarmPool{
		Name:        pool.Metadata.Name,
		TemplateRef: pool.Spec.TemplateRef,
	}
	for i := 0; i < pool.Spec.Replicas; i++ {
		warmPool.addIdleSandbox()
	}
	return warmPool, nil
}

func (p *WarmPool) ClaimIdle(claimID string) (Sandbox, error) {
	if claimID == "" {
		return Sandbox{}, errors.New("claim id is required")
	}
	for i := range p.sandboxes {
		if p.sandboxes[i].Phase == PhaseIdle {
			p.sandboxes[i].Phase = PhaseBound
			p.sandboxes[i].ClaimID = claimID
			return p.sandboxes[i], nil
		}
	}
	return Sandbox{}, errors.New("no idle sandbox available")
}

func (p *WarmPool) MarkRunning(sandboxID, claimID string) error {
	return p.updateClaimSandbox(sandboxID, claimID, PhaseBound, PhaseRunning)
}

func (p *WarmPool) MarkSucceeded(sandboxID, claimID string) error {
	return p.updateClaimSandbox(sandboxID, claimID, PhaseRunning, PhaseSucceeded)
}

// MarkFailed accepts both Bound and Running sandboxes: a claim can fail after
// starting, or between bind and start when the sandbox is lost or unusable.
func (p *WarmPool) MarkFailed(sandboxID, claimID string) error {
	for i := range p.sandboxes {
		if p.sandboxes[i].ID != sandboxID {
			continue
		}
		if p.sandboxes[i].ClaimID != claimID {
			return errors.New("sandbox is bound to a different claim")
		}
		phase := p.sandboxes[i].Phase
		if phase != PhaseBound && phase != PhaseRunning {
			return fmt.Errorf("invalid sandbox transition %s -> %s", phase, PhaseFailed)
		}
		p.sandboxes[i].Phase = PhaseFailed
		return nil
	}
	return fmt.Errorf("sandbox not found: %s", sandboxID)
}

// Replace destroys a sandbox whose claim reached a terminal phase and puts a
// fresh idle sandbox in its slot, keeping the active pool size stable.
func (p *WarmPool) Replace(sandboxID, claimID string) (Sandbox, error) {
	for i := range p.sandboxes {
		if p.sandboxes[i].ID != sandboxID {
			continue
		}
		if p.sandboxes[i].ClaimID != claimID {
			return Sandbox{}, errors.New("sandbox is bound to a different claim")
		}
		phase := p.sandboxes[i].Phase
		if phase != PhaseSucceeded && phase != PhaseFailed {
			return Sandbox{}, fmt.Errorf("sandbox must be terminal before replace, got %s", phase)
		}
		p.replacedCount++
		replacement := p.newIdleSandbox()
		p.sandboxes[i] = replacement
		return replacement, nil
	}
	return Sandbox{}, fmt.Errorf("sandbox not found: %s", sandboxID)
}

func (p *WarmPool) Status() v1alpha1.SandboxWarmPoolStatus {
	var status v1alpha1.SandboxWarmPoolStatus
	for _, sandbox := range p.sandboxes {
		switch sandbox.Phase {
		case PhaseIdle:
			status.IdleSandboxes++
		case PhaseBound:
			status.BoundClaims++
		case PhaseRunning:
			status.RunningClaims++
		}
	}
	status.ReplacedSandboxes = p.replacedCount
	return status
}

func (p *WarmPool) Sandboxes() []Sandbox {
	sandboxes := make([]Sandbox, len(p.sandboxes))
	copy(sandboxes, p.sandboxes)
	return sandboxes
}

func (p *WarmPool) addIdleSandbox() Sandbox {
	sandbox := p.newIdleSandbox()
	p.sandboxes = append(p.sandboxes, sandbox)
	return sandbox
}

func (p *WarmPool) newIdleSandbox() Sandbox {
	p.nextOrdinal++
	return Sandbox{
		ID:          fmt.Sprintf("%s-%d", p.Name, p.nextOrdinal),
		TemplateRef: p.TemplateRef,
		Phase:       PhaseIdle,
	}
}

func (p *WarmPool) updateClaimSandbox(sandboxID, claimID string, from, to Phase) error {
	for i := range p.sandboxes {
		if p.sandboxes[i].ID != sandboxID {
			continue
		}
		if p.sandboxes[i].ClaimID != claimID {
			return errors.New("sandbox is bound to a different claim")
		}
		if p.sandboxes[i].Phase != from {
			return fmt.Errorf("invalid sandbox transition %s -> %s", p.sandboxes[i].Phase, to)
		}
		p.sandboxes[i].Phase = to
		return nil
	}
	return fmt.Errorf("sandbox not found: %s", sandboxID)
}
