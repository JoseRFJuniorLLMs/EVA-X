// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

import (
	"testing"
)

func TestMapBehaviorToTrait(t *testing.T) {
	tests := []struct {
		name           string
		behavior       string
		expectedTraits int
		expectedFirst  string
		minRelevance   float64
	}{
		{
			name:           "Anxiety behavior - nail biting",
			behavior:       "rói unhas",
			expectedTraits: 3,
			expectedFirst:  "Ansiedade",
			minRelevance:   0.65,
		},
		{
			name:           "Depression behavior - slow speech",
			behavior:       "fala lenta",
			expectedTraits: 3,
			expectedFirst:  "Depressão",
			minRelevance:   0.55,
		},
		{
			name:           "Extroversion behavior - fast speech",
			behavior:       "fala rápida",
			expectedTraits: 3,
			expectedFirst:  "Extroversão",
			minRelevance:   0.70,
		},
		{
			name:           "Unknown behavior",
			behavior:       "comportamento desconhecido",
			expectedTraits: 0,
			expectedFirst:  "",
			minRelevance:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traits := MapBehaviorToTrait(tt.behavior)

			if len(traits) != tt.expectedTraits {
				t.Errorf("Expected %d traits, got %d", tt.expectedTraits, len(traits))
			}

			if tt.expectedTraits > 0 {
				if traits[0].Trait != tt.expectedFirst {
					t.Errorf("Expected first trait %s, got %s", tt.expectedFirst, traits[0].Trait)
				}

				if traits[0].Relevance < tt.minRelevance {
					t.Errorf("Expected relevance >= %f, got %f", tt.minRelevance, traits[0].Relevance)
				}
			}
		})
	}
}

func TestFilterIrrelevantBehaviors(t *testing.T) {
	behaviors := []string{
		"rói unhas",
		"comportamento irrelevante",
		"fala lenta",
		"outro comportamento desconhecido",
		"choro",
	}

	filtered := FilterIrrelevantBehaviors(behaviors)

	expected := 3 // "rói unhas", "fala lenta", "choro"
	if len(filtered) != expected {
		t.Errorf("Expected %d relevant behaviors, got %d", expected, len(filtered))
	}
}

func TestGetRelevanceScore(t *testing.T) {
	tests := []struct {
		name     string
		behavior string
		minScore float64
		maxScore float64
	}{
		{
			name:     "High relevance behavior",
			behavior: "choro",
			minScore: 0.70,
			maxScore: 1.0,
		},
		{
			name:     "Medium relevance behavior",
			behavior: "pausas frequentes",
			minScore: 0.40,
			maxScore: 0.80,
		},
		{
			name:     "Unknown behavior",
			behavior: "comportamento desconhecido",
			minScore: 0.0,
			maxScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := GetRelevanceScore(tt.behavior)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Expected score between %f and %f, got %f", tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestIsRelevantForTrait(t *testing.T) {
	tests := []struct {
		name         string
		behavior     string
		trait        string
		minRelevance float64
		expected     bool
	}{
		{
			name:         "Relevant behavior for trait",
			behavior:     "rói unhas",
			trait:        "Ansiedade",
			minRelevance: 0.80,
			expected:     true,
		},
		{
			name:         "Behavior not relevant enough",
			behavior:     "rói unhas",
			trait:        "Ansiedade",
			minRelevance: 0.95,
			expected:     false,
		},
		{
			name:         "Behavior not related to trait",
			behavior:     "fala rápida",
			trait:        "Ansiedade",
			minRelevance: 0.50,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRelevantForTrait(tt.behavior, tt.trait, tt.minRelevance)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
