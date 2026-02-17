// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

import (
	"math"
	"time"
)

// TargetQuality represents how easy it is to judge a specific user
type TargetQuality struct {
	UserID           int64
	Expressiveness   float64 // 0-1: how expressive the user is (tone variation, emotions)
	Consistency      float64 // 0-1: how consistent behavior is across sessions
	PsychologicalAdj float64 // 0-1: psychological adjustment/stability
	EaseOfJudgment   float64 // 0-1: overall ease of judging this user
	LastUpdated      time.Time
	SessionCount     int
}

// SessionData represents data from a single session
type SessionData struct {
	SessionID    string
	UserID       int64
	Timestamp    time.Time
	Duration     time.Duration
	ToneVariance float64   // Variance in tone throughout session
	EmotionCount int       // Number of distinct emotions detected
	EnergyLevels []float64 // Energy levels throughout session
	Behaviors    []string  // Observed behaviors
}

// CalculateExpressiveness calculates how expressive a user is based on session data
func CalculateExpressiveness(sessions []SessionData) float64 {
	if len(sessions) == 0 {
		return 0.5 // Default to medium expressiveness
	}

	totalToneVariance := 0.0
	totalEmotions := 0
	totalEnergyVariance := 0.0

	for _, session := range sessions {
		// Tone variance contributes to expressiveness
		totalToneVariance += session.ToneVariance

		// Number of distinct emotions
		totalEmotions += session.EmotionCount

		// Energy variance
		if len(session.EnergyLevels) > 1 {
			totalEnergyVariance += calculateVariance(session.EnergyLevels)
		}
	}

	avgToneVariance := totalToneVariance / float64(len(sessions))
	avgEmotions := float64(totalEmotions) / float64(len(sessions))
	avgEnergyVariance := totalEnergyVariance / float64(len(sessions))

	// Normalize and combine
	// High variance and many emotions = high expressiveness
	toneScore := math.Min(avgToneVariance*2, 1.0)     // Normalize to 0-1
	emotionScore := math.Min(avgEmotions/5.0, 1.0)    // 5+ emotions = max score
	energyScore := math.Min(avgEnergyVariance*3, 1.0) // Normalize to 0-1

	// Weighted average
	expressiveness := (toneScore*0.4 + emotionScore*0.4 + energyScore*0.2)

	return expressiveness
}

// CalculateConsistency calculates behavioral consistency across sessions
func CalculateConsistency(sessions []SessionData) float64 {
	if len(sessions) < 2 {
		return 0.5 // Not enough data, assume medium consistency
	}

	// Compare behaviors across sessions
	behaviorSets := make([]map[string]bool, len(sessions))
	for i, session := range sessions {
		behaviorSets[i] = make(map[string]bool)
		for _, behavior := range session.Behaviors {
			behaviorSets[i][behavior] = true
		}
	}

	// Calculate Jaccard similarity between consecutive sessions
	similarities := []float64{}
	for i := 1; i < len(behaviorSets); i++ {
		similarity := jaccardSimilarity(behaviorSets[i-1], behaviorSets[i])
		similarities = append(similarities, similarity)
	}

	// High average similarity = high consistency
	avgSimilarity := calculateAverage(similarities)

	// Also consider variance in tone and energy across sessions
	toneVariances := []float64{}
	energyMeans := []float64{}

	for _, session := range sessions {
		toneVariances = append(toneVariances, session.ToneVariance)
		if len(session.EnergyLevels) > 0 {
			energyMeans = append(energyMeans, calculateAverage(session.EnergyLevels))
		}
	}

	// Low variance in session-level metrics = high consistency
	toneConsistency := 1.0 - math.Min(calculateVariance(toneVariances), 1.0)
	energyConsistency := 1.0 - math.Min(calculateVariance(energyMeans), 1.0)

	// Combine behavioral and metric consistency
	consistency := (avgSimilarity*0.5 + toneConsistency*0.25 + energyConsistency*0.25)

	return consistency
}

