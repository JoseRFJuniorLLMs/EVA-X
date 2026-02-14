package personality

import (
	"time"
)

// JudgeQuality tracks EVA's ability to make accurate judgments
type JudgeQuality struct {
	TotalUsersServed   int
	Experience         float64   // 0-1 (normalized)
	Similarity         float64   // 0-1 (how similar user is to training data)
	HistoricalAccuracy float64   // 0-1 (rate of correct judgments)
	LastUpdated        time.Time
}

// Prediction represents a personality judgment made by EVA
type Prediction struct {
	UserID      int64
	Trait       string
	PredictedValue float64
	Confidence  float64
	Timestamp   time.Time
}

// Outcome represents the actual observed outcome
type Outcome struct {
	UserID       int64
	Trait        string
	ActualValue  float64
	WasCorrect   bool
	Timestamp    time.Time
}

// CalculateExperience calculates EVA's experience level
func CalculateExperience(totalUsers int) float64 {
	// Logarithmic scaling: experience grows slower as users increase
	// 10 users = 0.5, 100 users = 0.75, 1000 users = 0.90
	if totalUsers == 0 {
		return 0.0
	}
	
	experience := 1.0 - (1.0 / (1.0 + float64(totalUsers)/50.0))
	return min(experience, 1.0)
}

// CalculateSimilarity calculates how similar a user is to the training dataset
func CalculateSimilarity(userProfile PersonalityProfile) float64 {
	// This would ideally compare against a dataset of known profiles
	// For now, use a heuristic based on profile completeness and typicality
	
	// Check if profile matches common patterns
	enneaType := userProfile.EnneagramType
	
	// Common types in elderly population: 2, 6, 9
	commonTypes := map[int]bool{2: true, 6: true, 9: true}
	
	if commonTypes[enneaType] {
		return 0.80 // High similarity to common patterns
	}
	
	// Less common but still known types: 1, 4, 5
	if enneaType == 1 || enneaType == 4 || enneaType == 5 {
		return 0.60 // Medium similarity
	}
	
	// Rare types: 3, 7, 8
	return 0.40 // Lower similarity
}

// UpdateHistoricalAccuracy updates accuracy based on new feedback
func UpdateHistoricalAccuracy(currentAccuracy float64, prediction Prediction, outcome Outcome) float64 {
	// Calculate error
	error := abs(prediction.PredictedValue - outcome.ActualValue)
	wasAccurate := error < 0.20 // Within 20% = accurate
	
	// Update with exponential moving average (weight recent more)
	alpha := 0.1 // Learning rate
	newAccuracy := currentAccuracy
	
	if wasAccurate {
		newAccuracy = currentAccuracy*(1-alpha) + 1.0*alpha
	} else {
		newAccuracy = currentAccuracy*(1-alpha) + 0.0*alpha
	}
	
	return newAccuracy
}

// GetJudgeQuality calculates overall judge quality for a user
func GetJudgeQuality(userID int64, userProfile PersonalityProfile, totalUsers int, historicalAccuracy float64) JudgeQuality {
	experience := CalculateExperience(totalUsers)
	similarity := CalculateSimilarity(userProfile)
	
	return JudgeQuality{
		TotalUsersServed:   totalUsers,
		Experience:         experience,
		Similarity:         similarity,
		HistoricalAccuracy: historicalAccuracy,
		LastUpdated:        time.Now(),
	}
}

// AdjustConfidenceByJudgeQuality adjusts confidence based on judge quality
func AdjustConfidenceByJudgeQuality(baseConfidence float64, judgeQuality JudgeQuality) float64 {
	// Combine factors
	qualityMultiplier := (
		judgeQuality.Experience*0.3 +
		judgeQuality.Similarity*0.4 +
		judgeQuality.HistoricalAccuracy*0.3
	)
	
	return baseConfidence * qualityMultiplier
}

// GetJudgeQualityWarning generates a warning if judge quality is low
func GetJudgeQualityWarning(judgeQuality JudgeQuality) string {
	if judgeQuality.Similarity < 0.3 {
		return "Este perfil de usuário é novo para mim. Minha precisão pode ser menor."
	}
	
	if judgeQuality.Experience < 0.3 {
		return "Ainda estou aprendendo. Minha precisão melhorará com mais experiência."
	}
	
	if judgeQuality.HistoricalAccuracy < 0.6 {
		return "Minha taxa de acerto para perfis similares tem sido baixa. Proceda com cautela."
	}
	
	return ""
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
