package safety

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// AbuseDetector scans user input for signs of abuse or danger
type AbuseDetector struct {
	keywords     []string
	notifier     *GuardianNotifier
	emergencyLog *EmergencyLogger
	enabled      bool
}

// NewAbuseDetector creates a new abuse detector
func NewAbuseDetector(notifier *GuardianNotifier, logger *EmergencyLogger) *AbuseDetector {
	return &AbuseDetector{
		keywords: []string{
			// Physical abuse indicators
			"me bateram",
			"me bateu",
			"machucou",
			"doeu",
			"sangue",
			"roxo",
			"marca",

			// Sexual abuse indicators (Portuguese)
			"tio tocou",
			"tia tocou",
			"tirar roupa",
			"segredo",
			"não conta",
			"tocar",
			"partes íntimas",

			// Emotional abuse
			"odeio viver",
			"quero morrer",
			"me matar",
			"suicídio",

			// Neglect
			"ninguém cuida",
			"sozinho em casa",
			"sem comida",
			"com fome",
		},
		notifier:     notifier,
		emergencyLog: logger,
		enabled:      true,
	}
}

// ScanResult represents the result of abuse detection scan
type ScanResult struct {
	IsAbuse        bool
	Severity       Severity
	MatchedKeyword string
	Timestamp      time.Time
	RequiresAction bool
}

// Severity levels for abuse detection
type Severity string

const (
	SeverityCritical Severity = "critical" // Immediate danger
	SeverityHigh     Severity = "high"     // Likely abuse
	SeverityMedium   Severity = "medium"   // Concerning
	SeverityLow      Severity = "low"      // Monitor
)

// Scan analyzes input for abuse indicators
func (d *AbuseDetector) Scan(userID string, input string, age int) (*ScanResult, error) {
	if !d.enabled {
		return &ScanResult{IsAbuse: false}, nil
	}

	// Only scan for minors
	if age >= 18 {
		return &ScanResult{IsAbuse: false}, nil
	}

	inputLower := strings.ToLower(input)

	// Check for keywords
	for _, keyword := range d.keywords {
		if strings.Contains(inputLower, keyword) {
			result := &ScanResult{
				IsAbuse:        true,
				Severity:       d.determineSeverity(keyword),
				MatchedKeyword: keyword,
				Timestamp:      time.Now(),
				RequiresAction: true,
			}

			// Log the incident
			if err := d.logIncident(userID, input, result); err != nil {
				log.Printf("Failed to log abuse incident: %v", err)
			}

			// Notify if critical
			if result.Severity == SeverityCritical || result.Severity == SeverityHigh {
				if err := d.notifyAuthorities(userID, result); err != nil {
					log.Printf("Failed to notify authorities: %v", err)
				}
			}

			return result, nil
		}
	}

	return &ScanResult{IsAbuse: false}, nil
}

// determineSeverity assigns severity level based on keyword
func (d *AbuseDetector) determineSeverity(keyword string) Severity {
	criticalKeywords := []string{
		"tio tocou",
		"tia tocou",
		"tirar roupa",
		"quero morrer",
		"me matar",
		"suicídio",
	}

	highKeywords := []string{
		"me bateram",
		"me bateu",
		"machucou",
		"segredo",
		"não conta",
	}

	for _, k := range criticalKeywords {
		if k == keyword {
			return SeverityCritical
		}
	}

	for _, k := range highKeywords {
		if k == keyword {
			return SeverityHigh
		}
	}

	return SeverityMedium
}

// logIncident logs the abuse detection incident
func (d *AbuseDetector) logIncident(userID string, input string, result *ScanResult) error {
	if d.emergencyLog == nil {
		return fmt.Errorf("emergency logger not configured")
	}

	return d.emergencyLog.Log(EmergencyLog{
		UserID:         userID,
		Timestamp:      result.Timestamp,
		Severity:       result.Severity,
		MatchedKeyword: result.MatchedKeyword,
		Input:          input,
		Type:           "abuse_detection",
	})
}

// notifyAuthorities sends notifications to guardian and possibly authorities
func (d *AbuseDetector) notifyAuthorities(userID string, result *ScanResult) error {
	if d.notifier == nil {
		return fmt.Errorf("guardian notifier not configured")
	}

	message := fmt.Sprintf(
		"ALERTA DE SEGURANÇA: Detectado possível risco para menor (keyword: %s). Severidade: %s. Ação imediata necessária.",
		result.MatchedKeyword,
		result.Severity,
	)

	return d.notifier.NotifyGuardian(userID, message, true)
}

// GetSafeResponse returns a safe response for the child
func (d *AbuseDetector) GetSafeResponse(severity Severity) string {
	switch severity {
	case SeverityCritical, SeverityHigh:
		return "Você está seguro comigo. Vou chamar alguém que pode te ajudar melhor, tá bom? Você não fez nada errado."

	case SeverityMedium:
		return "Obrigada por compartilhar isso comigo. Vou avisar alguém que cuida de você para te ajudar."

	default:
		return "Estou aqui para te ouvir. Você quer conversar sobre isso?"
	}
}

// Disable temporarily disables abuse detection (for testing only)
func (d *AbuseDetector) Disable() {
	d.enabled = false
	log.Println("WARNING: Abuse detector disabled")
}

// Enable re-enables abuse detection
func (d *AbuseDetector) Enable() {
	d.enabled = true
	log.Println("Abuse detector enabled")
}

// GuardianNotifier handles notifications to guardians
type GuardianNotifier struct {
	// TODO: Implement notification system (email, SMS, push)
}

// NotifyGuardian sends notification to guardian
func (n *GuardianNotifier) NotifyGuardian(userID string, message string, urgent bool) error {
	// TODO: Implement actual notification
	log.Printf("GUARDIAN NOTIFICATION [urgent=%v]: UserID=%s, Message=%s", urgent, userID, message)
	return nil
}

// EmergencyLogger logs emergency incidents
type EmergencyLogger struct {
	// TODO: Implement secure logging system
}

// EmergencyLog represents an emergency log entry
type EmergencyLog struct {
	UserID         string
	Timestamp      time.Time
	Severity       Severity
	MatchedKeyword string
	Input          string
	Type           string
}

// Log writes an emergency log entry
func (l *EmergencyLogger) Log(entry EmergencyLog) error {
	// TODO: Implement secure logging (encrypted, audit trail)
	log.Printf("EMERGENCY LOG: %+v", entry)
	return nil
}
