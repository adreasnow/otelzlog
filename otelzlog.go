// Package otelzlog provides a bridge between zerolog and otel logging
package otelzlog

import (
	"context"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"
)

// New creates a new zerolog logger and embeds it in the context to be passed around your app.
func New(ctx context.Context, writers ...io.Writer) (context.Context, error) {
	if len(writers) == 0 {
		return ctx, fmt.Errorf("must specify at least one writer")
	}

	logger := log.Output(writers[0])

	if len(writers) > 1 {
		logger = logger.Output(io.MultiWriter(writers...))
	}

	ctx = logger.Hook(&Hook{}).WithContext(ctx)

	return ctx, nil
}