// CalculateEaseOfJudgment calculates overall ease of judging this user
func CalculateEaseOfJudgment(userID int64, sessions []SessionData) TargetQuality {
	expressiveness := CalculateExpressiveness(sessions)
	consistency := CalculateConsistency(sessions)

	// Psychological adjustment (estimated from session data)
	// More stable emotions and energy = higher adjustment
	psychAdj := estimatePsychologicalAdjustment(sessions)

	// Ease of judgment is a weighted combination
	// High expressiveness + high consistency = easy to judge
	// Low psychological adjustment makes it harder (more unpredictable)
	easeOfJudgment := (expressiveness*0.4 + consistency*0.4 + psychAdj*0.2)

	return TargetQuality{
		UserID:           userID,
		Expressiveness:   expressiveness,
		Consistency:      consistency,
		PsychologicalAdj: psychAdj,
		EaseOfJudgment:   easeOfJudgment,
		LastUpdated:      time.Now(),
		SessionCount:     len(sessions),
	}
}

// AdjustConfidence adjusts base confidence based on target quality
func AdjustConfidence(baseConfidence float64, targetQuality TargetQuality) float64 {
	// If user is hard to judge (low ease of judgment), reduce confidence
	if targetQuality.EaseOfJudgment < 0.5 {
		// Reduce confidence by up to 40% for very difficult targets
		reduction := (0.5 - targetQuality.EaseOfJudgment) * 0.8
		return baseConfidence * (1.0 - reduction)
	}

	// If user is easy to judge, slightly boost confidence
	if targetQuality.EaseOfJudgment > 0.7 {
		boost := (targetQuality.EaseOfJudgment - 0.7) * 0.3
		return math.Min(baseConfidence*(1.0+boost), 1.0)
	}

	// Medium ease of judgment = no adjustment
	return baseConfidence
}

// GetQualityNote generates a human-readable note about target quality
func GetQualityNote(targetQuality TargetQuality) string {
	if targetQuality.EaseOfJudgment < 0.3 {
		return "Usuário é muito difícil de ler - comportamento inconsistente e pouco expressivo"
	} else if targetQuality.EaseOfJudgment < 0.5 {
		return "Usuário é difícil de ler - pode ser reservado ou ter comportamento variável"
	} else if targetQuality.EaseOfJudgment > 0.8 {
		return "Usuário é fácil de ler - expressivo e consistente"
	}
	return "Usuário tem expressividade moderada"
}

// Helper functions

func estimatePsychologicalAdjustment(sessions []SessionData) float64 {
	if len(sessions) == 0 {
		return 0.5
	}

	// Collect all energy levels across sessions
	allEnergy := []float64{}
	for _, session := range sessions {
		allEnergy = append(allEnergy, session.EnergyLevels...)
	}

	// Low variance in energy = more stable = higher adjustment
	energyVar := calculateVariance(allEnergy)
	stabilityScore := 1.0 - math.Min(energyVar, 1.0)

	// Fewer extreme emotions = higher adjustment
	extremeEmotionCount := 0
	for _, session := range sessions {
		if session.EmotionCount > 5 {
			extremeEmotionCount++
		}
	}
	emotionStability := 1.0 - (float64(extremeEmotionCount) / float64(len(sessions)))

	// Combine
	psychAdj := (stabilityScore*0.6 + emotionStability*0.4)

	return psychAdj
}

func calculateTraitConsistency(values []float64) float64 {
	if len(values) < 2 {
		return 1.0
	}

	v := calculateVariance(values)
	// Higher variance = lower consistency
	consistency := 1.0 - v
	if consistency < 0 {
		return 0.0
	}
	return consistency
}

func jaccardSimilarity(set1, set2 map[string]bool) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 1.0
	}

	intersection := 0
	union := make(map[string]bool)

	for k := range set1 {
		union[k] = true
	}
	for k := range set2 {
		union[k] = true
		if set1[k] {
			intersection++
		}
	}

	if len(union) == 0 {
		return 0
	}

	return float64(intersection) / float64(len(union))
}
