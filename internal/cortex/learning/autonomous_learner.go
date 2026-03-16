// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/knowledge"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	nietzsche "nietzsche-sdk"
)

// LearningInsight representa um insight aprendido pela EVA
type LearningInsight struct {
	ID         string    `json:"id"`
	Topic      string    `json:"topic"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Source     string    `json:"source"`
	Tags       []string  `json:"tags"`
	Category   string    `json:"category"`
	Confidence float64   `json:"confidence"`
	LearnedAt  time.Time `json:"learned_at"`
}

// CurriculumItem representa um topico na fila de estudo
type CurriculumItem struct {
	ID            int64      `json:"id"`
	Topic         string     `json:"topic"`
	Category      string     `json:"category"`
	Priority      int        `json:"priority"`
	Status        string     `json:"status"`
	SourceHint    string     `json:"source_hint"`
	RequestedBy   string     `json:"requested_by"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	InsightsCount int        `json:"insights_count"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

// AutonomousLearner motor de aprendizagem autonoma da EVA
type AutonomousLearner struct {
	db            *database.DB
	cfg           *config.Config
	vectorAdapter *nietzscheInfra.VectorAdapter
	embedSvc      *knowledge.EmbeddingService
	httpClient    *http.Client
	collection    string
}

// NewAutonomousLearner cria um novo motor de aprendizagem
func NewAutonomousLearner(db *database.DB, cfg *config.Config, vectorAdapter *nietzscheInfra.VectorAdapter, embedSvc *knowledge.EmbeddingService) *AutonomousLearner {
	if db == nil {
		log.Warn().Msg("⚠️ [LEARNER] NietzscheDB unavailable — running in degraded mode (no curriculum)")
	}
	return &AutonomousLearner{
		db:            db,
		cfg:           cfg,
		vectorAdapter: vectorAdapter,
		embedSvc:      embedSvc,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
		collection:    "eva_learnings",
	}
}

// Start inicia o loop de aprendizagem autonoma (background)
func (l *AutonomousLearner) Start(ctx context.Context) {
	log.Info().Msg("📚 Autonomous Learner started (cycle: 6h)")

	// Garantir que a collection existe
	if l.vectorAdapter != nil {
		l.ensureCollection(ctx)
	}

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	// Primeiro ciclo apos 5 minutos (dar tempo do sistema subir)
	select {
	case <-time.After(5 * time.Minute):
		l.runCycle(ctx)
	case <-ctx.Done():
		return
	}

	for {
		select {
		case <-ticker.C:
			l.runCycle(ctx)
		case <-ctx.Done():
			log.Info().Msg("📚 Autonomous Learner stopped")
			return
		}
	}
}

// ensureCollection is a no-op - NietzscheDB handles collection management.
func (l *AutonomousLearner) ensureCollection(ctx context.Context) {
}

func (l *AutonomousLearner) runCycle(ctx context.Context) {
	log.Info().Msg("[LEARNER] Starting study cycle...")

	// Buscar proximo topico pendente
	item, err := l.nextPendingTopic(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("[LEARNER] Error fetching next topic")
		return
	}
	if item == nil {
		log.Info().Msg("[LEARNER] No pending topics — EVA is up to date")
		return
	}

	log.Info().Str("topic", item.Topic).Str("category", item.Category).Msg("[LEARNER] Studying topic...")

	// Marcar como studying
	l.updateStatus(ctx, item.ID, "studying", "")

	// Estudar o topico
	insights, err := l.StudyTopic(ctx, item.Topic)
	if err != nil {
		log.Error().Err(err).Str("topic", item.Topic).Msg("[LEARNER] Failed to study topic")
		l.updateStatus(ctx, item.ID, "failed", err.Error())
		return
	}

	// Marcar como completed
	l.completeItem(ctx, item.ID, len(insights))
	log.Info().Str("topic", item.Topic).Int("insights", len(insights)).Msg("📚 EVA learned about topic")
}

// StudyTopic pesquisa um topico, resume e armazena (chamavel via swarm ou background)
func (l *AutonomousLearner) StudyTopic(ctx context.Context, topic string) ([]LearningInsight, error) {
	// 1. Buscar conteudo via Gemini + Google Search grounding
	rawContent, err := l.searchWeb(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	if strings.TrimSpace(rawContent) == "" {
		return nil, fmt.Errorf("no content found for topic: %s", topic)
	}

	// 2. Resumir e extrair insights via Gemini
	insights, err := l.summarize(ctx, rawContent, topic)
	if err != nil {
		return nil, fmt.Errorf("summarization failed: %w", err)
	}

	// 3. Armazenar no NietzscheDB vector
	if l.vectorAdapter != nil && l.embedSvc != nil {
		if err := l.storeInsights(ctx, insights); err != nil {
			log.Warn().Err(err).Msg("[LEARNER] Failed to store in NietzscheDB vector (insights still returned)")
		}
	}

	return insights, nil
}

// searchWeb busca conteudo via Gemini REST com Google Search grounding
func (l *AutonomousLearner) searchWeb(ctx context.Context, query string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s",
		l.cfg.GoogleAPIKey,
	)

	prompt := fmt.Sprintf(
		"Pesquise sobre: %s\n\nRetorne um resumo completo e detalhado com as principais informacoes, conceitos, autores relevantes e fontes. "+
			"Foque em conteudo educativo e de qualidade. Minimo 500 palavras.",
		query,
	)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{
				{"text": prompt},
			}},
		},
		"tools": []map[string]interface{}{
			{"google_search": map[string]interface{}{}},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.3,
		},
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Extrair texto da resposta
	return l.extractText(result), nil
}

// summarize extrai insights estruturados do conteudo bruto
func (l *AutonomousLearner) summarize(ctx context.Context, rawContent, topic string) ([]LearningInsight, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s",
		l.cfg.GoogleAPIKey,
	)

	prompt := fmt.Sprintf(`Analise o seguinte conteudo sobre "%s" e extraia de 3 a 5 insights chave.

Para cada insight, retorne um JSON array com objetos contendo:
- "title": titulo curto do insight (max 80 chars)
- "summary": resumo em 2-3 paragrafos
- "source": fonte principal ou "Sintese de multiplas fontes"
- "tags": array de 3-5 tags relevantes
- "category": uma de [filosofia, ciencia, psicologia, saude, tecnologia, historia, arte, educacao, religiao, cultura]
- "confidence": 0.0-1.0 (quao confiavel e a informacao)

Responda APENAS com o JSON array, sem markdown ou texto adicional.

CONTEUDO:
%s`, topic, rawContent)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{
				{"text": prompt},
			}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.2,
			"responseMimeType": "application/json",
		},
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini summarize error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	text := l.extractText(result)
	if text == "" {
		return nil, fmt.Errorf("empty summarization response")
	}

	// Parsear JSON array de insights
	var rawInsights []struct {
		Title      string   `json:"title"`
		Summary    string   `json:"summary"`
		Source     string   `json:"source"`
		Tags       []string `json:"tags"`
		Category   string   `json:"category"`
		Confidence float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(text), &rawInsights); err != nil {
		return nil, fmt.Errorf("failed to parse insights JSON: %w (text: %.200s)", err, text)
	}

	now := time.Now()
	insights := make([]LearningInsight, 0, len(rawInsights))
	for i, ri := range rawInsights {
		insights = append(insights, LearningInsight{
			ID:         fmt.Sprintf("%s_%d_%d", sanitizeID(topic), now.Unix(), i),
			Topic:      topic,
			Title:      ri.Title,
			Summary:    ri.Summary,
			Source:     ri.Source,
			Tags:       ri.Tags,
			Category:   ri.Category,
			Confidence: ri.Confidence,
			LearnedAt:  now,
		})
	}

	return insights, nil
}

// storeInsights armazena insights no NietzscheDB com embeddings e cria edges de relacionamento.
// FIX: Uses proper UUIDs (server requires them), stores full content via InsertNode
// (the old Upsert path fell back to MergeNode which stored only {"id": timestamp}),
// and creates RELATED_TOPIC / BUILDS_ON edges between related learnings.
func (l *AutonomousLearner) storeInsights(ctx context.Context, insights []LearningInsight) error {
	sdk := l.vectorAdapter.SDK()
	// Collect newly inserted node IDs for edge creation
	var newNodes []storedNode

	for _, insight := range insights {
		// Gerar embedding do conteudo combinado
		embeddingText := fmt.Sprintf("%s: %s. %s", insight.Topic, insight.Title, insight.Summary)
		embedding, err := l.embedSvc.GenerateEmbedding(ctx, embeddingText)
		if err != nil {
			log.Warn().Err(err).Str("title", insight.Title).Msg("[LEARNER] Failed to generate embedding")
			continue
		}

		// Use proper UUID (server requires valid UUIDs, not timestamps)
		nodeID := uuid.New().String()

		// Build full content payload with node_label for type-safe querying
		content := map[string]interface{}{
			"node_label":  "Learning",
			"title":       insight.Title,
			"summary":     insight.Summary,
			"category":    insight.Category,
			"source_url":  insight.Source,
			"topic":       insight.Topic,
			"key_points":  insight.Tags,
			"confidence":  insight.Confidence,
			"timestamp":   insight.LearnedAt.Format(time.RFC3339),
			"learned_at":  insight.LearnedAt.Unix(),
		}

		// Convert embedding to float64 for the SDK
		coords := make([]float64, len(embedding))
		for i, f := range embedding {
			coords[i] = float64(f)
		}

		// Insert directly via SDK InsertNode — bypasses the broken Upsert→MergeNode fallback
		// that was only storing {"id": "timestamp"} as content
		_, err = sdk.InsertNode(ctx, nietzsche.InsertNodeOpts{
			ID:         nodeID,
			Coords:     coords,
			Content:    content,
			NodeType:   "Semantic",
			Energy:     0.7,
			Collection: l.collection,
		})
		if err != nil {
			log.Warn().Err(err).Str("title", insight.Title).Msg("[LEARNER] Failed to insert learning node")
			continue
		}

		log.Info().
			Str("id", nodeID).
			Str("title", insight.Title).
			Str("category", insight.Category).
			Msg("[LEARNER] Stored learning insight")

		newNodes = append(newNodes, storedNode{
			nodeID:    nodeID,
			embedding: embedding,
			category:  insight.Category,
			learnedAt: insight.LearnedAt,
		})

		// Pequeno delay entre embeddings para nao sobrecarregar a API
		time.Sleep(500 * time.Millisecond)
	}

	// --- FIX 2: Create relationship edges between learnings ---
	l.createLearningEdges(ctx, sdk, newNodes)

	return nil
}

// storedNode holds metadata for a newly inserted learning node, used for edge creation.
type storedNode struct {
	nodeID    string
	embedding []float32
	category  string
	learnedAt time.Time
}

// createLearningEdges searches for related existing learnings via KNN and creates
// RELATED_TOPIC (same category) and BUILDS_ON (similar content, newer) edges.
func (l *AutonomousLearner) createLearningEdges(ctx context.Context, sdk *nietzsche.NietzscheClient, newNodes []storedNode) {
	for _, node := range newNodes {
		// Convert embedding to float64 for KNN search
		vec64 := make([]float64, len(node.embedding))
		for i, f := range node.embedding {
			vec64[i] = float64(f)
		}

		// Search for related existing learnings (top 5 nearest neighbors)
		results, err := sdk.KnnSearch(ctx, vec64, 6, l.collection) // 6 because the node itself may appear
		if err != nil {
			log.Warn().Err(err).Str("id", node.nodeID).Msg("[LEARNER] KNN search for edges failed")
			continue
		}

		for _, r := range results {
			// Skip self-match
			if r.ID == node.nodeID {
				continue
			}

			// Get the neighbor node's content to check category
			neighbor, err := sdk.GetNode(ctx, r.ID, l.collection)
			if err != nil || !neighbor.Found {
				continue
			}

			neighborCategory, _ := neighbor.Content["category"].(string)
			neighborTimestamp, _ := neighbor.Content["learned_at"].(float64)

			// Determine edge type and weight
			edgeType := "Association"
			weight := r.Distance // similarity score from KNN

			if weight < 0.3 {
				continue // too dissimilar, skip
			}

			if neighborCategory != "" && neighborCategory == node.category {
				// Same category — RELATED_TOPIC
				edgeType = "Association" // NietzscheDB supports: Association, Hierarchical, LSystemGenerated, Pruned
				// Store the semantic edge type in metadata via weight encoding
				// Higher weight = stronger relationship
				if weight < 0.5 {
					weight = 0.5 // minimum weight for same-category
				}
			}

			// Check if this is a BUILDS_ON relationship (newer content on similar topic)
			isBuildOn := false
			if neighborTimestamp > 0 {
				neighborTime := time.Unix(int64(neighborTimestamp), 0)
				if node.learnedAt.After(neighborTime) && weight > 0.6 {
					isBuildOn = true
					edgeType = "Hierarchical" // newer builds on older — hierarchical relationship
				}
			}

			_, err = sdk.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
				From:       node.nodeID,
				To:         r.ID,
				EdgeType:   edgeType,
				Weight:     weight,
				Collection: l.collection,
			})
			if err != nil {
				log.Warn().Err(err).
					Str("from", node.nodeID).Str("to", r.ID).
					Msg("[LEARNER] Failed to create learning edge")
				continue
			}

			relType := "RELATED_TOPIC"
			if isBuildOn {
				relType = "BUILDS_ON"
			}
			log.Debug().
				Str("from", node.nodeID).Str("to", r.ID).
				Str("type", relType).Float64("weight", weight).
				Msg("[LEARNER] Created learning relationship edge")
		}
	}
}

// SearchLearnings busca semanticamente no conhecimento aprendido
func (l *AutonomousLearner) SearchLearnings(ctx context.Context, query string, limit int) ([]LearningInsight, error) {
	if l.vectorAdapter == nil || l.embedSvc == nil {
		return nil, nil
	}

	embedding, err := l.embedSvc.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	results, err := l.vectorAdapter.Search(ctx, l.collection, embedding, limit, 0)
	if err != nil {
		return nil, err
	}

	var insights []LearningInsight
	for _, point := range results {
		if point.Score < 0.5 {
			continue // Filtrar resultados irrelevantes
		}

		insight := LearningInsight{}
		if v, ok := point.Payload["topic"]; ok {
			if s, ok := v.(string); ok {
				insight.Topic = s
			}
		}
		if v, ok := point.Payload["title"]; ok {
			if s, ok := v.(string); ok {
				insight.Title = s
			}
		}
		if v, ok := point.Payload["summary"]; ok {
			if s, ok := v.(string); ok {
				insight.Summary = s
			}
		}
		if v, ok := point.Payload["source"]; ok {
			if s, ok := v.(string); ok {
				insight.Source = s
			}
		}
		if v, ok := point.Payload["category"]; ok {
			if s, ok := v.(string); ok {
				insight.Category = s
			}
		}
		if v, ok := point.Payload["confidence"]; ok {
			if d, ok := v.(float64); ok {
				insight.Confidence = d
			}
		}
		if v, ok := point.Payload["learned_at"]; ok {
			switch ts := v.(type) {
			case int64:
				insight.LearnedAt = time.Unix(ts, 0)
			case float64:
				insight.LearnedAt = time.Unix(int64(ts), 0)
			}
		}

		insights = append(insights, insight)
	}

	return insights, nil
}

// GetLearningContext monta contexto de conhecimento aprendido para injetar no prompt
func (l *AutonomousLearner) GetLearningContext(ctx context.Context, query string) string {
	if query == "" || l.vectorAdapter == nil || l.embedSvc == nil { //nolint:staticcheck
		return ""
	}

	insights, err := l.SearchLearnings(ctx, query, 3)
	if err != nil || len(insights) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, insight := range insights {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n",
			insight.Title,
			insight.Category,
			truncate(insight.Summary, 200),
		))
	}

	return sb.String()
}

// AddToCurriculum adiciona um topico na fila de estudo
func (l *AutonomousLearner) AddToCurriculum(ctx context.Context, topic, category, requestedBy string, priority int) error {
	if l.db == nil {
		return fmt.Errorf("NietzscheDB unavailable — cannot add to curriculum")
	}
	if category == "" {
		category = "geral"
	}
	if requestedBy == "" {
		requestedBy = "system"
	}
	if priority < 1 || priority > 5 {
		priority = 3
	}

	_, err := l.db.Insert(ctx, "eva_curriculum", map[string]interface{}{
		"topic":          topic,
		"category":       category,
		"priority":       priority,
		"requested_by":   requestedBy,
		"status":         "pending",
		"source_hint":    "",
		"insights_count": 0,
		"error_message":  "",
		"created_at":     time.Now().Format(time.RFC3339),
	})
	return err
}

// ListCurriculum lista topicos do curriculum
func (l *AutonomousLearner) ListCurriculum(ctx context.Context, status string, limit int) ([]CurriculumItem, error) {
	if l.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable — cannot list curriculum")
	}

	extraWhere := ""
	params := map[string]interface{}{}
	if status != "" {
		extraWhere = " AND n.status = $status"
		params["status"] = status
	}

	rows, err := l.db.QueryByLabel(ctx, "eva_curriculum", extraWhere, params, 0)
	if err != nil {
		return nil, err
	}

	var items []CurriculumItem
	for _, m := range rows {
		items = append(items, contentToCurriculumItem(m))
	}

	// Sort by priority DESC, created_at ASC (in Go)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// --- Helpers privados ---

func (l *AutonomousLearner) nextPendingTopic(ctx context.Context) (*CurriculumItem, error) {
	if l.db == nil {
		return nil, nil
	}

	rows, err := l.db.QueryByLabel(ctx, "eva_curriculum", " AND n.status = $status", map[string]interface{}{
		"status": "pending",
	}, 0)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// Convert to CurriculumItem slice for sorting
	items := make([]CurriculumItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, contentToCurriculumItem(m))
	}

	// Sort by priority DESC, created_at ASC and take first
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return &items[0], nil
}

func (l *AutonomousLearner) updateStatus(ctx context.Context, id int64, status, errMsg string) {
	if l.db == nil {
		return
	}
	updates := map[string]interface{}{
		"status": status,
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if err := l.db.Update(ctx, "eva_curriculum", map[string]interface{}{
		"id": id,
	}, updates); err != nil {
		log.Warn().Err(err).Int64("curriculum_id", id).Msgf("failed to update curriculum item status to %s", status)
	}
}

func (l *AutonomousLearner) completeItem(ctx context.Context, id int64, insightsCount int) {
	if l.db == nil {
		return
	}
	if err := l.db.Update(ctx, "eva_curriculum", map[string]interface{}{
		"id": id,
	}, map[string]interface{}{
		"status":         "completed",
		"completed_at":   time.Now().Format(time.RFC3339),
		"insights_count": insightsCount,
	}); err != nil {
		log.Warn().Err(err).Int64("curriculum_id", id).Msg("failed to mark curriculum item as completed")
	}
}

// contentToCurriculumItem converts a NietzscheDB content map to a CurriculumItem.
func contentToCurriculumItem(m map[string]interface{}) CurriculumItem {
	return CurriculumItem{
		ID:            database.GetInt64(m, "id"),
		Topic:         database.GetString(m, "topic"),
		Category:      database.GetString(m, "category"),
		Priority:      int(database.GetInt64(m, "priority")),
		Status:        database.GetString(m, "status"),
		SourceHint:    database.GetString(m, "source_hint"),
		RequestedBy:   database.GetString(m, "requested_by"),
		CreatedAt:     database.GetTime(m, "created_at"),
		CompletedAt:   database.GetTimePtr(m, "completed_at"),
		InsightsCount: int(database.GetInt64(m, "insights_count")),
		ErrorMessage:  database.GetString(m, "error_message"),
	}
}

func (l *AutonomousLearner) extractText(result map[string]interface{}) string {
	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return ""
	}
	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return ""
	}
	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return ""
	}
	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return ""
	}

	var texts []string
	for _, part := range parts {
		p, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := p["text"].(string); ok {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
