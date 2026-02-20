// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"encoding/json"
	"eva/internal/multimodal"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// VideoFrameMessage representa mensagem JSON de frame de vídeo
type VideoFrameMessage struct {
	Data      string `json:"data"`      // Base64 encoded frame (JPEG)
	Timestamp int64  `json:"timestamp"` // Unix timestamp em ms
	MimeType  string `json:"mime_type"` // "image/jpeg"
}

// HandleVideoStream gerencia conexão WebSocket para streaming de vídeo
// Endpoint: /video/stream?agendamento_id=X
func (h *Handler) HandleVideoStream(w http.ResponseWriter, r *http.Request) {
	// Extrai agendamento_id
	agIDStr := r.URL.Query().Get("agendamento_id")
	if agIDStr == "" {
		http.Error(w, "agendamento_id required", http.StatusBadRequest)
		return
	}

	// Recupera sessão
	session := GetSession(agIDStr)
	if session == nil {
		h.logger.Warn().Str("ag_id", agIDStr).Msg("Session not found for video stream")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Verifica se multimodal está habilitado
	mm := session.GetMultimodal()
	if mm == nil {
		h.logger.Warn().Str("ag_id", agIDStr).Msg("Multimodal not enabled for session")
		http.Error(w, "Multimodal not enabled", http.StatusForbidden)
		return
	}

	// Verifica se video está habilitado na config
	config := mm.GetConfig()
	if !config.EnableVideoInput {
		h.logger.Warn().Str("ag_id", agIDStr).Msg("Video input not enabled")
		http.Error(w, "Video input not enabled", http.StatusForbidden)
		return
	}

	// Upgrade para WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to upgrade to WebSocket")
		return
	}
	defer conn.Close()

	h.logger.Info().Str("ag_id", agIDStr).Msg("Video stream connection established")

	// Cria VideoStreamManager
	streamConfig := multimodal.DefaultVideoStreamConfig()
	streamConfig.MaxFPS = float64(config.VideoFrameRateFPS)

	streamManager := multimodal.NewVideoStreamManager(agIDStr, session.Client, streamConfig)

	// Inicia streaming
	if err := streamManager.StartStream(r.Context()); err != nil {
		h.logger.Error().Err(err).Msg("Failed to start video stream")
		return
	}
	defer streamManager.StopStream()

	// Configura timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Inicia ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Goroutine para enviar pings
	go func() {
		for range pingTicker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	// Loop de recebimento de frames
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error().Err(err).Msg("WebSocket unexpected close")
			}
			break
		}

		// Processa apenas mensagens de texto (JSON)
		if messageType != websocket.TextMessage {
			continue
		}

		// Parse JSON
		var frameMsg VideoFrameMessage
		if err := json.Unmarshal(message, &frameMsg); err != nil {
			h.logger.Warn().Err(err).Msg("Failed to parse video frame message")
			continue
		}

		// Valida dados
		if frameMsg.Data == "" {
			h.logger.Warn().Msg("Empty frame data received")
			continue
		}

		// Default mime type
		if frameMsg.MimeType == "" {
			frameMsg.MimeType = "image/jpeg"
		}

		// Cria VideoFrame
		frame := &multimodal.VideoFrame{
			Data:      []byte(frameMsg.Data), // Data já vem em base64 do cliente
			MimeType:  frameMsg.MimeType,
			Timestamp: time.Now(),
			SessionID: agIDStr,
		}

		// Adiciona ao stream (non-blocking)
		if err := streamManager.PushFrame(frame); err != nil {
			h.logger.Warn().Err(err).Msg("Failed to push frame to stream")
		}

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}

	// Log estatísticas ao finalizar
	stats := streamManager.GetStatistics()
	h.logger.Info().
		Str("ag_id", agIDStr).
		Interface("stats", stats).
		Msg("Video stream connection closed")
}
