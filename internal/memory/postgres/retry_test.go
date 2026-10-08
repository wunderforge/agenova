// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/memory"
)

type retryStateReader struct {
	state    *v0.IssuedState
	deadline time.Time
}

func (r retryStateReader) State(id string) *v0.IssuedState {
	if id != r.state.Claim.ID {
		return nil
	}
	return r.state
}
func (r retryStateReader) ClaimDeadline(id string) (time.Time, bool) {
	return r.deadline, id == r.state.Claim.ID
}

func TestGovernedRetryReusesPostgresReceiptAfterLostCommitAcknowledgement(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{"lost acknowledgement", errors.New("private-lost-commit-acknowledgement")},
		{"SQLSTATE 40003", stateError("40003")},
		{"wrapped SQLSTATE 40003", fmt.Errorf("private credential detail: %w", stateError("40003"))},
	} {
		t.Run(test.name, func(t *testing.T) { testGovernedReceiptRetry(t, test.err) })
	}
}

func testGovernedReceiptRetry(t *testing.T, commitErr error) {
	t.Helper()
	data, err := os.ReadFile("../../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	state, validationErr := v0.ParseSystemIssuedState(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	state.EffectiveAuthority.MemoryScopes = []string{namespace.Scope}
	state.EffectiveAuthority.MemoryOperations = []string{v0.MemoryWrite}
	body := "private-service-postgres-retry-sentinel"
	var receiptID string
	in := memory.WriteInput{ClaimID: state.Claim.ID, Body: body}
	steps := writeSteps(t, in)
	steps[len(steps)-1].check = func(_ context.Context, args []driver.NamedValue) {
		receiptID, _ = args[3].Value.(string)
		if receiptID == "" || args[5].Value != reference {
			t.Fatal("missing original receipt")
		}
	}
	encoded, _ := json.Marshal([]string{in.ClaimID, in.Body})
	digest := sha256.Sum256(encoded)
	steps = append(steps, tenant(t, namespace), step{query: `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`}, step{
		query: receiptSQL,
		rows:  [][]driver.Value{{hex.EncodeToString(digest[:]), reference, body, state.Claim.ID, time.Unix(100, 0)}},
		check: func(_ context.Context, args []driver.NamedValue) {
			if args[3].Value != receiptID {
				t.Fatal("governed retry queried a new receipt")
			}
		},
	})
	b, script := backend(t, steps...)
	script.commitErr = commitErr
	journal := facts.NewJournal()
	if err := journal.RegisterRequest(state.RequestRef, state.Principal); err != nil {
		t.Fatal(err)
	}
	if err := journal.BindClaim(state.RequestRef, *state.Claim); err != nil {
		t.Fatal(err)
	}
	service, err := memory.New(retryStateReader{state: state, deadline: time.Now().Add(time.Minute)}, journal, b, []memory.Namespace{namespace})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Bind(context.Background(), state.Claim.ID, *state.Claim.BackendIdentity)
	if err != nil {
		t.Fatal(err)
	}
	first, err := session.Invoke(context.Background(), memory.Request{ClaimID: state.Claim.ID, Operation: v0.MemoryWrite, Scope: namespace.Scope, Body: body})
	if err != nil || first.Status != memory.WriteUncertain || first.Retry == nil || receiptID != first.InvocationID {
		t.Fatal("lost acknowledgement was not preserved")
	}
	second, err := session.RetryWrite(context.Background(), first.Retry)
	if err != nil || second.Status != memory.Written || second.Reference != reference || second.InvocationID == first.InvocationID {
		t.Fatal("retry failed to resolve the committed receipt")
	}
	if script.commits != 1 || script.rollbacks != 1 || len(script.begins) != 2 || len(script.steps) != 0 {
		t.Fatal("retry inserted or committed a second record instead of reading the receipt")
	}
	got := journal.ForClaim(state.Claim.ID)
	if len(got) != 6 {
		t.Fatal("SQL replay lost unique audit attempts")
	}
	for i, result := range []memory.Result{first, second} {
		decision, attempt, outcome := got[i*3], got[i*3+1], got[i*3+2]
		if decision.Kind != "MemoryDecision" || decision.Result != v0.DecisionResultAllow || attempt.Kind != "ProviderAttempt" || outcome.Kind != "ProviderOutcome" || outcome.Memory == nil || outcome.Memory.Status != string(result.Status) {
			t.Fatal("commit classification lost correlated outcome metadata")
		}
		if decision.InvocationID != result.InvocationID || attempt.InvocationID != result.InvocationID || outcome.InvocationID != result.InvocationID {
			t.Fatal("retry reused an audit attempt instead of only the receipt")
		}
	}
	export, err := json.Marshal(struct {
		Results []memory.Result
		Facts   []facts.Fact
	}{[]memory.Result{first, second}, got})
	if err != nil || strings.Contains(string(export), "private") {
		t.Fatal("commit error, body or retry state leaked")
	}
}
