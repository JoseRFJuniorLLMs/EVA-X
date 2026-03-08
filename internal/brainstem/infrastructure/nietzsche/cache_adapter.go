// Copyright (C) 2025-2026 Jose R F Junior
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eva/internal/brainstem/logger"
)

// CacheAdapter provides a NietzscheDB-compatible caching interface backed by NietzscheDB.
// It wraps the native CacheSet/CacheGet/CacheDel gRPC methods with convenience
// helpers for JSON, string, and raw byte operations.
//
// This is the recommended replacement for any NietzscheDB client usage in EVA.
// The adapter is scoped to a single NietzscheDB collection (e.g. "eva_cache").
type CacheAdapter struct {
	client     *Client
	collection string // cache collection name (e.g. "eva_cache")
}

// NewCacheAdapter creates a new CacheAdapter backed by a NietzscheDB collection.
// The collection must already exist (created via EnsureCollections at startup).
// If client is nil the adapter methods will return errors (no silent fallback).
func NewCacheAdapter(client *Client, collection string) *CacheAdapter {
	if collection == "" {
		collection = "eva_cache"
	}
	return &CacheAdapter{
		client:     client,
		collection: collection,
	}
}

// ── Raw byte operations ─────────────────────────────────────────────────────

// Set stores raw bytes under key with a TTL.
// A zero TTL is treated as 24 hours (sensible default for embedding caches).
func (ca *CacheAdapter) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	log := logger.Nietzsche()

	if ca.client == nil {
		return fmt.Errorf("cache_adapter: client is nil")
	}

	ttlSecs := uint64(ttl.Seconds())
	if ttlSecs == 0 {
		ttlSecs = 86400 // 24h default
	}

	if err := ca.client.CacheSet(ctx, ca.collection, key, value, ttlSecs); err != nil {
		log.Debug().Err(err).Str("key", key).Str("col", ca.collection).Msg("[CacheAdapter] Set failed")
		return fmt.Errorf("cache_adapter set %s: %w", key, err)
	}

	return nil
}

// Get retrieves raw bytes for key. Returns (value, found, error).
// found is false when the key does not exist or has expired (not an error).
func (ca *CacheAdapter) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if ca.client == nil {
		return nil, false, fmt.Errorf("cache_adapter: client is nil")
	}

	val, found, err := ca.client.CacheGet(ctx, ca.collection, key)
	if err != nil {
		log := logger.Nietzsche()
		log.Debug().Err(err).Str("key", key).Str("col", ca.collection).Msg("[CacheAdapter] Get failed")
		return nil, false, fmt.Errorf("cache_adapter get %s: %w", key, err)
	}

	return val, found, nil
}

// Del removes a key from the cache.
func (ca *CacheAdapter) Del(ctx context.Context, key string) error {
	if ca.client == nil {
		return fmt.Errorf("cache_adapter: client is nil")
	}

	if err := ca.client.CacheDel(ctx, ca.collection, key); err != nil {
		log := logger.Nietzsche()
		log.Debug().Err(err).Str("key", key).Str("col", ca.collection).Msg("[CacheAdapter] Del failed")
		return fmt.Errorf("cache_adapter del %s: %w", key, err)
	}

	return nil
}

// ── JSON convenience ────────────────────────────────────────────────────────

// SetJSON marshals v to JSON and stores it with a TTL.
func (ca *CacheAdapter) SetJSON(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache_adapter setjson marshal: %w", err)
	}
	return ca.Set(ctx, key, data, ttl)
}

// GetJSON retrieves a key and unmarshals the value into out.
// Returns (found, error). If found is false, out is not modified.
func (ca *CacheAdapter) GetJSON(ctx context.Context, key string, out interface{}) (bool, error) {
	data, found, err := ca.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	if err := json.Unmarshal(data, out); err != nil {
		return false, fmt.Errorf("cache_adapter getjson unmarshal: %w", err)
	}
	return true, nil
}

// ── String convenience ──────────────────────────────────────────────────────

// SetString stores a string value with a TTL.
func (ca *CacheAdapter) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return ca.Set(ctx, key, []byte(value), ttl)
}

// GetString retrieves a string value. Returns (value, found, error).
func (ca *CacheAdapter) GetString(ctx context.Context, key string) (string, bool, error) {
	data, found, err := ca.Get(ctx, key)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return string(data), true, nil
}

// ── Introspection ───────────────────────────────────────────────────────────

// Collection returns the NietzscheDB collection name this adapter targets.
func (ca *CacheAdapter) Collection() string {
	return ca.collection
}
