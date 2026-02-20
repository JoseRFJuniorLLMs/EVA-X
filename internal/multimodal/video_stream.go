// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package multimodal

import (
	"context"
	"eva/internal/gemini"
	"fmt"
	"log"
	"sync"
	"time"
)

// VideoFrame representa um frame de vídeo
type VideoFrame struct {
	Data      []byte
	MimeType  string
	Timestamp time.Time
	SessionID string
}

// VideoStreamManager gerencia streaming de vídeo em tempo real
type VideoStreamManager struct {
	mu            sync.RWMutex
	sessionID     string
	geminiClient  *gemini.Client
	frameBuffer   chan *VideoFrame
	config        *VideoStreamConfig
	isStreaming   bool
	workerCtx     context.Context
	workerCancel  context.CancelFunc
	workerDone    chan struct{}
	framesDropped int64
	framesSent    int64
	lastFrameTime time.Time
}

// VideoStreamConfig configuração de streaming de vídeo
type VideoStreamConfig struct {
	BufferSize     int           // Tamanho do buffer de frames
	MaxFPS         float64       // FPS máximo (rate limiting)
	Quality        int           // Qualidade de compressão JPEG (1-100)
	EnableMemory   bool          // Se deve adicionar frames ao buffer de memória visual
	BatchSize      int           // Quantos frames enviar em batch para Gemini
	ProcessTimeout time.Duration // Timeout para processar cada frame
}

// DefaultVideoStreamConfig retorna configuração padrão
func DefaultVideoStreamConfig() *VideoStreamConfig {
	return &VideoStreamConfig{
		BufferSize:     30,              // 30 frames buffer
		MaxFPS:         1.0,              // 1 FPS (1 frame por segundo)
		Quality:        75,               // JPEG quality 75
		EnableMemory:   true,             // Salva frames em memória visual
		BatchSize:      5,                // Envia 5 frames por vez
		ProcessTimeout: 5 * time.Second,  // 5s timeout por frame
	}
}

// NewVideoStreamManager cria um novo gerenciador de stream de vídeo
func NewVideoStreamManager(
	sessionID string,
	geminiClient *gemini.Client,
	config *VideoStreamConfig,
) *VideoStreamManager {
	if config == nil {
		config = DefaultVideoStreamConfig()
	}

	return &VideoStreamManager{
		sessionID:    sessionID,
		geminiClient: geminiClient,
		frameBuffer:  make(chan *VideoFrame, config.BufferSize),
		config:       config,
		isStreaming:  false,
	}
}

// StartStream inicia o streaming de vídeo
func (vsm *VideoStreamManager) StartStream(ctx context.Context) error {
	vsm.mu.Lock()
	defer vsm.mu.Unlock()

	if vsm.isStreaming {
		return fmt.Errorf("stream already running")
	}

	// Cria contexto para worker
	vsm.workerCtx, vsm.workerCancel = context.WithCancel(ctx)
	vsm.workerDone = make(chan struct{})
	vsm.isStreaming = true
	vsm.framesDropped = 0
	vsm.framesSent = 0
	vsm.lastFrameTime = time.Now()

	// Inicia worker para processar frames
	go vsm.frameProcessorWorker()

	log.Printf("▶️ [VIDEO_STREAM] Started: session=%s, max_fps=%.1f, buffer=%d",
		vsm.sessionID, vsm.config.MaxFPS, vsm.config.BufferSize)

	return nil
}

// StopStream para o streaming de vídeo
func (vsm *VideoStreamManager) StopStream() error {
	vsm.mu.Lock()

	if !vsm.isStreaming {
		vsm.mu.Unlock()
		return fmt.Errorf("stream not running")
	}

	cancel := vsm.workerCancel
	done := vsm.workerDone
	vsm.isStreaming = false

	vsm.mu.Unlock()

	// Para worker
	cancel()
	<-done // Aguarda shutdown

	log.Printf("⏹️ [VIDEO_STREAM] Stopped: session=%s, sent=%d, dropped=%d",
		vsm.sessionID, vsm.framesSent, vsm.framesDropped)

	return nil
}

// PushFrame adiciona um frame ao buffer (non-blocking)
func (vsm *VideoStreamManager) PushFrame(frame *VideoFrame) error {
	vsm.mu.RLock()
	if !vsm.isStreaming {
		vsm.mu.RUnlock()
		return fmt.Errorf("stream not running")
	}
	vsm.mu.RUnlock()

	// Rate limiting: verifica FPS
	vsm.mu.Lock()
	minInterval := time.Duration(float64(time.Second) / vsm.config.MaxFPS)
	elapsed := time.Since(vsm.lastFrameTime)
	if elapsed < minInterval {
		vsm.mu.Unlock()
		// Frame descartado por rate limiting
		vsm.mu.Lock()
		vsm.framesDropped++
		vsm.mu.Unlock()
		return nil // Não é erro, apenas throttling
	}
	vsm.lastFrameTime = time.Now()
	vsm.mu.Unlock()

	// Tenta adicionar ao buffer (non-blocking)
	select {
	case vsm.frameBuffer <- frame:
		return nil
	default:
		// Buffer cheio - dropa frame mais antigo
		vsm.mu.Lock()
		vsm.framesDropped++
		vsm.mu.Unlock()
		log.Printf("⚠️ [VIDEO_STREAM] Buffer full, frame dropped: session=%s", vsm.sessionID)
		return nil // Não é erro fatal
	}
}

