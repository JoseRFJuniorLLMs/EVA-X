// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"
	"sync"
	"time"

	"eva/internal/brainstem/logger"
)

// ── AudioBuffer — persisted in NietzscheDB Lists, in-memory fallback ────────

const (
	audioSessionTTL    = 1 * time.Hour
	audioListName      = "audio_chunks"
	audioCollection    = "eva_core"
	cacheCollection    = "eva_core"
)

// AudioBuffer stores audio chunks per session using NietzscheDB ListRPush/ListLRange.
// Falls back to in-memory storage if NietzscheDB is unavailable.
// When a SensoryAdapter is attached, FlushToSensory can persist accumulated audio
// as compressed sensory data via the NietzscheDB InsertSensory RPC.
type AudioBuffer struct {
	client  *Client         // nil → in-memory-only mode
	sensory *SensoryAdapter // nil → sensory compression disabled

	mu       sync.Mutex
	sessions map[string]*audioSession // in-memory fallback
}

type audioSession struct {
	chunks    [][]byte
	createdAt time.Time
}

// NewAudioBuffer creates an audio buffer backed by NietzscheDB.
// If client is nil, operates in in-memory-only mode (legacy behavior).
// The provided context controls the lifetime of the background cleanup goroutine.
func NewAudioBuffer(ctx context.Context, client *Client) *AudioBuffer {
	ab := &AudioBuffer{
		client:   client,
		sessions: make(map[string]*audioSession),
	}
	go ab.cleanupLoop(ctx)
	return ab
}

// SetSensoryAdapter attaches a SensoryAdapter for persisting accumulated audio
// as compressed sensory data. This enables FlushToSensory to delegate audio
// storage to the multi-modal sensory compression layer.
func (ab *AudioBuffer) SetSensoryAdapter(adapter *SensoryAdapter) {
	ab.sensory = adapter
}

// FlushToSensory retrieves all buffered audio for a session and stores it as
// compressed audio sensory data via the SensoryAdapter. The audio buffer is NOT
// cleared (callers can still retrieve raw chunks via GetFullAudio).
// Returns an error if no SensoryAdapter is attached or if the store fails.
func (ab *AudioBuffer) FlushToSensory(ctx context.Context, sessionID, nodeID, encoderVersion string) error {
	if ab.sensory == nil {
		return fmt.Errorf("audio buffer: no SensoryAdapter attached, cannot flush to sensory layer")
	}

	log := logger.Nietzsche()

	// Retrieve all accumulated audio chunks
	fullAudio, err := ab.GetFullAudio(ctx, sessionID, false)
	if err != nil {
		return fmt.Errorf("audio buffer flush to sensory: get full audio: %w", err)
	}
	if len(fullAudio) == 0 {
		log.Debug().Str("session", sessionID).Msg("[AudioBuffer] no audio data to flush")
		return nil
	}

	// Delegate to the SensoryAdapter for audio-modality storage
	err = ab.sensory.StoreAudioSensory(ctx, audioCollection, nodeID, fullAudio, encoderVersion)
	if err != nil {
		return fmt.Errorf("audio buffer flush to sensory: store: %w", err)
	}

	log.Info().
		Str("session", sessionID).
		Str("node_id", nodeID).
		Int("audio_bytes", len(fullAudio)).
		Msg("[AudioBuffer] audio flushed to sensory layer")
	return nil
}

// AppendAudioChunk appends an audio chunk to the session.
// Uses NietzscheDB ListRPush if available, falls back to in-memory.
func (ab *AudioBuffer) AppendAudioChunk(ctx context.Context, sessionID string, data []byte) error {
	// Copy data to avoid holding references to caller's buffer
	chunk := make([]byte, len(data))
	copy(chunk, data)

	// Try NietzscheDB first
	if ab.client != nil {
		_, err := ab.client.ListRPush(ctx, sessionID, audioListName, chunk, audioCollection)
		if err == nil {
			return nil
		}
		// Fallback to in-memory on error
		log := logger.Nietzsche()
		log.Debug().Err(err).Str("session", sessionID).Msg("[AudioBuffer] ListRPush failed, using in-memory fallback")
	}

	// In-memory fallback
	ab.mu.Lock()
	defer ab.mu.Unlock()

	sess, ok := ab.sessions[sessionID]
	if !ok {
		sess = &audioSession{createdAt: time.Now()}
		ab.sessions[sessionID] = sess
	}
	sess.chunks = append(sess.chunks, chunk)
	return nil
}

