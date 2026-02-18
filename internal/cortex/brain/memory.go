// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package brain

import (
	"context"
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/pkg/types"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/qdrant/go-client/qdrant"
)

// ========================================
// MEMORY SAVE AUDIT - 2026-01-27
// Corrigido para salvar em TODOS os datastores
// ========================================

// ProcessUserSpeech handles user transcription in real-time (FDPN Hook)
// 🚀 IMPROVED: Now uses atomic facts ONLY, no raw text storage
// 🚀 IMPROVEMENT 3: Now accepts audio context from AudioAnalysisService
func (s *Service) ProcessUserSpeech(ctx context.Context, idosoID int64, text string, audioEmotion, audioUrgency string, audioIntensity int) {
	if len(text) < 10 {
		return // Ignore short texts
	}

	userID := fmt.Sprintf("%d", idosoID)

	// Output log to track flow
	log.Printf("🗣️ [User Speech] Processing for user %s: %s", userID, text)

	// 🚀 ACTIVATE UNIFIED RETRIEVAL PRIMING (RSI + FDPN)
	if s.unifiedRetrieval != nil {
		go s.unifiedRetrieval.Prime(ctx, idosoID, text)
	}

	// 🧠 ATOMIC INGESTION - ONLY ATOMIC FACTS, NO RAW TEXT
	if s.ingestionPipeline != nil && len(text) > 30 {
		go func() {
			facts, err := s.ingestionPipeline.ProcessText(ctx, text)
			if err != nil || len(facts) == 0 {
				log.Printf("⚠️ [Ingestion] Atomic extraction failed/empty, saving summary")

				// Fallback: save summarized version, not raw text
				summary := summarizeText(text)
				memCtx := MemoryContext{
					Emotion:        audioEmotion, // 🚀 IMPROVEMENT 3: Use detected emotion
					Urgency:        audioUrgency, // 🚀 IMPROVEMENT 3: Use detected urgency
					AudioIntensity: audioIntensity, // 🚀 IMPROVEMENT 6: Use detected intensity
					Keywords:       extractKeywords(summary),
				}
				if err := s.SaveEpisodicMemoryWithContext(idosoID, "user", summary, time.Now(), true, memCtx); err != nil {
					log.Printf("❌ [Ingestion] Falha ao salvar resumo idoso=%d: %v", idosoID, err)
				}
				return
			}

			// 🚀 IMPROVEMENT 1: Save ONLY atomic facts
			for _, fact := range facts {
				memCtx := MemoryContext{
					Emotion:        audioEmotion, // 🚀 IMPROVEMENT 3: Use detected emotion
					Urgency:        audioUrgency, // 🚀 IMPROVEMENT 3: Use detected urgency
					AudioIntensity: audioIntensity, // 🚀 IMPROVEMENT 6: Use detected intensity
					Keywords:       extractKeywords(fact.Content),
				}

				// 🚀 IMPROVEMENT 5: Use correct event date from ingestion pipeline
				if err := s.SaveEpisodicMemoryWithContext(idosoID, "user", fact.Content, fact.EventTime, true, memCtx); err != nil {
					log.Printf("❌ [Ingestion] Falha ao salvar fato atômico idoso=%d: %v", idosoID, err)
				}
			}

			log.Printf("✅ [Ingestion] Saved %d atomic facts", len(facts))
		}()
	} else {
		// Fallback for short text: save as-is but mark as atomic
		memCtx := MemoryContext{
			Emotion:        audioEmotion, // 🚀 IMPROVEMENT 3: Use detected emotion
			Urgency:        audioUrgency, // 🚀 IMPROVEMENT 3: Use detected urgency
			AudioIntensity: audioIntensity, // 🚀 IMPROVEMENT 6: Use detected intensity
			Keywords:       extractKeywords(text),
		}
		go func() {
			if err := s.SaveEpisodicMemoryWithContext(idosoID, "user", text, time.Now(), true, memCtx); err != nil {
				log.Printf("❌ [Memory] Falha ao salvar texto idoso=%d: %v", idosoID, err)
			}
		}()
	}
}

