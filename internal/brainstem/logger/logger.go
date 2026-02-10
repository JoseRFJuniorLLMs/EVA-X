package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

// LogLevel representa os níveis de log
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)

// Init inicializa o logger global
func Init(level LogLevel, environment string) {
	// Pretty print em desenvolvimento
	if environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Definir nível de log
	switch level {
	case DebugLevel:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case InfoLevel:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case WarnLevel:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case ErrorLevel:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case FatalLevel:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	Logger = log.Logger.With().Timestamp().Str("service", "eva-mind").Logger()
}

// Helpers para diferentes componentes

func WebSocket() zerolog.Logger {
	return Logger.With().Str("component", "websocket").Logger()
}

func Scheduler() zerolog.Logger {
	return Logger.With().Str("component", "scheduler").Logger()
}

func Gemini() zerolog.Logger {
	return Logger.With().Str("component", "gemini").Logger()
}

func Database() zerolog.Logger {
	return Logger.With().Str("component", "database").Logger()
}

func Push() zerolog.Logger {
	return Logger.With().Str("component", "push").Logger()
}

func Worker() zerolog.Logger {
	return Logger.With().Str("component", "worker").Logger()
}

// Exemplo de uso:
//
// import "eva-mind/internal/brainstem/logger"
//
// log := logger.WebSocket()
// log.Info().Str("cpf", client.CPF).Msg("Cliente registrado")
// log.Error().Err(err).Msg("Erro ao processar mensagem")
// log.Debug().Int("audio_count", count).Msg("Áudio processado")
