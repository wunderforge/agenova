// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package telemetry

import (
	"context"
	logpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestThreeRealOTLPSignalsAndBoundedOutage(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("forwarded credential")
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen[r.URL.Path] = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	}))
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=secret-canary")
	ctx := context.Background()
	instrument, err := New(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	root, end := instrument.Start(ctx, "work", "request:one", "claim:one", "")
	instrument.Decision(root, "admission", "Deny")
	_, callEnd := instrument.Start(root, "model.invoke", "request:one", "claim:one", "invocation:one")
	callEnd(false)
	end(true)
	if TraceID(root) == "" {
		t.Fatal("missing trace correlation")
	}
	flush, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = instrument.Shutdown(flush); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	traces := &tracepb.ExportTraceServiceRequest{}
	metrics := &metricpb.ExportMetricsServiceRequest{}
	logs := &logpb.ExportLogsServiceRequest{}
	if proto.Unmarshal(seen["/v1/traces"], traces) != nil || len(traces.ResourceSpans) == 0 {
		t.Fatal("no traces")
	}
	if proto.Unmarshal(seen["/v1/metrics"], metrics) != nil || len(metrics.ResourceMetrics) == 0 {
		t.Fatal("no metrics")
	}
	if proto.Unmarshal(seen["/v1/logs"], logs) != nil || len(logs.ResourceLogs) == 0 {
		t.Fatal("no logs")
	}
	sawDecision := false
	for _, rm := range metrics.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "agenova.governance.decisions" {
					sawDecision = true
				}
				if h := m.GetHistogram(); h != nil {
					for _, dp := range h.DataPoints {
						for _, a := range dp.Attributes {
							if a.Key != "operation" && a.Key != "result" {
								t.Fatalf("unbounded metric label: %s", a.Key)
							}
						}
					}
				}
			}
		}
	}
	if !sawDecision {
		t.Fatal("governance decision metric missing")
	}
	server.Close()
	instrument, err = New(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, end := instrument.Start(ctx, "work", "req", "", "")
		end(true)
	}
	if time.Since(start) > time.Second {
		t.Fatal("export blocked execution")
	}
	shutdown, cancelShutdown := context.WithTimeout(ctx, 3*time.Second)
	defer cancelShutdown()
	instrument.Shutdown(shutdown)
}
