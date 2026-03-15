// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// ============================================================================
// Package eva_memory -- Memoria Meta-Cognitiva da EVA
// ============================================================================
// A EVA sabe o que sabe. Ela lembra conversas passadas, reconhece padroes,
// sabe quais topicos discutiu e com que frequencia, e injeta esse
// auto-conhecimento no seu contexto de sistema.
//
// Grafo NietzscheDB:
//   (:EvaSession)-[:HAS_TURN]->(:EvaTurn)
//   (:EvaTurn)-[:ABOUT]->(:EvaTopic)
//   (:EvaSession)-[:DISCUSSED]->(:EvaTopic)
//   (:EvaTopic)-[:RELATED_TO]->(:EvaTopic)
//   (:EvaInsight)-[:ABOUT]->(:EvaTopic)
//
// Usado por: geminiWeb (eva_handler.go -> /ws/eva)

package eva_memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// EvaMemory gerencia a memoria meta-cognitiva da EVA via NietzscheDB
type EvaMemory struct {
	graph     *nietzscheInfra.GraphAdapter
	mindGraph *nietzscheInfra.GraphAdapter // eva_mind collection for internalized memories
	embedFunc func(ctx context.Context, text string) ([]float32, error)
}

// New cria uma nova instancia de EvaMemory
func New(graphAdapter *nietzscheInfra.GraphAdapter) *EvaMemory {
	return &EvaMemory{graph: graphAdapter}
}

// SetMindAdapter injects the eva_mind graph adapter for InternalizeMemory.
func (em *EvaMemory) SetMindAdapter(adapter *nietzscheInfra.GraphAdapter) {
	em.mindGraph = adapter
}

// SetEmbedFunc injects the vector generation dependency for real embeddings
func (em *EvaMemory) SetEmbedFunc(embedFunc func(ctx context.Context, text string) ([]float32, error)) {
	em.embedFunc = embedFunc
}

// minInternalizeLen is the minimum content length for InternalizeMemory.
// Messages shorter than this (e.g. "ok", "yes", "sim") are skipped.
const minInternalizeLen = 10

// InternalizeMemory stores a conversation memory as an Episodic node in eva_mind.
// It creates a node with the given content, valence, and source. Energy defaults
// to 0.5 for new memories. If embedFunc is present, it generates a real 3072D vector.
// Otherwise, coords are left zero (or handled by hash fallback in the infra layer).
// Trivial messages (< 10 chars) are silently skipped.
func (em *EvaMemory) InternalizeMemory(content string, valence float64, source string) error {
	// Skip trivial messages
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < minInternalizeLen {
		return nil
	}

	adapter := em.mindGraph
	if adapter == nil {
		// Fallback to default graph adapter if mindGraph not set
		adapter = em.graph
	}
	if adapter == nil {
		return fmt.Errorf("no graph adapter available for InternalizeMemory")
	}

	nodeID := uuid.New().String()
	now := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 3072-dimensional zero coords for relational data in poincare space (fallback)
	coords := make([]float64, 3072)

	// FASE 1 P1-E FIX: Generate actual contextual embeddings for eva_mind
	if em.embedFunc != nil {
		// Retry up to 2 times with backoff on transient failures
		var vec32 []float32
		var embedErr error
		for attempt := 0; attempt < 3; attempt++ {
			vec32, embedErr = em.embedFunc(ctx, trimmed)
			if embedErr == nil && len(vec32) > 0 {
				break
			}
			if attempt < 2 {
				time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
			}
		}
		if embedErr == nil && len(vec32) > 0 {
			maxDim := 3072
			if len(vec32) < maxDim {
				maxDim = len(vec32)
			}
			for i := 0; i < maxDim; i++ {
				coords[i] = float64(vec32[i])
			}
			log.Info().Str("node_id", nodeID).Int("dim", len(vec32)).Msg("[EVA-MEMORY] Real embedding generated for eva_mind")
		} else {
			log.Warn().Err(embedErr).Str("node_id", nodeID).Int("content_len", len(trimmed)).
				Msg("[EVA-MEMORY] Embedding generation failed after 3 attempts — node stored with zero coords (KNN will not find it)")
		}
	} else {
		log.Warn().Msg("[EVA-MEMORY] embedFunc is nil — InternalizeMemory storing zero-vector node (KNN disabled)")
	}

	_, err := adapter.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:       nodeID,
		NodeType: "Episodic",
		Energy:   0.5,
		Coords:   coords,
		Content: map[string]interface{}{
			"content":    trimmed,
			"valence":    valence,
			"source":     source,
			"created_at": now.Format(time.RFC3339),
			"node_label": "ConversationMemory",
		},
	})
	if err != nil {
		log.Error().Err(err).
			Str("node_id", nodeID).
			Str("source", source).
			Int("content_len", len(trimmed)).
			Msg("[EVA-MEMORY] InternalizeMemory failed")
		return fmt.Errorf("internalize memory: %w", err)
	}

	log.Debug().
		Str("node_id", nodeID).
		Str("source", source).
		Float64("valence", valence).
		Int("content_len", len(trimmed)).
		Msg("[EVA-MEMORY] Memory internalized to eva_mind")
	return nil
}

