package otelzlog

import (
	"context"
	"fmt"

	"github.com/adreasnow/otelzlog/otelrecorder"
	"go.opentelemetry.io/otel"
)

func ExampleNew() {
	// Make sure there's something that can receive your otel telemetry
	// and set up your OTEL exporters
	recorder := otelrecorder.NewRecorder()
	defer recorder.Cleanup()

	// Create your new logger
	logger := New(
		"test",
		WithLoggerProvider(recorder.LogProvider),
	)

	// Start a span and send a log event.
	tracer := otel.Tracer("test.service")

	func() {
		ctx, span := tracer.Start(context.Background(), "segment")
		defer span.End()
		// The context with the span is passed to the logger with the `Ctx` method.
		logger.Info().Ctx(ctx).Msg("test message")
	}()

	// Check that the log event has made it to the telemetry
	{
		logs := recorder.GetLogs()

		if len(logs) == 1 {
			fmt.Println(logs[0].Body.AsString())
		}
	}

	// Output:
	// test message
}
