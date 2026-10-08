// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/memory"
)

type step struct {
	query string
	rows  [][]driver.Value
	err   error
	check func(context.Context, []driver.NamedValue)
}
type script struct {
	t                  *testing.T
	mu                 sync.Mutex
	steps              []step
	connects, queries  int
	begins             []driver.TxOptions
	commits, rollbacks int
	commitErr          error
}
type connector struct{ s *script }

func (c connector) Connect(context.Context) (driver.Conn, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.connects++
	return connection{c.s}, nil
}
func (c connector) Driver() driver.Driver { return testDriver{} }

type testDriver struct{}

func (testDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("test connector required")
}

type connection struct{ s *script }

func (c connection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c connection) Close() error { return nil }
func (c connection) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c connection) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.begins = append(c.s.begins, opts)
	return transaction{c.s}, nil
}
func (c connection) next(ctx context.Context, query string, args []driver.NamedValue) step {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.queries++
	if len(c.s.steps) == 0 {
		c.s.t.Errorf("unexpected SQL: %s", query)
		return step{err: errFailed}
	}
	s := c.s.steps[0]
	c.s.steps = c.s.steps[1:]
	if query != s.query {
		c.s.t.Errorf("SQL mismatch: %s", query)
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > memory.CallTimeout {
		c.s.t.Error("unbounded database operation")
	}
	if s.check != nil {
		s.check(ctx, args)
	}
	return s
}
func (c connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	s := c.next(ctx, query, args)
	return driver.RowsAffected(1), s.err
}
func (c connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s := c.next(ctx, query, args)
	return &testRows{rows: s.rows}, s.err
}

type transaction struct{ s *script }

func (tx transaction) Commit() error {
	tx.s.mu.Lock()
	defer tx.s.mu.Unlock()
	tx.s.commits++
	return tx.s.commitErr
}
func (tx transaction) Rollback() error {
	tx.s.mu.Lock()
	defer tx.s.mu.Unlock()
	tx.s.rollbacks++
	return nil
}

type testRows struct {
	rows  [][]driver.Value
	index int
}

func (r *testRows) Columns() []string {
	n := 4
	if len(r.rows) > 0 {
		n = len(r.rows[0])
	}
	columns := make([]string, n)
	for i := range columns {
		columns[i] = "column"
	}
	return columns
}
func (r *testRows) Close() error { return nil }
func (r *testRows) Next(values []driver.Value) error {
	if r.index == len(r.rows) {
		return io.EOF
	}
	copy(values, r.rows[r.index])
	r.index++
	return nil
}

func backend(t *testing.T, steps ...step) (*Backend, *script) {
	t.Helper()
	s := &script{t: t, steps: steps}
	db := sql.OpenDB(connector{s})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		if len(s.steps) != 0 {
			t.Errorf("%d SQL steps unexecuted", len(s.steps))
		}
	})
	return &Backend{db: db}, s
}

var namespace = memory.Namespace{Team: "team-a", Project: "payments", Scope: "docs"}

const reference = "memory:00000000000000000000000000000001"

func tenant(t *testing.T, n memory.Namespace) step {
	return step{query: tenantSQL, check: func(_ context.Context, args []driver.NamedValue) {
		t.Helper()
		if len(args) != 3 || args[0].Value != n.Team || args[1].Value != n.Project || args[2].Value != n.Scope {
			t.Fatal("ownership was not transaction-local and bound")
		}
	}}
}
func TestReadinessRejectsUnsafeRolesAndSchema(t *testing.T) {
	for disabled := -1; disabled < 4; disabled++ {
		flags := []driver.Value{true, true, true, true}
		if disabled >= 0 {
			flags[disabled] = false
		}
		b, _ := backend(t, step{query: readinessSQL, rows: [][]driver.Value{flags}})
		_, err := New(context.Background(), b.db)
		if (disabled == -1 && err != nil) || (disabled >= 0 && !errors.Is(err, memory.ErrUnavailable)) {
			t.Fatalf("flags %v: %v", flags, err)
		}
	}
	if _, err := New(context.Background(), nil); !errors.Is(err, memory.ErrUnavailable) {
		t.Fatal("missing pool accepted")
	}
}

func writeSteps(t *testing.T, in memory.WriteInput) []step {
	return []step{tenant(t, namespace), {query: `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`},
		{query: receiptSQL},
		{query: insertSQL, rows: [][]driver.Value{{reference, in.Body, in.ClaimID, time.Unix(100, 0)}}, check: func(_ context.Context, args []driver.NamedValue) {
			if args[4].Value != in.Body || args[5].Value != in.ClaimID {
				t.Fatal("write payload was not parameterized")
			}
		}},
		{query: receiptInsertSQL, check: func(_ context.Context, args []driver.NamedValue) {
			if args[3].Value != in.InvocationID || args[5].Value != reference {
				t.Fatal("receipt was not correlated")
			}
		}}}
}

