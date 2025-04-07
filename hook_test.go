package otelzlog

import (
	"context"
	"testing"
	"time"

	"github.com/adreasnow/otelstack"
	"github.com/adreasnow/otelstack/jaeger"
	"github.com/adreasnow/otelstack/seq"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"

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

var serviceName = "test-service"

func setupOTEL(ctx context.Context, port nat.Port) (func(), error) {
	shutdown := func() {}
	otelResources, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", serviceName)))
	if err != nil {
		log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error setting up resources")
		return shutdown, err
	}

	{ // set up otel tracer
		exporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint("localhost:"+port.Port()),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error setting up trace exporter")
			return shutdown, err
		}

		otel.SetTracerProvider(
			sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(otelResources),
			),
		)

		shutdown = func() {
			if err := exporter.Shutdown(context.Background()); err != nil {
				log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error shutting down the trace exporter")
			}
		}
	}

	{ // set up otel logger
		exporter, err := otlploggrpc.New(ctx,
			otlploggrpc.WithEndpoint("localhost:"+port.Port()),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error setting up log exporter")
			return shutdown, err
		}

		otelLogGlobal.SetLoggerProvider(
			sdklog.NewLoggerProvider(
				sdklog.WithProcessor(
					sdklog.NewSimpleProcessor(exporter),
				),
				sdklog.WithResource(otelResources),
			),
		)

		shutdown = func() {
			if err := exporter.Shutdown(context.Background()); err != nil {
				log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error shutting down the trace exporter")
			}
			if err := exporter.Shutdown(context.Background()); err != nil {
				log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error shutting down the log exporter")
			}
		}
	}
	return shutdown, nil
}

func setupOTELStack(t *testing.T) (stack *otelstack.Stack) {
	t.Helper()
	stack = otelstack.New(false, true, true)
	shutdownStack, err := stack.Start(t.Context())
	require.NoError(t, err, "must be able to start otelstack")

	t.Cleanup(func() {
		if err := shutdownStack(context.Background()); err != nil {
			t.Logf("error shutting down the stack: %v", err)
		}
	})

	shutdown, err := setupOTEL(t.Context(), stack.Collector.Ports[4317])
	require.NoError(t, err, "must be able to set up OTEL logger")
	t.Cleanup(shutdown)
	return
}

func sendTestEvents(ctx context.Context, t *testing.T) (spanID string, traceID string) {
	t.Helper()

	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "test.segment")
	span.SetAttributes(attribute.String("test-attribute-value", "test-attribute-vale"))
	log.Ctx(ctx).Info().Ctx(ctx).Str("test-key", "test-value").Msg("test log")
	spanID = span.SpanContext().SpanID().String()
	traceID = span.SpanContext().TraceID().String()
	span.End()

	time.Sleep(time.Second * 3)
	return
}

func checkEvents(t *testing.T, stack *otelstack.Stack, spanID string, traceID string) {
	events, _, err := stack.Seq.GetEvents(1, 10)
	require.NoError(t, err, "must be able to get events from seq")

	traces, _, err := stack.Jaeger.GetTraces(1, 10, serviceName)
	require.NoError(t, err, "must be able to get events from seq")

	{ // check logs
		require.Len(t, events, 1)
		require.Len(t, events[0].Messages, 1)
		assert.Equal(t, "test log", events[0].Messages[0].Text)

		assert.Equal(t, "INFO", events[0].Level)

		assert.Equal(t, traceID, events[0].TraceID)
		assert.Equal(t, spanID, events[0].SpanID)

		assert.Contains(t, events[0].Properties, seq.Property{
			Name:  "test-key",
			Value: "test-value",
		})
		assert.Contains(t, events[0].Properties, seq.Property{
			Name:  "level",
			Value: "info",
		})

		assert.Equal(t, seq.Resource{
			Name: "service",
			Value: struct {
				Name string `json:"name"`
			}{Name: serviceName},
		}, events[0].Resource[0])

	}

	{ // check traces
		require.Len(t, traces, 1)
		require.Len(t, traces[0].Spans, 1)
		assert.Equal(t, "test.segment", traces[0].Spans[0].OperationName)

		assert.Equal(t, traceID, traces[0].Spans[0].TraceID)
		assert.Equal(t, spanID, traces[0].Spans[0].SpanID)

		require.Len(t, traces[0].Spans[0].Tags, 3)
		assert.Contains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
			Key:   "otel.scope.name",
			Type:  "string",
			Value: serviceName,
		})
		assert.Contains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
			Key:   "test-attribute-value",
			Type:  "string",
			Value: "test-attribute-vale",
		})

		require.Len(t, traces[0].Spans[0].Logs, 1)
		require.Len(t, traces[0].Spans[0].Logs[0].Fields, 4)
		assert.Contains(t, traces[0].Spans[0].Logs[0].Fields, jaeger.KeyValue{
			Key:   "event",
			Type:  "string",
			Value: "test log",
		})
		assert.Contains(t, traces[0].Spans[0].Logs[0].Fields, jaeger.KeyValue{
			Key:   "level",
			Type:  "string",
			Value: "info",
		})
		assert.Contains(t, traces[0].Spans[0].Logs[0].Fields, jaeger.KeyValue{
			Key:   "test-key",
			Type:  "string",
			Value: "test-value",
		})
	}
}

