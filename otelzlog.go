// Package otelzlog provides a bridge between zerolog and otel logging
package otelzlog

import (
	"context"
	"io"
	"runtime/debug"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	otelLog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

type config struct {
	provider otelLog.LoggerProvider

	source       bool
	sourceOffset int

	attachSpanError bool
	attachSpanEvent bool

	writers []io.Writer

	loggerOpts []otelLog.LoggerOption
}

// Option configures the zerolog hook.
type Option interface {
	apply(config) config
}

type optFunc func(config) config

func (f optFunc) apply(c config) config {
	return f(c)
}

// WithVersion returns an [Option] that configures the version of the
// [log.Logger] used by a [Hoo]. The version should be the version of the
// package that is being logged.
func WithVersion(version string) Option {
	return optFunc(func(c config) config {
		c.loggerOpts = append(c.loggerOpts, otelLog.WithInstrumentationVersion(version))
		return c
	})
}

// WithSchemaURL returns an [Option] that configures the semantic convention
// schema URL of the [log.Logger] used by a [Hook]. The schemaURL should be
// the schema URL for the semantic conventions used in log records.
func WithSchemaURL(schemaURL string) Option {
	return optFunc(func(c config) config {
		c.loggerOpts = append(c.loggerOpts, otelLog.WithSchemaURL(schemaURL))
		return c
	})
}

// WithAttributes returns an [Option] that configures the instrumentation scope
// attributes of the [log.Logger] used by a [Hook].
func WithAttributes(attributes ...attribute.KeyValue) Option {
	return optFunc(func(c config) config {
		c.loggerOpts = append(c.loggerOpts, otelLog.WithInstrumentationAttributes(attributes...))
		return c
	})
}

// WithLoggerProvider returns an [Option] that configures [log.LoggerProvider]
// used by a [Hook] to create its [log.Logger].
//
// By default if this Option is not provided, the Handler will use the global
// LoggerProvider.
func WithLoggerProvider(provider otelLog.LoggerProvider) Option {
	return optFunc(func(c config) config {
		c.provider = provider
		return c
	})
}

// WithWriter returns an [Option] that configures writers used by a
// [Hook]. Multiple writers can be specified.
func WithWriter(w io.Writer) Option {
	return optFunc(func(c config) config {
		c.writers = append(c.writers, w)
		return c
	})
}

// WithSource returns an [Option] that configures the [Hook] to include
// the source location of the log record in log attributes. Offset should
// be increased if using a helper function to wrap the logger call.
func WithSource(source bool, offset int) Option {
	return optFunc(func(c config) config {
		c.source = source
		c.sourceOffset = offset
		return c
	})
}

// WithAttachSpanError returns an [Option] that configures the [Hook]
// to attach errors from `log.Error().Err()` to the associated otel span.
func WithAttachSpanError(attach bool) Option {
	return optFunc(func(c config) config {
		c.attachSpanError = attach
		return c
	})
}

// WithAttachSpanEvent returns an [Option] that configures the [Hook]
// to attach an event to the otel span the zerolog event.
func WithAttachSpanEvent(attach bool) Option {
	return optFunc(func(c config) config {
		c.attachSpanEvent = attach
		return c
	})
}

// WithStackHandling returns an [Option] that sets
// zerolog.ErrorStackMarshaler in order to extract the stack when .Stack()
// is called on a .Error() event.
func WithStackHandling() Option {
	return optFunc(func(c config) config {
		zerolog.ErrorStackMarshaler = func(_ error) any {
			return string(debug.Stack())
		}
		return c
	})
}

func newCfg(options []Option) config {
	var c config
	for _, opt := range options {
		c = opt.apply(c)
	}

	if c.provider == nil {
		c.provider = global.GetLoggerProvider()
	}

	return c
}

// New creates a new zerolog logger and embeds it in the context to be passed around your app.
func New(ctx context.Context, name string, options ...Option) context.Context {
	logger := log.Logger

	cfg := newCfg(options)

	switch {
	case len(cfg.writers) == 0:
		logger = log.Logger

	default:
		logger = logger.Output(io.MultiWriter(cfg.writers...))
	}

	hook := Hook{
		otelLogger:      cfg.provider.Logger(name, cfg.loggerOpts...),
		source:          cfg.source,
		attachSpanError: cfg.attachSpanError,
		attachSpanEvent: cfg.attachSpanEvent,
	}

	if cfg.source {
		logger = logger.With().CallerWithSkipFrameCount(cfg.sourceOffset + 2).Logger()
	}

	ctx = logger.Hook(&hook).WithContext(ctx)

	return ctx
}
