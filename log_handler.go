// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var logWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type logMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func (s *SignalingServer) handleLogStream(w http.ResponseWriter, r *http.Request) {
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
	conn.WriteJSON(logMessage{Type: "status", Data: "connected"})

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
				if err := conn.WriteMessage(gws.PingMessage, nil); err != nil {
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
			if err := conn.WriteJSON(logMessage{Type: "log", Data: line}); err != nil {
				cmd.Process.Kill()
				return
			}
		}
	}

	cmd.Wait()
	log.Info().Msg("Log stream session ended")
}
