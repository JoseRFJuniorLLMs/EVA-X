package database

import (
	"context"
	"time"

	"eva-mind/pkg/models"
)

func (db *DB) CreateHistorico(ctx context.Context, hist *models.Historico) (int, error) {
	query := `
        INSERT INTO historico_ligacoes 
        (agendamento_id, idoso_id, call_sid, status, inicio)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `

	var id int
	err := db.Pool.QueryRow(
		ctx,
		query,
		hist.AgendamentoID,
		hist.IdosoID,
		hist.CallSID,
		hist.Status,
		time.Now(),
	).Scan(&id)

	return id, err
}

func (db *DB) UpdateHistorico(ctx context.Context, id int, updates map[string]interface{}) error {
	// Implementar update dinâmico baseado no map
	query := `
        UPDATE historico_ligacoes
        SET fim = COALESCE($1, fim),
            status = COALESCE($2, status),
            qualidade_audio = COALESCE($3, qualidade_audio),
            interrupcoes_detectadas = COALESCE($4, interrupcoes_detectadas)
        WHERE id = $5
    `

	_, err := db.Pool.Exec(
		ctx,
		query,
		updates["fim"],
		updates["status"],
		updates["qualidade_audio"],
		updates["interrupcoes_detectadas"],
		id,
	)

	return err
}
