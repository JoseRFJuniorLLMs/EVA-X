package personality

import (
	"time"
)

// BigFiveProfile represents the Big Five (OCEAN) personality dimensions
type BigFiveProfile struct {
	Openness          float64 // 0-1: Openness to experience
	Conscientiousness float64 // 0-1: Conscientiousness
	Extraversion      float64 // 0-1: Extraversion
	Agreeableness     float64 // 0-1: Agreeableness
	Neuroticism       float64 // 0-1: Neuroticism
	Confidence        float64 // 0-1: Confidence in this profile
	LastUpdated       time.Time
}

// PersonalityProfile combines Enneagram and Big Five
type PersonalityProfile struct {
	EnneagramType         int
	EnneagramDistribution map[int]float64
	BigFive               BigFiveProfile
	Confidence            float64
}

// EnneagramToBigFive maps Enneagram types to approximate Big Five profiles
var EnneagramToBigFive = map[int]BigFiveProfile{
	1: {Openness: 0.4, Conscientiousness: 0.9, Extraversion: 0.4, Agreeableness: 0.6, Neuroticism: 0.5, Confidence: 0.60, LastUpdated: time.Time{}},
	2: {Openness: 0.6, Conscientiousness: 0.6, Extraversion: 0.8, Agreeableness: 0.9, Neuroticism: 0.5, Confidence: 0.60, LastUpdated: time.Time{}},
	3: {Openness: 0.5, Conscientiousness: 0.9, Extraversion: 0.9, Agreeableness: 0.4, Neuroticism: 0.3, Confidence: 0.60, LastUpdated: time.Time{}},
	4: {Openness: 0.9, Conscientiousness: 0.3, Extraversion: 0.3, Agreeableness: 0.4, Neuroticism: 0.9, Confidence: 0.60, LastUpdated: time.Time{}},
	5: {Openness: 0.9, Conscientiousness: 0.5, Extraversion: 0.2, Agreeableness: 0.3, Neuroticism: 0.6, Confidence: 0.60, LastUpdated: time.Time{}},
	6: {Openness: 0.3, Conscientiousness: 0.8, Extraversion: 0.3, Agreeableness: 0.6, Neuroticism: 0.8, Confidence: 0.60, LastUpdated: time.Time{}},
	7: {Openness: 0.9, Conscientiousness: 0.3, Extraversion: 0.9, Agreeableness: 0.6, Neuroticism: 0.2, Confidence: 0.60, LastUpdated: time.Time{}},
	8: {Openness: 0.5, Conscientiousness: 0.6, Extraversion: 0.9, Agreeableness: 0.2, Neuroticism: 0.3, Confidence: 0.60, LastUpdated: time.Time{}},
	9: {Openness: 0.5, Conscientiousness: 0.4, Extraversion: 0.4, Agreeableness: 0.9, Neuroticism: 0.3, Confidence: 0.60, LastUpdated: time.Time{}},
}

// InferBigFiveFromEnneagram infers Big Five from Enneagram type
func InferBigFiveFromEnneagram(enneaType int) BigFiveProfile {
	if profile, exists := EnneagramToBigFive[enneaType]; exists {
		profile.Confidence = 0.60 // Medium confidence for inference
		profile.LastUpdated = time.Now()
		return profile
	}

	// Default profile if type unknown
	return BigFiveProfile{
		Openness:          0.5,
		Conscientiousness: 0.5,
		Extraversion:      0.5,
		Agreeableness:     0.5,
		Neuroticism:       0.5,
		Confidence:        0.30,
		LastUpdated:       time.Now(),
	}
}

