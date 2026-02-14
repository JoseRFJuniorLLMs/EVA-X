package personality

import (
	"strings"
)

// TraitRelevance represents the relevance of a behavior to a personality trait
type TraitRelevance struct {
	Trait      string
	Relevance  float64 // 0-1: how relevant this behavior is to this trait
	Confidence float64 // 0-1: how confident we are in this mapping
}

// BehaviorTraitMapping defines the relationship between behaviors and traits
var BehaviorTraitMapping = map[string][]TraitRelevance{
	// Anxiety-related behaviors
	"rói unhas": {
		{Trait: "Ansiedade", Relevance: 0.85, Confidence: 0.90},
		{Trait: "Neuroticismo", Relevance: 0.70, Confidence: 0.75},
		{Trait: "Estresse", Relevance: 0.65, Confidence: 0.70},
	},
	"voz trêmula": {
		{Trait: "Ansiedade", Relevance: 0.90, Confidence: 0.85},
		{Trait: "Medo", Relevance: 0.80, Confidence: 0.80},
		{Trait: "Neuroticismo", Relevance: 0.60, Confidence: 0.70},
	},
	"pausas frequentes": {
		{Trait: "Ansiedade", Relevance: 0.70, Confidence: 0.65},
		{Trait: "Incerteza", Relevance: 0.75, Confidence: 0.70},
		{Trait: "Neuroticismo", Relevance: 0.50, Confidence: 0.60},
	},

	// Depression-related behaviors
	"fala lenta": {
		{Trait: "Depressão", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Cansaço", Relevance: 0.70, Confidence: 0.80},
		{Trait: "Neuroticismo", Relevance: 0.55, Confidence: 0.65},
	},
	"tom monótono": {
		{Trait: "Depressão", Relevance: 0.85, Confidence: 0.80},
		{Trait: "Apatia", Relevance: 0.75, Confidence: 0.75},
		{Trait: "Baixa_Energia", Relevance: 0.70, Confidence: 0.70},
	},
	"choro": {
		{Trait: "Tristeza", Relevance: 0.95, Confidence: 0.95},
		{Trait: "Depressão", Relevance: 0.75, Confidence: 0.80},
		{Trait: "Neuroticismo", Relevance: 0.60, Confidence: 0.70},
	},

	// Extroversion-related behaviors
	"fala rápida": {
		{Trait: "Extroversão", Relevance: 0.75, Confidence: 0.70},
		{Trait: "Entusiasmo", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Energia_Alta", Relevance: 0.70, Confidence: 0.70},
	},
	"risadas frequentes": {
		{Trait: "Extroversão", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Alegria", Relevance: 0.85, Confidence: 0.85},
		{Trait: "Sociabilidade", Relevance: 0.75, Confidence: 0.70},
	},

	// Conscientiousness-related behaviors
	"pontualidade": {
		{Trait: "Conscienciosidade", Relevance: 0.90, Confidence: 0.90},
		{Trait: "Organização", Relevance: 0.85, Confidence: 0.85},
		{Trait: "Responsabilidade", Relevance: 0.80, Confidence: 0.80},
	},
	"esquecimento": {
		{Trait: "Conscienciosidade", Relevance: 0.75, Confidence: 0.70}, // Low conscientiousness
		{Trait: "Declínio_Cognitivo", Relevance: 0.80, Confidence: 0.65},
		{Trait: "Estresse", Relevance: 0.60, Confidence: 0.60},
	},

	// Agreeableness-related behaviors
	"tom gentil": {
		{Trait: "Amabilidade", Relevance: 0.85, Confidence: 0.80},
		{Trait: "Empatia", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Cooperação", Relevance: 0.75, Confidence: 0.70},
	},
	"tom agressivo": {
		{Trait: "Amabilidade", Relevance: 0.80, Confidence: 0.75}, // Low agreeableness
		{Trait: "Raiva", Relevance: 0.90, Confidence: 0.85},
		{Trait: "Frustração", Relevance: 0.75, Confidence: 0.75},
	},

	// Openness-related behaviors
	"curiosidade": {
		{Trait: "Abertura", Relevance: 0.90, Confidence: 0.85},
		{Trait: "Criatividade", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Intelectualidade", Relevance: 0.75, Confidence: 0.70},
	},
	"resistência a mudanças": {
		{Trait: "Abertura", Relevance: 0.85, Confidence: 0.80}, // Low openness
		{Trait: "Conservadorismo", Relevance: 0.80, Confidence: 0.75},
		{Trait: "Rigidez", Relevance: 0.70, Confidence: 0.70},
	},
}

// MapBehaviorToTrait maps an observed behavior to relevant personality traits
func MapBehaviorToTrait(behavior string) []TraitRelevance {
	behavior = strings.ToLower(strings.TrimSpace(behavior))

	// Direct match
	if traits, exists := BehaviorTraitMapping[behavior]; exists {
		return traits
	}

	// Fuzzy match (contains)
	for key, traits := range BehaviorTraitMapping {
		if strings.Contains(behavior, key) || strings.Contains(key, behavior) {
			// Reduce confidence for fuzzy matches
			fuzzied := make([]TraitRelevance, len(traits))
			for i, t := range traits {
				fuzzied[i] = TraitRelevance{
					Trait:      t.Trait,
					Relevance:  t.Relevance,
					Confidence: t.Confidence * 0.8, // Reduce confidence by 20%
				}
			}
			return fuzzied
		}
	}

	// No match found
	return []TraitRelevance{}
}

// FilterIrrelevantBehaviors filters out behaviors that don't map to any traits
func FilterIrrelevantBehaviors(behaviors []string) []string {
	relevant := []string{}
	for _, behavior := range behaviors {
		traits := MapBehaviorToTrait(behavior)
		if len(traits) > 0 {
			relevant = append(relevant, behavior)
		}
	}
	return relevant
}

// GetRelevanceScore calculates the overall relevance of a behavior to personality assessment
func GetRelevanceScore(behavior string) float64 {
	traits := MapBehaviorToTrait(behavior)
	if len(traits) == 0 {
		return 0.0
	}

	// Average relevance weighted by confidence
	totalWeightedRelevance := 0.0
	totalConfidence := 0.0

	for _, t := range traits {
		totalWeightedRelevance += t.Relevance * t.Confidence
		totalConfidence += t.Confidence
	}

	if totalConfidence == 0 {
		return 0.0
	}

	return totalWeightedRelevance / totalConfidence
}

// IsRelevantForTrait checks if a behavior is relevant for a specific trait
func IsRelevantForTrait(behavior, trait string, minRelevance float64) bool {
	traits := MapBehaviorToTrait(behavior)
	for _, t := range traits {
		if strings.EqualFold(t.Trait, trait) && t.Relevance >= minRelevance {
			return true
		}
	}
	return false
}
