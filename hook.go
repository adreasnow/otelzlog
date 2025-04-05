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
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	otelGlobalLogger "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

// Hook is the parent struct of the otelzlog handler
type Hook struct{}

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
		// log to the zerolog logger if there is an error with the request
		zlog.Ctx(e.GetCtx()).Error().Ctx(e.GetCtx()).
			Err(err).
			Str("log.level", level.String()).
			Str("log.message", msg).
			Msg("could not unmarshal the zerolog event's attribute buffer")
	}

	// convert zerolog attrs into otel log and span attrs
	logAttributes := processSpanAttrs(ctx, msg, level, logData)

	// create the otel log event and send it
	sendLogMessage(ctx, msg, level, logAttributes)
}

// processSpanAttrs converts each pulled attribute into the equivalent otel log counterparts.
// It also adds the attributes into the span and sets the span as errored if the level is error or greater.
func processSpanAttrs(ctx context.Context, msg string, level zerolog.Level, logData map[string]any) (logAttributes []log.KeyValue) {
	traceAttributes := []attribute.KeyValue{}
	for k, v := range logData {
		switch k {
		// if there was was an attribute called "error", then record the error in the span and
		// add it to the log attributes only (not the trace attributes)
		case zerolog.ErrorFieldName:
			trace.SpanFromContext(ctx).RecordError(errors.New(v.(string)))
			logAttribute := convertAttribute(v)
			logAttributes = append(logAttributes, log.KeyValue{
				Key:   k,
				Value: logAttribute,
			})

		default:
			logAttribute := convertAttribute(v)
			logAttributes = append(logAttributes, log.KeyValue{
				Key:   k,
				Value: logAttribute,
			})

			traceAttribute := convertLogToAttribute(logAttribute)
			traceAttributes = append(traceAttributes, attribute.KeyValue{
				Key:   attribute.Key(k),
				Value: traceAttribute,
			})
		}
	}

	// add an otel span event (attach the log to the span)
	trace.SpanFromContext(ctx).AddEvent(msg,
		trace.WithAttributes(traceAttributes...),
	)

	// set the span as errored if level is >= error
	if level >= 3 {
		trace.SpanFromContext(ctx).SetStatus(codes.Error, "")
	}

	return
}

func sendLogMessage(ctx context.Context, msg string, level zerolog.Level, logAttributes []log.KeyValue) {
	severityNumber, severityText := convertLevel(level)

	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(log.StringValue(msg))
	record.SetSeverity(severityNumber)
	record.SetSeverityText(severityText)
	record.AddAttributes(logAttributes...)

	otelGlobalLogger.GetLoggerProvider().Logger("").Emit(ctx, record)
}
