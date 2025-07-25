package otelzlog

import (
	"testing"

	"github.com/adreasnow/otelzlog/otelrecorder"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

var serviceName = "test-service"

func sendTestEvents(t *testing.T, logger *zerolog.Logger, recorder *otelrecorder.Recorder) (spanID string, traceID string) {
	t.Helper()

	tracer := recorder.TracerProvider.Tracer(serviceName)

	ctx, span := tracer.Start(t.Context(), "test.segment")
	span.SetAttributes(attribute.String("test-attribute-key", "test-attribute-value"))

	logger.Info().Ctx(ctx).Str("test-key", "test-value").Msg("test log")

	spanID = span.SpanContext().SpanID().String()
	traceID = span.SpanContext().TraceID().String()

	span.End()

	return
}

func checkEvents(t *testing.T, recorder *otelrecorder.Recorder, spanID string, traceID string) {
	logs := recorder.GetLogs()
	spans := recorder.GetSpans()

	{ // check logs
		require.Len(t, logs, 1)
		assert.Equal(t, "test log", logs[0].Body.String())

		assert.Equal(t, log.SeverityInfo, logs[0].Severity)

		attrMap := otelrecorder.LogKVListToMap(logs[0].Attributes)

		assert.Equal(t, "test-value", attrMap["test-key"].AsString())

		assert.Equal(t, spanID, trace.SpanContextFromContext(logs[0].Context).SpanID().String())
		assert.Equal(t, traceID, trace.SpanContextFromContext(logs[0].Context).TraceID().String())

	}

	{ // check traces
		require.Len(t, spans, 1)
		span, ok := spans["test.segment"]
		require.True(t, ok)

		assert.Equal(t, "test.segment", span.Name)

		assert.Equal(t, traceID, span.SpanContext.TraceID().String())
		assert.Equal(t, spanID, span.SpanContext.SpanID().String())

		require.Len(t, span.Attributes, 1)
		assert.Equal(t, attribute.Key("test-attribute-key"), span.Attributes[0].Key)
		assert.Equal(t, "test-attribute-value", span.Attributes[0].Value.AsString())

		assert.Len(t, span.Events, 1)
		assert.Equal(t, "test log", span.Events[0].Name)

		attrMap := otelrecorder.AttributeKVListToMap(span.Events[0].Attributes)
		assert.Equal(t, "info", attrMap["level"].AsString())
	}
}

func TestOtelzlog(t *testing.T) {
	recorder := otelrecorder.NewRecorder()
	t.Cleanup(recorder.Cleanup)

	logger := New("test",
		WithLoggerProvider(recorder.LogProvider),
		WithAttachSpanEvent(true),
	)

	spanID, traceID := sendTestEvents(t, logger, recorder)
	checkEvents(t, recorder, spanID, traceID)
}
