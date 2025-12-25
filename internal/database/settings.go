package database

import (
	"context"
)

// GetSystemSetting busca uma configuração pela chave na tabela configuracoes_sistema
func (db *DB) GetSystemSetting(ctx context.Context, key string) (string, error) {
	var value string
	// Schema: configuracoes_sistema(chave, valor)
	query := `SELECT valor FROM configuracoes_sistema WHERE chave = $1 AND ativa = true`
	err := db.Pool.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// ConfirmMedication atualiza o status de medicamento de um agendamento
func (db *DB) ConfirmMedication(ctx context.Context, agendamentoID int, tomou bool) error {
	query := `
		UPDATE agendamentos 
		SET medicamento_tomado = $2, 
		    medicamento_confirmado_em = NOW(),
			status = CASE WHEN $2 = true THEN 'concluido' ELSE status END
		WHERE id = $1
	`
	_, err := db.Pool.Exec(ctx, query, agendamentoID, tomou)
	return err
}