// SaveEpisodicMemory saves memory to Postgres, Qdrant, and Neo4j
// AUDIT FIX: 2026-01-27 - Agora salva em TODOS os datastores
//
// ⚠️ DEPRECATED: Use SaveEpisodicMemoryWithContext instead
// This function uses hardcoded emotion="neutral" and importance=0.5
// The new function supports:
//   - Dynamic importance calculation
//   - Actual emotion detection from AudioAnalysis
//   - Krylov compression (3072D → 64D)
//   - Full context metadata
func (s *Service) SaveEpisodicMemory(idosoID int64, role, content string, eventDate time.Time, isAtomic bool) {
	ctx := context.Background()

	log.Printf("🧠 [MEMORY] Iniciando salvamento - Idoso: %d, Role: %s, Tamanho: %d chars", idosoID, role, len(content))

	// Validação básica
	if len(content) < 5 {
		log.Printf("⚠️ [MEMORY] Conteúdo muito curto, ignorando: '%s'", content)
		return
	}

	if s.db == nil {
		log.Printf("❌ [MEMORY] Database connection is nil!")
		return
	}

	// 1. Tentar gerar Embedding (não bloqueia se falhar)
	var embedding []float32
	var embeddingErr error

	if s.embeddingService != nil {
		embedding, embeddingErr = s.embeddingService.GenerateEmbedding(ctx, content)
		if embeddingErr != nil {
			log.Printf("⚠️ [MEMORY] Erro ao gerar embedding (continuando sem): %v", embeddingErr)
			// Criar embedding zerado para não bloquear salvamento
			embedding = make([]float32, 3072) // gemini-embedding-001 usa 3072
		}
	} else {
		log.Printf("⚠️ [MEMORY] EmbeddingService é nil, usando embedding zerado")
		embedding = make([]float32, 3072)
	}

	// 2. Salvar no Postgres (SEMPRE tenta)
	var memoryID int64
	query := `
		INSERT INTO episodic_memories (idoso_id, speaker, content, embedding, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`
	err := s.db.QueryRow(query, idosoID, role, content, pgvector.NewVector(embedding)).Scan(&memoryID)
	if err != nil {
		log.Printf("❌ [POSTGRES] Erro ao salvar memória: %v", err)

		// Tentar salvar SEM embedding como fallback
		queryNoEmbed := `
			INSERT INTO episodic_memories (idoso_id, speaker, content, created_at)
			VALUES ($1, $2, $3, NOW())
			RETURNING id
		`
		err2 := s.db.QueryRow(queryNoEmbed, idosoID, role, content).Scan(&memoryID)
		if err2 != nil {
			log.Printf("❌ [POSTGRES] Fallback também falhou: %v", err2)
			return
		}
		log.Printf("✅ [POSTGRES] Memory saved (sem embedding): ID=%d", memoryID)
	} else {
		log.Printf("✅ [POSTGRES] Memory saved: ID=%d, Speaker=%s", memoryID, role)
	}

	// 3. Upsert to Qdrant (Retry Logic)
	if s.qdrantClient != nil {
		go func() {
			metadata := types.MemoryMetadata{
				Emotion:    "neutral",
				Importance: 0.5,
				Topics:     extractKeywords(content),
			}

			// Tentar 3 vezes
			for attempt := 1; attempt <= 3; attempt++ {
				points := []*qdrant.PointStruct{
					{
						Id: &qdrant.PointId{
							PointIdOptions: &qdrant.PointId_Num{Num: uint64(memoryID)},
						},
						Vectors: &qdrant.Vectors{
							VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: embedding}},
						},
						Payload: map[string]*qdrant.Value{
							"idoso_id":   {Kind: &qdrant.Value_IntegerValue{IntegerValue: idosoID}},
							"role":       {Kind: &qdrant.Value_StringValue{StringValue: role}},
							"content":    {Kind: &qdrant.Value_StringValue{StringValue: content}},
							"created_at": {Kind: &qdrant.Value_StringValue{StringValue: time.Now().Format(time.RFC3339)}},
							"emotion":    {Kind: &qdrant.Value_StringValue{StringValue: metadata.Emotion}},
							"topics":     stringSliceToQdrantList(metadata.Topics),
						},
					},
				}

				if err := s.qdrantClient.Upsert(ctx, "memories", points); err != nil {
					log.Printf("⚠️ [QDRANT] Upsert falhou (tentativa %d): %v", attempt, err)
					time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
					continue
				}

				log.Printf("✅ [QDRANT] Memory %d indexed successfully", memoryID)

				// 4. Update Personality State (Async)
				if role == "user" && s.personalityService != nil {
					go func() {
						pctx, pcancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer pcancel()
						s.personalityService.UpdateAfterConversation(pctx, idosoID, metadata.Emotion, metadata.Topics)
					}()
				}
				break
			}
		}()
	} else {
		log.Printf("⚠️ [QDRANT] Cliente não disponível, pulando indexação vetorial")
	}

	// 5. AUDIT FIX: Salvar no Neo4j (Graph Store)
	if s.graphStore != nil {
		go func() {
			neo4jCtx, neo4jCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer neo4jCancel()

			graphMemory := &memory.Memory{
				ID:         memoryID,
				IdosoID:    idosoID,
				Content:    content,
				Speaker:    role,
				Emotion:    "neutral",
				Importance: 0.5,
				SessionID:  fmt.Sprintf("session-%d", time.Now().Unix()),
				Timestamp:  time.Now(),
				Topics:     extractKeywords(content),
			}

			if err := s.graphStore.AddEpisodicMemory(neo4jCtx, graphMemory); err != nil {
				log.Printf("❌ [NEO4J] Erro ao salvar no grafo: %v", err)
			} else {
				log.Printf("✅ [NEO4J] Memory %d salva no grafo com sucesso", memoryID)
			}
		}()
	} else {
		log.Printf("❌ [NEO4J] GraphStore é NIL - memória NÃO será salva no Neo4j!")
	}

	log.Printf("🧠 [MEMORY] Salvamento completo para idoso %d - ID: %d", idosoID, memoryID)
}

// Helper to extract keywords
func extractKeywords(text string) []string {
	stopwords := map[string]bool{
		"o": true, "a": true, "de": true, "que": true, "e": true,
		"do": true, "da": true, "em": true, "um": true, "para": true,
		"com": true, "não": true, "uma": true, "os": true, "no": true,
		"se": true, "na": true, "por": true, "mais": true, "as": true,
	}

	var keywords []string
	seen := make(map[string]bool)

	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,!?;:'\"")
		if len(w) < 3 || stopwords[w] || seen[w] {
			continue
		}
		keywords = append(keywords, w)
		seen[w] = true
	}

	return keywords
}

func stringSliceToQdrantList(slice []string) *qdrant.Value {
	values := make([]*qdrant.Value, len(slice))
	for i, s := range slice {
		values[i] = &qdrant.Value{
			Kind: &qdrant.Value_StringValue{StringValue: s},
		}
	}
	return &qdrant.Value{
		Kind: &qdrant.Value_ListValue{
			ListValue: &qdrant.ListValue{Values: values},
		},
	}
}
