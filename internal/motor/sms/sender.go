package sms

import (
	"fmt"
	"log"
	"time"
)

// AlertTemplates contains pre-formatted message templates
type AlertTemplates struct {
	EmergencyAlert      string
	MissedCallAlert     string
	MedicationReminder  string
	HealthCheckReminder string
}

// DefaultTemplates returns the default message templates
func DefaultTemplates() AlertTemplates {
	return AlertTemplates{
		EmergencyAlert:      "🚨 ALERTA EVA: %s precisa de ajuda urgente. Motivo: %s. Por favor, verifique imediatamente.",
		MissedCallAlert:     "⚠️ EVA: %s não atendeu a chamada agendada. Por favor, verifique se está tudo bem.",
		MedicationReminder:  "💊 EVA: Lembrete - %s precisa tomar %s. Por favor, confirme a medicação.",
		HealthCheckReminder: "🏥 EVA: %s não fez check-in há %s. Considere verificar.",
	}
}

// SendEmergencyAlert sends an emergency alert via SMS
func (s *TwilioService) SendEmergencyAlert(phoneNumber, elderName, reason string) (*MessageResult, error) {
	templates := DefaultTemplates()
	message := fmt.Sprintf(templates.EmergencyAlert, elderName, reason)

	result, err := s.SendSMS(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar alerta de emergência SMS para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("🚨 Alerta de emergência SMS enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendEmergencyAlertWhatsApp sends an emergency alert via WhatsApp
func (s *TwilioService) SendEmergencyAlertWhatsApp(phoneNumber, elderName, reason string) (*MessageResult, error) {
	templates := DefaultTemplates()
	message := fmt.Sprintf(templates.EmergencyAlert, elderName, reason)

	result, err := s.SendWhatsApp(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar alerta de emergência WhatsApp para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("🚨 Alerta de emergência WhatsApp enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendMissedCallAlert sends a missed call alert via SMS
func (s *TwilioService) SendMissedCallAlert(phoneNumber, elderName string) (*MessageResult, error) {
	templates := DefaultTemplates()
	message := fmt.Sprintf(templates.MissedCallAlert, elderName)

	result, err := s.SendSMS(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar alerta de chamada perdida SMS para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("📵 Alerta de chamada perdida SMS enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendMissedCallAlertWhatsApp sends a missed call alert via WhatsApp
func (s *TwilioService) SendMissedCallAlertWhatsApp(phoneNumber, elderName string) (*MessageResult, error) {
	templates := DefaultTemplates()
	message := fmt.Sprintf(templates.MissedCallAlert, elderName)

	result, err := s.SendWhatsApp(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar alerta de chamada perdida WhatsApp para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("📵 Alerta de chamada perdida WhatsApp enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendMedicationReminder sends a medication reminder via SMS
func (s *TwilioService) SendMedicationReminder(phoneNumber, elderName, medicationName string) (*MessageResult, error) {
	templates := DefaultTemplates()
	message := fmt.Sprintf(templates.MedicationReminder, elderName, medicationName)

	result, err := s.SendSMS(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar lembrete de medicação SMS para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("💊 Lembrete de medicação SMS enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendHealthCheckReminder sends a health check reminder via SMS
func (s *TwilioService) SendHealthCheckReminder(phoneNumber, elderName string, lastCheckIn time.Duration) (*MessageResult, error) {
	templates := DefaultTemplates()

	// Format duration in Portuguese
	durationStr := formatDuration(lastCheckIn)
	message := fmt.Sprintf(templates.HealthCheckReminder, elderName, durationStr)

	result, err := s.SendSMS(phoneNumber, message)
	if err != nil {
		log.Printf("❌ Erro ao enviar lembrete de check-in SMS para %s: %v", phoneNumber, err)
		return result, err
	}

	log.Printf("🏥 Lembrete de check-in SMS enviado para %s sobre %s", phoneNumber, elderName)
	return result, nil
}

// SendBulkEmergencyAlert sends emergency alerts to multiple phone numbers
func (s *TwilioService) SendBulkEmergencyAlert(phoneNumbers []string, elderName, reason string) []*MessageResult {
	results := make([]*MessageResult, 0, len(phoneNumbers))

	for _, phone := range phoneNumbers {
		result, err := s.SendEmergencyAlert(phone, elderName, reason)
		if err != nil {
			log.Printf("❌ Falha ao enviar para %s: %v", phone, err)
		}
		results = append(results, result)
	}

	// Count successes
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	log.Printf("📊 Bulk SMS: %d/%d enviados com sucesso", successCount, len(phoneNumbers))
	return results
}

// formatDuration formats a duration in Portuguese
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 1 {
		minutes := int(d.Minutes())
		if minutes < 1 {
			return "menos de 1 minuto"
		}
		return fmt.Sprintf("%d minutos", minutes)
	}
	if hours < 24 {
		return fmt.Sprintf("%d horas", hours)
	}
	days := hours / 24
	if days == 1 {
		return "1 dia"
	}
	return fmt.Sprintf("%d dias", days)
}
