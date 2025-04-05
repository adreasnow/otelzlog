# OTEL Wrapper for zerolog

[![godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/adreasnow/otelzlog) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/adreasnow/otelzlog/main/LICENSE) [![Build Status](https://github.com/adreasnow/otelzlog/actions/workflows/test-tag.yaml/badge.svg?branch=main)](https://github.com/adreasnow/otelzlog/actions/workflows/test-tag.yaml) [![Go Coverage](https://github.com/adreasnow/otelzlog/wiki/coverage.svg)](https://raw.githack.com/wiki/adreasnow/otelzlog/coverage.html)

The otelzlog package provides a hook to use in order to send zerolog events to OTEL logs. It performs the following actions as a part of the hook:

1. Extracts the events from the zerolog logger.
1. Converts the attributes into OTEL log and OTEL span attributes while attaching any error as an exception in the span.
1. Creates a span event for the incoming log event and attaches all the associated attributes.
1. Sets the span as failed if the log level is `ERROR` or greater.
1. Sends an OTEL log event with the associated log level and all the attached log attributes.

## Usage

```go
// Initialise your OTEL configuration and save your logger to the global otel config with "go.opentelemetry.io/otel/log/global"
...
global.SetLoggerProvider(provider)
...

// Initialise the logger with as few, or as many writers as you'd like and save it to the context.
buf := new(bytes.Buffer)

ctx = otelzlog.New(ctx,
	zerolog.ConsoleWriter{Out: os.Stdout},
	zerolog.ConsoleWriter{Out: buf}
	zerolog.TestWriter{T: t},
)

// Set up your spans and traces as needed
tracer := otel.Tracer(")
ctx, span = tracer.Start(ctx, "test.segment")
time.sleep(time.Second) // do some work
defer span.End()

// Call the logger using log.Ctx(ctx) an pass in the context to the event
log.Ctx(ctx).Info().Ctx(ctx).Str("key", "value").Msg("Hello World")

// Spans will be extracted from the context and logs will be sent to your otel collector
// Attributes will be added from the logger, but not from the span
```

The syntax can be cumbersome, so wrapper functions are a good idea, e.g.:

```go
package log

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Debug(ctx context.Context) *zerolog.Event {
	return log.Ctx(ctx).Debug().Ctx(ctx)
}

func Warn(ctx context.Context) *zerolog.Event {
	return log.Ctx(ctx).Warn().Ctx(ctx)
}

...
```
