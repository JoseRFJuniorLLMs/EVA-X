package ram

import (
	"context"
	"fmt"
	"time"
)

// FeedbackLoop (E3) - Aprende com feedback do cuidador
// Aplica Hebbian boost (correto) ou decay (incorreto) nos pesos
type FeedbackLoop struct {
	hebbianRT      HebbianRealTime
	graphStore     GraphStore
	db             Database
	learningRate   float64 // 0.05 default
	boostFactor    float64 // 1.5 default (50% boost se correto)
	decayFactor    float64 // 0.7 default (30% decay se incorreto)
}

// HebbianRealTime interface para Hebbian
type HebbianRealTime interface {
	UpdateWeights(ctx context.Context, patientID int64, nodeIDs []string) error
	BoostWeight(ctx context.Context, sourceID, targetID string, factor float64) error
	DecayWeight(ctx context.Context, sourceID, targetID string, factor float64) error
}

// Database interface para PostgreSQL
type Database interface {
	StoreFeedback(ctx context.Context, feedback *Feedback) error
	GetFeedbackHistory(ctx context.Context, patientID int64, limit int) ([]Feedback, error)
}

// Feedback feedback do cuidador
type Feedback struct {
	ID               int64
	PatientID        int64
	InterpretationID string
	Correct          bool
	CorrectedText    string
	Timestamp        time.Time
	AppliedToGraph   bool
	NodesAffected    []string
}

// FeedbackStats estatísticas de feedback
type FeedbackStats struct {
	PatientID         int64
	TotalFeedbacks    int
	CorrectCount      int
	IncorrectCount    int
	AccuracyRate      float64
	AvgResponseTimeMs int64
	LastFeedbackDate  time.Time
	MostCorrectedNodes []NodeFeedbackStat
}

// NodeFeedbackStat estatística de feedback por nó
type NodeFeedbackStat struct {
	NodeID        string
	NodeName      string
	CorrectCount  int
	IncorrectCount int
	AccuracyRate  float64
}

// NewFeedbackLoop cria novo feedback loop
func NewFeedbackLoop(hebbianRT HebbianRealTime, graphStore GraphStore, db Database) *FeedbackLoop {
	return &FeedbackLoop{
		hebbianRT:    hebbianRT,
		graphStore:   graphStore,
		db:           db,
		learningRate: 0.05,
		boostFactor:  1.5,  // +50% boost
		decayFactor:  0.7,  // -30% decay
	}
}

// Apply aplica feedback ao grafo (Hebbian boost ou decay)
func (f *FeedbackLoop) Apply(ctx context.Context, feedback *Feedback) error {
	// 1. Extrair nós mencionados na interpretação
	// TODO: Implementar NER (Named Entity Recognition) mais sofisticado
	// Por enquanto, usar heurística simples

	// 2. Identificar arestas relevantes
	edges, err := f.identifyRelevantEdges(ctx, feedback)
	if err != nil {
		return fmt.Errorf("failed to identify relevant edges: %w", err)
	}

	if len(edges) == 0 {
		// Sem arestas para ajustar
		return f.storeFeedback(ctx, feedback)
	}

	// 3. Aplicar boost ou decay
	nodesAffected := make([]string, 0)

	for _, edge := range edges {
		if feedback.Correct {
			// Feedback positivo → Boost Hebbiano
			if err := f.hebbianRT.BoostWeight(ctx, edge.SourceID, edge.TargetID, f.boostFactor); err != nil {
				return fmt.Errorf("failed to boost weight: %w", err)
			}
		} else {
			// Feedback negativo → Decay Hebbiano
			if err := f.hebbianRT.DecayWeight(ctx, edge.SourceID, edge.TargetID, f.decayFactor); err != nil {
				return fmt.Errorf("failed to decay weight: %w", err)
			}
		}

		nodesAffected = append(nodesAffected, edge.SourceID, edge.TargetID)
	}

	// 4. Remover duplicatas
	nodesAffected = uniqueStrings(nodesAffected)
	feedback.NodesAffected = nodesAffected
	feedback.AppliedToGraph = true

	// 5. Salvar feedback no banco
	if err := f.storeFeedback(ctx, feedback); err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	return nil
}

// identifyRelevantEdges identifica arestas relevantes para o feedback
func (f *FeedbackLoop) identifyRelevantEdges(ctx context.Context, feedback *Feedback) ([]Edge, error) {
	// Estratégia: identificar entidades mencionadas e suas arestas

	// TODO: Implementar NER (Named Entity Recognition)
	// Por enquanto, usar lista de nomes comuns como placeholder

	// Exemplo simplificado:
	// Se interpretação menciona "Maria" e "café", buscar aresta Maria-café

	edges := make([]Edge, 0)

	// Placeholder: criar arestas fictícias para teste
	// Na implementação real, usar NER + graph query

	edge := Edge{
		SourceID: "entity_maria",
		TargetID: "entity_cafe",
		Weight:   0.75,
	}

	edges = append(edges, edge)

	return edges, nil
}

