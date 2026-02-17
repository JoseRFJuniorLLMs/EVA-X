// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// ============================================================================
// Package eva_memory — Memoria Meta-Cognitiva da EVA
// ============================================================================
// A EVA sabe o que sabe. Ela lembra conversas passadas, reconhece padroes,
// sabe quais topicos discutiu e com que frequencia, e injeta esse
// auto-conhecimento no seu contexto de sistema.
//
// Grafo Neo4j:
//   (:EvaSession)-[:HAS_TURN]->(:EvaTurn)
//   (:EvaTurn)-[:ABOUT]->(:EvaTopic)
//   (:EvaSession)-[:DISCUSSED]->(:EvaTopic)
//   (:EvaTopic)-[:RELATED_TO]->(:EvaTopic)
//   (:EvaInsight)-[:ABOUT]->(:EvaTopic)
//
// Usado por: geminiWeb (eva_handler.go → /ws/eva)

package eva_memory

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// EvaMemory gerencia a memoria meta-cognitiva da EVA via Neo4j
type EvaMemory struct {
	neo4j *graph.Neo4jClient
}

// New cria uma nova instancia de EvaMemory
func New(neo4jClient *graph.Neo4jClient) *EvaMemory {
	return &EvaMemory{neo4j: neo4jClient}
}

// InitSchema cria constraints e indexes no Neo4j para o grafo da EVA
func (em *EvaMemory) InitSchema(ctx context.Context) error {
	queries := []string{
		// Constraints de unicidade
		`CREATE CONSTRAINT eva_session_id IF NOT EXISTS FOR (s:EvaSession) REQUIRE s.id IS UNIQUE`,
		`CREATE CONSTRAINT eva_turn_id IF NOT EXISTS FOR (t:EvaTurn) REQUIRE t.id IS UNIQUE`,
		`CREATE CONSTRAINT eva_topic_name IF NOT EXISTS FOR (t:EvaTopic) REQUIRE t.name IS UNIQUE`,
		`CREATE CONSTRAINT eva_insight_id IF NOT EXISTS FOR (i:EvaInsight) REQUIRE i.id IS UNIQUE`,

		// Indexes para buscas rapidas
		`CREATE INDEX eva_session_started IF NOT EXISTS FOR (s:EvaSession) ON (s.started_at)`,
		`CREATE INDEX eva_turn_timestamp IF NOT EXISTS FOR (t:EvaTurn) ON (t.timestamp)`,
		`CREATE INDEX eva_topic_frequency IF NOT EXISTS FOR (t:EvaTopic) ON (t.frequency)`,
	}

	for _, q := range queries {
		if _, err := em.neo4j.ExecuteWrite(ctx, q, nil); err != nil {
			// Constraints podem ja existir, nao falhar
			log.Warn().Err(err).Str("query", q[:60]).Msg("[EVA-MEMORY] Schema query warning")
		}
	}

	log.Info().Msg("[EVA-MEMORY] Neo4j schema inicializado")
	return nil
}

// StartSession registra uma nova sessao de conversa no grafo
func (em *EvaMemory) StartSession(ctx context.Context, sessionID string) error {
	query := `
		CREATE (s:EvaSession {
			id: $id,
			started_at: datetime($started),
			turn_count: 0,
			status: 'active'
		})
	`
	params := map[string]interface{}{
		"id":      sessionID,
		"started": time.Now().Format(time.RFC3339),
	}

	_, err := em.neo4j.ExecuteWrite(ctx, query, params)
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("[EVA-MEMORY] Falha ao criar sessao")
		return err
	}

	log.Info().Str("session", sessionID).Msg("[EVA-MEMORY] Sessao iniciada")
	return nil
}

// EndSession finaliza uma sessao e gera resumo automatico
func (em *EvaMemory) EndSession(ctx context.Context, sessionID string) error {
	query := `
		MATCH (s:EvaSession {id: $id})
		SET s.ended_at = datetime($ended),
		    s.status = 'completed'
	`
	params := map[string]interface{}{
		"id":    sessionID,
		"ended": time.Now().Format(time.RFC3339),
	}

	_, err := em.neo4j.ExecuteWrite(ctx, query, params)
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("[EVA-MEMORY] Falha ao finalizar sessao")
	}
	return err
}

