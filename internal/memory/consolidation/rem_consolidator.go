package consolidation

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"
	krylovmem "eva-mind/internal/memory/krylov"

	"gonum.org/v1/gonum/mat"
)

// REMConsolidator implementa consolidacao de memoria inspirada no sono REM
// Pipeline: Episodicas quentes -> SRC selective replay -> Spectral clustering -> Centroide Krylov -> Nó semantico Neo4j -> Prune redundancias
// Ciencia: Rasch & Born (2013) - "About sleep's role in memory" (Physiological Reviews)
//          Tadros et al. (2022) - "Sleep-like Unsupervised Replay" (Nature Communications)
type REMConsolidator struct {
	neo4j     *graph.Neo4jClient
	krylov    *krylovmem.KrylovMemoryManager
	tau       float64 // Constante de decay temporal em dias
	minHot    int     // Minimo de memorias quentes para consolidar
	srcConfig *SelectiveReplayConfig
	hebbian   *HebbianStrengthener
	mu        sync.Mutex
}

// ConsolidationResult resultado de um ciclo de consolidacao
type ConsolidationResult struct {
	CycleTime               time.Time `json:"cycle_time"`
	EpisodicProcessed       int       `json:"episodic_processed"`
	CommunitiesFormed       int       `json:"communities_formed"`
	SemanticNodesCreated    int       `json:"semantic_nodes_created"`
	MemoriesPruned          int       `json:"memories_pruned"`
	StorageSavedPercent     float64   `json:"storage_saved_percent"`
	DissonantMemories       int       `json:"dissonant_memories"`
	HebbianEdgesStrengthened int      `json:"hebbian_edges_strengthened"`
	AvgDissonance           float64   `json:"avg_dissonance"`
	Duration                string    `json:"duration"`
}

// EpisodicMemory representa uma memoria episodica para consolidacao
type EpisodicMemory struct {
	ID              string
	Content         string
	Embedding       []float64
	ActivationScore float64
	CreatedAt       time.Time
	PatientID       int64
}

// ProtoConcept conceito abstrato extraido de um cluster de memorias
type ProtoConcept struct {
	Centroid         []float64
	CommonSignifiers []string
	ExemplarIDs      []string // 3 exemplos prototipos
	MemberCount      int
	AbstractionLevel int
	Label            string
}

// NewREMConsolidator cria um novo consolidador REM
func NewREMConsolidator(neo4j *graph.Neo4jClient, krylov *krylovmem.KrylovMemoryManager) *REMConsolidator {
	return &REMConsolidator{
		neo4j:     neo4j,
		krylov:    krylov,
		tau:       90.0,
		minHot:    5,
		srcConfig: DefaultSelectiveReplayConfig(),
		hebbian:   NewHebbianStrengthener(neo4j, 1.5),
	}
}

// ConsolidateNightly executa consolidacao noturna para um paciente
// Deve ser chamado pelo scheduler (cron) as 3h da manha
func (r *REMConsolidator) ConsolidateNightly(ctx context.Context, patientID int64) (*ConsolidationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()
	result := &ConsolidationResult{CycleTime: start}

	log.Printf("[REM] Iniciando consolidacao noturna para paciente %d", patientID)

	// 1. Buscar memorias episodicas "quentes" (alto activation score nas ultimas 24h)
	hotMemories, err := r.getHotEpisodicMemories(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar memorias quentes: %w", err)
	}

	result.EpisodicProcessed = len(hotMemories)
	if len(hotMemories) < r.minHot {
		log.Printf("[REM] Paciente %d: apenas %d memorias quentes (minimo %d), pulando consolidacao",
			patientID, len(hotMemories), r.minHot)
		result.Duration = time.Since(start).String()
		return result, nil
	}

	// 2. SRC Selective Replay: prioritize dissonant memories (Tadros et al., 2022)
	if r.srcConfig != nil && len(hotMemories) >= 3 {
		replayResult := r.ExecuteSelectiveReplay(ctx, patientID, hotMemories, r.srcConfig, r.hebbian)
		if replayResult != nil {
			result.DissonantMemories = replayResult.DissonantCount
			result.HebbianEdgesStrengthened = replayResult.HebbianEdges
			result.AvgDissonance = replayResult.AvgDissonance
		}
	} else {
		// Fallback: replay all (original behavior)
		for _, mem := range hotMemories {
			if len(mem.Embedding) > 0 {
				_ = r.krylov.UpdateSubspace(mem.Embedding)
			}
		}
	}

	// 3. Clustering: agrupar memorias similares via similaridade coseno
	communities := r.clusterBySimilarity(hotMemories)
	result.CommunitiesFormed = len(communities)

	// 4. Abstracao: para cada comunidade, gerar proto-conceito semantico
	for _, comm := range communities {
		if len(comm) < 2 {
			continue
		}

		concept := r.abstractCommunity(comm)

		// 5. Criar no semantico no Neo4j
		err := r.createSemanticNode(ctx, patientID, concept)
		if err != nil {
			log.Printf("[REM] Erro ao criar no semantico: %v", err)
			continue
		}
		result.SemanticNodesCreated++

		// 6. Prunar memorias redundantes dentro da comunidade
		// Manter os 3 exemplares, deletar o resto
		if len(comm) > 3 {
			pruned := r.pruneRedundantMemories(ctx, comm, concept.ExemplarIDs)
			result.MemoriesPruned += pruned
		}
	}

	// Calcular economia de storage
	if result.EpisodicProcessed > 0 {
		result.StorageSavedPercent = float64(result.MemoriesPruned) / float64(result.EpisodicProcessed) * 100.0
	}

	result.Duration = time.Since(start).String()

	log.Printf("[REM] Consolidacao paciente %d: %d episodicas -> %d comunidades -> %d semanticas, %d podadas (%.1f%% storage economizado) em %s",
		patientID, result.EpisodicProcessed, result.CommunitiesFormed,
		result.SemanticNodesCreated, result.MemoriesPruned,
		result.StorageSavedPercent, result.Duration)

	return result, nil
}

