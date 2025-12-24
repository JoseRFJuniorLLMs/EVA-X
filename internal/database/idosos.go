package database

import (
	"context"

	"eva-mind/pkg/models"
)

func (db *DB) GetIdosoByID(ctx context.Context, id int) (*models.CallContext, error) {
	query := `
		SELECT 
			id,
			nome,
			telefone,
			nivel_cognitivo,
			limitacoes_auditivas
		FROM idosos
		WHERE id = $1
	`

	var callCtx models.CallContext
	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&callCtx.IdosoID,
		&callCtx.IdosoNome,
		&callCtx.Telefone,
		&callCtx.NivelCognitivo,
		&callCtx.LimitacoesAuditivas,
	)

	if err != nil {
		return nil, err
	}

	return &callCtx, nil
}
