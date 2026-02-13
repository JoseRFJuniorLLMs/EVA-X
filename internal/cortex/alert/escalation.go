package alert

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"eva-mind/internal/brainstem/push"
	"eva-mind/internal/motor/email"
	"eva-mind/internal/motor/sms"
)

// AlertPriority defines the urgency level of an alert
type AlertPriority string

const (
	PriorityCritical AlertPriority = "critical" // Immediate escalation
	PriorityHigh     AlertPriority = "high"     // Fast escalation (2 min)
	PriorityMedium   AlertPriority = "medium"   // Normal escalation (5 min)
	PriorityLow      AlertPriority = "low"      // Slow escalation (15 min)
)

// DeliveryChannel represents a notification channel
type DeliveryChannel string

const (
	ChannelPush     DeliveryChannel = "push"
	ChannelSMS      DeliveryChannel = "sms"
	ChannelWhatsApp DeliveryChannel = "whatsapp"
	ChannelEmail    DeliveryChannel = "email"
	ChannelCall     DeliveryChannel = "call"
)

// CaregiverContact holds contact information for a caregiver
type CaregiverContact struct {
	ID          int64
	Name        string
	FCMToken    string
	PhoneNumber string
	Email       string
	Priority    int // 1 = primary, 2 = secondary, etc.
}

// AlertAttempt records a delivery attempt
type AlertAttempt struct {
	Channel     DeliveryChannel
	Success     bool
	MessageID   string
	Error       string
	AttemptedAt time.Time
	Latency     time.Duration
}

// EscalationResult holds the complete result of an escalation
type EscalationResult struct {
	AlertID       string
	ElderName     string
	Reason        string
	Priority      AlertPriority
	Attempts      []AlertAttempt
	Acknowledged  bool
	AcknowledgedBy string
	AcknowledgedAt time.Time
	StartedAt     time.Time
	CompletedAt   time.Time
	FinalChannel  DeliveryChannel
}

// EscalationService orchestrates multi-channel alert delivery
type EscalationService struct {
	firebase    *push.FirebaseService
	twilio      *sms.TwilioService
	emailSvc    *email.EmailService
	db          *sql.DB
	callbackURL string

	// Escalation timeouts by priority
	timeouts map[AlertPriority]time.Duration

	// Active escalations (for acknowledgment)
	activeAlerts sync.Map // alertID -> *EscalationResult
}

// EscalationConfig holds configuration for the service
type EscalationConfig struct {
	Firebase    *push.FirebaseService
	Twilio      *sms.TwilioService
	Email       *email.EmailService
	DB          *sql.DB
	CallbackURL string // Base URL for call acknowledgment
}

// NewEscalationService creates a new escalation orchestrator
func NewEscalationService(cfg EscalationConfig) *EscalationService {
	svc := &EscalationService{
		firebase:    cfg.Firebase,
		twilio:      cfg.Twilio,
		emailSvc:    cfg.Email,
		db:          cfg.DB,
		callbackURL: cfg.CallbackURL,
		timeouts: map[AlertPriority]time.Duration{
			PriorityCritical: 30 * time.Second,  // Escalate after 30s
			PriorityHigh:     2 * time.Minute,   // Escalate after 2 min
			PriorityMedium:   5 * time.Minute,   // Escalate after 5 min
			PriorityLow:      15 * time.Minute,  // Escalate after 15 min
		},
	}

	log.Println("✅ EscalationService initialized")
	log.Printf("   - Firebase: %v", cfg.Firebase != nil)
	log.Printf("   - Twilio: %v", cfg.Twilio != nil)
	log.Printf("   - Email: %v", cfg.Email != nil)

	return svc
}

