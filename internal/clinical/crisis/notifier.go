package crisis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// Notifier handles multi-channel notifications
type Notifier struct {
	webhookURL string
	emailAPI   string
	smsAPI     string
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		webhookURL: os.Getenv("CRISIS_WEBHOOK_URL"),
		emailAPI:   os.Getenv("EMAIL_API_URL"),
		smsAPI:     os.Getenv("SMS_API_URL"),
	}
}

// NotificationPayload represents notification data
type NotificationPayload struct {
	EventID          int64     `json:"event_id"`
	PatientID        int64     `json:"patient_id"`
	CrisisType       string    `json:"crisis_type"`
	Severity         string    `json:"severity"`
	TriggerStatement string    `json:"trigger_statement"`
	Timestamp        time.Time `json:"timestamp"`
	RequiresAction   bool      `json:"requires_action"`
}

// NotifyPsychologist sends notification to psychologist
func (n *Notifier) NotifyPsychologist(ctx context.Context, event *CrisisEvent) error {
	payload := NotificationPayload{
		EventID:          event.ID,
		PatientID:        event.PatientID,
		CrisisType:       string(event.CrisisType),
		Severity:         event.Severity,
		TriggerStatement: event.TriggerStatement,
		Timestamp:        event.CreatedAt,
		RequiresAction:   event.ResponseActions["require_acknowledgment"],
	}

	// 1. Send webhook (real-time)
	if n.webhookURL != "" {
		err := n.sendWebhook(ctx, payload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send webhook")
		}
	}

	// 2. Send email (backup)
	if n.emailAPI != "" {
		err := n.sendEmail(ctx, payload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send email")
		}
	}

	// 3. Send SMS (critical only)
	if event.Severity == "CRITICAL" && n.smsAPI != "" {
		err := n.sendSMS(ctx, payload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send SMS")
		}
	}

	return nil
}

// sendWebhook sends webhook notification
func (n *Notifier) sendWebhook(ctx context.Context, payload NotificationPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Crisis-Alert", "true")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Info().
		Int64("event_id", payload.EventID).
		Str("severity", payload.Severity).
		Msg("Webhook notification sent")

	return nil
}

// sendEmail sends email notification
func (n *Notifier) sendEmail(ctx context.Context, payload NotificationPayload) error {
	// TODO: Integrate with SendGrid, AWS SES, or similar

	emailBody := fmt.Sprintf(`
🚨 CRISIS ALERT

Event ID: %d
Patient ID: %d
Crisis Type: %s
Severity: %s

Trigger Statement: "%s"

Time: %s

%s
	`,
		payload.EventID,
		payload.PatientID,
		payload.CrisisType,
		payload.Severity,
		payload.TriggerStatement,
		payload.Timestamp.Format(time.RFC3339),
		func() string {
			if payload.RequiresAction {
				return "⚠️ REQUIRES IMMEDIATE ACKNOWLEDGMENT"
			}
			return ""
		}(),
	)

	log.Info().
		Int64("event_id", payload.EventID).
		Msg("Email notification sent (simulated)")

	// TODO: Actually send email
	_ = emailBody

	return nil
}

// sendSMS sends SMS notification
func (n *Notifier) sendSMS(ctx context.Context, payload NotificationPayload) error {
	// TODO: Integrate with Twilio or similar

	smsBody := fmt.Sprintf(
		"🚨 CRISIS ALERT: %s (%s) - Patient %d - Requires immediate attention",
		payload.CrisisType,
		payload.Severity,
		payload.PatientID,
	)

	log.Info().
		Int64("event_id", payload.EventID).
		Msg("SMS notification sent (simulated)")

	// TODO: Actually send SMS
	_ = smsBody

	return nil
}
