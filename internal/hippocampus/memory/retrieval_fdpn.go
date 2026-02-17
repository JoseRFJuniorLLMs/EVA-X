// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - FDPN → Retrieval Integration
// Integra energia de ativação do FDPN no ranking de busca
// Fase B do plano de implementação
package memory

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// RetrieveHybridWithFDPN busca memórias usando Qdrant + FDPN boost
// O FDPN prima o grafo ANTES da busca e injeta energia no ranking
func (r *RetrievalService) RetrieveHybridWithFDPN(ctx context.Context, idosoID int64, query string, k int) ([]*SearchResult, error) {
	if r.qdrant == nil {
		return nil, fmt.Errorf("qdrant client not initialized")
	}

	log.Printf("🔍 [RETRIEVAL+FDPN] Query=\"%s\" (patient=%d)", query, idosoID)

	// 1. Gerar embedding da query
	queryEmbedding, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	// 2. ✅ NOVO: Prima o grafo ANTES da busca
	activatedNodes := make(map[string]float64)
	if r.fdpn != nil {
		activatedNodes, err = r.fdpn.GetActivatedNodes(ctx, fmt.Sprintf("%d", idosoID), query)
		if err != nil {
			log.Printf("⚠️ [RETRIEVAL+FDPN] FDPN prime failed: %v (continuing without boost)", err)
		} else {
			log.Printf("✅ [RETRIEVAL+FDPN] FDPN activated %d nodes", len(activatedNodes))
		}
	}

	// 3. Busca no Qdrant
	qResults, err := r.qdrant.Search(ctx, "memories", queryEmbedding, uint64(k*2), nil) // 2x para ter margem pós-boost
	if err != nil {
		return nil, fmt.Errorf("erro busca Qdrant: %w", err)
	}

	var allResults []*SearchResult
	var memoryIDs []int64
	resultsMap := make(map[int64]*SearchResult)
	var activatedNodeIDs []string

	// 4. Processar resultados do Qdrant
	for _, qr := range qResults {
		// Extrair ID do payload
		p := qr.Payload
		var memID int64

		if idVal, ok := p["id"].GetKind().(*qdrant.Value_IntegerValue); ok {
			memID = idVal.IntegerValue
		} else if idVal, ok := p["id"].GetKind().(*qdrant.Value_DoubleValue); ok {
			memID = int64(idVal.DoubleValue)
		} else {
			continue
		}

		// Criar SearchResult
		result := &SearchResult{
			Memory:     &Memory{ID: memID},
			Similarity: float64(qr.Score),
			Score:      float64(qr.Score), // Score inicial = similarity
		}

		memoryIDs = append(memoryIDs, memID)
		resultsMap[memID] = result
	}

	// 5. ✅ NOVO: Boost memórias cujos nós foram ativados pelo FDPN
	if len(activatedNodes) > 0 {
		boostedCount := 0
		for memID, result := range resultsMap {
			nodeID := fmt.Sprintf("memory_%d", memID)

			if activationScore, exists := activatedNodes[nodeID]; exists {
				originalScore := result.Score
				// Boost: +15% proporcional à ativação
				boostFactor := 0.15 * activationScore
				result.Score *= (1.0 + boostFactor)

				log.Printf("   🎯 [RETRIEVAL+FDPN] Boosted memory %d: %.3f → %.3f (+%.1f%%)",
					memID, originalScore, result.Score, boostFactor*100)

				boostedCount++
				activatedNodeIDs = append(activatedNodeIDs, nodeID)
			}
		}

		log.Printf("✅ [RETRIEVAL+FDPN] Boosted %d/%d memories", boostedCount, len(resultsMap))
	}

	// 6. Busca recursiva via Neo4j (top 3 semanticamente)
	var topSemanticIDs []int64
	for i := 0; i < len(memoryIDs) && i < 3; i++ {
		topSemanticIDs = append(topSemanticIDs, memoryIDs[i])
	}

	if r.graphStore != nil && len(topSemanticIDs) > 0 {
		log.Printf("🕸️ [RETRIEVAL+FDPN] Iniciando busca recursiva para %d sementes", len(topSemanticIDs))

		for _, seedID := range topSemanticIDs {
			relatedIDs, err := r.graphStore.GetRelatedMemoriesRecursive(ctx, seedID, 5)
			if err != nil {
				log.Printf("⚠️ [RETRIEVAL+FDPN] Erro busca recursiva: %v", err)
				continue
			}

			for _, relatedID := range relatedIDs {
				if _, exists := resultsMap[relatedID]; !exists {
					// Memória relacionada via grafo (não via Qdrant)
					result := &SearchResult{
						Memory:     &Memory{ID: relatedID},
						Similarity: 0.85, // Score "Associação Forte"
						Score:      0.85,
					}
					memoryIDs = append(memoryIDs, relatedID)
					resultsMap[relatedID] = result
				}
			}
		}
	}

	// 7. Hidratar com dados do Postgres
	if len(memoryIDs) == 0 {
		return []*SearchResult{}, nil
	}

	queryIDs := make([]string, len(memoryIDs))
	args := make([]interface{}, len(memoryIDs))
	for i, id := range memoryIDs {
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
		return nil, fmt.Errorf("erro ao hidratar memórias: %w", err)
	}
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
			log.Printf("⚠️ [RETRIEVAL+FDPN] Erro scan: %v", err)
			continue
		}

		if topics != "" {
			mem.Topics = strings.Split(topics, ",")
		}

		if result, exists := resultsMap[mem.ID]; exists {
			result.Memory = mem
			allResults = append(allResults, result)
		}
	}

	// 8. Ordenar por Score (com boost do FDPN aplicado)
	sortByScore(allResults)

	// 9. Limitar a k resultados
	if len(allResults) > k {
		allResults = allResults[:k]
	}

	// 10. ✅ NOVO: Atualizar Hebbian Real-Time (goroutine)
	if r.hebbianRT != nil && len(activatedNodeIDs) > 0 {
		go r.hebbianRT.UpdateWeights(ctx, idosoID, activatedNodeIDs)
	}

	log.Printf("✅ [RETRIEVAL+FDPN] Returned %d results (boosted by FDPN)", len(allResults))

	return allResults, nil
}

