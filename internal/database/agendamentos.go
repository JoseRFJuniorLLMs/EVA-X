package database

import (
	"context"

	"eva-mind/pkg/models"
)

func (db *DB) GetCallContext(ctx context.Context, agendamentoID int) (*models.CallContext, error) {
	query := `
        SELECT 
            a.id,
            a.idoso_id,
            i.nome as idoso_nome,
            i.telefone,
            a.dados_tarefa,
            i.nivel_cognitivo,
            i.limitacoes_auditivas,
            i.tom_voz,
            a.gemini_session_handle,
            a.retry_interval_minutes,
            EXTRACT(YEAR FROM AGE(i.data_nascimento))::int as idade,
            i.timezone
        FROM agendamentos a
        JOIN idosos i ON a.idoso_id = i.id
        WHERE a.id = $1
    `

	var callCtx models.CallContext
	var dadosTarefa map[string]interface{}
	var sessionHandle *string

	err := db.Pool.QueryRow(ctx, query, agendamentoID).Scan(
		&callCtx.AgendamentoID,
		&callCtx.IdosoID,
		&callCtx.IdosoNome,
		&callCtx.Telefone,
		&dadosTarefa,
		&callCtx.NivelCognitivo,
		&callCtx.LimitacoesAuditivas,
		&callCtx.TomVoz,
		&sessionHandle,
		&callCtx.RetryInterval,
		&callCtx.Idade,
		&callCtx.Timezone,
	)

	if err != nil {
		return nil, err
	}

	// Extrai medicamento do JSON dados_tarefa
	if med, ok := dadosTarefa["medicamento"].(string); ok {
		callCtx.Medicamento = med
	} else if med, ok := dadosTarefa["remedios"].(string); ok { // Fallback
		callCtx.Medicamento = med
	}

	if sessionHandle != nil {
		callCtx.SessionHandle = *sessionHandle
	}

	return &callCtx, nil
}

func (db *DB) GetPendingCalls(ctx context.Context) ([]models.Agendamento, error) {
	// Limpeza dos logs de depuração agora que o problema foi identificado

	query := `
        SELECT 
            a.id,
            a.idoso_id,
            i.telefone,
            i.nome as nome_idoso,
            a.data_hora_agendada,
            a.dados_tarefa,
            a.status,
            a.tentativas_realizadas,
            a.max_retries,
            a.retry_interval_minutes,
            a.prioridade
        FROM agendamentos a
        JOIN idosos i ON a.idoso_id = i.id
        WHERE (a.data_hora_agendada <= (NOW() + INTERVAL '1 minute') OR (a.proxima_tentativa IS NOT NULL AND a.proxima_tentativa <= NOW()))
          AND a.status IN ('agendado', 'pendente', 'aguardando_retry')
          AND a.tentativas_realizadas < a.max_retries
        ORDER BY 
            CASE a.prioridade 
                WHEN 'alta' THEN 1 
                WHEN 'normal' THEN 2 
                WHEN 'baixa' THEN 3 
            END ASC, 
            a.data_hora_agendada ASC
        LIMIT 50
    `

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agendamentos []models.Agendamento
	for rows.Next() {
		var ag models.Agendamento
		err := rows.Scan(
			&ag.ID,
			&ag.IdosoID,
			&ag.Telefone,
			&ag.NomeIdoso,
			&ag.Horario,
			&ag.DadosTarefa,
			&ag.Status,
			&ag.TentativasRealizadas,
			&ag.MaxRetries,
			&ag.RetryIntervalMinutes,
			&ag.Prioridade,
		)
		if err != nil {
			println("Erro ao ler linha do banco:", err.Error())
			continue
		}

		// Extrai medicamento do JSON
		if med, ok := ag.DadosTarefa["medicamento"].(string); ok {
			ag.Remedios = med
		} else if med, ok := ag.DadosTarefa["remedios"].(string); ok {
			ag.Remedios = med
		}

		agendamentos = append(agendamentos, ag)
	}

	return agendamentos, nil
}

func (db *DB) UpdateCallStatus(ctx context.Context, agendamentoID int, status string, retryInMinutes int) error {
	var query string
	var args []interface{}

	if status == "concluido" {
		query = `
            UPDATE agendamentos
            SET status = $1,
                data_hora_realizada = NOW(),
                atualizado_em = NOW()
            WHERE id = $2
        `
		args = []interface{}{status, agendamentoID}
	} else if status == "aguardando_retry" {
		query = `
            UPDATE agendamentos
            SET status = $1,
                proxima_tentativa = NOW() + ($2 || ' minutes')::interval,
                atualizado_em = NOW()
            WHERE id = $3
        `
		args = []interface{}{status, retryInMinutes, agendamentoID}
	} else {
		query = `
            UPDATE agendamentos
            SET status = $1,
                atualizado_em = NOW()
            WHERE id = $2
        `
		args = []interface{}{status, agendamentoID}
	}

	_, err := db.Pool.Exec(ctx, query, args...)
	return err
}

func (db *DB) SaveSessionHandle(ctx context.Context, agendamentoID int, handle string, checkpoint map[string]interface{}) error {
	query := `
        UPDATE agendamentos
        SET gemini_session_handle = $1,
            ultima_interacao_estado = $2,
            session_expires_at = NOW() + INTERVAL '2 hours',
            atualizado_em = NOW()
        WHERE id = $3
    `

	_, err := db.Pool.Exec(ctx, query, handle, checkpoint, agendamentoID)
	return err
}

func (db *DB) IncrementAttempts(ctx context.Context, agendamentoID int) error {
	query := `
        UPDATE agendamentos
        SET tentativas_realizadas = tentativas_realizadas + 1,
            atualizado_em = NOW()
        WHERE id = $1
    `
	_, err := db.Pool.Exec(ctx, query, agendamentoID)
	return err
}