// InferBigFiveFromBehavior infers Big Five from observed behavior
func InferBigFiveFromBehavior(sessions []SessionData) BigFiveProfile {
	if len(sessions) == 0 {
		return BigFiveProfile{
			Openness:          0.5,
			Conscientiousness: 0.5,
			Extraversion:      0.5,
			Agreeableness:     0.5,
			Neuroticism:       0.5,
			Confidence:        0.20,
			LastUpdated:       time.Now(),
		}
	}

	// Extraversion: % of time talking vs listening, energy vocal
	extraversion := calculateExtraversion(sessions)

	// Neuroticism: frequency of negative emotions, anxiety
	neuroticism := calculateNeuroticism(sessions)

	// Conscientiousness: punctuality, follow-through
	conscientiousness := calculateConscientiousness(sessions)

	// Agreeableness: cooperative tone, empathy
	agreeableness := calculateAgreeableness(sessions)

	// Openness: variety of topics, curiosity
	openness := calculateOpenness(sessions)

	// Confidence increases with more sessions
	confidence := minFloat(float64(len(sessions))/10.0, 0.90)

	return BigFiveProfile{
		Openness:          openness,
		Conscientiousness: conscientiousness,
		Extraversion:      extraversion,
		Agreeableness:     agreeableness,
		Neuroticism:       neuroticism,
		Confidence:        confidence,
		LastUpdated:       time.Now(),
	}
}

// BlendBigFive blends inference from Enneagram and behavior
func BlendBigFive(fromBehavior, fromEnnea BigFiveProfile, sessionCount int) BigFiveProfile {
	// Weight behavior more heavily as sessions increase
	behaviorWeight := minFloat(float64(sessionCount)/10.0, 0.80)
	enneaWeight := 1.0 - behaviorWeight

	return BigFiveProfile{
		Openness:          fromBehavior.Openness*behaviorWeight + fromEnnea.Openness*enneaWeight,
		Conscientiousness: fromBehavior.Conscientiousness*behaviorWeight + fromEnnea.Conscientiousness*enneaWeight,
		Extraversion:      fromBehavior.Extraversion*behaviorWeight + fromEnnea.Extraversion*enneaWeight,
		Agreeableness:     fromBehavior.Agreeableness*behaviorWeight + fromEnnea.Agreeableness*enneaWeight,
		Neuroticism:       fromBehavior.Neuroticism*behaviorWeight + fromEnnea.Neuroticism*enneaWeight,
		Confidence:        maxFloat(fromBehavior.Confidence, fromEnnea.Confidence),
		LastUpdated:       time.Now(),
	}
}

// Helper functions for behavioral inference

func calculateExtraversion(sessions []SessionData) float64 {
	totalEnergy := 0.0
	count := 0

	for _, session := range sessions {
		for _, energy := range session.EnergyLevels {
			totalEnergy += energy
			count++
		}
	}

	if count == 0 {
		return 0.5
	}

	avgEnergy := totalEnergy / float64(count)
	return minFloat(avgEnergy, 1.0)
}

func calculateNeuroticism(sessions []SessionData) float64 {
	negativeEmotions := 0
	totalEmotions := 0

	for _, session := range sessions {
		totalEmotions += session.EmotionCount
		// Count negative emotions (simplified)
		for _, behavior := range session.Behaviors {
			if containsKeyword(behavior, []string{"ansioso", "triste", "preocupado", "medo"}) {
				negativeEmotions++
			}
		}
	}

	if totalEmotions == 0 {
		return 0.5
	}

	return minFloat(float64(negativeEmotions)/float64(totalEmotions), 1.0)
}

func calculateConscientiousness(sessions []SessionData) float64 {
	// Simplified: assume punctuality and session regularity
	// In real implementation, would track actual punctuality
	return 0.6 // Default medium conscientiousness
}

func calculateAgreeableness(sessions []SessionData) float64 {
	// Simplified: look for cooperative language
	cooperativeBehaviors := 0
	totalBehaviors := 0

	for _, session := range sessions {
		totalBehaviors += len(session.Behaviors)
		for _, behavior := range session.Behaviors {
			if containsKeyword(behavior, []string{"gentil", "cooperativo", "empático", "amável"}) {
				cooperativeBehaviors++
			}
		}
	}

	if totalBehaviors == 0 {
		return 0.5
	}

	return minFloat(float64(cooperativeBehaviors)/float64(totalBehaviors)*2, 1.0)
}

func calculateOpenness(sessions []SessionData) float64 {
	// Simplified: variety of topics discussed
	uniqueTopics := make(map[string]bool)

	for _, session := range sessions {
		for _, behavior := range session.Behaviors {
			uniqueTopics[behavior] = true
		}
	}

	// More unique topics = higher openness
	return minFloat(float64(len(uniqueTopics))/20.0, 1.0)
}
