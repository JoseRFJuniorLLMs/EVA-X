package thinking

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AuditLogger gerencia o log de análises de saúde no banco de dados
type AuditLogger struct {
	db *sql.DB
}

// NewAuditLogger cria um novo logger de auditoria
func NewAuditLogger(db *sql.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

// LogHealthAnalysis registra uma análise de saúde no banco
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

	query := `
		INSERT INTO health_thinking_audit (
			idoso_id,
			concern,
			thought_process,
			risk_level,
			recommended_actions,
			seek_medical_care,
			urgency_level,
			final_answer
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	var auditID int64
	err = al.db.QueryRowContext(
		ctx,
		query,
		idosoID,
		concern,
		thoughtProcessJSON,
		response.RiskLevel.String(),
		actionsJSON,
		response.SeekMedicalCare,
		response.UrgencyLevel,
		response.FinalAnswer,
	).Scan(&auditID)

	if err != nil {
		return 0, fmt.Errorf("erro ao inserir auditoria: %w", err)
	}

	return auditID, nil
}

// MarkCaregiverNotified marca que o cuidador foi notificado
func (al *AuditLogger) MarkCaregiverNotified(ctx context.Context, auditID int64) error {
	query := `SELECT mark_caregiver_notified($1)`

	_, err := al.db.ExecContext(ctx, query, auditID)
	if err != nil {
		return fmt.Errorf("erro ao marcar notificação: %w", err)
	}

	return nil
}

// GetPendingCriticalAlerts retorna alertas críticos não notificados
func (al *AuditLogger) GetPendingCriticalAlerts(ctx context.Context) ([]CriticalAlert, error) {
	query := `
		SELECT 
			id,
			idoso_id,
			idoso_nome,
			concern,
			risk_level,
			urgency_level,
			created_at,
			minutes_since_alert
		FROM v_critical_alerts_pending
		ORDER BY created_at DESC
	`

	rows, err := al.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar alertas: %w", err)
	}
	defer rows.Close()

	var alerts []CriticalAlert
	for rows.Next() {
		var alert CriticalAlert
		err := rows.Scan(
			&alert.ID,
			&alert.IdosoID,
			&alert.IdosoNome,
			&alert.Concern,
			&alert.RiskLevel,
			&alert.UrgencyLevel,
			&alert.CreatedAt,
			&alert.MinutesSinceAlert,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear alerta: %w", err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// GetHealthSummary retorna resumo de preocupações de saúde de um idoso
func (al *AuditLogger) GetHealthSummary(ctx context.Context, idosoID int64) (*HealthSummary, error) {
	query := `
		SELECT 
			total_concerns,
			critical_count,
			high_count,
			medium_count,
			low_count,
			notified_count,
			last_concern_date
		FROM v_health_concerns_summary
		WHERE idoso_id = $1
	`

	var summary HealthSummary
	var lastConcern sql.NullTime

	err := al.db.QueryRowContext(ctx, query, idosoID).Scan(
		&summary.TotalConcerns,
		&summary.CriticalCount,
		&summary.HighCount,
		&summary.MediumCount,
		&summary.LowCount,
		&summary.NotifiedCount,
		&lastConcern,
	)

	if err == sql.ErrNoRows {
		// Nenhuma preocupação registrada
		return &HealthSummary{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("erro ao buscar resumo: %w", err)
	}

	if lastConcern.Valid {
		summary.LastConcernDate = &lastConcern.Time
	}

	return &summary, nil
}

// CriticalAlert representa um alerta crítico pendente
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

// HealthSummary representa um resumo de saúde de um idoso
type HealthSummary struct {
	TotalConcerns   int        `json:"total_concerns"`
	CriticalCount   int        `json:"critical_count"`
	HighCount       int        `json:"high_count"`
	MediumCount     int        `json:"medium_count"`
	LowCount        int        `json:"low_count"`
	NotifiedCount   int        `json:"notified_count"`
	LastConcernDate *time.Time `json:"last_concern_date,omitempty"`
}
