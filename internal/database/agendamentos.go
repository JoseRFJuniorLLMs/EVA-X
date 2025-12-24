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
            a.remedios,
            i.nivel_cognitivo,
            i.limitacoes_auditivas,
            a.gemini_session_handle,
            a.ultima_interacao_estado
        FROM agendamentos a
        JOIN idosos i ON a.idoso_id = i.id
        WHERE a.id = $1
    `

	var callCtx models.CallContext
	var remediosJSON, estadoJSON, sessionHandle *string
	var limitacoesAuditivas *bool

	err := db.Pool.QueryRow(ctx, query, agendamentoID).Scan(
		&callCtx.AgendamentoID,
		&callCtx.IdosoID,
		&callCtx.IdosoNome,
		&callCtx.Telefone,
		&remediosJSON,
		&callCtx.NivelCognitivo,
		&limitacoesAuditivas,
		&sessionHandle,
		&estadoJSON,
	)

	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if remediosJSON != nil {
		callCtx.Medicamento = *remediosJSON
	}
	if sessionHandle != nil {
		callCtx.SessionHandle = *sessionHandle
	}
	if limitacoesAuditivas != nil {
		callCtx.LimitacoesAuditivas = *limitacoesAuditivas
	}

	return &callCtx, nil
}

func (db *DB) GetPendingCalls(ctx context.Context) ([]models.Agendamento, error) {
	query := `
        SELECT 
            id,
            idoso_id,
            telefone,
            nome_idoso,
            horario,
            remedios,
            status,
            tentativas_realizadas
        FROM agendamentos
        WHERE horario <= NOW()
          AND status = 'pendente'
        ORDER BY horario ASC
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
		var remedios *string

		err := rows.Scan(
			&ag.ID,
			&ag.IdosoID,
			&ag.Telefone,
			&ag.NomeIdoso,
			&ag.Horario,
			&remedios,
			&ag.Status,
			&ag.TentativasRealizadas,
		)
		if err != nil {
			continue
		}

		if remedios != nil {
			ag.Remedios = *remedios
		}

		agendamentos = append(agendamentos, ag)
	}

	return agendamentos, nil
}

func (db *DB) UpdateCallStatus(ctx context.Context, agendamentoID int, status string, callSID *string) error {
	query := `
        UPDATE agendamentos
        SET status = $1,
            call_sid = COALESCE($2, call_sid),
            updated_at = NOW()
        WHERE id = $3
    `

	_, err := db.Pool.Exec(ctx, query, status, callSID, agendamentoID)
	return err
}

func (db *DB) SaveSessionHandle(ctx context.Context, agendamentoID int, handle string, checkpoint map[string]interface{}) error {
	query := `
        UPDATE agendamentos
        SET gemini_session_handle = $1,
            ultima_interacao_estado = $2,
            updated_at = NOW()
        WHERE id = $3
    `

	_, err := db.Pool.Exec(ctx, query, handle, checkpoint, agendamentoID)
	return err
}

func (db *DB) IncrementAttempts(ctx context.Context, agendamentoID int) error {
	query := `
        UPDATE agendamentos
        SET tentativas_realizadas = tentativas_realizadas + 1,
            updated_at = NOW()
        WHERE id = $1
    `
	_, err := db.Pool.Exec(ctx, query, agendamentoID)
	return err
}
