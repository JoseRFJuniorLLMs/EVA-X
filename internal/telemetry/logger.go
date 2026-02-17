// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

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
