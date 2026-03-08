// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package brain

import (
	"context"
	"log"
)

// BuildSystemPrompt constructs the system prompt using Unified Retrieval (RSI)
func (s *Service) BuildSystemPrompt(idosoID int64) string {
	// Delegar para o UnifiedRetrieval (Lacanian + Cognitive Engine)
	// Passamos strings vazias para texto atual/anterior pois é o setup inicial
	prompt, _, err := s.unifiedRetrieval.GetPromptForGemini(context.Background(), idosoID, "", "")
	if err != nil {
		log.Printf("⚠️ [Brain] Failed to build unified prompt: %v. Using fallback.", err)
		return "VOCE E A EVA. Ocorreu um erro ao carregar sua memoria completa. Apenas converse naturalmente."
	}
	return prompt
}
