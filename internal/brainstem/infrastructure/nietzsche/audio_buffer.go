// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AudioBuffer replaces Redis RPush/LRange for audio streaming.
// Audio sessions are short-lived (~5 min), so in-memory with TTL is sufficient.
type AudioBuffer struct {
	mu       sync.Mutex
	sessions map[string]*audioSession
}

type audioSession struct {
	chunks    [][]byte
	createdAt time.Time
}

const audioSessionTTL = 1 * time.Hour

// NewAudioBuffer creates an in-memory audio buffer (replaces Redis).
func NewAudioBuffer() *AudioBuffer {
	ab := &AudioBuffer{
		sessions: make(map[string]*audioSession),
	}
	go ab.cleanupLoop()
	return ab
}

// AppendAudioChunk adds an audio chunk to the session buffer.
func (ab *AudioBuffer) AppendAudioChunk(_ context.Context, sessionID string, data []byte) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	sess, ok := ab.sessions[sessionID]
	if !ok {
		sess = &audioSession{createdAt: time.Now()}
		ab.sessions[sessionID] = sess
	}

	// Copy data to avoid holding references to caller's buffer
	chunk := make([]byte, len(data))
	copy(chunk, data)
	sess.chunks = append(sess.chunks, chunk)
	return nil
}

// GetFullAudio retrieves all audio chunks for a session, optionally clearing them.
func (ab *AudioBuffer) GetFullAudio(_ context.Context, sessionID string, clear bool) ([]byte, error) {
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

// cleanupLoop removes expired sessions every 5 minutes.
func (ab *AudioBuffer) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
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

// Close is a no-op (satisfies interface compatibility).
func (ab *AudioBuffer) Close() error {
	return nil
}

// SessionCount returns the number of active audio sessions (for metrics).
func (ab *AudioBuffer) SessionCount() int {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	return len(ab.sessions)
}

// CacheStore replaces Redis Set/Get with TTL for generic caching.
type CacheStore struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	value     string
	expiresAt time.Time
}

// NewCacheStore creates an in-memory cache with TTL (replaces Redis cache).
func NewCacheStore() *CacheStore {
	cs := &CacheStore{
		items: make(map[string]*cacheItem),
	}
	go cs.cleanupLoop()
	return cs
}

// Set stores a value with expiration.
func (cs *CacheStore) Set(_ context.Context, key string, value interface{}, expiration time.Duration) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.items[key] = &cacheItem{
		value:     fmt.Sprintf("%v", value),
		expiresAt: time.Now().Add(expiration),
	}
	return nil
}

// Get retrieves a value. Returns ("", ErrCacheMiss) if not found or expired.
func (cs *CacheStore) Get(_ context.Context, key string) (string, error) {
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

func (cs *CacheStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
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
