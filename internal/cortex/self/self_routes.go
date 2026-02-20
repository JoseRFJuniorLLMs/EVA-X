// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package self

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// RegisterRoutes registra rotas HTTP para Core Memory
func RegisterRoutes(router *mux.Router, engine *CoreMemoryEngine) {
	// Subrouter com prefixo /self
	selfRouter := router.PathPrefix("/self").Subrouter()

	// Personalidade de EVA
	selfRouter.HandleFunc("/personality", getPersonalityHandler(engine)).Methods("GET")
	selfRouter.HandleFunc("/identity", getIdentityHandler(engine)).Methods("GET")

	// Memórias
	selfRouter.HandleFunc("/memories", getMemoriesHandler(engine)).Methods("GET")
	selfRouter.HandleFunc("/memories/search", searchMemoriesHandler(engine)).Methods("POST")
	selfRouter.HandleFunc("/memories/stats", getMemoryStatsHandler(engine)).Methods("GET")

	// Meta-insights
	selfRouter.HandleFunc("/insights", getMetaInsightsHandler(engine)).Methods("GET")
	selfRouter.HandleFunc("/insights/{id}", getMetaInsightByIDHandler(engine)).Methods("GET")

	// Ensino direto
	selfRouter.HandleFunc("/teach", teachEVAHandler(engine)).Methods("POST")

	// Processamento de sessão (chamado internamente)
	selfRouter.HandleFunc("/session/process", processSessionHandler(engine)).Methods("POST")

	// Análises e estatísticas
	selfRouter.HandleFunc("/analytics/diversity", getDiversityScoreHandler(engine)).Methods("GET")
	selfRouter.HandleFunc("/analytics/growth", getPersonalityGrowthHandler(engine)).Methods("GET")
}

// --- HANDLERS ---

// getPersonalityHandler retorna personalidade atual de EVA
func getPersonalityHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		personality, err := engine.GetEVAPersonality(ctx)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"personality": personality,
			"big_five": map[string]float64{
				"openness":          personality.Openness,
				"conscientiousness": personality.Conscientiousness,
				"extraversion":      personality.Extraversion,
				"agreeableness":     personality.Agreeableness,
				"neuroticism":       personality.Neuroticism,
			},
			"enneagram": map[string]int{
				"primary_type": personality.PrimaryType,
				"wing":         personality.Wing,
			},
			"experience": map[string]int{
				"total_sessions":  personality.TotalSessions,
				"crises_handled":  personality.CrisesHandled,
				"breakthroughs":   personality.Breakthroughs,
			},
		})
	}
}

// getIdentityHandler retorna contexto de identidade para priming
func getIdentityHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		identityContext, err := engine.GetIdentityContext(ctx)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"identity_text": identityContext,
		})
	}
}

// getMemoriesHandler retorna memórias de EVA
func getMemoriesHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Query params opcionais
		memoryType := r.URL.Query().Get("type")
		limitStr := r.URL.Query().Get("limit")

		limit := 50 // default
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil {
				limit = l
			}
		}

		// NQL query — relação criada em recordMemory é REMEMBERS
		nql := `MATCH (eva:EvaSelf)-[:REMEMBERS]->(mem:CoreMemory) RETURN mem`
		if memoryType != "" {
			nql = fmt.Sprintf(
				`MATCH (eva:EvaSelf)-[:REMEMBERS]->(mem:CoreMemory) WHERE mem.memory_type = "%s" RETURN mem`,
				memoryType,
			)
		}

		records, err := engine.ExecuteReadQuery(ctx, nql, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var memories []map[string]interface{}
		for _, record := range records {
			memory := make(map[string]interface{})
			for _, key := range []string{"id", "content", "memory_type", "abstraction_level", "importance_weight", "reinforcement_count", "created_at"} {
				if val, ok := record[key]; ok {
					memory[key] = val
				}
			}
			memories = append(memories, memory)
			if len(memories) >= limit {
				break
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"memories": memories,
			"count":    len(memories),
		})
	}
}