// ConsolidateAll executa consolidacao para todos os pacientes ativos
func (r *REMConsolidator) ConsolidateAll(ctx context.Context) ([]*ConsolidationResult, error) {
	patientIDs, err := r.getActivePatientIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar pacientes ativos: %w", err)
	}

	log.Printf("[REM] Iniciando consolidacao noturna para %d pacientes", len(patientIDs))

	var results []*ConsolidationResult
	for _, pid := range patientIDs {
		res, err := r.ConsolidateNightly(ctx, pid)
		if err != nil {
			log.Printf("[REM] Erro na consolidacao do paciente %d: %v", pid, err)
			continue
		}
		results = append(results, res)
	}

	return results, nil
}

// getHotEpisodicMemories busca memorias com alto activation score nas ultimas 24h
func (r *REMConsolidator) getHotEpisodicMemories(ctx context.Context, patientID int64) ([]EpisodicMemory, error) {
	query := `
		MATCH (p:Person {id: $patientId})-[:EXPERIENCED]->(m:Event)
		WHERE m.type = 'episodic'
		  AND m.timestamp > datetime() - duration('P1D')
		WITH m, COALESCE(m.activation_score, 1.0) AS score
		ORDER BY score DESC
		LIMIT 200
		RETURN toString(id(m)) AS memId,
		       COALESCE(m.content, '') AS content,
		       score AS activationScore,
		       m.created_at AS createdAt
	`

	records, err := r.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return nil, err
	}

	var memories []EpisodicMemory
	for _, rec := range records {
		memID, _ := rec.Get("memId")
		content, _ := rec.Get("content")
		score, _ := rec.Get("activationScore")

		mem := EpisodicMemory{
			ID:        fmt.Sprintf("%v", memID),
			Content:   fmt.Sprintf("%v", content),
			PatientID: patientID,
		}

		if s, ok := score.(float64); ok {
			mem.ActivationScore = s
		}

		memories = append(memories, mem)
	}

	return memories, nil
}

// clusterBySimilarity agrupa memorias por similaridade coseno usando threshold
func (r *REMConsolidator) clusterBySimilarity(memories []EpisodicMemory) [][]EpisodicMemory {
	if len(memories) == 0 {
		return nil
	}

	threshold := 0.7
	assigned := make([]bool, len(memories))
	var communities [][]EpisodicMemory

	for i := range memories {
		if assigned[i] {
			continue
		}

		community := []EpisodicMemory{memories[i]}
		assigned[i] = true

		for j := i + 1; j < len(memories); j++ {
			if assigned[j] {
				continue
			}

			sim := r.cosineSimilarity(memories[i].Embedding, memories[j].Embedding)
			if sim > threshold {
				community = append(community, memories[j])
				assigned[j] = true
			}
		}

		communities = append(communities, community)
	}

	return communities
}

