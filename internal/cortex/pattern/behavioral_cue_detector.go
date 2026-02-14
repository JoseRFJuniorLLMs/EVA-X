package pattern

import (
	"strings"
	"time"
)

// BehavioralCueType represents the type of behavioral cue detected
type BehavioralCueType string

const (
	CueTypeIncongruence BehavioralCueType = "incongruence"
	CueTypePause        BehavioralCueType = "pause"
	CueTypeToneShift    BehavioralCueType = "tone_shift"
	CueTypeRecurrence   BehavioralCueType = "recurrence"
)

// Severity levels for behavioral cues
const (
	SeverityLow      = 0.3
	SeverityMedium   = 0.6
	SeverityHigh     = 0.8
	SeverityCritical = 0.95
)

// BehavioralCue represents a detected behavioral pattern that may indicate psychological state
type BehavioralCue struct {
	Type        BehavioralCueType
	Severity    float64 // 0-1
	Description string
	Timestamp   time.Time
	Metadata    map[string]interface{} // Additional context
}

// AudioContext represents audio analysis data
type AudioContext struct {
	Emotion   string
	Intensity float64 // 1-10
	Urgency   string  // LOW, MEDIUM, HIGH, CRITICAL
	ToneScore float64 // -1 (very negative) to +1 (very positive)
	Energy    float64 // 0-1
	Pace      float64 // words per minute
}

// AudioChunk represents a segment of audio with metadata
type AudioChunk struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Tone      float64 // -1 to +1
	Energy    float64 // 0-1
	Pace      float64 // words per minute
	IsSilence bool
}

// DetectIncongruence detects mismatches between verbal content and audio tone
func DetectIncongruence(text string, audioAnalysis AudioContext) *BehavioralCue {
	text = strings.ToLower(strings.TrimSpace(text))

	// Positive words with negative tone = RED FLAG
	positiveWords := []string{"bem", "feliz", "ótimo", "bom", "alegre", "contente", "tranquilo"}
	negativeWords := []string{"mal", "triste", "péssimo", "ruim", "deprimido", "ansioso", "preocupado"}

	hasPositiveWord := false
	hasNegativeWord := false

	for _, word := range positiveWords {
		if strings.Contains(text, word) {
			hasPositiveWord = true
			break
		}
	}

	for _, word := range negativeWords {
		if strings.Contains(text, word) {
			hasNegativeWord = true
			break
		}
	}

	// Detect incongruence
	var severity float64
	var description string

	if hasPositiveWord && audioAnalysis.ToneScore < -0.3 {
		// Saying positive things with negative tone
		severity = 0.85
		description = "Incongruência detectada: palavras positivas com tom negativo/triste"
	} else if hasNegativeWord && audioAnalysis.ToneScore > 0.3 {
		// Saying negative things with positive tone (less common, but possible)
		severity = 0.60
		description = "Incongruência detectada: palavras negativas com tom positivo (possível minimização)"
	} else if hasPositiveWord && audioAnalysis.Energy < 0.3 {
		// Saying positive things with low energy
		severity = 0.70
		description = "Incongruência detectada: palavras positivas com energia muito baixa"
	} else {
		// No incongruence detected
		return nil
	}

	return &BehavioralCue{
		Type:        CueTypeIncongruence,
		Severity:    severity,
		Description: description,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"text":       text,
			"tone_score": audioAnalysis.ToneScore,
			"energy":     audioAnalysis.Energy,
			"emotion":    audioAnalysis.Emotion,
		},
	}
}

// DetectSignificantPauses detects pauses longer than a threshold
func DetectSignificantPauses(audioStream []AudioChunk, pauseThreshold time.Duration) []BehavioralCue {
	cues := []BehavioralCue{}

	for i, chunk := range audioStream {
		if !chunk.IsSilence {
			continue
		}

		if chunk.Duration < pauseThreshold {
			continue
		}

		// Determine severity based on pause length
		severity := 0.0
		if chunk.Duration < 5*time.Second {
			severity = SeverityLow
		} else if chunk.Duration < 10*time.Second {
			severity = SeverityMedium
		} else if chunk.Duration < 20*time.Second {
			severity = SeverityHigh
		} else {
			severity = SeverityCritical
		}

		// Try to infer context from surrounding chunks
		context := "pausa significativa"
		if i > 0 {
			prevChunk := audioStream[i-1]
			if prevChunk.Tone < -0.5 {
				context = "pausa após fala emocionalmente negativa (possível choro ou reflexão profunda)"
				severity += 0.1
			}
		}

		cues = append(cues, BehavioralCue{
			Type:        CueTypePause,
			Severity:    min(severity, 1.0),
			Description: context,
			Timestamp:   chunk.StartTime,
			Metadata: map[string]interface{}{
				"duration_seconds": chunk.Duration.Seconds(),
			},
		})
	}

	return cues
}

