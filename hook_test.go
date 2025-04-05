package otelzlog

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adreasnow/otelstack"
	"github.com/pkg/errors"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"

	otelLogGlobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupOTEL(ctx context.Context) (func(), error) {
	shutdown := func() {}
	otelResources, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", os.Getenv("OTEL_SERVICE_NAME"))))
	if err != nil {
		log.Ctx(ctx).Error().Ctx(ctx).Err(err).Msg("error setting up resources")
		return shutdown, err
	}

	{ // set up otel tracer
		exporter, err := otlptracegrpc.New(ctx)
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
		exporter, err := otlploggrpc.New(ctx)
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

func setupOTELSack(t *testing.T) (stack *otelstack.Stack) {
	t.Helper()
	stack = otelstack.New(false, true, true)
	shutdownStack, err := stack.Start(t.Context())
	require.NoError(t, err, "must be able to start otelstack")
	stack.SetTestEnvGRPC(t)

	t.Setenv("OTEL_SERVICE_NAME", "test-service")

	t.Cleanup(func() {
		if err := shutdownStack(context.Background()); err != nil {
			t.Logf("error shutting down the stack: %v", err)
		}
	})

	shutdown, err := setupOTEL(t.Context())
	require.NoError(t, err, "must be able to set up OTEL logger")
	t.Cleanup(shutdown)
	return
}

func sendTestEvents(ctx context.Context, t *testing.T) (spanID string, traceID string) {
	t.Helper()

	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "test.segment")
	span.SetAttributes(attribute.String("test", "test"))
	log.Ctx(ctx).Info().Ctx(ctx).Str("test.string", "test-value").Msg("test log")
	spanID = span.SpanContext().SpanID().String()
	traceID = span.SpanContext().TraceID().String()
	span.End()

	time.Sleep(time.Second * 3)
	return
}

func checkEvents(t *testing.T, stack *otelstack.Stack, spanID string, traceID string) {
	events, err := stack.Seq.GetEvents(1, 10)
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

func TestHook(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		stack := setupOTELSack(t)

		ctx := log.
			Hook(&Hook{}).
			WithContext(t.Context())

		spanID, traceID := sendTestEvents(ctx, t)

		checkEvents(t, stack, spanID, traceID)
	})

	t.Run("error", func(t *testing.T) {
		stack := setupOTELSack(t)
		fmt.Println("http://localhost:" + stack.Jaeger.Ports[16686].Port())
		fmt.Println("http://localhost:" + stack.Seq.Ports[80].Port())

		ctx := log.
			Hook(&Hook{}).
			WithContext(t.Context())

		tracer := otel.Tracer("")
		func() {
			ctx, span := tracer.Start(ctx, "test.segment")
			span.SetAttributes(attribute.String("test", "test"))
			defer span.End()
			func() {
				ctx, span := tracer.Start(ctx, "test.segment")
				span.SetAttributes(attribute.String("test", "test"))
				defer span.End()

				log.Ctx(ctx).Error().Ctx(ctx).
					Err(errors.WithMessage(errors.New("hook: an error ocxurred"), "hook: an error occurred in a lower down function")).
					Str("test.string", "test-value").
					Msg("test log")
				// spanID := span.SpanContext().SpanID().String()
				// traceID := span.SpanContext().TraceID().String()

			}()
		}()

		// checkEvents(t, stack, spanID, traceID)

		time.Sleep(time.Minute * 3)
	})

	t.Run("panic", func(t *testing.T) {
		stack := setupOTELSack(t)
		fmt.Println("http://localhost:" + stack.Jaeger.Ports[16686].Port())

		ctx := log.
			Hook(&Hook{}).
			WithContext(t.Context())

		spanID, traceID := sendTestEvents(ctx, t)

		checkEvents(t, stack, spanID, traceID)

		time.Sleep(time.Minute * 3)
	})
}
