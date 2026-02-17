// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package situation

import (
	"context"
	"testing"
	"time"
)

func TestSituationalModulator_Infer_Funeral(t *testing.T) {
	modulator := NewModulator(nil, nil)

	sit, err := modulator.Infer(context.Background(), "user123", "Faleceu minha mãe hoje", []Event{})

	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	if !contains(sit.Stressors, "luto") {
		t.Errorf("Expected 'luto' in stressors, got: %v", sit.Stressors)
	}

	if sit.EmotionScore >= 0 {
		t.Errorf("Expected negative emotion score, got: %.2f", sit.EmotionScore)
	}

	if sit.Intensity < 0.5 {
		t.Errorf("Expected high intensity (>0.5), got: %.2f", sit.Intensity)
	}
}

func TestSituationalModulator_Infer_Hospital(t *testing.T) {
	modulator := NewModulator(nil, nil)

	sit, err := modulator.Infer(context.Background(), "user456", "Estou internado no hospital", []Event{})

	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	if !contains(sit.Stressors, "hospital") {
		t.Errorf("Expected 'hospital' in stressors, got: %v", sit.Stressors)
	}
}

func TestSituationalModulator_Infer_Party(t *testing.T) {
	modulator := NewModulator(nil, nil)

	sit, err := modulator.Infer(context.Background(), "user789", "Hoje é meu aniversário, estou feliz", []Event{})

	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	if !contains(sit.Stressors, "aniversário") {
		t.Errorf("Expected 'aniversário' in stressors, got: %v", sit.Stressors)
	}

	if sit.EmotionScore <= 0 {
		t.Errorf("Expected positive emotion score, got: %.2f", sit.EmotionScore)
	}
}

func TestSituationalModulator_ModulateWeights_Funeral(t *testing.T) {
	modulator := NewModulator(nil, nil)

	baseWeights := map[string]float64{
		"ANSIEDADE":        0.5,
		"BUSCA_SEGURANÇA":  0.4,
		"EXTROVERSÃO":      0.6,
	}

	sit := Situation{
		Stressors: []string{"luto"},
		Intensity: 0.8,
	}

	modulated := modulator.ModulateWeights(baseWeights, sit)

	// ANSIEDADE deve aumentar (1.8x)
	expectedAnsiedade := 0.5 * 1.8
	if modulated["ANSIEDADE"] != expectedAnsiedade {
		t.Errorf("Expected ANSIEDADE=%.2f, got: %.2f", expectedAnsiedade, modulated["ANSIEDADE"])
	}

	// BUSCA_SEGURANÇA deve aumentar (2.0x)
	expectedSeguranca := 0.4 * 2.0
	if modulated["BUSCA_SEGURANÇA"] != expectedSeguranca {
		t.Errorf("Expected BUSCA_SEGURANÇA=%.2f, got: %.2f", expectedSeguranca, modulated["BUSCA_SEGURANÇA"])
	}

	// EXTROVERSÃO deve diminuir (0.5x)
	expectedExtroversao := 0.6 * 0.5
	if modulated["EXTROVERSÃO"] != expectedExtroversao {
		t.Errorf("Expected EXTROVERSÃO=%.2f, got: %.2f", expectedExtroversao, modulated["EXTROVERSÃO"])
	}
}

func TestSituationalModulator_ModulateWeights_Hospital(t *testing.T) {
	modulator := NewModulator(nil, nil)

	baseWeights := map[string]float64{
		"ALERTA":          0.5,
		"BUSCA_SEGURANÇA": 0.4,
	}

	sit := Situation{
		Stressors: []string{"hospital"},
		Intensity: 0.7,
	}

	modulated := modulator.ModulateWeights(baseWeights, sit)

	// ALERTA deve aumentar (2.0x)
	if modulated["ALERTA"] <= baseWeights["ALERTA"] {
		t.Errorf("Expected ALERTA to increase, got: %.2f", modulated["ALERTA"])
	}

	// BUSCA_SEGURANÇA deve aumentar (1.5x)
	if modulated["BUSCA_SEGURANÇA"] <= baseWeights["BUSCA_SEGURANÇA"] {
		t.Errorf("Expected BUSCA_SEGURANÇA to increase, got: %.2f", modulated["BUSCA_SEGURANÇA"])
	}
}

func TestSituationalModulator_ModulateWeights_MadrugadaSozinho(t *testing.T) {
	modulator := NewModulator(nil, nil)

	baseWeights := map[string]float64{
		"SOLIDÃO":   0.3,
		"ANSIEDADE": 0.5,
	}

	sit := Situation{
		SocialContext: "sozinho",
		TimeOfDay:     "madrugada",
		Intensity:     0.6,
	}

	modulated := modulator.ModulateWeights(baseWeights, sit)

	// SOLIDÃO deve aumentar (1.5x)
	expectedSolidao := 0.3 * 1.5
	if modulated["SOLIDÃO"] != expectedSolidao {
		t.Errorf("Expected SOLIDÃO=%.2f, got: %.2f", expectedSolidao, modulated["SOLIDÃO"])
	}

	// ANSIEDADE deve aumentar (1.3x)
	expectedAnsiedade := 0.5 * 1.3
	if modulated["ANSIEDADE"] != expectedAnsiedade {
		t.Errorf("Expected ANSIEDADE=%.2f, got: %.2f", expectedAnsiedade, modulated["ANSIEDADE"])
	}
}

