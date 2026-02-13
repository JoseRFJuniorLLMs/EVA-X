package medgemma

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// AuditLogger gerencia o log de análises de imagens médicas
type AuditLogger struct {
	db *sql.DB
}

// NewAuditLogger cria um novo logger de auditoria
func NewAuditLogger(db *sql.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

// LogPrescriptionAnalysis registra análise de receita
func (al *AuditLogger) LogPrescriptionAnalysis(ctx context.Context, idosoID int64, imageURL string, analysis *PrescriptionAnalysis) (int64, error) {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar análise: %w", err)
	}

	query := `
		INSERT INTO medical_image_analysis (
			idoso_id,
			image_type,
			image_url,
			analysis_result,
			requires_medical_attention
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var analysisID int64
	err = al.db.QueryRowContext(
		ctx,
		query,
		idosoID,
		"prescription",
		imageURL,
		analysisJSON,
		analysis.ControlledMedications, // Receitas controladas requerem atenção
	).Scan(&analysisID)

	if err != nil {
		return 0, fmt.Errorf("erro ao inserir análise: %w", err)
	}

	log.Printf("✅ Análise de receita salva: ID %d, %d medicamentos", analysisID, len(analysis.Medications))
	return analysisID, nil
}

// LogWoundAnalysis registra análise de ferida
func (al *AuditLogger) LogWoundAnalysis(ctx context.Context, idosoID int64, imageURL string, analysis *WoundAnalysis) (int64, error) {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar análise: %w", err)
	}

	query := `
		INSERT INTO medical_image_analysis (
			idoso_id,
			image_type,
			image_url,
			analysis_result,
			severity,
			requires_medical_attention
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	var analysisID int64
	err = al.db.QueryRowContext(
		ctx,
		query,
		idosoID,
		"wound",
		imageURL,
		analysisJSON,
		analysis.Severity,
		analysis.SeekMedicalCare,
	).Scan(&analysisID)

	if err != nil {
		return 0, fmt.Errorf("erro ao inserir análise: %w", err)
	}

	log.Printf("✅ Análise de ferida salva: ID %d, gravidade %s", analysisID, analysis.Severity)
	return analysisID, nil
}

// MarkNotified marca que o cuidador foi notificado
func (al *AuditLogger) MarkNotified(ctx context.Context, analysisID int64) error {
	query := `SELECT mark_medical_image_notified($1)`

	_, err := al.db.ExecContext(ctx, query, analysisID)
	if err != nil {
		return fmt.Errorf("erro ao marcar notificação: %w", err)
	}

	return nil
}

// GetPendingAlerts retorna alertas pendentes de notificação
func (al *AuditLogger) GetPendingAlerts(ctx context.Context) ([]MedicalAlert, error) {
	query := `
		SELECT 
			id,
			idoso_id,
			idoso_nome,
			image_type,
			severity,
			created_at,
			minutes_since_analysis
		FROM v_medical_image_alerts
		ORDER BY severity DESC, created_at DESC
	`

	rows, err := al.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar alertas: %w", err)
	}
	defer rows.Close()

	var alerts []MedicalAlert
	for rows.Next() {
		var alert MedicalAlert
		var severity sql.NullString

		err := rows.Scan(
			&alert.ID,
			&alert.IdosoID,
			&alert.IdosoNome,
			&alert.ImageType,
			&severity,
			&alert.CreatedAt,
			&alert.MinutesSinceAnalysis,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear alerta: %w", err)
		}

		if severity.Valid {
			alert.Severity = severity.String
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// SaveMedicationsFromPrescription salva medicamentos extraídos de receita
func (al *AuditLogger) SaveMedicationsFromPrescription(ctx context.Context, idosoID int64, medications []Medication) error {
	for _, med := range medications {
		// Inserir ou atualizar medicamento
		query := `
			INSERT INTO medicamentos (
				idoso_id,
				nome,
				dosagem,
				frequencia,
				horarios,
				ativo
			) VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (idoso_id, nome) 
			DO UPDATE SET
				dosagem = EXCLUDED.dosagem,
				frequencia = EXCLUDED.frequencia,
				horarios = EXCLUDED.horarios,
				ativo = true,
				updated_at = NOW()
		`

		_, err := al.db.ExecContext(
			ctx,
			query,
			idosoID,
			med.Name,
			med.Dosage,
			med.Frequency,
			med.Schedule,
		)

		if err != nil {
			log.Printf("⚠️ Erro ao salvar medicamento %s: %v", med.Name, err)
			continue
		}

		log.Printf("✅ Medicamento salvo: %s %s", med.Name, med.Dosage)
	}

	return nil
}

// MedicalAlert representa um alerta de imagem médica
type MedicalAlert struct {
	ID                   int64   `json:"id"`
	IdosoID              int64   `json:"idoso_id"`
	IdosoNome            string  `json:"idoso_nome"`
	ImageType            string  `json:"image_type"`
	Severity             string  `json:"severity"`
	CreatedAt            string  `json:"created_at"`
	MinutesSinceAnalysis float64 `json:"minutes_since_analysis"`
}
