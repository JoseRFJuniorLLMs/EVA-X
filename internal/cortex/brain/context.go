package brain

import (
	"context"
	"database/sql"
	"log"
)

// BuildSystemPrompt constructs the system prompt using Unified Retrieval (RSI)
func (s *Service) BuildSystemPrompt(idosoID int64) string {
	// Delegar para o UnifiedRetrieval (Lacanian + Cognitive Engine)
	// Passamos strings vazias para texto atual/anterior pois é o setup inicial
	prompt, err := s.unifiedRetrieval.GetPromptForGemini(context.Background(), idosoID, "", "")
	if err != nil {
		log.Printf("⚠️ [Brain] Failed to build unified prompt: %v. Using fallback.", err)
		return "VOCÊ É A EVA. Ocorreu um erro ao carregar sua memória completa. Apenas converse naturalmente."
	}
	return prompt
}

// Helper seguro para NullString
func getString(ns sql.NullString, def string) string {
	if ns.Valid && ns.String != "" {
		return ns.String
	}
	return def
}
