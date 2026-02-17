// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateFloat32Embedding creates a normalized random float32 embedding
func generateFloat32Embedding(dim int, seed int64) []float32 {
	r := rand.New(rand.NewSource(seed))
	emb := make([]float32, dim)
	var norm float64
	for i := range emb {
		emb[i] = float32(r.NormFloat64())
		norm += float64(emb[i]) * float64(emb[i])
	}
	norm = math.Sqrt(norm)
	for i := range emb {
		emb[i] /= float32(norm)
	}
	return emb
}

// generateSimilarFloat32Embedding creates an embedding similar to base
func generateSimilarFloat32Embedding(base []float32, noise float64, seed int64) []float32 {
	r := rand.New(rand.NewSource(seed))
	emb := make([]float32, len(base))
	var norm float64
	for i := range emb {
		emb[i] = base[i] + float32(r.NormFloat64()*noise)
		norm += float64(emb[i]) * float64(emb[i])
	}
	norm = math.Sqrt(norm)
	for i := range emb {
		emb[i] /= float32(norm)
	}
	return emb
}

func TestDetectShiftsInSession_AbruptChange(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	dim := 64

	// Create a session with an abrupt topic change in the middle
	baseHealth := generateFloat32Embedding(dim, 1)
	baseFamily := generateFloat32Embedding(dim, 999) // very different

	messages := []SessionMessage{
		{Content: "tomei meu remédio hoje", Speaker: "user", Embedding: generateSimilarFloat32Embedding(baseHealth, 0.05, 10)},
		{Content: "a pressão está melhor", Speaker: "user", Embedding: generateSimilarFloat32Embedding(baseHealth, 0.05, 11)},
		{Content: "meu marido faleceu ano passado", Speaker: "user", Embedding: generateSimilarFloat32Embedding(baseFamily, 0.05, 20)}, // abrupt shift
		{Content: "sinto muita falta dele", Speaker: "user", Embedding: generateSimilarFloat32Embedding(baseFamily, 0.05, 21)},
	}

	shifts := detector.DetectShiftsInSession("session-1", messages)

	// Should detect at least one abrupt change between msg 2 and 3
	var abruptShifts []ShiftEvent
	for _, s := range shifts {
		if s.ShiftType == ShiftAbruptChange {
			abruptShifts = append(abruptShifts, s)
		}
	}
	assert.GreaterOrEqual(t, len(abruptShifts), 1, "should detect abrupt topic change")
	if len(abruptShifts) > 0 {
		assert.Equal(t, 2, abruptShifts[0].MessageIndex, "abrupt change at message index 2")
	}
}

func TestDetectShiftsInSession_EmotionalFlip(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	dim := 32

	emb := generateFloat32Embedding(dim, 42)

	messages := []SessionMessage{
		{Content: "hoje foi um dia de muita alegria e felicidade", Speaker: "user", Embedding: emb},
		{Content: "mas depois veio a tristeza e o medo do abandono", Speaker: "user", Embedding: emb},
	}

	shifts := detector.DetectShiftsInSession("session-2", messages)

	var emotionalFlips []ShiftEvent
	for _, s := range shifts {
		if s.ShiftType == ShiftEmotionalFlip {
			emotionalFlips = append(emotionalFlips, s)
		}
	}
	assert.GreaterOrEqual(t, len(emotionalFlips), 1, "should detect emotional flip")
}

func TestDetectShiftsInSession_TopicReturn(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	detector.returnGapThreshold = 3 // lower for testing
	dim := 32

	emb := generateFloat32Embedding(dim, 42)

	messages := []SessionMessage{
		{Content: "estou preocupada com a saúde", Speaker: "user", Embedding: emb},   // idx 0: "saúde"
		{Content: "ontem fui ao mercado", Speaker: "user", Embedding: emb},            // idx 1
		{Content: "meu neto visitou", Speaker: "user", Embedding: emb},                // idx 2
		{Content: "o tempo está bonito", Speaker: "user", Embedding: emb},             // idx 3
		{Content: "voltando ao assunto da saúde", Speaker: "user", Embedding: emb},    // idx 4: return to "saúde"
	}

	shifts := detector.DetectShiftsInSession("session-3", messages)

	var returns []ShiftEvent
	for _, s := range shifts {
		if s.ShiftType == ShiftTopicReturn {
			returns = append(returns, s)
		}
	}
	assert.GreaterOrEqual(t, len(returns), 1, "should detect topic return")
}

