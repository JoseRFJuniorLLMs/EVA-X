// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"eva-mind/internal/brainstem/push"
)

// NotificationService gerencia notificações para cuidadores
type NotificationService struct {
	db          *sql.DB
	pushService *push.FirebaseService
	auditLogger *AuditLogger
}

// NewNotificationService cria um novo serviço de notificação
func NewNotificationService(db *sql.DB, pushService *push.FirebaseService) *NotificationService {
	return &NotificationService{
		db:          db,
		pushService: pushService,
		auditLogger: NewAuditLogger(db),
	}
}

// NotifyCaregiver envia notificação para o cuidador sobre preocupação de saúde
func (ns *NotificationService) NotifyCaregiver(ctx context.Context, idosoID int64, auditID int64, concern string, riskLevel RiskLevel) error {
	// Buscar cuidador principal do idoso
	caregiverID, err := ns.getPrimaryCaregiverID(ctx, idosoID)
	if err != nil {
		return fmt.Errorf("erro ao buscar cuidador: %w", err)
	}

	if caregiverID == 0 {
		log.Printf("⚠️ Nenhum cuidador encontrado para idoso %d", idosoID)
		return nil
	}

	// Buscar token FCM do cuidador
	fcmToken, err := ns.getFCMToken(ctx, caregiverID)
	if err != nil || fcmToken == "" {
		log.Printf("⚠️ Token FCM não encontrado para cuidador %d", caregiverID)
		return nil
	}

	// Construir mensagem de notificação
	title, body := ns.buildNotificationMessage(riskLevel, concern)

	// Enviar notificação push usando método correto do Firebase
	_, err = ns.pushService.SendAlertNotification(fcmToken, fmt.Sprintf("Idoso #%d", idosoID), fmt.Sprintf("%s: %s", title, body))

	if err != nil {
		return fmt.Errorf("erro ao enviar notificação: %w", err)
	}

	// Marcar como notificado no banco
	err = ns.auditLogger.MarkCaregiverNotified(ctx, auditID)
	if err != nil {
		log.Printf("⚠️ Erro ao marcar notificação: %v", err)
	}

	log.Printf("✅ Cuidador %d notificado sobre alerta de saúde (idoso %d, risco %s)", caregiverID, idosoID, riskLevel.String())
	return nil
}

// getPrimaryCaregiverID busca o ID do cuidador principal
func (ns *NotificationService) getPrimaryCaregiverID(ctx context.Context, idosoID int64) (int64, error) {
	query := `
		SELECT u.id
		FROM usuarios u
		JOIN idosos i ON i.usuario_id = u.id
		WHERE i.id = $1
		LIMIT 1
	`

	var caregiverID int64
	err := ns.db.QueryRowContext(ctx, query, idosoID).Scan(&caregiverID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return caregiverID, nil
}

// getFCMToken busca o token FCM do usuário
func (ns *NotificationService) getFCMToken(ctx context.Context, userID int64) (string, error) {
	query := `
		SELECT fcm_token
		FROM usuarios
		WHERE id = $1 AND fcm_token IS NOT NULL
	`

	var token string
	err := ns.db.QueryRowContext(ctx, query, userID).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return token, nil
}

// buildNotificationMessage constrói título e corpo da notificação
func (ns *NotificationService) buildNotificationMessage(riskLevel RiskLevel, concern string) (string, string) {
	var title, body string

	switch riskLevel {
	case RiskCritical:
		title = "🚨 ALERTA CRÍTICO DE SAÚDE"
		body = fmt.Sprintf("Atenção urgente necessária: %s", truncate(concern, 100))
	case RiskHigh:
		title = "⚠️ Alerta de Saúde Importante"
		body = fmt.Sprintf("Requer atenção: %s", truncate(concern, 100))
	case RiskMedium:
		title = "💡 Preocupação de Saúde"
		body = fmt.Sprintf("Monitorar: %s", truncate(concern, 100))
	default:
		title = "ℹ️ Informação de Saúde"
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
			log.Printf("⚠️ Erro ao notificar alerta %d: %v", alert.ID, err)
		}
	}

	return nil
}

// parseRiskLevelString converte string para RiskLevel
func parseRiskLevelString(s string) RiskLevel {
	switch s {
	case "CRÍTICO":
		return RiskCritical
	case "ALTO":
		return RiskHigh
	case "MÉDIO":
		return RiskMedium
	default:
		return RiskLow
	}
}
