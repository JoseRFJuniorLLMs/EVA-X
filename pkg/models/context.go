package models

import "time"

type Agendamento struct {
	ID                   int       `json:"id"`
	IdosoID              int       `json:"idoso_id"`
	Telefone             string    `json:"telefone"`
	NomeIdoso            string    `json:"nome_idoso"`
	Horario              time.Time `json:"horario"`
	Remedios             string    `json:"remedios"`
	Status               string    `json:"status"`
	TentativasRealizadas int       `json:"tentativas_realizadas"`
	CallSID              *string   `json:"call_sid,omitempty"`
}

type CallContext struct {
	AgendamentoID       int    `json:"agendamento_id"`
	IdosoID             int    `json:"idoso_id"`
	IdosoNome           string `json:"idoso_nome"`
	Telefone            string `json:"telefone"`
	Medicamento         string `json:"medicamento"`
	NivelCognitivo      string `json:"nivel_cognitivo"`
	LimitacoesAuditivas bool   `json:"limitacoes_auditivas"`
	SessionHandle       string `json:"session_handle"`
}

// Historico representa o log de uma chamada realizada
type Historico struct {
	ID                     int        `json:"id"`
	AgendamentoID          int        `json:"agendamento_id"`
	IdosoID                int        `json:"idoso_id"`
	CallSID                string     `json:"call_sid"`
	Status                 string     `json:"status"`
	Inicio                 time.Time  `json:"inicio"`
	Fim                    *time.Time `json:"fim,omitempty"`
	QualidadeAudio         *float64   `json:"qualidade_audio,omitempty"`
	InterrupcoesDetectadas *int       `json:"interrupcoes_detectadas,omitempty"`
}
