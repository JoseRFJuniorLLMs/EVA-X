// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"encoding/json"
	"eva/internal/brainstem/database"
	"fmt"
	"log"
)

type ContextService struct {
	db *database.DB
}

func NewContextService(db *database.DB) *ContextService {
	return &ContextService{db: db}
}

// SaveAnalysis persiste o resultado de um raciocínio (Audio ou Graph)
func (s *ContextService) SaveAnalysis(ctx context.Context, idosoID int64, analysisType string, content string) error {
	// Tenta converter string para JSON se possível, senão salva como string num objeto
	var jsonContent interface{}
	if err := json.Unmarshal([]byte(content), &jsonContent); err != nil {
		// Não é JSON válido, envelopar
		jsonContent = map[string]string{"raw_text": content}
	}

	if err := s.db.CreateGeminiAnalysis(idosoID, analysisType, jsonContent); err != nil {
		return fmt.Errorf("falha ao salvar contexto: %w", err)
	}

	log.Printf("💾 [CONTEXT] Análise '%s' salva para Idoso %d", analysisType, idosoID)
	return nil
}

// GetContextSummary recupera contexto recente formatado para o Prompt
func (s *ContextService) GetContextSummary(ctx context.Context, idosoID int64) (string, error) {
	analyses, err := s.db.GetRecentAnalyses(idosoID, 5) // Pegar últimas 5 análises
	if err != nil {
		return "", err
	}

	if len(analyses) == 0 {
		return "", nil
	}

	summary := "\n=== CONTEXTO RECENTE (FATOS & ANÁLISES) ===\n"
	for _, a := range analyses {
		summary += fmt.Sprintf("- [%s] (%s): %s\n", a.CreatedAt.Format("15:04 02/01"), a.Tipo, string(a.Conteudo))
	}
	summary += "==============================================\n"

	return summary, nil
}
