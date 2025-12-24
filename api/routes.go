package api

import (
	"net/http"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/voice"

	"github.com/rs/zerolog"
)

func NewRouter(db *database.DB, cfg *config.Config, logger zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Health check (agora chamando funções organizadas no health.go)
	mux.HandleFunc("/health", handleHealth(db))
	mux.HandleFunc("/health/live", handleLiveness())
	mux.HandleFunc("/health/ready", handleReadiness(db))

	// Voice WebSocket - ESSENCIAL para o projeto
	voiceHandler := voice.NewHandler(db, cfg, logger)
	mux.HandleFunc("/media-stream/", voiceHandler.HandleMediaStream)

	// TwiML - ESSENCIAL para o Twilio
	mux.HandleFunc("/calls/twiml", handleTwiML(cfg))

	// Metrics
	mux.Handle("/metrics", handleMetrics())

	return mux
}

// handleTwiML gera o XML dinâmico para o Twilio
func handleTwiML(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agendamentoID := r.URL.Query().Get("agendamento_id")
		if agendamentoID == "" {
			http.Error(w, "Missing agendamento_id", http.StatusBadRequest)
			return
		}

		// Importante: Twilio precisa se conectar via WSS ao nosso servidor
		twiml := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Connect>
        <Stream url="wss://` + cfg.ServiceDomain + `/media-stream/` + agendamentoID + `" />
    </Connect>
</Response>`

		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(twiml))
	}
}
