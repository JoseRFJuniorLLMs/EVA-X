package personality

import (
	"fmt"
	"math"
	"time"
)

// PersonalityTrajectory tracks personality evolution over time
type PersonalityTrajectory struct {
	UserID          int64
	Snapshots       []TrajectorySnapshot
	BaselineProfile BigFiveProfile // First 10 sessions
	CurrentProfile  BigFiveProfile
	StabilityIndex  float64 // 0-1: how stable personality is
	Anomalies       []AnomalyEvent
	LastUpdated     time.Time
}

// TrajectorySnapshot represents personality at a point in time
type TrajectorySnapshot struct {
	Timestamp time.Time
	Profile   BigFiveProfile
	SessionID string
}

// AnomalyEvent represents an abnormal personality change
type AnomalyEvent struct {
	Trait         string
	OldValue      float64
	NewValue      float64
	ChangeAmount  float64
	ChangePeriod  time.Duration
	Severity      string // "LOW", "MEDIUM", "HIGH", "CRITICAL"
	PossibleCause string
	Timestamp     time.Time
}

// ClinicalAlert represents an alert for caregivers
type ClinicalAlert struct {
	UserID      int64
	Severity    string
	Message     string
	Anomaly     AnomalyEvent
	Timestamp   time.Time
	ActionTaken string
}

// GetPersonalityTrajectory retrieves the full trajectory for a user
func GetPersonalityTrajectory(userID int64, snapshots []TrajectorySnapshot) PersonalityTrajectory {
	if len(snapshots) == 0 {
		return PersonalityTrajectory{
			UserID:      userID,
			Snapshots:   []TrajectorySnapshot{},
			LastUpdated: time.Now(),
		}
	}

	// Baseline is average of first 10 sessions (or all if < 10)
	baselineCount := minInt(10, len(snapshots))
	baseline := averageBigFive(snapshots[:baselineCount])

	current := snapshots[len(snapshots)-1].Profile

	// Calculate stability
	stability := calculateStability(snapshots)

	// Detect anomalies
	anomalies := DetectAnomalies(PersonalityTrajectory{
		UserID:          userID,
		Snapshots:       snapshots,
		BaselineProfile: baseline,
		CurrentProfile:  current,
		StabilityIndex:  stability,
	})

	return PersonalityTrajectory{
		UserID:          userID,
		Snapshots:       snapshots,
		BaselineProfile: baseline,
		CurrentProfile:  current,
		StabilityIndex:  stability,
		Anomalies:       anomalies,
		LastUpdated:     time.Now(),
	}
}

// DetectAnomalies detects abnormal personality changes
func DetectAnomalies(trajectory PersonalityTrajectory) []AnomalyEvent {
	anomalies := []AnomalyEvent{}

	if len(trajectory.Snapshots) < 2 {
		return anomalies
	}

	baseline := trajectory.BaselineProfile
	current := trajectory.CurrentProfile

	// Check each Big Five dimension
	traits := map[string]struct{ old, new float64 }{
		"Openness":          {baseline.Openness, current.Openness},
		"Conscientiousness": {baseline.Conscientiousness, current.Conscientiousness},
		"Extraversion":      {baseline.Extraversion, current.Extraversion},
		"Agreeableness":     {baseline.Agreeableness, current.Agreeableness},
		"Neuroticism":       {baseline.Neuroticism, current.Neuroticism},
	}

	timeSinceBaseline := time.Since(trajectory.Snapshots[0].Timestamp)

	for trait, values := range traits {
		change := values.new - values.old

		// Significant change threshold: >0.30 in any trait
		if math.Abs(change) > 0.30 {
			severity := determineSeverity(change, timeSinceBaseline)
			possibleCause := InferCause(trait, change, []LifeEvent{})

			anomalies = append(anomalies, AnomalyEvent{
				Trait:         trait,
				OldValue:      values.old,
				NewValue:      values.new,
				ChangeAmount:  change,
				ChangePeriod:  timeSinceBaseline,
				Severity:      severity,
				PossibleCause: possibleCause,
				Timestamp:     time.Now(),
			})
		}
	}

	return anomalies
}

// InferCause infers possible cause of personality change
func InferCause(trait string, change float64, events []LifeEvent) string {
	// Neuroticismo ↑↑ + evento "morte_conjuge" = luto patológico
	if trait == "Neuroticism" && change > 0.40 {
		for _, event := range events {
			if event.Type == "morte_conjuge" || event.Type == "morte_familiar" {
				return "luto_nao_resolvido"
			}
		}
		return "depressao_emergente"
	}

	// Conscientiousness ↓↓ súbito = possível demência
	if trait == "Conscientiousness" && change < -0.35 {
		return "declinio_cognitivo_possivel"
	}

	// Extraversion ↓↓ = possível isolamento social ou depressão
	if trait == "Extraversion" && change < -0.35 {
		return "isolamento_social_ou_depressao"
	}

	// Agreeableness ↓↓ = possível irritabilidade por dor ou frustração
	if trait == "Agreeableness" && change < -0.35 {
		return "irritabilidade_dor_ou_frustracao"
	}

	// Openness ↓↓ = possível rigidez cognitiva
	if trait == "Openness" && change < -0.30 {
		return "rigidez_cognitiva"
	}

	return "desconhecido"
}

