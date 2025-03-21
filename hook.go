package otelzlog

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	otelGlobalLogger "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

type Hook struct{}

// When the log is called, this function extracts the attributes and log level from
// the `*zerolog.Event`, and pulls the span from the passed in context in order to
// build the respective otel log.Record
func (h *Hook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := e.GetCtx()

	if !e.Enabled() {
		return
	}

	// pull the buffer from the zerolog.Event
	ev := fmt.Sprintf("%s}", reflect.ValueOf(e).Elem().FieldByName("buf"))
	var logData map[string]any
	_ = json.Unmarshal([]byte(ev), &logData)

	// convert each pulled attribute into the equivalent otel log coutnerpart
	var attributes []log.KeyValue
	for k, v := range logData {
		attributes = append(attributes, log.KeyValue{
			Key:   k,
			Value: convertAttrribute(v),
		})
	}

	var error any
	for k, v := range logData {
		switch k {
		case zerolog.ErrorFieldName:
			error = v
		}
	}

	if level >= 3 {
		trace.SpanFromContext(ctx).SetStatus(codes.Error, fmt.Sprintf("%s: %v", msg, error))
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
