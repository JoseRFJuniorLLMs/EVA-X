package personality

// TraitVisibility maps personality traits to their observability (0-1)
// Higher values = easier to observe quickly
var TraitVisibility = map[string]float64{
	// EASY (observable quickly - 1-2 sessions)
	"Extroversão":   0.95,
	"Ansiedade":     0.90,
	"Entusiasmo":    0.85,
	"Tristeza":      0.80,
	"Raiva":         0.80,
	"Alegria":       0.85,
	"Sociabilidade": 0.90,

	// MEDIUM (observable with time - 3-5 sessions)
	"Conscienciosidade": 0.60,
	"Amabilidade":       0.55,
	"Organização":       0.60,
	"Cooperação":        0.55,
	"Empatia":           0.50,

	// HARD (require extended observation - 6+ sessions)
	"Neuroticismo":      0.40,
	"Abertura":          0.35,
	"Valores_morais":    0.30,
	"Crenças_profundas": 0.20,
	"Integridade":       0.25,
}

// GetTraitVisibility returns the visibility score for a trait
func GetTraitVisibility(trait string) float64 {
	if visibility, exists := TraitVisibility[trait]; exists {
		return visibility
	}
	return 0.50 // Default to medium visibility
}

// AdjustConfidenceByTrait adjusts confidence based on trait visibility and session count
func AdjustConfidenceByTrait(trait string, sessionCount int) float64 {
	visibility := GetTraitVisibility(trait)

	// Calculate required sessions for reliable judgment
	requiredSessions := int((1.0-visibility)*10) + 1 // 1-10 sessions

	if sessionCount >= requiredSessions {
		return 1.0 // Full confidence
	}

	// Partial confidence based on progress
	return float64(sessionCount) / float64(requiredSessions)
}

// ShouldAdmitUncertainty determines if EVA should admit uncertainty
func ShouldAdmitUncertainty(trait string, sessionCount int) bool {
	confidence := AdjustConfidenceByTrait(trait, sessionCount)
	return confidence < 0.70 // Admit uncertainty if confidence < 70%
}

// GetUncertaintyMessage generates a message admitting uncertainty
func GetUncertaintyMessage(trait string, sessionCount int) string {
	visibility := GetTraitVisibility(trait)
	confidence := AdjustConfidenceByTrait(trait, sessionCount)

	if visibility > 0.80 {
		return "Ainda não tenho dados suficientes para julgar isso com certeza."
	} else if visibility > 0.50 {
		return "Este traço requer mais tempo de observação. Minha avaliação ainda é preliminar."
	} else {
		return "Este é um traço profundo que requer observação prolongada. Ainda não posso fazer um julgamento confiável."
	}
}