// GetActivatedNodes obtém nós ativados pelo FDPN
// Wrapper para compatibilidade com FDPNEngine atual
func (e *FDPNEngine) GetActivatedNodes(ctx context.Context, userID string, query string) (map[string]float64, error) {
	// Extrair keywords
	keywords := e.extractKeywords(query)
	if len(keywords) == 0 {
		return make(map[string]float64), nil
	}

	activatedNodes := make(map[string]float64)

	// Para cada keyword, buscar nós ativados no cache local
	for _, kw := range keywords {
		cacheKey := fmt.Sprintf("%s:%s", userID, kw)
		if value, ok := e.localCache.Load(cacheKey); ok {
			// Ativação encontrada no cache
			if activation, ok := value.(float64); ok {
				nodeID := fmt.Sprintf("node_%s", kw)
				activatedNodes[nodeID] = activation
			} else if activation, ok := value.(*SubgraphActivation); ok {
				// SubgraphActivation completa
				for _, node := range activation.Nodes {
					activatedNodes[node.ID] = node.Activation
				}
			}
		}
	}

	// Se não há cache, fazer priming síncrono (simplificado)
	if len(activatedNodes) == 0 {
		for _, kw := range keywords {
			// Ativação base proporcional ao keyword
			activation := calculateKeywordActivation(kw)
			nodeID := fmt.Sprintf("node_%s", kw)
			activatedNodes[nodeID] = activation
		}
	}

	return activatedNodes, nil
}

// calculateKeywordActivation calcula ativação base para um keyword
func calculateKeywordActivation(keyword string) float64 {
	// Keywords mais longos = mais específicos = maior ativação
	baseActivation := 0.5
	if len(keyword) > 5 {
		baseActivation = 0.7
	}
	if len(keyword) > 10 {
		baseActivation = 0.9
	}
	return baseActivation
}

// sortByScore ordena resultados por Score (descendente)
func sortByScore(results []*SearchResult) {
	// Bubble sort simplificado (para poucos elementos é eficiente)
	n := len(results)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}

// extractNodeIDFromMemory extrai nodeID de uma memória
func extractNodeIDFromMemory(memID int64) string {
	return fmt.Sprintf("memory_%d", memID)
}

// FDPNBoostStats estatísticas do boost FDPN
type FDPNBoostStats struct {
	TotalMemories   int
	BoostedMemories int
	AvgBoostFactor  float64
	MaxBoostFactor  float64
}

// GetFDPNBoostStats retorna estatísticas do boost
func (r *RetrievalService) GetFDPNBoostStats(results []*SearchResult, activatedNodes map[string]float64) *FDPNBoostStats {
	stats := &FDPNBoostStats{
		TotalMemories: len(results),
	}

	var totalBoost float64
	for _, result := range results {
		nodeID := extractNodeIDFromMemory(result.Memory.ID)
		if activation, exists := activatedNodes[nodeID]; exists {
			boostFactor := 0.15 * activation
			stats.BoostedMemories++
			totalBoost += boostFactor

			if boostFactor > stats.MaxBoostFactor {
				stats.MaxBoostFactor = boostFactor
			}
		}
	}

	if stats.BoostedMemories > 0 {
		stats.AvgBoostFactor = totalBoost / float64(stats.BoostedMemories)
	}

	return stats
}
