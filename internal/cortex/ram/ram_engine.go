package ram

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// RAMEngine - Realistic Accuracy Model
// Gera múltiplas interpretações, valida contra histórico, aprende com feedback
type RAMEngine struct {
	interpretationGen *InterpretationGenerator
	historicalVal     *HistoricalValidator
	feedbackLoop      *FeedbackLoop
	config            *RAMConfig
}

// RAMConfig configuração do RAM
type RAMConfig struct {
	NumInterpretations       int     // 3 default
	MinConfidenceThreshold   float64 // 0.6 default
	HistoricalValidationEnabled bool
	FeedbackLearningRate     float64 // 0.05 default
	MaxResponseTimeMs        int     // 2000ms default
}

// Interpretation representa uma interpretação alternativa
type Interpretation struct {
	ID              string
	Content         string
	Confidence      float64           // 0-1
	PlausibilityScore float64         // 0-1
	HistoricalScore float64           // 0-1 (consistência com histórico)
	CombinedScore   float64           // média ponderada
	SupportingFacts []SupportingFact
	Contradictions  []Contradiction
	ReasoningPath   []string
	GeneratedAt     time.Time
}

// SupportingFact fato que suporta a interpretação
type SupportingFact struct {
	MemoryID   int64
	Content    string
	Similarity float64
	Timestamp  time.Time
}

// Contradiction contradição detectada
type Contradiction struct {
	MemoryID    int64
	Content     string
	Reason      string
	Severity    string // high, medium, low
}

// RAMResponse resposta completa do RAM
type RAMResponse struct {
	Query            string
	Interpretations  []Interpretation
	BestInterpretation *Interpretation
	Confidence       float64
	RequiresReview   bool
	ReviewReason     string
	ProcessingTimeMs int64
	Metadata         RAMMetadata
}

// RAMMetadata metadados do processamento
type RAMMetadata struct {
	TotalInterpretations int
	ValidatedAgainstHistory bool
	HistoricalMemoriesChecked int
	ContradictionsFound    int
	FeedbackAvailable      bool
}

// NewRAMEngine cria novo RAM engine
func NewRAMEngine(
	generator *InterpretationGenerator,
	validator *HistoricalValidator,
	feedback *FeedbackLoop,
	config *RAMConfig,
) *RAMEngine {
	if config == nil {
		config = DefaultRAMConfig()
	}

	return &RAMEngine{
		interpretationGen: generator,
		historicalVal:     validator,
		feedbackLoop:      feedback,
		config:            config,
	}
}

// DefaultRAMConfig retorna configuração padrão
func DefaultRAMConfig() *RAMConfig {
	return &RAMConfig{
		NumInterpretations:       3,
		MinConfidenceThreshold:   0.6,
		HistoricalValidationEnabled: true,
		FeedbackLearningRate:     0.05,
		MaxResponseTimeMs:        2000,
	}
}

// Process processa query e retorna interpretações alternativas
func (r *RAMEngine) Process(ctx context.Context, patientID int64, query string, context string) (*RAMResponse, error) {
	startTime := time.Now()

	response := &RAMResponse{
		Query: query,
		Metadata: RAMMetadata{},
	}

	// E1: Gerar interpretações alternativas
	interpretations, err := r.interpretationGen.Generate(ctx, patientID, query, context, r.config.NumInterpretations)
	if err != nil {
		return nil, fmt.Errorf("failed to generate interpretations: %w", err)
	}

	response.Interpretations = interpretations
	response.Metadata.TotalInterpretations = len(interpretations)

	// E2: Validar contra histórico
	if r.config.HistoricalValidationEnabled && r.historicalVal != nil {
		for i := range interpretations {
			validationResult, err := r.historicalVal.Validate(ctx, patientID, &interpretations[i])
			if err != nil {
				// Log error mas continua
				fmt.Printf("Failed to validate interpretation %s: %v\n", interpretations[i].ID, err)
				continue
			}

			interpretations[i].HistoricalScore = validationResult.ConsistencyScore
			interpretations[i].SupportingFacts = validationResult.SupportingFacts
			interpretations[i].Contradictions = validationResult.Contradictions

			response.Metadata.HistoricalMemoriesChecked += validationResult.MemoriesChecked
			response.Metadata.ContradictionsFound += len(validationResult.Contradictions)
		}

		response.Metadata.ValidatedAgainstHistory = true
	}

	// Calcular combined score (plausibility + historical)
	for i := range interpretations {
		interpretations[i].CombinedScore = r.calculateCombinedScore(&interpretations[i])
	}

	// Ordenar por combined score
	sort.Slice(interpretations, func(i, j int) bool {
		return interpretations[i].CombinedScore > interpretations[j].CombinedScore
	})

	// Selecionar melhor interpretação
	if len(interpretations) > 0 {
		response.BestInterpretation = &interpretations[0]
		response.Confidence = interpretations[0].CombinedScore
	}

	// Determinar se requer revisão
	response.RequiresReview, response.ReviewReason = r.shouldRequireReview(interpretations)

	// Verificar se há feedback disponível para aprendizado
	response.Metadata.FeedbackAvailable = r.feedbackLoop != nil

	response.ProcessingTimeMs = time.Since(startTime).Milliseconds()

	return response, nil
}

