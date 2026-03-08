// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consolidation

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateEmbedding creates a normalized random embedding
func generateEmbedding(dim int, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	emb := make([]float64, dim)
	var norm float64
	for i := range emb {
		emb[i] = r.NormFloat64()
		norm += emb[i] * emb[i]
	}
	norm = math.Sqrt(norm)
	for i := range emb {
		emb[i] /= norm
	}
	return emb
}

// generateSimilarEmbedding creates an embedding similar to base (adds small noise)
func generateSimilarEmbedding(base []float64, noise float64, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	emb := make([]float64, len(base))
	var norm float64
	for i := range emb {
		emb[i] = base[i] + r.NormFloat64()*noise
		norm += emb[i] * emb[i]
	}
	norm = math.Sqrt(norm)
	for i := range emb {
		emb[i] /= norm
	}
	return emb
}

func TestScoNietzscheDBsonance_Basic(t *testing.T) {
	dim := 64
	base := generateEmbedding(dim, 42)

	memories := []EpisodicMemory{
		// Coherent cluster (similar embeddings, high activation)
		{ID: "m1", Embedding: generateSimilarEmbedding(base, 0.05, 1), ActivationScore: 0.9},
		{ID: "m2", Embedding: generateSimilarEmbedding(base, 0.05, 2), ActivationScore: 0.8},
		{ID: "m3", Embedding: generateSimilarEmbedding(base, 0.05, 3), ActivationScore: 0.7},
		// Outlier (very different embedding, high activation = dissonant)
		{ID: "m4", Embedding: generateEmbedding(dim, 999), ActivationScore: 0.95},
		// Low activation (even if outlier, low dissonance)
		{ID: "m5", Embedding: generateEmbedding(dim, 888), ActivationScore: 0.1},
	}

	scores := ScoNietzscheDBsonance(memories, 3)
	require.Len(t, scores, 5)

	// Should be sorted by dissonance DESC
	for i := 1; i < len(scores); i++ {
		assert.GreaterOrEqual(t, scores[i-1].Dissonance, scores[i].Dissonance,
			"scores should be sorted by dissonance DESC")
	}

	// The outlier with high activation (m4) should be most dissonant
	assert.Equal(t, "m4", scores[0].MemoryID, "high-activation outlier should be most dissonant")

	// m5 (low activation) should NOT be top even though it's an outlier
	m5Score := findScore(scores, "m5")
	require.NotNil(t, m5Score)
	assert.Less(t, m5Score.Dissonance, scores[0].Dissonance,
		"low-activation outlier should have less dissonance than high-activation outlier")
}

func TestScoNietzscheDBsonance_AllSimilar(t *testing.T) {
	dim := 64
	base := generateEmbedding(dim, 42)

	// All memories very similar + high activation → low dissonance
	memories := make([]EpisodicMemory, 10)
	for i := range memories {
		memories[i] = EpisodicMemory{
			ID:              fmt.Sprintf("m%d", i),
			Embedding:       generateSimilarEmbedding(base, 0.01, int64(i)),
			ActivationScore: 0.9,
		}
	}

	scores := ScoNietzscheDBsonance(memories, 5)
	require.Len(t, scores, 10)

	// All should have high coherence → low dissonance
	for _, s := range scores {
		assert.Greater(t, s.Coherence, 0.8, "similar memories should have high coherence")
		assert.Less(t, s.Dissonance, 0.2, "similar memories should have low dissonance")
	}
}

func TestScoNietzscheDBsonance_EmptyAndSmall(t *testing.T) {
	assert.Nil(t, ScoNietzscheDBsonance(nil, 5))
	assert.Nil(t, ScoNietzscheDBsonance([]EpisodicMemory{}, 5))

	// Single memory
	scores := ScoNietzscheDBsonance([]EpisodicMemory{
		{ID: "m1", Embedding: generateEmbedding(32, 1), ActivationScore: 1.0},
	}, 5)
	require.Len(t, scores, 1)
}

func TestScoNietzscheDBsonance_NoEmbeddings(t *testing.T) {
	memories := []EpisodicMemory{
		{ID: "m1", Embedding: nil, ActivationScore: 0.8},
		{ID: "m2", Embedding: []float64{}, ActivationScore: 0.5},
	}

	scores := ScoNietzscheDBsonance(memories, 3)
	require.Len(t, scores, 2)

	// Should get neutral coherence (0.5)
	for _, s := range scores {
		assert.InDelta(t, 0.5, s.Coherence, 0.01)
	}
}

func TestSelectTopDissonant(t *testing.T) {
	scores := []DissonanceScore{
		{MemoryID: "m1", Dissonance: 0.9},
		{MemoryID: "m2", Dissonance: 0.7},
		{MemoryID: "m3", Dissonance: 0.3},
		{MemoryID: "m4", Dissonance: 0.05}, // below threshold
	}

	// Budget=2, threshold=0.1
	selected := SelectTopDissonant(scores, 2, 0.1)
	require.Len(t, selected, 2)
	assert.Equal(t, "m1", selected[0].MemoryID)
	assert.Equal(t, "m2", selected[1].MemoryID)

	// Budget=10 but threshold=0.5 filters
	selected = SelectTopDissonant(scores, 10, 0.5)
	require.Len(t, selected, 2)

	// Budget=1
	selected = SelectTopDissonant(scores, 1, 0.0)
	require.Len(t, selected, 1)
}

func TestCosineSim(t *testing.T) {
	// Identical vectors
	v := []float64{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSim(v, v), 0.001)

	// Orthogonal vectors
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSim(a, b), 0.001)

	// Opposite vectors
	c := []float64{-1, 0, 0}
	assert.InDelta(t, -1.0, cosineSim(a, c), 0.001)

	// Empty/nil
	assert.Equal(t, 0.0, cosineSim(nil, nil))
	assert.Equal(t, 0.0, cosineSim([]float64{1}, []float64{1, 2}))
}

func BenchmarkDissonanceScoring(b *testing.B) {
	dim := 64
	memories := make([]EpisodicMemory, 1000)
	for i := range memories {
		memories[i] = EpisodicMemory{
			ID:              fmt.Sprintf("m%d", i),
			Embedding:       generateEmbedding(dim, int64(i)),
			ActivationScore: float64(i%10) / 10.0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScoNietzscheDBsonance(memories, 5)
	}
}

// Helper
func findScore(scores []DissonanceScore, id string) *DissonanceScore {
	for _, s := range scores {
		if s.MemoryID == id {
			return &s
		}
	}
	return nil
}

