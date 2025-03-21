package otelzlog

import (
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

func TestConvertAttrribute(t *testing.T) {
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
			out := convertAttrribute(tt.input)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestConvertUintValue(t *testing.T) {
	t.Parallel()

}

// func convertUintValue(v uint64) log.Value {
// 	if v > math.MaxInt64 {
// 		return log.StringValue(strconv.FormatUint(v, 10))
// 	}
// 	return log.Int64Value(int64(v))
// }
