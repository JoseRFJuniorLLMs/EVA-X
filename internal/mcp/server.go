// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// EmbeddingFunc generates embeddings for text
type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

// VectorSearchFunc searches vectors in NietzscheDB
type VectorSearchFunc func(ctx context.Context, collection string, vector []float32, limit int) ([]VectorResult, error)

// VectorResult represents a vector search result
type VectorResult struct {
	ID      int64
	Score   float32
	Content string
}

// Server implements Model Context Protocol server
type Server struct {
	db           *sql.DB
	router       *mux.Router
	embedFunc    EmbeddingFunc
	vectorSearch VectorSearchFunc
}

// NewServer creates a new MCP server
func NewServer(db *sql.DB) *Server {
	if db == nil {
		log.Warn().Msg("⚠️ [MCP] NietzscheDB unavailable — running in degraded mode")
	}

	s := &Server{
		db:     db,
		router: mux.NewRouter(),
	}

	s.setupRoutes()
	return s
}

// SetEmbeddingFunc sets the embedding function for vector search
func (s *Server) SetEmbeddingFunc(f EmbeddingFunc) {
	s.embedFunc = f
}

// SetVectorSearchFunc sets the vector search function
func (s *Server) SetVectorSearchFunc(f VectorSearchFunc) {
	s.vectorSearch = f
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
	if s.db == nil {
		http.Error(w, `{"error":"NietzscheDB unavailable"}`, http.StatusServiceUnavailable)
		return
	}
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
	if s.db == nil {
		http.Error(w, `{"error":"NietzscheDB unavailable"}`, http.StatusServiceUnavailable)
		return
	}
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
	if s.db == nil {
		http.Error(w, `{"error":"NietzscheDB unavailable"}`, http.StatusServiceUnavailable)
		return
	}
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

	// Vector search with ILIKE fallback
	if s.embedFunc != nil && s.vectorSearch != nil {
		memories, vectorErr := s.recallWithVectorSearch(ctx, req)
		if vectorErr == nil && len(memories) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": memories,
				"count":    len(memories),
				"method":   "vector",
			})
			return
		}
		if vectorErr != nil {
			log.Warn().Err(vectorErr).Msg("[MCP] Vector search failed, falling back to text search")
		}
	}

	// Fallback: text search
	if s.db == nil {
		http.Error(w, `{"error":"NietzscheDB unavailable"}`, http.StatusServiceUnavailable)
		return
	}
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

// getPrompt retrieves a specific prompt by name
func (s *Server) getPrompt(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	promptName := vars["name"]

	// Lookup from known prompts
	knownPrompts := map[string]Prompt{
		"therapeutic_conversation": {
			Name:        "therapeutic_conversation",
			Description: "System prompt for therapeutic conversations",
			Template:    "You are EVA, a compassionate AI therapist. Your role is to provide emotional support and guidance.",
			Arguments: []PromptArgument{
				{Name: "patient_name", Description: "Patient's name", Required: true},
				{Name: "persona_type", Description: "Enneagram persona type", Required: false},
			},
		},
		"crisis_intervention": {
			Name:        "crisis_intervention",
			Description: "System prompt for crisis situations",
			Template:    "CRISIS MODE: Provide immediate emotional support and safety assessment.",
			Arguments: []PromptArgument{
				{Name: "severity_level", Description: "Crisis severity (low/medium/high/critical)", Required: true},
			},
		},
	}

	prompt, ok := knownPrompts[promptName]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"error":"prompt not found: %s"}`, promptName), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prompt)
}

// recallWithVectorSearch performs semantic recall using vector embeddings
func (s *Server) recallWithVectorSearch(ctx context.Context, req RecallRequest) ([]map[string]interface{}, error) {
	embedding, err := s.embedFunc(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}

	results, err := s.vectorSearch(ctx, "memories", embedding, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	memories := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		memories = append(memories, map[string]interface{}{
			"id":         r.ID,
			"content":    r.Content,
			"score":      r.Score,
		})
	}

	return memories, nil
}

// authMiddleware validates MCP_API_KEY on all legacy MCP endpoints
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("MCP_API_KEY")
		if apiKey == "" {
			http.Error(w, `{"error":"MCP endpoint disabled: MCP_API_KEY not configured"}`, http.StatusForbidden)
			return
		}
		authHeader := r.Header.Get("X-MCP-Key")
		if authHeader == "" {
			authHeader = r.Header.Get("Authorization")
		}
		if authHeader != apiKey {
			log.Warn().Str("ip", r.RemoteAddr).Msg("[MCP-Legacy] Unauthorized access attempt")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.authMiddleware(s.router).ServeHTTP(w, r)
}
