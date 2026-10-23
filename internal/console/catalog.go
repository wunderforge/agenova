// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	"sort"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

// CatalogEntry is a project-qualified registration, not an execution grant.
type CatalogEntry struct {
	Project   string            `json:"project"`
	Template  *v0.AgentTemplate `json:"template"`
	CanSubmit bool              `json:"canSubmit"`
}

func (s *Service) catalog(ctx context.Context, p v0.Principal) ([]CatalogEntry, error) {
	if s.setupFor == nil || s.access == nil {
		return nil, errors.New("authorized catalog unavailable")
	}
	base, err := s.setupFor(ctx, p, "", "")
	if err != nil {
		return nil, err
	}
	out := []CatalogEntry{}
	for _, project := range base.Projects {
		view, err := s.setupFor(ctx, p, project, "")
		if err != nil {
			return nil, err
		}
		for _, name := range view.Templates {
			if len(out) >= 200 {
				return nil, errors.New("catalog limit exceeded")
			}
			selected, err := s.setupFor(ctx, p, project, name)
			if err != nil {
				return nil, err
			}
			if selected.Template == nil || selected.Project != project || selected.Template.Metadata.Name != name {
				return nil, errors.New("catalog registration unavailable")
			}
			out = append(out, CatalogEntry{Project: project, Template: selected.Template, CanSubmit: s.access.Authorize(ctx, p, "work.submit", project) == nil})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Template.Metadata.Name == out[j].Template.Metadata.Name {
			return out[i].Project < out[j].Project
		}
		return out[i].Template.Metadata.Name < out[j].Template.Metadata.Name
	})
	return out, nil
}
