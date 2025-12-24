package models

import "time"

type CallContext struct {
    AgendamentoID        int       `json:"agendamento_id"`
    IdosoID              int       `json:"idoso_id"`
    IdosoNome            string    `json:"idoso_nome"`
    Telefone             string    `json:"telefone"`
    Medicamento          string    `json:"medicamento"`
    NivelCognitivo       string    `json:"nivel_cognitivo"`
    LimitacoesAuditivas  bool      `json:"limitacoes_auditivas"`
    SessionHandle        string    `json:"session_handle,omitempty"`
    CheckpointState      string    `json:"checkpoint_state,omitempty"`
}

type Agendamento struct {
    ID                    int       `json:"id"`
    IdosoID               int       `json:"idoso_id"`
    Telefone              string    `json:"telefone"`
    NomeIdoso             string    `json:"nome_idoso"`
    Horario               time.Time `json:"horario"`
    Remedios              string    `json:"remedios"`
    Status                string    `json:"status"`
    TentativasRealizadas  int       `json:"tentativas_realizadas"`
}

type Historico struct {
    ID            int       `json:"id"`
    AgendamentoID int       `json:"agendamento_id"`
    IdosoID       int       `json:"idoso_id"`
    CallSID       string    `json:"call_sid"`
    Status        string    `json:"status"`
    Inicio        time.Time `json:"inicio"`
}