// storeFeedback salva feedback no banco
func (f *FeedbackLoop) storeFeedback(ctx context.Context, feedback *Feedback) error {
	if f.db == nil {
		return nil // DB não configurado
	}

	return f.db.StoreFeedback(ctx, feedback)
}

// GetStats retorna estatísticas de feedback
func (f *FeedbackLoop) GetStats(ctx context.Context, patientID int64) (*FeedbackStats, error) {
	if f.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	// Recuperar histórico de feedbacks
	feedbacks, err := f.db.GetFeedbackHistory(ctx, patientID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback history: %w", err)
	}

	stats := &FeedbackStats{
		PatientID:      patientID,
		TotalFeedbacks: len(feedbacks),
	}

	if len(feedbacks) == 0 {
		return stats, nil
	}

	// Contar corretos/incorretos
	for _, fb := range feedbacks {
		if fb.Correct {
			stats.CorrectCount++
		} else {
			stats.IncorrectCount++
		}
	}

	// Calcular accuracy
	if stats.TotalFeedbacks > 0 {
		stats.AccuracyRate = float64(stats.CorrectCount) / float64(stats.TotalFeedbacks)
	}

	// Última data de feedback
	stats.LastFeedbackDate = feedbacks[0].Timestamp

	// Calcular estatísticas por nó
	nodeStats := f.calculateNodeStats(feedbacks)
	stats.MostCorrectedNodes = nodeStats

	return stats, nil
}

// calculateNodeStats calcula estatísticas por nó
func (f *FeedbackLoop) calculateNodeStats(feedbacks []Feedback) []NodeFeedbackStat {
	nodeMap := make(map[string]*NodeFeedbackStat)

	for _, fb := range feedbacks {
		for _, nodeID := range fb.NodesAffected {
			if _, exists := nodeMap[nodeID]; !exists {
				nodeMap[nodeID] = &NodeFeedbackStat{
					NodeID:   nodeID,
					NodeName: nodeID, // TODO: Resolver nome real do nó
				}
			}

			if fb.Correct {
				nodeMap[nodeID].CorrectCount++
			} else {
				nodeMap[nodeID].IncorrectCount++
			}
		}
	}

	// Converter map para slice e calcular accuracy
	stats := make([]NodeFeedbackStat, 0)
	for _, nodeStat := range nodeMap {
		total := nodeStat.CorrectCount + nodeStat.IncorrectCount
		if total > 0 {
			nodeStat.AccuracyRate = float64(nodeStat.CorrectCount) / float64(total)
		}
		stats = append(stats, *nodeStat)
	}

	// Ordenar por total de feedbacks (descendente)
	// Bubble sort simples
	for i := 0; i < len(stats)-1; i++ {
		for j := 0; j < len(stats)-i-1; j++ {
			total1 := stats[j].CorrectCount + stats[j].IncorrectCount
			total2 := stats[j+1].CorrectCount + stats[j+1].IncorrectCount
			if total1 < total2 {
				stats[j], stats[j+1] = stats[j+1], stats[j]
			}
		}
	}

	// Retornar top 10
	if len(stats) > 10 {
		stats = stats[:10]
	}

	return stats
}

// SetLearningRate ajusta learning rate
func (f *FeedbackLoop) SetLearningRate(rate float64) {
	if rate > 0.0 && rate <= 1.0 {
		f.learningRate = rate
	}
}

// SetBoostFactor ajusta boost factor
func (f *FeedbackLoop) SetBoostFactor(factor float64) {
	if factor >= 1.0 && factor <= 2.0 {
		f.boostFactor = factor
	}
}

// SetDecayFactor ajusta decay factor
func (f *FeedbackLoop) SetDecayFactor(factor float64) {
	if factor >= 0.5 && factor <= 1.0 {
		f.decayFactor = factor
	}
}

// GetConfig retorna configuração atual
func (f *FeedbackLoop) GetConfig() map[string]float64 {
	return map[string]float64{
		"learning_rate": f.learningRate,
		"boost_factor":  f.boostFactor,
		"decay_factor":  f.decayFactor,
	}
}

// Edge representa uma aresta no grafo
type Edge struct {
	SourceID string
	TargetID string
	Weight   float64
}

// Helper functions

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