// InitSchema creates initial nodes/structure in NietzscheDB for the EVA graph.
// NietzscheDB does not need explicit constraints/indexes --
// collections are created at startup via EnsureCollections.
// This method is kept for API compatibility but is now a no-op.
func (em *EvaMemory) InitSchema(ctx context.Context) error {
	log.Info().Msg("[EVA-MEMORY] NietzscheDB schema ready (no explicit constraints needed)")
	return nil
}

// StartSession registra uma nova sessao de conversa no grafo
func (em *EvaMemory) StartSession(ctx context.Context, sessionID string) error {
	now := time.Now()

	_, err := em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Semantic",
		MatchKeys: map[string]interface{}{
			"id":         sessionID,
			"node_label": "EvaSession",
		},
		OnCreateSet: map[string]interface{}{
			"started_at": now.Format(time.RFC3339),
			"turn_count": 0,
			"status":     "active",
		},
	})
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("[EVA-MEMORY] Falha ao criar sessao")
		return err
	}

	log.Info().Str("session", sessionID).Msg("[EVA-MEMORY] Sessao iniciada")
	return nil
}

// EndSession finaliza uma sessao e gera resumo automatico.
// Sessions with 0 turns and duration < 5s are considered spam and deleted.
func (em *EvaMemory) EndSession(ctx context.Context, sessionID string) error {
	now := time.Now()

	// Query the session to check turn_count and duration
	nql := `MATCH (s:EvaSession) WHERE s.id = $sessionId RETURN s`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"sessionId": sessionID,
	}, "")
	if err != nil || len(result.Nodes) == 0 {
		log.Warn().Str("session", sessionID).Msg("[EVA-MEMORY] Sessao nao encontrada ao finalizar")
		return err
	}

	node := result.Nodes[0]
	turnCount := 0
	if tc, ok := node.Content["turn_count"].(float64); ok {
		turnCount = int(tc)
	}

	// Check session duration
	durationOK := false
	if startedStr, ok := node.Content["started_at"].(string); ok {
		if startedAt, err := time.Parse(time.RFC3339, startedStr); err == nil {
			durationOK = now.Sub(startedAt) >= 5*time.Second
		}
	}

	// Skip persisting empty sessions: 0 turns AND duration < 5s
	if turnCount == 0 && !durationOK {
		log.Info().Str("session", sessionID).Msg("[EVA-MEMORY] Sessao vazia (0 turnos, <5s) — removendo spam")
		if err := em.graph.DeleteNode(ctx, node.ID, ""); err != nil {
			log.Warn().Err(err).Str("node_id", node.ID).Msg("[EVA-MEMORY] Failed to delete empty session node")
		}
		return nil
	}

	_, err = em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Semantic",
		MatchKeys: map[string]interface{}{
			"id":         sessionID,
			"node_label": "EvaSession",
		},
		OnMatchSet: map[string]interface{}{
			"ended_at": now.Format(time.RFC3339),
			"status":   "completed",
		},
	})
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("[EVA-MEMORY] Falha ao finalizar sessao")
	}

	// P1-C FIX: Prevent infinite accumulation of EvaSession and EvaTurn nodes
	// by pruning sessions older than the top 20 most recent ones asynchronously.
	go func() {
		pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer pruneCancel()
		em.pruneOldSessions(pruneCtx, 20)
	}()

	return err
}

