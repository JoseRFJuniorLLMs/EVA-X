// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package medgemma

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// AuditLogger gerencia o log de analises de imagens medicas
type AuditLogger struct {
	db *database.DB
}

// NewAuditLogger cria um novo logger de auditoria
func NewAuditLogger(db *database.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

// LogPrescriptionAnalysis registra analise de receita
func (al *AuditLogger) LogPrescriptionAnalysis(ctx context.Context, idosoID int64, imageURL string, analysis *PrescriptionAnalysis) (int64, error) {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar analise: %w", err)
	}

	content := map[string]interface{}{
		"idoso_id":                  idosoID,
		"image_type":               "prescription",
		"image_url":                imageURL,
		"analysis_result":          string(analysisJSON),
		"requires_medical_attention": analysis.ControlledMedications,
		"created_at":               time.Now().Format(time.RFC3339),
	}

	analysisID, err := al.db.Insert(ctx, "medical_image_analysis", content)
	if err != nil {
		return 0, fmt.Errorf("erro ao inserir analise: %w", err)
	}

	log.Printf("Analise de receita salva: ID %d, %d medicamentos", analysisID, len(analysis.Medications))
	return analysisID, nil
}

// LogWoundAnalysis registra analise de ferida
func (al *AuditLogger) LogWoundAnalysis(ctx context.Context, idosoID int64, imageURL string, analysis *WoundAnalysis) (int64, error) {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar analise: %w", err)
	}

	content := map[string]interface{}{
		"idoso_id":                  idosoID,
		"image_type":               "wound",
		"image_url":                imageURL,
		"analysis_result":          string(analysisJSON),
		"severity":                 analysis.Severity,
		"requires_medical_attention": analysis.SeekMedicalCare,
		"created_at":               time.Now().Format(time.RFC3339),
	}

	analysisID, err := al.db.Insert(ctx, "medical_image_analysis", content)
	if err != nil {
		return 0, fmt.Errorf("erro ao inserir analise: %w", err)
	}

	log.Printf("Analise de ferida salva: ID %d, gravidade %s", analysisID, analysis.Severity)
	return analysisID, nil
}

// MarkNotified marca que o cuidador foi notificado
func (al *AuditLogger) MarkNotified(ctx context.Context, analysisID int64) error {
	err := al.db.Update(ctx, "medical_image_analysis",
		map[string]interface{}{"id": analysisID},
		map[string]interface{}{
			"notified":    true,
			"notified_at": time.Now().Format(time.RFC3339),
		})
	if err != nil {
		return fmt.Errorf("erro ao marcar notificacao: %w", err)
	}

	return nil
}

// GetPendingAlerts retorna alertas pendentes de notificacao
func (al *AuditLogger) GetPendingAlerts(ctx context.Context) ([]MedicalAlert, error) {
	rows, err := al.db.QueryByLabel(ctx, "medical_image_analysis",
		" AND n.requires_medical_attention = $req AND n.notified = $notified",
		map[string]interface{}{"req": true, "notified": false}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar alertas: %w", err)
	}

	var alerts []MedicalAlert
	for _, m := range rows {
		createdAt := database.GetTime(m, "created_at")
		minutesSince := time.Since(createdAt).Minutes()

		alert := MedicalAlert{
			ID:                   database.GetInt64(m, "id"),
			IdosoID:              database.GetInt64(m, "idoso_id"),
			IdosoNome:            database.GetString(m, "idoso_nome"),
			ImageType:            database.GetString(m, "image_type"),
			Severity:             database.GetString(m, "severity"),
			CreatedAt:            createdAt.Format(time.RFC3339),
			MinutesSinceAnalysis: minutesSince,
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// SaveMedicationsFromPrescription salva medicamentos extraidos de receita
func (al *AuditLogger) SaveMedicationsFromPrescription(ctx context.Context, idosoID int64, medications []Medication) error {
	for _, med := range medications {
		// Check if medication exists
		rows, _ := al.db.QueryByLabel(ctx, "medicamentos",
			" AND n.idoso_id = $idoso_id AND n.nome = $nome",
			map[string]interface{}{"idoso_id": idosoID, "nome": med.Name}, 1)

		if len(rows) > 0 {
			// Update existing
			err := al.db.Update(ctx, "medicamentos",
				map[string]interface{}{"idoso_id": idosoID, "nome": med.Name},
				map[string]interface{}{
					"dosagem":    med.Dosage,
					"frequencia": med.Frequency,
					"horarios":   med.Schedule,
					"ativo":      true,
					"updated_at": time.Now().Format(time.RFC3339),
				})
			if err != nil {
				log.Printf("Erro ao atualizar medicamento %s: %v", med.Name, err)
				continue
			}
		} else {
			// Insert new
			_, err := al.db.Insert(ctx, "medicamentos", map[string]interface{}{
				"idoso_id":   idosoID,
				"nome":       med.Name,
				"dosagem":    med.Dosage,
				"frequencia": med.Frequency,
				"horarios":   med.Schedule,
				"ativo":      true,
				"created_at": time.Now().Format(time.RFC3339),
			})
			if err != nil {
				log.Printf("Erro ao salvar medicamento %s: %v", med.Name, err)
				continue
			}
		}

		log.Printf("Medicamento salvo: %s %s", med.Name, med.Dosage)
	}

	return nil
}

// MedicalAlert representa um alerta de imagem medica
type MedicalAlert struct {
	ID                   int64   `json:"id"`
	IdosoID              int64   `json:"idoso_id"`
	IdosoNome            string  `json:"idoso_nome"`
	ImageType            string  `json:"image_type"`
	Severity             string  `json:"severity"`
	CreatedAt            string  `json:"created_at"`
	MinutesSinceAnalysis float64 `json:"minutes_since_analysis"`
}
