// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"fmt"
	"log"

	"eva/internal/brainstem/database"
	"eva/internal/brainstem/push"
)

// NotificationService gerencia notificacoes para cuidadores
type NotificationService struct {
	db          *database.DB
	pushService *push.FirebaseService
	auditLogger *AuditLogger
}

// NewNotificationService cria um novo servico de notificacao
func NewNotificationService(db *database.DB, pushService *push.FirebaseService) *NotificationService {
	return &NotificationService{
		db:          db,
		pushService: pushService,
		auditLogger: NewAuditLogger(db),
	}
}

// NotifyCaregiver envia notificacao para o cuidador sobre preocupacao de saude
func (ns *NotificationService) NotifyCaregiver(ctx context.Context, idosoID int64, auditID int64, concern string, riskLevel RiskLevel) error {
	// Buscar cuidador principal do idoso
	caregiverID, err := ns.getPrimaryCaregiverID(ctx, idosoID)
	if err != nil {
		return fmt.Errorf("erro ao buscar cuidador: %w", err)
	}

	if caregiverID == 0 {
		log.Printf("Nenhum cuidador encontrado para idoso %d", idosoID)
		return nil
	}

	// Buscar token FCM do cuidador
	fcmToken, err := ns.getFCMToken(ctx, caregiverID)
	if err != nil || fcmToken == "" {
		log.Printf("Token FCM nao encontrado para cuidador %d", caregiverID)
		return nil
	}

	// Construir mensagem de notificacao
	title, body := ns.buildNotificationMessage(riskLevel, concern)

	// Enviar notificacao push usando metodo correto do Firebase
	_, err = ns.pushService.SendAlertNotification(fcmToken, fmt.Sprintf("Idoso #%d", idosoID), fmt.Sprintf("%s: %s", title, body))

	if err != nil {
		return fmt.Errorf("erro ao enviar notificacao: %w", err)
	}

	// Marcar como notificado no banco
	err = ns.auditLogger.MarkCaregiverNotified(ctx, auditID)
	if err != nil {
		log.Printf("Erro ao marcar notificacao: %v", err)
	}

	log.Printf("Cuidador %d notificado sobre alerta de saude (idoso %d, risco %s)", caregiverID, idosoID, riskLevel.String())
	return nil
}

// getPrimaryCaregiverID busca o ID do cuidador principal
func (ns *NotificationService) getPrimaryCaregiverID(ctx context.Context, idosoID int64) (int64, error) {
	// Look up the idoso to find the usuario_id (caregiver)
	rows, err := ns.db.QueryByLabel(ctx, "idosos",
		" AND n.id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 1)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	caregiverID := database.GetInt64(rows[0], "usuario_id")
	return caregiverID, nil
}

// getFCMToken busca o token FCM do usuario
func (ns *NotificationService) getFCMToken(ctx context.Context, userID int64) (string, error) {
	rows, err := ns.db.QueryByLabel(ctx, "usuarios",
		" AND n.id = $user_id",
		map[string]interface{}{"user_id": userID}, 1)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}

	token := database.GetString(rows[0], "fcm_token")
	return token, nil
}

// buildNotificationMessage constroi titulo e corpo da notificacao
func (ns *NotificationService) buildNotificationMessage(riskLevel RiskLevel, concern string) (string, string) {
	var title, body string

	switch riskLevel {
	case RiskCritical:
		title = "ALERTA CRITICO DE SAUDE"
		body = fmt.Sprintf("Atencao urgente necessaria: %s", truncate(concern, 100))
	case RiskHigh:
		title = "Alerta de Saude Importante"
		body = fmt.Sprintf("Requer atencao: %s", truncate(concern, 100))
	case RiskMedium:
		title = "Preocupacao de Saude"
		body = fmt.Sprintf("Monitorar: %s", truncate(concern, 100))
	default:
		title = "Informacao de Saude"
		body = fmt.Sprintf("Registrado: %s", truncate(concern, 100))
	}

	return title, body
}

// truncate limita o tamanho de uma string
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// CheckPendingAlerts verifica e envia alertas pendentes (executar periodicamente)
func (ns *NotificationService) CheckPendingAlerts(ctx context.Context) error {
	alerts, err := ns.auditLogger.GetPendingCriticalAlerts(ctx)
	if err != nil {
		return fmt.Errorf("erro ao buscar alertas pendentes: %w", err)
	}

	for _, alert := range alerts {
		err := ns.NotifyCaregiver(ctx, alert.IdosoID, alert.ID, alert.Concern, parseRiskLevelString(alert.RiskLevel))
		if err != nil {
			log.Printf("Erro ao notificar alerta %d: %v", alert.ID, err)
		}
	}

	return nil
}

// parseRiskLevelString converte string para RiskLevel
func parseRiskLevelString(s string) RiskLevel {
	switch s {
	case "CRITICO":
		return RiskCritical
	case "ALTO":
		return RiskHigh
	case "MEDIO":
		return RiskMedium
	default:
		return RiskLow
	}
}