// StoreTurn salva um turno de conversa (user ou assistant) no grafo
// Extrai topicos automaticamente do conteudo e conecta ao grafo
func (em *EvaMemory) StoreTurn(ctx context.Context, sessionID, role, content string) error {
	turnID := fmt.Sprintf("%s-%s-%d", sessionID, role, time.Now().UnixNano())

	// 1. Criar no do turno e conectar a sessao
	query := `
		MATCH (s:EvaSession {id: $sessionId})
		CREATE (t:EvaTurn {
			id: $turnId,
			role: $role,
			content: $content,
			timestamp: datetime($ts)
		})
		CREATE (s)-[:HAS_TURN]->(t)
		SET s.turn_count = s.turn_count + 1
	`
	params := map[string]interface{}{
		"sessionId": sessionID,
		"turnId":    turnID,
		"role":      role,
		"content":   content,
		"ts":        time.Now().Format(time.RFC3339),
	}

	if _, err := em.neo4j.ExecuteWrite(ctx, query, params); err != nil {
		log.Error().Err(err).Msg("[EVA-MEMORY] Falha ao salvar turno")
		return err
	}

	// 2. Extrair topicos do conteudo e conectar
	topics := extractTopics(content)
	for _, topic := range topics {
		em.connectTopic(ctx, sessionID, turnID, topic)
	}

	return nil
}

