package otelzlog

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

func TestConvertLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input          zerolog.Level
		expectedLevel  log.Severity
		expectedString string
	}{
		{
			input:          zerolog.TraceLevel,
			expectedLevel:  log.SeverityTrace,
			expectedString: "TRACE",
		},
		{
			input:          zerolog.DebugLevel,
			expectedLevel:  log.SeverityDebug,
			expectedString: "DEBUG",
		},
		{
			input:          zerolog.InfoLevel,
			expectedLevel:  log.SeverityInfo,
			expectedString: "INFO",
		},
		{
			input:          zerolog.WarnLevel,
			expectedLevel:  log.SeverityWarn,
			expectedString: "WARN",
		},
		{
			input:          zerolog.ErrorLevel,
			expectedLevel:  log.SeverityError,
			expectedString: "ERROR",
		},
		{
			input:          zerolog.FatalLevel,
			expectedLevel:  log.SeverityFatal,
			expectedString: "FATAL",
		},
		{
			input:          zerolog.PanicLevel,
			expectedLevel:  log.SeverityFatal,
			expectedString: "FATAL",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.input), func(t *testing.T) {
			outLevel, outString := convertLevel(tt.input)
			assert.Equal(t, tt.expectedLevel, outLevel)
			assert.Equal(t, tt.expectedString, outString)
		})
	}
}

func TestConvertAttribute(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		input    any
		expected log.Value
	}{
		{
			input:    true,
			expected: log.BoolValue(true),
		},
		{
			input:    "test",
			expected: log.StringValue("test"),
		},
		{
			input:    int(10),
			expected: log.Int64Value(10),
		},
		{
			input:    int8(10),
			expected: log.Int64Value(10),
		},
		{
			input:    int16(10),
			expected: log.Int64Value(10),
		},
		{
			input:    int32(10),
			expected: log.Int64Value(10),
		},
		{
			input:    int64(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uint(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uint8(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uint16(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uint32(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uint64(10),
			expected: log.Int64Value(10),
		},
		{
			input:    uintptr(10),
			expected: log.Int64Value(10),
		},
		{
			input:    float32(10.222),
			expected: log.Float64Value(float64(float32(10.222))),
		},
		{
			input:    float64(10.1),
			expected: log.Float64Value(10.1),
		},
		{
			input:    time.Second,
			expected: log.Int64Value(1000000000),
		},
		{
			input:    now,
			expected: log.Int64Value(now.UnixNano()),
		},
		{
			input: complex64(20 + 3i),
			expected: log.MapValue(
				log.Float64("r", real(complex128(complex64(20+3i)))),
				log.Float64("i", imag(complex128(complex64(20+3i)))),
			),
		},
		{
			input: complex128(20 + 3i),
			expected: log.MapValue(
				log.Float64("r", real(complex128(complex64(20+3i)))),
				log.Float64("i", imag(complex128(complex64(20+3i)))),
			),
		},
		{
			input:    []byte("abcd"),
			expected: log.BytesValue([]byte("abcd")),
		},
		{
			input:    fmt.Errorf("test"),
			expected: log.StringValue("test"),
		},

		// slices
		{
			input: []bool{true, true},
			expected: log.SliceValue(
				log.BoolValue(true),
				log.BoolValue(true),
			),
		},
		{
			input: []string{"test", "test"},
			expected: log.SliceValue(
				log.StringValue("test"),
				log.StringValue("test"),
			),
		},
		{
			input: []time.Time{now, now},
			expected: log.SliceValue(
				log.Int64Value(now.UnixNano()),
				log.Int64Value(now.UnixNano()),
			),
		},

		// maps
		{
			input: map[int64]bool{10: true},
			expected: log.MapValue(
				log.KeyValue{Key: "10", Value: log.BoolValue(true)},
			),
		},
		{
			input: map[string]string{"a": "test"},
			expected: log.MapValue(
				log.KeyValue{Key: "a", Value: log.StringValue("test")},
			),
		},
		{
			input: map[float32]time.Time{20.1: now},
			expected: log.MapValue(
				log.KeyValue{Key: "20.1", Value: log.Int64Value(now.UnixNano())},
			),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt.input), func(t *testing.T) {
			out := convertAttribute(tt.input)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestConvertUintValue(t *testing.T) {
	t.Parallel()

	for range 100 {
		in := rand.Uint64()
		t.Run(fmt.Sprintf("%d", in), func(t *testing.T) {
			v := convertUintValue(in)
			if in > math.MaxInt64 {
				assert.Equal(t, log.KindString, v.Kind())
				assert.Equal(t, fmt.Sprintf("%d", in), v.String())
			} else {
				assert.Equal(t, log.KindInt64, v.Kind())
				assert.Equal(t, int64(in), v.AsInt64())
			}
		})
	}
}

func TestConvertLogToAttribute(t *testing.T) {
	f64 := rand.Float64()
	i64 := rand.Int64()
	tests := []struct {
		input    log.Value
		expected attribute.Value
	}{
		{
			input:    log.StringValue("test"),
			expected: attribute.StringValue("test"),
		},
		{
			input:    log.Float64Value(f64),
			expected: attribute.Float64Value(f64),
		},
		{
			input:    log.Int64Value(i64),
			expected: attribute.Int64Value(i64),
		},
		{
			input:    log.BoolValue(true),
			expected: attribute.BoolValue(true),
		},
		{
			input:    log.BytesValue([]byte("test")),
			expected: attribute.StringValue("test"),
		},
		{
			input: log.SliceValue(
				log.Int64Value(1),
				log.Int64Value(2),
				log.Int64Value(3),
			),
			expected: attribute.StringValue("[1 2 3]"),
		},
		{
			input: log.MapValue(
				log.Int64("a", 1),
				log.Int64("b", 2),
				log.Int64("c", 3),
			),
			expected: attribute.StringValue("[a:1 b:2 c:3]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.AsString(), func(t *testing.T) {
			out := convertLogToAttribute(tt.input)
			assert.Equal(t, out, tt.expected)
		})
	}
}
