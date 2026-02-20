// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package speaker

import (
	"eva/internal/cortex/voice/dsp"
	"math"
)

// TimbreAnalyzer performs quick, local (non-API) voice analysis for real-time feedback.
type TimbreAnalyzer struct {
	mfcc *dsp.MFCCExtractor
}

// TimbreSnapshot represents a real-time voice analysis result.
type TimbreSnapshot struct {
	PitchHz     float64 `json:"pitch_hz"`
	Energy      float64 `json:"energy"`       // 0-1 normalized
	SpeechRate  float64 `json:"speech_rate"`   // estimated relative (0-1)
	Emotion     string  `json:"emotion"`       // "neutro", "estresse", "tristeza", "energia", "calma"
	StressLevel float64 `json:"stress_level"`  // 0-1
}

// NewTimbreAnalyzer creates a new analyzer.
func NewTimbreAnalyzer() *TimbreAnalyzer {
	cfg := dsp.DefaultMFCCConfig()
	return &TimbreAnalyzer{
		mfcc: dsp.NewMFCCExtractor(cfg),
	}
}

// AnalyzeQuick performs a fast analysis of PCM audio (16-bit LE, 16kHz mono).
// Designed to run on ~3s audio buffers without API calls.
func (t *TimbreAnalyzer) AnalyzeQuick(pcmData []byte) *TimbreSnapshot {
	samples := dsp.PCM16ToFloat64(pcmData)
	if len(samples) < 800 { // less than 50ms
		return &TimbreSnapshot{Emotion: "neutro"}
	}

	// Energy (RMS normalized to 0-1, with -60dB as floor)
	rms := dsp.RMS(samples)
	energyDB := 20 * math.Log10(rms+1e-10)
	energy := math.Max(0, math.Min(1, (energyDB+60)/60)) // -60dB=0, 0dB=1

	// Pitch (F0) via autocorrelation
	// Analyze the middle portion for stability
	start := len(samples) / 4
	end := start + len(samples)/2
	if end > len(samples) {
		end = len(samples)
	}
	pitchHz := dsp.EstimatePitchAutocorrelation(samples[start:end], 16000, 50, 500)

	// Speech rate estimate: count zero-crossings as proxy
	var zeroCrossings int
	for i := 1; i < len(samples); i++ {
		if (samples[i-1] >= 0 && samples[i] < 0) || (samples[i-1] < 0 && samples[i] >= 0) {
			zeroCrossings++
		}
	}
	durationSec := float64(len(samples)) / 16000.0
	zcRate := float64(zeroCrossings) / durationSec
	// Normalize: typical speech 500-3000 ZC/s
	speechRate := math.Max(0, math.Min(1, (zcRate-500)/2500))

	// Classify emotion using simple rule-based system
	emotion, stressLevel := classifyEmotion(pitchHz, energy, speechRate)

	return &TimbreSnapshot{
		PitchHz:     math.Round(pitchHz*100) / 100,
		Energy:      math.Round(energy*1000) / 1000,
		SpeechRate:  math.Round(speechRate*1000) / 1000,
		Emotion:     emotion,
		StressLevel: math.Round(stressLevel*1000) / 1000,
	}
}

// classifyEmotion uses pitch, energy, and speech rate to estimate emotional state.
// This is a simplified rule-based classifier for real-time use.
// For clinical-grade analysis, the ProsodyAnalyzer (Gemini-based) should be used.
func classifyEmotion(pitchHz, energy, speechRate float64) (string, float64) {
	// No pitch detected — likely silence or noise
	if pitchHz < 50 {
		return "neutro", 0
	}

	// Stress: high pitch + high energy + fast speech
	stressScore := 0.0
	if pitchHz > 220 {
		stressScore += 0.4 * math.Min((pitchHz-220)/80, 1.0)
	}
	if energy > 0.6 {
		stressScore += 0.3 * math.Min((energy-0.6)/0.4, 1.0)
	}
	if speechRate > 0.6 {
		stressScore += 0.3 * math.Min((speechRate-0.6)/0.4, 1.0)
	}

	if stressScore > 0.6 {
		return "estresse", stressScore
	}

	// Sadness: low pitch + low energy + slow speech
	if pitchHz < 140 && energy < 0.3 && speechRate < 0.3 {
		sadScore := 0.0
		sadScore += 0.4 * (1.0 - pitchHz/140)
		sadScore += 0.3 * (1.0 - energy/0.3)
		sadScore += 0.3 * (1.0 - speechRate/0.3)
		return "tristeza", sadScore
	}

	// High energy: high energy + moderate/high pitch
	if energy > 0.5 && pitchHz > 160 {
		return "energia", energy
	}

	// Calm: low energy + moderate pitch + slow speech
	if energy < 0.4 && speechRate < 0.4 {
		return "calma", 0.1
	}

	return "neutro", stressScore
}
