// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"context"
	"eva/internal/brainstem/database"
	"eva/internal/brainstem/push"
	"fmt"
	"log"
	"time"
)

// AlertFamilyWithSeverity cria um alerta e notifica a família
// NOTA: Versão legada - usar motor/actions.AlertFamilyWithSeverity de preferência
func AlertFamilyWithSeverity(db *database.DB, pushService *push.FirebaseService, idosoID int64, reason, severity string) error {
	log.Printf("🚨 Alerta de Família: %s (Severidade: %s)", reason, severity)

	// 1. Salvar no NietzscheDB
	ctx := context.Background()
	alertID, err := db.Insert(ctx, "alertas", map[string]interface{}{
		"idoso_id":    idosoID,
		"mensagem":    reason,
		"severidade":  severity,
		"visualizado": false,
		"criada_em":   time.Now().Format(time.RFC3339),
	})
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
func ConfirmMedication(db *database.DB, pushService *push.FirebaseService, idosoID int64, medName string) error {
	log.Printf("💊 Medicamento confirmado: %s", medName)

	ctx := context.Background()
	_, err := db.Insert(ctx, "historico_medicamentos", map[string]interface{}{
		"idoso_id":    idosoID,
		"medicamento": medName,
		"tomado":      true,
		"data_hora":   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("❌ Erro ao registrar medicamento: %v", err)
	}

	return nil
}

// ScheduleAppointment agenda um compromisso
// NOTA: Versão legada - usar motor/actions.ScheduleAppointment de preferência
func ScheduleAppointment(db *database.DB, idosoID int64, timestampStr, tipo, description string) error {
	log.Printf("📅 Agendamento solicitado: %s - %s em %s", tipo, description, timestampStr)

	// Parse ISO 8601
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		timestamp, err = time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			return fmt.Errorf("formato de data inválido: %v", err)
		}
	}

	ctx := context.Background()
	_, err = db.Insert(ctx, "agendamentos", map[string]interface{}{
		"idoso_id":           idosoID,
		"tipo":               tipo,
		"data_hora_agendada": timestamp.Format(time.RFC3339),
		"descricao":          description,
		"status":             "agendado",
		"criado_em":          time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("❌ Erro ao agendar: %v", err)
		return fmt.Errorf("erro ao salvar agendamento: %w", err)
	}

	return nil
}
