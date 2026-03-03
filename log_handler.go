// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var logWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Permitir conexões sem Origin (ex: apps mobile, curl)
		}
		allowedOrigins := []string{
			"https://eva-ia.org",
			"https://www.eva-ia.org",
			"https://app.eva-ia.org",
			"http://localhost:3000",
			"http://localhost:8080",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

type logMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func (s *SignalingServer) handleLogStream(w http.ResponseWriter, r *http.Request) {
	// Auth: require EVA_LOGS_TOKEN via query param ?token=...
	expectedToken := os.Getenv("EVA_LOGS_TOKEN")
	if expectedToken == "" {
		log.Error().Msg("[LogStream] EVA_LOGS_TOKEN not set — endpoint disabled")
		http.Error(w, "log stream disabled: token not configured", http.StatusForbidden)
		return
	}
	if r.URL.Query().Get("token") != expectedToken {
		log.Warn().Str("ip", r.RemoteAddr).Msg("[LogStream] Unauthorized access attempt")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := logWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Log WS upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Number of historical lines
	lines := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 5000 {
			lines = n
		}
	}

	// Start journalctl subprocess
	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", "eva-x",
		"-f",
		"-o", "short-iso",
		"-n", strconv.Itoa(lines),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create journalctl stdout pipe")
		conn.WriteJSON(logMessage{Type: "error", Data: "failed to start log stream"})
		return
	}

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start journalctl")
		conn.WriteJSON(logMessage{Type: "error", Data: "journalctl not available"})
		return
	}

	log.Info().Msg("Log stream session started")

	var wsMu sync.Mutex // protege escritas concorrentes no WebSocket

	wsMu.Lock()
	conn.WriteJSON(logMessage{Type: "status", Data: "connected"})
	wsMu.Unlock()

	// Goroutine: read from client (detect disconnect)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Ping keepalive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-pingTicker.C:
				wsMu.Lock()
				err := conn.WriteMessage(gws.PingMessage, nil)
				wsMu.Unlock()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Stream journal lines to WebSocket
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return
		default:
			line := scanner.Text()
			wsMu.Lock()
			err := conn.WriteJSON(logMessage{Type: "log", Data: line})
			wsMu.Unlock()
			if err != nil {
				cmd.Process.Kill()
				return
			}
		}
	}

	cmd.Wait()
	log.Info().Msg("Log stream session ended")
}