// frameProcessorWorker processa frames do buffer e envia para Gemini
func (vsm *VideoStreamManager) frameProcessorWorker() {
	defer close(vsm.workerDone)

	batch := make([]*MediaChunk, 0, vsm.config.BatchSize)

	for {
		select {
		case <-vsm.workerCtx.Done():
			// Processa batch final se houver
			if len(batch) > 0 {
				vsm.sendBatchToGemini(context.Background(), batch)
			}
			return

		case frame := <-vsm.frameBuffer:
			// Processa frame
			chunk, err := vsm.processFrame(frame)
			if err != nil {
				log.Printf("⚠️ [VIDEO_STREAM] Failed to process frame: %v", err)
				continue
			}

			// Adiciona ao batch
			batch = append(batch, chunk)

			// Envia batch se atingiu tamanho ou timeout
			if len(batch) >= vsm.config.BatchSize {
				vsm.sendBatchToGemini(vsm.workerCtx, batch)
				batch = batch[:0] // Limpa batch
			}
		}
	}
}

// processFrame processa um frame individual
func (vsm *VideoStreamManager) processFrame(frame *VideoFrame) (*MediaChunk, error) {
	// Cria MediaChunk (já está em formato base64 se vier do WebSocket)
	chunk := &MediaChunk{
		MimeType:  frame.MimeType,
		Data:      string(frame.Data), // Assume já base64
		Timestamp: frame.Timestamp,
		Metadata: map[string]interface{}{
			"session_id": vsm.sessionID,
			"frame_type": "video_stream",
		},
	}

	return chunk, nil
}

// sendBatchToGemini envia batch de frames para Gemini via WebSocket
func (vsm *VideoStreamManager) sendBatchToGemini(ctx context.Context, batch []*MediaChunk) {
	if len(batch) == 0 {
		return
	}

	// Verifica se client está ativo
	if !vsm.geminiClient.HasActiveConnection() {
		log.Printf("⚠️ [VIDEO_STREAM] Gemini connection not active, skipping batch")
		return
	}

	// Converte MediaChunk (multimodal) → gemini.MediaChunk
	geminiChunks := make([]*gemini.MediaChunk, len(batch))
	for i, chunk := range batch {
		geminiChunks[i] = &gemini.MediaChunk{
			MimeType:  chunk.MimeType,
			Data:      chunk.Data,
			Timestamp: chunk.Timestamp,
			Metadata:  chunk.Metadata,
		}
	}

	// Envia batch
	if err := vsm.geminiClient.SendMediaBatch(ctx, geminiChunks); err != nil {
		log.Printf("❌ [VIDEO_STREAM] Failed to send batch: %v", err)
		return
	}

	vsm.mu.Lock()
	vsm.framesSent += int64(len(batch))
	vsm.mu.Unlock()

	log.Printf("📤 [VIDEO_STREAM] Batch sent: session=%s, frames=%d, total_sent=%d",
		vsm.sessionID, len(batch), vsm.framesSent)
}

// GetStatistics retorna estatísticas do streaming
func (vsm *VideoStreamManager) GetStatistics() map[string]interface{} {
	vsm.mu.RLock()
	defer vsm.mu.RUnlock()

	return map[string]interface{}{
		"session_id":     vsm.sessionID,
		"is_streaming":   vsm.isStreaming,
		"frames_sent":    vsm.framesSent,
		"frames_dropped": vsm.framesDropped,
		"buffer_size":    len(vsm.frameBuffer),
		"buffer_cap":     cap(vsm.frameBuffer),
		"max_fps":        vsm.config.MaxFPS,
	}
}

// IsStreaming verifica se está streamando
func (vsm *VideoStreamManager) IsStreaming() bool {
	vsm.mu.RLock()
	defer vsm.mu.RUnlock()
	return vsm.isStreaming
}

// GetConfig retorna configuração
func (vsm *VideoStreamManager) GetConfig() *VideoStreamConfig {
	vsm.mu.RLock()
	defer vsm.mu.RUnlock()
	return vsm.config
}

// UpdateConfig atualiza configuração em runtime (apenas se não estiver streamando)
func (vsm *VideoStreamManager) UpdateConfig(config *VideoStreamConfig) error {
	vsm.mu.Lock()
	defer vsm.mu.Unlock()

	if vsm.isStreaming {
		return fmt.Errorf("cannot update config while streaming")
	}

	vsm.config = config
	vsm.frameBuffer = make(chan *VideoFrame, config.BufferSize)

	log.Printf("✅ [VIDEO_STREAM] Config updated: session=%s", vsm.sessionID)
	return nil
}