func TestSituationalModulator_ModulateWeights_Party_IntrovertPerson(t *testing.T) {
	modulator := NewModulator(nil, nil)

	// Pessoa introvertida (EXTROVERSÃO baixa)
	baseWeights := map[string]float64{
		"EXTROVERSÃO": 0.3,
	}

	sit := Situation{
		Stressors: []string{"festa"},
		Intensity: 0.8,
	}

	modulated := modulator.ModulateWeights(baseWeights, sit)

	// EXTROVERSÃO deve aumentar (comportamento incomum)
	if modulated["EXTROVERSÃO"] <= baseWeights["EXTROVERSÃO"] {
		t.Errorf("Expected EXTROVERSÃO to increase for introvert at party, got: %.2f", modulated["EXTROVERSÃO"])
	}
}

func TestSituationalModulator_ModulateWeights_Crisis(t *testing.T) {
	modulator := NewModulator(nil, nil)

	baseWeights := map[string]float64{
		"ALERTA":    0.5,
		"ANSIEDADE": 0.5,
	}

	sit := Situation{
		Stressors: []string{"crise"},
		Intensity: 0.9,
	}

	modulated := modulator.ModulateWeights(baseWeights, sit)

	// ALERTA deve aumentar muito (2.5x)
	expectedAlerta := 0.5 * 2.5
	if modulated["ALERTA"] != expectedAlerta {
		t.Errorf("Expected ALERTA=%.2f, got: %.2f", expectedAlerta, modulated["ALERTA"])
	}

	// ANSIEDADE deve aumentar (2.0x)
	expectedAnsiedade := 0.5 * 2.0
	if modulated["ANSIEDADE"] != expectedAnsiedade {
		t.Errorf("Expected ANSIEDADE=%.2f, got: %.2f", expectedAnsiedade, modulated["ANSIEDADE"])
	}
}

func TestSituationalModulator_Performance(t *testing.T) {
	modulator := NewModulator(nil, nil)

	start := time.Now()
	_, err := modulator.Infer(context.Background(), "user123", "Estou bem, nada demais", []Event{})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	// Deve ser < 10ms (na verdade deve ser < 1ms sem cache miss)
	if duration.Milliseconds() > 10 {
		t.Errorf("Infer too slow: %v (expected <10ms)", duration)
	}
}

func TestGetTimeOfDay(t *testing.T) {
	tests := []struct {
		hour     int
		expected string
	}{
		{2, "madrugada"},
		{5, "madrugada"},
		{7, "manha"},
		{11, "manha"},
		{13, "tarde"},
		{17, "tarde"},
		{19, "noite"},
		{23, "noite"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			testTime := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, time.UTC)
			result := getTimeOfDay(testTime)
			if result != tt.expected {
				t.Errorf("For hour %d, expected %s, got %s", tt.hour, tt.expected, result)
			}
		})
	}
}

func TestInferEmotion(t *testing.T) {
	tests := []struct {
		text     string
		expected string // "positive", "negative", "neutral"
	}{
		{"Estou muito feliz hoje", "positive"},
		{"Que dia maravilhoso", "positive"},
		{"Estou triste e deprimido", "negative"},
		{"Tudo está péssimo", "negative"},
		{"Fui ao mercado", "neutral"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			score := inferEmotion(tt.text)

			switch tt.expected {
			case "positive":
				if score <= 0 {
					t.Errorf("Expected positive score for '%s', got: %.2f", tt.text, score)
				}
			case "negative":
				if score >= 0 {
					t.Errorf("Expected negative score for '%s', got: %.2f", tt.text, score)
				}
			case "neutral":
				if score < -0.2 || score > 0.2 {
					t.Errorf("Expected neutral score for '%s', got: %.2f", tt.text, score)
				}
			}
		})
	}
}

func TestCalculateIntensity(t *testing.T) {
	tests := []struct {
		stressors    []string
		emotionScore float64
		minExpected  float64
	}{
		{[]string{"luto"}, -0.8, 0.5},
		{[]string{"luto", "crise"}, -0.9, 0.8},
		{[]string{}, 0.1, 0.0},
		{[]string{"festa"}, 0.6, 0.3},
	}

	for _, tt := range tests {
		t.Run("intensity", func(t *testing.T) {
			intensity := calculateIntensity(tt.stressors, tt.emotionScore)
			if intensity < tt.minExpected {
				t.Errorf("Expected intensity >= %.2f, got: %.2f", tt.minExpected, intensity)
			}
			if intensity > 1.0 {
				t.Errorf("Intensity should be <= 1.0, got: %.2f", intensity)
			}
		})
	}
}