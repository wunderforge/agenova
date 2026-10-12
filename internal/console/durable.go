// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/workstore"
)

// DurableWork persists evidence behind the existing application boundary. It
// cannot issue authority, select a runtime or replay stored tasks.
type DurableWork interface {
	Reserve(context.Context, workstore.Snapshot) (bool, error)
	Existing(context.Context, v0.Principal, string, string) (evidence.View, bool, error)
	Append(context.Context, facts.Fact, *v0.IssuedState) error
	StageResult(context.Context, string, evidence.Outcome) error
	Finish(context.Context, evidence.View) error
	LastSequence(context.Context) (uint64, error)
	Pending(context.Context) ([]workstore.Snapshot, error)
	GetFor(context.Context, v0.Principal, string) (evidence.View, error)
	GetClaimFor(context.Context, v0.Principal, string) (evidence.View, error)
	ListFor(context.Context, v0.Principal) ([]evidence.View, error)
}
type submissionKey struct{ name, digest string }

var ErrRecoveryRequired = errors.New("unfinished durable Work requires fail-closed recovery before admission")

func (s *Service) configureDurability() error {
	if s.durable == nil {
		return nil
	}
	if s.prepareFor == nil {
		return errors.New("durable Work requires authenticated member mode")
	}
	pending, err := s.durable.Pending(context.Background())
	if err != nil {
		return err
	}
	for _, snapshot := range pending {
		// A crash after the terminal fact but before Finish only lost publication.
		// Repair that acknowledgement without replaying runtime/provider operations.
		view, ok := completedEvidence(snapshot)
		if !ok {
			return ErrRecoveryRequired
		}
		if err := s.durable.Finish(context.Background(), view); err != nil {
			return err
		}
	}
	last, err := s.durable.LastSequence(context.Background())
	if err != nil {
		return err
	}
	s.journal, err = facts.NewDurableJournal(last, func(f facts.Fact) error {
		if _, ok := s.persisted.Load(f.RequestRef); !ok {
			return nil
		} // admission commits its initial facts atomically
		return s.storageResult(s.durable.Append(context.Background(), f, nil))
	})
	return err
}
func (s *Service) storageResult(err error) error {
	if err != nil {
		s.storageFailed.Store(true)
		return workstore.ErrUnavailable
	}
	return nil
}
func (s *Service) persistAdmission(owner v0.Principal, key submissionKey, view evidence.View) error {
	if s.durable == nil {
		return nil
	}
	view.Facts = s.journal.ForRequest(view.RequestRef)
	existing, err := s.durable.Reserve(context.Background(), workstore.Snapshot{Owner: owner, InputName: key.name, InputDigest: key.digest, View: view})
	if err != nil {
		if errors.Is(err, workstore.ErrCapacity) {
			return ErrCapacity
		}
		if errors.Is(err, workstore.ErrConflict) {
			return ErrConflict
		}
		return s.storageResult(err)
	}
	if existing {
		return ErrConflict
	} // never dispatch an already committed admission
	s.persisted.Store(view.RequestRef, true)
	return nil
}
func (s *Service) persistFinished(ref string) error {
	if s.durable == nil {
		return nil
	}
	if s.storageFailed.Load() {
		return workstore.ErrUnavailable
	}
	view, err := s.QueryRequest(ref)
	if err != nil {
		return s.storageResult(err)
	}
	return s.storageResult(s.durable.Finish(context.Background(), view))
}
func (s *Service) ListForResult(p v0.Principal) ([]evidence.View, error) {
	if s.durable != nil {
		return s.durable.ListFor(context.Background(), p)
	}
	return s.ListFor(p), nil
}

func completedEvidence(snapshot workstore.Snapshot) (evidence.View, bool) {
	view := evidence.Clone(snapshot.View)
	if view.State == nil || v0.ValidateIssuedState(view.State) != nil || view.State.Claim == nil || len(view.Facts) == 0 {
		return evidence.View{}, false
	}
	last := view.Facts[len(view.Facts)-1]
	if last.Kind != "RunOutcome" || last.ClaimID != view.State.Claim.ID || last.RequestRef != view.RequestRef {
		return evidence.View{}, false
	}
	phase := view.State.Claim.Phase
	if phase != v0.ClaimPhaseSucceeded && phase != v0.ClaimPhaseFailed && phase != v0.ClaimPhaseExpired {
		return evidence.View{}, false
	}
	if last.Operation != string(phase) && !(last.Operation == "Cancelled" && phase == v0.ClaimPhaseFailed) {
		return evidence.View{}, false
	}
	view.Outcome = &evidence.Outcome{Status: last.Operation, Failure: last.Reason}
	if phase == v0.ClaimPhaseSucceeded {
		if snapshot.StagedResult == nil || snapshot.StagedResult.Status != "Succeeded" || snapshot.StagedResult.Text == "" || snapshot.StagedResult.Model == nil {
			return evidence.View{}, false
		}
		view.Outcome.Text = snapshot.StagedResult.Text
		view.Outcome.Model = snapshot.StagedResult.Model
	}
	return view, true
}
