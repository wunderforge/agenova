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
	sqlstore "github.com/wunderforge/agenova/internal/history/sqlite"
	"path/filepath"
	"testing"
	"time"
)

type failingRepository struct {
	history.Repository
	kind string
}

func (r failingRepository) Append(f facts.Fact, s *v0.IssuedState) error {
	if f.Kind == r.kind {
		return history.ErrUnavailable
	}
	return r.Repository.Append(f, s)
}
func TestDurableServiceRestartAndDuplicateProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.db")
	r, err := sqlstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{}
	s, err := NewServiceWithOptions(b, e, p, app.ReferencePrincipalTeamA, Options{Repository: r})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(verticalRequest(t, "durable")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "durable")
	s.Close()
	r.Close()
	r, err = sqlstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s, err = NewServiceWithOptions(&verticalBackend{}, &verticalExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{Repository: r})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	recovered, err := s.QueryRequest("durable")
	if err != nil || recovered.Outcome == nil || recovered.Outcome.Status != v.Outcome.Status || len(recovered.Facts) != len(v.Facts) {
		t.Fatalf("lost evidence: %+v %v", recovered, err)
	}
	if _, ok := s.runner.ClaimAuthority(v.State.Claim.ID); ok {
		t.Fatal("restored authority")
	}
	if _, err = s.Submit(verticalRequest(t, "durable")); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
}
func TestStorageFailureStopsAllocationAndProvider(t *testing.T) {
	for _, kind := range []string{"RequestResolution", "ProviderAttempt"} {
		t.Run(kind, func(t *testing.T) {
			r, err := sqlstore.Open(filepath.Join(t.TempDir(), "evidence.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{}
			s, err := NewServiceWithOptions(b, e, p, app.ReferencePrincipalTeamA, Options{Repository: failingRepository{r, kind}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Submit(verticalRequest(t, "blocked"))
			if kind == "ProviderAttempt" {
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					s.mu.RLock()
					failed := s.storageFailed
					s.mu.RUnlock()
					if failed {
						break
					}
					time.Sleep(time.Millisecond)
				}
			}
			s.Close()
			if kind == "RequestResolution" && (err == nil || b.calls.Load() != 0) {
				t.Fatalf("allocation after failed admission: %v %d", err, b.calls.Load())
			}
			if p.calls.Load() != 0 {
				t.Fatal("provider called after evidence failure")
			}
			if _, err = s.QueryRequest("blocked"); !errors.Is(err, history.ErrUnavailable) {
				t.Fatalf("exposed uncommitted result: %v", err)
			}
		})
	}
}

func (r failingRepository) Create(v evidence.View, a history.Admission) error {
	if r.kind == "RequestResolution" {
		return history.ErrUnavailable
	}
	return r.Repository.Create(v, a)
}
