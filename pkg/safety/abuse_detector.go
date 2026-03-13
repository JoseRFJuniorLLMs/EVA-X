// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	nietzsche "nietzsche-sdk"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// AlertFamilyFunc sends real notifications (push + email + SMS) to caregivers.
// Matches the same signature as swarm.AlertFunc so callers can pass deps.AlertFamily directly.
type AlertFamilyFunc func(ctx context.Context, userID int64, reason, severity string) error

// AbuseDetector scans user input for signs of abuse or danger
type AbuseDetector struct {
	keywords     []string
	notifier     *GuardianNotifier
	emergencyLog *EmergencyLogger
	enabled      bool
	alertFamily  AlertFamilyFunc
	ndb          *nietzscheInfra.Client
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

// notifyAuthorities sends real notifications to caregivers and persists the alert in NietzscheDB.
func (d *AbuseDetector) notifyAuthorities(userID string, result *ScanResult) error {
	message := fmt.Sprintf(
		"ALERTA DE SEGURANÇA: Detectado possível risco para menor (keyword: %s). Severidade: %s. Ação imediata necessária.",
		result.MatchedKeyword,
		result.Severity,
	)

	var firstErr error

	// 1. Send real notification via AlertFamily (push + email + SMS)
	if d.alertFamily != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Map Severity to the string format expected by AlertFamily
		severityStr := "alta"
		if result.Severity == SeverityCritical {
			severityStr = "critica"
		}

		// Parse userID to int64 for AlertFamily; use 0 if unparseable
		var uid int64
		fmt.Sscanf(userID, "%d", &uid)

		if err := d.alertFamily(ctx, uid, message, severityStr); err != nil {
			log.Printf("ABUSE ALERT: Failed to send real notification for user %s: %v", userID, err)
			firstErr = fmt.Errorf("alertFamily failed: %w", err)
		} else {
			log.Printf("ABUSE ALERT: Real notification sent for user %s (severity=%s)", userID, result.Severity)
		}
	} else {
		log.Printf("WARNING: AlertFamily not configured — abuse notification NOT sent for user %s", userID)
	}

	// 2. Fallback: also log via legacy GuardianNotifier if configured
	if d.notifier != nil {
		if err := d.notifier.NotifyGuardian(userID, message, true); err != nil {
			log.Printf("ABUSE ALERT: GuardianNotifier fallback failed: %v", err)
		}
	}

	// 3. Persist abuse alert as a node in NietzscheDB (patient_graph collection)
	if d.ndb != nil {
		if err := d.persistAbuseAlert(userID, result); err != nil {
			log.Printf("ABUSE ALERT: Failed to persist alert in NietzscheDB: %v", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("persistAbuseAlert failed: %w", err)
			}
		}
	} else {
		log.Printf("WARNING: NietzscheDB client not configured — abuse alert NOT persisted for user %s", userID)
	}

	return firstErr
}

// persistAbuseAlert stores the abuse detection as a Semantic node with edge in NietzscheDB.
func (d *AbuseDetector) persistAbuseAlert(userID string, result *ScanResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build content payload
	content := map[string]interface{}{
		"node_label": "AbuseAlert",
		"type":       "abuse_detection",
		"severity":   string(result.Severity),
		"evidence":   result.MatchedKeyword,
		"timestamp":  result.Timestamp.Format(time.RFC3339),
		"patient_id": userID,
	}

	// Generate 3072-dim zero coords for relational data (patient_graph is 3072D poincare)
	coords := make([]float64, 3072)

	// Insert the abuse alert node
	nodeResult, err := d.ndb.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType:   "Semantic", // node_label "AbuseAlert" is in content
		Content:    content,
		Coords:     coords,
		Energy:     1.0,
		Collection: "patient_graph",
	})
	if err != nil {
		return fmt.Errorf("InsertNode AbuseAlert: %w", err)
	}

	contentJSON, _ := json.Marshal(content)
	log.Printf("ABUSE ALERT: Persisted node %s in patient_graph: %s", nodeResult.ID, string(contentJSON))

	// Create edge: patient → abuse alert (ABUSE_DETECTED)
	// The userID may be a node ID in patient_graph; create edge if it looks like a valid reference
	if userID != "" {
		_, err = d.ndb.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:       userID,
			To:         nodeResult.ID,
			EdgeType:   "Association",
			Weight:     1.0,
			Collection: "patient_graph",
		})
		if err != nil {
			// Edge creation failure is non-fatal — the node is already persisted
			log.Printf("ABUSE ALERT: Failed to create ABUSE_DETECTED edge (from=%s to=%s): %v", userID, nodeResult.ID, err)
		} else {
			log.Printf("ABUSE ALERT: Created ABUSE_DETECTED edge from %s to %s", userID, nodeResult.ID)
		}
	}

	return nil
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

// SetAlertFamily configures the real notification function (push + email + SMS).
// Pass deps.AlertFamily from the swarm Dependencies.
func (d *AbuseDetector) SetAlertFamily(fn AlertFamilyFunc) {
	d.alertFamily = fn
}

// SetNietzscheClient configures NietzscheDB client for persisting abuse alerts.
func (d *AbuseDetector) SetNietzscheClient(client *nietzscheInfra.Client) {
	d.ndb = client
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
