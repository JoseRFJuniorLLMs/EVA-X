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

	"github.com/rs/zerolog/log"
)

// EvaMemory gerencia a memoria meta-cognitiva da EVA via NietzscheDB
type EvaMemory struct {
	graph *nietzscheInfra.GraphAdapter
}

// New cria uma nova instancia de EvaMemory
func New(graphAdapter *nietzscheInfra.GraphAdapter) *EvaMemory {
	return &EvaMemory{graph: graphAdapter}
}

// InitSchema creates initial nodes/structure in NietzscheDB for the EVA graph.
// NietzscheDB does not need explicit constraints/indexes like Neo4j --
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
		NodeType: "EvaSession",
		MatchKeys: map[string]interface{}{
			"id": sessionID,
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

// EndSession finaliza uma sessao e gera resumo automatico
func (em *EvaMemory) EndSession(ctx context.Context, sessionID string) error {
	now := time.Now()

	_, err := em.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "EvaSession",
		MatchKeys: map[string]interface{}{
			"id": sessionID,
		},
		OnMatchSet: map[string]interface{}{
			"ended_at": now.Format(time.RFC3339),
			"status":   "completed",
		},
	})
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("[EVA-MEMORY] Falha ao finalizar sessao")
	}
	return err
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
		Content: map[string]interface{}{
			"id":        turnID,
			"role":      role,
			"content":   content,
			"timestamp": now.Format(time.RFC3339),
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
		NodeType: "EvaSession",
		MatchKeys: map[string]interface{}{
			"id": sessionID,
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
		NodeType: "EvaTopic",
		MatchKeys: map[string]interface{}{
			"name": topicName,
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
func (em *EvaMemory) getRecentSessions(ctx context.Context, limit int) ([]string, error) {
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

		// Get topics for this session via BFS
		topicList := "nenhum topico"
		if sessionID != "" {
			topicIDs, err := em.graph.BfsWithEdgeType(ctx, node.ID, "DISCUSSED", 1, "")
			if err == nil && len(topicIDs) > 0 {
				var topicNames []string
				for _, tid := range topicIDs {
					tNode, err := em.graph.GetNode(ctx, tid, "")
					if err == nil {
						if name, ok := tNode.Content["name"].(string); ok {
							topicNames = append(topicNames, name)
						}
					}
				}
				if len(topicNames) > 0 {
					topicList = strings.Join(topicNames, ", ")
				}
			}
		}

		results = append(results, fmt.Sprintf("- %v (%v turnos): %s", started, turnCount, topicList))
	}
	return results, nil
}

// getTopTopics retorna os topicos mais discutidos
func (em *EvaMemory) getTopTopics(ctx context.Context, limit int) ([]string, error) {
	nql := `MATCH (t:EvaTopic) WHERE t.frequency > 0 RETURN t ORDER BY t.frequency DESC LIMIT $limit`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"limit": limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, node := range result.Nodes {
		name := node.Content["name"]
		freq := node.Content["frequency"]
		results = append(results, fmt.Sprintf("- %v (%vx mencionado)", name, freq))
	}
	return results, nil
}

// getRecentTopics retorna topicos discutidos nos ultimos N dias
func (em *EvaMemory) getRecentTopics(ctx context.Context, days int) ([]string, error) {
	cutoff := nietzscheInfra.DaysAgoUnix(days)
	nql := `MATCH (t:EvaTopic) WHERE t.last_seen > $cutoff RETURN t ORDER BY t.last_seen DESC`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"cutoff": cutoff,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, node := range result.Nodes {
		if name, ok := node.Content["name"].(string); ok {
			results = append(results, name)
		}
	}
	return results, nil
}

// getInsights retorna insights meta-cognitivos da EVA
func (em *EvaMemory) getInsights(ctx context.Context, limit int) ([]string, error) {
	nql := `MATCH (i:EvaInsight) RETURN i ORDER BY i.created_at DESC LIMIT $limit`
	result, err := em.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"limit": limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var results []string
	for _, node := range result.Nodes {
		content := node.Content["content"]
		itype := node.Content["type"]
		results = append(results, fmt.Sprintf("- [%v] %v", itype, content))
	}
	return results, nil
}

// GenerateInsight cria um insight meta-cognitivo baseado em padroes detectados
func (em *EvaMemory) GenerateInsight(ctx context.Context, content, insightType string, topicName string) error {
	insightID := fmt.Sprintf("insight-%d", time.Now().UnixNano())
	now := nietzscheInfra.NowUnix()

	// Create insight node
	insightResult, err := em.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Content: map[string]interface{}{
			"id":         insightID,
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
			NodeType: "EvaTopic",
			MatchKeys: map[string]interface{}{
				"name": topicName,
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

// DetectPatterns analisa o grafo e gera insights automaticos
func (em *EvaMemory) DetectPatterns(ctx context.Context) error {
	// Find topics with frequency >= 10 that don't have insights
	nql := `MATCH (t:EvaTopic) WHERE t.frequency >= 10 RETURN t`
	result, err := em.graph.ExecuteNQL(ctx, nql, nil, "")
	if err != nil {
		return err
	}

	for _, node := range result.Nodes {
		name, _ := node.Content["name"].(string)
		freq := node.Content["frequency"]

		// Check if this topic already has an insight connected
		insightIDs, err := em.graph.BfsWithEdgeType(ctx, node.ID, "ABOUT", 1, "")
		if err == nil && len(insightIDs) > 0 {
			// Check if any of them are EvaInsight nodes
			hasInsight := false
			for _, iid := range insightIDs {
				iNode, err := em.graph.GetNode(ctx, iid, "")
				if err == nil && iNode.NodeType == "EvaInsight" {
					hasInsight = true
					break
				}
			}
			if hasInsight {
				continue
			}
		}

		insight := fmt.Sprintf("O topico '%v' foi discutido %v vezes. Tenho bastante experiencia neste assunto.", name, freq)
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
		"diagnostico":    {"diagnostico", "diagnóstico", "gota espessa", "esfregaco", "teste rapido", "tdr", "microscopia", "lamina"},
		"tratamento":     {"tratamento", "artemeter", "lumefantrina", "artesunato", "act", "antimalarico", "medicamento", "dose", "dosagem", "prescricao"},
		"malaria_grave":  {"malaria grave", "malaria cerebral", "complicada", "emergencia", "artesunato ev", "parenteral", "internacao"},
		"epidemiologia":  {"epidemiologia", "epidemiologica", "endemica", "prevalencia", "incidencia", "caso", "surto", "transmissao"},
		"especies":       {"falciparum", "vivax", "malariae", "ovale", "knowlesi", "plasmodium", "especie"},
		"prevencao":      {"prevencao", "prevenção", "mosquiteiro", "remti", "pidom", "repelente", "profilaxia", "quimioprofilaxia"},
		"gravidez":       {"gravidez", "gravida", "gestante", "gestacao", "prenatal", "tip"},
		"pediatria":      {"crianca", "pediatria", "infantil", "neonatal", "recém-nascido", "neonato"},
		"laboratorio":    {"laboratorio", "microscopia", "coloracao", "giemsa", "parasitemia", "hematocrito"},
		"vetor":          {"anopheles", "mosquito", "vetor", "gambiae", "funestus", "larva"},
		"angola":         {"angola", "angolano", "luanda", "provincia", "municipio"},
		"sistema":        {"sistema", "dashboard", "plataforma", "deteccao", "yolo", "ia", "inteligencia artificial"},
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
