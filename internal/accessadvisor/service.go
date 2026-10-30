// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package accessadvisor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/toolbackend"
)

type Error struct{ Code, Message string }

func (e *Error) Error() string          { return e.Message }
func reject(code, message string) error { return &Error{code, message} }

type Info struct {
	Enabled      bool     `json:"enabled"`
	AllowedModes []string `json:"allowedModes"`
	DefaultMode  string   `json:"defaultMode"`
	Engine       string   `json:"engine,omitempty"`
	CallsUsed    int      `json:"callsUsed"`
	MaxCalls     int      `json:"maxCalls"`
}
type Plan struct {
	Project      string           `json:"project"`
	Template     string           `json:"template"`
	ID           string           `json:"id"`
	Status       string           `json:"status"`
	IntentDigest string           `json:"intentDigest"`
	GuideDigest  string           `json:"guideDigest"`
	GuideVersion string           `json:"guideVersion"`
	ProfileID    string           `json:"profileID,omitempty"`
	Explanation  string           `json:"explanation"`
	Questions    []string         `json:"questions"`
	Engine       string           `json:"engine"`
	Model        *PlannerModel    `json:"model,omitempty"`
	Request      *v0.ClaimRequest `json:"request,omitempty"`
	Created      time.Time        `json:"created"`
	Expires      time.Time        `json:"expires"`
	owner        string
}
type entry struct {
	digest string
	plan   Plan
	err    error
	done   chan struct{}
}
type Service struct {
	cfg      v0.AccessAdvisorConfig
	selector Selector
	mu       sync.Mutex
	entries  map[string]*entry
	ids      map[string]*entry
	calls    int
	slots    chan struct{}
	now      func() time.Time
}

