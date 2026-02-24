// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"database/sql"
	"eva/pkg/crypto"
	"fmt"
	"log"
	"sort"
	"time"
)

type Agendamento struct {
	ID                   int64
	IdosoID              int64
	Tipo                 string
	DataHoraAgendada     time.Time
	DataHoraRealizada    *time.Time
	Status               string
	Prioridade           string
	DadosTarefa          string
	MaxRetries           int
	TentativasRealizadas int
}

type Idoso struct {
	ID                  int64
	Nome                string
	DataNascimento      time.Time
	Telefone            string
	CPF                 string
	DeviceToken         string
	Ativo               bool
	NivelCognitivo      string
	LimitacoesAuditivas sql.NullBool
	UsaAparelhoAuditivo sql.NullBool
	TomVoz              string
	PreferenciaHorario  string
}

func contentToAgendamento(m map[string]interface{}) Agendamento {
	return Agendamento{
		ID:                   getInt64(m, "id"),
		IdosoID:              getInt64(m, "idoso_id"),
		Tipo:                 getString(m, "tipo"),
		DataHoraAgendada:     getTime(m, "data_hora_agendada"),
		DataHoraRealizada:    getTimePtr(m, "data_hora_realizada"),
		Status:               getString(m, "status"),
		Prioridade:           getString(m, "prioridade"),
		DadosTarefa:          getString(m, "dados_tarefa"),
		MaxRetries:           int(getInt64(m, "max_retries")),
		TentativasRealizadas: int(getInt64(m, "tentativas_realizadas")),
	}
}

func contentToIdoso(m map[string]interface{}) *Idoso {
	idoso := &Idoso{
		ID:                  getInt64(m, "id"),
		Nome:                getString(m, "nome"),
		DataNascimento:      getTime(m, "data_nascimento"),
		Telefone:            getString(m, "telefone"),
		CPF:                 getString(m, "cpf"),
		DeviceToken:         getString(m, "device_token"),
		Ativo:               getBool(m, "ativo"),
		NivelCognitivo:      getString(m, "nivel_cognitivo"),
		LimitacoesAuditivas: getNullBool(m, "limitacoes_auditivas"),
		UsaAparelhoAuditivo: getNullBool(m, "usa_aparelho_auditivo"),
		TomVoz:              getString(m, "tom_voz"),
		PreferenciaHorario:  getString(m, "preferencia_horario_ligacao"),
	}
	// LGPD Art. 46: decrypt sensitive fields
	idoso.Nome = crypto.Decrypt(idoso.Nome)
	idoso.CPF = crypto.Decrypt(idoso.CPF)
	idoso.Telefone = crypto.Decrypt(idoso.Telefone)
	return idoso
}

func (db *DB) GetPendingAgendamentos(limit int) ([]Agendamento, error) {
	ctx := context.Background()
	now := time.Now()

	rows, err := db.queryNodesByLabel(ctx, "agendamentos",
		` AND n.status = $status`, map[string]interface{}{
			"status": "agendado",
		}, 0) // fetch all matching, filter + sort in Go
	if err != nil {
		return nil, fmt.Errorf("failed to query agendamentos: %w", err)
	}

	var agendamentos []Agendamento
	for _, m := range rows {
		a := contentToAgendamento(m)
		if !a.DataHoraAgendada.After(now) {
			agendamentos = append(agendamentos, a)
		}
	}

	sort.Slice(agendamentos, func(i, j int) bool {
		return agendamentos[i].DataHoraAgendada.Before(agendamentos[j].DataHoraAgendada)
	})

	if limit > 0 && len(agendamentos) > limit {
		agendamentos = agendamentos[:limit]
	}

	return agendamentos, nil
}

// GetPendingAgendamentosByIdoso retorna agendamentos pendentes filtrados por idoso_id
func (db *DB) GetPendingAgendamentosByIdoso(idosoID int64, limit int) ([]Agendamento, error) {
	ctx := context.Background()
	now := time.Now()

	rows, err := db.queryNodesByLabel(ctx, "agendamentos",
		` AND n.status = $status AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"status":   "agendado",
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query agendamentos by idoso: %w", err)
	}

	var agendamentos []Agendamento
	for _, m := range rows {
		a := contentToAgendamento(m)
		if !a.DataHoraAgendada.After(now) {
			agendamentos = append(agendamentos, a)
		}
	}

	sort.Slice(agendamentos, func(i, j int) bool {
		return agendamentos[i].DataHoraAgendada.Before(agendamentos[j].DataHoraAgendada)
	})

	if limit > 0 && len(agendamentos) > limit {
		agendamentos = agendamentos[:limit]
	}

	return agendamentos, nil
}

func (db *DB) GetIdoso(id int64) (*Idoso, error) {
	ctx := context.Background()
	m, err := db.getNode(ctx, "idosos", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("idoso not found")
	}
	return contentToIdoso(m), nil
}

func (db *DB) UpdateAgendamentoStatus(id int64, status string) error {
	ctx := context.Background()
	err := db.updateFields(ctx, "agendamentos",
		map[string]interface{}{"id": float64(id)},
		map[string]interface{}{
			"status":        status,
			"atualizado_em": time.Now().Format(time.RFC3339),
		})
	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}
	return nil
}

func (db *DB) GetIdosoByCPF(cpf string) (*Idoso, error) {
	ctx := context.Background()

	// LGPD Art. 46: lookup via cpf_hash (SHA-256)
	cpfHash := crypto.HashCPF(cpf)

	rows, err := db.queryNodesByLabel(ctx, "idosos",
		` AND n.cpf_hash = $hash`, map[string]interface{}{
			"hash": cpfHash,
		}, 1)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar CPF: %w", err)
	}

	// Filter active only in Go (NQL boolean matching may vary)
	filtered := rows[:0]
	for _, m := range rows {
		if getBool(m, "ativo") {
			filtered = append(filtered, m)
		}
	}
	rows = filtered

	// Fallback: query all active idosos and match stripped CPF digits
	if len(rows) == 0 {
		allRows, err := db.queryNodesByLabel(ctx, "idosos", "", nil, 0)
		if err != nil {
			return nil, fmt.Errorf("erro ao consultar CPF (fallback): %w", err)
		}
		strippedCPF := stripNonDigits(cpf)
		for _, m := range allRows {
			if !getBool(m, "ativo") {
				continue
			}
			storedCPF := crypto.Decrypt(getString(m, "cpf"))
			if stripNonDigits(storedCPF) == strippedCPF {
				rows = append(rows, m)
				break
			}
		}
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("idoso nao encontrado ou inativo")
	}

	idoso := contentToIdoso(rows[0])

	maskedCPF := "***"
	if len(cpf) >= 3 {
		maskedCPF = "***" + cpf[len(cpf)-3:]
	}
	log.Printf("[NIETZSCHE] CPF consultado: %s -> ID: %d", maskedCPF, idoso.ID)
	return idoso, nil
}
