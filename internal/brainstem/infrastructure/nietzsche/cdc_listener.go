// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"eva/internal/brainstem/logger"

	"github.com/gorilla/websocket"

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

// ── WebSocket Bridge ────────────────────────────────────────────────────────
// Bridges NietzscheDB CDC events to WebSocket clients for real-time streaming
// (e.g., the Perspektive 3D visualization dashboard).

const (
	// wsWriteTimeout is the maximum time to wait for a single WS write.
	// Slow clients that cannot accept within this window are disconnected.
	wsWriteTimeout = 5 * time.Second

	// maxBridgeClients caps the number of simultaneous Perspektive viewers.
	maxBridgeClients = 50
)

// PerspektiveEvent is the JSON envelope sent to Perspektive WS clients.
// embedding_preview carries the first 3 Poincare coordinates (3D position).
type PerspektiveEvent struct {
	Op               string    `json:"op"`
	Collection       string    `json:"collection"`
	ID               string    `json:"id"`
	NodeType         string    `json:"node_type,omitempty"`
	Energy           float32   `json:"energy,omitempty"`
	Depth            float32   `json:"depth,omitempty"`
	EmbeddingPreview []float64 `json:"embedding_preview,omitempty"`
	Timestamp        int64     `json:"ts"`
	LSN              uint64    `json:"lsn"`
}

// WebSocketBridge fans-out enriched CDC events to connected Perspektive clients.
type WebSocketBridge struct {
	mu       sync.RWMutex
	clients  map[string]*websocket.Conn
	listener *CDCListener
	client   *Client // NietzscheDB client for node enrichment lookups
}

// NewWebSocketBridge creates a bridge wired to the given CDC listener.
func NewWebSocketBridge(listener *CDCListener, nzClient *Client) *WebSocketBridge {
	return &WebSocketBridge{
		clients:  make(map[string]*websocket.Conn),
		listener: listener,
		client:   nzClient,
	}
}

// AddClient registers a new WebSocket connection.
// Returns false if the max client cap has been reached.
func (b *WebSocketBridge) AddClient(connID string, conn *websocket.Conn) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.clients) >= maxBridgeClients {
		return false
	}
	b.clients[connID] = conn

	log := logger.Nietzsche()
	log.Info().Str("conn_id", connID).Int("total", len(b.clients)).Msg("[WSBridge] Client connected")
	return true
}

// RemoveClient unregisters a WebSocket connection (idempotent).
func (b *WebSocketBridge) RemoveClient(connID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, connID)

	log := logger.Nietzsche()
	log.Info().Str("conn_id", connID).Int("total", len(b.clients)).Msg("[WSBridge] Client disconnected")
}

// ClientCount returns the current number of connected clients.
func (b *WebSocketBridge) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// Start subscribes to the given collections via the CDCListener and broadcasts
// enriched events to all connected WebSocket clients.
// Blocks until ctx is cancelled. Call in a goroutine.
func (b *WebSocketBridge) Start(ctx context.Context, collections []string) {
	log := logger.Nietzsche()

	for _, col := range collections {
		collection := col // capture for closure
		b.listener.Subscribe(collection, func(event nietzsche.CDCEvent) {
			b.handleCDCEvent(ctx, event)
		})
	}

	log.Info().Strs("collections", collections).Msg("[WSBridge] Subscriptions registered, starting CDC listener")
	b.listener.Start(ctx)
}

// handleCDCEvent enriches a raw CDC event with node metadata and broadcasts it.
func (b *WebSocketBridge) handleCDCEvent(ctx context.Context, event nietzsche.CDCEvent) {
	log := logger.Nietzsche()

	pe := PerspektiveEvent{
		Op:         event.EventType,
		Collection: event.Collection,
		ID:         event.EntityID,
		Timestamp:  event.Timestamp.UnixMilli(),
		LSN:        event.LSN,
	}

	// Enrich with node data for insert/update operations (not deletes or edges).
	if isNodeMutation(event.EventType) && event.EntityID != "" {
		enrichCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		node, err := b.client.GetNode(enrichCtx, event.EntityID, event.Collection)
		cancel()
		if err == nil && node.Found {
			pe.NodeType = node.NodeType
			pe.Energy = node.Energy
			pe.Depth = node.Depth
			// First 3 Poincare coordinates as 3D preview
			if len(node.Embedding) >= 3 {
				pe.EmbeddingPreview = node.Embedding[:3]
			} else if len(node.Embedding) > 0 {
				pe.EmbeddingPreview = node.Embedding
			}
		} else if err != nil {
			log.Debug().Err(err).Str("id", event.EntityID).Msg("[WSBridge] Node enrichment failed (sending basic event)")
		}
	}

	b.broadcastJSON(pe)
}

// broadcastJSON serializes the event and sends it to all connected clients.
// Slow clients that exceed wsWriteTimeout are disconnected.
func (b *WebSocketBridge) broadcastJSON(event PerspektiveEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log := logger.Nietzsche()
		log.Error().Err(err).Msg("[WSBridge] Failed to marshal event")
		return
	}

	b.mu.RLock()
	snapshot := make(map[string]*websocket.Conn, len(b.clients))
	for id, conn := range b.clients {
		snapshot[id] = conn
	}
	b.mu.RUnlock()

	var stale []string
	for id, conn := range snapshot {
		conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			stale = append(stale, id)
		}
	}

	// Remove clients that failed to receive within the timeout.
	if len(stale) > 0 {
		log := logger.Nietzsche()
		for _, id := range stale {
			b.RemoveClient(id)
			if conn, ok := snapshot[id]; ok {
				conn.Close()
			}
			log.Warn().Str("conn_id", id).Msg("[WSBridge] Evicted slow client")
		}
	}
}

// isNodeMutation returns true for CDC event types that relate to node mutations.
func isNodeMutation(eventType string) bool {
	switch eventType {
	case "INSERT_NODE", "UPDATE_NODE", "BATCH_INSERT_NODES":
		return true
	default:
		return false
	}
}