// DetectToneShifts detects sudden changes in tone or pace
func DetectToneShifts(audioStream []AudioChunk) []BehavioralCue {
	if len(audioStream) < 2 {
		return []BehavioralCue{}
	}

	cues := []BehavioralCue{}

	for i := 1; i < len(audioStream); i++ {
		if audioStream[i].IsSilence || audioStream[i-1].IsSilence {
			continue
		}

		prev := audioStream[i-1]
		curr := audioStream[i]

		// Detect tone shift
		toneChange := curr.Tone - prev.Tone
		if abs(toneChange) > 0.5 {
			severity := min(abs(toneChange), 1.0)
			description := ""

			if toneChange < 0 {
				description = "Mudança súbita para tom mais negativo/triste"
			} else {
				description = "Mudança súbita para tom mais positivo (possível máscara emocional)"
			}

			cues = append(cues, BehavioralCue{
				Type:        CueTypeToneShift,
				Severity:    severity,
				Description: description,
				Timestamp:   curr.StartTime,
				Metadata: map[string]interface{}{
					"tone_change":   toneChange,
					"previous_tone": prev.Tone,
					"current_tone":  curr.Tone,
				},
			})
		}

		// Detect pace shift
		paceChange := (curr.Pace - prev.Pace) / prev.Pace
		if abs(paceChange) > 0.4 { // 40% change
			severity := min(abs(paceChange), 1.0)
			description := ""

			if paceChange < 0 {
				description = "Velocidade de fala caiu significativamente (possível cansaço ou emoção)"
			} else {
				description = "Velocidade de fala aumentou significativamente (possível ansiedade ou agitação)"
			}

			cues = append(cues, BehavioralCue{
				Type:        CueTypeToneShift,
				Severity:    severity * 0.8, // Slightly lower severity than tone shifts
				Description: description,
				Timestamp:   curr.StartTime,
				Metadata: map[string]interface{}{
					"pace_change":   paceChange,
					"previous_pace": prev.Pace,
					"current_pace":  curr.Pace,
				},
			})
		}
	}

	return cues
}

// CountRecurrentSignifiers counts how often specific words/phrases appear
func CountRecurrentSignifiers(transcript string) map[string]int {
	transcript = strings.ToLower(transcript)

	// Significant words/phrases to track
	signifiers := []string{
		"sozinho", "solidão", "abandono", "medo", "morte", "morrer",
		"dor", "sofrimento", "tristeza", "depressão", "ansiedade",
		"mãe", "pai", "filho", "filha", "esposo", "esposa",
		"saudade", "falta", "perda", "luto",
		"cansado", "cansaço", "exausto",
		"não aguento", "não consigo", "impossível",
	}

	counts := make(map[string]int)

	for _, signifier := range signifiers {
		count := strings.Count(transcript, signifier)
		if count > 0 {
			counts[signifier] = count
		}
	}

	return counts
}

// DetectRecurrence creates behavioral cues for recurrent signifiers
func DetectRecurrence(transcript string, minOccurrences int) []BehavioralCue {
	counts := CountRecurrentSignifiers(transcript)
	cues := []BehavioralCue{}

	for signifier, count := range counts {
		if count < minOccurrences {
			continue
		}

		// Severity increases with frequency
		severity := min(float64(count)/10.0, 1.0)

		// Critical signifiers get higher severity
		criticalSignifiers := []string{"morte", "morrer", "não aguento", "impossível", "abandono"}
		for _, critical := range criticalSignifiers {
			if signifier == critical {
				severity = min(severity*1.5, 1.0)
				break
			}
		}

		cues = append(cues, BehavioralCue{
			Type:        CueTypeRecurrence,
			Severity:    severity,
			Description: "Palavra recorrente: \"" + signifier + "\" (" + string(rune(count)) + "x)",
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"signifier": signifier,
				"count":     count,
			},
		})
	}

	return cues
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
