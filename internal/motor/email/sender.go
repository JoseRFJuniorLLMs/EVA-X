// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"fmt"
	"log"
)

// SendMissedCallAlert envia email de chamada perdida
func (s *EmailService) SendMissedCallAlert(caregiverEmail, caregiverName, elderName string) error {
	subject := fmt.Sprintf("⚠️ Chamada Não Atendida - %s", elderName)
	htmlBody := MissedCallAlertTemplate(elderName, caregiverName)

	if err := s.SendEmail(caregiverEmail, subject, htmlBody); err != nil {
		log.Printf("❌ Erro ao enviar email de chamada perdida: %v", err)
		return err
	}

	log.Printf("📧 Email de chamada perdida enviado para: %s", caregiverEmail)
	return nil
}

// SendEmergencyAlert envia email de emergência
func (s *EmailService) SendEmergencyAlert(caregiverEmail, caregiverName, elderName, reason string) error {
	subject := fmt.Sprintf("🚨 ALERTA CRÍTICO - %s", elderName)
	htmlBody := EmergencyAlertTemplate(elderName, caregiverName, reason)

	if err := s.SendEmail(caregiverEmail, subject, htmlBody); err != nil {
		log.Printf("❌ Erro ao enviar email de emergência: %v", err)
		return err
	}

	log.Printf("📧 Email de emergência enviado para: %s", caregiverEmail)
	return nil
}
