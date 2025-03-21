package otelzlog

import (
	"context"
	"io"

	"github.com/rs/zerolog/log"
)

// Creates a new zerolog logger and embeds it in the context to be passed around your app
func New(ctx context.Context, defaultWriter io.Writer) context.Context {
	ctx = log.
		Output(defaultWriter).
		Hook(&Hook{}).
		WithContext(ctx)

	return ctx
}
