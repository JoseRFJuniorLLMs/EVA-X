package multimodal

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Interfaces para componentes de pipeline (implementados em EVA-Memory)
type VisualEmbedder interface {
	EmbedBatch(ctx context.Context, entries []*VisualMemoryEntry) error
}

type VisualKrylovManager interface {
	CompressBatch(ctx context.Context, entries []*VisualMemoryEntry) error
}

type VisualStorageManager interface {
	StoreBatch(ctx context.Context, entries []*VisualMemoryEntry) error
}

// MultimodalSession gerencia uma sessão multimodal com buffer de memória visual
type MultimodalSession struct {
	mu               sync.RWMutex
	sessionID        string
	config           *MultimodalConfig
	imageProcessor   MediaProcessor
	videoProcessor   MediaProcessor
	visualMemoryBuf  []*VisualMemoryEntry
	visualMemorySize int
	lastFrameTime    time.Time

	// Memory pipeline components (injected externally from EVA-Memory)
	embedder       VisualEmbedder
	krylovManager  VisualKrylovManager
	storageManager VisualStorageManager

	// Worker control
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerDone   chan struct{}
}

// NewMultimodalSession cria uma nova sessão multimodal
func NewMultimodalSession(sessionID string, config *MultimodalConfig) *MultimodalSession {
	if config == nil {
		config = DefaultMultimodalConfig()
	}

	return &MultimodalSession{
		sessionID:        sessionID,
		config:           config,
		visualMemoryBuf:  make([]*VisualMemoryEntry, 0, 100),
		visualMemorySize: 0,
		lastFrameTime:    time.Now(),
	}
}

// SetImageProcessor define o processador de imagens
func (s *MultimodalSession) SetImageProcessor(processor MediaProcessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imageProcessor = processor
}

// SetVideoProcessor define o processador de vídeo
func (s *MultimodalSession) SetVideoProcessor(processor MediaProcessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videoProcessor = processor
}

// GetImageProcessor retorna o processador de imagens
func (s *MultimodalSession) GetImageProcessor() MediaProcessor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.imageProcessor
}

// GetVideoProcessor retorna o processador de vídeo
func (s *MultimodalSession) GetVideoProcessor() MediaProcessor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoProcessor
}

// AddToMemoryBuffer adiciona uma entrada ao buffer de memória visual
func (s *MultimodalSession) AddToMemoryBuffer(entry *VisualMemoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.visualMemoryBuf = append(s.visualMemoryBuf, entry)
	s.visualMemorySize++

	log.Printf("📷 [MULTIMODAL] Added to buffer: session=%s, type=%s, buffer_size=%d",
		s.sessionID, entry.MediaType, len(s.visualMemoryBuf))
}

// GetMemoryBuffer retorna cópia do buffer atual
func (s *MultimodalSession) GetMemoryBuffer() []*VisualMemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Retorna cópia para evitar race conditions
	bufCopy := make([]*VisualMemoryEntry, len(s.visualMemoryBuf))
	copy(bufCopy, s.visualMemoryBuf)
	return bufCopy
}

// SetMemoryPipeline injeta componentes do pipeline de memória
func (s *MultimodalSession) SetMemoryPipeline(
	embedder VisualEmbedder,
	krylovManager VisualKrylovManager,
	storageManager VisualStorageManager,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.embedder = embedder
	s.krylovManager = krylovManager
	s.storageManager = storageManager

	log.Printf("✅ [MULTIMODAL] Memory pipeline configured for session=%s", s.sessionID)
}

