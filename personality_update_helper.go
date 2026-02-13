package main

import (
	"context"
	"log"
	"time"
)

// updatePersonalityAfterConversation atualiza estado de personalidade após uma conversa
func (s *SignalingServer) updatePersonalityAfterConversation(idosoID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Buscar última emoção detectada nas memórias recentes
	memories, err := s.memoryStore.GetRecent(ctx, idosoID, 5)
	if err != nil {
		log.Printf("⚠️ [PERSONALITY] Erro ao buscar memórias: %v", err)
		return
	}

	// Detectar emoção dominante
	emotionCounts := make(map[string]int)
	var topics []string

	for _, mem := range memories {
		if mem.Emotion != "" {
			emotionCounts[mem.Emotion]++
		}
		topics = append(topics, mem.Topics...)
	}

	// Pegar emoção mais frequente
	dominantEmotion := "neutro"
	maxCount := 0
	for emotion, count := range emotionCounts {
		if count > maxCount {
			maxCount = count
			dominantEmotion = emotion
		}
	}

	// Atualizar estado de personalidade
	err = s.personalityService.UpdateAfterConversation(ctx, idosoID, dominantEmotion, topics)
	if err != nil {
		log.Printf("❌ [PERSONALITY] Erro ao atualizar: %v", err)
		return
	}

	// Buscar estado atualizado para log
	state, err := s.personalityService.GetState(ctx, idosoID)
	if err == nil {
		log.Printf("🧠 [PERSONALITY] Atualizado: Idoso %d - Nível %d/10 (%d conversas) - Emoção: %s",
			idosoID,
			state.RelationshipLevel,
			state.ConversationCount,
			state.DominantEmotion,
		)
	}
}