// SendEmergencyAlert sends an emergency alert with automatic escalation
// Order: Push -> WhatsApp -> SMS -> Email -> Voice Call
func (s *EscalationService) SendEmergencyAlert(
	ctx context.Context,
	elderName, reason string,
	priority AlertPriority,
	contacts []CaregiverContact,
) *EscalationResult {
	alertID := fmt.Sprintf("alert-%d", time.Now().UnixNano())

	result := &EscalationResult{
		AlertID:   alertID,
		ElderName: elderName,
		Reason:    reason,
		Priority:  priority,
		Attempts:  make([]AlertAttempt, 0),
		StartedAt: time.Now(),
	}

	// Store for acknowledgment tracking
	s.activeAlerts.Store(alertID, result)
	defer s.activeAlerts.Delete(alertID)

	log.Printf("🚨 [%s] Iniciando escalonamento de alerta para %s", alertID, elderName)
	log.Printf("   Prioridade: %s | Contatos: %d", priority, len(contacts))

	timeout := s.timeouts[priority]

	// Try each channel in order
	channels := []DeliveryChannel{ChannelPush, ChannelWhatsApp, ChannelSMS, ChannelEmail, ChannelCall}

	for _, channel := range channels {
		// Check if already acknowledged
		if result.Acknowledged {
			log.Printf("✅ [%s] Alerta confirmado via %s", alertID, result.FinalChannel)
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			log.Printf("⚠️ [%s] Escalonamento cancelado: %v", alertID, ctx.Err())
			result.CompletedAt = time.Now()
			return result
		default:
		}

		// Attempt delivery on this channel
		success := s.tryChannel(ctx, channel, elderName, reason, contacts, result)

		if success {
			result.FinalChannel = channel

			// For non-critical, wait for acknowledgment before escalating
			if priority != PriorityCritical {
				acknowledged := s.waitForAcknowledgment(ctx, alertID, timeout)
				if acknowledged {
					result.Acknowledged = true
					break
				}
				log.Printf("⏰ [%s] Timeout aguardando confirmação via %s, escalando...", alertID, channel)
			}
		}
	}

	result.CompletedAt = time.Now()

	// Log final result
	successCount := 0
	for _, a := range result.Attempts {
		if a.Success {
			successCount++
		}
	}
	log.Printf("📊 [%s] Escalonamento finalizado: %d/%d tentativas bem-sucedidas",
		alertID, successCount, len(result.Attempts))

	// Save to database
	s.saveEscalationLog(result)

	return result
}

// tryChannel attempts delivery on a specific channel
func (s *EscalationService) tryChannel(
	ctx context.Context,
	channel DeliveryChannel,
	elderName, reason string,
	contacts []CaregiverContact,
	result *EscalationResult,
) bool {
	start := time.Now()
	var success bool
	var messageID, errMsg string

	switch channel {
	case ChannelPush:
		if s.firebase != nil {
			for _, c := range contacts {
				if c.FCMToken != "" {
					pushResult, err := s.firebase.SendAlertNotification(c.FCMToken, elderName, reason)
					if err == nil && pushResult.Success {
						success = true
						messageID = pushResult.MessageID
						log.Printf("📱 [Push] Enviado para %s", c.Name)
					} else if err != nil {
						errMsg = err.Error()
						log.Printf("❌ [Push] Falha para %s: %v", c.Name, err)
					}
				}
			}
		} else {
			errMsg = "Firebase not configured"
		}

	case ChannelWhatsApp:
		if s.twilio != nil && s.twilio.HasWhatsApp() {
			for _, c := range contacts {
				if c.PhoneNumber != "" {
					waResult, err := s.twilio.SendEmergencyAlertWhatsApp(c.PhoneNumber, elderName, reason)
					if err == nil && waResult.Success {
						success = true
						messageID = waResult.MessageID
						log.Printf("📱 [WhatsApp] Enviado para %s", c.Name)
					} else if err != nil {
						errMsg = err.Error()
						log.Printf("❌ [WhatsApp] Falha para %s: %v", c.Name, err)
					}
				}
			}
		} else {
			errMsg = "WhatsApp not configured"
		}

	case ChannelSMS:
		if s.twilio != nil && s.twilio.HasSMS() {
			for _, c := range contacts {
				if c.PhoneNumber != "" {
					smsResult, err := s.twilio.SendEmergencyAlert(c.PhoneNumber, elderName, reason)
					if err == nil && smsResult.Success {
						success = true
						messageID = smsResult.MessageID
						log.Printf("📱 [SMS] Enviado para %s", c.Name)
					} else if err != nil {
						errMsg = err.Error()
						log.Printf("❌ [SMS] Falha para %s: %v", c.Name, err)
					}
				}
			}
		} else {
			errMsg = "SMS not configured"
		}

	case ChannelEmail:
		if s.emailSvc != nil {
			for _, c := range contacts {
				if c.Email != "" {
					err := s.emailSvc.SendEmergencyAlert(c.Email, c.Name, elderName, reason)
					if err == nil {
						success = true
						log.Printf("📧 [Email] Enviado para %s", c.Name)
					} else {
						errMsg = err.Error()
						log.Printf("❌ [Email] Falha para %s: %v", c.Name, err)
					}
				}
			}
		} else {
			errMsg = "Email not configured"
		}

	case ChannelCall:
		if s.twilio != nil && s.twilio.HasSMS() { // Uses same phone number
			// Only call primary contact for voice
			for _, c := range contacts {
				if c.PhoneNumber != "" && c.Priority == 1 {
					callbackURL := fmt.Sprintf("%s/callback/call-ack?alert_id=%s", s.callbackURL, result.AlertID)
					callResult, err := s.twilio.MakeEmergencyCall(c.PhoneNumber, elderName, reason, callbackURL)
					if err == nil && callResult.Success {
						success = true
						messageID = callResult.CallSID
						log.Printf("📞 [Call] Ligação iniciada para %s", c.Name)
					} else if err != nil {
						errMsg = err.Error()
						log.Printf("❌ [Call] Falha para %s: %v", c.Name, err)
					}
					break // Only call primary
				}
			}
		} else {
			errMsg = "Voice calls not configured"
		}
	}

	// Record attempt
	result.Attempts = append(result.Attempts, AlertAttempt{
		Channel:     channel,
		Success:     success,
		MessageID:   messageID,
		Error:       errMsg,
		AttemptedAt: start,
		Latency:     time.Since(start),
	})

	return success
}