// connectTopic conecta um turno a um topico, incrementando frequencia
func (em *EvaMemory) connectTopic(ctx context.Context, sessionID, turnID, topicName string) {
	query := `
		MATCH (t:EvaTurn {id: $turnId})
		MATCH (s:EvaSession {id: $sessionId})

		MERGE (topic:EvaTopic {name: $topicName})
		ON CREATE SET topic.frequency = 1,
		              topic.first_seen = datetime(),
		              topic.last_seen = datetime()
		ON MATCH SET  topic.frequency = topic.frequency + 1,
		              topic.last_seen = datetime()

		MERGE (t)-[:ABOUT]->(topic)
		MERGE (s)-[:DISCUSSED]->(topic)
	`
	params := map[string]interface{}{
		"turnId":    turnID,
		"sessionId": sessionID,
		"topicName": topicName,
	}

	if _, err := em.neo4j.ExecuteWrite(ctx, query, params); err != nil {
		log.Warn().Err(err).Str("topic", topicName).Msg("[EVA-MEMORY] Falha ao conectar topico")
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
	query := `
		MATCH (s:EvaSession)
		WHERE s.status = 'completed'
		WITH s ORDER BY s.started_at DESC LIMIT $limit

		OPTIONAL MATCH (s)-[:DISCUSSED]->(topic:EvaTopic)
		WITH s, collect(DISTINCT topic.name) AS topics

		OPTIONAL MATCH (s)-[:HAS_TURN]->(t:EvaTurn)
		WITH s, topics, count(t) AS turnCount
		ORDER BY s.started_at DESC

		RETURN s.started_at AS started,
		       s.turn_count AS turns,
		       topics,
		       turnCount
	`
	params := map[string]interface{}{"limit": limit}

	records, err := em.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, rec := range records {
		started, _ := rec.Get("started")
		topics, _ := rec.Get("topics")
		turns, _ := rec.Get("turns")

		topicList := "nenhum topico"
		if ts, ok := topics.([]interface{}); ok && len(ts) > 0 {
			strs := make([]string, len(ts))
			for i, t := range ts {
				strs[i] = fmt.Sprintf("%v", t)
			}
			topicList = strings.Join(strs, ", ")
		}

		results = append(results, fmt.Sprintf("- %v (%v turnos): %s", started, turns, topicList))
	}
	return results, nil
}

// getTopTopics retorna os topicos mais discutidos
func (em *EvaMemory) getTopTopics(ctx context.Context, limit int) ([]string, error) {
	query := `
		MATCH (t:EvaTopic)
		WHERE t.frequency > 0
		RETURN t.name AS name, t.frequency AS freq, t.last_seen AS lastSeen
		ORDER BY t.frequency DESC
		LIMIT $limit
	`
	params := map[string]interface{}{"limit": limit}

	records, err := em.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, rec := range records {
		name, _ := rec.Get("name")
		freq, _ := rec.Get("freq")
		results = append(results, fmt.Sprintf("- %v (%vx mencionado)", name, freq))
	}
	return results, nil
}

// getRecentTopics retorna topicos discutidos nos ultimos N dias
func (em *EvaMemory) getRecentTopics(ctx context.Context, days int) ([]string, error) {
	query := `
		MATCH (t:EvaTopic)
		WHERE t.last_seen > datetime() - duration({days: $days})
		RETURN t.name AS name
		ORDER BY t.last_seen DESC
	`
	params := map[string]interface{}{"days": days}

	records, err := em.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, rec := range records {
		name, _ := rec.Get("name")
		results = append(results, fmt.Sprintf("%v", name))
	}
	return results, nil
}

// getInsights retorna insights meta-cognitivos da EVA
func (em *EvaMemory) getInsights(ctx context.Context, limit int) ([]string, error) {
	query := `
		MATCH (i:EvaInsight)
		RETURN i.content AS content, i.type AS type
		ORDER BY i.created_at DESC
		LIMIT $limit
	`
	params := map[string]interface{}{"limit": limit}

	records, err := em.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, rec := range records {
		content, _ := rec.Get("content")
		itype, _ := rec.Get("type")
		results = append(results, fmt.Sprintf("- [%v] %v", itype, content))
	}
	return results, nil
}

// GenerateInsight cria um insight meta-cognitivo baseado em padroes detectados
func (em *EvaMemory) GenerateInsight(ctx context.Context, content, insightType string, topicName string) error {
	insightID := fmt.Sprintf("insight-%d", time.Now().UnixNano())

	query := `
		CREATE (i:EvaInsight {
			id: $id,
			content: $content,
			type: $type,
			created_at: datetime()
		})
	`
	params := map[string]interface{}{
		"id":      insightID,
		"content": content,
		"type":    insightType,
	}

	if _, err := em.neo4j.ExecuteWrite(ctx, query, params); err != nil {
		return err
	}

	// Conectar ao topico se fornecido
	if topicName != "" {
		linkQuery := `
			MATCH (i:EvaInsight {id: $insightId})
			MERGE (t:EvaTopic {name: $topicName})
			MERGE (i)-[:ABOUT]->(t)
		`
		linkParams := map[string]interface{}{
			"insightId": insightID,
			"topicName": topicName,
		}
		em.neo4j.ExecuteWrite(ctx, linkQuery, linkParams)
	}

	log.Info().Str("type", insightType).Str("topic", topicName).Msg("[EVA-MEMORY] Insight gerado")
	return nil
}

// DetectPatterns analisa o grafo e gera insights automaticos
func (em *EvaMemory) DetectPatterns(ctx context.Context) error {
	// Detectar topicos muito frequentes (>10 mencoes) sem insight
	query := `
		MATCH (t:EvaTopic)
		WHERE t.frequency >= 10
		AND NOT EXISTS { MATCH (:EvaInsight)-[:ABOUT]->(t) }
		RETURN t.name AS name, t.frequency AS freq
	`

	records, err := em.neo4j.ExecuteRead(ctx, query, nil)
	if err != nil {
		return err
	}

	for _, rec := range records {
		name, _ := rec.Get("name")
		freq, _ := rec.Get("freq")
		insight := fmt.Sprintf("O topico '%v' foi discutido %v vezes. Tenho bastante experiencia neste assunto.", name, freq)
		em.GenerateInsight(ctx, insight, "pattern", fmt.Sprintf("%v", name))
	}

	return nil
}

// GetSessionHistory retorna o historico de turnos de uma sessao especifica
func (em *EvaMemory) GetSessionHistory(ctx context.Context, sessionID string) ([]map[string]string, error) {
	query := `
		MATCH (s:EvaSession {id: $sessionId})-[:HAS_TURN]->(t:EvaTurn)
		RETURN t.role AS role, t.content AS content, t.timestamp AS ts
		ORDER BY t.timestamp ASC
	`
	params := map[string]interface{}{"sessionId": sessionID}

	records, err := em.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, err
	}

	var history []map[string]string
	for _, rec := range records {
		role, _ := rec.Get("role")
		content, _ := rec.Get("content")
		history = append(history, map[string]string{
			"role":    fmt.Sprintf("%v", role),
			"content": fmt.Sprintf("%v", content),
		})
	}
	return history, nil
}

// extractTopics extrai topicos de malaria do conteudo usando keywords
// Simples e rapido — nao depende de LLM
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
