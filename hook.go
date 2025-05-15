// Package otelzlog hook holds the hook that is attached to the zerolog logger
package otelzlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	otelLog "go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Hook is the parent struct of the otelzlog handler
type Hook struct {
	otelLogger      otelLog.Logger
	source          bool
	attachSpanError bool
	attachSpanEvent bool
}

// Run extracts the attributes and log level from the `*zerolog.Event`, and
// pulls the span from the passed in context in order to build the respective
// otel log.Record
func (h *Hook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := e.GetCtx()

	// return early if the logger isn't enabled for this log level
	if !e.Enabled() {
		return
	}

	var logData map[string]any
	ev := fmt.Sprintf("%s}", reflect.ValueOf(e).Elem().FieldByName("buf"))
	if err := json.Unmarshal([]byte(ev), &logData); err != nil {
		// log to the zerolog logger if there is an error reflecting the event's attribute buffer
		zlog.Ctx(e.GetCtx()).Error().Ctx(e.GetCtx()).
			Err(err).
			Str("log.level", level.String()).
			Str("log.message", msg).
			Msg("could not unmarshal the zerolog event's attribute buffer")
	}

	// convert zerolog attrs into otel log and span attrs
	logAttributes := h.processSpanAttrs(ctx, msg, logData)

	// create the otel log event and send it
	h.sendLogMessage(ctx, msg, level, logAttributes)
}

// processSpanAttrs converts each pulled attribute into the equivalent otel log counterparts.
// It also adds the attributes into the span and adds the error as an exception.
func (h *Hook) processSpanAttrs(ctx context.Context, msg string, logData map[string]any) (logAttributes []otelLog.KeyValue) {
	var errorAttr otelLog.KeyValue
	var stackAttr otelLog.KeyValue

	for k, v := range logData {
		switch k {
		// if there is an attribute called "error", then record the error in the span and
		// add it to the log attributes only (not the trace attributes)
		case zerolog.ErrorFieldName:
			errorAttr = otelLog.String(string(semconv.ExceptionMessageKey), v.(string))

		// if there is an attribute called "stack", then record the stack in the span and
		// add it to the log attributes only (not the trace attributes)
		case zerolog.ErrorStackFieldName:
			stackAttr = otelLog.String(string(semconv.ExceptionStacktraceKey), v.(string))

		// If there is a "caller" object in the log and if source is enabled in [Hook], then
		// append these using semconv fields instead of generic string attributes.
		case zerolog.CallerFieldName:
			sourcePath, ok := v.(string)
			if !ok || !h.source {
				continue
			}

			filepath, line, err := extractSource(sourcePath)
			if err != nil {
				continue
			}

			logAttributes = append(logAttributes,
				otelLog.String(string(semconv.CodeFilepathKey), filepath),
				otelLog.Int(string(semconv.CodeLineNumberKey), line),
			)

		default:
			logAttributes = append(logAttributes, otelLog.KeyValue{
				Key:   k,
				Value: convertAttribute(v),
			})
		}
	}

	// If enabled, add an otel span event (attach the log to the span).
	if h.attachSpanEvent {
		traceAttributes := []attribute.KeyValue{}

		for _, logAttr := range logAttributes {
			traceAttributes = append(traceAttributes, attribute.KeyValue{
				Key:   attribute.Key(logAttr.Key),
				Value: convertLogToAttribute(logAttr.Value),
			})
		}

		trace.SpanFromContext(ctx).AddEvent(msg,
			trace.WithAttributes(traceAttributes...),
		)
	}

	// Add an error and stack to the log attributes if provided
	if !errorAttr.Value.Empty() {
		logAttributes = append(logAttributes, errorAttr)
	}
	if !stackAttr.Value.Empty() {
		logAttributes = append(logAttributes, stackAttr)
	}

	// If enabled, attach the error and stack to the trace.
	if h.attachSpanError && !errorAttr.Value.Empty() {
		trace.SpanFromContext(ctx).RecordError(
			errors.New(errorAttr.Value.String()),
			trace.WithStackTrace(!stackAttr.Value.Empty()),
		)
	}

	return
}

func (h *Hook) sendLogMessage(ctx context.Context, msg string, level zerolog.Level, logAttributes []otelLog.KeyValue) {
	severityNumber, severityText := convertLevel(level)

	record := otelLog.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(otelLog.StringValue(msg))
	record.SetSeverity(severityNumber)
	record.SetSeverityText(severityText)
	record.AddAttributes(logAttributes...)

	h.otelLogger.Emit(ctx, record)
}
