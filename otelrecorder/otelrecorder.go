// Package otelrecorder provides a utility for recording OpenTelemetry logs and traces in tests.
package otelrecorder

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/logtest"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type Recorder struct {
	LogProvider    *logtest.Recorder
	TracerProvider *trace.TracerProvider
	TraceRecorder  *tracetest.InMemoryExporter
}

func NewRecorder() *Recorder {
	r := &Recorder{}

	r.LogProvider = logtest.NewRecorder()
	r.TraceRecorder = tracetest.NewInMemoryExporter()

	r.TracerProvider = trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSyncer(r.TraceRecorder),
		trace.WithResource(resource.Empty()),
	)

	return r
}

func (r *Recorder) Cleanup() {
	if err := r.TraceRecorder.Shutdown(context.Background()); err != nil {
		fmt.Printf("error shutting down otel trace exporter: %v\n", err)
	}
	if err := r.TracerProvider.Shutdown(context.Background()); err != nil {
		fmt.Printf("error shutting down otel trace provider: %v\n", err)
	}
}

func (r *Recorder) GetLogs() []logtest.Record {
	records := []logtest.Record{}

	for _, recorded := range r.LogProvider.Result() {
		records = append(records, recorded...)
	}

	return records
}

func AttributeKVListToMap(attrs []attribute.KeyValue) map[string]attribute.Value {
	out := map[string]attribute.Value{}

	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value
	}
	return out
}

func LogKVListToMap(attrs []log.KeyValue) map[string]log.Value {
	out := map[string]log.Value{}

	for _, attr := range attrs {
		out[attr.Key] = attr.Value
	}
	return out
}

func (r *Recorder) GetSpans() map[string]tracetest.SpanStub {
	out := map[string]tracetest.SpanStub{}
	for _, span := range r.TraceRecorder.GetSpans() {
		out[span.Name] = span
	}

	return out
}
