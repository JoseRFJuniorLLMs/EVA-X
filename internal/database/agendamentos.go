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
            a.gemini_session_handle
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
		&sessionHandle,
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
	// 🔍 DEBUG: Verificar se o Go enxerga a tabela
	var total int
	_ = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM agendamentos").Scan(&total)
	println("\n--- DEBUG DATABASE ---")
	println("Total de registros na tabela agendamentos:", total)
	println("----------------------\n")

	query := `
        SELECT 
            a.id,
            a.idoso_id,
            i.telefone,
            i.nome as nome_idoso,
            a.data_hora_agendada,
            a.dados_tarefa,
            a.status,
            a.tentativas_realizadas
        FROM agendamentos a
        JOIN idosos i ON a.idoso_id = i.id
        WHERE a.status IN ('agendado', 'pendente', 'em_andamento')
        ORDER BY a.data_hora_agendada ASC
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
		var dadosTarefa map[string]interface{}

		err := rows.Scan(
			&ag.ID,
			&ag.IdosoID,
			&ag.Telefone,
			&ag.NomeIdoso,
			&ag.Horario,
			&dadosTarefa,
			&ag.Status,
			&ag.TentativasRealizadas,
		)
		if err != nil {
			// ✅ Log de erro crítico para depuração
			println("Erro ao ler linha do banco:", err.Error())
			continue
		}

		// Extrai medicamento
		if med, ok := dadosTarefa["medicamento"].(string); ok {
			ag.Remedios = med
		} else if med, ok := dadosTarefa["remedios"].(string); ok {
			ag.Remedios = med
		}

		agendamentos = append(agendamentos, ag)
	}

	return agendamentos, nil
}

func (db *DB) UpdateCallStatus(ctx context.Context, agendamentoID int, status string, callSID *string) error {
	query := `
        UPDATE agendamentos
        SET status = $1,
            gemini_session_handle = COALESCE($2, gemini_session_handle),
            updated_at = NOW()
        WHERE id = $3
    `
	// Note: eva-v7 uses twilio_call_sid in historico, but let's see if agendamentos has it.
	// In eva-v7 agendamentos, there is no call_sid column!
	// We'll update only status and handle here.

	_, err := db.Pool.Exec(ctx, query, status, nil, agendamentoID)
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
