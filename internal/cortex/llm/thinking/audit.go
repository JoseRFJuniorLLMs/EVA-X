// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eva/internal/brainstem/database"
)

// AuditLogger gerencia o log de analises de saude no banco de dados
type AuditLogger struct {
	db *database.DB
}

// NewAuditLogger cria um novo logger de auditoria
func NewAuditLogger(db *database.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

// LogHealthAnalysis registra uma analise de saude no banco
func (al *AuditLogger) LogHealthAnalysis(ctx context.Context, idosoID int64, concern string, response *ThinkingResponse) (int64, error) {
	// Converter thought_process para JSON
	thoughtProcessJSON, err := json.Marshal(response.ThoughtProcess)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar thought_process: %w", err)
	}

	// Converter recommended_actions para JSON
	actionsJSON, err := json.Marshal(response.RecommendedActions)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar recommended_actions: %w", err)
	}

	content := map[string]interface{}{
		"idoso_id":            idosoID,
		"concern":             concern,
		"thought_process":     string(thoughtProcessJSON),
		"risk_level":          response.RiskLevel.String(),
		"recommended_actions": string(actionsJSON),
		"seek_medical_care":   response.SeekMedicalCare,
		"urgency_level":       response.UrgencyLevel,
		"final_answer":        response.FinalAnswer,
		"created_at":          time.Now().Format(time.RFC3339),
	}

	auditID, err := al.db.Insert(ctx, "health_thinking_audit", content)
	if err != nil {
		return 0, fmt.Errorf("erro ao inserir auditoria: %w", err)
	}

	return auditID, nil
}

// MarkCaregiverNotified marca que o cuidador foi notificado
func (al *AuditLogger) MarkCaregiverNotified(ctx context.Context, auditID int64) error {
	err := al.db.Update(ctx, "health_thinking_audit",
		map[string]interface{}{"id": auditID},
		map[string]interface{}{
			"caregiver_notified":    true,
			"caregiver_notified_at": time.Now().Format(time.RFC3339),
		})
	if err != nil {
		return fmt.Errorf("erro ao marcar notificacao: %w", err)
	}

	return nil
}

// GetPendingCriticalAlerts retorna alertas criticos nao notificados
func (al *AuditLogger) GetPendingCriticalAlerts(ctx context.Context) ([]CriticalAlert, error) {
	rows, err := al.db.QueryByLabel(ctx, "health_thinking_audit",
		" AND n.caregiver_notified = $notified AND (n.risk_level = $critical OR n.risk_level = $high)",
		map[string]interface{}{
			"notified": false,
			"critical": "CRITICO",
			"high":     "ALTO",
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar alertas: %w", err)
	}

	var alerts []CriticalAlert
	for _, m := range rows {
		createdAt := database.GetTime(m, "created_at")
		minutesSince := time.Since(createdAt).Minutes()

		alert := CriticalAlert{
			ID:                database.GetInt64(m, "id"),
			IdosoID:           database.GetInt64(m, "idoso_id"),
			IdosoNome:         database.GetString(m, "idoso_nome"),
			Concern:           database.GetString(m, "concern"),
			RiskLevel:         database.GetString(m, "risk_level"),
			UrgencyLevel:      database.GetString(m, "urgency_level"),
			CreatedAt:         createdAt,
			MinutesSinceAlert: minutesSince,
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// GetHealthSummary retorna resumo de preocupacoes de saude de um idoso
func (al *AuditLogger) GetHealthSummary(ctx context.Context, idosoID int64) (*HealthSummary, error) {
	rows, err := al.db.QueryByLabel(ctx, "health_thinking_audit",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar resumo: %w", err)
	}

	if len(rows) == 0 {
		// Nenhuma preocupacao registrada
		return &HealthSummary{}, nil
	}

	summary := &HealthSummary{
		TotalConcerns: len(rows),
	}

	var lastConcernTime time.Time

	for _, m := range rows {
		riskLevel := database.GetString(m, "risk_level")
		switch riskLevel {
		case "CRITICO":
			summary.CriticalCount++
		case "ALTO":
			summary.HighCount++
		case "MEDIO":
			summary.MediumCount++
		default:
			summary.LowCount++
		}

		notified := database.GetBool(m, "caregiver_notified")
		if notified {
			summary.NotifiedCount++
		}

		created := database.GetTime(m, "created_at")
		if created.After(lastConcernTime) {
			lastConcernTime = created
		}
	}

	if !lastConcernTime.IsZero() {
		summary.LastConcernDate = &lastConcernTime
	}

	return summary, nil
}

// CriticalAlert representa um alerta critico pendente
type CriticalAlert struct {
	ID                int64     `json:"id"`
	IdosoID           int64     `json:"idoso_id"`
	IdosoNome         string    `json:"idoso_nome"`
	Concern           string    `json:"concern"`
	RiskLevel         string    `json:"risk_level"`
	UrgencyLevel      string    `json:"urgency_level"`
	CreatedAt         time.Time `json:"created_at"`
	MinutesSinceAlert float64   `json:"minutes_since_alert"`
}

// HealthSummary representa um resumo de saude de um idoso
type HealthSummary struct {
	TotalConcerns   int        `json:"total_concerns"`
	CriticalCount   int        `json:"critical_count"`
	HighCount       int        `json:"high_count"`
	MediumCount     int        `json:"medium_count"`
	LowCount        int        `json:"low_count"`
	NotifiedCount   int        `json:"notified_count"`
	LastConcernDate *time.Time `json:"last_concern_date,omitempty"`
}
