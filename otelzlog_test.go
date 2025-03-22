package otelzlog

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Creates a new zerolog logger and embeds it in the context to be passed around your app
func TestNew(t *testing.T) {
	t.Run("no writers", func(t *testing.T) {
		t.Parallel()
		_, err := New(t.Context())
		require.Error(t, err, "must have an error if no writers are passed in")
	})

	t.Run("one writer", func(t *testing.T) {
		buf1 := new(bytes.Buffer)

		ctx, err := New(t.Context(),
			zerolog.ConsoleWriter{Out: buf1, NoColor: true},
		)
		require.NoError(t, err, "must not return an error")

		log.Ctx(ctx).Info().Msg("test message")

		line1 := buf1.String()

		assert.Contains(t, line1, "INF test message\n")
	})

	t.Run("multiple writers", func(t *testing.T) {
		buf1 := new(bytes.Buffer)
		buf2 := new(bytes.Buffer)

		ctx, err := New(t.Context(),
			zerolog.ConsoleWriter{Out: buf1, NoColor: true},
			zerolog.ConsoleWriter{Out: buf2, NoColor: true},
		)
		require.NoError(t, err)

		log.Ctx(ctx).Info().Msg("test message")

		line1 := buf1.String()
		line2 := buf2.String()

		assert.Equal(t, line1, line2)
		assert.Contains(t, line1, "INF test message\n")
	})
}