// searchMemoriesHandler busca memórias por similaridade semântica
func searchMemoriesHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "JSON inválido")
			return
		}

		if req.TopK == 0 {
			req.TopK = 10
		}

		ctx := r.Context()

		// Busca todas as memórias
		query := `
			MATCH (eva:EvaSelf)-[:REMEMBERS]->(mem:CoreMemory)
			RETURN mem.id AS id, mem.content AS content, mem.embedding AS embedding, mem.reinforcement_count AS reinforcement
		`

		records, err := engine.ExecuteReadQuery(ctx, query, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Converte para ExistingMemory
		var memories []ExistingMemory
		for _, record := range records {
			id := record["id"]
			content := record["content"]
			embeddingRaw := record["embedding"]
			reinforcement := record["reinforcement_count"]

			// Converte embedding
			var embedding []float32
			if embSlice, ok := embeddingRaw.([]interface{}); ok {
				for _, v := range embSlice {
					if f, ok := v.(float64); ok {
						embedding = append(embedding, float32(f))
					}
				}
			}

			var reinforcementInt int
			switch v := reinforcement.(type) {
			case int64:
				reinforcementInt = int(v)
			case float64:
				reinforcementInt = int(v)
			}

			memories = append(memories, ExistingMemory{
				ID:                 fmt.Sprintf("%v", id),
				Content:            fmt.Sprintf("%v", content),
				Embedding:          embedding,
				ReinforcementCount: reinforcementInt,
			})
		}

		// Usa deduplicador para buscar similares
		similar, err := engine.deduplicator.GetSimilarMemories(ctx, req.Query, memories, req.TopK)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"query":   req.Query,
			"results": similar,
		})
	}
}

// getMemoryStatsHandler retorna estatísticas sobre memórias
func getMemoryStatsHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		query := `
			MATCH (eva:EvaSelf)-[:REMEMBERS]->(mem:CoreMemory)
			RETURN
				count(mem) AS total,
				mem.memory_type AS type
		`

		records, err := engine.ExecuteReadQuery(ctx, query, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		stats := make(map[string]int)
		total := 0

		for _, record := range records {
			memType := record["memory_type"]
			countVal := record["total"]

			typeStr := fmt.Sprintf("%v", memType)
			var countInt int
			switch v := countVal.(type) {
			case int64:
				countInt = int(v)
			case float64:
				countInt = int(v)
			}
			stats[typeStr] = countInt
			total += countInt
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total_memories": total,
			"by_type":        stats,
		})
	}
}

// getMetaInsightsHandler retorna meta-insights
func getMetaInsightsHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		query := `
			MATCH (eva:EvaSelf)-[:DISCOVERED]->(insight:MetaInsight)
			RETURN insight.id AS id,
			       insight.content AS content,
			       insight.evidence_count AS evidence,
			       insight.confidence AS confidence,
			       insight.discovered_at AS discovered_at
			ORDER BY insight.confidence DESC, insight.evidence_count DESC
		`

		records, err := engine.ExecuteReadQuery(ctx, query, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var insights []map[string]interface{}
		for _, record := range records {
			insight := make(map[string]interface{})
			for _, key := range []string{"id", "content", "evidence_count", "confidence", "discovered_at"} {
				if val, ok := record[key]; ok {
					insight[key] = val
				}
			}
			insights = append(insights, insight)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"insights": insights,
			"count":    len(insights),
		})
	}
}

// getMetaInsightByIDHandler retorna um meta-insight específico
func getMetaInsightByIDHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)
		insightID := vars["id"]

		query := `
			MATCH (eva:EvaSelf)-[:DISCOVERED]->(insight:MetaInsight {id: $id})
			OPTIONAL MATCH (insight)<-[:SUPPORTS]-(mem:CoreMemory)
			RETURN insight, collect(mem.content) AS supporting_memories
		`

		records, err := engine.ExecuteReadQuery(ctx, query, map[string]interface{}{"id": insightID})
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(records) == 0 {
			respondError(w, http.StatusNotFound, "Meta-insight não encontrado")
			return
		}

		record := records[0]
		insight := record
		memories := record["supporting_memories"]

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"insight":             insight,
			"supporting_memories": memories,
		})
	}
}