// SubmitFeedback submete feedback do cuidador (E3)
func (r *RAMEngine) SubmitFeedback(ctx context.Context, patientID int64, interpretationID string, correct bool, correctedText string) error {
	if r.feedbackLoop == nil {
		return fmt.Errorf("feedback loop not initialized")
	}

	feedback := &Feedback{
		PatientID:       patientID,
		InterpretationID: interpretationID,
		Correct:         correct,
		CorrectedText:   correctedText,
		Timestamp:       time.Now(),
	}

	// Aplicar feedback → Hebbian boost/decay
	if err := r.feedbackLoop.Apply(ctx, feedback); err != nil {
		return fmt.Errorf("failed to apply feedback: %w", err)
	}

	return nil
}

// GetInterpretationByID recupera interpretação por ID
func (r *RAMEngine) GetInterpretationByID(ctx context.Context, patientID int64, interpretationID string) (*Interpretation, error) {
	// TODO: Implementar cache/storage de interpretações
	return nil, fmt.Errorf("not implemented")
}

// GetFeedbackStats retorna estatísticas de feedback
func (r *RAMEngine) GetFeedbackStats(ctx context.Context, patientID int64) (*FeedbackStats, error) {
	if r.feedbackLoop == nil {
		return nil, fmt.Errorf("feedback loop not initialized")
	}

	return r.feedbackLoop.GetStats(ctx, patientID)
}

// calculateCombinedScore calcula score combinado
func (r *RAMEngine) calculateCombinedScore(interp *Interpretation) float64 {
	// Pesos: 40% plausibility, 40% historical, 20% confidence
	plausibilityWeight := 0.4
	historicalWeight := 0.4
	confidenceWeight := 0.2

	score := (interp.PlausibilityScore * plausibilityWeight) +
		(interp.HistoricalScore * historicalWeight) +
		(interp.Confidence * confidenceWeight)

	// Penalizar se tem contradições
	if len(interp.Contradictions) > 0 {
		penalty := 0.1 * float64(len(interp.Contradictions))
		score = score * (1.0 - penalty)
	}

	// Clamp [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// shouldRequireReview determina se requer revisão do cuidador
func (r *RAMEngine) shouldRequireReview(interpretations []Interpretation) (bool, string) {
	if len(interpretations) == 0 {
		return true, "no_interpretations_generated"
	}

	best := interpretations[0]

	// Caso 1: Confidence muito baixa
	if best.CombinedScore < r.config.MinConfidenceThreshold {
		return true, fmt.Sprintf("low_confidence (%.2f < %.2f)", best.CombinedScore, r.config.MinConfidenceThreshold)
	}

	// Caso 2: Contradições de alta severidade
	for _, contradiction := range best.Contradictions {
		if contradiction.Severity == "high" {
			return true, "high_severity_contradiction_detected"
		}
	}

	// Caso 3: Múltiplas interpretações com scores similares (ambiguidade)
	if len(interpretations) >= 2 {
		secondBest := interpretations[1]
		scoreDiff := best.CombinedScore - secondBest.CombinedScore

		if scoreDiff < 0.1 { // Scores muito próximos
			return true, fmt.Sprintf("ambiguous (score diff: %.2f)", scoreDiff)
		}
	}

	return false, ""
}

// ExplainInterpretation explica o raciocínio de uma interpretação
func (r *RAMEngine) ExplainInterpretation(interp *Interpretation) string {
	explanation := fmt.Sprintf("Interpretação: %s\n", interp.Content)
	explanation += fmt.Sprintf("Confiança: %.2f%%\n\n", interp.CombinedScore*100)

	if len(interp.SupportingFacts) > 0 {
		explanation += "Evidências que suportam:\n"
		for i, fact := range interp.SupportingFacts {
			if i >= 3 {
				break // Top 3
			}
			explanation += fmt.Sprintf("- %s (similaridade: %.2f)\n", fact.Content, fact.Similarity)
		}
		explanation += "\n"
	}

	if len(interp.Contradictions) > 0 {
		explanation += "⚠️ Contradições detectadas:\n"
		for _, contradiction := range interp.Contradictions {
			explanation += fmt.Sprintf("- %s (%s)\n", contradiction.Reason, contradiction.Severity)
		}
		explanation += "\n"
	}

	if len(interp.ReasoningPath) > 0 {
		explanation += "Caminho de raciocínio:\n"
		for i, step := range interp.ReasoningPath {
			explanation += fmt.Sprintf("%d. %s\n", i+1, step)
		}
	}

	return explanation
}

// GetConfig retorna configuração atual
func (r *RAMEngine) GetConfig() *RAMConfig {
	return r.config
}

// SetConfig atualiza configuração
func (r *RAMEngine) SetConfig(config *RAMConfig) {
	if config != nil {
		r.config = config
	}
}
