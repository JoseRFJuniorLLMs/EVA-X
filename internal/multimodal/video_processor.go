// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package multimodal

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

// VideoMetadata contém informações sobre o vídeo
type VideoMetadata struct {
	Duration  time.Duration
	Width     int
	Height    int
	FrameRate float64
	Format    string
	SizeMB    float64
}

// FrameExtractor interface para abstrair extração de frames
// Implementação real (FFmpeg wrapper) será feita na Fase 5
type FrameExtractor interface {
	ExtractFrame(videoData []byte, timestamp time.Duration) ([]byte, error)
	GetMetadata(videoData []byte) (*VideoMetadata, error)
}

// VideoProcessor processa vídeos para envio ao Gemini
type VideoProcessor struct {
	config         *MultimodalConfig
	frameExtractor FrameExtractor // Opcional - nil na Fase 1
}

// NewVideoProcessor cria novo processador de vídeo
func NewVideoProcessor(config *MultimodalConfig, extractor FrameExtractor) *VideoProcessor {
	if config == nil {
		config = DefaultMultimodalConfig()
	}
	return &VideoProcessor{
		config:         config,
		frameExtractor: extractor,
	}
}

// GetType retorna o tipo de mídia
func (p *VideoProcessor) GetType() MediaType {
	return MediaTypeVideo
}

// Validate valida o vídeo (tamanho básico)
func (p *VideoProcessor) Validate(input []byte) error {
	// Valida tamanho
	sizeMB := float64(len(input)) / (1024 * 1024)
	if sizeMB > float64(p.config.MaxVideoSizeMB) {
		return fmt.Errorf("video size %.2fMB exceeds limit %dMB",
			sizeMB, p.config.MaxVideoSizeMB)
	}

	// Se temos extractor, valida metadata
	if p.frameExtractor != nil {
		metadata, err := p.frameExtractor.GetMetadata(input)
		if err != nil {
			return fmt.Errorf("invalid video: %w", err)
		}

		// Valida duração (max 10 min default)
		if metadata.Duration > 10*time.Minute {
			return fmt.Errorf("video duration %v exceeds 10 minutes", metadata.Duration)
		}
	}

	return nil
}

// Process processa o vídeo e retorna MediaChunk
// Na Fase 1, apenas faz base64 encode do vídeo completo
// Fase 5 implementará extração de frames se necessário
func (p *VideoProcessor) Process(ctx context.Context, input []byte) (*MediaChunk, error) {
	if err := p.Validate(input); err != nil {
		return nil, err
	}

	// Para vídeo, enviamos o arquivo completo (Gemini Live API processa internamente)
	encoded := base64.StdEncoding.EncodeToString(input)

	metadata := make(map[string]interface{})
	metadata["size_bytes"] = len(input)

	// Se temos extractor, adiciona metadata detalhada
	if p.frameExtractor != nil {
		videoMeta, err := p.frameExtractor.GetMetadata(input)
		if err == nil {
			metadata["duration_sec"] = videoMeta.Duration.Seconds()
			metadata["width"] = videoMeta.Width
			metadata["height"] = videoMeta.Height
			metadata["frame_rate"] = videoMeta.FrameRate
			metadata["format"] = videoMeta.Format
		}
	}

	return &MediaChunk{
		MimeType:  "video/mp4",
		Data:      encoded,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}, nil
}
