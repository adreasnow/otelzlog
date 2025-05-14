package otelzlog

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("no writer", func(t *testing.T) {
		stack := setupOTELStack(t)

		ctx := New(t.Context(),
			"test",
			WithAttachSpanError(true),
			WithAttachSpanEvent(true),
		)

		log.Ctx(ctx).Info().Msg("test message")

		spanID, traceID := sendTestEvents(ctx, t)
		checkEvents(t, stack, spanID, traceID)
	})

	t.Run("one writer", func(t *testing.T) {
		stack := setupOTELStack(t)

		buf := new(bytes.Buffer)
		ctx := New(t.Context(),
			"test",
			WithWriter(zerolog.ConsoleWriter{Out: buf, NoColor: true}),
			WithAttachSpanError(true),
			WithAttachSpanEvent(true),
		)

		log.Ctx(ctx).Info().Msg("test message")

		spanID, traceID := sendTestEvents(ctx, t)
		checkEvents(t, stack, spanID, traceID)

		assert.Contains(t, buf.String(), "INF test message\n")
	})

	t.Run("multiple writers", func(t *testing.T) {
		stack := setupOTELStack(t)

		buf1 := new(bytes.Buffer)
		buf2 := new(bytes.Buffer)

		ctx := New(t.Context(),
			"test",
			WithWriter(zerolog.ConsoleWriter{Out: buf1, NoColor: true}),
			WithWriter(zerolog.ConsoleWriter{Out: buf2, NoColor: true}),
			WithAttachSpanError(true),
			WithAttachSpanEvent(true),
		)

		log.Ctx(ctx).Info().Msg("test message")

		spanID, traceID := sendTestEvents(ctx, t)
		checkEvents(t, stack, spanID, traceID)

		assert.Equal(t, buf1.String(), buf2.String())
		assert.Contains(t, buf1.String(), "INF test message\n")
	})
}
