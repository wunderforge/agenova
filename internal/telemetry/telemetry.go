// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package telemetry emits allowlisted diagnostics. It never records payloads,
// raw errors, credentials or arbitrary metric labels.
package telemetry

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logapi "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	logSDK "go.opentelemetry.io/otel/sdk/log"
	metricSDK "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSDK "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"net/url"
	"sync/atomic"
	"time"
)

type Telemetry struct {
	traces    *traceSDK.TracerProvider
	metrics   *metricSDK.MeterProvider
	logs      *logSDK.LoggerProvider
	tracer    trace.Tracer
	logger    logapi.Logger
	calls     metric.Int64Counter
	decisions metric.Int64Counter
	duration  metric.Float64Histogram
	failures  *atomic.Int64
}
type traceExporter struct {
	traceSDK.SpanExporter
	failed *atomic.Int64
}

func (e traceExporter) ExportSpans(ctx context.Context, spans []traceSDK.ReadOnlySpan) error {
	err := e.SpanExporter.ExportSpans(ctx, spans)
	if err != nil {
		e.failed.Add(1)
	}
	return nil /* safe health replaces raw endpoint/error logging */
}

type metricExporter struct {
	metricSDK.Exporter
	failed *atomic.Int64
}

func (e metricExporter) Export(ctx context.Context, data *metricdata.ResourceMetrics) error {
	if e.Exporter.Export(ctx, data) != nil {
		e.failed.Add(1)
	}
	return nil
}

type logExporter struct {
	logSDK.Exporter
	failed *atomic.Int64
}

func (e logExporter) Export(ctx context.Context, records []logSDK.Record) error {
	if e.Exporter.Export(ctx, records) != nil {
		e.failed.Add(1)
	}
	return nil
}

// New explicitly configures OTLP/HTTP and overrides exporter headers from the
// ambient environment: no user/provider token is forwarded to telemetry.
func New(ctx context.Context, endpoint string) (*Telemetry, error) {
	if endpoint == "" {
		return nil, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid telemetry endpoint")
	}
	t := &Telemetry{failures: &atomic.Int64{}}
	res := resource.NewSchemaless(attribute.String("service.name", "agenova-control-plane"))
	te, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint), otlptracehttp.WithHeaders(map[string]string{}), otlptracehttp.WithTimeout(time.Second), otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}))
	if err != nil {
		return nil, errors.New("telemetry initialization failed")
	}
	t.traces = traceSDK.NewTracerProvider(traceSDK.WithResource(res), traceSDK.WithSampler(traceSDK.AlwaysSample()), traceSDK.WithBatcher(traceExporter{te, t.failures}, traceSDK.WithMaxQueueSize(256), traceSDK.WithMaxExportBatchSize(64), traceSDK.WithBatchTimeout(time.Second), traceSDK.WithExportTimeout(2*time.Second)))
	me, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint), otlpmetrichttp.WithHeaders(map[string]string{}), otlpmetrichttp.WithTimeout(time.Second), otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{Enabled: false}))
	if err != nil {
		t.traces.Shutdown(ctx)
		return nil, errors.New("telemetry initialization failed")
	}
	t.metrics = metricSDK.NewMeterProvider(metricSDK.WithResource(res), metricSDK.WithReader(metricSDK.NewPeriodicReader(metricExporter{me, t.failures}, metricSDK.WithInterval(5*time.Second), metricSDK.WithTimeout(2*time.Second))))
	le, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(endpoint), otlploghttp.WithURLPath("/v1/logs"), otlploghttp.WithHeaders(map[string]string{}), otlploghttp.WithTimeout(time.Second), otlploghttp.WithRetry(otlploghttp.RetryConfig{Enabled: false}))
	if err != nil {
		t.traces.Shutdown(ctx)
		t.metrics.Shutdown(ctx)
		return nil, errors.New("telemetry initialization failed")
	}
	t.logs = logSDK.NewLoggerProvider(logSDK.WithResource(res), logSDK.WithProcessor(logSDK.NewBatchProcessor(logExporter{le, t.failures}, logSDK.WithMaxQueueSize(256), logSDK.WithExportMaxBatchSize(64), logSDK.WithExportInterval(time.Second), logSDK.WithExportTimeout(2*time.Second))))
	t.tracer = t.traces.Tracer("agenova")
	t.logger = t.logs.Logger("agenova")
	meter := t.metrics.Meter("agenova")
	t.calls, _ = meter.Int64Counter("agenova.operations")
	t.decisions, _ = meter.Int64Counter("agenova.governance.decisions")
	t.duration, _ = meter.Float64Histogram("agenova.operation.duration", metric.WithUnit("s"))
	meter.Int64ObservableCounter("agenova.telemetry.export.failures", metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error { o.Observe(t.failures.Load()); return nil }))
	return t, nil
}

