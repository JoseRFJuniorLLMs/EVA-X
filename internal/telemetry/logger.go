package telemetry

import (
	"os"

	"github.com/rs/zerolog"
)

func NewLogger(env string) zerolog.Logger {
	output := os.Stdout
	if env == "development" {
		// output = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	return zerolog.New(output).With().Timestamp().Logger()
}