// GetFullAudio retrieves all audio chunks for a session, optionally clearing them.
// Tries NietzscheDB first, falls back to in-memory.
func (ab *AudioBuffer) GetFullAudio(ctx context.Context, sessionID string, clear bool) ([]byte, error) {
	// Try NietzscheDB first
	if ab.client != nil {
		values, err := ab.client.ListLRange(ctx, sessionID, audioListName, 0, -1, audioCollection)
		if err == nil && len(values) > 0 {
			totalSize := 0
			for _, v := range values {
				totalSize += len(v)
			}
			fullAudio := make([]byte, 0, totalSize)
			for _, v := range values {
				fullAudio = append(fullAudio, v...)
			}
			// Note: clear is not supported via NietzscheDB Lists (no LDel RPC).
			// Chunks naturally expire with the session node.
			return fullAudio, nil
		}
	}

	// In-memory fallback
	ab.mu.Lock()
	defer ab.mu.Unlock()

	sess, ok := ab.sessions[sessionID]
	if !ok {
		return []byte{}, nil
	}

	totalSize := 0
	for _, chunk := range sess.chunks {
		totalSize += len(chunk)
	}

	fullAudio := make([]byte, 0, totalSize)
	for _, chunk := range sess.chunks {
		fullAudio = append(fullAudio, chunk...)
	}

	if clear {
		delete(ab.sessions, sessionID)
	}

	return fullAudio, nil
}

// cleanupLoop removes expired in-memory sessions every 5 minutes.
// It exits when ctx is cancelled, preventing goroutine leaks.
func (ab *AudioBuffer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ab.mu.Lock()
			now := time.Now()
			for id, sess := range ab.sessions {
				if now.Sub(sess.createdAt) > audioSessionTTL {
					delete(ab.sessions, id)
				}
			}
			ab.mu.Unlock()
		}
	}
}

// Close is a no-op (satisfies interface compatibility).
func (ab *AudioBuffer) Close() error {
	return nil
}

// SessionCount returns the number of active in-memory audio sessions (for metrics).
func (ab *AudioBuffer) SessionCount() int {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	return len(ab.sessions)
}

// ── CacheStore — persisted in NietzscheDB Cache, in-memory fallback ─────────

// CacheStore provides TTL-based caching using NietzscheDB CacheSet/CacheGet.
// Falls back to in-memory if NietzscheDB is unavailable.
type CacheStore struct {
	client *Client // nil → in-memory-only mode

	mu    sync.RWMutex
	items map[string]*cacheItem // in-memory fallback
}

type cacheItem struct {
	value     string
	expiresAt time.Time
}

// NewCacheStore creates a cache store backed by NietzscheDB.
// If client is nil, operates in in-memory-only mode (legacy behavior).
// The provided context controls the lifetime of the background cleanup goroutine.
func NewCacheStore(ctx context.Context, client *Client) *CacheStore {
	cs := &CacheStore{
		client: client,
		items:  make(map[string]*cacheItem),
	}
	go cs.cleanupLoop(ctx)
	return cs
}

// Set stores a value with expiration.
// Uses NietzscheDB CacheSet if available, falls back to in-memory.
func (cs *CacheStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	strVal := fmt.Sprintf("%v", value)

	// Try NietzscheDB first
	if cs.client != nil {
		ttlSecs := uint64(expiration.Seconds())
		if ttlSecs == 0 {
			ttlSecs = 3600 // default 1h
		}
		err := cs.client.CacheSet(ctx, cacheCollection, key, []byte(strVal), ttlSecs)
		if err == nil {
			return nil
		}
		log := logger.Nietzsche()
		log.Debug().Err(err).Str("key", key).Msg("[CacheStore] CacheSet failed, using in-memory fallback")
	}

	// In-memory fallback
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.items[key] = &cacheItem{
		value:     strVal,
		expiresAt: time.Now().Add(expiration),
	}
	return nil
}

// Get retrieves a value. Returns ("", error) if not found or expired.
// Tries NietzscheDB first, falls back to in-memory.
func (cs *CacheStore) Get(ctx context.Context, key string) (string, error) {
	// Try NietzscheDB first
	if cs.client != nil {
		val, found, err := cs.client.CacheGet(ctx, cacheCollection, key)
		if err == nil && found {
			return string(val), nil
		}
	}

	// In-memory fallback
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	item, ok := cs.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return "", fmt.Errorf("cache miss: %s", key)
	}
	return item.value, nil
}

// Close is a no-op.
func (cs *CacheStore) Close() error {
	return nil
}

func (cs *CacheStore) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cs.mu.Lock()
			now := time.Now()
			for k, item := range cs.items {
				if now.After(item.expiresAt) {
					delete(cs.items, k)
				}
			}
			cs.mu.Unlock()
		}
	}
}
