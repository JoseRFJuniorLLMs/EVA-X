// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package attention

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// WaveletAttention implementa atencao multi-escala inspirada em oscilacoes neurais
// Escalas: Focus(16D,5min) -> Context(64D,1h) -> Day(256D,1dia) -> Memory(1024D,1sem)
// Ciencia: Buschman & Kastner (2015) - "From behavior to neural dynamics: an integrated theory of attention"
type WaveletAttention struct {
	scales []AttentionScale
	mu     sync.RWMutex
}

// AttentionScale uma escala temporal de atencao
type AttentionScale struct {
	Name        string  // "focus", "context", "day", "memory"
	Dimension   int     // 16, 64, 256, 1024
	TimeConst   float64 // Constante temporal em minutos
	Weight      float64 // Peso relativo (0-1)
}

// AttentionWeight peso de atencao para uma memoria
type AttentionWeight struct {
	MemoryID    string             `json:"memory_id"`
	TotalScore  float64            `json:"total_score"`
	ScaleScores map[string]float64 `json:"scale_scores"` // score por escala
}

// AttentionResult resultado completo da atencao multi-escala
type AttentionResult struct {
	Query          string            `json:"query"`
	Weights        []AttentionWeight `json:"weights"`
	DominantScale  string            `json:"dominant_scale"`
	ProcessingTime string            `json:"processing_time"`
}

// MemoryCandidate candidato a ser atendido
type MemoryCandidate struct {
	ID        string
	Embedding []float64
	Age       time.Duration // Idade da memoria
	Content   string
}

// NewWaveletAttention cria motor de atencao multi-escala
func NewWaveletAttention() *WaveletAttention {
	return &WaveletAttention{
		scales: []AttentionScale{
			{Name: "focus", Dimension: 16, TimeConst: 5.0, Weight: 0.35},       // Ultimos 5 min
			{Name: "context", Dimension: 64, TimeConst: 60.0, Weight: 0.30},    // Ultima 1h
			{Name: "day", Dimension: 256, TimeConst: 1440.0, Weight: 0.20},     // Ultimo dia
			{Name: "memory", Dimension: 1024, TimeConst: 10080.0, Weight: 0.15}, // Ultima semana
		},
	}
}

// AttendMultiScale computa atencao em multiplas escalas temporais
// Retorna scores ponderados por escala e time-decay
func (w *WaveletAttention) AttendMultiScale(queryEmbedding []float64, candidates []MemoryCandidate) (*AttentionResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	start := time.Now()

	if len(queryEmbedding) == 0 || len(candidates) == 0 {
		return &AttentionResult{
			Weights:        nil,
			ProcessingTime: time.Since(start).String(),
		}, nil
	}

	weights := make([]AttentionWeight, len(candidates))

	for i, candidate := range candidates {
		weights[i] = AttentionWeight{
			MemoryID:    candidate.ID,
			ScaleScores: make(map[string]float64),
		}

		for _, scale := range w.scales {
			// 1. Similaridade coseno (usando dimensao truncada para simular escala)
			dim := scale.Dimension
			if dim > len(queryEmbedding) {
				dim = len(queryEmbedding)
			}
			if dim > len(candidate.Embedding) {
				dim = len(candidate.Embedding)
			}

			similarity := cosineSimilarityTruncated(queryEmbedding, candidate.Embedding, dim)

			// 2. Time decay exponencial baseado na constante temporal da escala
			ageMinutes := candidate.Age.Minutes()
			timeDecay := math.Exp(-ageMinutes / scale.TimeConst)

			// 3. Score final = similaridade * decay * peso_escala
			scaleScore := similarity * timeDecay * scale.Weight

			weights[i].ScaleScores[scale.Name] = scaleScore
			weights[i].TotalScore += scaleScore
		}
	}

	// Ordenar por score total (maior primeiro)
	sortWeights(weights)

	// Determinar escala dominante
	dominantScale := w.findDominantScale(weights)

	result := &AttentionResult{
		Weights:        weights,
		DominantScale:  dominantScale,
		ProcessingTime: time.Since(start).String(),
	}

	log.Printf("[WAVELET] Atencao multi-escala: %d candidatos, escala dominante=%s, tempo=%s",
		len(candidates), dominantScale, result.ProcessingTime)

	return result, nil
}