func TestWriteCommitsRowAndReceiptTogetherAndSanitizesCommitFault(t *testing.T) {
	in := memory.WriteInput{InvocationID: "inv:trusted", ClaimID: "claim:trusted", Body: "% _ SQL-like ' content"}
	for _, commitErr := range []error{nil, errors.New("private connection credential error"), stateError("40001"), stateError("23514")} {
		b, s := backend(t, writeSteps(t, in)...)
		s.commitErr = commitErr
		record, err := b.Write(context.Background(), namespace, in)
		if commitErr == nil && (err != nil || record.Reference != reference) {
			t.Fatalf("write: %+v %v", record, err)
		}
		if commitErr != nil && err == nil {
			t.Fatal("commit failure hidden")
		}
		if _, ok := commitErr.(stateError); ok && !errors.Is(err, errFailed) {
			t.Fatalf("definite rollback: %v", err)
		}
		_, serverRollback := commitErr.(stateError)
		if commitErr != nil && !serverRollback && !errors.Is(err, memory.ErrWriteUncertain) {
			t.Fatalf("lost acknowledgement: %v", err)
		}
		if err != nil && strings.Contains(err.Error(), "credential") {
			t.Fatal("raw driver error escaped")
		}
		if s.commits != 1 || len(s.begins) != 1 || s.begins[0].ReadOnly {
			t.Fatal("write transaction missing")
		}
	}
}

func TestReceiptReplayNeverWritesAndDifferentBodyFails(t *testing.T) {
	in := memory.WriteInput{InvocationID: "inv:trusted", ClaimID: "claim:trusted", Body: "private body"}
	encoded, _ := json.Marshal([]string{in.ClaimID, in.Body})
	digest := sha256.Sum256(encoded)
	for _, same := range []bool{true, false} {
		stored := hex.EncodeToString(digest[:])
		if !same {
			stored = strings.Repeat("0", 64)
		}
		b, s := backend(t, tenant(t, namespace), step{query: `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`}, step{query: receiptSQL, rows: [][]driver.Value{{stored, reference, in.Body, in.ClaimID, time.Unix(100, 0)}}})
		r, err := b.Write(context.Background(), namespace, in)
		if same && (err != nil || r.Reference != reference) {
			t.Fatalf("replay: %+v %v", r, err)
		}
		if !same && !errors.Is(err, errFailed) {
			t.Fatal("different content reused receipt")
		}
		if s.commits != 0 || s.rollbacks != 1 {
			t.Fatal("replay inserted or left transaction open")
		}
	}
}

func TestReceiptFailureRollsBackWithoutCommit(t *testing.T) {
	in := memory.WriteInput{InvocationID: "inv", ClaimID: "claim", Body: "body"}
	steps := writeSteps(t, in)
	steps[len(steps)-1].err = stateError("23514")
	b, s := backend(t, steps...)
	if _, err := b.Write(context.Background(), namespace, in); !errors.Is(err, errFailed) {
		t.Fatalf("receipt failure: %v", err)
	}
	if s.commits != 0 || s.rollbacks != 1 {
		t.Fatal("row committed without receipt")
	}
}

func TestSearchUsesLiteralParametersReadonlyTransactionAndResetsOwnership(t *testing.T) {
	other := memory.Namespace{Team: "team-b", Project: "other", Scope: "docs"}
	query := `% _ ' OR true -- \\`
	check := func(_ context.Context, args []driver.NamedValue) {
		if args[3].Value != query || args[4].Value != int64(memory.DefaultLimit) {
			t.Fatal("query or limit not bound literally")
		}
	}
	b, s := backend(t, tenant(t, namespace), step{query: searchSQL, rows: [][]driver.Value{{reference, "stored", "prior-claim", time.Unix(100, 0)}}, check: check}, tenant(t, other), step{query: searchSQL, check: check})
	first, err := b.Search(context.Background(), namespace, memory.SearchInput{InvocationID: "inv:1", Query: query})
	if err != nil || len(first) != 1 || first[0].Namespace != namespace {
		t.Fatalf("search: %+v %v", first, err)
	}
	second, err := b.Search(context.Background(), other, memory.SearchInput{InvocationID: "inv:2", Query: query})
	if err != nil || len(second) != 0 {
		t.Fatalf("empty: %+v %v", second, err)
	}
	if len(s.begins) != 2 || !s.begins[0].ReadOnly || !s.begins[1].ReadOnly || s.commits != 2 {
		t.Fatal("search is not readonly/isolated")
	}
}

type stateError string

func (e stateError) Error() string    { return "private sql detail" }
func (e stateError) SQLState() string { return string(e) }

func TestInvalidInputAndTypedFaults(t *testing.T) {
	b, s := backend(t)
	if _, err := b.Write(context.Background(), namespace, memory.WriteInput{Body: strings.Repeat("x", memory.MaxBodyBytes+1)}); err == nil {
		t.Fatal("invalid write accepted")
	}
	if _, err := b.Search(context.Background(), namespace, memory.SearchInput{InvocationID: "inv", Query: "x", Limit: 11}); err == nil {
		t.Fatal("invalid search accepted")
	}
	if len(s.begins) != 0 {
		t.Fatal("invalid input reached pool")
	}
	for _, test := range []struct{ in, errorClass error }{{context.Canceled, context.Canceled}, {context.DeadlineExceeded, context.DeadlineExceeded}, {stateError("08006"), memory.ErrUnavailable}, {stateError("42601"), errFailed}, {io.EOF, memory.ErrUnavailable}} {
		if !errors.Is(fault(test.in), test.errorClass) {
			t.Fatalf("wrong error class: %v", test.in)
		}
	}
}

func TestMigrationScopeAndDenyByDefault(t *testing.T) {
	sql := MigrationSQL()
	for _, required := range []string{"FORCE ROW LEVEL SECURITY", "current_setting('agenova.team', true)", "current_setting('agenova.project', true)", "current_setting('agenova.scope', true)", "FOREIGN KEY (team, project, scope, memory_id)", "REVOKE ALL"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %s", required)
		}
	}
	if strings.Contains(sql, "GRANT ALL") || strings.Contains(sql, "CREATE ROLE") {
		t.Fatal("migration invents standing credentials/authority")
	}
}
