package otelzlog

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/noop"
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

func TestWithVersion(t *testing.T) {
	c := config{}

	c = WithVersion("1.0.0").apply(c)
	assert.Len(t, c.loggerOpts, 1)
}

func TestWithSchemaURL(t *testing.T) {
	c := config{}
	c = WithSchemaURL("url").apply(c)
	assert.Len(t, c.loggerOpts, 1)
}

func TestWithAttributes(t *testing.T) {
	c := config{}
	c = WithAttributes(
		attribute.String("key", "value"),
		attribute.String("key", "value"),
	).apply(c)
	assert.Len(t, c.loggerOpts, 1)
}

func TestWithLoggerProvider(t *testing.T) {
	c := config{}
	provider := noop.NewLoggerProvider()

	c = WithLoggerProvider(provider).apply(c)
	assert.Equal(t, provider, c.provider)
}

func TestWithWriter(t *testing.T) {
	c := config{}
	buf1 := new(bytes.Buffer)
	buf2 := new(bytes.Buffer)

	c = WithWriter(buf1).apply(c)
	c = WithWriter(buf2).apply(c)

	assert.Equal(t, buf1, c.writers[0])
	assert.Equal(t, buf2, c.writers[1])
}

func TestWithSource(t *testing.T) {
	c := config{}

	c = WithSource(true, 1).apply(c)

	assert.True(t, c.source)
	assert.Equal(t, 1, c.sourceOffset)
}

func TestWithAttachSpanError(t *testing.T) {
	c := config{}

	c = WithAttachSpanError(true).apply(c)

	assert.True(t, c.attachSpanError)
}

func TestWithAttachSpanEvent(t *testing.T) {
	c := config{}

	c = WithAttachSpanEvent(true).apply(c)

	assert.True(t, c.attachSpanEvent)
}

func TestWithStackHandling(t *testing.T) {
	c := config{}

	zerolog.ErrorStackMarshaler = nil

	WithStackHandling().apply(c)

	assert.NotNil(t, zerolog.ErrorStackMarshaler)
}

func TestWithSetSpanErrorStatus(t *testing.T) {
	c := config{}

	c = WithSetSpanErrorStatus(true, zerolog.ErrorLevel).apply(c)

	assert.True(t, c.setSpanError)
	assert.Equal(t, zerolog.ErrorLevel, c.setSpanErrorLevel)
}
