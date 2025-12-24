package voice

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"eva-mind/internal/config"
	"eva-mind/internal/database"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Validar origem em produção
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	db     *database.DB
	cfg    *config.Config
	logger zerolog.Logger
}

func NewHandler(db *database.DB, cfg *config.Config, logger zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		cfg:    cfg,
		logger: logger,
	}
}

func (h *Handler) HandleMediaStream(w http.ResponseWriter, r *http.Request) {
	// Extrai agendamento_id da URL
	path := r.URL.Path
	parts := strings.Split(path, "/")
	agendamentoIDStr := parts[len(parts)-1]

	agendamentoID, err := strconv.Atoi(agendamentoIDStr)
	if err != nil {
		h.logger.Error().Err(err).Msg("Invalid agendamento_id")
		http.Error(w, "Invalid agendamento ID", http.StatusBadRequest)
		return
	}

	// Upgrade para WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	h.logger.Info().Int("agendamento_id", agendamentoID).Msg("New WebSocket connection")

	// Busca contexto do banco
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	callCtx, err := h.db.GetCallContext(ctx, agendamentoID)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get call context")
		return
	}

	// Inicia sessão Gemini
	session := NewGeminiSession(conn, callCtx, h.db, h.cfg, h.logger)
	if err := session.Start(ctx); err != nil {
		h.logger.Error().Err(err).Msg("Gemini session failed")
	}
}
