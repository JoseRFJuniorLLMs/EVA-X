// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// RetrievalService busca memórias por similaridade semântica
// RetrievalService busca memórias por similaridade semântica
// FDPNActivator interface para obter nós ativados pelo FDPN
type FDPNActivator interface {
	GetActivatedNodes(ctx context.Context, userID string, query string) (map[string]float64, error)
}

type RetrievalService struct {
	db            *sql.DB
	embedder      *EmbeddingService
	vectorAdapter *nietzscheInfra.VectorAdapter
	graphStore    *GraphStore // Para busca recursiva via NietzscheDB graph
	fdpn          FDPNActivator
	hebbianRT     *HebbianRealTime
}

// NewRetrievalService cria um novo serviço de busca
func NewRetrievalService(db *sql.DB, embedder *EmbeddingService, vectorAdapter *nietzscheInfra.VectorAdapter, graphStore *GraphStore) *RetrievalService {
	return &RetrievalService{
		db:            db,
		embedder:      embedder,
		vectorAdapter: vectorAdapter,
		graphStore:    graphStore, // Injetando GraphStore para busca recursiva
	}
}

// SearchResult representa um resultado de busca com score de similaridade
type SearchResult struct {
	Memory     *Memory
	Similarity float64 // 0.0 (nada similar) a 1.0 (idêntico)
	Score      float64 // Score composto (Smart Forgetting): similaridade + recência + importância
}

// Retrieve busca as K memórias mais relevantes para uma query (NietzscheDB vector)
func (r *RetrievalService) Retrieve(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
	// 1. Gerar embedding da query
	queryEmbedding, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	if r.vectorAdapter == nil {
		return nil, fmt.Errorf("vector adapter not initialized")
	}

	log.Printf("🔍 [MEMORY] Vector Search: Query=\"%s\"", query)

	// 2. BUSCA NO NIETZSCHEDB
	// Use HybridSearch (BM25 + KNN) when text query is available alongside vector.
	// Falls back to pure vector Search if HybridSearch fails or query is empty.
	var qResults []nietzscheInfra.VectorSearchResult
	if query != "" && r.vectorAdapter != nil {
		hybridResults, hybridErr := r.vectorAdapter.HybridSearch(ctx, "memories", query, queryEmbedding, k)
		if hybridErr != nil {
			log.Printf("⚠️ [MEMORY] HybridSearch failed, falling back to KNN: %v", hybridErr)
		} else {
			qResults = hybridResults
		}
	}
	// Fallback to pure vector search if hybrid was not used or failed
	if len(qResults) == 0 {
		var err2 error
		qResults, err2 = r.vectorAdapter.Search(ctx, "memories", queryEmbedding, k, idosoID)
		if err2 != nil {
			return nil, fmt.Errorf("erro busca vetorial: %w", err2)
		}
	}

	var allResults []*SearchResult
	resultsMap := make(map[int64]bool)
	var topSemanticIDs []int64

	// 3. Construir Memory diretamente do payload NietzscheDB (sem PG!)
	//    O payload já contém todos os campos (salvo no Store() step 3).
	for i, qr := range qResults {
		mem := memoryFromPayload(qr.Payload)
		if mem == nil || mem.ID == 0 {
			continue
		}

		allResults = append(allResults, &SearchResult{
			Memory:     mem,
			Similarity: qr.Score,
		})
		resultsMap[mem.ID] = true

		// Guardar os top 3 para expansão recursiva
		if i < 3 {
			topSemanticIDs = append(topSemanticIDs, mem.ID)
		}
	}

	log.Printf("✅ [MEMORY] %d resultados hidratados direto do NietzscheDB (sem PG)", len(allResults))

	// 4. BUSCA RECURSIVA VIA GRAFO (NietzscheDB graph A1/A5)
	//    Estes IDs NÃO têm payload — hidratamos do PG apenas eles.
	var graphOnlyIDs []int64
	if r.graphStore != nil && len(topSemanticIDs) > 0 {
		log.Printf("🕸️ [MEMORY] Iniciando Busca Recursiva para %d sementes", len(topSemanticIDs))

		for _, seedID := range topSemanticIDs {
			relatedIDs, err := r.graphStore.GetRelatedMemoriesRecursive(ctx, seedID, 5)
			if err != nil {
				log.Printf("⚠️ [MEMORY] Erro busca recursiva: %v", err)
				continue
			}

			for _, relatedID := range relatedIDs {
				if !resultsMap[relatedID] {
					graphOnlyIDs = append(graphOnlyIDs, relatedID)
					resultsMap[relatedID] = true
				}
			}
		}
	}

	// 5. HIDRATAR APENAS graph-recursive IDs do Postgres (minoria)
	if len(graphOnlyIDs) > 0 {
		queryIDs := make([]string, len(graphOnlyIDs))
		args := make([]interface{}, len(graphOnlyIDs))
		for i, id := range graphOnlyIDs {
			queryIDs[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}

		sqlQuery := fmt.Sprintf(`
			SELECT id, idoso_id, timestamp, speaker, content, emotion,
			       importance, topics, session_id, event_date, is_atomic
			FROM episodic_memories
			WHERE id IN (%s)
		`, strings.Join(queryIDs, ","))

		rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
		if err != nil {
			log.Printf("⚠️ [MEMORY] PG hydration fallback failed: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				mem := &Memory{}
				var topics string
				err := rows.Scan(
					&mem.ID, &mem.IdosoID, &mem.Timestamp, &mem.Speaker, &mem.Content,
					&mem.Emotion, &mem.Importance, &topics, &mem.SessionID,
					&mem.EventDate, &mem.IsAtomic,
				)
				if err != nil {
					continue
				}
				mem.Topics = parsePostgresArray(topics)
				allResults = append(allResults, &SearchResult{
					Memory:     mem,
					Similarity: 0.85, // Score "Associação Forte"
				})
			}
		}
		log.Printf("✅ [MEMORY] %d resultados graph-recursive hidratados do PG", len(graphOnlyIDs))
	}

	if len(allResults) == 0 {
		return []*SearchResult{}, nil
	}

	// Reordenar baseado no score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Similarity > allResults[j].Similarity
	})

	return allResults, nil
}

