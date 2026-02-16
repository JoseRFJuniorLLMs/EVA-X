package voice

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// profileCache mantém os perfis de voz em memória com TTL configurável.
// Evita round-trips ao Neo4j em cada identificação (que acontece a cada ~3s).
//
// Thread-safe. Suporta invalidação manual (ex: após novo enroll).

type profileCache struct {
	mu        sync.RWMutex
	profiles  []VoiceProfile
	loadedAt  time.Time
	ttl       time.Duration
	store     *Neo4jStore
	log       *zap.Logger
	loading   bool
	loadCond  *sync.Cond
}

func newProfileCache(store *Neo4jStore, ttl time.Duration, log *zap.Logger) *profileCache {
	c := &profileCache{
		ttl:   ttl,
		store: store,
		log:   log,
	}
	c.loadCond = sync.NewCond(&c.mu)
	return c
}

// Get retorna os perfis do cache. Se expirado, recarrega do Neo4j.
// Múltiplas goroutines esperando pelo mesmo reload recebem o mesmo resultado
// (evita thundering herd).
func (c *profileCache) Get(ctx context.Context) ([]VoiceProfile, error) {
	c.mu.RLock()
	if !c.isExpired() {
		profiles := c.profiles
		c.mu.RUnlock()
		return profiles, nil
	}
	c.mu.RUnlock()

	// Upgrade para write lock e recarrega
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: outra goroutine pode ter recarregado enquanto esperávamos
	if !c.isExpired() {
		return c.profiles, nil
	}

	// Se já está carregando, espera
	for c.loading {
		c.loadCond.Wait()
	}
	if !c.isExpired() {
		return c.profiles, nil
	}

	c.loading = true
	// Carrega fora do lock para não bloquear readers existentes
	c.mu.Unlock()

	profiles, err := c.store.LoadActiveProfiles(ctx)

	c.mu.Lock()
	c.loading = false
	c.loadCond.Broadcast()

	if err != nil {
		c.log.Error("profile cache reload failed", zap.Error(err))
		// Retorna cache antigo se disponível
		return c.profiles, err
	}

	c.profiles = profiles
	c.loadedAt = time.Now()
	c.log.Debug("profile cache refreshed", zap.Int("count", len(profiles)))
	return c.profiles, nil
}

// Invalidate força o próximo Get() a recarregar do Neo4j.
// Deve ser chamado após UpsertProfile ou DeleteProfile.
func (c *profileCache) Invalidate() {
	c.mu.Lock()
	c.loadedAt = time.Time{} // Zero time → sempre expirado
	c.mu.Unlock()
}

func (c *profileCache) isExpired() bool {
	return time.Since(c.loadedAt) > c.ttl
}
