package otelzlog

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/adreasnow/otelstack/jaeger"
	"github.com/adreasnow/otelstack/seq"
	"github.com/pkg/errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"

	otelLogGlobal "go.opentelemetry.io/otel/log/global"
)

func TestHook(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{
				otelLogger:      otelLogGlobal.GetLoggerProvider().Logger("test"),
				attachSpanError: true,
				attachSpanEvent: true,
			}).
			WithContext(t.Context())

		spanID, traceID := sendTestEvents(ctx, t)

		time.Sleep(time.Second * 30)

		checkEvents(t, stack, spanID, traceID)
	})

	t.Run("error without attaching to span", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{
				otelLogger: otelLogGlobal.GetLoggerProvider().Logger("test"),
			}).
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
			assert.Equal(t, events[0].Exception, testErr.Error())
		}
	})

	t.Run("error with attaching to span", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{
				otelLogger:      otelLogGlobal.GetLoggerProvider().Logger("test"),
				attachSpanError: true,
				attachSpanEvent: true,
			}).
			WithContext(t.Context())

		tracer := otel.Tracer(serviceName)
		var parentSpan trace.Span
		var childSpan trace.Span
		var childCtx context.Context
		var testErr error
		func() {
			ctx, parentSpan = tracer.Start(ctx, "segment.parent")
			defer parentSpan.End()
			func() {
				childCtx, childSpan = tracer.Start(ctx, "segment.child")
				defer childSpan.End()

				testErr = errors.WithMessage(errors.New("hook: an error occurred"), "hook: an error occurred in a lower down function")
				log.Ctx(childCtx).Error().Ctx(childCtx).
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
			assert.Equal(t, events[0].Exception, testErr.Error())
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
				// these shouldn't be present unless setSpanError=true in Hook{}
				assert.NotContains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
					Key:   "error",
					Type:  "bool",
					Value: true,
				})
				assert.NotContains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
					Key:   string(semconv.OtelStatusCodeKey),
					Type:  "string",
					Value: "ERROR",
				})

				require.Len(t, spanMap["segment.child"].Logs, 1)
				require.Len(t, spanMap["segment.child"].Logs[0].Fields, 4)
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
					Key:   "level",
					Type:  "string",
					Value: "error",
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

	t.Run("error with stack from panic attaching to span", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{
				otelLogger:      otelLogGlobal.GetLoggerProvider().Logger("test"),
				attachSpanError: true,
				attachSpanEvent: true,
			}).
			WithContext(t.Context())

		tracer := otel.Tracer(serviceName)
		var parentSpan trace.Span
		var childSpan trace.Span
		var childCtx context.Context

		var testErr error
		func() {
			ctx, parentSpan = tracer.Start(ctx, "segment.parent")
			defer parentSpan.End()
			defer func() {
				if r := recover(); r != nil {
					testErr = errors.New("recovered from a panic during another process")
					log.Ctx(ctx).Error().Ctx(ctx).Str("stack", "stack-trace").Err(testErr).Send()
				}
			}()
			func() {
				childCtx, childSpan = tracer.Start(ctx, "segment.child")
				defer childSpan.End()

				log.Ctx(childCtx).Panic().Ctx(childCtx).Send()
			}()
		}()

		time.Sleep(time.Second * 3)

		events, _, err := stack.Seq.GetEvents(2, 10)
		require.NoError(t, err, "must be able to get events from seq")

		traces, _, err := stack.Jaeger.GetTraces(1, 10, serviceName)
		require.NoError(t, err, "must be able to get events from jaeger")

		{ // check logs
			require.Len(t, events, 2)
			require.Len(t, events[0].Messages, 1)
			{ // parent
				assert.Equal(t, "(No message)", events[0].Messages[0].Text)
				assert.Equal(t, "ERROR", events[0].Level)

				assert.Equal(t, parentSpan.SpanContext().TraceID().String(), events[0].TraceID)
				assert.Equal(t, parentSpan.SpanContext().SpanID().String(), events[0].SpanID)

				require.Len(t, events[0].Properties, 3)
				assert.Contains(t, events[0].Properties, seq.Property{
					Name:  "level",
					Value: "error",
				})
				assert.Contains(t, events[0].Exception, testErr.Error())
			}

			{ // child
				assert.Equal(t, "(No message)", events[1].Messages[0].Text)
				assert.Equal(t, "FATAL", events[1].Level)

				assert.Equal(t, childSpan.SpanContext().TraceID().String(), events[1].TraceID)
				assert.Equal(t, childSpan.SpanContext().SpanID().String(), events[1].SpanID)

				require.Len(t, events[1].Properties, 2)
				assert.Contains(t, events[1].Properties, seq.Property{
					Name:  "level",
					Value: "panic",
				})
			}
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
				for _, l := range spanMap["segment.child"].Logs {
					switch len(l.Fields) {
					case 2:
						assert.Contains(t, l.Fields, jaeger.KeyValue{
							Key:   "level",
							Type:  "string",
							Value: "panic",
						})
					case 3:
						assert.Contains(t, l.Fields, jaeger.KeyValue{
							Key:   "event",
							Type:  "string",
							Value: "exception",
						})
						assert.Contains(t, l.Fields, jaeger.KeyValue{
							Key:   "exception.message",
							Type:  "string",
							Value: "",
						})
						assert.Contains(t, l.Fields, jaeger.KeyValue{
							Key:   "exception.type",
							Type:  "string",
							Value: ".string",
						})
					default:
						t.Fatalf("could not match field to expected")
					}
				}
			}

			{ // parent span
				require.Len(t, spanMap["segment.parent"].Tags, 2)
				assert.Contains(t, spanMap["segment.parent"].Tags, jaeger.KeyValue{
					Key:   "otel.scope.name",
					Type:  "string",
					Value: serviceName,
				})

				require.Len(t, spanMap["segment.parent"].Logs, 1)
				require.Len(t, spanMap["segment.parent"].Logs[0].Fields, 5)

				assert.Contains(t, spanMap["segment.parent"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "event",
					Type:  "string",
					Value: "exception",
				})
				assert.Contains(t, spanMap["segment.parent"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "exception.message",
					Type:  "string",
					Value: testErr.Error(),
				})
				assert.Contains(t, spanMap["segment.parent"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "level",
					Type:  "string",
					Value: "error",
				})
				assert.Contains(t, spanMap["segment.parent"].Logs[0].Fields, jaeger.KeyValue{
					Key:   "exception.stacktrace",
					Type:  "string",
					Value: "stack-trace",
				})

			}
		}
	})

	t.Run("error with set span status", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := log.
			Hook(&Hook{
				otelLogger:        otelLogGlobal.GetLoggerProvider().Logger("test"),
				attachSpanError:   true,
				attachSpanEvent:   true,
				setSpanError:      true,
				setSpanErrorLevel: zerolog.ErrorLevel,
			}).
			WithContext(t.Context())

		tracer := otel.Tracer(serviceName)
		var testErr error
		func() {
			ctx, span := tracer.Start(ctx, "segment.child")
			defer span.End()

			testErr = errors.New("hook: an error occurred")
			log.Ctx(ctx).Error().Ctx(ctx).
				Err(testErr).
				Msg("test log")
		}()

		time.Sleep(time.Second * 3)

		traces, _, err := stack.Jaeger.GetTraces(1, 10, serviceName)
		require.NoError(t, err, "must be able to get events from jaeger")

		// time.Sleep(time.Second * 30)

		{ // check traces
			require.Len(t, traces, 1)
			require.Len(t, traces[0].Spans, 1)

			require.Len(t, traces[0].Spans[0].Tags, 4)
			assert.Contains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
				Key:   "error",
				Type:  "bool",
				Value: true,
			})
			assert.Contains(t, traces[0].Spans[0].Tags, jaeger.KeyValue{
				Key:   string(semconv.OtelStatusCodeKey),
				Type:  "string",
				Value: "ERROR",
			})
		}
	})

	t.Run("source", func(t *testing.T) {
		stack := setupOTELStack(t)

		buf := new(bytes.Buffer)

		ctx := log.With().CallerWithSkipFrameCount(0).Logger().
			Output(buf).
			Hook(&Hook{
				otelLogger: otelLogGlobal.GetLoggerProvider().Logger("test"),
				source:     true,
			}).WithContext(t.Context())

		tracer := otel.Tracer(serviceName)
		var parentSpan trace.Span
		var childSpan trace.Span
		func() {
			ctx, parentSpan = tracer.Start(ctx, "segment.parent")
			defer parentSpan.End()
			func() {
				ctx, childSpan = tracer.Start(ctx, "segment.child")
				defer childSpan.End()
				log.Ctx(ctx).Info().Ctx(ctx).
					Msg("test log")
			}()
		}()

		time.Sleep(time.Second * 3)

		events, _, err := stack.Seq.GetEvents(1, 10)
		require.NoError(t, err, "must be able to get events from seq")

		{ // check logs
			require.Len(t, events, 1)
			require.Len(t, events[0].Messages, 1)
			assert.Equal(t, "test log", events[0].Messages[0].Text)

			assert.Equal(t, "INFO", events[0].Level)

			assert.Equal(t, childSpan.SpanContext().TraceID().String(), events[0].TraceID)
			assert.Equal(t, childSpan.SpanContext().SpanID().String(), events[0].SpanID)

			require.Len(t, events[0].Properties, 3)
			assert.Contains(t, events[0].Properties, seq.Property{
				Name:  "level",
				Value: "info",
			})

			m := map[string]any{}
			err := json.Unmarshal(buf.Bytes(), &m)
			require.NoError(t, err)

			filepath, line, err := extractSource(m["caller"].(string))
			require.NoError(t, err)

			assert.Contains(t, events[0].Properties, seq.Property{
				Name: "code",
				Value: map[string]any{
					"filepath": filepath,
					"lineno":   float64(line),
				},
			})
		}
	})
}
