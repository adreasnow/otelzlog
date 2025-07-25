package otelrecorder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

func TestNewRecorder(t *testing.T) {
	recorder := NewRecorder()
	t.Cleanup(recorder.Cleanup)

	assert.NotNil(t, recorder.LogProvider)
	assert.NotNil(t, recorder.TracerProvider)

}

func TestGetLogs(t *testing.T) {
	recorder := NewRecorder()
	t.Cleanup(recorder.Cleanup)

	record := log.Record{}
	record.SetEventName("log event")

	recorder.LogProvider.Logger("").Emit(t.Context(), record)

	logs := recorder.GetLogs()
	require.Len(t, logs, 1)

	assert.Equal(t, "log event", logs[0].EventName)
}

func TestGetSpans(t *testing.T) {
	recorder := NewRecorder()
	t.Cleanup(recorder.Cleanup)

	tracer := recorder.TracerProvider.Tracer("")
	_, span := tracer.Start(t.Context(), "test span")
	span.End()

	spans := recorder.GetSpans()
	require.Len(t, spans, 1)
	require.Contains(t, spans, "test span")
	assert.Equal(t, "test span", spans["test span"].Name)
}

func TestLogKVListToMap(t *testing.T) {
	attrList := []log.KeyValue{
		log.String("str", "value1"),
		log.Int("int", 2),
		log.Bool("bool", true),
	}

	attrMap := LogKVListToMap(attrList)

	require.Len(t, attrMap, 3)
	assert.Equal(t, "value1", attrMap["str"].AsString())
	assert.Equal(t, int64(2), attrMap["int"].AsInt64())
	assert.Equal(t, true, attrMap["bool"].AsBool())
}

func TestAttributeKVListToMap(t *testing.T) {
	attrList := []attribute.KeyValue{
		attribute.String("str", "value1"),
		attribute.Int("int", 2),
		attribute.Bool("bool", true),
	}

	attrMap := AttributeKVListToMap(attrList)

	require.Len(t, attrMap, 3)
	assert.Equal(t, "value1", attrMap["str"].AsString())
	assert.Equal(t, int64(2), attrMap["int"].AsInt64())
	assert.Equal(t, true, attrMap["bool"].AsBool())
}
