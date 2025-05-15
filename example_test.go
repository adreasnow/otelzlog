package otelzlog

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/adreasnow/otelstack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ExampleNew() {
	// Make sure there's something that can receive your otel telemetry
	stack := otelstack.New(false, true, false)
	shutdownStack, err := stack.Start(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("could not set start stack")
	}
	defer func() {
		if err := shutdownStack(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("error shutting down otelstack")
		}
	}()

	// Set up your otel exporters
	shutdownOTEL, err := setupOTEL(context.Background(), stack.Collector.Ports[4317])
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup OTEL logger")
	}
	defer shutdownOTEL()

	// Create your new logger
	buf := new(bytes.Buffer)
	ctx := New(context.Background(),
		"test",
		WithWriter(zerolog.ConsoleWriter{Out: buf, NoColor: true}),
		WithAttachSpanError(true),
		WithAttachSpanEvent(true),
		WithSource(true, 0),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new logger")
	}

	// Send a log event
	log.Ctx(ctx).Info().Msg("test message")

	// Check that the log event has made it to the telemetry
	{
		events, _, err := stack.Seq.GetEvents(1, 30)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get events from seq")
		}

		noNewLine := strings.Split(buf.String(), "\n")[0]
		noDate := strings.Split(noNewLine, " ")[1:]
		fmt.Println(noDate)
		fmt.Println(events[0].Messages[0])
	}

	// Output:
	// [INF example_test.go:48 > test message]
	// {test message}
}