// Start accepts only the fixed operation vocabulary. Correlation IDs are span
// attributes, never metric labels. The completion callback accepts a boolean,
// so raw backend/provider errors cannot accidentally enter telemetry.
func (t *Telemetry) Start(ctx context.Context, operation, requestRef, claimID, invocationID string) (context.Context, func(bool)) {
	if t == nil {
		return ctx, func(bool) {}
	}
	switch operation {
	case "work", "admission", "runtime.allocate", "runtime.observe", "runtime.start", "runtime.terminate", "runtime.cleanup", "worker.execute", "model.invoke", "tool.invoke":
	default:
		operation = "unknown"
	}
	attrs := []attribute.KeyValue{attribute.String("agenova.request.ref", requestRef)}
	if claimID != "" {
		attrs = append(attrs, attribute.String("agenova.claim.id", claimID))
	}
	if invocationID != "" {
		attrs = append(attrs, attribute.String("agenova.invocation.id", invocationID))
	}
	ctx, span := t.tracer.Start(ctx, operation, trace.WithAttributes(attrs...))
	start := time.Now()
	return ctx, func(ok bool) {
		result := "succeeded"
		if !ok {
			result = "failed"
			span.SetStatus(codes.Error, "operation failed")
		}
		labels := metric.WithAttributes(attribute.String("operation", operation), attribute.String("result", result))
		t.calls.Add(ctx, 1, labels)
		t.duration.Record(ctx, time.Since(start).Seconds(), labels)
		var record logapi.Record
		record.SetTimestamp(time.Now())
		record.SetBody(logapi.StringValue("operation completed"))
		record.SetSeverity(logapi.SeverityInfo)
		record.AddAttributes(logapi.String("operation", operation), logapi.String("result", result), logapi.String("request_ref", requestRef), logapi.String("claim_id", claimID), logapi.String("invocation_id", invocationID))
		t.logger.Emit(ctx, record)
		span.End()
	}
}
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	return errors.Join(t.traces.Shutdown(ctx), t.metrics.Shutdown(ctx), t.logs.Shutdown(ctx))
}
func (t *Telemetry) Failures() int64 {
	if t == nil {
		return 0
	}
	return t.failures.Load()
}

// Decision records only a fixed interface/result vocabulary, without Policy or payload text.
func (t *Telemetry) Decision(ctx context.Context, component, result string) {
	if t == nil {
		return
	}
	switch component {
	case "admission", "model", "tool":
	default:
		return
	}
	switch result {
	case "Allow", "Deny", "ApprovalRequired":
	default:
		return
	}
	t.decisions.Add(ctx, 1, metric.WithAttributes(attribute.String("component", component), attribute.String("decision", result)))
	var record logapi.Record
	record.SetTimestamp(time.Now())
	record.SetBody(logapi.StringValue("governance decision"))
	record.SetSeverity(logapi.SeverityInfo)
	record.AddAttributes(logapi.String("component", component), logapi.String("decision", result))
	t.logger.Emit(ctx, record)
}