// FlushMemoryBuffer processa e persiste o buffer de memória visual
// Pipeline: Embed → Compress (Krylov) → Store (Postgres + Qdrant)
func (s *MultimodalSession) FlushMemoryBuffer(ctx context.Context) error {
	s.mu.Lock()
	if len(s.visualMemoryBuf) == 0 {
		s.mu.Unlock()
		return nil // Nada a fazer
	}

	// Copia buffer e limpa
	entries := make([]*VisualMemoryEntry, len(s.visualMemoryBuf))
	copy(entries, s.visualMemoryBuf)
	s.visualMemoryBuf = s.visualMemoryBuf[:0] // Limpa buffer mantendo capacidade
	s.mu.Unlock()

	log.Printf("🔄 [MULTIMODAL] Flushing %d visual memories (session=%s)", len(entries), s.sessionID)

	// Verifica se pipeline está configurado
	if s.embedder == nil || s.krylovManager == nil || s.storageManager == nil {
		return fmt.Errorf("memory pipeline not configured")
	}

	startTime := time.Now()

	// Fase 1: Gerar embeddings 3072D
	if err := s.embedder.EmbedBatch(ctx, entries); err != nil {
		return fmt.Errorf("failed to embed batch: %w", err)
	}

	// Fase 2: Comprimir via Krylov 3072D → 64D
	if err := s.krylovManager.CompressBatch(ctx, entries); err != nil {
		return fmt.Errorf("failed to compress batch: %w", err)
	}

	// Fase 3: Persistir no Postgres + Qdrant
	if err := s.storageManager.StoreBatch(ctx, entries); err != nil {
		return fmt.Errorf("failed to store batch: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ [MULTIMODAL] Flush complete: %d memories, elapsed=%v (session=%s)",
		len(entries), elapsed, s.sessionID)

	return nil
}

// StartMemoryWorker inicia worker periódico para flush automático
// interval: intervalo entre flushes (ex: 30s)
func (s *MultimodalSession) StartMemoryWorker(ctx context.Context, interval time.Duration) {
	s.mu.Lock()

	// Cancela worker anterior se existir
	if s.workerCancel != nil {
		s.workerCancel()
		<-s.workerDone // Aguarda shutdown
	}

	s.workerCtx, s.workerCancel = context.WithCancel(ctx)
	s.workerDone = make(chan struct{})

	s.mu.Unlock()

	go s.memoryWorkerLoop(interval)

	log.Printf("▶️ [MULTIMODAL] Memory worker started: interval=%v, session=%s",
		interval, s.sessionID)
}

// StopMemoryWorker para o worker de memória
func (s *MultimodalSession) StopMemoryWorker() {
	s.mu.Lock()
	if s.workerCancel == nil {
		s.mu.Unlock()
		return
	}

	cancel := s.workerCancel
	done := s.workerDone
	s.mu.Unlock()

	cancel()
	<-done // Aguarda shutdown completo

	log.Printf("⏹️ [MULTIMODAL] Memory worker stopped: session=%s", s.sessionID)
}

// memoryWorkerLoop executa loop de flush periódico
func (s *MultimodalSession) memoryWorkerLoop(interval time.Duration) {
	defer close(s.workerDone)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.workerCtx.Done():
			// Flush final antes de parar
			if err := s.FlushMemoryBuffer(context.Background()); err != nil {
				log.Printf("⚠️ [MULTIMODAL] Final flush failed: %v", err)
			}
			return

		case <-ticker.C:
			// Flush periódico
			if err := s.FlushMemoryBuffer(s.workerCtx); err != nil {
				log.Printf("⚠️ [MULTIMODAL] Periodic flush failed: %v", err)
			}
		}
	}
}

// GetBufferSize retorna tamanho atual do buffer
func (s *MultimodalSession) GetBufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.visualMemoryBuf)
}

// GetConfig retorna configuração da sessão
func (s *MultimodalSession) GetConfig() *MultimodalConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// HasVisualContext verifica se sessão tem contexto visual
func (s *MultimodalSession) HasVisualContext() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.visualMemorySize > 0 || len(s.visualMemoryBuf) > 0
}

// GetStatistics retorna estatísticas da sessão
func (s *MultimodalSession) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"session_id":         s.sessionID,
		"buffer_size":        len(s.visualMemoryBuf),
		"total_memories":     s.visualMemorySize,
		"image_enabled":      s.config.EnableImageInput,
		"video_enabled":      s.config.EnableVideoInput,
		"worker_running":     s.workerCancel != nil,
		"pipeline_configured": s.embedder != nil && s.krylovManager != nil && s.storageManager != nil,
	}
}

// Close fecha a sessão e para worker
func (s *MultimodalSession) Close() error {
	// Para worker
	s.StopMemoryWorker()

	// Flush final (best effort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.FlushMemoryBuffer(ctx); err != nil {
		log.Printf("⚠️ [MULTIMODAL] Close flush failed: %v", err)
		return err
	}

	log.Printf("🔒 [MULTIMODAL] Session closed: session=%s", s.sessionID)
	return nil
}

