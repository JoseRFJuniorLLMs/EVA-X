// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"net/http"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	gws "github.com/gorilla/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var perspektiveUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// Allow localhost dev and known production origins.
		for _, allowed := range []string{
			"http://localhost",
			"https://localhost",
			"http://127.0.0.1",
			"https://127.0.0.1",
			"https://eva.health",
		} {
			if strings.HasPrefix(origin, allowed) {
				return true
			}
		}
		log.Warn().Str("origin", origin).Msg("[Perspektive] Rejected WS origin")
		return false
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// handlePerspektiveWS upgrades an HTTP request to a WebSocket connection and
// registers it with the CDC WebSocket bridge for real-time Perspektive events.
//
// Query parameters:
//   - token: authentication token (required in production)
//
// The handler reads pings/pongs from the client to detect disconnection and
// performs cleanup via defer when the connection closes.
func handlePerspektiveWS(w http.ResponseWriter, r *http.Request, bridge *nietzscheInfra.WebSocketBridge) {
	// Token validation (query string for WS — Authorization header not available during upgrade).
	token := r.URL.Query().Get("token")
	if token == "" {
		log.Warn().Str("remote", r.RemoteAddr).Msg("[Perspektive] Missing auth token")
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	// TODO: validate token against auth service when ready.
	// For now, accept any non-empty token during development.

	conn, err := perspektiveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("[Perspektive] WS upgrade failed")
		return
	}

	connID := uuid.New().String()

	// Register with bridge; reject if at capacity.
	if !bridge.AddClient(connID, conn) {
		log.Warn().Str("remote", r.RemoteAddr).Msg("[Perspektive] Max clients reached, rejecting")
		conn.WriteMessage(gws.CloseMessage,
			gws.FormatCloseMessage(gws.CloseTryAgainLater, "max clients reached"))
		conn.Close()
		return
	}

	// Ensure cleanup on any exit path.
	defer func() {
		bridge.RemoveClient(connID)
		conn.Close()
	}()

	log.Info().
		Str("conn_id", connID).
		Str("remote", r.RemoteAddr).
		Int("clients", bridge.ClientCount()).
		Msg("[Perspektive] Client connected")

	// Keepalive read loop — we only expect pings/pongs from the client.
	// Any received text message is silently ignored (read-only stream).
	conn.SetReadLimit(512) // pings are tiny
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping ticker — keep the connection alive through proxies/firewalls.
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Read loop runs in the foreground; ping loop runs in background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				// Normal closure or network error — exit gracefully.
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
