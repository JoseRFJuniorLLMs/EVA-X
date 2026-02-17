// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package pattern

import (
	"testing"
	"time"
)

func TestDetectIncongruence(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		audioAnalysis AudioContext
		expectCue     bool
		minSeverity   float64
	}{
		{
			name: "Positive words with negative tone",
			text: "estou bem",
			audioAnalysis: AudioContext{
				ToneScore: -0.7,
				Energy:    0.5,
			},
			expectCue:   true,
			minSeverity: 0.80,
		},
		{
			name: "Positive words with low energy",
			text: "estou feliz",
			audioAnalysis: AudioContext{
				ToneScore: 0.2,
				Energy:    0.1,
			},
			expectCue:   true,
			minSeverity: 0.65,
		},
		{
			name: "Congruent positive",
			text: "estou bem",
			audioAnalysis: AudioContext{
				ToneScore: 0.7,
				Energy:    0.8,
			},
			expectCue:   false,
			minSeverity: 0.0,
		},
		{
			name: "Congruent negative",
			text: "estou triste",
			audioAnalysis: AudioContext{
				ToneScore: -0.7,
				Energy:    0.3,
			},
			expectCue:   false,
			minSeverity: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cue := DetectIncongruence(tt.text, tt.audioAnalysis)

			if tt.expectCue && cue == nil {
				t.Error("Expected cue to be detected, but got nil")
			}

			if !tt.expectCue && cue != nil {
				t.Errorf("Expected no cue, but got: %+v", cue)
			}

			if tt.expectCue && cue != nil {
				if cue.Severity < tt.minSeverity {
					t.Errorf("Expected severity >= %f, got %f", tt.minSeverity, cue.Severity)
				}

				if cue.Type != CueTypeIncongruence {
					t.Errorf("Expected type %s, got %s", CueTypeIncongruence, cue.Type)
				}
			}
		})
	}
}

func TestDetectSignificantPauses(t *testing.T) {
	now := time.Now()

	audioStream := []AudioChunk{
		{StartTime: now, EndTime: now.Add(2 * time.Second), Duration: 2 * time.Second, IsSilence: false},
		{StartTime: now.Add(2 * time.Second), EndTime: now.Add(6 * time.Second), Duration: 4 * time.Second, IsSilence: true}, // Significant pause
		{StartTime: now.Add(6 * time.Second), EndTime: now.Add(8 * time.Second), Duration: 2 * time.Second, IsSilence: false},
		{StartTime: now.Add(8 * time.Second), EndTime: now.Add(20 * time.Second), Duration: 12 * time.Second, IsSilence: true}, // Very long pause
	}

	cues := DetectSignificantPauses(audioStream, 3*time.Second)

	if len(cues) != 2 {
		t.Errorf("Expected 2 cues, got %d", len(cues))
	}

	// Check first pause (4 seconds)
	if cues[0].Severity < SeverityLow || cues[0].Severity > SeverityMedium {
		t.Errorf("Expected low-medium severity for 4s pause, got %f", cues[0].Severity)
	}

	// Check second pause (12 seconds)
	if cues[1].Severity < SeverityHigh {
		t.Errorf("Expected high severity for 12s pause, got %f", cues[1].Severity)
	}
}

func TestDetectToneShifts(t *testing.T) {
	now := time.Now()

	audioStream := []AudioChunk{
		{StartTime: now, Tone: 0.5, Pace: 120, IsSilence: false},
		{StartTime: now.Add(2 * time.Second), Tone: -0.6, Pace: 120, IsSilence: false}, // Big tone shift
		{StartTime: now.Add(4 * time.Second), Tone: -0.5, Pace: 60, IsSilence: false},  // Big pace shift
	}

	cues := DetectToneShifts(audioStream)

	if len(cues) < 2 {
		t.Errorf("Expected at least 2 cues (tone + pace shift), got %d", len(cues))
	}

	// Check for tone shift detection
	foundToneShift := false
	for _, cue := range cues {
		if cue.Type == CueTypeToneShift && cue.Metadata["tone_change"] != nil {
			foundToneShift = true
			if cue.Severity < 0.5 {
				t.Errorf("Expected high severity for large tone shift, got %f", cue.Severity)
			}
		}
	}

	if !foundToneShift {
		t.Error("Expected to detect tone shift")
	}
}

func TestCountRecurrentSignifiers(t *testing.T) {
	transcript := "me sinto sozinho, muito sozinho. a solidão é terrível. tenho medo da morte."

	counts := CountRecurrentSignifiers(transcript)

	if counts["sozinho"] != 2 {
		t.Errorf("Expected 'sozinho' to appear 2 times, got %d", counts["sozinho"])
	}

	if counts["solidão"] != 1 {
		t.Errorf("Expected 'solidão' to appear 1 time, got %d", counts["solidão"])
	}

	if counts["medo"] != 1 {
		t.Errorf("Expected 'medo' to appear 1 time, got %d", counts["medo"])
	}

	if counts["morte"] != 1 {
		t.Errorf("Expected 'morte' to appear 1 time, got %d", counts["morte"])
	}
}

func TestDetectRecurrence(t *testing.T) {
	transcript := "me sinto sozinho, muito sozinho, sempre sozinho. não aguento mais, não aguento."

	cues := DetectRecurrence(transcript, 2)

	if len(cues) != 2 {
		t.Errorf("Expected 2 recurrence cues, got %d", len(cues))
	}

	// Check for "sozinho" (appears 3 times)
	foundSozinho := false
	for _, cue := range cues {
		if cue.Metadata["signifier"] == "sozinho" {
			foundSozinho = true
			if cue.Metadata["count"] != 3 {
				t.Errorf("Expected 'sozinho' count of 3, got %d", cue.Metadata["count"])
			}
		}
	}

	if !foundSozinho {
		t.Error("Expected to detect 'sozinho' recurrence")
	}
}
