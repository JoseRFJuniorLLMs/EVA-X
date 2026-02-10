package learning

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// MetaLearner implementa meta-aprendizado: aprender a aprender
// Monitora falhas de retrieval, detecta padroes, e ajusta estrategias
// Ciencia: Wang et al. (2016) - "Learning to reinforcement learn"
type MetaLearner struct {
	strategies      []RetrievalStrategy
	failureLog      []FailureRecord
	parameterTuning map[string]float64 // Hiperparametros ajustaveis
	mu              sync.RWMutex
	lastEvaluation  time.Time
	evaluationCycle int // A cada N interacoes, reavalia
}

// RetrievalStrategy uma estrategia de retrieval aprendida
type RetrievalStrategy struct {
	Name          string    `json:"name"`
	QueryType     string    `json:"query_type"`     // "emotional", "causal", "factual", "temporal"
	PreferredDB   string    `json:"preferred_db"`   // "krylov", "neo4j", "qdrant", "combined"
	Effectiveness float64   `json:"effectiveness"`  // 0-1 (atualizado continuamente)
	UsageCount    int64     `json:"usage_count"`
	SuccessCount  int64     `json:"success_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// FailureRecord registro de falha de retrieval
type FailureRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	QueryType   string    `json:"query_type"`
	QueryText   string    `json:"query_text"`
	Strategy    string    `json:"strategy_used"`
	Reason      string    `json:"reason"`
	PatientID   int64     `json:"patient_id"`
	RetrievedN  int       `json:"retrieved_count"`
	WasUseful   bool      `json:"was_useful"`
}

// MetaLearningStats estatisticas do meta-learner
type MetaLearningStats struct {
	TotalStrategies    int                `json:"total_strategies"`
	TotalFailures      int                `json:"total_failures"`
	OverallAccuracy    float64            `json:"overall_accuracy"`
	StrategyStats      []RetrievalStrategy `json:"strategy_stats"`
	ParameterTuning    map[string]float64 `json:"parameter_tuning"`
	LastEvaluation     string             `json:"last_evaluation"`
}

// NewMetaLearner cria um novo meta-learner com estrategias iniciais
func NewMetaLearner() *MetaLearner {
	ml := &MetaLearner{
		evaluationCycle: 100, // Avalia a cada 100 interacoes
		parameterTuning: map[string]float64{
			"krylov_k":              64.0,   // Dimensao Krylov
			"similarity_threshold":  0.7,    // Threshold de similaridade
			"neo4j_depth":           2.0,    // Profundidade de busca no grafo
			"decay_tau":             90.0,   // Constante de decay temporal
			"pruning_max_age":       30.0,   // Idade maxima para poda
			"consolidation_min_hot": 5.0,    // Minimo de memorias quentes
		},
		strategies: []RetrievalStrategy{
			{
				Name:          "krylov_semantic",
				QueryType:     "factual",
				PreferredDB:   "krylov",
				Effectiveness: 0.85,
				CreatedAt:     time.Now(),
			},
			{
				Name:          "neo4j_causal",
				QueryType:     "causal",
				PreferredDB:   "neo4j",
				Effectiveness: 0.75,
				CreatedAt:     time.Now(),
			},
			{
				Name:          "combined_emotional",
				QueryType:     "emotional",
				PreferredDB:   "combined",
				Effectiveness: 0.80,
				CreatedAt:     time.Now(),
			},
			{
				Name:          "qdrant_temporal",
				QueryType:     "temporal",
				PreferredDB:   "qdrant",
				Effectiveness: 0.70,
				CreatedAt:     time.Now(),
			},
		},
	}

	return ml
}

// RecordOutcome registra o resultado de um retrieval (sucesso ou falha)
func (ml *MetaLearner) RecordOutcome(queryType, queryText, strategyUsed string, patientID int64, retrievedCount int, wasUseful bool) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	// Registrar no log
	record := FailureRecord{
		Timestamp:  time.Now(),
		QueryType:  queryType,
		QueryText:  queryText,
		Strategy:   strategyUsed,
		PatientID:  patientID,
		RetrievedN: retrievedCount,
		WasUseful:  wasUseful,
	}

	if !wasUseful {
		record.Reason = fmt.Sprintf("retrieval of %d items was not useful for %s query", retrievedCount, queryType)
	}

	ml.failureLog = append(ml.failureLog, record)

	// Limitar log a ultimas 1000 entradas
	if len(ml.failureLog) > 1000 {
		ml.failureLog = ml.failureLog[len(ml.failureLog)-1000:]
	}

	// Atualizar efetividade da estrategia usada
	for i := range ml.strategies {
		if ml.strategies[i].Name == strategyUsed {
			ml.strategies[i].UsageCount++
			if wasUseful {
				ml.strategies[i].SuccessCount++
			}
			if ml.strategies[i].UsageCount > 0 {
				ml.strategies[i].Effectiveness = float64(ml.strategies[i].SuccessCount) / float64(ml.strategies[i].UsageCount)
			}
			break
		}
	}

	// Verificar se e hora de avaliar
	totalInteractions := int64(0)
	for _, s := range ml.strategies {
		totalInteractions += s.UsageCount
	}
	if totalInteractions%int64(ml.evaluationCycle) == 0 {
		ml.evaluateAndAdapt()
	}
}

// SelectStrategy seleciona a melhor estrategia para um tipo de query
func (ml *MetaLearner) SelectStrategy(queryType string) *RetrievalStrategy {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	var best *RetrievalStrategy
	bestScore := -1.0

	for i := range ml.strategies {
		s := &ml.strategies[i]

		// Score = effectiveness * type_match_bonus
		score := s.Effectiveness
		if s.QueryType == queryType {
			score *= 1.5 // Bonus por match de tipo
		}

		// Exploration bonus: estrategias pouco usadas recebem boost
		if s.UsageCount < 10 {
			score += 0.1
		}

		if score > bestScore {
			bestScore = score
			best = s
		}
	}

	return best
}

// GetRecommendedParameters retorna parametros ajustados pelo meta-learner
func (ml *MetaLearner) GetRecommendedParameters() map[string]float64 {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	params := make(map[string]float64)
	for k, v := range ml.parameterTuning {
		params[k] = v
	}
	return params
}

// evaluateAndAdapt avalia performance e adapta estrategias/parametros
func (ml *MetaLearner) evaluateAndAdapt() {
	ml.lastEvaluation = time.Now()

	log.Println("[META-LEARNER] Iniciando avaliacao periodica...")

	// 1. Podar estrategias ineficazes (< 30% de sucesso com > 20 usos)
	prunedCount := 0
	activeStrategies := make([]RetrievalStrategy, 0, len(ml.strategies))
	for _, s := range ml.strategies {
		if s.Effectiveness < 0.3 && s.UsageCount > 20 {
			log.Printf("[META-LEARNER] Estrategia '%s' podada (effectiveness=%.2f, usage=%d)",
				s.Name, s.Effectiveness, s.UsageCount)
			prunedCount++
			continue
		}
		activeStrategies = append(activeStrategies, s)
	}
	ml.strategies = activeStrategies

	// 2. Detectar padroes de falha
	failurePatterns := ml.detectFailurePatterns()

	// 3. Ajustar parametros baseado em padroes
	for pattern, count := range failurePatterns {
		if count > 10 {
			ml.adjustParameters(pattern, count)
		}
	}

	// 4. Sintetizar nova estrategia se necessario
	if prunedCount > 0 {
		ml.synthesizeNewStrategy(failurePatterns)
	}

	log.Printf("[META-LEARNER] Avaliacao completa: %d estrategias ativas, %d podadas, %d padroes de falha",
		len(ml.strategies), prunedCount, len(failurePatterns))
}

// detectFailurePatterns agrupa falhas por tipo
func (ml *MetaLearner) detectFailurePatterns() map[string]int {
	patterns := make(map[string]int)

	// Analisar ultimas 100 falhas
	recentFailures := ml.failureLog
	if len(recentFailures) > 100 {
		recentFailures = recentFailures[len(recentFailures)-100:]
	}

	for _, f := range recentFailures {
		if !f.WasUseful {
			key := fmt.Sprintf("%s_%s", f.QueryType, f.Strategy)
			patterns[key]++
		}
	}

	return patterns
}

// adjustParameters ajusta hiperparametros baseado em padroes de falha
func (ml *MetaLearner) adjustParameters(pattern string, failCount int) {
	// Ajuste conservador (max 10% por ciclo)
	adjustRate := math.Min(float64(failCount)/100.0, 0.1)

	switch {
	case pattern == "factual_krylov_semantic":
		// Se queries factuais falham no Krylov, aumentar dimensao
		current := ml.parameterTuning["krylov_k"]
		ml.parameterTuning["krylov_k"] = math.Min(current*(1+adjustRate), 256)
		log.Printf("[META-LEARNER] Krylov K ajustado: %.0f -> %.0f", current, ml.parameterTuning["krylov_k"])

	case pattern == "emotional_combined_emotional":
		// Se queries emocionais falham, diminuir threshold
		current := ml.parameterTuning["similarity_threshold"]
		ml.parameterTuning["similarity_threshold"] = math.Max(current*(1-adjustRate), 0.3)
		log.Printf("[META-LEARNER] Threshold ajustado: %.2f -> %.2f", current, ml.parameterTuning["similarity_threshold"])

	case pattern == "causal_neo4j_causal":
		// Se queries causais falham no Neo4j, aumentar profundidade
		current := ml.parameterTuning["neo4j_depth"]
		ml.parameterTuning["neo4j_depth"] = math.Min(current+1, 4)
		log.Printf("[META-LEARNER] Neo4j depth ajustado: %.0f -> %.0f", current, ml.parameterTuning["neo4j_depth"])
	}
}

// synthesizeNewStrategy cria nova estrategia baseada em padroes de falha
func (ml *MetaLearner) synthesizeNewStrategy(failurePatterns map[string]int) {
	// Encontrar o tipo de query com mais falhas
	worstType := ""
	worstCount := 0
	for pattern, count := range failurePatterns {
		if count > worstCount {
			worstCount = count
			worstType = pattern
		}
	}

	if worstType == "" {
		return
	}

	// Criar estrategia combinada para o pior tipo
	newStrategy := RetrievalStrategy{
		Name:          fmt.Sprintf("adaptive_%s_%d", worstType, time.Now().Unix()),
		QueryType:     worstType,
		PreferredDB:   "combined",
		Effectiveness: 0.5, // Comeca neutro
		CreatedAt:     time.Now(),
	}

	ml.strategies = append(ml.strategies, newStrategy)
	log.Printf("[META-LEARNER] Nova estrategia sintetizada: %s para queries tipo '%s'",
		newStrategy.Name, worstType)
}

// GetStatistics retorna estatisticas completas
func (ml *MetaLearner) GetStatistics() map[string]interface{} {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	totalUsage := int64(0)
	totalSuccess := int64(0)
	for _, s := range ml.strategies {
		totalUsage += s.UsageCount
		totalSuccess += s.SuccessCount
	}

	overallAccuracy := 0.0
	if totalUsage > 0 {
		overallAccuracy = float64(totalSuccess) / float64(totalUsage)
	}

	strategies := make([]map[string]interface{}, len(ml.strategies))
	for i, s := range ml.strategies {
		strategies[i] = map[string]interface{}{
			"name":          s.Name,
			"query_type":    s.QueryType,
			"preferred_db":  s.PreferredDB,
			"effectiveness": fmt.Sprintf("%.1f%%", s.Effectiveness*100),
			"usage_count":   s.UsageCount,
		}
	}

	return map[string]interface{}{
		"engine":            "meta_learner",
		"total_strategies":  len(ml.strategies),
		"overall_accuracy":  fmt.Sprintf("%.1f%%", overallAccuracy*100),
		"total_interactions": totalUsage,
		"failure_log_size":  len(ml.failureLog),
		"strategies":        strategies,
		"parameter_tuning":  ml.parameterTuning,
		"last_evaluation":   ml.lastEvaluation.Format(time.RFC3339),
		"status":            "active",
	}
}