// pruneOldSessions maintains only the N most recent completed sessions, deleting older ones
// and their associated EvaTurn nodes to prevent infinite graph database accumulation (P1-C).
func (em *EvaMemory) pruneOldSessions(ctx context.Context, keep int) {
	// NQL SKIP is used to fetch all sessions strictly older than our 'keep' threshold.
	nql := fmt.Sprintf(`MATCH (s:EvaSession) WHERE s.status = "completed" RETURN s ORDER BY s.started_at DESC SKIP %d`, keep)
	result, err := em.graph.ExecuteNQL(ctx, nql, nil, "")
	if err != nil || len(result.Nodes) == 0 {
		return
	}

	pruned := 0
	for _, node := range result.Nodes {
		// Find and delete associated turns using BFS
		turnIDs, err := em.graph.BfsWithEdgeType(ctx, node.ID, "HAS_TURN", 1, "")
		if err == nil {
			for _, tid := range turnIDs {
				if err := em.graph.DeleteNode(ctx, tid, ""); err != nil {
					log.Warn().Err(err).Str("turn_id", tid).Msg("[EVA-MEMORY] Failed to delete turn node during prune")
				}
			}
		}
		// Delete the session node itself
		if err := em.graph.DeleteNode(ctx, node.ID, ""); err != nil {
			log.Warn().Err(err).Str("node_id", node.ID).Msg("[EVA-MEMORY] Failed to delete session node during prune")
		}
		pruned++
	}

	if pruned > 0 {
		log.Info().Int("pruned_sessions", pruned).Msg("[EVA-MEMORY] Cleaned up old session nodes to prevent infinite accumulation")
	}
}

// StoreTurn salva um turno de conversa (user ou assistant) no grafo
// Extrai topicos automaticamente do conteudo e conecta ao grafo
func (em *EvaMemory) StoreTurn(ctx context.Context, sessionID, role, content string) error {
	turnID := fmt.Sprintf("%s-%s-%d", sessionID, role, time.Now().UnixNano())
	now := time.Now()

	// 1. Find the session node
	nql := `MATCH (s:EvaSession) WHERE s.id = $sessionId RETURN s`
	sessionResult, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"sessionId": sessionID,
	}, "")
	if err != nil || len(sessionResult.Nodes) == 0 {
		log.Error().Err(err).Msg("[EVA-MEMORY] Sessao nao encontrada para salvar turno")
		return fmt.Errorf("session %s not found", sessionID)
	}
	sessionNodeID := sessionResult.Nodes[0].ID

	// 2. Create the turn node
	turnResult, err := em.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType: "Semantic",
		Content: map[string]interface{}{
			"id":         turnID,
			"node_label": "EvaTurn",
			"role":       role,
			"content":    content,
			"timestamp":  now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("[EVA-MEMORY] Falha ao salvar turno")
		return err
	}

	// 3. Create edge: Session -HAS_TURN-> Turn
	_, err = em.graph.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:     sessionNodeID,
		To:       turnResult.ID,
		EdgeType: "HAS_TURN",
	})
	if err != nil {
		log.Error().Err(err).Msg("[EVA-MEMORY] Falha ao conectar turno a sessao")
		return err
	}

	// 4. Update session turn_count
	currentCount := 0
	if tc, ok := sessionResult.Nodes[0].Content["turn_count"].(float64); ok {
		currentCount = int(tc)
	}
	_, err = em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Semantic",
		MatchKeys: map[string]interface{}{
			"id":         sessionID,
			"node_label": "EvaSession",
		},
		OnMatchSet: map[string]interface{}{
			"turn_count": currentCount + 1,
		},
	})
	if err != nil {
		log.Warn().Err(err).Msg("[EVA-MEMORY] Falha ao atualizar turn_count")
	}

	// 5. Extract topics and connect
	topics := extractTopics(content)
	for _, topic := range topics {
		em.connectTopic(ctx, sessionNodeID, turnResult.ID, topic)
	}

	return nil
}

