// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"encoding/json"
	"eva-mind/internal/gemini"
	"eva-mind/internal/multimodal"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HandleMediaUpload processa upload de mídia (imagens/vídeo) via HTTP POST
// Endpoint: POST /media/upload?agendamento_id=X
// Content-Type: image/jpeg, image/png, video/mp4, etc
func (h *Handler) HandleMediaUpload(w http.ResponseWriter, r *http.Request) {
	// Valida método HTTP
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extrai agendamento_id
	agIDStr := r.URL.Query().Get("agendamento_id")
	if agIDStr == "" {
		http.Error(w, "agendamento_id required", http.StatusBadRequest)
		return
	}

	// Recupera sessão
	session := GetSession(agIDStr)
	if session == nil {
		h.logger.Warn().Str("ag_id", agIDStr).Msg("Session not found for media upload")
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

	// Determina tipo de mídia pelo Content-Type
	mediaType := r.Header.Get("Content-Type")
	if mediaType == "" {
		http.Error(w, "Content-Type header required", http.StatusBadRequest)
		return
	}

	// Lê corpo da requisição (máximo razoável para evitar DoS)
	maxSize := int64(50 * 1024 * 1024) // 50MB máximo
	limitedReader := io.LimitReader(r.Body, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(data) == 0 {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("ag_id", agIDStr).
		Str("content_type", mediaType).
		Int("size_bytes", len(data)).
		Msg("Media upload received")

	// Seleciona processador baseado no tipo de mídia
	var processor multimodal.MediaProcessor
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		processor = mm.GetImageProcessor()
		if processor == nil {
			http.Error(w, "Image processing not available", http.StatusInternalServerError)
			return
		}
	case strings.HasPrefix(mediaType, "video/"):
		processor = mm.GetVideoProcessor()
		if processor == nil {
			http.Error(w, "Video processing not available", http.StatusInternalServerError)
			return
		}
	default:
		h.logger.Warn().Str("content_type", mediaType).Msg("Unsupported media type")
		http.Error(w, fmt.Sprintf("Unsupported media type: %s", mediaType), http.StatusUnsupportedMediaType)
		return
	}

	// Processa mídia
	ctx := r.Context()
	chunk, err := processor.Process(ctx, data)
	if err != nil {
		h.logger.Error().Err(err).Str("ag_id", agIDStr).Msg("Failed to process media")
		http.Error(w, fmt.Sprintf("Failed to process media: %v", err), http.StatusBadRequest)
		return
	}

	// Envia para Gemini via WebSocket
	if !session.Client.HasActiveConnection() {
		h.logger.Error().Str("ag_id", agIDStr).Msg("Gemini connection not active")
		http.Error(w, "Connection not active", http.StatusServiceUnavailable)
		return
	}

	// Converte multimodal.MediaChunk para gemini.MediaChunk
	geminiChunk := &gemini.MediaChunk{
		MimeType:  chunk.MimeType,
		Data:      chunk.Data,
		Timestamp: chunk.Timestamp,
		Metadata:  chunk.Metadata,
	}

	if err := session.Client.SendMediaChunk(ctx, geminiChunk); err != nil {
		h.logger.Error().Err(err).Str("ag_id", agIDStr).Msg("Failed to send media to Gemini")
		http.Error(w, "Failed to send media", http.StatusInternalServerError)
		return
	}

	// Adiciona ao buffer de memória visual (será processado na Fase 3)
	visualEntry := &multimodal.VisualMemoryEntry{
		SessionID: agIDStr,
		Timestamp: chunk.Timestamp,
		MediaType: multimodal.MediaType(chunk.MimeType),
		RawData:   data,
		// Embedding e Compressed serão preenchidos na Fase 3
	}
	mm.AddToMemoryBuffer(visualEntry)

	// Resposta de sucesso
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "processing",
		"media_type":  chunk.MimeType,
		"size_bytes":  len(data),
		"session_id":  agIDStr,
		"buffer_size": len(mm.GetMemoryBuffer()),
	})

	h.logger.Info().
		Str("ag_id", agIDStr).
		Str("media_type", chunk.MimeType).
		Msg("Media uploaded and sent to Gemini successfully")
}
