// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

// ToolExecuteRequest represents a remote tool execution request (MCP bridge)
type ToolExecuteRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	IdosoID  int64                  `json:"idoso_id"`
}

// POST /api/v1/tools/execute
// Secured by MCP_API_KEY — only for MCP stdio server bridge
func (s *SignalingServer) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	// 1. Auth: require MCP_API_KEY (env var on server). No fallback — must be set.
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		log.Error().Msg("🚫 [MCP] MCP_API_KEY not set on server — endpoint disabled")
		http.Error(w, `{"error":"endpoint disabled: MCP_API_KEY not configured"}`, http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("X-MCP-Key")
	if authHeader != apiKey {
		log.Warn().Str("ip", r.RemoteAddr).Msg("🚫 [MCP] Unauthorized tool execution attempt")
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// 2. Parse request
	var req ToolExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ToolName == "" {
		http.Error(w, `{"error":"tool_name is required"}`, http.StatusBadRequest)
		return
	}

	// 3. Use creator's idoso_id if not provided
	if req.IdosoID == 0 {
		// Default to creator's ID (will be resolved from CPF in production)
		req.IdosoID = 1
	}

	// 4. Execute tool
	if s.toolsHandler == nil {
		http.Error(w, `{"error":"tools handler not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	log.Info().
		Str("tool", req.ToolName).
		Int64("idoso_id", req.IdosoID).
		Str("ip", r.RemoteAddr).
		Msg("🔧 [MCP] Executing tool")

	result, err := s.toolsHandler.ExecuteTool(req.ToolName, req.Args, req.IdosoID)
	if err != nil {
		log.Error().Err(err).Str("tool", req.ToolName).Msg("❌ [MCP] Tool execution failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// 5. Return result
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
