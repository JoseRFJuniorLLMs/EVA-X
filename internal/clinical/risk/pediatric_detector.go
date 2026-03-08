// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package risk

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

// PediatricRiskDetector detects self-harm risk in children's natural language
type PediatricRiskDetector struct {
	db *database.DB

	// Metaphor patterns by risk level
	metaphors map[string]float64

	// Age-adapted patterns
	agePatterns map[string][]string // age_range -> patterns
}

// NewPediatricRiskDetector creates a new pediatric risk detector
func NewPediatricRiskDetector(db *database.DB) *PediatricRiskDetector {
	return &PediatricRiskDetector{
		db: db,
		metaphors: map[string]float64{
			// Death metaphors
			"virar estrela":        0.7,
			"ir pro céu":           0.6,
			"dormir pra sempre":    0.9,
			"nunca mais acordar":   0.9,
			"sumir":                0.5,
			"desaparecer":          0.5,
			"ir embora pra sempre": 0.6,

			// Self-worth indicators
			"todo mundo seria mais feliz sem mim": 0.8,
			"ninguém ia sentir minha falta":       0.8,
			"sou um peso":                         0.7,
			"sou inútil":                          0.6,
			"não sirvo pra nada":                  0.7,
			"sou burro":                           0.4,
			"sou bobão":                           0.3,

			// Hopelessness
			"nada vai melhorar":    0.6,
			"sempre vai ser assim": 0.5,
			"não tem jeito":        0.5,
			"não adianta":          0.4,

			// Projection onto objects
			"minha boneca também tá triste": 0.5,
			"meu ursinho quer ir embora":    0.6,
			"meu brinquedo tá cansado":      0.4,

			// Direct statements (rare in children)
			"quero morrer":   1.0,
			"quero me matar": 1.0,
			"vou me matar":   1.0,
		},
		agePatterns: map[string][]string{
			"5-7": {
				"magical thinking", "animism", "projection onto toys",
			},
			"8-10": {
				"metaphors", "indirect statements", "self-worth issues",
			},
			"11-13": {
				"more direct", "hopelessness", "social comparison",
			},
		},
	}
}

// RiskAssessment represents a risk assessment result
type RiskAssessment struct {
	ID                int64     `json:"id"`
	PatientID         int64     `json:"patient_id"`
	SessionID         int64     `json:"session_id"`
	Statement         string    `json:"statement"`
	RiskLevel         string    `json:"risk_level"` // NONE, LOW, MODERATE, HIGH, CRITICAL
	RiskScore         float64   `json:"risk_score"` // 0-1
	DetectedMetaphors []string  `json:"detected_metaphors"`
	ContextualFactors []string  `json:"contextual_factors"`
	Age               int       `json:"age"`
	RecommendedAction string    `json:"recommended_action"`
	CreatedAt         time.Time `json:"created_at"`
}

// AnalyzeStatement analyzes a child's statement for risk indicators
func (p *PediatricRiskDetector) AnalyzeStatement(ctx context.Context, statement string, patientID, sessionID int64, age int) (*RiskAssessment, error) {
	assessment := &RiskAssessment{
		PatientID:         patientID,
		SessionID:         sessionID,
		Statement:         statement,
		Age:               age,
		DetectedMetaphors: []string{},
		ContextualFactors: []string{},
		CreatedAt:         time.Now(),
	}

	// 1. Check for metaphor patterns
	statementLower := strings.ToLower(statement)
	maxScore := 0.0

	for metaphor, score := range p.metaphors {
		if strings.Contains(statementLower, metaphor) {
			assessment.DetectedMetaphors = append(assessment.DetectedMetaphors, metaphor)
			if score > maxScore {
				maxScore = score
			}
		}
	}

	// 2. Get contextual factors from history
	contextScore, factors := p.getContextualScore(ctx, patientID, statementLower)
	assessment.ContextualFactors = factors

	// 3. Calculate final risk score
	// Base score from metaphors (70%) + contextual score (30%)
	assessment.RiskScore = (maxScore * 0.7) + (contextScore * 0.3)

	// 4. Determine risk level
	assessment.RiskLevel = p.getRiskLevel(assessment.RiskScore)

	// 5. Recommended action
	assessment.RecommendedAction = p.getRecommendedAction(assessment.RiskLevel, age)

	// 6. Store assessment
	err := p.storeAssessment(ctx, assessment)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store risk assessment")
	}

	// 7. Log critical alerts
	if assessment.RiskLevel == "HIGH" || assessment.RiskLevel == "CRITICAL" {
		log.Warn().
			Int64("patient_id", patientID).
			Str("risk_level", assessment.RiskLevel).
			Float64("risk_score", assessment.RiskScore).
			Strs("metaphors", assessment.DetectedMetaphors).
			Msg("🚨 PEDIATRIC RISK ALERT")
	}

	return assessment, nil
}