// teachEVAHandler permite ensinar EVA diretamente
func teachEVAHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Lesson   string  `json:"lesson"`
			Category string  `json:"category"`
			Importance float64 `json:"importance"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "JSON inválido")
			return
		}

		if req.Lesson == "" {
			respondError(w, http.StatusBadRequest, "Campo 'lesson' obrigatório")
			return
		}

		if req.Importance == 0 {
			req.Importance = 0.8 // default
		}

		ctx := r.Context()

		if err := engine.TeachEVA(ctx, req.Lesson, req.Importance); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"message": "Lição ensinada com sucesso",
			"lesson":  req.Lesson,
		})
	}
}

// processSessionHandler processa fim de sessão (chamado internamente)
func processSessionHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SessionID        string   `json:"session_id"`
			Transcript       string   `json:"transcript"`
			Duration         int      `json:"duration"`
			CrisisDetected   bool     `json:"crisis_detected"`
			UserSatisfaction float64  `json:"user_satisfaction"`
			Topics           []string `json:"topics"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "JSON inválido")
			return
		}

		ctx := r.Context()

		sessionData := SessionData{
			SessionID:      req.SessionID,
			Transcript:     req.Transcript,
			DurationMinutes: float64(req.Duration),
			CrisisHappened: req.CrisisDetected,
		}

		if err := engine.ProcessSessionEnd(ctx, sessionData); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "Sessão processada com sucesso",
			"session_id": req.SessionID,
		})
	}
}

// getDiversityScoreHandler retorna score de diversidade das memórias
func getDiversityScoreHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Busca todas as memórias com embeddings
		query := `
			MATCH (eva:EvaSelf)-[:REMEMBERS]->(mem:CoreMemory)
			RETURN mem.id AS id, mem.content AS content, mem.embedding AS embedding, mem.reinforcement_count AS reinforcement
		`

		records, err := engine.ExecuteReadQuery(ctx, query, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var memories []ExistingMemory
		for _, record := range records {
			id := record["id"]
			content := record["content"]
			embeddingRaw := record["embedding"]
			reinforcement := record["reinforcement_count"]

			var embedding []float32
			if embSlice, ok := embeddingRaw.([]interface{}); ok {
				for _, v := range embSlice {
					if f, ok := v.(float64); ok {
						embedding = append(embedding, float32(f))
					}
				}
			}

			var reinforcementInt int
			switch v := reinforcement.(type) {
			case int64:
				reinforcementInt = int(v)
			case float64:
				reinforcementInt = int(v)
			}

			memories = append(memories, ExistingMemory{
				ID:                 fmt.Sprintf("%v", id),
				Content:            fmt.Sprintf("%v", content),
				Embedding:          embedding,
				ReinforcementCount: reinforcementInt,
			})
		}

		diversityScore := engine.deduplicator.CalculateDiversityScore(memories)

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"diversity_score": diversityScore,
			"interpretation":  interpretDiversity(diversityScore),
			"total_memories":  len(memories),
		})
	}
}

// getPersonalityGrowthHandler retorna evolução da personalidade
func getPersonalityGrowthHandler(engine *CoreMemoryEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// TODO: Implementar histórico de personalidade (requer schema adicional)
		// Por enquanto, retorna personalidade atual
		personality, err := engine.GetEVAPersonality(ctx)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"current":  personality,
			"message":  "Histórico de evolução será implementado em breve",
		})
	}
}

// --- HELPERS ---

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func interpretDiversity(score float64) string {
	if score >= 0.8 {
		return "Alta diversidade - memórias cobrem temas variados"
	} else if score >= 0.5 {
		return "Diversidade moderada - alguma repetição de temas"
	} else if score >= 0.3 {
		return "Baixa diversidade - muitas memórias similares"
	}
	return "Muito baixa diversidade - considere consolidar memórias"
}
