package models

import "time"

type Idoso struct {
	ID                  int64     `json:"id"`
	Nome                string    `json:"nome"`
	DataNascimento      time.Time `json:"data_nascimento"`
	Telefone            string    `json:"telefone"`
	CPF                 string    `json:"cpf"`
	DeviceToken         string    `json:"device_token"`
	Ativo               bool      `json:"ativo"`
	NivelCognitivo      string    `json:"nivel_cognitivo"`
	LimitacoesAuditivas bool      `json:"limitacoes_auditivas"`
	UsaAparelhoAuditivo bool      `json:"usa_aparelho_auditivo"`
	TomVoz              string    `json:"tom_voz"`
	PreferenciaHorario  string    `json:"preferencia_horario_ligacao"`
}