// connectTopic conecta um turno a um topico, incrementando frequencia
func (em *EvaMemory) connectTopic(ctx context.Context, sessionNodeID, turnNodeID, topicName string) {
	now := nietzscheInfra.NowUnix()

	// MERGE topic node
	topicResult, err := em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Semantic",
		MatchKeys: map[string]interface{}{
			"name":       topicName,
			"node_label": "EvaTopic",
		},
		OnCreateSet: map[string]interface{}{
			"frequency":  1,
			"first_seen": now,
			"last_seen":  now,
		},
		OnMatchSet: map[string]interface{}{
			"frequency": "INCREMENT",
			"last_seen": now,
		},
	})
	if err != nil {
		log.Warn().Err(err).Str("topic", topicName).Msg("[EVA-MEMORY] Falha ao merge topico")
		return
	}

	// Turn -ABOUT-> Topic
	_, err = em.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: turnNodeID,
		ToNodeID:   topicResult.NodeID,
		EdgeType:   "ABOUT",
	})
	if err != nil {
		log.Warn().Err(err).Str("topic", topicName).Msg("[EVA-MEMORY] Falha ao conectar turno ao topico")
	}

	// Session -DISCUSSED-> Topic
	_, err = em.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: sessionNodeID,
		ToNodeID:   topicResult.NodeID,
		EdgeType:   "DISCUSSED",
	})
	if err != nil {
		log.Warn().Err(err).Str("topic", topicName).Msg("[EVA-MEMORY] Falha ao conectar sessao ao topico")
	}
}

// LoadMetaCognition carrega o estado meta-cognitivo da EVA para injecao no system prompt
// Retorna uma string formatada com: ultimas conversas, topicos frequentes, insights
func (em *EvaMemory) LoadMetaCognition(ctx context.Context) (string, error) {
	var sections []string

	// 1. Ultimas conversas (resumos das 5 sessoes mais recentes)
	recentSessions, err := em.getRecentSessions(ctx, 5)
	if err == nil && len(recentSessions) > 0 {
		sections = append(sections, "=== CONVERSAS RECENTES ===")
		for _, s := range recentSessions {
			sections = append(sections, s)
		}
	}

	// 2. Topicos mais discutidos (top 10)
	topTopics, err := em.getTopTopics(ctx, 10)
	if err == nil && len(topTopics) > 0 {
		sections = append(sections, "\n=== TOPICOS QUE DOMINO (por frequencia) ===")
		for _, t := range topTopics {
			sections = append(sections, t)
		}
	}

	// 3. Topicos recentes (ultimos 7 dias)
	recentTopics, err := em.getRecentTopics(ctx, 7)
	if err == nil && len(recentTopics) > 0 {
		sections = append(sections, "\n=== TOPICOS RECENTES (ultimos 7 dias) ===")
		sections = append(sections, strings.Join(recentTopics, ", "))
	}

	// 4. Insights meta-cognitivos
	insights, err := em.getInsights(ctx, 5)
	if err == nil && len(insights) > 0 {
		sections = append(sections, "\n=== MEUS INSIGHTS ===")
		for _, i := range insights {
			sections = append(sections, i)
		}
	}

	if len(sections) == 0 {
		return "", nil // Sem memorias ainda
	}

	header := "\n\n=== MEMORIA META-COGNITIVA DA EVA ===\n"
	header += "Eu lembro das conversas anteriores. Uso esse conhecimento para dar respostas mais contextualizadas.\n"
	return header + strings.Join(sections, "\n"), nil
}

// getRecentSessions retorna resumos das N sessoes mais recentes

