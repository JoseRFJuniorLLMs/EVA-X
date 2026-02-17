// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package utils

import (
	"math"
)

// VoiceActivityDetector detecta atividade de voz em áudio PCM
type VoiceActivityDetector struct {
	Threshold    float64 // Threshold RMS para detectar fala (padrão: 500.0)
	MinDuration  int     // Mínimo de samples consecutivos para considerar fala (padrão: 3)
	activeCount  int     // Contador de frames consecutivos com atividade
	silenceCount int     // Contador de frames consecutivos de silêncio
}

// NewVAD cria um novo detector de atividade de voz
func NewVAD(threshold float64) *VoiceActivityDetector {
	if threshold == 0 {
		threshold = 500.0 // Valor padrão calibrado para conversação normal
	}
	return &VoiceActivityDetector{
		Threshold:   threshold,
		MinDuration: 3,
	}
}

// DetectActivity analisa um chunk de áudio PCM e retorna se há atividade vocal
func (vad *VoiceActivityDetector) DetectActivity(pcmData []byte) bool {
	if len(pcmData) < 2 {
		return false
	}

	// Converte bytes para int16 samples
	samples := bytesToInt16(pcmData)

	// Calcula RMS (Root Mean Square)
	rms := calculateRMS(samples)

	// Detecta atividade baseado no threshold
	if rms > vad.Threshold {
		vad.activeCount++
		vad.silenceCount = 0

		// Requer MinDuration frames consecutivos para confirmar atividade
		return vad.activeCount >= vad.MinDuration
	}

	vad.silenceCount++
	vad.activeCount = 0
	return false
}

// IsInSilence retorna true se está em período de silêncio prolongado
func (vad *VoiceActivityDetector) IsInSilence() bool {
	return vad.silenceCount > 10 // ~200ms de silêncio
}

// Reset reseta os contadores internos
func (vad *VoiceActivityDetector) Reset() {
	vad.activeCount = 0
	vad.silenceCount = 0
}

// calculateRMS calcula o Root Mean Square do sinal de áudio
func calculateRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}

	var sum float64
	for _, sample := range samples {
		val := float64(sample)
		sum += val * val
	}

	mean := sum / float64(len(samples))
	return math.Sqrt(mean)
}

// bytesToInt16 converte bytes PCM para int16 samples (little-endian)
func bytesToInt16(bytes []byte) []int16 {
	if len(bytes)%2 != 0 {
		bytes = bytes[:len(bytes)-1]
	}

	samples := make([]int16, len(bytes)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(bytes[i*2]) | int16(bytes[i*2+1])<<8
	}
	return samples
}