func New(c *v0.AccessAdvisorConfig, selector Selector) (*Service, error) {
	cfg, e := v0.NormalizeAccessAdvisor(c)
	if e != nil {
		return nil, e
	}
	if cfg.Enabled && selector == nil {
		return nil, errors.New("enabled Advisor needs an explicit selector")
	}
	return &Service{cfg: cfg, selector: selector, entries: map[string]*entry{}, ids: map[string]*entry{}, slots: make(chan struct{}, max(1, cfg.MaxConcurrent)), now: time.Now}, nil
}
func (s *Service) Info() Info {
	if s == nil {
		return Info{AllowedModes: []string{"manual"}, DefaultMode: "manual"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return Info{s.cfg.Enabled, append([]string(nil), s.cfg.AllowedModes...), s.cfg.DefaultMode, s.cfg.Engine, s.calls, s.cfg.MaxCalls}
}
func (s *Service) Allows(mode string) bool {
	if s == nil || !s.cfg.Enabled {
		return false
	}
	for _, m := range s.cfg.AllowedModes {
		if m == mode {
			return true
		}
	}
	return false
}
func Owner(p v0.Principal) string { return p.Team + "\x00" + p.Subject }
func IntentDigest(r *v0.ClaimRequest) string {
	b, _ := json.Marshal(r)
	var c v0.ClaimRequest
	_ = json.Unmarshal(b, &c)
	c.Spec.RequestedAccess = v0.ClaimRequestedAccess{}
	return Digest(c)
}
func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func HasAccess(a v0.ClaimRequestedAccess) bool {
	return len(a.Tools)+len(a.ResourceScopes)+len(a.MemoryScopes)+len(a.NetworkProfiles) > 0 || a.ModelProfile != ""
}
func clone(p Plan) Plan {
	owner := p.owner
	b, _ := json.Marshal(p)
	var c Plan
	_ = json.Unmarshal(b, &c)
	c.owner = owner
	return c
}

func (s *Service) Plan(ctx context.Context, p v0.Principal, r *v0.ClaimRequest, t *v0.AgentTemplate, catalog []toolbackend.Descriptor) (Plan, error) {
	if s == nil || !s.cfg.Enabled {
		return Plan{}, reject("advisor_disabled", "Access advice is disabled.")
	}
	if p.Subject == "" || p.Team == "" || r == nil || v0.ValidateClaimRequest(r) != nil || t == nil || r.Spec.TemplateRef != t.Metadata.Name || v0.ValidateAgentTemplate(t) != nil || t.Spec.AccessGuide == nil {
		return Plan{}, reject("invalid_advice_context", "A valid task and registered access guide are required.")
	}
	if HasAccess(r.Spec.RequestedAccess) {
		return Plan{}, reject("explicit_access", "Use normal submission for explicit permissions; advice never overwrites them.")
	}
	objective, ok := r.Spec.Task.Input["objective"].(string)
	if !ok || strings.TrimSpace(objective) == "" || len(objective) > 4096 {
		return Plan{}, reject("invalid_task", "Provide a task objective up to 4096 bytes.")
	}
	owner, digest := Owner(p), IntentDigest(r)
	key := owner + "\x00" + r.Metadata.Name
	s.mu.Lock()
	if prior := s.entries[key]; prior != nil {
		if prior.digest != digest {
			s.mu.Unlock()
			return Plan{}, reject("advice_conflict", "This request name already belongs to a different task.")
		}
		done := prior.done
		s.mu.Unlock()
		select {
		case <-done:
			return clone(prior.plan), prior.err
		case <-ctx.Done():
			return Plan{}, reject("advice_pending", "Planning remains in progress; query or retry the same name.")
		}
	}
	if len(s.entries) >= s.cfg.MaxRecords || s.calls >= s.cfg.MaxCalls {
		s.mu.Unlock()
		return Plan{}, reject("advisor_budget", "Advisor capacity or call limit reached; no task was started.")
	}
	select {
	case s.slots <- struct{}{}:
	default:
		s.mu.Unlock()
		return Plan{}, reject("advisor_busy", "Advisor is busy; no task was started.")
	}
	var id [16]byte
	if _, e := rand.Read(id[:]); e != nil {
		s.mu.Unlock()
		<-s.slots
		return Plan{}, reject("advisor_unavailable", "Advisor is unavailable.")
	}
	now := s.now()
	e := &entry{digest: digest, done: make(chan struct{}), plan: Plan{Project: r.Spec.ProjectRef, Template: r.Spec.TemplateRef, ID: "advice-" + hex.EncodeToString(id[:]), Status: "planning", IntentDigest: digest, GuideDigest: v0.AccessGuideDigest(t), GuideVersion: t.Spec.AccessGuide.Version, Engine: s.cfg.Engine, Created: now, Expires: now.Add(10 * time.Minute), Questions: []string{}, owner: owner}}
	s.entries[key] = e
	s.ids[e.plan.ID] = e
	s.calls++
	s.mu.Unlock()
	duration, _ := time.ParseDuration(s.cfg.Timeout)
	bounded, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	choice, err := s.selector.Select(bounded, SelectionContext{r.Spec.Task, r.Spec.ProjectRef, t.Spec.AccessGuide, t.Spec.CapabilityCeiling, catalog})
	if bounded.Err() != nil {
		err = bounded.Err()
	}
	result := clone(e.plan)
	switch {
	case err != nil:
		result.Status = "failed"
		err = reject("advisor_failed", "Advisor did not return a valid result; no task was started.")
	case len(choice.Explanation) > 1024 || len(choice.Questions) > 4 || len(choice.ProfileID) > 64:
		result.Status = "failed"
		err = reject("advisor_invalid", "Advisor output is outside its bounds.")
	default:
		for _, q := range choice.Questions {
			if len(q) > 256 {
				err = reject("advisor_invalid", "Advisor question is outside its bounds.")
			}
		}
		result.Explanation = choice.Explanation
		result.Questions = choice.Questions
		result.Model = choice.Model
		if result.Questions == nil {
			result.Questions = []string{}
		}
		if err != nil {
			result.Status = "failed"
		} else if choice.ProfileID == "" || len(choice.Questions) > 0 {
			result.Status = "clarification"
		} else {
			var profile *v0.TaskAccessProfile
			for i := range t.Spec.AccessGuide.Profiles {
				if t.Spec.AccessGuide.Profiles[i].ID == choice.ProfileID {
					profile = &t.Spec.AccessGuide.Profiles[i]
				}
			}
			if profile == nil {
				result.Status = "failed"
				err = reject("advisor_invalid", "Advisor selected an unknown access profile.")
			} else {
				available := true
				for _, op := range profile.RequestedAccess.Tools {
					matched := false
					for _, d := range catalog {
						if d.Operation == op {
							for _, scope := range profile.RequestedAccess.ResourceScopes {
								matched = matched || scope == d.ResourceScope
							}
						}
					}
					available = available && matched
				}
				if !available {
					result.Status = "failed"
					err = reject("capability_unavailable", "Selected access profile requires an unavailable tool or resource.")
				} else {
					b, _ := json.Marshal(r)
					var request v0.ClaimRequest
					_ = json.Unmarshal(b, &request)
					request.Spec.RequestedAccess = profile.RequestedAccess
					if request.Spec.RequestedAccess.ModelProfile == "" {
						request.Spec.RequestedAccess.ModelProfile = t.Spec.Defaults.ModelProfile
					}
					if v0.ValidateClaimRequest(&request) != nil {
						result.Status = "failed"
						err = reject("advisor_invalid", "The suggested request is invalid.")
					} else {
						result.Status = "ready"
						result.ProfileID = profile.ID
						result.Request = &request
					}
				}
			}
		}
	}
	s.mu.Lock()
	e.plan = result
	e.err = err
	close(e.done)
	s.mu.Unlock()
	<-s.slots
	return clone(result), err
}

func (s *Service) Get(p v0.Principal, id string) (Plan, error) {
	if s == nil {
		return Plan{}, reject("advisor_disabled", "Access advice is disabled.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.ids[id]
	if e == nil || e.plan.owner != Owner(p) {
		return Plan{}, reject("advice_not_found", "No advice was found for this caller.")
	}
	select {
	case <-e.done:
	default:
		return Plan{}, reject("advice_pending", "Planning is still in progress.")
	}
	if !s.now().Before(e.plan.Expires) {
		return Plan{}, reject("advice_expired", "Advice expired; request a new plan explicitly.")
	}
	return clone(e.plan), nil
}

// RequiredGranted is a preflight stop check, never an authority calculation.
func RequiredGranted(a v0.ClaimRequestedAccess, g *v0.EffectiveAuthority) bool {
	if g == nil {
		return false
	}
	for _, pair := range []struct{ want, got []string }{{a.Tools, g.Tools}, {a.ResourceScopes, g.ResourceScopes}, {a.MemoryScopes, g.MemoryScopes}, {a.NetworkProfiles, g.NetworkProfiles}} {
		for _, v := range pair.want {
			found := false
			for _, x := range pair.got {
				found = found || x == v
			}
			if !found {
				return false
			}
		}
	}
	return a.ModelProfile == "" || a.ModelProfile == g.ModelProfile
}
