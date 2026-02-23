// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

import (
	"context"
	"fmt"
	"math"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// PersonalityState representa o estado emocional da relação EVA <-> Idoso
type PersonalityState struct {
	IdosoID            int64     `json:"idoso_id"`
	RelationshipLevel  int       `json:"relationship_level"` // 1-10
	ConversationCount  int       `json:"conversation_count"`
	LastInteraction    time.Time `json:"last_interaction"`
	DominantEmotion    string    `json:"dominant_emotion"`
	FavoriteTopics     []string  `json:"favorite_topics"`
	FirstMeetingDate   time.Time `json:"first_meeting_date"`
	DaysSinceFirstMeet int       `json:"days_since_first_meet"`
}

// PersonalityService gerencia o estado de personalidade via NietzscheDB
type PersonalityService struct {
	client *nietzscheInfra.GraphAdapter
}

// NewPersonalityService cria um novo serviço com NietzscheDB
func NewPersonalityService(client *nietzscheInfra.GraphAdapter) *PersonalityService {
	return &PersonalityService{client: client}
}

// GetState recupera o estado de personalidade de um idoso do Grafo
func (p *PersonalityService) GetState(ctx context.Context, idosoID int64) (*PersonalityState, error) {
	if p.client == nil {
		return nil, fmt.Errorf("nietzsche client not initialized")
	}

	opts := nietzscheInfra.MergeNodeOpts{
		NodeType:  "PersonalityState",
		MatchKeys: map[string]interface{}{"idoso_id": idosoID},
		OnCreateSet: map[string]interface{}{
			"relationship_level": 1,
			"conversation_count": 0,
			"last_interaction":   nietzscheInfra.NowUnix(),
			"dominant_emotion":   "neutro",
			"favorite_topics":    []string{},
			"first_meeting_date": nietzscheInfra.NowUnix(),
		},
	}

	result, err := p.client.MergeNode(ctx, opts)
	if err != nil {
		return nil, err
	}

	state := &PersonalityState{
		IdosoID: idosoID,
	}

	// Helper to extract fields from result.Content
	c := result.Content
	if v, ok := c["relationship_level"].(float64); ok {
		state.RelationshipLevel = int(v)
	}
	if v, ok := c["conversation_count"].(float64); ok {
		state.ConversationCount = int(v)
	}
	if v, ok := c["last_interaction"].(float64); ok {
		state.LastInteraction = time.Unix(int64(v), 0)
	}
	if v, ok := c["dominant_emotion"].(string); ok {
		state.DominantEmotion = v
	}
	if v, ok := c["favorite_topics"].([]interface{}); ok {
		for _, topic := range v {
			if t, ok := topic.(string); ok {
				state.FavoriteTopics = append(state.FavoriteTopics, t)
			}
		}
	}
	if v, ok := c["first_meeting_date"].(float64); ok {
		state.FirstMeetingDate = time.Unix(int64(v), 0)
	}

	state.DaysSinceFirstMeet = int(time.Since(state.FirstMeetingDate).Hours() / 24)

	return state, nil
}

// UpdateAfterConversation atualiza o estado após uma conversa
func (p *PersonalityService) UpdateAfterConversation(ctx context.Context, idosoID int64, detectedEmotion string, topics []string) error {
	state, err := p.GetState(ctx, idosoID)
	if err != nil {
		return err
	}

	newCount := state.ConversationCount + 1
	newLevel := CalculateRelationshipLevel(newCount)

	// Update in NietzscheDB
	_, err = p.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "PersonalityState",
		MatchKeys: map[string]interface{}{"idoso_id": idosoID},
		OnMatchSet: map[string]interface{}{
			"conversation_count": newCount,
			"last_interaction":   nietzscheInfra.NowUnix(),
			"relationship_level": newLevel,
			"dominant_emotion":   detectedEmotion,
		},
	})
	if err != nil {
		return err
	}

	// Update topics
	if len(topics) > 0 {
		return p.updateFavoriteTopics(ctx, idosoID, state.FavoriteTopics, topics)
	}

	return nil
}

// updateFavoriteTopics atualiza os tópicos favoritos (mantém top 5)
func (p *PersonalityService) updateFavoriteTopics(ctx context.Context, idosoID int64, current []string, newTopics []string) error {
	merged := append(current, newTopics...)
	unique := uniqueStrings(merged)

	if len(unique) > 5 {
		unique = unique[:5]
	}

	_, err := p.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "PersonalityState",
		MatchKeys: map[string]interface{}{"idoso_id": idosoID},
		OnMatchSet: map[string]interface{}{
			"favorite_topics": unique,
		},
	})
	return err
}

// GetDaysSinceLastInteraction retorna quantos dias desde última conversa
func (p *PersonalityService) GetDaysSinceLastInteraction(ctx context.Context, idosoID int64) (int, error) {
	state, err := p.GetState(ctx, idosoID)
	if err != nil {
		return 0, err
	}
	return int(time.Since(state.LastInteraction).Hours() / 24), nil
}

// CalculateRelationshipLevel calcula nível baseado em número de conversas
func CalculateRelationshipLevel(conversations int) int {
	if conversations == 0 {
		return 1
	}
	level := int(math.Log2(float64(conversations)) + 1)
	if level > 10 {
		return 10
	}
	if level < 1 {
		return 1
	}
	return level
}

// GetRelationshipStyle retorna o estilo de tratamento baseado no nível
func GetRelationshipStyle(level int) string {
	switch {
	case level <= 2:
		return "formal"
	case level <= 5:
		return "friendly"
	case level <= 8:
		return "intimate"
	default:
		return "family"
	}
}

// GetRelationshipLabel retorna label descritiva
func GetRelationshipLabel(level int) string {
	labels := map[int]string{
		1:  "Nos conhecendo",
		2:  "Conhecidas",
		3:  "Amigas",
		4:  "Boas amigas",
		5:  "Amigas próximas",
		6:  "Confidentes",
		7:  "Muito próximas",
		8:  "Inseparáveis",
		9:  "Como família",
		10: "Família do coração",
	}
	return labels[level]
}

func uniqueStrings(arr []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range arr {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
