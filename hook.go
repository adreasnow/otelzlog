// Package otelzlog hook holds the hook that is attached to the zerolog logger
package otelzlog

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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

	if !e.Enabled() {
		return
	}

	// pull the buffer from the zerolog.Event
	ev := fmt.Sprintf("%s}", reflect.ValueOf(e).Elem().FieldByName("buf"))
	var logData map[string]any
	err := json.Unmarshal([]byte(ev), &logData)
	if err != nil {
		// log to the zerolog logger if there is an error with the request
		zlog.Ctx(e.GetCtx()).Error().Ctx(e.GetCtx()).
			Err(err).
			Str("log.level", level.String()).
			Str("log.message", msg).
			Msg("could not pull unmarshal the zerolog event's attribute buffer")
	}

	// convert each pulled attribute into the equivalent otel log counterpart
	var attributes []log.KeyValue
	for k, v := range logData {
		attributes = append(attributes, log.KeyValue{
			Key:   k,
			Value: convertAttribute(v),
		})
	}

	var errorAny any
	for k, v := range logData {
		switch k {
		case zerolog.ErrorFieldName:
			errorAny = v
		}
	}

	if level >= 3 {
		trace.SpanFromContext(ctx).SetStatus(codes.Error, fmt.Sprintf("%s: %v", msg, errorAny))
	}

	span := trace.SpanFromContext(ctx).SpanContext()
	if span.IsValid() {
		attributes = append(attributes, log.KeyValue{
			Key:   "SpanId",
			Value: log.StringValue(span.SpanID().String()),
		})
		attributes = append(attributes, log.KeyValue{
			Key:   "TraceId",
			Value: log.StringValue(span.TraceID().String()),
		})
	}

	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetBody(log.StringValue(msg))

	severityNumber, severityText := convertLevel(level)
	record.SetSeverity(severityNumber)
	record.SetSeverityText(severityText)
	record.AddAttributes(attributes...)

	otelGlobalLogger.GetLoggerProvider().
		Logger(os.Getenv("OTEL_SERVICE_NAME")).
		Emit(ctx, record)
}
