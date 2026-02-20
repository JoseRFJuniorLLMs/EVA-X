// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/infrastructure/graph"
	"fmt"
	"log"
	"time"
)

// GraphStore gerencia o armazenamento de memórias em Grafo (Neo4j)
type GraphStore struct {
	client *graph.Neo4jClient
	cfg    *config.Config
}

// NewGraphStore cria um novo gerenciador de memórias em grafo
func NewGraphStore(client *graph.Neo4jClient, cfg *config.Config) *GraphStore {
	return &GraphStore{
		client: client,
		cfg:    cfg,
	}
}

// AddEpisodicMemory salva uma memória "explodida" em nós com metadados temporais
func (g *GraphStore) AddEpisodicMemory(ctx context.Context, memory *Memory) error {
	// 1. Criar nó do Evento Base
	query := `
		MERGE (p:Person {id: $idosoId})
		CREATE (e:Event {
			id: $id,
			content: $content,
			timestamp: datetime($timestamp),
			event_date: datetime($eventDate),
			is_atomic: $isAtomic,
			speaker: $speaker,
			emotion: $emotion,
			importance: $importance,
			sessionId: $sessionId,
			type: 'episodic',
			activation_score: 1.0
		})
		CREATE (p)-[:EXPERIENCED]->(e)
	`

	params := map[string]interface{}{
		"idosoId":    memory.IdosoID,
		"content":    memory.Content,
		"timestamp":  memory.Timestamp.Format(time.RFC3339),
		"eventDate":  memory.EventDate.Format(time.RFC3339),
		"isAtomic":   memory.IsAtomic,
		"speaker":    memory.Speaker,
		"emotion":    memory.Emotion,
		"importance": memory.Importance,
		"sessionId":  memory.SessionID,
	}

	// Se memory.ID for zero (novo), gerar UUID ou usar timestamp
	if memory.ID == 0 {
		params["id"] = fmt.Sprintf("%d-%d", memory.IdosoID, time.Now().UnixNano())
	} else {
		params["id"] = fmt.Sprintf("%d", memory.ID)
	}

	_, err := g.client.ExecuteWrite(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create base event node: %w", err)
	}

	// 2. Extrair e conectar entidades (Simplificado por agora - idealmente via LLM)
	// Aqui poderíamos conectar Topicos
	if len(memory.Topics) > 0 {
		for _, topic := range memory.Topics {
			topicQuery := `
				MATCH (e:Event {id: $eventId})
				MATCH (p:Person {id: $idosoId})
				
				MERGE (t:Topic {name: $topic})
				ON CREATE SET t.created = datetime()
				
				// Conectar Event -> Topic
				MERGE (e)-[:RELATED_TO]->(t)
				
				// ✅ Conectar Person -> Topic COM CONTADOR
				MERGE (p)-[r:MENTIONED]->(t)
				ON CREATE SET r.count = 1, r.first_mention = datetime()
				ON MATCH SET 
					r.count = r.count + 1,
					r.last_mention = datetime()
			`
			topicParams := map[string]interface{}{
				"eventId": params["id"],
				"idosoId": memory.IdosoID,
				"topic":   topic,
			}

			// ✅ CORREÇÃO P6: Agora loga erros explicitamente
			_, err := g.client.ExecuteWrite(ctx, topicQuery, topicParams)
			if err != nil {
				log.Printf("❌ [NEO4J] Falha ao conectar topic '%s' para evento %s: %v",
					topic, params["id"], err)
				// Continuar com outros topics mesmo se um falhar
			} else {
				log.Printf("✅ [NEO4J] Topic conectado: '%s' → Person %d", topic, memory.IdosoID)
			}
		}
	}

	// 3. Conectar Emoções (Se houver na análise)
	if memory.Emotion != "" && memory.Emotion != "neutro" {
		emotionQuery := `
			MATCH (p:Person {id: $idosoId})
			MERGE (em:Emotion {name: $emotion})
			MERGE (p)-[r:FEELS]->(em)
			ON CREATE SET r.count = 1, r.first_felt = datetime()
			ON MATCH SET 
				r.count = r.count + 1,
				r.last_felt = datetime()
		`
		emotionParams := map[string]interface{}{
			"idosoId": memory.IdosoID,
			"emotion": memory.Emotion,
		}

		// ✅ CORREÇÃO P6: Agora loga erros explicitamente
		_, err := g.client.ExecuteWrite(ctx, emotionQuery, emotionParams)
		if err != nil {
			log.Printf("❌ [NEO4J] Falha ao conectar emoção '%s' para Person %d: %v",
				memory.Emotion, memory.IdosoID, err)
		} else {
			log.Printf("✅ [NEO4J] Emoção conectada: '%s' → Person %d", memory.Emotion, memory.IdosoID)
		}
	}

	return nil
}

// GetRelatedMemoriesRecursive busca memórias relacionadas recursivamente via grafo
// Usa caminhos de comprimento variável para encontrar conexões indiretas (Topic -> Event -> Topic -> Event)
func (g *GraphStore) GetRelatedMemoriesRecursive(ctx context.Context, memoryID int64, limit int) ([]int64, error) {
	// Query para buscar eventos conectados por até 2 "saltos" de tópicos (Event->Topic->Event->Topic->Event)
	// Isso corresponde a 4 relacionamentos :RELATED_TO
	query := `
		MATCH (start:Event {id: $id})
		MATCH path = (start)-[:RELATED_TO*1..4]-(related:Event)
		WHERE related.id <> start.id
		// Evitar loops e voltar pro mesmo
		
		RETURN DISTINCT related.id as id, length(path) as hops, related.importance as importance
		ORDER BY hops ASC, importance DESC
		LIMIT $limit
	`

	params := map[string]interface{}{
		"id":    fmt.Sprintf("%d", memoryID),
		"limit": limit,
	}

	result, err := g.client.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query recursive relations: %w", err)
	}

	var relatedIDs []int64
	for _, record := range result {
		// O ID no Neo4j é string/string formatada, mas precisamos retornar int64 para o sistema
		val, ok := record.Get("id")
		if !ok {
			continue
		}

		idStr, ok := val.(string)
		if !ok {
			continue
		}

		// Parse int64
		var id int64
		_, err := fmt.Sscanf(idStr, "%d", &id)
		if err == nil {
			relatedIDs = append(relatedIDs, id)
		}
	}

	log.Printf("🕸️ [GRAPH] Busca Recursiva (ID %d): Encontrados %d eventos relacionados", memoryID, len(relatedIDs))
	return relatedIDs, nil
}
