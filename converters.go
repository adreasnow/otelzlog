// Package otelzlog converters hold the functions that are needed to convert
// between zerolog and otel logging event types
package otelzlog

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// convertLevel converts the logging level from a zerolog.Level into a an otel log.Severity
// and the corresponding severity level string
func convertLevel(level zerolog.Level) (log.Severity, string) {
	switch level {
	case zerolog.TraceLevel:
		return log.SeverityTrace, "TRACE"
	case zerolog.DebugLevel:
		return log.SeverityDebug, "DEBUG"
	default:
		fallthrough
	case zerolog.InfoLevel:
		return log.SeverityInfo, "INFO"
	case zerolog.WarnLevel:
		return log.SeverityWarn, "WARN"
	case zerolog.ErrorLevel:
		return log.SeverityError, "ERROR"
	case zerolog.FatalLevel:
		return log.SeverityFatal, "FATAL"
	case zerolog.PanicLevel:
		return log.SeverityFatal, "FATAL"
	}
}

// convertAttribute converts value from `any` into the equivalent otel log.Value.
// This function is a direct copy paste from the otelslog package.
func convertAttribute(v any) log.Value {
	switch val := v.(type) {
	case bool:
		return log.BoolValue(val)
	case string:
		return log.StringValue(val)
	case int:
		return log.Int64Value(int64(val))
	case int8:
		return log.Int64Value(int64(val))
	case int16:
		return log.Int64Value(int64(val))
	case int32:
		return log.Int64Value(int64(val))
	case int64:
		return log.Int64Value(val)
	case uint:
		return convertUintValue(uint64(val))
	case uint8:
		return log.Int64Value(int64(val))
	case uint16:
		return log.Int64Value(int64(val))
	case uint32:
		return log.Int64Value(int64(val))
	case uint64:
		return convertUintValue(val)
	case uintptr:
		return convertUintValue(uint64(val))
	case float32:
		return log.Float64Value(float64(val))
	case float64:
		return log.Float64Value(val)
	case time.Duration:
		return log.Int64Value(val.Nanoseconds())
	case complex64:
		r := log.Float64("r", real(complex128(val)))
		i := log.Float64("i", imag(complex128(val)))
		return log.MapValue(r, i)
	case complex128:
		r := log.Float64("r", real(val))
		i := log.Float64("i", imag(val))
		return log.MapValue(r, i)
	case time.Time:
		return log.Int64Value(val.UnixNano())
	case []byte:
		return log.BytesValue(val)
	case error:
		return log.StringValue(val.Error())
	}

	t := reflect.TypeOf(v)
	if t == nil {
		return log.Value{}
	}
	val := reflect.ValueOf(v)
	switch t.Kind() {
	case reflect.Struct:
		return log.StringValue(fmt.Sprintf("%+v", v))
	case reflect.Slice, reflect.Array:
		items := make([]log.Value, 0, val.Len())
		for i := range val.Len() {
			items = append(items, convertAttribute(val.Index(i).Interface()))
		}
		return log.SliceValue(items...)
	case reflect.Map:
		kvs := make([]log.KeyValue, 0, val.Len())
		for _, k := range val.MapKeys() {
			var key string
			switch k.Kind() {
			case reflect.String:
				key = k.String()
			default:
				key = fmt.Sprintf("%+v", k.Interface())
			}
			kvs = append(kvs, log.KeyValue{
				Key:   key,
				Value: convertAttribute(val.MapIndex(k).Interface()),
			})
		}
		return log.MapValue(kvs...)
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return log.Value{}
		}
		return convertAttribute(val.Elem().Interface())
	}

	// Try to handle this as gracefully as possible.
	//
	// Don't panic here. it is preferable to have user's open issue
	// asking why their attributes have a "unhandled: " prefix than
	// say that their code is panicking.
	return log.StringValue(fmt.Sprintf("unhandled: (%s) %+v", t, v))
}

func convertUintValue(v uint64) log.Value {
	if v > math.MaxInt64 {
		return log.StringValue(strconv.FormatUint(v, 10))
	}
	return log.Int64Value(int64(v))
}

func convertLogToAttribute(attr log.Value) attribute.Value {
	switch attr.Kind() {
	case log.KindString:
		return attribute.StringValue(attr.String())
	case log.KindFloat64:
		return attribute.Float64Value(attr.AsFloat64())
	case log.KindInt64:
		return attribute.Int64Value(attr.AsInt64())
	case log.KindBool:
		return attribute.BoolValue(attr.AsBool())
	case log.KindBytes:
		return attribute.StringValue(fmt.Sprintf("%s", attr.AsBytes()))
	case log.KindSlice:
		return attribute.StringValue(fmt.Sprintf("%v", attr.AsSlice()))
	case log.KindMap:
		return attribute.StringValue(fmt.Sprintf("%v", attr.AsMap()))
	case log.KindEmpty:
		return attribute.StringValue("")
	}

	return attribute.StringValue(attr.AsString())
}
