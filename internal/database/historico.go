package database

import (
	"context"
	"time"

	"eva-mind/pkg/models"
)

func (db *DB) CreateHistorico(ctx context.Context, hist *models.Historico) (int, error) {
	query := `
        INSERT INTO historico_ligacoes 
        (agendamento_id, idoso_id, twilio_call_sid, status, inicio_chamada)
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
	query := `
        UPDATE historico_ligacoes
        SET fim_chamada = COALESCE($1, fim_chamada),
            status = COALESCE($2, status),
            qualidade_audio = COALESCE($3, qualidade_audio),
            interrupcoes_detectadas = COALESCE($4, interrupcoes_detectadas),
            twilio_call_sid = COALESCE($5, twilio_call_sid),
            transcricao_completa = COALESCE($6, transcricao_completa),
            transcricao_resumo = COALESCE($7, transcricao_resumo),
            sentimento_geral = COALESCE($8, sentimento_geral),
            sentimento_intensidade = COALESCE($9, sentimento_intensidade),
            duracao_segundos = COALESCE($10, duracao_segundos)
        WHERE id = $11
    `

	_, err := db.Pool.Exec(
		ctx,
		query,
		updates["fim"],
		updates["status"],
		updates["qualidade_audio"],
		updates["interrupcoes_detectadas"],
		updates["call_sid"],
		updates["transcricao_completa"],
		updates["transcricao_resumo"],
		updates["sentimento_geral"],
		updates["sentimento_intensidade"],
		updates["duracao_segundos"],
		id,
	)

	return err
}
