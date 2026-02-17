// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MediaChunk representa um chunk de mídia (definido localmente para evitar import cycle)
type MediaChunk struct {
	MimeType  string                 `json:"mime_type"`
	Data      string                 `json:"data"` // base64
	Timestamp time.Time              `json:"-"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MultimodalMessage representa uma mensagem multimodal para o Gemini Live API
// Formato conforme documentação: {"realtime_input": {"media_chunks": [...]}}
type MultimodalMessage struct {
	RealtimeInput struct {
		MediaChunks []MediaChunk `json:"media_chunks"`
	} `json:"realtime_input"`
}

// SendMediaChunk envia um chunk de mídia (imagem/vídeo) para o Gemini Live API
// Usa o mesmo WebSocket do áudio, garantindo que não há interferência
func (c *Client) SendMediaChunk(ctx context.Context, chunk *MediaChunk) error {
	if c.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}

	// Cria payload multimodal
	msg := MultimodalMessage{}
	msg.RealtimeInput.MediaChunks = []MediaChunk{*chunk}

	// Envia via WriteJSON (método existente do Client)
	if err := c.WriteJSON(msg); err != nil {
		log.Printf("❌ Failed to send media chunk: %v", err)
		return fmt.Errorf("failed to send media chunk: %w", err)
	}

	log.Printf("📤 Media chunk sent to Gemini (mime_type=%s, size=%d bytes)",
		chunk.MimeType, len(chunk.Data))

	return nil
}

// SendMediaBatch envia múltiplos chunks de mídia de uma vez (otimização)
// Útil para enviar frames de vídeo em batch
func (c *Client) SendMediaBatch(ctx context.Context, chunks []*MediaChunk) error {
	if c.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	if len(chunks) == 0 {
		return nil // Nada a fazer
	}

	// Cria payload com múltiplos chunks
	msg := MultimodalMessage{}
	for _, chunk := range chunks {
		if chunk != nil {
			msg.RealtimeInput.MediaChunks = append(msg.RealtimeInput.MediaChunks, *chunk)
		}
	}

	if len(msg.RealtimeInput.MediaChunks) == 0 {
		return nil // Todos chunks eram nil
	}

	// Envia via WriteJSON
	if err := c.WriteJSON(msg); err != nil {
		log.Printf("❌ Failed to send media batch: %v", err)
		return fmt.Errorf("failed to send batch: %w", err)
	}

	log.Printf("📤 Media batch sent to Gemini (%d chunks)", len(msg.RealtimeInput.MediaChunks))

	return nil
}

// HasActiveConnection verifica se a conexão WebSocket está ativa
// Útil para verificar antes de enviar mídia
func (c *Client) HasActiveConnection() bool {
	return c.conn != nil
}
