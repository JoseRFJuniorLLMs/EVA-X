package database

import (
	"context"
	"time"

	"eva-mind/pkg/models"
)

func (db *DB) CreateHistorico(ctx context.Context, hist *models.Historico) (int, error) {
	query := `
        INSERT INTO historico_ligacoes 
        (agendamento_id, idoso_id, twilio_call_sid, inicio_chamada)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `

	var id int
	err := db.Pool.QueryRow(
		ctx,
		query,
		hist.AgendamentoID,
		hist.IdosoID,
		hist.CallSID,
		time.Now(),
	).Scan(&id)

	return id, err
}

func (db *DB) UpdateHistorico(ctx context.Context, id int, updates map[string]interface{}) error {
	query := `
        UPDATE historico_ligacoes
        SET fim_chamada = COALESCE($1, fim_chamada),
            qualidade_audio = COALESCE($2, qualidade_audio),
            interrupcoes_detectadas = COALESCE($3, interrupcoes_detectadas),
            twilio_call_sid = COALESCE($4, twilio_call_sid),
            transcricao_completa = COALESCE($5, transcricao_completa),
            transcricao_resumo = COALESCE($6, transcricao_resumo),
            sentimento_geral = COALESCE($7, sentimento_geral),
            sentimento_intensidade = COALESCE($8, sentimento_intensidade),
            duracao_segundos = COALESCE($9, duracao_segundos)
        WHERE id = $10
    `

	_, err := db.Pool.Exec(
		ctx,
		query,
		updates["fim"],
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