func TestDetectShiftsInSession_Empty(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	assert.Nil(t, detector.DetectShiftsInSession("s", nil))
	assert.Nil(t, detector.DetectShiftsInSession("s", []SessionMessage{{}}))
}

func TestDetectShiftsInSession_AssistantMessagesSkipped(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	dim := 32

	base1 := generateFloat32Embedding(dim, 1)
	base2 := generateFloat32Embedding(dim, 999)

	messages := []SessionMessage{
		{Content: "tomei remédio", Speaker: "user", Embedding: base1},
		{Content: "que bom!", Speaker: "assistant", Embedding: base2}, // should be skipped
		{Content: "a pressão melhorou", Speaker: "user", Embedding: generateSimilarFloat32Embedding(base1, 0.05, 10)},
	}

	shifts := detector.DetectShiftsInSession("session-skip", messages)

	// No abrupt change because assistant message is skipped
	for _, s := range shifts {
		if s.ShiftType == ShiftAbruptChange {
			// Only user→user transitions should be analyzed
			assert.NotEqual(t, "assistant", messages[s.MessageIndex].Speaker)
		}
	}
}

func TestBuildAvoidanceProfile(t *testing.T) {
	detector := NewNarrativeShiftDetector(nil, nil)
	now := time.Now()

	shifts := []ShiftEvent{
		{ShiftType: ShiftAbruptChange, FromTopics: []string{"morte"}, CosineDelta: 0.8, Timestamp: now},
		{ShiftType: ShiftAbruptChange, FromTopics: []string{"morte"}, CosineDelta: 0.7, Timestamp: now.Add(-1 * time.Hour)},
		{ShiftType: ShiftAbruptChange, FromTopics: []string{"solidão"}, CosineDelta: 0.6, Timestamp: now},
		{ShiftType: ShiftTopicReturn, ToTopics: []string{"família"}, Timestamp: now},
		{ShiftType: ShiftTopicReturn, ToTopics: []string{"família"}, Timestamp: now.Add(-2 * time.Hour)},
		{ShiftType: ShiftEmotionalFlip, EmotionalShift: 1.5, Timestamp: now},
	}

	profile := detector.BuildAvoidanceProfile(context.Background(), 123, shifts, 7)

	require.NotNil(t, profile)
	assert.Equal(t, int64(123), profile.PatientID)
	assert.Equal(t, 1, profile.EmotionalFlipCount)

	// "morte" should be avoided (2 occurrences)
	assert.GreaterOrEqual(t, len(profile.AvoidedTopics), 1)
	assert.Equal(t, "morte", profile.AvoidedTopics[0].Topic)
	assert.Equal(t, 2, profile.AvoidedTopics[0].AvoidanceCount)

	// "família" should be circular (2 returns)
	assert.Contains(t, profile.CircularTopics, "família")
}

func TestComputeEmotionalValence(t *testing.T) {
	tests := []struct {
		text    string
		wantMin float64
		wantMax float64
	}{
		{"estou muito feliz e com alegria", 0.3, 1.0},
		{"sinto tristeza e medo", -1.0, -0.3},
		{"fui ao mercado", -0.1, 0.1}, // neutral
		{"", -0.1, 0.1},
	}

	for _, tt := range tests {
		v := computeEmotionalValence(tt.text)
		assert.GreaterOrEqual(t, v, tt.wantMin, "text=%q", tt.text)
		assert.LessOrEqual(t, v, tt.wantMax, "text=%q", tt.text)
	}
}

func TestExtractTopicKeywords(t *testing.T) {
	keywords := extractTopicKeywords("o médico disse que a pressão está alta")
	assert.Contains(t, keywords, "médico")
	assert.Contains(t, keywords, "pressão")
	assert.Contains(t, keywords, "alta")
	assert.NotContains(t, keywords, "o")
	assert.NotContains(t, keywords, "que")
}

func TestCosineSim32(t *testing.T) {
	// Identical
	v := []float32{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSim32(v, v), 0.001)

	// Orthogonal
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSim32(a, b), 0.001)

	// Empty
	assert.Equal(t, 0.0, cosineSim32(nil, nil))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
}