// RetrieveRecent busca memórias recentes (últimos N dias) sem usar embedding.
// Tenta NietzscheDB NQL primeiro, fallback para Postgres.
func (r *RetrievalService) RetrieveRecent(ctx context.Context, idosoID int64, days int, limit int) ([]*Memory, error) {
	log.Printf("🔍 [MEMORY] Busca Recente: Idoso=%d, Dias=%d, Limit=%d", idosoID, days, limit)

	// 1. Tentar NietzscheDB NQL
	if r.vectorAdapter != nil {
		cutoffMs := time.Now().AddDate(0, 0, -days).UnixMilli()
		nql := `MATCH (n) WHERE n.idoso_id = $idoso_id RETURN n ORDER BY n.importance DESC, n.timestamp DESC LIMIT $limit`
		params := map[string]interface{}{
			"idoso_id": idosoID,
			"limit":    limit,
		}
		payloads, err := r.vectorAdapter.ExecuteNQL(ctx, nql, params, "memories")
		if err == nil && len(payloads) > 0 {
			var memories []*Memory
			for _, p := range payloads {
				mem := memoryFromPayload(p)
				if mem == nil || mem.ID == 0 {
					continue
				}
				// Filter by time window (NQL WHERE may not support time arithmetic)
				if mem.Timestamp.UnixMilli() >= cutoffMs {
					memories = append(memories, mem)
				}
			}
			if len(memories) > 0 {
				log.Printf("✅ [MEMORY] RetrieveRecent: %d memórias do NietzscheDB", len(memories))
				return memories, nil
			}
		}
	}

	// 2. Fallback para Postgres
	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion,
		       importance, topics, session_id
		FROM episodic_memories
		WHERE idoso_id = $1
		  AND timestamp > NOW() - INTERVAL '1 day' * $2
		ORDER BY importance DESC, timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, idosoID, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		memory := &Memory{}
		var topics string
		err := rows.Scan(
			&memory.ID, &memory.IdosoID, &memory.Timestamp, &memory.Speaker,
			&memory.Content, &memory.Emotion, &memory.Importance, &topics,
			&memory.SessionID,
		)
		if err != nil {
			return nil, err
		}
		memory.Topics = parsePostgresArray(topics)
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

// RetrieveHybrid combina busca semântica + temporal
// Retorna memórias relevantes E recentes
func (r *RetrievalService) RetrieveHybrid(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
	// Buscar memórias semânticas
	semantic, err := r.Retrieve(ctx, idosoID, query, k)
	if err != nil {
		return nil, err
	}

	// Buscar memórias recentes (últimos 3 dias)
	recent, err := r.RetrieveRecent(ctx, idosoID, 3, k/2)
	if err != nil {
		log.Printf("⚠️ Erro ao buscar memórias recentes: %v", err)
		return semantic, nil // Retorna apenas semânticas
	}

	// Mesclar e deduplicar
	seen := make(map[int64]bool)
	var combined []*SearchResult

	// Adicionar semânticas primeiro
	for _, res := range semantic {
		if !seen[res.Memory.ID] {
			combined = append(combined, res)
			seen[res.Memory.ID] = true
		}
	}

	// Adicionar recentes (se não duplicadas)
	for _, mem := range recent {
		if !seen[mem.ID] {
			combined = append(combined, &SearchResult{
				Memory:     mem,
				Similarity: 0.9, // Score artificial alto para recentes
			})
			seen[mem.ID] = true

			if len(combined) >= k {
				break
			}
		}
	}

	// Aplicar Smart Forgetting (recência + importância) sobre os resultados combinados
	applySmartForgettingRanking(combined)

	return combined, nil
}

// applySmartForgettingRanking aplica o princípio de Smart Forgetting:
//   - Memórias muito antigas e pouco importantes perdem peso
//   - Memórias recentes e/ou importantes ganham prioridade
//   - Similaridade continua sendo o fator principal
func applySmartForgettingRanking(results []*SearchResult) {
	now := time.Now()

	for _, res := range results {
		// Fallback: se não tiver memória associada, usa apenas similaridade
		if res == nil || res.Memory == nil {
			res.Score = res.Similarity
			continue
		}

		// 1) Recência: decai exponencialmente com o tempo (janela ~30 dias)
		ageDays := now.Sub(res.Memory.Timestamp).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		// τ = 30 dias → memórias com ~30 dias ainda têm ~37% do peso de recência
		recencyBoost := math.Exp(-ageDays / 30.0)

		// 2) Importância: normaliza para [0,1] e dá boost adicional
		imp := res.Memory.Importance
		if imp < 0 {
			imp = 0
		}
		if imp > 1 {
			imp = 1
		}
		// Base 0.5 + até +0.5 dependendo da importância
		importanceBoost := 0.5 + 0.5*imp

		// 3) Similaridade continua sendo o peso principal
		sim := res.Similarity
		if sim < 0 {
			sim = 0
		}
		if sim > 1 {
			sim = 1
		}

		// 4) Score composto (pode ser ajustado depois):
		//    60% similaridade, 25% recência, 15% importância
		res.Score = sim*0.60 + recencyBoost*0.25 + importanceBoost*0.15
	}

	// Ordena em ordem decrescente de Score
	sort.Slice(results, func(i, j int) bool {
		// Em caso de empate, usa similaridade como desempate
		if results[i].Score == results[j].Score {
			return results[i].Similarity > results[j].Similarity
		}
		return results[i].Score > results[j].Score
	})
}

// RetrieveWithMode implementa o sistema híbrido:
//   - modo "fast"/"linear"/"sistema1": busca rápida (linear) usando Retrieve
//   - modo "slow"/"filotaxica"/"phyllotaxic"/"exploratory"/"sistema2": busca exploratória usando RetrieveHybrid
//   - modo "auto" (default): roteia com heurística simples baseada no texto da query
func (r *RetrievalService) RetrieveWithMode(ctx context.Context, idosoID int64, query string, k int, mode string) ([]*SearchResult, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))

	switch mode {
	case "fast", "linear", "sistema1":
		// Sistema 1: busca direta, sem enriquecimento extra
		return r.Retrieve(ctx, idosoID, query, k)

	case "slow", "filotaxica", "phyllotaxic", "exploratory", "sistema2":
		// Sistema 2: combina semântica + temporal + Smart Forgetting
		return r.RetrieveHybrid(ctx, idosoID, query, k)

	case "auto", "":
		// Heurística simples:
		// - Se a pergunta parece pedir "histórico", "todas as vezes", "ao longo do tempo" → modo lento
		// - Caso contrário → modo rápido
		q := strings.ToLower(query)
		if strings.Contains(q, "histórico") ||
			strings.Contains(q, "historico") ||
			strings.Contains(q, "todas as vezes") ||
			strings.Contains(q, "ao longo") ||
			strings.Contains(q, "ao longo do tempo") ||
			strings.Contains(q, "últimos meses") ||
			strings.Contains(q, "ultimos meses") {
			return r.RetrieveHybrid(ctx, idosoID, query, k)
		}
		return r.Retrieve(ctx, idosoID, query, k)

	default:
		// Modo desconhecido → fallback para rápido
		return r.Retrieve(ctx, idosoID, query, k)
	}
}
