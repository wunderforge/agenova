// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/history"
)

func (s *Service) configureRepository() error {
	if s.repository == nil {
		return nil
	}
	if err := s.repository.Recover(); err != nil {
		return err
	}
	seq, err := s.repository.LastSequence()
	if err != nil {
		return err
	}
	s.journal, err = facts.NewDurableJournal(seq, func(f facts.Fact) error {
		if f.Kind == "RequestReceived" || f.Kind == "RequestResolution" {
			return nil
		}
		err := s.repository.Append(f, nil)
		if err != nil {
			s.failStorage()
		}
		return err
	})
	return err
}
func (s *Service) persistInitial(p app.PreparedAssignment, owner v0.Principal, key submissionKey, view evidence.View) error {
	if s.repository == nil {
		return s.persistAdmission(owner, key, view, history.Admission{Principal: p.Admission.Principal, Policy: p.Policy, Template: p.Template, Revision: s.revision})
	}
	view.Facts = s.journal.ForRequest(view.RequestRef)
	err := s.repository.Create(view, history.Admission{Principal: p.Admission.Principal, Policy: p.Policy, Template: p.Template, Revision: s.revision})
	if err != nil {
		s.failStorage()
		if errors.Is(err, history.ErrConflict) {
			return ErrConflict
		}
	}
	return err
}
