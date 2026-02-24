// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// SaveVitalSign saves a vital sign measurement (manual/voice entry)
func (db *DB) SaveVitalSign(idosoID int64, tipo, valor, unidade, metodo, observacao string) error {
	ctx := context.Background()

	_, err := db.insertRow(ctx, "sinais_vitais", map[string]interface{}{
		"idoso_id":     idosoID,
		"tipo":         tipo,
		"valor":        valor,
		"unidade":      unidade,
		"metodo":       metodo,
		"data_medicao": time.Now().Format(time.RFC3339),
		"observacao":   observacao,
	})
	if err != nil {
		return fmt.Errorf("failed to save vital sign: %w", err)
	}
	return nil
}

// SaveDeviceHealthData saves health data from devices/Google Fit
func (db *DB) SaveDeviceHealthData(idosoID int64, bpm int, steps int) error {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)

	_, err := db.insertRow(ctx, "sinais_vitais_health", map[string]interface{}{
		"cliente_id":       idosoID,
		"bpm":              bpm,
		"timestamp_coleta": now,
		"created_at":       now,
	})
	if err != nil {
		return fmt.Errorf("failed to save device health data: %w", err)
	}

	if steps > 0 {
		_, _ = db.insertRow(ctx, "atividade", map[string]interface{}{
			"cliente_id":       idosoID,
			"passos":           steps,
			"timestamp_coleta": now,
			"created_at":       now,
		})
	}

	return nil
}

// GetRecentVitalSigns gets recent vital signs for an idoso
func (db *DB) GetRecentVitalSigns(idosoID int64, tipo string, limit int) ([]VitalSign, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "sinais_vitais",
		` AND n.idoso_id = $idoso_id AND n.tipo = $tipo`, map[string]interface{}{
			"idoso_id": idosoID,
			"tipo":     tipo,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get vital signs: %w", err)
	}

	var signs []VitalSign
	for _, m := range rows {
		signs = append(signs, VitalSign{
			ID:          getInt64(m, "id"),
			Tipo:        getString(m, "tipo"),
			Valor:       getString(m, "valor"),
			Unidade:     getString(m, "unidade"),
			Metodo:      getString(m, "metodo"),
			DataMedicao: getTime(m, "data_medicao"),
			Observacao:  getString(m, "observacao"),
		})
	}

	// Sort by data_medicao DESC
	sort.Slice(signs, func(i, j int) bool {
		return signs[i].DataMedicao.After(signs[j].DataMedicao)
	})

	if limit > 0 && len(signs) > limit {
		signs = signs[:limit]
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
