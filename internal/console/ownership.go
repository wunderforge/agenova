// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"errors"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
)

// QueryFor checks ownership before observing any Work or runtime evidence.
// Foreign and nonexistent references return the same error.
func (s *Service) QueryFor(p v0.Principal, ref string, claim bool) (evidence.View, error) {
	if p.Subject == "" || p.Team == "" || p.AuthenticationContext != "oidc" || s.access == nil {
		return evidence.View{}, errors.New("record not found")
	}
	s.mu.RLock()
	key, project := "", ""
	own := false
	for candidate, r := range s.records {
		if r.owner.Team != p.Team || r.view.Request == nil {
			continue
		}
		match := candidate == ref
		if claim {
			match = r.view.State != nil && r.view.State.Claim != nil && r.view.State.Claim.ID == ref
		}
		if match {
			key, project, own = candidate, r.view.Request.Spec.ProjectRef, r.owner == p
			break
		}
	}
	s.mu.RUnlock()
	if key == "" {
		return evidence.View{}, errors.New("record not found")
	}
	// Never hold the execution/record lock across a database authorization read.
	allowed := own && s.access.Authorize(context.Background(), p, "work.read-own", project) == nil
	if !allowed {
		allowed = s.access.Authorize(context.Background(), p, "work.read-project", project) == nil
	}
	if !allowed {
		return evidence.View{}, errors.New("record not found")
	}
	return s.QueryRequest(key)
}

func (s *Service) ListFor(p v0.Principal) []evidence.View {
	s.mu.RLock()
	refs := []string{}
	for _, ref := range s.order {
		if s.records[ref].owner.Team == p.Team {
			refs = append(refs, ref)
		}
	}
	s.mu.RUnlock()
	views := []evidence.View{}
	for _, ref := range refs {
		if v, err := s.QueryFor(p, ref, false); err == nil {
			views = append(views, v)
		}
	}
	return views
}

// authorizeRun rechecks live member/project access at each governed operation.
// The existing claim gateway still independently enforces effective authority.
func (s *Service) authorizeRun(ctx context.Context, ref string) error {
	s.mu.RLock()
	r := s.records[ref]
	if r == nil || r.view.Request == nil {
		s.mu.RUnlock()
		return errors.New("work access unavailable")
	}
	p, project, sessionCheck := r.owner, r.view.Request.Spec.ProjectRef, r.sessionCheck
	s.mu.RUnlock()
	if s.prepareFor == nil {
		return nil
	}
	if s.access == nil {
		return errors.New("work access unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if sessionCheck != nil {
		if err := sessionCheck(ctx); err != nil {
			return err
		}
	}
	return s.access.Authorize(ctx, p, "work.submit", project)
}
