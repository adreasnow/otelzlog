# OTEL Wrapper for zerolog

[![godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/adreasnow/otelzlog) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/adreasnow/otelzlog/main/LICENSE) [![Build Status](https://github.com/adreasnow/otelzlog/actions/workflows/test-tag.yaml/badge.svg?branch=main)](https://github.com/adreasnow/otelzlog/actions/workflows/test-tag.yaml) [![Go Coverage](https://github.com/adreasnow/otelzlog/wiki/coverage.svg)](https://raw.githack.com/wiki/adreasnow/otelzlog/coverage.html)

The otelzlog package provides a hook to use in order to send zerolog events to OTEL logs

## Usage

```go
// initiliase your OTEL configuration and save your logger to the global otel config with "go.opentelemetry.io/otel/log/global"
...
global.SetLoggerProvider(provider)

// initialise the logger, saving it to the context, and pass in your default logger
ctx = otelzlog.New(ctx, zerolog.ConsoleWriter{Out: os.Stdout})

// set up your spans and traces as needed
tracer := otel.Tracer("service-name")
ctx, span = tracer.Start(ctx, "test.segment")
defer span.End()

// call the logger using log.Ctx(ctx) an pass in the context to the event
log.Ctx(ctx).Info().Ctx(ctx).Msg("Hello World")

// spans will be extracted from the context and logs will be sent toyour otel collector
```

The syntax can be cumbersome, so wrapper functions are a good idea, e.g.

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
