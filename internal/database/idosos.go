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
			limitacoes_auditivas,
			familiar_principal->>'nome' as familiar_nome,
			familiar_principal->>'telefone' as familiar_telefone
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
		&callCtx.FamiliarNome,
		&callCtx.FamiliarTelefone,
	)

	if err != nil {
		return nil, err
	}

	return &callCtx, nil
}