// GenerateAlerts generates clinical alerts from anomalies
func GenerateAlerts(anomalies []AnomalyEvent, userID int64) []ClinicalAlert {
	alerts := []ClinicalAlert{}

	for _, anomaly := range anomalies {
		if anomaly.Severity == "CRITICAL" || anomaly.Severity == "HIGH" {
			message := fmt.Sprintf(
				"Mudança %s em %s: %.2f → %.2f (Δ%+.2f) em %s. Causa provável: %s",
				anomaly.Severity,
				anomaly.Trait,
				anomaly.OldValue,
				anomaly.NewValue,
				anomaly.ChangeAmount,
				formatDuration(anomaly.ChangePeriod),
				anomaly.PossibleCause,
			)

			actionTaken := determineAction(anomaly)

			alerts = append(alerts, ClinicalAlert{
				UserID:      userID,
				Severity:    anomaly.Severity,
				Message:     message,
				Anomaly:     anomaly,
				Timestamp:   time.Now(),
				ActionTaken: actionTaken,
			})
		}
	}

	return alerts
}

// Helper functions

func calculateStability(snapshots []TrajectorySnapshot) float64 {
	if len(snapshots) < 2 {
		return 1.0 // Perfectly stable (no variance)
	}

	// Calculate variance in each dimension
	variances := []float64{}

	// Extract all values for each dimension
	openness := []float64{}
	conscientiousness := []float64{}
	extraversion := []float64{}
	agreeableness := []float64{}
	neuroticism := []float64{}

	for _, snapshot := range snapshots {
		openness = append(openness, snapshot.Profile.Openness)
		conscientiousness = append(conscientiousness, snapshot.Profile.Conscientiousness)
		extraversion = append(extraversion, snapshot.Profile.Extraversion)
		agreeableness = append(agreeableness, snapshot.Profile.Agreeableness)
		neuroticism = append(neuroticism, snapshot.Profile.Neuroticism)
	}

	variances = append(variances, calculateVariance(openness))
	variances = append(variances, calculateVariance(conscientiousness))
	variances = append(variances, calculateVariance(extraversion))
	variances = append(variances, calculateVariance(agreeableness))
	variances = append(variances, calculateVariance(neuroticism))

	// Average variance
	avgVariance := calculateAverage(variances)

	// Stability = 1 - variance (normalized)
	stability := 1.0 - math.Min(avgVariance, 1.0)

	return stability
}

func determineSeverity(change float64, period time.Duration) string {
	absChange := math.Abs(change)

	// Critical: >0.40 change in <1 month
	if absChange > 0.40 && period < 30*24*time.Hour {
		return "CRITICAL"
	}

	// High: >0.35 change in <2 months
	if absChange > 0.35 && period < 60*24*time.Hour {
		return "HIGH"
	}

	// Medium: >0.30 change in <3 months
	if absChange > 0.30 && period < 90*24*time.Hour {
		return "MEDIUM"
	}

	return "LOW"
}

func determineAction(anomaly AnomalyEvent) string {
	switch anomaly.PossibleCause {
	case "depressao_emergente":
		return "Switched persona to psychologist mode"
	case "declinio_cognitivo_possivel":
		return "Alert sent to caregivers + medical evaluation recommended"
	case "luto_nao_resolvido":
		return "Grief counseling protocol activated"
	default:
		return "Monitoring increased"
	}
}

func averageBigFive(snapshots []TrajectorySnapshot) BigFiveProfile {
	if len(snapshots) == 0 {
		return BigFiveProfile{}
	}

	sumO, sumC, sumE, sumA, sumN := 0.0, 0.0, 0.0, 0.0, 0.0

	for _, snapshot := range snapshots {
		sumO += snapshot.Profile.Openness
		sumC += snapshot.Profile.Conscientiousness
		sumE += snapshot.Profile.Extraversion
		sumA += snapshot.Profile.Agreeableness
		sumN += snapshot.Profile.Neuroticism
	}

	count := float64(len(snapshots))

	return BigFiveProfile{
		Openness:          sumO / count,
		Conscientiousness: sumC / count,
		Extraversion:      sumE / count,
		Agreeableness:     sumA / count,
		Neuroticism:       sumN / count,
		Confidence:        0.80,
		LastUpdated:       time.Now(),
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days < 7 {
		return fmt.Sprintf("%d dias", days)
	} else if days < 30 {
		return fmt.Sprintf("%d semanas", days/7)
	} else {
		return fmt.Sprintf("%d meses", days/30)
	}
}
