package otelzlog

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adreasnow/otelstack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ExampleNew() {
	// Make sure there's something that can recieve your otel telemetry
	stack := otelstack.New()
	shutdownStack, err := stack.Start(context.Background())
	defer func() {
		if err := shutdownStack(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("error shutting down otelstack")
		}
	}()

	serviceName := "test-service"
	os.Setenv("OTEL_SERVICE_NAME", serviceName)
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:"+stack.Collector.Ports[4317].Port())

	// Set up your otel exporters
	shutdownOTEL, err := setupOTEL(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup OTEL logger")
	}
	defer shutdownOTEL()

	// Create your new logger
	buf := new(bytes.Buffer)
	ctx, err := New(context.Background(),
		zerolog.ConsoleWriter{Out: buf, NoColor: true},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new logger")
	}

	// Send a log event
	log.Ctx(ctx).Info().Msg("test message")

	// Check that the log event has made it to the telemetry
	{
		events, err := stack.Seq.GetEvents(1, 30)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get events from seq")
		}

		noNewLine := strings.Split(buf.String(), "\n")[0]
		noDate := strings.Split(noNewLine, " ")[1:]
		fmt.Println(noDate)
		fmt.Println(events[0].MessageTemplateTokens[0])
	}

	// Output:
	// [INF test message]
	// {test message}
}
