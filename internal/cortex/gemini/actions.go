package gemini

import (
	"database/sql"
	"eva-mind/internal/brainstem/push"
	"fmt"
	"log"
	"time"
)

// AlertFamilyWithSeverity cria um alerta e notifica a família
// NOTA: Versão legada - usar motor/actions.AlertFamilyWithSeverity de preferência
func AlertFamilyWithSeverity(db *sql.DB, pushService *push.FirebaseService, idosoID int64, reason, severity string) error {
	log.Printf("🚨 Alerta de Família: %s (Severidade: %s)", reason, severity)

	// 1. Salvar no banco
	query := `
		INSERT INTO alertas (idoso_id, mensagem, severidade, visualizado, criada_em)
		VALUES ($1, $2, $3, false, NOW())
		RETURNING id
	`
	var alertID int64
	err := db.QueryRow(query, idosoID, reason, severity).Scan(&alertID)
	if err != nil {
		log.Printf("❌ Erro ao salvar alerta: %v", err)
		return fmt.Errorf("erro ao salvar alerta: %w", err)
	}

	// 2. Enviar Push Notification
	if pushService != nil {
		title := fmt.Sprintf("Alerta de Saúde (%s)", severity)
		body := reason
		data := map[string]string{
			"type":     "alert",
			"alert_id": fmt.Sprintf("%d", alertID),
			"severity": severity,
		}

		// Enviar para o tópico do cuidador/familiar
		topic := fmt.Sprintf("idoso_%d", idosoID)
		err := pushService.SendNotificationToTopic(topic, title, body, data)
		if err != nil {
			log.Printf("⚠️ Erro ao enviar push: %v", err)
		}
	}

	return nil
}

// ConfirmMedication registra a confirmação de medicamento
// NOTA: Versão legada - usar motor/actions.ConfirmMedication de preferência
func ConfirmMedication(db *sql.DB, pushService *push.FirebaseService, idosoID int64, medName string) error {
	log.Printf("💊 Medicamento confirmado: %s", medName)

	// 1. Registrar no histórico
	query := `
		INSERT INTO historico_medicamentos (idoso_id, medicamento, tomado, data_hora)
		VALUES ($1, $2, true, NOW())
	`
	_, err := db.Exec(query, idosoID, medName)
	if err != nil {
		log.Printf("❌ Erro ao registrar medicamento: %v", err)
	}

	return nil
}

// ScheduleAppointment agenda um compromisso
// NOTA: Versão legada - usar motor/actions.ScheduleAppointment de preferência
func ScheduleAppointment(db *sql.DB, idosoID int64, timestampStr, tipo, description string) error {
	log.Printf("📅 Agendamento solicitado: %s - %s em %s", tipo, description, timestampStr)

	// Parse ISO 8601
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		// Tentar formatos alternativos se falhar
		timestamp, err = time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			return fmt.Errorf("formato de data inválido: %v", err)
		}
	}

	query := `
		INSERT INTO agendamentos (idoso_id, tipo, data_hora_agendada, descricao, status, criado_em)
		VALUES ($1, $2, $3, $4, 'agendado', NOW())
	`
	_, err = db.Exec(query, idosoID, tipo, timestamp, description)
	if err != nil {
		log.Printf("❌ Erro ao agendar: %v", err)
		return fmt.Errorf("erro ao salvar agendamento: %w", err)
	}

	return nil
}
