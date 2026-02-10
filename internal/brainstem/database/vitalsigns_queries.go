package database

import (
	"fmt"
	"time"
)

// SaveVitalSign saves a vital sign measurement to the database (manual/voice entry)
func (db *DB) SaveVitalSign(idosoID int64, tipo, valor, unidade, metodo, observacao string) error {
	query := `
		INSERT INTO sinais_vitais (idoso_id, tipo, valor, unidade, metodo, data_medicao, observacao)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := db.Conn.Exec(query, idosoID, tipo, valor, unidade, metodo, time.Now(), observacao)
	if err != nil {
		return fmt.Errorf("failed to save vital sign: %w", err)
	}
	return nil
}

// SaveDeviceHealthData saves health data from devices/Google Fit
func (db *DB) SaveDeviceHealthData(idosoID int64, bpm int, steps int) error {
	query := `
		INSERT INTO sinais_vitais_health (cliente_id, bpm, timestamp_coleta, created_at)
		VALUES ($1, $2, NOW(), NOW())
	`
	// Note: mapping idoso_id to cliente_id if they are the same in the DB
	_, err := db.Conn.Exec(query, idosoID, bpm)
	if err != nil {
		return fmt.Errorf("failed to save device health data: %w", err)
	}

	if steps > 0 {
		querySteps := `
			INSERT INTO atividade (cliente_id, passos, timestamp_coleta, created_at)
			VALUES ($1, $2, NOW(), NOW())
		`
		_, _ = db.Conn.Exec(querySteps, idosoID, steps)
	}

	return nil
}

// GetRecentVitalSigns gets recent vital signs for an idoso
func (db *DB) GetRecentVitalSigns(idosoID int64, tipo string, limit int) ([]VitalSign, error) {
	query := `
		SELECT id, tipo, valor, unidade, metodo, data_medicao, observacao
		FROM sinais_vitais
		WHERE idoso_id = $1 AND tipo = $2
		ORDER BY data_medicao DESC
		LIMIT $3
	`
	rows, err := db.Conn.Query(query, idosoID, tipo, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get vital signs: %w", err)
	}
	defer rows.Close()

	var signs []VitalSign
	for rows.Next() {
		var sign VitalSign
		err := rows.Scan(&sign.ID, &sign.Tipo, &sign.Valor, &sign.Unidade, &sign.Metodo, &sign.DataMedicao, &sign.Observacao)
		if err != nil {
			continue
		}
		signs = append(signs, sign)
	}
	return signs, nil
}

type VitalSign struct {
	ID          int64
	Tipo        string
	Valor       string
	Unidade     string
	Metodo      string
	DataMedicao time.Time
	Observacao  string
}
