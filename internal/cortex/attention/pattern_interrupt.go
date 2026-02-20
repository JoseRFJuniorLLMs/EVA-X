// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package attention

import (
	"eva/internal/cortex/attention/models"
	"math"
	"time"
)

// PatternInterrupt - Detecta loops e interrompe com choques conscientes
type PatternInterrupt struct {
	similarityThreshold float64
}

func NewPatternInterrupt(threshold float64) *PatternInterrupt {
	return &PatternInterrupt{
		similarityThreshold: threshold,
	}
}

// DetectLoop - Detecta padrões repetitivos
func (pi *PatternInterrupt) DetectLoop(
	input string,
	state *models.ExecutiveState,
) bool {

	if len(state.PatternBuffer) < 3 {
		return false
	}

	// Gera embedding do input atual
	currentVector := generateEmbedding(input) // placeholder

	// Compara com buffer de padrões
	// Pegamos os mais recentes
	start := len(state.PatternBuffer) - 5
	if start < 0 {
		start = 0
	}
	recentPatterns := state.PatternBuffer[start:]

	similarCount := 0
	for _, pattern := range recentPatterns {
		similarity := pi.cosineSimilarity(currentVector, pattern.Vector)
		if similarity > pi.similarityThreshold {
			similarCount++
		}
	}

	// Se 3+ padrões similares → loop detectado
	return similarCount >= 3
}

// GenerateInterruption - Gera pergunta para interromper padrão
func (pi *PatternInterrupt) GenerateInterruption(
	state *models.ExecutiveState,
) string {

	// Exemplos de interrupções Gurdjieffianas
	interruptions := []string{
		"Você percebe que já mencionou isso algumas vezes?",
		"O que mudaria se você parasse de pensar nisso por um momento?",
		"Quantas vezes você já teve esse mesmo pensamento hoje?",
		"Existe algo diferente que você ainda não considerou?",
	}

	// Seleciona baseado no contexto
	// Por ora, simples rotação baseada no tempo ou turnos
	idx := int(time.Now().UnixNano()) % len(interruptions)
	if state.TurnNumber > 0 {
		idx = state.TurnNumber % len(interruptions)
	}
	return interruptions[idx]
}

// cosineSimilarity - Similaridade cosseno entre vetores
func (pi *PatternInterrupt) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func generateEmbedding(text string) []float64 {
	// Deterministic hash for testing/mocking
	// Real implementation would call OpenAI/Gemini/Vertex
	h := 0
	for _, c := range text {
		h = 31*h + int(c)
	}

	// Normalize to avoid overflow issues relative to float
	seed := float64(h)

	vec := make([]float64, 384)
	for i := range vec {
		// Create a pattern based on seed and index
		vec[i] = math.Sin(seed + float64(i))
	}
	return vec
}
