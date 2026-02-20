// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"fmt"
	"log"
	"time"
)

// GraphStore gerencia o armazenamento de memorias em Grafo (NietzscheDB)
type GraphStore struct {
	client *nietzscheInfra.GraphAdapter
	cfg    *config.Config
}

// NewGraphStore cria um novo gerenciador de memorias em grafo
func NewGraphStore(client *nietzscheInfra.GraphAdapter, cfg *config.Config) *GraphStore {
	return &GraphStore{
		client: client,
		cfg:    cfg,
	}
}

// AddEpisodicMemory salva uma memoria "explodida" em nos com metadados temporais
func (g *GraphStore) AddEpisodicMemory(ctx context.Context, memory *Memory) error {
	// 1. Criar/Merge no Person
	personResult, err := g.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Person",
		MatchKeys: map[string]interface{}{"id": memory.IdosoID},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Person node: %w", err)
	}
	personNodeID := personResult.NodeID

	// Gerar ID do evento
	var eventID string
	if memory.ID == 0 {
		eventID = fmt.Sprintf("%d-%d", memory.IdosoID, time.Now().UnixNano())
	} else {
		eventID = fmt.Sprintf("%d", memory.ID)
	}

	// 2. Criar no Event via MergeNode
	eventResult, err := g.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Event",
		MatchKeys: map[string]interface{}{"id": eventID},
		OnCreateSet: map[string]interface{}{
			"content":          memory.Content,
			"timestamp":        float64(memory.Timestamp.Unix()),
			"event_date":       float64(memory.EventDate.Unix()),
			"is_atomic":        memory.IsAtomic,
			"speaker":          memory.Speaker,
			"emotion":          memory.Emotion,
			"importance":       memory.Importance,
			"sessionId":        memory.SessionID,
			"type":             "episodic",
			"activation_score": 1.0,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create base event node: %w", err)
	}
	eventNodeID := eventResult.NodeID

	// 3. Criar aresta Person -> EXPERIENCED -> Event
	_, err = g.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: personNodeID,
		ToNodeID:   eventNodeID,
		EdgeType:   "EXPERIENCED",
	})
	if err != nil {
		return fmt.Errorf("failed to create EXPERIENCED edge: %w", err)
	}

	// 4. Extrair e conectar entidades (Topics)
	if len(memory.Topics) > 0 {
		for _, topic := range memory.Topics {
			// Merge Topic node
			topicResult, err := g.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
				NodeType:    "Topic",
				MatchKeys:   map[string]interface{}{"name": topic},
				OnCreateSet: map[string]interface{}{"created": nietzscheInfra.NowUnix()},
			})
			if err != nil {
				log.Printf("[NIETZSCHE] Falha ao criar topic '%s' para evento %s: %v",
					topic, eventID, err)
				continue
			}
			topicNodeID := topicResult.NodeID

			// Event -> RELATED_TO -> Topic
			_, err = g.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: eventNodeID,
				ToNodeID:   topicNodeID,
				EdgeType:   "RELATED_TO",
			})
			if err != nil {
				log.Printf("[NIETZSCHE] Falha ao conectar event->topic '%s': %v", topic, err)
				continue
			}

			// Person -> MENTIONED -> Topic (com contador)
			_, err = g.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID:  personNodeID,
				ToNodeID:    topicNodeID,
				EdgeType:    "MENTIONED",
				OnCreateSet: map[string]interface{}{"count": 1, "first_mention": nietzscheInfra.NowUnix()},
				OnMatchSet:  map[string]interface{}{"count_increment": 1, "last_mention": nietzscheInfra.NowUnix()},
			})
			if err != nil {
				log.Printf("[NIETZSCHE] Falha ao conectar person->topic '%s': %v", topic, err)
				continue
			}

			log.Printf("[NIETZSCHE] Topic conectado: '%s' -> Person %d", topic, memory.IdosoID)
		}
	}

	// 5. Conectar Emocoes (Se houver na analise)
	if memory.Emotion != "" && memory.Emotion != "neutro" {
		emotionResult, err := g.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType:  "Emotion",
			MatchKeys: map[string]interface{}{"name": memory.Emotion},
		})
		if err != nil {
			log.Printf("[NIETZSCHE] Falha ao criar emocao '%s' para Person %d: %v",
				memory.Emotion, memory.IdosoID, err)
		} else {
			_, err = g.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID:  personNodeID,
				ToNodeID:    emotionResult.NodeID,
				EdgeType:    "FEELS",
				OnCreateSet: map[string]interface{}{"count": 1, "first_felt": nietzscheInfra.NowUnix()},
				OnMatchSet:  map[string]interface{}{"count_increment": 1, "last_felt": nietzscheInfra.NowUnix()},
			})
			if err != nil {
				log.Printf("[NIETZSCHE] Falha ao conectar emocao '%s' para Person %d: %v",
					memory.Emotion, memory.IdosoID, err)
			} else {
				log.Printf("[NIETZSCHE] Emocao conectada: '%s' -> Person %d", memory.Emotion, memory.IdosoID)
			}
		}
	}

	return nil
}

// GetRelatedMemoriesRecursive busca memorias relacionadas recursivamente via grafo
// Usa BFS para encontrar conexoes indiretas (Event->Topic->Event->Topic->Event)
func (g *GraphStore) GetRelatedMemoriesRecursive(ctx context.Context, memoryID int64, limit int) ([]int64, error) {
	// BFS com profundidade 4 (equivalente a *1..4)
	startID := fmt.Sprintf("%d", memoryID)
	nodeIDs, err := g.client.Bfs(ctx, startID, 4, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query recursive relations: %w", err)
	}

	var relatedIDs []int64
	count := 0
	for _, nodeIDStr := range nodeIDs {
		if nodeIDStr == startID {
			continue
		}
		// Buscar o no para verificar se e um Event
		node, err := g.client.GetNode(ctx, nodeIDStr, "")
		if err != nil {
			continue
		}
		// Verificar se e um Event (pelo content)
		if nodeType, ok := node.Content["type"]; ok {
			if nodeType == "episodic" {
				// Extrair ID do content
				if idVal, ok := node.Content["id"]; ok {
					if idStr, ok := idVal.(string); ok {
						var id int64
						_, err := fmt.Sscanf(idStr, "%d", &id)
						if err == nil {
							relatedIDs = append(relatedIDs, id)
							count++
							if count >= limit {
								break
							}
						}
					}
				}
			}
		}
	}

	log.Printf("[GRAPH] Busca Recursiva (ID %d): Encontrados %d eventos relacionados", memoryID, len(relatedIDs))
	return relatedIDs, nil
}