// GetImmediateContext retorna memorias do contexto imediato (ultimos 5 min)
func (w *WaveletAttention) GetImmediateContext(queryEmbedding []float64, candidates []MemoryCandidate, topK int) []AttentionWeight {
	return w.getByScale(queryEmbedding, candidates, "focus", topK)
}

// GetSessionContext retorna memorias do contexto da sessao (ultima hora)
func (w *WaveletAttention) GetSessionContext(queryEmbedding []float64, candidates []MemoryCandidate, topK int) []AttentionWeight {
	return w.getByScale(queryEmbedding, candidates, "context", topK)
}

// GetDayContext retorna memorias do dia
func (w *WaveletAttention) GetDayContext(queryEmbedding []float64, candidates []MemoryCandidate, topK int) []AttentionWeight {
	return w.getByScale(queryEmbedding, candidates, "day", topK)
}

// GetLongTermContext retorna memorias de longo prazo (ultima semana)
func (w *WaveletAttention) GetLongTermContext(queryEmbedding []float64, candidates []MemoryCandidate, topK int) []AttentionWeight {
	return w.getByScale(queryEmbedding, candidates, "memory", topK)
}

// getByScale retorna top-K memorias para uma escala especifica
func (w *WaveletAttention) getByScale(queryEmbedding []float64, candidates []MemoryCandidate, scaleName string, topK int) []AttentionWeight {
	var scale *AttentionScale
	for i := range w.scales {
		if w.scales[i].Name == scaleName {
			scale = &w.scales[i]
			break
		}
	}
	if scale == nil {
		return nil
	}

	weights := make([]AttentionWeight, len(candidates))
	for i, candidate := range candidates {
		dim := scale.Dimension
		if dim > len(queryEmbedding) {
			dim = len(queryEmbedding)
		}
		if dim > len(candidate.Embedding) {
			dim = len(candidate.Embedding)
		}

		similarity := cosineSimilarityTruncated(queryEmbedding, candidate.Embedding, dim)
		timeDecay := math.Exp(-candidate.Age.Minutes() / scale.TimeConst)

		weights[i] = AttentionWeight{
			MemoryID:    candidate.ID,
			TotalScore:  similarity * timeDecay,
			ScaleScores: map[string]float64{scaleName: similarity * timeDecay},
		}
	}

	sortWeights(weights)

	if topK > 0 && topK < len(weights) {
		weights = weights[:topK]
	}

	return weights
}

// findDominantScale determina qual escala contribuiu mais para os top resultados
func (w *WaveletAttention) findDominantScale(weights []AttentionWeight) string {
	if len(weights) == 0 {
		return "none"
	}

	topN := 5
	if topN > len(weights) {
		topN = len(weights)
	}

	scaleTotals := make(map[string]float64)
	for i := 0; i < topN; i++ {
		for scale, score := range weights[i].ScaleScores {
			scaleTotals[scale] += score
		}
	}

	dominant := ""
	maxTotal := 0.0
	for scale, total := range scaleTotals {
		if total > maxTotal {
			maxTotal = total
			dominant = scale
		}
	}

	return dominant
}

// GetStatistics retorna configuracao das escalas
func (w *WaveletAttention) GetStatistics() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	scales := make([]map[string]interface{}, len(w.scales))
	for i, s := range w.scales {
		scales[i] = map[string]interface{}{
			"name":       s.Name,
			"dimension":  s.Dimension,
			"time_const": fmt.Sprintf("%.0f min", s.TimeConst),
			"weight":     s.Weight,
		}
	}

	return map[string]interface{}{
		"engine": "wavelet_attention",
		"scales": scales,
		"status": "active",
	}
}

// cosineSimilarityTruncated calcula coseno similarity usando apenas as primeiras dim dimensoes
func cosineSimilarityTruncated(a, b []float64, dim int) float64 {
	if dim <= 0 {
		return 0.0
	}

	dot := 0.0
	normA := 0.0
	normB := 0.0

	for i := 0; i < dim; i++ {
		va := 0.0
		vb := 0.0
		if i < len(a) {
			va = a[i]
		}
		if i < len(b) {
			vb = b[i]
		}
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}

// sortWeights ordena pesos por TotalScore decrescente
func sortWeights(weights []AttentionWeight) {
	for i := 0; i < len(weights); i++ {
		for j := i + 1; j < len(weights); j++ {
			if weights[j].TotalScore > weights[i].TotalScore {
				weights[i], weights[j] = weights[j], weights[i]
			}
		}
	}
}
