package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"eva-mind/internal/hippocampus/memory"
)

// EntityHandler handler para entity resolution endpoints
type EntityHandler struct {
	resolver *memory.EntityResolver
}

// NewEntityHandler cria novo handler
func NewEntityHandler(resolver *memory.EntityResolver) *EntityHandler {
	return &EntityHandler{
		resolver: resolver,
	}
}

// RegisterRoutes registra rotas de entity resolution
func (h *EntityHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/entities/duplicates/{patient_id}", h.GetDuplicateCandidates).Methods("GET")
	router.HandleFunc("/api/v1/entities/merge/{patient_id}", h.MergeEntities).Methods("POST")
	router.HandleFunc("/api/v1/entities/auto-resolve/{patient_id}", h.AutoResolve).Methods("POST")
	router.HandleFunc("/api/v1/entities/resolve-name/{patient_id}", h.ResolveName).Methods("POST")
	router.HandleFunc("/api/v1/entities/threshold", h.GetThreshold).Methods("GET")
	router.HandleFunc("/api/v1/entities/threshold", h.SetThreshold).Methods("PUT")
}

// GetDuplicateCandidates retorna candidatos para merge
// GET /api/v1/entities/duplicates/:patient_id
func (h *EntityHandler) GetDuplicateCandidates(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Encontrar candidatos
	candidates, err := h.resolver.FindDuplicateEntities(ctx, patientID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find duplicates: %v", err), http.StatusInternalServerError)
		return
	}

	// Response
	response := DuplicateCandidatesResponse{
		PatientID:       patientID,
		TotalCandidates: len(candidates),
		Candidates:      make([]CandidateDTO, len(candidates)),
	}

	for i, candidate := range candidates {
		response.Candidates[i] = CandidateDTO{
			SourceID:   candidate.SourceID,
			TargetID:   candidate.TargetID,
			SourceName: candidate.SourceName,
			TargetName: candidate.TargetName,
			Similarity: candidate.Similarity,
			Confidence: candidate.Confidence,
			ReasonCode: candidate.ReasonCode,
			Action:     getActionForConfidence(candidate.Confidence),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// MergeEntities executa merge de entidades
// POST /api/v1/entities/merge/:patient_id
// Body: { "candidates": [{"source_id": "...", "target_id": "..."}] }
func (h *EntityHandler) MergeEntities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	// Parse body
	var req MergeEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Candidates) == 0 {
		http.Error(w, "No candidates provided", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Converter DTOs para MergeCandidates
	candidates := make([]memory.MergeCandidate, len(req.Candidates))
	for i, dto := range req.Candidates {
		candidates[i] = memory.MergeCandidate{
			SourceID:   dto.SourceID,
			TargetID:   dto.TargetID,
			SourceName: dto.SourceName,
			TargetName: dto.TargetName,
			Similarity: dto.Similarity,
		}
	}

	// Executar merges
	results, err := h.resolver.MergeEntities(ctx, patientID, candidates)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to merge entities: %v", err), http.StatusInternalServerError)
		return
	}

	// Response
	response := MergeEntitiesResponse{
		PatientID:    patientID,
		TotalMerges:  len(results),
		Successful:   0,
		Failed:       0,
		Results:      make([]MergeResultDTO, len(results)),
	}

	for i, result := range results {
		if result.Success {
			response.Successful++
		} else {
			response.Failed++
		}

		response.Results[i] = MergeResultDTO{
			SourceID:         result.SourceID,
			TargetID:         result.TargetID,
			Success:          result.Success,
			EdgesMoved:       result.EdgesMoved,
			PropertiesMerged: result.PropertiesMerged,
			Error:            formatError(result.Error),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AutoResolve executa auto-resolução
// POST /api/v1/entities/auto-resolve/:patient_id?dry_run=true
func (h *EntityHandler) AutoResolve(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	// Parse query params
	dryRun := r.URL.Query().Get("dry_run") == "true"

	ctx := r.Context()

	// Executar auto-resolve
	stats, err := h.resolver.AutoResolve(ctx, patientID, dryRun)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to auto-resolve: %v", err), http.StatusInternalServerError)
		return
	}

	// Response
	response := AutoResolveResponse{
		PatientID:         patientID,
		DryRun:            dryRun,
		CandidatesFound:   stats.CandidatesFound,
		MergesPerformed:   stats.MergesPerformed,
		EdgesConsolidated: stats.EdgesConsolidated,
		DurationMs:        stats.Duration.Milliseconds(),
		Message:           generateAutoResolveMessage(dryRun, stats),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ResolveName resolve um nome em tempo real
// POST /api/v1/entities/resolve-name/:patient_id
// Body: { "entity_name": "minha mãe Maria" }
func (h *EntityHandler) ResolveName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	// Parse body
	var req ResolveNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.EntityName == "" {
		http.Error(w, "entity_name is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Resolver nome
	canonicalName, matched, err := h.resolver.ResolveEntityName(ctx, patientID, req.EntityName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to resolve name: %v", err), http.StatusInternalServerError)
		return
	}

	// Response
	response := ResolveNameResponse{
		PatientID:     patientID,
		InputName:     req.EntityName,
		CanonicalName: canonicalName,
		Matched:       matched,
		Message:       generateResolveMessage(req.EntityName, canonicalName, matched),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetThreshold retorna threshold atual
// GET /api/v1/entities/threshold
func (h *EntityHandler) GetThreshold(w http.ResponseWriter, r *http.Request) {
	threshold := h.resolver.GetSimilarityThreshold()

	response := ThresholdResponse{
		Threshold: threshold,
		Message:   fmt.Sprintf("Current similarity threshold: %.2f", threshold),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetThreshold configura novo threshold
// PUT /api/v1/entities/threshold
// Body: { "threshold": 0.90 }
func (h *EntityHandler) SetThreshold(w http.ResponseWriter, r *http.Request) {
	var req SetThresholdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Threshold < 0.0 || req.Threshold > 1.0 {
		http.Error(w, "Threshold must be between 0.0 and 1.0", http.StatusBadRequest)
		return
	}

	h.resolver.SetSimilarityThreshold(req.Threshold)

	response := ThresholdResponse{
		Threshold: req.Threshold,
		Message:   fmt.Sprintf("Threshold updated to %.2f", req.Threshold),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DTOs

type DuplicateCandidatesResponse struct {
	PatientID       int64          `json:"patient_id"`
	TotalCandidates int            `json:"total_candidates"`
	Candidates      []CandidateDTO `json:"candidates"`
}

type CandidateDTO struct {
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	SourceName string  `json:"source_name"`
	TargetName string  `json:"target_name"`
	Similarity float64 `json:"similarity"`
	Confidence string  `json:"confidence"` // high, medium, low
	ReasonCode string  `json:"reason_code"`
	Action     string  `json:"action"` // auto_merge, review_required
}

type MergeEntitiesRequest struct {
	Candidates []MergeCandidateDTO `json:"candidates"`
}

type MergeCandidateDTO struct {
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	SourceName string  `json:"source_name,omitempty"`
	TargetName string  `json:"target_name,omitempty"`
	Similarity float64 `json:"similarity,omitempty"`
}

type MergeEntitiesResponse struct {
	PatientID   int64            `json:"patient_id"`
	TotalMerges int              `json:"total_merges"`
	Successful  int              `json:"successful"`
	Failed      int              `json:"failed"`
	Results     []MergeResultDTO `json:"results"`
}

type MergeResultDTO struct {
	SourceID         string `json:"source_id"`
	TargetID         string `json:"target_id"`
	Success          bool   `json:"success"`
	EdgesMoved       int    `json:"edges_moved"`
	PropertiesMerged int    `json:"properties_merged"`
	Error            string `json:"error,omitempty"`
}

type AutoResolveResponse struct {
	PatientID         int64  `json:"patient_id"`
	DryRun            bool   `json:"dry_run"`
	CandidatesFound   int    `json:"candidates_found"`
	MergesPerformed   int    `json:"merges_performed"`
	EdgesConsolidated int    `json:"edges_consolidated"`
	DurationMs        int64  `json:"duration_ms"`
	Message           string `json:"message"`
}

type ResolveNameRequest struct {
	EntityName string `json:"entity_name"`
}

type ResolveNameResponse struct {
	PatientID     int64  `json:"patient_id"`
	InputName     string `json:"input_name"`
	CanonicalName string `json:"canonical_name"`
	Matched       bool   `json:"matched"`
	Message       string `json:"message"`
}

type ThresholdResponse struct {
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

type SetThresholdRequest struct {
	Threshold float64 `json:"threshold"`
}

// Helper functions

func getActionForConfidence(confidence string) string {
	if confidence == "high" {
		return "auto_merge"
	}
	return "review_required"
}

func formatError(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func generateAutoResolveMessage(dryRun bool, stats *memory.ResolutionStats) string {
	if dryRun {
		return fmt.Sprintf("Dry-run completed: found %d duplicate candidates (no merges performed)", stats.CandidatesFound)
	}
	return fmt.Sprintf("Auto-resolve completed: %d merges performed, %d edges consolidated", stats.MergesPerformed, stats.EdgesConsolidated)
}

func generateResolveMessage(input, canonical string, matched bool) string {
	if matched {
		return fmt.Sprintf("'%s' resolved to canonical name '%s'", input, canonical)
	}
	return fmt.Sprintf("'%s' is a new entity (no match found)", input)
}
