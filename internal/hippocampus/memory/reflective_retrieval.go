// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Reflective Memory Management (RMM)
// Reference: Wang et al. (arXiv:2503.08026, 2025) - Reflective Memory Management for LLM agents
// Adds post-retrieval reflection: re-ranks results by contextual relevance,
// temporal coherence, emotional alignment, and contradiction detection.
package memory

import (
	"math"
	"sort"
	"strings"
	"time"
)

// ReflectionScore holds per-dimension reflection scores
type ReflectionScore struct {
	ContextualRelevance float64 `json:"contextual_relevance"` // keyword overlap (0-1)
	TemporalCoherence   float64 `json:"temporal_coherence"`   // event date matches query time (0-1)
	EmotionalAlignment  float64 `json:"emotional_alignment"`  // emotional tone match (0-1)
	ContradictionFlag   float64 `json:"contradiction_flag"`   // contradiction with other results (0-1, higher=worse)
}

// ReflectedResult wraps a SearchResult with reflection analysis
type ReflectedResult struct {
	*SearchResult
	Reflection ReflectionScore `json:"reflection"`
	FinalScore float64         `json:"final_score"`
}

// ReflectionWeights controls the scoring formula
type ReflectionWeights struct {
	Similarity          float64
	ContextualRelevance float64
	TemporalCoherence   float64
	EmotionalAlignment  float64
	Contradiction       float64
}

// DefaultReflectionWeights returns the default scoring weights
func DefaultReflectionWeights() ReflectionWeights {
	return ReflectionWeights{
		Similarity:          0.40,
		ContextualRelevance: 0.25,
		TemporalCoherence:   0.15,
		EmotionalAlignment:  0.10,
		Contradiction:       0.10,
	}
}

// temporalMarkers maps Portuguese temporal words to approximate day offsets
var temporalMarkers = map[string]int{
	"hoje":       0,
	"ontem":      -1,
	"anteontem":  -2,
	"amanhã":     1,
	"semana":     -7,
	"mês":        -30,
	"ano":        -365,
	"manhã":      0,
	"tarde":      0,
	"noite":      0,
	"passada":    -7,
	"passado":    -7,
	"última":     -7,
	"último":     -7,
	"recente":    -3,
	"agora":      0,
}

// emotionFamilies groups emotions into positive/negative families for alignment
var emotionFamilies = map[string]string{
	// Positive
	"alegria":     "positive",
	"felicidade":  "positive",
	"amor":        "positive",
	"esperança":   "positive",
	"orgulho":     "positive",
	"gratidão":    "positive",
	// Negative
	"tristeza":    "negative",
	"medo":        "negative",
	"ansiedade":   "negative",
	"raiva":       "negative",
	"solidão":     "negative",
	"culpa":       "negative",
	"desespero":   "negative",
	"angústia":    "negative",
	"frustração":  "negative",
	"preocupação": "negative",
	// Neutral
	"saudade":     "ambivalent",
	"nostalgia":   "ambivalent",
}

// portugueseStopwords for keyword extraction
var portugueseStopwords = map[string]bool{
	"o": true, "a": true, "os": true, "as": true,
	"um": true, "uma": true, "uns": true, "umas": true,
	"de": true, "do": true, "da": true, "dos": true, "das": true,
	"em": true, "no": true, "na": true, "nos": true, "nas": true,
	"por": true, "para": true, "com": true, "sem": true,
	"que": true, "e": true, "é": true, "ou": true, "se": true,
	"eu": true, "ele": true, "ela": true, "nós": true,
	"me": true, "te": true, "meu": true, "minha": true,
	"seu": true, "sua": true, "como": true, "mais": true,
	"muito": true, "já": true, "não": true, "sim": true,
	"está": true, "estou": true, "foi": true, "ser": true,
	"ter": true, "quando": true, "qual": true, "quais": true,
}

// ReflectAndRerank applies multi-dimensional reflection to re-rank retrieval results.
// No LLM calls — pure algorithmic post-processing.
func ReflectAndRerank(query string, results []*SearchResult, k int) []*ReflectedResult {
	if len(results) == 0 {
		return nil
	}

	weights := DefaultReflectionWeights()
	queryKeywords := extractKeywords(query)
	queryTemporalDays := extractTemporalOffset(query)
	queryEmotionFamily := detectEmotionFamily(query)

	reflected := make([]*ReflectedResult, len(results))
	for i, sr := range results {
		ref := &ReflectedResult{SearchResult: sr}

		// 1. Contextual Relevance (keyword overlap)
		ref.Reflection.ContextualRelevance = computeKeywordOverlap(queryKeywords, sr.Memory.Content)

		// 2. Temporal Coherence
		ref.Reflection.TemporalCoherence = computeTemporalCoherence(queryTemporalDays, sr.Memory.EventDate)

		// 3. Emotional Alignment
		ref.Reflection.EmotionalAlignment = computeEmotionalAlignment(queryEmotionFamily, sr.Memory.Emotion)

		reflected[i] = ref
	}

	// 4. Contradiction Detection (pairwise)
	detectContradictions(reflected)

	// 5. Compute final score
	for _, r := range reflected {
		r.FinalScore = weights.Similarity*r.Similarity +
			weights.ContextualRelevance*r.Reflection.ContextualRelevance +
			weights.TemporalCoherence*r.Reflection.TemporalCoherence +
			weights.EmotionalAlignment*r.Reflection.EmotionalAlignment +
			weights.Contradiction*(1.0-r.Reflection.ContradictionFlag)
	}

	// 6. Sort by FinalScore DESC
	sort.Slice(reflected, func(i, j int) bool {
		return reflected[i].FinalScore > reflected[j].FinalScore
	})

	// Return top K
	if k > 0 && k < len(reflected) {
		reflected = reflected[:k]
	}

	return reflected
}