func TestHook(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{}).
			WithContext(t.Context())

		spanID, traceID := sendTestEvents(ctx, t)

		checkEvents(t, stack, spanID, traceID)
	})

	t.Run("error", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{}).
			WithContext(t.Context())

		tracer := otel.Tracer(serviceName)
		var parentSpan trace.Span
		var childSpan trace.Span
		var testErr error
		func() {
			ctx, parentSpan = tracer.Start(ctx, "segment.parent")
			defer parentSpan.End()
			func() {
				ctx, childSpan = tracer.Start(ctx, "segment.child")
				defer childSpan.End()

				testErr = errors.WithMessage(errors.New("hook: an error occurred"), "hook: an error occurred in a lower down function")
				log.Ctx(ctx).Error().Ctx(ctx).
					Err(testErr).
					Msg("test log")
			}()
		}()

		time.Sleep(time.Second * 3)

		events, _, err := stack.Seq.GetEvents(1, 10)
		require.NoError(t, err, "must be able to get events from seq")

		traces, _, err := stack.Jaeger.GetTraces(1, 10, serviceName)
		require.NoError(t, err, "must be able to get events from jaeger")

		{ // check logs
			require.Len(t, events, 1)
			require.Len(t, events[0].Messages, 1)
			assert.Equal(t, "test log", events[0].Messages[0].Text)

			assert.Equal(t, "ERROR", events[0].Level)

			assert.Equal(t, childSpan.SpanContext().TraceID().String(), events[0].TraceID)
			assert.Equal(t, childSpan.SpanContext().SpanID().String(), events[0].SpanID)

			require.Len(t, events[0].Properties, 3)
			assert.Contains(t, events[0].Properties, seq.Property{
				Name:  "level",
				Value: "error",
			})
			assert.Contains(t, events[0].Properties, seq.Property{
				Name:  "error",
				Value: testErr.Error(),
			})
		}

		{ // check traces
			require.Len(t, traces, 1)
			require.Len(t, traces[0].Spans, 2)
			spanMap := map[string]jaeger.Span{
				traces[0].Spans[0].OperationName: traces[0].Spans[0],
				traces[0].Spans[1].OperationName: traces[0].Spans[1],
			}
			assert.Contains(t, spanMap, "segment.parent")
			assert.Contains(t, spanMap, "segment.child")

			assert.Equal(t, parentSpan.SpanContext().TraceID().String(), spanMap["segment.parent"].TraceID)
			assert.Equal(t, parentSpan.SpanContext().SpanID().String(), spanMap["segment.parent"].SpanID)
			assert.Equal(t, childSpan.SpanContext().TraceID().String(), spanMap["segment.child"].TraceID)
			assert.Equal(t, childSpan.SpanContext().SpanID().String(), spanMap["segment.child"].SpanID)

			require.Len(t, spanMap["segment.child"].References, 1)
			assert.Equal(t, jaeger.Reference{
				RefType: "CHILD_OF",
				TraceID: spanMap["segment.parent"].TraceID,
				SpanID:  spanMap["segment.parent"].SpanID,
			}, spanMap["segment.child"].References[0])

			{ // child span
				require.Len(t, spanMap["segment.child"].Tags, 2)
				assert.Contains(t, spanMap["segment.child"].Tags, jaeger.KeyValue{
					Key:   "otel.scope.name",
					Type:  "string",
					Value: serviceName,
				})

				require.Len(t, spanMap["segment.child"].Logs, 2)
				require.Len(t, spanMap["segment.child"].Logs[0].Fields, 3)
				assert.Contains(t, spanMap["segment.child"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "event",
					Type:  "string",
					Value: "exception",
				})
				assert.Contains(t, spanMap["segment.child"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "exception.message",
					Type:  "string",
					Value: testErr.Error(),
				})
				assert.Contains(t, spanMap["segment.child"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "exception.type",
					Type:  "string",
					Value: "*errors.errorString",
				})
			}

			{ // parent span
				require.Len(t, spanMap["segment.parent"].Tags, 2)
				assert.Contains(t, spanMap["segment.parent"].Tags, jaeger.KeyValue{
					Key:   "otel.scope.name",
					Type:  "string",
					Value: serviceName,
				})
			}
		}
	})
}
