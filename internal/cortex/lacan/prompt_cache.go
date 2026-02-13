package lacan

import (
	"sync"
	"time"
)

// ============================================================================
// PERFORMANCE FIX: Cache de Prompts
// Issue: Prompt de 85-100KB regenerado a cada start_call
// Fix: Cache com TTL de 5 minutos (prompts mudam apenas quando medicamentos mudam)
// Impacto esperado: -70% latência no setupGemini
// ============================================================================

// CachedPrompt armazena um prompt com timestamp
type CachedPrompt struct {
	Prompt    string
	CreatedAt time.Time
}

// PromptCache gerencia cache de prompts por idoso
type PromptCache struct {
	mu    sync.RWMutex
	data  map[int64]*CachedPrompt
	ttl   time.Duration
	hits  int64
	misses int64
}

// NewPromptCache cria um novo cache de prompts
func NewPromptCache(ttl time.Duration) *PromptCache {
	if ttl == 0 {
		ttl = 5 * time.Minute // Default 5 minutos
	}

	cache := &PromptCache{
		data: make(map[int64]*CachedPrompt),
		ttl:  ttl,
	}

	// Goroutine para limpar entradas expiradas a cada minuto
	go cache.cleanupLoop()

	return cache
}

// Get recupera um prompt do cache se válido
func (c *PromptCache) Get(idosoID int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.data[idosoID]
	if !ok {
		c.misses++
		return "", false
	}

	// Verificar TTL
	if time.Since(cached.CreatedAt) > c.ttl {
		c.misses++
		return "", false
	}

	c.hits++
	return cached.Prompt, true
}

// Set salva um prompt no cache
func (c *PromptCache) Set(idosoID int64, prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[idosoID] = &CachedPrompt{
		Prompt:    prompt,
		CreatedAt: time.Now(),
	}
}

// Invalidate remove um prompt do cache (quando medicamentos mudam)
func (c *PromptCache) Invalidate(idosoID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, idosoID)
}

// InvalidateAll limpa todo o cache
func (c *PromptCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[int64]*CachedPrompt)
}

// GetStats retorna estatísticas do cache
func (c *PromptCache) GetStats() (hits, misses int64, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits = c.hits
	misses = c.misses
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	return
}

// cleanupLoop remove entradas expiradas periodicamente
func (c *PromptCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup remove entradas expiradas
func (c *PromptCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, cached := range c.data {
		if now.Sub(cached.CreatedAt) > c.ttl {
			delete(c.data, id)
		}
	}
}