// abstractCommunity gera um proto-conceito a partir de um cluster de memorias
func (r *REMConsolidator) abstractCommunity(comm []EpisodicMemory) *ProtoConcept {
	concept := &ProtoConcept{
		MemberCount:      len(comm),
		AbstractionLevel: 1,
	}

	// Calcular centroide (media dos embeddings)
	if len(comm) > 0 && len(comm[0].Embedding) > 0 {
		dim := len(comm[0].Embedding)
		centroid := make([]float64, dim)
		validCount := 0

		for _, mem := range comm {
			if len(mem.Embedding) == dim {
				for d := 0; d < dim; d++ {
					centroid[d] += mem.Embedding[d]
				}
				validCount++
			}
		}

		if validCount > 0 {
			for d := range centroid {
				centroid[d] /= float64(validCount)
			}
			// Normalizar centroide
			norm := 0.0
			for _, v := range centroid {
				norm += v * v
			}
			norm = math.Sqrt(norm)
			if norm > 1e-10 {
				for d := range centroid {
					centroid[d] /= norm
				}
			}
			concept.Centroid = centroid
		}
	}

	// Selecionar 3 exemplares (os de maior activation score)
	maxExemplars := 3
	if len(comm) < maxExemplars {
		maxExemplars = len(comm)
	}
	for i := 0; i < maxExemplars; i++ {
		concept.ExemplarIDs = append(concept.ExemplarIDs, comm[i].ID)
	}

	// Label = conteudo do exemplar mais ativado
	if len(comm) > 0 && comm[0].Content != "" {
		label := comm[0].Content
		if len(label) > 100 {
			label = label[:100]
		}
		concept.Label = label
	}

	return concept
}

// createSemanticNode cria um no semantico no Neo4j a partir de um proto-conceito
func (r *REMConsolidator) createSemanticNode(ctx context.Context, patientID int64, concept *ProtoConcept) error {
	query := `
		MATCH (p:Person {id: $patientId})
		CREATE (s:SemanticMemory {
			label: $label,
			member_count: $memberCount,
			abstraction_level: $abstractionLevel,
			timestamp: datetime(),
			source: 'rem_consolidation'
		})
		CREATE (p)-[:HAS_SEMANTIC]->(s)
		WITH s
		UNWIND $exemplarIds AS eid
		MATCH (m:Event) WHERE toString(id(m)) = eid
		CREATE (s)-[:ABSTRACTED_FROM]->(m)
		RETURN toString(id(s)) AS newId
	`

	_, err := r.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId":        patientID,
		"label":            concept.Label,
		"memberCount":      concept.MemberCount,
		"abstractionLevel": concept.AbstractionLevel,
		"exemplarIds":      concept.ExemplarIDs,
	})

	return err
}

// pruneRedundantMemories remove memorias redundantes, mantendo os exemplares
func (r *REMConsolidator) pruneRedundantMemories(ctx context.Context, comm []EpisodicMemory, keepIDs []string) int {
	keepSet := make(map[string]bool)
	for _, id := range keepIDs {
		keepSet[id] = true
	}

	pruned := 0
	for _, mem := range comm {
		if keepSet[mem.ID] {
			continue
		}

		// Marcar como consolidada (nao deletar fisicamente, soft-delete)
		query := `
		MATCH (m:Event) WHERE toString(id(m)) = $memId
		SET m.consolidated = true,
		    m.consolidated_at = datetime(),
		    m.type = 'consolidated_episodic'
		`
		_, err := r.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
			"memId": mem.ID,
		})
		if err == nil {
			pruned++
		}
	}

	return pruned
}

// getActivePatientIDs retorna IDs de pacientes com atividade recente
func (r *REMConsolidator) getActivePatientIDs(ctx context.Context) ([]int64, error) {
	query := `
		MATCH (p:Person)-[:EXPERIENCED]->(m:Event)
		WHERE m.timestamp > datetime() - duration('P1D')
		RETURN DISTINCT p.id AS patientId
	`

	records, err := r.neo4j.ExecuteRead(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, rec := range records {
		pid, _ := rec.Get("patientId")
		if id, ok := pid.(int64); ok {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// cosineSimilarity calcula similaridade coseno entre dois vetores
func (r *REMConsolidator) cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	va := mat.NewVecDense(len(a), a)
	vb := mat.NewVecDense(len(b), b)

	dot := mat.Dot(va, vb)
	normA := mat.Norm(va, 2)
	normB := mat.Norm(vb, 2)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}

// GetStatistics retorna estatisticas do consolidador
func (r *REMConsolidator) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"engine":   "rem_consolidator",
		"tau_days": r.tau,
		"min_hot":  r.minHot,
		"status":   "active",
	}
}
