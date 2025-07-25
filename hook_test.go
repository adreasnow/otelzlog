package otelzlog

import (
	"fmt"
	"testing"

	"github.com/adreasnow/otelzlog/otelrecorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

func TestHook(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).
			Hook(&Hook{
				otelLogger:      recorder.LogProvider.Logger("test"),
				attachSpanEvent: true,
			})

		spanID, traceID := sendTestEvents(t, &logger, recorder)

		checkEvents(t, recorder, spanID, traceID)
	})

	t.Run("error without attaching to span", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).
			Hook(&Hook{
				otelLogger: recorder.LogProvider.Logger("test"),
			})

		tracer := recorder.TracerProvider.Tracer(serviceName)
		var segment trace.Span
		var testErr error

		{
			ctx := t.Context()
			ctx, segment = tracer.Start(ctx, "segment")
			defer segment.End()

			testErr = fmt.Errorf("hook: an error occurred")
			logger.Error().Ctx(ctx).
				Err(testErr).
				Msg("test log")
		}

		logs := recorder.GetLogs()

		{ // check logs
			require.Len(t, logs, 1)

			attrMap := otelrecorder.LogKVListToMap(logs[0].Attributes)

			assert.Len(t, attrMap, 3)
			assert.Equal(t, "error", attrMap["level"].AsString())
			assert.Equal(t, "exception", attrMap["event"].AsString())
			assert.Equal(t, "hook: an error occurred", attrMap["exception.message"].AsString())
		}

		{ // check spans
			// no events should be recorded in the span
			spans := recorder.GetSpans()
			assert.Len(t, spans["segment"].Events, 0)
		}
	})

	t.Run("error with attaching to span", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).
			Hook(&Hook{
				otelLogger:      recorder.LogProvider.Logger("test"),
				attachSpanEvent: true,
			})

		tracer := recorder.TracerProvider.Tracer(serviceName)

		{
			ctx, segment := tracer.Start(t.Context(), "segment")

			testErr := fmt.Errorf("hook: an error occurred")
			logger.Error().Ctx(ctx).
				Err(testErr).
				Msg("test log")

			segment.End()
		}

		logs := recorder.GetLogs()
		{ // check logs
			require.Len(t, logs, 1)
			assert.Equal(t, "test log", logs[0].Body.String())

			assert.Equal(t, log.SeverityError, logs[0].Severity)

			attrMap := otelrecorder.LogKVListToMap(logs[0].Attributes)

			assert.Len(t, attrMap, 3)
			assert.Equal(t, "error", attrMap["level"].AsString())
			assert.Equal(t, "exception", attrMap["event"].AsString())
			assert.Equal(t, "hook: an error occurred", attrMap["exception.message"].AsString())
		}

		spans := recorder.GetSpans()
		{ // check spans
			require.Len(t, spans, 1)

			span, ok := spans["segment"]
			require.True(t, ok)

			assert.Equal(t, codes.Unset, span.Status.Code)
			assert.Equal(t, "", span.Status.Description)

			assert.Len(t, span.Events, 1)
			assert.Equal(t, "test log", span.Events[0].Name)

			attrMap := otelrecorder.AttributeKVListToMap(span.Events[0].Attributes)
			assert.Len(t, attrMap, 3)

			assert.Equal(t, "error", attrMap["level"].AsString())
			assert.Equal(t, "exception", attrMap["event"].AsString())
			assert.Equal(t, "hook: an error occurred", attrMap["exception.message"].AsString())
		}
	})

	t.Run("error with set span status", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).
			Hook(&Hook{
				otelLogger:        recorder.LogProvider.Logger("test"),
				setSpanError:      true,
				setSpanErrorLevel: zerolog.ErrorLevel,
			})

		tracer := recorder.TracerProvider.Tracer(serviceName)
		var testErr error
		{
			ctx := t.Context()
			ctx, span := tracer.Start(ctx, "segment")

			testErr = fmt.Errorf("hook: an error occurred")
			logger.Error().Ctx(ctx).
				Err(testErr).
				Msg("test log")
			span.End()
		}

		spans := recorder.GetSpans()
		{ // check spans
			require.Len(t, spans, 1)

			span, ok := spans["segment"]
			require.True(t, ok)

			assert.Equal(t, codes.Error, span.Status.Code)

			assert.Len(t, span.Events, 0)
		}

	})

	t.Run("source", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).With().CallerWithSkipFrameCount(2).
			Logger().
			Hook(&Hook{
				otelLogger: recorder.LogProvider.Logger("test"),
				source:     true,
			})

		tracer := recorder.TracerProvider.Tracer(serviceName)
		{
			ctx, span := tracer.Start(t.Context(), "segment")
			logger.Info().Ctx(ctx).
				Msg("test log")
			span.End()
		}

		logs := recorder.GetLogs()

		{ // check logs
			require.Len(t, logs, 1)
			attrMap := otelrecorder.LogKVListToMap(logs[0].Attributes)

			require.Len(t, attrMap, 3)
			fmt.Println(attrMap)
			assert.Contains(t, attrMap["code.filepath"].AsString(), "otelzlog/hook_test.go")
			assert.Equal(t, int64(195), attrMap["code.lineno"].AsInt64())
			assert.Equal(t, "info", attrMap["level"].AsString())

		}
	})

	t.Run("error with stack from panic attaching to span", func(t *testing.T) {
		recorder := otelrecorder.NewRecorder()
		t.Cleanup(recorder.Cleanup)

		logger := zerolog.
			New(zerolog.TestWriter{T: t}).
			Hook(&Hook{
				otelLogger: recorder.LogProvider.Logger("test"),
			})

		tracer := recorder.TracerProvider.Tracer(serviceName)

		var testErr error
		func() {
			ctx := t.Context()
			ctx, span := tracer.Start(ctx, "segment.parent")
			defer span.End()
			defer func() {
				if r := recover(); r != nil {
					testErr = fmt.Errorf("recovered from a panic during another process")
					logger.Error().Ctx(ctx).Str("stack", "stack-trace").Err(testErr).Send()
				}
			}()
			func() {
				logger.Panic().Ctx(ctx).Send()
			}()
		}()

		logs := recorder.GetLogs()

		{ // check logs
			require.Len(t, logs, 2)
			{ // panic log
				panicLog := logs[0]
				assert.Equal(t, log.SeverityFatal, panicLog.Severity)
			}

			{ // recover log
				recoverLog := logs[1]
				assert.Equal(t, log.SeverityError, recoverLog.Severity)

				attrMap := otelrecorder.LogKVListToMap(recoverLog.Attributes)
				require.Len(t, attrMap, 4)

				assert.Equal(t, "exception", attrMap["event"].AsString())
				assert.Equal(t, testErr.Error(), attrMap["exception.message"].AsString())
				assert.Equal(t, "stack-trace", attrMap["exception.stacktrace"].AsString())
				assert.Equal(t, "error", attrMap["level"].AsString())
			}
		}
	})
}