// getContextualScore analyzes conversation history for risk factors
func (p *PediatricRiskDetector) getContextualScore(ctx context.Context, patientID int64, statement string) (float64, []string) {
	factors := []string{}
	score := 0.0

	// Check recent conversation history: first get conversations for patient
	var recentStatements []string

	convRows, err := p.db.QueryByLabel(ctx, "conversations",
		" AND n.patient_id = $pid", map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return 0.0, factors
	}

	// Collect conversation IDs
	convIDs := make(map[int64]bool)
	for _, c := range convRows {
		convIDs[database.GetInt64(c, "id")] = true
	}

	// Get messages from those conversations, sorted by created_at DESC, limit 20
	msgRows, err := p.db.QueryByLabel(ctx, "messages", "", nil, 0)
	if err != nil {
		return 0.0, factors
	}

	// Filter messages belonging to patient's conversations
	type msgEntry struct {
		content   string
		createdAt time.Time
	}
	var msgs []msgEntry
	for _, m := range msgRows {
		cid := database.GetInt64(m, "conversation_id")
		if convIDs[cid] {
			msgs = append(msgs, msgEntry{
				content:   database.GetString(m, "content"),
				createdAt: database.GetTime(m, "created_at"),
			})
		}
	}

	// Sort by created_at DESC and limit 20
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].createdAt.After(msgs[j].createdAt)
	})
	if len(msgs) > 20 {
		msgs = msgs[:20]
	}

	for _, m := range msgs {
		recentStatements = append(recentStatements, strings.ToLower(m.content))
	}

	// Factor 1: Repeated themes of sadness
	sadnessCount := 0
	for _, stmt := range recentStatements {
		if strings.Contains(stmt, "triste") || strings.Contains(stmt, "choro") || strings.Contains(stmt, "chorar") {
			sadnessCount++
		}
	}
	if sadnessCount >= 3 {
		factors = append(factors, "repeated_sadness_theme")
		score += 0.2
	}

	// Factor 2: Isolation themes
	isolationCount := 0
	for _, stmt := range recentStatements {
		if strings.Contains(stmt, "sozinho") || strings.Contains(stmt, "ninguém") || strings.Contains(stmt, "sem amigos") {
			isolationCount++
		}
	}
	if isolationCount >= 2 {
		factors = append(factors, "isolation_theme")
		score += 0.3
	}

	// Factor 3: Loss themes
	if strings.Contains(statement, "perdi") || strings.Contains(statement, "foi embora") || strings.Contains(statement, "fugiu") {
		factors = append(factors, "recent_loss")
		score += 0.2
	}

	// Factor 4: Hopelessness pattern
	hopelessCount := 0
	for _, stmt := range recentStatements {
		if strings.Contains(stmt, "nunca") || strings.Contains(stmt, "sempre") || strings.Contains(stmt, "não adianta") {
			hopelessCount++
		}
	}
	if hopelessCount >= 2 {
		factors = append(factors, "hopelessness_pattern")
		score += 0.3
	}

	return score, factors
}

// getRiskLevel converts score to risk level
func (p *PediatricRiskDetector) getRiskLevel(score float64) string {
	switch {
	case score >= 0.8:
		return "CRITICAL"
	case score >= 0.6:
		return "HIGH"
	case score >= 0.4:
		return "MODERATE"
	case score >= 0.2:
		return "LOW"
	default:
		return "NONE"
	}
}