// humanTimeStr formats a RFC3339 string as human-readable relative time.
func humanTimeStr(rfc3339 interface{}) string {
	s, ok := rfc3339.(string)
	if !ok || s == "" {
		return "?"
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -6)

	hhmm := t.Format("15:04")

	if t.After(today) || t.Equal(today) {
		return "hoje " + hhmm
	}
	if t.After(yesterday) || t.Equal(yesterday) {
		return "ontem " + hhmm
	}
	if t.After(weekAgo) {
		dias := []string{"domingo", "segunda", "terca", "quarta", "quinta", "sexta", "sabado"}
		return dias[t.Weekday()] + " " + hhmm
	}
	meses := []string{"", "jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
	return fmt.Sprintf("%d/%s %s", t.Day(), meses[t.Month()], hhmm)
}

func (em *EvaMemory) getRecentSessions(ctx context.Context, limit int) ([]string, error) {
	// Optimized: Try to get session and topics in fewer queries if possible,
	// or at least simplify the BFS logic.
	nql := `MATCH (s:EvaSession) WHERE s.status = "completed" RETURN s ORDER BY s.started_at DESC LIMIT $limit`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"limit": limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, node := range result.Nodes {
		started := node.Content["started_at"]
		turnCount := node.Content["turn_count"]
		sessionID, _ := node.Content["id"].(string)

		// Get topics for this session via a single NQL join-like query
		topicList := "nenhum topico"
		if sessionID != "" {
			topicNql := `MATCH (s:EvaSession)-[:DISCUSSED]->(t:EvaTopic) WHERE s.id = $sid RETURN t.name as name`
			topicRes, err := em.graph.ExecuteNQL(ctx, topicNql, map[string]interface{}{"sid": sessionID}, "")
			if err == nil && len(topicRes.ScalarRows) > 0 {
				var names []string
				for _, row := range topicRes.ScalarRows {
					names = append(names, fmt.Sprintf("%v", row["name"]))
				}
				topicList = strings.Join(names, ", ")
			}
		}

		results = append(results, fmt.Sprintf("- %s (%v turnos): %s", humanTimeStr(started), turnCount, topicList))
	}
	return results, nil
}

// getTopTopics retorna os topicos mais discutidos
func (em *EvaMemory) getTopTopics(ctx context.Context, limit int) ([]string, error) {
	// NQL already supports ORDER BY and LIMIT
	nql := `MATCH (t:EvaTopic) WHERE t.frequency > 0 RETURN t.name as name, t.frequency as freq ORDER BY t.frequency DESC LIMIT $limit`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"limit": limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, row := range result.ScalarRows {
		results = append(results, fmt.Sprintf("- %v (%vx mencionado)", row["name"], row["freq"]))
	}
	return results, nil
}

// getRecentTopics retorna topicos discutidos nos ultimos N dias
func (em *EvaMemory) getRecentTopics(ctx context.Context, days int) ([]string, error) {
	cutoff := nietzscheInfra.DaysAgoUnix(days)
	nql := `MATCH (t:EvaTopic) WHERE t.last_seen > $cutoff RETURN t.name as name ORDER BY t.last_seen DESC`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"cutoff": cutoff,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, row := range result.ScalarRows {
		results = append(results, fmt.Sprintf("%v", row["name"]))
	}
	return results, nil
}

// getInsights retorna insights meta-cognitivos da EVA
func (em *EvaMemory) getInsights(ctx context.Context, limit int) ([]string, error) {
	nql := `MATCH (i:EvaInsight) RETURN i.content as content, i.type as type ORDER BY i.created_at DESC LIMIT $limit`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"limit": limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, row := range result.ScalarRows {
		results = append(results, fmt.Sprintf("- [%v] %v", row["type"], row["content"]))
	}
	return results, nil
}

// GenerateInsight cria um insight meta-cognitivo baseado em padroes detectados
func (em *EvaMemory) GenerateInsight(ctx context.Context, content, insightType string, topicName string) error {
	insightID := fmt.Sprintf("insight-%d", time.Now().UnixNano())
	now := nietzscheInfra.NowUnix()

	// Create insight node
	insightResult, err := em.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType: "Semantic",
		Content: map[string]interface{}{
			"id":         insightID,
			"node_label": "EvaInsight",
			"content":    content,
			"type":       insightType,
			"created_at": now,
		},
	})
	if err != nil {
		return err
	}

	// Connect to topic if provided
	if topicName != "" {
		topicResult, err := em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType: "Semantic",
			MatchKeys: map[string]interface{}{
				"name":       topicName,
				"node_label": "EvaTopic",
			},
		})
		if err == nil {
			_, err = em.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: insightResult.ID,
				ToNodeID:   topicResult.NodeID,
				EdgeType:   "ABOUT",
			})
			if err != nil {
				log.Warn().Err(err).Str("topic", topicName).Msg("[EVA-MEMORY] Falha ao conectar insight ao topico")
			}
		}
	}

	log.Info().Str("type", insightType).Str("topic", topicName).Msg("[EVA-MEMORY] Insight gerado")
	return nil
}

// DetectPatterns analisa o grafo e gera insights automaticos.
// Optimized to find topics that NEED insights in ONE query using NQL's ability to filter.
func (em *EvaMemory) DetectPatterns(ctx context.Context) error {
	// NQL: Find topics with high frequency that are NOT connected to an EvaInsight via ABOUT
	// Note: NQL might not support full NOT EXISTS subqueries yet, so we use a left-join pattern or
	// just fetch topics and filter the ones without insights in Go, but much more efficiently.

	nql := `
		MATCH (t:EvaTopic) 
		WHERE t.frequency >= 10 
		RETURN t.name as name, t.frequency as freq, t.id as id
	`
	result, err := em.graph.ExecuteNQL(ctx, nql, nil, "")
	if err != nil {
		return err
	}

	for _, row := range result.ScalarRows {
		name := fmt.Sprintf("%v", row["name"])
		freq := row["freq"]
		topicNodeID := fmt.Sprintf("%v", row["id"])

		// Fast check for existing insight using BfsWithEdgeType (depth 1)
		insightIDs, err := em.graph.BfsWithEdgeType(ctx, topicNodeID, "ABOUT", 1, "")
		if err == nil && len(insightIDs) > 0 {
			// Topic already has an insight (or something) connected via ABOUT
			continue
		}

		insight := fmt.Sprintf("O topico '%s' foi discutido %v vezes. Tenho bastante experiencia neste assunto.", name, freq)
		em.GenerateInsight(ctx, insight, "pattern", name)
	}

	return nil
}

