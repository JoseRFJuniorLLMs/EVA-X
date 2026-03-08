// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"encoding/json"
	"eva/pkg/models"
	"fmt"
	"sync"
	"time"
)

// advisoryMu replaces PostgreSQL pg_advisory_lock for in-process mutual exclusion.
// NietzscheDB does not need advisory locks for our scheduler use case.
var advisoryMu sync.Mutex

// GetIdosoByID recupera os dados do idoso e do familiar principal
func (db *DB) GetIdosoByID(ctx context.Context, id int) (*models.CallContext, error) {
	m, err := db.getNode(ctx, "idosos", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get idoso %d: %w", id, err)
	}
	if m == nil {
		return nil, fmt.Errorf("idoso %d not found", id)
	}

	callCtx := &models.CallContext{
		IdosoID:             int(getInt64(m, "id")),
		IdosoNome:           getString(m, "nome"),
		Telefone:            getString(m, "telefone"),
		NivelCognitivo:      getString(m, "nivel_cognitivo"),
		LimitacoesAuditivas: getBool(m, "limitacoes_auditivas"),
	}

	// Extract familiar_principal JSONB (stored as map or JSON string)
	if fp, ok := m["familiar_principal"]; ok && fp != nil {
		switch v := fp.(type) {
		case map[string]interface{}:
			callCtx.FamiliarNome = getString(v, "nome")
			callCtx.FamiliarTelefone = getString(v, "telefone")
		case string:
			var fpMap map[string]interface{}
			if json.Unmarshal([]byte(v), &fpMap) == nil {
				callCtx.FamiliarNome = getString(fpMap, "nome")
				callCtx.FamiliarTelefone = getString(fpMap, "telefone")
			}
		}
	}

	return callCtx, nil
}

// GetCallContext recupera o contexto completo para uma chamada (agendamento + idoso)
func (db *DB) GetCallContext(ctx context.Context, agendamentoID int) (*models.CallContext, error) {
	// Step 1: fetch the agendamento
	ag, err := db.getNode(ctx, "agendamentos", agendamentoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agendamento %d: %w", agendamentoID, err)
	}
	if ag == nil {
		return nil, fmt.Errorf("agendamento %d not found", agendamentoID)
	}

	// Step 2: fetch the related idoso
	idosoID := getInt64(ag, "idoso_id")
	idoso, err := db.getNode(ctx, "idosos", idosoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get idoso %d for agendamento %d: %w", idosoID, agendamentoID, err)
	}
	if idoso == nil {
		return nil, fmt.Errorf("idoso %d not found for agendamento %d", idosoID, agendamentoID)
	}

	// Step 3: combine fields
	callCtx := &models.CallContext{
		AgendamentoID:       int(getInt64(ag, "id")),
		IdosoID:             int(idosoID),
		IdosoNome:           getString(idoso, "nome"),
		Telefone:            getString(idoso, "telefone"),
		NivelCognitivo:      getString(idoso, "nivel_cognitivo"),
		LimitacoesAuditivas: getBool(idoso, "limitacoes_auditivas"),
		TomVoz:              getString(idoso, "tom_voz"),
		SessionHandle:       getString(ag, "gemini_session_handle"),
		RetryInterval:       int(getInt64(ag, "retry_interval_minutes")),
		Timezone:            getString(idoso, "timezone"),
	}

	// Calculate idade from data_nascimento
	dob := getTime(idoso, "data_nascimento")
	if !dob.IsZero() {
		callCtx.Idade = int(time.Since(dob).Hours() / 24 / 365.25)
	}

	// Extract familiar_principal JSONB
	if fp, ok := idoso["familiar_principal"]; ok && fp != nil {
		switch v := fp.(type) {
		case map[string]interface{}:
			callCtx.FamiliarNome = getString(v, "nome")
			callCtx.FamiliarTelefone = getString(v, "telefone")
		case string:
			var fpMap map[string]interface{}
			if json.Unmarshal([]byte(v), &fpMap) == nil {
				callCtx.FamiliarNome = getString(fpMap, "nome")
				callCtx.FamiliarTelefone = getString(fpMap, "telefone")
			}
		}
	}

	// Parse dados_tarefa for medicamento and persona
	if dt, ok := ag["dados_tarefa"]; ok && dt != nil {
		var dadosTarefa map[string]interface{}
		switch v := dt.(type) {
		case map[string]interface{}:
			dadosTarefa = v
		case string:
			json.Unmarshal([]byte(v), &dadosTarefa)
		}
		if dadosTarefa != nil {
			if med, ok := dadosTarefa["medicamento"].(string); ok {
				callCtx.Medicamento = med
			}
			if persona, ok := dadosTarefa["persona"].(string); ok {
				callCtx.Persona = persona
			}
		}
	}

	return callCtx, nil
}

// AcquireLock obtains an in-process mutex lock (replaces pg_advisory_lock).
// Always returns true since it blocks until the lock is acquired.
func (db *DB) AcquireLock(ctx context.Context, lockID int) (bool, error) {
	advisoryMu.Lock()
	return true, nil
}