// waitForAcknowledgment waits for the alert to be acknowledged
func (s *EscalationService) waitForAcknowledgment(ctx context.Context, alertID string, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-ticker.C:
			if val, ok := s.activeAlerts.Load(alertID); ok {
				result := val.(*EscalationResult)
				if result.Acknowledged {
					return true
				}
			}
		}
	}
}

// AcknowledgeAlert marks an alert as acknowledged
func (s *EscalationService) AcknowledgeAlert(alertID, acknowledgedBy string) bool {
	if val, ok := s.activeAlerts.Load(alertID); ok {
		result := val.(*EscalationResult)
		result.Acknowledged = true
		result.AcknowledgedBy = acknowledgedBy
		result.AcknowledgedAt = time.Now()
		log.Printf("✅ Alerta %s confirmado por %s", alertID, acknowledgedBy)
		return true
	}
	return false
}

// SendMissedCallAlert sends a missed call alert with escalation
func (s *EscalationService) SendMissedCallAlert(
	ctx context.Context,
	elderName string,
	contacts []CaregiverContact,
) *EscalationResult {
	return s.SendEmergencyAlert(ctx, elderName, "Não atendeu chamada agendada", PriorityMedium, contacts)
}

// GetContactsForElder retrieves caregiver contacts from database
func (s *EscalationService) GetContactsForElder(elderID int64) ([]CaregiverContact, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT c.id, c.nome, c.fcm_token, c.telefone, c.email,
		       COALESCE(ci.prioridade, 99) as prioridade
		FROM cuidadores c
		LEFT JOIN cuidador_idoso ci ON c.id = ci.cuidador_id AND ci.idoso_id = $1
		WHERE ci.idoso_id = $1 OR c.tipo = 'responsavel'
		ORDER BY prioridade ASC
	`

	rows, err := s.db.Query(query, elderID)
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []CaregiverContact
	for rows.Next() {
		var c CaregiverContact
		var fcmToken, phone, emailAddr sql.NullString

		if err := rows.Scan(&c.ID, &c.Name, &fcmToken, &phone, &emailAddr, &c.Priority); err != nil {
			log.Printf("⚠️ Erro ao ler contato: %v", err)
			continue
		}

		c.FCMToken = fcmToken.String
		c.PhoneNumber = phone.String
		c.Email = emailAddr.String

		contacts = append(contacts, c)
	}

	return contacts, nil
}

// saveEscalationLog saves the escalation result to database for audit
func (s *EscalationService) saveEscalationLog(result *EscalationResult) {
	if s.db == nil {
		return
	}

	query := `
		INSERT INTO escalation_logs (
			alert_id, elder_name, reason, priority,
			acknowledged, acknowledged_by, acknowledged_at,
			final_channel, attempts_count, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	var ackAt *time.Time
	if result.Acknowledged {
		ackAt = &result.AcknowledgedAt
	}

	_, err := s.db.Exec(query,
		result.AlertID, result.ElderName, result.Reason, string(result.Priority),
		result.Acknowledged, result.AcknowledgedBy, ackAt,
		string(result.FinalChannel), len(result.Attempts),
		result.StartedAt, result.CompletedAt,
	)

	if err != nil {
		log.Printf("⚠️ Erro ao salvar log de escalonamento: %v", err)
	}
}

// GetActiveAlerts returns all currently active (unacknowledged) alerts
func (s *EscalationService) GetActiveAlerts() []*EscalationResult {
	var active []*EscalationResult
	s.activeAlerts.Range(func(key, value interface{}) bool {
		result := value.(*EscalationResult)
		if !result.Acknowledged {
			active = append(active, result)
		}
		return true
	})
	return active
}
