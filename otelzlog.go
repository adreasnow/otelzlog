// Package otelzlog provides a bridge between zerolog and otel logging
package otelzlog

import (
	"context"
	"io"

	"github.com/rs/zerolog/log"
)

// New creates a new zerolog logger and embeds it in the context to be passed around your app.
func New(ctx context.Context, writers ...io.Writer) context.Context {
	logger := log.Logger

	switch {
	case len(writers) == 0:
		logger = log.Logger

	case len(writers) == 1:
		logger = log.Output(writers[0])

	case len(writers) > 1:
		logger = logger.Output(io.MultiWriter(writers...))
	}

	ctx = logger.Hook(&Hook{}).WithContext(ctx)

	return ctx
}
