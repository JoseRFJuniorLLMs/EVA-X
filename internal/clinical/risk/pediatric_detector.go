package risk

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"

	"github.com/rs/zerolog/log"
)

// PediatricRiskDetector detects self-harm risk in children's natural language
type PediatricRiskDetector struct {
	db    *sql.DB
	neo4j *graph.Neo4jClient

	// Metaphor patterns by risk level
	metaphors map[string]float64

	// Age-adapted patterns
	agePatterns map[string][]string // age_range -> patterns
}

// NewPediatricRiskDetector creates a new pediatric risk detector
func NewPediatricRiskDetector(db *sql.DB, neo4j *graph.Neo4jClient) *PediatricRiskDetector {
	return &PediatricRiskDetector{
		db:    db,
		neo4j: neo4j,
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

	// Check recent conversation history
	var recentStatements []string
	rows, err := p.db.QueryContext(ctx, `
		SELECT content 
		FROM messages 
		WHERE conversation_id IN (
			SELECT id FROM conversations WHERE patient_id = $1
		)
		ORDER BY created_at DESC 
		LIMIT 20
	`, patientID)

	if err != nil {
		return 0.0, factors
	}
	defer rows.Close()

	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			recentStatements = append(recentStatements, strings.ToLower(content))
		}
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

	return p.db.QueryRowContext(ctx, `
		INSERT INTO risk_detections (
			patient_id, session_id, statement, risk_level, risk_score,
			detected_metaphors, contextual_factors, age, recommended_action, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, assessment.PatientID, assessment.SessionID, assessment.Statement,
		assessment.RiskLevel, assessment.RiskScore, metaphorsJSON, factorsJSON,
		assessment.Age, assessment.RecommendedAction, assessment.CreatedAt,
	).Scan(&assessment.ID)
}

// GetRecentAssessments retrieves recent risk assessments for a patient
func (p *PediatricRiskDetector) GetRecentAssessments(ctx context.Context, patientID int64, limit int) ([]*RiskAssessment, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, patient_id, session_id, statement, risk_level, risk_score,
		       detected_metaphors, contextual_factors, age, recommended_action, created_at
		FROM risk_detections
		WHERE patient_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, patientID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assessments []*RiskAssessment
	for rows.Next() {
		var a RiskAssessment
		var metaphorsJSON, factorsJSON []byte

		err := rows.Scan(
			&a.ID, &a.PatientID, &a.SessionID, &a.Statement, &a.RiskLevel, &a.RiskScore,
			&metaphorsJSON, &factorsJSON, &a.Age, &a.RecommendedAction, &a.CreatedAt,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(metaphorsJSON, &a.DetectedMetaphors)
		json.Unmarshal(factorsJSON, &a.ContextualFactors)

		assessments = append(assessments, &a)
	}

	return assessments, rows.Err()
}

// GetRiskTrend analyzes risk trend over time
func (p *PediatricRiskDetector) GetRiskTrend(ctx context.Context, patientID int64, days int) ([]float64, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT risk_score
		FROM risk_detections
		WHERE patient_id = $1
		AND created_at > NOW() - INTERVAL '$2 days'
		ORDER BY created_at ASC
	`, patientID, days)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []float64
	for rows.Next() {
		var score float64
		if err := rows.Scan(&score); err == nil {
			scores = append(scores, score)
		}
	}

	return scores, rows.Err()
}
