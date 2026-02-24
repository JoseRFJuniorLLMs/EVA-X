// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"
)

type GeminiAnalysis struct {
	ID        int64           `json:"id"`
	IdosoID   int64           `json:"idoso_id"`
	Tipo      string          `json:"tipo"` // "AUDIO" | "GRAPH" | "TEXT"
	Conteudo  json.RawMessage `json:"conteudo"`
	CreatedAt time.Time       `json:"created_at"`
}

func contentToGeminiAnalysis(m map[string]interface{}) GeminiAnalysis {
	a := GeminiAnalysis{
		ID:        getInt64(m, "id"),
		IdosoID:   getInt64(m, "idoso_id"),
		Tipo:      getString(m, "tipo"),
		CreatedAt: getTime(m, "created_at"),
	}
	// conteudo may be stored as a string (JSON) or a map
	if raw, ok := m["conteudo"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			a.Conteudo = json.RawMessage(v)
		default:
			if b, err := json.Marshal(v); err == nil {
				a.Conteudo = b
			}
		}
	}
	return a
}

// CreateGeminiAnalysis salva uma nova analise no NietzscheDB
func (db *DB) CreateGeminiAnalysis(idosoID int64, tipo string, conteudo interface{}) error {
	ctx := context.Background()

	conteudoJSON, err := json.Marshal(conteudo)
	if err != nil {
		return fmt.Errorf("erro ao marshalar conteudo da analise: %w", err)
	}

	_, err = db.insertRow(ctx, "analise_gemini", map[string]interface{}{
		"idoso_id":   idosoID,
		"tipo":       tipo,
		"conteudo":   string(conteudoJSON),
		"created_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("erro ao salvar analise gemini: %w", err)
	}

	log.Printf("[NIETZSCHE] Analise salva: Idoso=%d, Tipo=%s, Tamanho=%d bytes", idosoID, tipo, len(conteudoJSON))
	return nil
}

// GetRecentAnalyses recupera as ultimas N analises de um idoso para contexto
func (db *DB) GetRecentAnalyses(idosoID int64, limit int) ([]GeminiAnalysis, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "analise_gemini",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar analises recentes: %w", err)
	}

	log.Printf("[NIETZSCHE] Buscando analises recentes: Idoso=%d, Limit=%d", idosoID, limit)

	var analyses []GeminiAnalysis
	for _, m := range rows {
		analyses = append(analyses, contentToGeminiAnalysis(m))
	}

	// Sort by created_at DESC
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].CreatedAt.After(analyses[j].CreatedAt)
	})

	if limit > 0 && len(analyses) > limit {
		analyses = analyses[:limit]
	}

	return analyses, nil
}