// ReleaseLock releases the in-process mutex lock (replaces pg_advisory_unlock).
func (db *DB) ReleaseLock(ctx context.Context, lockID int) (bool, error) {
	advisoryMu.Unlock()
	return true, nil
}

// GetSystemSetting busca uma configuracao pela chave na tabela configuracoes_sistema
func (db *DB) GetSystemSetting(ctx context.Context, key string) (string, error) {
	rows, err := db.queryNodesByLabel(ctx, "configuracoes_sistema",
		" AND n.chave = $chave AND n.ativa = $ativa",
		map[string]interface{}{
			"chave": key,
			"ativa": true,
		}, 1)
	if err != nil {
		return "", fmt.Errorf("failed to query setting %q: %w", key, err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("setting %q not found", key)
	}
	return getString(rows[0], "valor"), nil
}

// GetPendingCalls busca agendamentos prontos para execucao.
// Delegates to GetPendingAgendamentos and enriches with idoso data.
func (db *DB) GetPendingCalls(ctx context.Context) ([]models.Agendamento, error) {
	dbAgendamentos, err := db.GetPendingAgendamentos(50)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending agendamentos: %w", err)
	}

	var result []models.Agendamento
	for _, a := range dbAgendamentos {
		// Fetch idoso data for telefone and nome
		idoso, err := db.getNode(ctx, "idosos", a.IdosoID)
		if err != nil || idoso == nil {
			continue
		}

		ag := models.Agendamento{
			ID:                   int(a.ID),
			IdosoID:              int(a.IdosoID),
			Telefone:             getString(idoso, "telefone"),
			NomeIdoso:            getString(idoso, "nome"),
			Horario:              a.DataHoraAgendada,
			Status:               a.Status,
			TentativasRealizadas: a.TentativasRealizadas,
			MaxRetries:           a.MaxRetries,
			Prioridade:           a.Prioridade,
		}

		// Parse dados_tarefa
		if a.DadosTarefa != "" {
			var dt map[string]interface{}
			if json.Unmarshal([]byte(a.DadosTarefa), &dt) == nil {
				ag.DadosTarefa = dt
				if med, ok := dt["medicamento"].(string); ok {
					ag.Remedios = med
				}
			}
		}

		// Fetch retry_interval_minutes from the raw node
		agNode, _ := db.getNode(ctx, "agendamentos", a.ID)
		if agNode != nil {
			ag.RetryIntervalMinutes = int(getInt64(agNode, "retry_interval_minutes"))
		}

		result = append(result, ag)
	}

	return result, nil
}

// UpdateCallStatus updates the status of an agendamento and related timestamps.
func (db *DB) UpdateCallStatus(ctx context.Context, agendamentoID int, status string, retryInMinutes int) error {
	matchKeys := map[string]interface{}{
		"id": float64(agendamentoID),
	}
	now := time.Now().Format(time.RFC3339)

	updates := map[string]interface{}{
		"status":        status,
		"atualizado_em": now,
	}

	if status == "concluido" {
		updates["data_hora_realizada"] = now
	} else if status == "aguardando_retry" {
		proxima := time.Now().Add(time.Duration(retryInMinutes) * time.Minute)
		updates["proxima_tentativa"] = proxima.Format(time.RFC3339)
	}

	return db.updateFields(ctx, "agendamentos", matchKeys, updates)
}

// IncrementAttempts increments tentativas_realizadas for an agendamento.
func (db *DB) IncrementAttempts(ctx context.Context, agendamentoID int) error {
	// First read current value
	m, err := db.getNode(ctx, "agendamentos", agendamentoID)
	if err != nil {
		return fmt.Errorf("failed to get agendamento %d: %w", agendamentoID, err)
	}
	if m == nil {
		return fmt.Errorf("agendamento %d not found", agendamentoID)
	}

	current := getInt64(m, "tentativas_realizadas")

	return db.updateFields(ctx, "agendamentos",
		map[string]interface{}{"id": float64(agendamentoID)},
		map[string]interface{}{
			"tentativas_realizadas": current + 1,
			"atualizado_em":         time.Now().Format(time.RFC3339),
		})
}

// SaveSessionHandle saves the Gemini session handle and checkpoint for an agendamento.
func (db *DB) SaveSessionHandle(ctx context.Context, agendamentoID int, handle string, checkpoint map[string]interface{}) error {
	cpRaw, _ := json.Marshal(checkpoint)
	expiresAt := time.Now().Add(2 * time.Hour).Format(time.RFC3339)

	return db.updateFields(ctx, "agendamentos",
		map[string]interface{}{"id": float64(agendamentoID)},
		map[string]interface{}{
			"gemini_session_handle":   handle,
			"ultima_interacao_estado": string(cpRaw),
			"session_expires_at":      expiresAt,
			"atualizado_em":           time.Now().Format(time.RFC3339),
		})
}
