// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// Server implements Model Context Protocol server
type Server struct {
	db     *sql.DB
	router *mux.Router
}

// NewServer creates a new MCP server
func NewServer(db *sql.DB) *Server {
	s := &Server{
		db:     db,
		router: mux.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures MCP endpoints
func (s *Server) setupRoutes() {
	// Resources endpoints
	s.router.HandleFunc("/mcp/resources", s.listResources).Methods("GET")
	s.router.HandleFunc("/mcp/resources/{id}", s.getResource).Methods("GET")

	// Tools endpoints
	s.router.HandleFunc("/mcp/tools/remember", s.rememberTool).Methods("POST")
	s.router.HandleFunc("/mcp/tools/recall", s.recallTool).Methods("POST")

	// Prompts endpoints
	s.router.HandleFunc("/mcp/prompts", s.listPrompts).Methods("GET")
	s.router.HandleFunc("/mcp/prompts/{name}", s.getPrompt).Methods("GET")
}

// Resource represents an MCP resource
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mimeType"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// listResources lists available memory resources
func (s *Server) listResources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get patient_id from query params
	patientID := r.URL.Query().Get("patient_id")
	if patientID == "" {
		http.Error(w, "patient_id required", http.StatusBadRequest)
		return
	}

	// Query memories
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, created_at
		FROM memories
		WHERE patient_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, patientID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to query memories")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	resources := []Resource{}
	for rows.Next() {
		var id int64
		var content string
		var createdAt time.Time

		if err := rows.Scan(&id, &content, &createdAt); err != nil {
			continue
		}

		resources = append(resources, Resource{
			URI:         fmt.Sprintf("memory://%s/%d", patientID, id),
			Name:        fmt.Sprintf("Memory %d", id),
			Description: content,
			MimeType:    "text/plain",
			Metadata: map[string]interface{}{
				"created_at": createdAt,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resources": resources,
	})
}

// getResource retrieves a specific memory resource
func (s *Server) getResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	resourceID := vars["id"]

	var content string
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT content, created_at
		FROM memories
		WHERE id = $1
	`, resourceID).Scan(&content, &createdAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to get memory")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": content,
		"metadata": map[string]interface{}{
			"created_at": createdAt,
		},
	})
}

// RememberRequest represents a remember tool request
type RememberRequest struct {
	PatientID int64  `json:"patient_id"`
	Content   string `json:"content"`
}

// rememberTool stores a new memory
func (s *Server) rememberTool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req RememberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var memoryID int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO memories (patient_id, content, event_time, ingestion_time, created_at)
		VALUES ($1, $2, NOW(), NOW(), NOW())
		RETURNING id
	`, req.PatientID, req.Content).Scan(&memoryID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to store memory")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"memory_id": memoryID,
		"status":    "stored",
	})
}

// RecallRequest represents a recall tool request
type RecallRequest struct {
	PatientID int64  `json:"patient_id"`
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
}

// recallTool retrieves memories
func (s *Server) recallTool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req RecallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	// Simple text search (TODO: use vector search)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, event_time, importance_score
		FROM memories
		WHERE patient_id = $1
		AND content ILIKE '%' || $2 || '%'
		ORDER BY importance_score DESC, event_time DESC
		LIMIT $3
	`, req.PatientID, req.Query, req.Limit)

	if err != nil {
		log.Error().Err(err).Msg("Failed to recall memories")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	memories := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var content string
		var eventTime time.Time
		var importance float64

		if err := rows.Scan(&id, &content, &eventTime, &importance); err != nil {
			continue
		}

		memories = append(memories, map[string]interface{}{
			"id":         id,
			"content":    content,
			"event_time": eventTime,
			"importance": importance,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"memories": memories,
		"count":    len(memories),
	})
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Template    string           `json:"template"`
	Arguments   []PromptArgument `json:"arguments"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// listPrompts lists available system prompts
func (s *Server) listPrompts(w http.ResponseWriter, r *http.Request) {
	prompts := []Prompt{
		{
			Name:        "therapeutic_conversation",
			Description: "System prompt for therapeutic conversations",
			Template:    "You are EVA, a compassionate AI therapist. Your role is to provide emotional support and guidance.",
			Arguments: []PromptArgument{
				{Name: "patient_name", Description: "Patient's name", Required: true},
				{Name: "persona_type", Description: "Enneagram persona type", Required: false},
			},
		},
		{
			Name:        "crisis_intervention",
			Description: "System prompt for crisis situations",
			Template:    "CRISIS MODE: Provide immediate emotional support and safety assessment.",
			Arguments: []PromptArgument{
				{Name: "severity_level", Description: "Crisis severity (low/medium/high/critical)", Required: true},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"prompts": prompts,
	})
}

// getPrompt retrieves a specific prompt
func (s *Server) getPrompt(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	promptName := vars["name"]

	// TODO: Load from database or config
	prompt := Prompt{
		Name:        promptName,
		Description: "System prompt",
		Template:    "Default template",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prompt)
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
