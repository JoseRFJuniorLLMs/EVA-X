// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"eva-mind/pkg/models"
)

// GetIdosoByID recupera os dados do idoso e do familiar principal
func (db *DB) GetIdosoByID(ctx context.Context, id int) (*models.CallContext, error) {
	query := `
		SELECT 
			id,
			nome,
			telefone,
			nivel_cognitivo,
			limitacoes_auditivas,
			familiar_principal->>'nome' as familiar_nome,
			familiar_principal->>'telefone' as familiar_telefone
		FROM idosos
		WHERE id = $1
	`

	var callCtx models.CallContext
	err := db.Conn.QueryRowContext(ctx, query, id).Scan(
		&callCtx.IdosoID,
		&callCtx.IdosoNome,
		&callCtx.Telefone,
		&callCtx.NivelCognitivo,
		&callCtx.LimitacoesAuditivas,
		&callCtx.FamiliarNome,
		&callCtx.FamiliarTelefone,
	)

	if err != nil {
		return nil, err
	}

	return &callCtx, nil
}

// GetCallContext recupera o contexto completo para uma chamada (Join Agendamentos + Idosos)
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
            i.familiar_principal->>'nome' as familiar_nome,
            i.familiar_principal->>'telefone' as familiar_telefone,
            a.gemini_session_handle,
            a.retry_interval_minutes,
            EXTRACT(YEAR FROM AGE(i.data_nascimento))::int as idade,
            i.timezone
        FROM agendamentos a
        JOIN idosos i ON a.idoso_id = i.id
        WHERE a.id = $1
    `

	var callCtx models.CallContext
	var dadosTarefaRaw []byte
	var sessionHandle sql.NullString

	err := db.Conn.QueryRowContext(ctx, query, agendamentoID).Scan(
		&callCtx.AgendamentoID,
		&callCtx.IdosoID,
		&callCtx.IdosoNome,
		&callCtx.Telefone,
		&dadosTarefaRaw,
		&callCtx.NivelCognitivo,
		&callCtx.LimitacoesAuditivas,
		&callCtx.TomVoz,
		&callCtx.FamiliarNome,
		&callCtx.FamiliarTelefone,
		&sessionHandle,
		&callCtx.RetryInterval,
		&callCtx.Idade,
		&callCtx.Timezone,
	)

	if err != nil {
		return nil, err
	}

	// Parse JSON dados_tarefa
	var dadosTarefa map[string]interface{}
	if err := json.Unmarshal(dadosTarefaRaw, &dadosTarefa); err == nil {
		if med, ok := dadosTarefa["medicamento"].(string); ok {
			callCtx.Medicamento = med
		}
		if persona, ok := dadosTarefa["persona"].(string); ok {
			callCtx.Persona = persona
		}
	}

	if sessionHandle.Valid {
		callCtx.SessionHandle = sessionHandle.String
	}

	return &callCtx, nil
}

// AcquireLock tenta obter um lock consultivo do Postgres para evitar processamento duplicado
func (db *DB) AcquireLock(ctx context.Context, lockID int) (bool, error) {
	var acquired bool
	query := "SELECT pg_try_advisory_lock($1)"
	err := db.Conn.QueryRowContext(ctx, query, lockID).Scan(&acquired)
	return acquired, err
}

// ReleaseLock libera o lock consultivo
func (db *DB) ReleaseLock(ctx context.Context, lockID int) (bool, error) {
	var released bool
	query := "SELECT pg_advisory_unlock($1)"
	err := db.Conn.QueryRowContext(ctx, query, lockID).Scan(&released)
	return released, err
}

// GetSystemSetting busca uma configuração pela chave na tabela configuracoes_sistema
func (db *DB) GetSystemSetting(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT valor FROM configuracoes_sistema WHERE chave = $1 AND ativa = true`
	err := db.Conn.QueryRowContext(ctx, query, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetPendingCalls busca agendamentos prontos para execução
func (db *DB) GetPendingCalls(ctx context.Context) ([]models.Agendamento, error) {
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

	rows, err := db.Conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agendamentos []models.Agendamento
	for rows.Next() {
		var ag models.Agendamento
		var dadosTarefaRaw []byte
		err := rows.Scan(
			&ag.ID,
			&ag.IdosoID,
			&ag.Telefone,
			&ag.NomeIdoso,
			&ag.Horario,
			&dadosTarefaRaw,
			&ag.Status,
			&ag.TentativasRealizadas,
			&ag.MaxRetries,
			&ag.RetryIntervalMinutes,
			&ag.Prioridade,
		)
		if err != nil {
			continue
		}

		// Unmarshal dadosTarefa
		json.Unmarshal(dadosTarefaRaw, &ag.DadosTarefa)

		if med, ok := ag.DadosTarefa["medicamento"].(string); ok {
			ag.Remedios = med
		}

		agendamentos = append(agendamentos, ag)
	}

	return agendamentos, nil
}

func (db *DB) UpdateCallStatus(ctx context.Context, agendamentoID int, status string, retryInMinutes int) error {
	var query string
	var err error

	if status == "concluido" {
		query = `UPDATE agendamentos SET status = $1, data_hora_realizada = NOW(), atualizado_em = NOW() WHERE id = $2`
		_, err = db.Conn.ExecContext(ctx, query, status, agendamentoID)
	} else if status == "aguardando_retry" {
		query = `UPDATE agendamentos SET status = $1, proxima_tentativa = NOW() + ($2 || ' minutes')::interval, atualizado_em = NOW() WHERE id = $3`
		_, err = db.Conn.ExecContext(ctx, query, status, retryInMinutes, agendamentoID)
	} else {
		query = `UPDATE agendamentos SET status = $1, atualizado_em = NOW() WHERE id = $2`
		_, err = db.Conn.ExecContext(ctx, query, status, agendamentoID)
	}

	return err
}

func (db *DB) IncrementAttempts(ctx context.Context, agendamentoID int) error {
	query := `UPDATE agendamentos SET tentativas_realizadas = tentativas_realizadas + 1, atualizado_em = NOW() WHERE id = $1`
	_, err := db.Conn.ExecContext(ctx, query, agendamentoID)
	return err
}

func (db *DB) SaveSessionHandle(ctx context.Context, agendamentoID int, handle string, checkpoint map[string]interface{}) error {
	cpRaw, _ := json.Marshal(checkpoint)
	query := `UPDATE agendamentos SET gemini_session_handle = $1, ultima_interacao_estado = $2, session_expires_at = NOW() + INTERVAL '2 hours', atualizado_em = NOW() WHERE id = $3`
	_, err := db.Conn.ExecContext(ctx, query, handle, cpRaw, agendamentoID)
	return err
}

// Reimplementação de métodos adicionais necessários para o scheduler
