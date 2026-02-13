package knowledge

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ============================================================================
// PERFORMANCE FIX: Cache de Embeddings
// Issue: Cada texto gera nova chamada Gemini API (custo + latencia)
// Fix: Cache Redis com TTL 24h + cache local LRU
// Impacto esperado: 90% reducao em chamadas de API
// ============================================================================

// EmbeddingCache gerencia cache de embeddings em dois niveis
type EmbeddingCache struct {
	redis     *redis.Client
	local     *localCache
	ttl       time.Duration
	hits      int64
	misses    int64
	mu        sync.RWMutex
}

// localCache implementa cache LRU em memoria
type localCache struct {
	items    map[string]*cacheItem
	maxItems int
	mu       sync.RWMutex
}

type cacheItem struct {
	embedding []float32
	expiresAt time.Time
}

// NewEmbeddingCache cria um novo cache de embeddings
func NewEmbeddingCache(redisClient *redis.Client) *EmbeddingCache {
	return &EmbeddingCache{
		redis: redisClient,
		local: &localCache{
			items:    make(map[string]*cacheItem),
			maxItems: 1000, // Cache local com max 1000 embeddings
		},
		ttl: 24 * time.Hour,
	}
}

// cacheKey gera chave unica para o texto
func (c *EmbeddingCache) cacheKey(text string) string {
	hash := md5.Sum([]byte(text))
	return "emb:" + hex.EncodeToString(hash[:])
}

// Get busca embedding no cache (local primeiro, depois Redis)
func (c *EmbeddingCache) Get(ctx context.Context, text string) ([]float32, bool) {
	key := c.cacheKey(text)

	// 1. Tentar cache local primeiro (mais rapido)
	if emb, ok := c.local.get(key); ok {
		c.recordHit()
		return emb, true
	}

	// 2. Tentar Redis se nao encontrou localmente
	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var embedding []float32
			if err := json.Unmarshal(data, &embedding); err == nil {
				// Atualizar cache local
				c.local.set(key, embedding, c.ttl)
				c.recordHit()
				return embedding, true
			}
		}
	}

	c.recordMiss()
	return nil, false
}

// Set salva embedding no cache (ambos niveis)
func (c *EmbeddingCache) Set(ctx context.Context, text string, embedding []float32) {
	key := c.cacheKey(text)

	// 1. Salvar no cache local
	c.local.set(key, embedding, c.ttl)

	// 2. Salvar no Redis (async para nao bloquear)
	if c.redis != nil {
		go func() {
			data, err := json.Marshal(embedding)
			if err != nil {
				return
			}
			if err := c.redis.Set(ctx, key, data, c.ttl).Err(); err != nil {
				log.Printf("⚠️ [CACHE] Erro ao salvar embedding no Redis: %v", err)
			}
		}()
	}
}

// GetStats retorna estatisticas do cache
func (c *EmbeddingCache) GetStats() (hits, misses int64, hitRate float64) {
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

func (c *EmbeddingCache) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

func (c *EmbeddingCache) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

// ============================================================================
// Local Cache Implementation (LRU simples)
// ============================================================================

func (lc *localCache) get(key string) ([]float32, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	item, ok := lc.items[key]
	if !ok {
		return nil, false
	}

	// Verificar expiracao
	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.embedding, true
}

func (lc *localCache) set(key string, embedding []float32, ttl time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Eviction simples: se atingiu limite, remove item mais antigo
	if len(lc.items) >= lc.maxItems {
		lc.evictOldest()
	}

	lc.items[key] = &cacheItem{
		embedding: embedding,
		expiresAt: time.Now().Add(ttl),
	}
}

func (lc *localCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range lc.items {
		if oldestKey == "" || item.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiresAt
		}
	}

	if oldestKey != "" {
		delete(lc.items, oldestKey)
	}
}

// ============================================================================
// Signifier Cache (para FindRelatedSignifiers)
// ============================================================================

// SignifierCache gerencia cache de cadeias de significantes
type SignifierCache struct {
	redis *redis.Client
	local *sync.Map
	ttl   time.Duration
}

// NewSignifierCache cria cache para signifiers
func NewSignifierCache(redisClient *redis.Client) *SignifierCache {
	return &SignifierCache{
		redis: redisClient,
		local: &sync.Map{},
		ttl:   5 * time.Minute, // TTL menor pois muda mais frequentemente
	}
}

// signifierKey gera chave para cache de signifiers
func (sc *SignifierCache) signifierKey(idosoID int64, text string) string {
	hash := md5.Sum([]byte(text))
	return "sig:" + string(rune(idosoID)) + ":" + hex.EncodeToString(hash[:8])
}

// GetSignifiers busca signifiers cacheados
func (sc *SignifierCache) GetSignifiers(ctx context.Context, idosoID int64, text string) ([]SignifierChain, bool) {
	key := sc.signifierKey(idosoID, text)

	// Cache local
	if val, ok := sc.local.Load(key); ok {
		return val.([]SignifierChain), true
	}

	// Redis
	if sc.redis != nil {
		data, err := sc.redis.Get(ctx, key).Bytes()
		if err == nil {
			var chains []SignifierChain
			if err := json.Unmarshal(data, &chains); err == nil {
				sc.local.Store(key, chains)
				return chains, true
			}
		}
	}

	return nil, false
}

// SetSignifiers salva signifiers no cache
func (sc *SignifierCache) SetSignifiers(ctx context.Context, idosoID int64, text string, chains []SignifierChain) {
	key := sc.signifierKey(idosoID, text)

	sc.local.Store(key, chains)

	if sc.redis != nil {
		go func() {
			data, err := json.Marshal(chains)
			if err != nil {
				return
			}
			sc.redis.Set(ctx, key, data, sc.ttl)
		}()
	}
}
