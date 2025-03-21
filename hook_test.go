package otelzlog

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/adreasnow/otelstack"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"

	otelLogGlobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestHook(t *testing.T) {
	stack := otelstack.Stack{}
	shutdownStack, err := stack.Start(t.Context())
	require.NoError(t, err, "must be able to start otelstack")
	t.Cleanup(func() { shutdownStack(context.Background()) })

	otelResources, err := resource.New(t.Context(), resource.WithAttributes(attribute.String("service.name", os.Getenv("OTEL_SERVICE_NAME"))))
	require.NoError(t, err, "must be able to set up resources")
	{ // set up otel tracer
		exporter, err := otlptracegrpc.New(t.Context())
		require.NoError(t, err, "must be able to set up trace exporter")

		otel.SetTracerProvider(
			sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(otelResources),
			),
		)

		t.Cleanup(func() { exporter.Shutdown(context.Background()) })
	}

	{ // set up otel logger
		exporter, err := otlploggrpc.New(t.Context())
		require.NoError(t, err, "must be able to set up log exporter")

		otelLogGlobal.SetLoggerProvider(
			sdklog.NewLoggerProvider(
				sdklog.WithProcessor(
					sdklog.NewBatchProcessor(exporter),
				),
				sdklog.WithResource(otelResources),
			),
		)

		t.Cleanup(func() { exporter.Shutdown(context.Background()) })
	}

	ctx := log.
		Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		Hook(&Hook{}).
		WithContext(t.Context())

	var spanID string
	var traceID string
	var span trace.Span
	{ // send log
		tracer := otel.Tracer(os.Getenv("OTEL_SERVICE_NAME"))
		ctx, span = tracer.Start(ctx, "test.segment")
		span.SetAttributes(attribute.String("test", "test"))
		log.Ctx(ctx).Info().Ctx(ctx).Str("test.string", "test-value").Msg("test log")
		spanID = span.SpanContext().SpanID().String()
		traceID = span.SpanContext().TraceID().String()
		span.End()
	}

	time.Sleep(time.Second * 3)

	events, err := stack.Seq.GetEvents(ctx)
	require.NoError(t, err, "must be able to get events from seq")

	require.Len(t, events, 1)
	require.Len(t, events[0].MessageTemplateTokens, 1)
	assert.Equal(t, "test log", events[0].MessageTemplateTokens[0].Text)

	m := map[string]any{}
	for _, kv := range events[0].Properties {
		m[kv.Name] = kv.Value
	}

	// test.string becomes a map
	assert.Equal(t, map[string]any{"string": any("test-value")}, m["test"])

	assert.Equal(t, traceID, m["TraceId"])
	assert.Equal(t, spanID, m["SpanId"])
}