// getRecommendedAction provides action recommendations
func (p *PediatricRiskDetector) getRecommendedAction(level string, age int) string {
	switch level {
	case "CRITICAL":
		return "IMMEDIATE ACTION: Contact emergency services and parents. Do not leave child alone. Conduct full C-SSRS assessment."
	case "HIGH":
		return "URGENT: Contact parents immediately. Schedule emergency session within 24h. Consider hospitalization assessment."
	case "MODERATE":
		return "IMPORTANT: Discuss with supervisor. Contact parents within 48h. Increase session frequency."
	case "LOW":
		return "MONITOR: Document in clinical notes. Discuss in next session. Watch for pattern escalation."
	default:
		return "Continue regular monitoring."
	}
}

// storeAssessment stores risk assessment in database
func (p *PediatricRiskDetector) storeAssessment(ctx context.Context, assessment *RiskAssessment) error {
	metaphorsJSON, _ := json.Marshal(assessment.DetectedMetaphors)
	factorsJSON, _ := json.Marshal(assessment.ContextualFactors)

	id, err := p.db.Insert(ctx, "risk_detections", map[string]interface{}{
		"patient_id":         assessment.PatientID,
		"session_id":         assessment.SessionID,
		"statement":          assessment.Statement,
		"risk_level":         assessment.RiskLevel,
		"risk_score":         assessment.RiskScore,
		"detected_metaphors": string(metaphorsJSON),
		"contextual_factors": string(factorsJSON),
		"age":                assessment.Age,
		"recommended_action": assessment.RecommendedAction,
		"created_at":         assessment.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	assessment.ID = id
	return nil
}

// GetRecentAssessments retrieves recent risk assessments for a patient
func (p *PediatricRiskDetector) GetRecentAssessments(ctx context.Context, patientID int64, limit int) ([]*RiskAssessment, error) {
	rows, err := p.db.QueryByLabel(ctx, "risk_detections",
		" AND n.patient_id = $pid", map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return nil, err
	}

	var assessments []*RiskAssessment
	for _, m := range rows {
		a := &RiskAssessment{
			ID:                database.GetInt64(m, "id"),
			PatientID:         database.GetInt64(m, "patient_id"),
			SessionID:         database.GetInt64(m, "session_id"),
			Statement:         database.GetString(m, "statement"),
			RiskLevel:         database.GetString(m, "risk_level"),
			RiskScore:         database.GetFloat64(m, "risk_score"),
			Age:               int(database.GetInt64(m, "age")),
			RecommendedAction: database.GetString(m, "recommended_action"),
			CreatedAt:         database.GetTime(m, "created_at"),
		}

		var metaphors []string
		json.Unmarshal([]byte(database.GetString(m, "detected_metaphors")), &metaphors)
		a.DetectedMetaphors = metaphors

		var factors []string
		json.Unmarshal([]byte(database.GetString(m, "contextual_factors")), &factors)
		a.ContextualFactors = factors

		assessments = append(assessments, a)
	}

	// Sort by created_at DESC
	sort.Slice(assessments, func(i, j int) bool {
		return assessments[i].CreatedAt.After(assessments[j].CreatedAt)
	})

	// Apply limit
	if limit > 0 && len(assessments) > limit {
		assessments = assessments[:limit]
	}

	return assessments, nil
}

// GetRiskTrend analyzes risk trend over time
func (p *PediatricRiskDetector) GetRiskTrend(ctx context.Context, patientID int64, days int) ([]float64, error) {
	rows, err := p.db.QueryByLabel(ctx, "risk_detections",
		" AND n.patient_id = $pid", map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Filter by date range and collect with timestamps for sorting
	type scored struct {
		score     float64
		createdAt time.Time
	}
	var filtered []scored
	for _, m := range rows {
		createdAt := database.GetTime(m, "created_at")
		if createdAt.After(cutoff) {
			filtered = append(filtered, scored{
				score:     database.GetFloat64(m, "risk_score"),
				createdAt: createdAt,
			})
		}
	}

	// Sort by created_at ASC
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].createdAt.Before(filtered[j].createdAt)
	})

	scores := make([]float64, len(filtered))
	for i, f := range filtered {
		scores[i] = f.score
	}

	return scores, nil
}
