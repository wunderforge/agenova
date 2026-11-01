// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package modelgateway

import (
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"sync"
	"sync/atomic"
	"testing"
)

type budgetProvider struct{ calls atomic.Int32 }

func (p *budgetProvider) Invoke(string, Request) error { p.calls.Add(1); return nil }

func TestBudgetAtomicallyProtectsActualProviderAcrossGatewayInstances(t *testing.T) {
	for _, limit := range []int{2, 8, 20} {
		t.Run(string(rune('A'+limit)), func(t *testing.T) {
			req := teamARequest(t)
			claims := gatewaytest.NewClaims()
			claims.Put(req.ClaimID, v0.ClaimPhaseRunning)
			a := claims.Authorities[req.ClaimID]
			a.Runtime.MaxModelCalls = limit
			claims.Authorities[req.ClaimID] = a
			journal := facts.NewJournal()
			_ = journal.RegisterRequest("work", v0.Principal{Subject: "test"})
			_ = journal.BindClaim("work", v0.SandboxClaim{ID: req.ClaimID, RequestRef: "work", AuthorityRef: "test"})
			provider := &budgetProvider{}
			reserve := WithCallBudget(func(_ Request, s app.ClaimAuthoritySnapshot, id string) (bool, error) {
				return journal.ReserveModelCall("work", req.ClaimID, id, s.EffectiveAuthority.Runtime.MaxModelCalls)
			})
			gateways := []*Gateway{NewGateway(claims, nil, nil, WithAdapter(provider), reserve), NewGateway(claims, nil, nil, WithAdapter(provider), reserve)}
			var wg sync.WaitGroup
			var denied atomic.Int32
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					d, e := gateways[i%2].Invoke(req)
					if e != nil {
						t.Error(e)
					}
					if d.Result == v0.DecisionResultDeny {
						if d.Category != "model-call-limit" {
							t.Errorf("wrong denial: %+v", d)
						}
						denied.Add(1)
					}
				}(i)
			}
			wg.Wait()
			if int(provider.calls.Load()) != limit || int(denied.Load()) != 40-limit || len(journal.ForRequest("work")) != limit {
				t.Fatal("budget exceeded or reservation lost", provider.calls.Load(), denied.Load())
			}
		})
	}
}
func TestBudgetRecordingFailureAndMissingAccountingFailClosed(t *testing.T) {
	for _, missing := range []bool{false, true} {
		req := teamARequest(t)
		claims := gatewaytest.NewClaims()
		claims.Put(req.ClaimID, v0.ClaimPhaseRunning)
		a := claims.Authorities[req.ClaimID]
		a.Runtime.MaxModelCalls = 2
		claims.Authorities[req.ClaimID] = a
		provider := &budgetProvider{}
		opts := []Option{WithAdapter(provider)}
		if !missing {
			opts = append(opts, WithCallBudget(func(Request, app.ClaimAuthoritySnapshot, string) (bool, error) {
				return false, errors.New("storage failed")
			}))
		}
		_, _ = NewGateway(claims, nil, nil, opts...).Invoke(req)
		if provider.calls.Load() != 0 {
			t.Fatal("dispatched without budget accounting")
		}
	}
}
