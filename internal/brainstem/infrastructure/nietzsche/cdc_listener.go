// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"sync"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// CDCHandler is a callback invoked for each change event.
type CDCHandler func(event nietzsche.CDCEvent)

// CDCListener subscribes to NietzscheDB Change Data Capture streams.
// Used for cache invalidation, audit logging, and real-time dashboard updates.
type CDCListener struct {
	client   *Client
	mu       sync.RWMutex
	handlers map[string][]CDCHandler // collection → handlers
}

// NewCDCListener creates a new CDC listener.
func NewCDCListener(client *Client) *CDCListener {
	return &CDCListener{
		client:   client,
		handlers: make(map[string][]CDCHandler),
	}
}

// Subscribe registers a handler for change events on a collection.
// Must be called before Start().
func (cl *CDCListener) Subscribe(collection string, handler CDCHandler) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.handlers[collection] = append(cl.handlers[collection], handler)
}

// Start begins listening for CDC events on all subscribed collections.
// Blocks until ctx is cancelled. Call in a goroutine.
func (cl *CDCListener) Start(ctx context.Context) {
	log := logger.Nietzsche()

	cl.mu.RLock()
	collections := make([]string, 0, len(cl.handlers))
	for col := range cl.handlers {
		collections = append(collections, col)
	}
	cl.mu.RUnlock()

	if len(collections) == 0 {
		log.Warn().Msg("[CDC] No subscriptions registered, listener not starting")
		return
	}

	log.Info().Strs("collections", collections).Msg("[CDC] Starting CDC listener")

	var wg sync.WaitGroup
	for _, col := range collections {
		wg.Add(1)
		go func(collection string) {
			defer wg.Done()
			cl.listenCollection(ctx, collection)
		}(col)
	}

	wg.Wait()
	log.Info().Msg("[CDC] CDC listener stopped")
}

// listenCollection subscribes to a single collection's CDC stream.
func (cl *CDCListener) listenCollection(ctx context.Context, collection string) {
	log := logger.Nietzsche()

	sub, err := cl.client.SubscribeCDC(ctx, collection, 0)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("[CDC] Failed to subscribe")
		return
	}
	defer sub.Close()

	log.Info().Str("collection", collection).Msg("[CDC] Subscribed to CDC stream")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			event, err := sub.Recv()
			if err != nil {
				log.Warn().Err(err).Str("collection", collection).Msg("[CDC] Stream error, stopping")
				return
			}

			cl.mu.RLock()
			handlers := cl.handlers[collection]
			cl.mu.RUnlock()

			for _, h := range handlers {
				h(event)
			}
		}
	}
}
