package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"eva-mind/internal/cortex/ram"
)

// RAMHandler handler para RAM endpoints
type RAMHandler struct {
	ramEngine *ram.RAMEngine
}

// NewRAMHandler cria novo handler
func NewRAMHandler(ramEngine *ram.RAMEngine) *RAMHandler {
	return &RAMHandler{
		ramEngine: ramEngine,
	}
}

// RegisterRoutes registra rotas de RAM
func (h *RAMHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/ram/process/{patient_id}", h.ProcessQuery).Methods("POST")
	router.HandleFunc("/api/v1/ram/feedback/{patient_id}", h.SubmitFeedback).Methods("POST")
	router.HandleFunc("/api/v1/ram/interpretations/{patient_id}/{interpretation_id}", h.GetInterpretation).Methods("GET")
	router.HandleFunc("/api/v1/ram/feedback/stats/{patient_id}", h.GetFeedbackStats).Methods("GET")
	router.HandleFunc("/api/v1/ram/config", h.GetConfig).Methods("GET")
	router.HandleFunc("/api/v1/ram/config", h.UpdateConfig).Methods("PUT")
}

// ProcessQuery processa query e retorna interpretações alternativas
// POST /api/v1/ram/process/:patient_id
func (h *RAMHandler) ProcessQuery(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	// Parse body
	var req ProcessQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Processar com RAM
	response, err := h.ramEngine.Process(ctx, patientID, req.Query, req.Context)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process query: %v", err), http.StatusInternalServerError)
		return
	}

	// Converter para DTO
	dto := h.ramResponseToDTO(response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// SubmitFeedback submete feedback do cuidador
// POST /api/v1/ram/feedback/:patient_id
func (h *RAMHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	// Parse body
	var req SubmitFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.InterpretationID == "" {
		http.Error(w, "interpretation_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Submeter feedback
	err = h.ramEngine.SubmitFeedback(ctx, patientID, req.InterpretationID, req.Correct, req.CorrectedText)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit feedback: %v", err), http.StatusInternalServerError)
		return
	}

	// Response
	response := SubmitFeedbackResponse{
		PatientID:        patientID,
		InterpretationID: req.InterpretationID,
		Correct:          req.Correct,
		Applied:          true,
		Message:          h.generateFeedbackMessage(req.Correct),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetInterpretation recupera interpretação por ID
// GET /api/v1/ram/interpretations/:patient_id/:interpretation_id
func (h *RAMHandler) GetInterpretation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	interpretationID := vars["interpretation_id"]
	if interpretationID == "" {
		http.Error(w, "interpretation_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Recuperar interpretação
	interpretation, err := h.ramEngine.GetInterpretationByID(ctx, patientID, interpretationID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get interpretation: %v", err), http.StatusInternalServerError)
		return
	}

	if interpretation == nil {
		http.Error(w, "Interpretation not found", http.StatusNotFound)
		return
	}

	// Converter para DTO
	dto := h.interpretationToDTO(*interpretation)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// GetFeedbackStats retorna estatísticas de feedback
// GET /api/v1/ram/feedback/stats/:patient_id
func (h *RAMHandler) GetFeedbackStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	patientID, err := strconv.ParseInt(vars["patient_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid patient_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Obter estatísticas
	stats, err := h.ramEngine.GetFeedbackStats(ctx, patientID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get feedback stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Converter para DTO
	dto := h.feedbackStatsToDTO(stats)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

// GetConfig retorna configuração atual do RAM
// GET /api/v1/ram/config
func (h *RAMHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config := h.ramEngine.GetConfig()

	response := RAMConfigDTO{
		NumInterpretations:     config.NumInterpretations,
		MinConfidenceThreshold: config.MinConfidenceThreshold,
		HistoricalValidation:   config.HistoricalValidationEnabled,
		FeedbackLearningRate:   config.FeedbackLearningRate,
		MaxResponseTimeMs:      config.MaxResponseTimeMs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig atualiza configuração do RAM
// PUT /api/v1/ram/config
func (h *RAMHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validações
	if req.NumInterpretations != nil && (*req.NumInterpretations < 1 || *req.NumInterpretations > 5) {
		http.Error(w, "num_interpretations must be between 1 and 5", http.StatusBadRequest)
		return
	}

	if req.MinConfidenceThreshold != nil && (*req.MinConfidenceThreshold < 0.0 || *req.MinConfidenceThreshold > 1.0) {
		http.Error(w, "min_confidence_threshold must be between 0.0 and 1.0", http.StatusBadRequest)
		return
	}

	// Atualizar config
	config := h.ramEngine.GetConfig()

	if req.NumInterpretations != nil {
		config.NumInterpretations = *req.NumInterpretations
	}
	if req.MinConfidenceThreshold != nil {
		config.MinConfidenceThreshold = *req.MinConfidenceThreshold
	}
	if req.HistoricalValidation != nil {
		config.HistoricalValidationEnabled = *req.HistoricalValidation
	}
	if req.FeedbackLearningRate != nil {
		config.FeedbackLearningRate = *req.FeedbackLearningRate
	}

	h.ramEngine.SetConfig(config)

	// Response
	response := RAMConfigDTO{
		NumInterpretations:     config.NumInterpretations,
		MinConfidenceThreshold: config.MinConfidenceThreshold,
		HistoricalValidation:   config.HistoricalValidationEnabled,
		FeedbackLearningRate:   config.FeedbackLearningRate,
		MaxResponseTimeMs:      config.MaxResponseTimeMs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions

func (h *RAMHandler) ramResponseToDTO(response *ram.RAMResponse) RAMResponseDTO {
	dto := RAMResponseDTO{
		Query:            response.Query,
		Interpretations:  make([]InterpretationDTO, len(response.Interpretations)),
		Confidence:       response.Confidence,
		RequiresReview:   response.RequiresReview,
		ReviewReason:     response.ReviewReason,
		ProcessingTimeMs: response.ProcessingTimeMs,
		Metadata:         h.metadataToDTO(response.Metadata),
	}

	for i, interp := range response.Interpretations {
		dto.Interpretations[i] = h.interpretationToDTO(interp)
	}

	if response.BestInterpretation != nil {
		best := h.interpretationToDTO(*response.BestInterpretation)
		dto.BestInterpretation = &best
	}

	return dto
}

func (h *RAMHandler) interpretationToDTO(interp ram.Interpretation) InterpretationDTO {
	return InterpretationDTO{
		ID:                interp.ID,
		Content:           interp.Content,
		Confidence:        interp.Confidence,
		PlausibilityScore: interp.PlausibilityScore,
		HistoricalScore:   interp.HistoricalScore,
		CombinedScore:     interp.CombinedScore,
		SupportingFactsCount: len(interp.SupportingFacts),
		ContradictionsCount:  len(interp.Contradictions),
		ReasoningPath:        interp.ReasoningPath,
	}
}

func (h *RAMHandler) metadataToDTO(metadata ram.RAMMetadata) RAMMetadataDTO {
	return RAMMetadataDTO{
		TotalInterpretations:      metadata.TotalInterpretations,
		ValidatedAgainstHistory:   metadata.ValidatedAgainstHistory,
		HistoricalMemoriesChecked: metadata.HistoricalMemoriesChecked,
		ContradictionsFound:       metadata.ContradictionsFound,
		FeedbackAvailable:         metadata.FeedbackAvailable,
	}
}

func (h *RAMHandler) feedbackStatsToDTO(stats *ram.FeedbackStats) FeedbackStatsDTO {
	return FeedbackStatsDTO{
		PatientID:      stats.PatientID,
		TotalFeedbacks: stats.TotalFeedbacks,
		CorrectCount:   stats.CorrectCount,
		IncorrectCount: stats.IncorrectCount,
		AccuracyRate:   stats.AccuracyRate,
		LastFeedbackDate: stats.LastFeedbackDate.Format("2006-01-02 15:04:05"),
	}
}

func (h *RAMHandler) generateFeedbackMessage(correct bool) string {
	if correct {
		return "Positive feedback applied. Hebbian weights boosted for related associations."
	}
	return "Negative feedback applied. Hebbian weights decayed for incorrect associations."
}

// DTOs

type ProcessQueryRequest struct {
	Query   string `json:"query"`
	Context string `json:"context,omitempty"`
}

type RAMResponseDTO struct {
	Query              string              `json:"query"`
	Interpretations    []InterpretationDTO `json:"interpretations"`
	BestInterpretation *InterpretationDTO  `json:"best_interpretation"`
	Confidence         float64             `json:"confidence"`
	RequiresReview     bool                `json:"requires_review"`
	ReviewReason       string              `json:"review_reason,omitempty"`
	ProcessingTimeMs   int64               `json:"processing_time_ms"`
	Metadata           RAMMetadataDTO      `json:"metadata"`
}

type InterpretationDTO struct {
	ID                   string   `json:"id"`
	Content              string   `json:"content"`
	Confidence           float64  `json:"confidence"`
	PlausibilityScore    float64  `json:"plausibility_score"`
	HistoricalScore      float64  `json:"historical_score"`
	CombinedScore        float64  `json:"combined_score"`
	SupportingFactsCount int      `json:"supporting_facts_count"`
	ContradictionsCount  int      `json:"contradictions_count"`
	ReasoningPath        []string `json:"reasoning_path,omitempty"`
}

type RAMMetadataDTO struct {
	TotalInterpretations      int  `json:"total_interpretations"`
	ValidatedAgainstHistory   bool `json:"validated_against_history"`
	HistoricalMemoriesChecked int  `json:"historical_memories_checked"`
	ContradictionsFound       int  `json:"contradictions_found"`
	FeedbackAvailable         bool `json:"feedback_available"`
}

type SubmitFeedbackRequest struct {
	InterpretationID string `json:"interpretation_id"`
	Correct          bool   `json:"correct"`
	CorrectedText    string `json:"corrected_text,omitempty"`
}

type SubmitFeedbackResponse struct {
	PatientID        int64  `json:"patient_id"`
	InterpretationID string `json:"interpretation_id"`
	Correct          bool   `json:"correct"`
	Applied          bool   `json:"applied"`
	Message          string `json:"message"`
}

type FeedbackStatsDTO struct {
	PatientID        int64   `json:"patient_id"`
	TotalFeedbacks   int     `json:"total_feedbacks"`
	CorrectCount     int     `json:"correct_count"`
	IncorrectCount   int     `json:"incorrect_count"`
	AccuracyRate     float64 `json:"accuracy_rate"`
	LastFeedbackDate string  `json:"last_feedback_date"`
}

type RAMConfigDTO struct {
	NumInterpretations     int     `json:"num_interpretations"`
	MinConfidenceThreshold float64 `json:"min_confidence_threshold"`
	HistoricalValidation   bool    `json:"historical_validation"`
	FeedbackLearningRate   float64 `json:"feedback_learning_rate"`
	MaxResponseTimeMs      int     `json:"max_response_time_ms"`
}

type UpdateConfigRequest struct {
	NumInterpretations     *int     `json:"num_interpretations,omitempty"`
	MinConfidenceThreshold *float64 `json:"min_confidence_threshold,omitempty"`
	HistoricalValidation   *bool    `json:"historical_validation,omitempty"`
	FeedbackLearningRate   *float64 `json:"feedback_learning_rate,omitempty"`
}
