package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type GeminiAnalysis struct {
	ID        int64           `json:"id"`
	IdosoID   int64           `json:"idoso_id"`
	Tipo      string          `json:"tipo"` // "AUDIO" | "GRAPH" | "TEXT"
	Conteudo  json.RawMessage `json:"conteudo"`
	CreatedAt time.Time       `json:"created_at"`
}

// CreateGeminiAnalysis salva uma nova análise no banco de dados
func (db *DB) CreateGeminiAnalysis(idosoID int64, tipo string, conteudo interface{}) error {
	conteudoJSON, err := json.Marshal(conteudo)
	if err != nil {
		return fmt.Errorf("erro ao marshalar conteudo da analise: %w", err)
	}

	query := `
		INSERT INTO analise_gemini (idoso_id, tipo, conteudo, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
	`
	_, err = db.Conn.Exec(query, idosoID, tipo, conteudoJSON)
	if err != nil {
		return fmt.Errorf("erro ao salvar analise gemini: %w", err)
	}

	log.Printf("📥 [POSTGRES] Análise salva: Idoso=%d, Tipo=%s, Tamanho=%d bytes", idosoID, tipo, len(conteudoJSON))
	return nil
}

// GetRecentAnalyses recupera as últimas N análises de um idoso para contexto
func (db *DB) GetRecentAnalyses(idosoID int64, limit int) ([]GeminiAnalysis, error) {
	query := `
		SELECT id, idoso_id, tipo, conteudo, created_at
		FROM analise_gemini
		WHERE idoso_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := db.Conn.Query(query, idosoID, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar analises recentes: %w", err)
	}
	defer rows.Close()

	log.Printf("🔍 [POSTGRES] Buscando análises recentes: Idoso=%d, Limit=%d", idosoID, limit)

	var analyses []GeminiAnalysis
	for rows.Next() {
		var a GeminiAnalysis
		var conteudoBytes []byte
		if err := rows.Scan(&a.ID, &a.IdosoID, &a.Tipo, &conteudoBytes, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Conteudo = json.RawMessage(conteudoBytes)
		analyses = append(analyses, a)
	}
	return analyses, nil
}
