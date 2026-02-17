// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Agendamento struct {
	ID                   int                    `json:"id"`
	IdosoID              int                    `json:"idoso_id"`
	Telefone             string                 `json:"telefone"`
	NomeIdoso            string                 `json:"nome_idoso"`
	Horario              time.Time              `json:"horario"` // Mapping to data_hora_agendada
	DataHoraRealizada    *time.Time             `json:"data_hora_realizada,omitempty"`
	Remedios             string                 `json:"remedios"` // Extracted from dados_tarefa
	Status               string                 `json:"status"`
	MaxRetries           int                    `json:"max_retries"`
	RetryIntervalMinutes int                    `json:"retry_interval_minutes"`
	TentativasRealizadas int                    `json:"tentativas_realizadas"`
	ProximaTentativa     *time.Time             `json:"proxima_tentativa,omitempty"`
	EscalationPolicy     string                 `json:"escalation_policy"`
	Prioridade           string                 `json:"prioridade"`
	DeviceToken          string                 `json:"device_token"`
	CallSID              *string                `json:"call_sid,omitempty"`
	GeminiSessionHandle  string                 `json:"gemini_session_handle"`
	SessionExpiresAt     *time.Time             `json:"session_expires_at,omitempty"`
	DadosTarefa          map[string]interface{} `json:"dados_tarefa"`
}

type CallContext struct {
	AgendamentoID       int    `json:"agendamento_id"`
	IdosoID             int    `json:"idoso_id"`
	IdosoNome           string `json:"idoso_nome"`
	Telefone            string `json:"telefone"`
	Medicamento         string `json:"medicamento"`
	NivelCognitivo      string `json:"nivel_cognitivo"`
	LimitacoesAuditivas bool   `json:"limitacoes_auditivas"`
	TomVoz              string `json:"tom_voz"`
	SessionHandle       string `json:"session_handle"`
	RetryInterval       int    `json:"retry_interval"`
	Idade               int    `json:"idade"`
	Timezone            string `json:"timezone"`
	Persona             string `json:"persona"` // ✅ NEW: Active persona name
	FamiliarNome        string `json:"familiar_nome"`
	FamiliarTelefone    string `json:"familiar_telefone"`
}

// Historico representa o log de uma chamada realizada
type Historico struct {
	ID                     int        `json:"id"`
	AgendamentoID          int        `json:"agendamento_id"`
	IdosoID                int        `json:"idoso_id"`
	CallSID                string     `json:"call_sid"`
	Inicio                 time.Time  `json:"inicio"`
	Fim                    *time.Time `json:"fim,omitempty"`
	QualidadeAudio         *float64   `json:"qualidade_audio,omitempty"`
	InterrupcoesDetectadas *int       `json:"interrupcoes_detectadas,omitempty"`
}