// extractKeywords returns content words from text (removes stopwords and punctuation)
func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var keywords []string
	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:\"'()[]")
		if len(cleaned) > 2 && !portugueseStopwords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}
	return keywords
}

// computeKeywordOverlap returns fraction of query keywords found in content
func computeKeywordOverlap(queryKeywords []string, content string) float64 {
	if len(queryKeywords) == 0 {
		return 0.5 // neutral if no keywords
	}

	contentLower := strings.ToLower(content)
	hits := 0
	for _, kw := range queryKeywords {
		if strings.Contains(contentLower, kw) {
			hits++
		}
	}

	return float64(hits) / float64(len(queryKeywords))
}

// extractTemporalOffset detects temporal markers and returns approximate day offset (nil if none)
func extractTemporalOffset(text string) *int {
	words := strings.Fields(strings.ToLower(text))
	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:")
		if offset, ok := temporalMarkers[cleaned]; ok {
			return &offset
		}
	}
	return nil
}

// computeTemporalCoherence scores how well result's event date matches the query's temporal context
func computeTemporalCoherence(queryDayOffset *int, eventDate time.Time) float64 {
	if queryDayOffset == nil {
		return 0.5 // neutral if query has no temporal marker
	}
	if eventDate.IsZero() {
		return 0.3 // penalty if memory has no event date
	}

	expectedDate := time.Now().AddDate(0, 0, *queryDayOffset)
	daysDiff := math.Abs(expectedDate.Sub(eventDate).Hours() / 24)

	if daysDiff < 1 {
		return 1.0 // exact match
	}
	if daysDiff < 3 {
		return 0.7 // close
	}
	if daysDiff < 7 {
		return 0.4 // within a week
	}
	return 0.1 // far
}

// detectEmotionFamily returns the emotion family of the query (positive/negative/ambivalent/unknown)
func detectEmotionFamily(text string) string {
	words := strings.Fields(strings.ToLower(text))
	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:")
		if family, ok := emotionFamilies[cleaned]; ok {
			return family
		}
	}

	// Heuristic: "triste", "medo", "feliz" are common question topics
	if strings.Contains(strings.ToLower(text), "triste") || strings.Contains(strings.ToLower(text), "medo") {
		return "negative"
	}
	if strings.Contains(strings.ToLower(text), "feliz") || strings.Contains(strings.ToLower(text), "alegr") {
		return "positive"
	}

	return "unknown"
}

// computeEmotionalAlignment scores alignment between query emotion and result emotion
func computeEmotionalAlignment(queryFamily, resultEmotion string) float64 {
	if queryFamily == "unknown" || resultEmotion == "" {
		return 0.5 // neutral
	}

	resultFamily, ok := emotionFamilies[strings.ToLower(resultEmotion)]
	if !ok {
		return 0.5
	}

	if queryFamily == resultFamily {
		return 1.0 // same family
	}
	if queryFamily == "ambivalent" || resultFamily == "ambivalent" {
		return 0.6 // ambivalent aligns with either
	}
	// Opposite families (positive vs negative)
	return 0.2
}

// detectContradictions finds pairs of results that may contradict each other.
// High content similarity + opposing emotions = potential contradiction.
func detectContradictions(results []*ReflectedResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			// Check if contents are very similar
			overlapI := computeKeywordOverlap(
				extractKeywords(results[i].Memory.Content),
				results[j].Memory.Content,
			)
			overlapJ := computeKeywordOverlap(
				extractKeywords(results[j].Memory.Content),
				results[i].Memory.Content,
			)
			contentSim := (overlapI + overlapJ) / 2

			if contentSim < 0.5 {
				continue // not similar enough to contradict
			}

			// Check emotional opposition
			famI := emotionFamilies[strings.ToLower(results[i].Memory.Emotion)]
			famJ := emotionFamilies[strings.ToLower(results[j].Memory.Emotion)]

			if (famI == "positive" && famJ == "negative") || (famI == "negative" && famJ == "positive") {
				contradiction := contentSim * 0.8
				results[i].Reflection.ContradictionFlag = math.Max(results[i].Reflection.ContradictionFlag, contradiction)
				results[j].Reflection.ContradictionFlag = math.Max(results[j].Reflection.ContradictionFlag, contradiction)
			}
		}
	}
}