// GetSessionHistory retorna o historico de turnos de uma sessao especifica
func (em *EvaMemory) GetSessionHistory(ctx context.Context, sessionID string) ([]map[string]string, error) {
	// Find session node
	nql := `MATCH (s:EvaSession) WHERE s.id = $sessionId RETURN s`
	sessionResult, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"sessionId": sessionID,
	}, "")
	if err != nil || len(sessionResult.Nodes) == 0 {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	sessionNodeID := sessionResult.Nodes[0].ID

	// BFS through HAS_TURN edges to find turns
	turnIDs, err := em.graph.BfsWithEdgeType(ctx, sessionNodeID, "HAS_TURN", 1, "")
	if err != nil {
		return nil, err
	}

	type turnWithTimestamp struct {
		data map[string]string
		ts   string
	}
	var turns []turnWithTimestamp

	for _, tid := range turnIDs {
		node, err := em.graph.GetNode(ctx, tid, "")
		if err != nil {
			continue
		}

		role := fmt.Sprintf("%v", node.Content["role"])
		content := fmt.Sprintf("%v", node.Content["content"])
		ts := ""
		if t, ok := node.Content["timestamp"].(string); ok {
			ts = t
		}

		turns = append(turns, turnWithTimestamp{
			data: map[string]string{
				"role":    role,
				"content": content,
			},
			ts: ts,
		})
	}

	// Sort by timestamp ASC
	for i := 0; i < len(turns); i++ {
		for j := i + 1; j < len(turns); j++ {
			if turns[i].ts > turns[j].ts {
				turns[i], turns[j] = turns[j], turns[i]
			}
		}
	}

	var history []map[string]string
	for _, t := range turns {
		history = append(history, t.data)
	}
	return history, nil
}

// extractTopics extrai topicos do conteudo usando keywords
// Simples e rapido -- nao depende de LLM
func extractTopics(content string) []string {
	lower := strings.ToLower(content)
	var topics []string
	seen := make(map[string]bool)

	topicKeywords := map[string][]string{
		"diagnostico":   {"diagnostico", "diagnóstico", "gota espessa", "esfregaco", "teste rapido", "tdr", "microscopia", "lamina"},
		"tratamento":    {"tratamento", "artemeter", "lumefantrina", "artesunato", "act", "antimalarico", "medicamento", "dose", "dosagem", "prescricao"},
		"malaria_grave": {"malaria grave", "malaria cerebral", "complicada", "emergencia", "artesunato ev", "parenteral", "internacao"},
		"epidemiologia": {"epidemiologia", "epidemiologica", "endemica", "prevalencia", "incidencia", "caso", "surto", "transmissao"},
		"especies":      {"falciparum", "vivax", "malariae", "ovale", "knowlesi", "plasmodium", "especie"},
		"prevencao":     {"prevencao", "prevenção", "mosquiteiro", "remti", "pidom", "repelente", "profilaxia", "quimioprofilaxia"},
		"gravidez":      {"gravidez", "gravida", "gestante", "gestacao", "prenatal", "tip"},
		"pediatria":     {"crianca", "pediatria", "infantil", "neonatal", "recém-nascido", "neonato"},
		"laboratorio":   {"laboratorio", "microscopia", "coloracao", "giemsa", "parasitemia", "hematocrito"},
		"vetor":         {"anopheles", "mosquito", "vetor", "gambiae", "funestus", "larva"},
		"angola":        {"angola", "angolano", "luanda", "provincia", "municipio"},
		"sistema":       {"sistema", "dashboard", "plataforma", "deteccao", "yolo", "ia", "inteligencia artificial"},
	}

	for topic, keywords := range topicKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) && !seen[topic] {
				topics = append(topics, topic)
				seen[topic] = true
				break
			}
		}
	}

	return topics
}
