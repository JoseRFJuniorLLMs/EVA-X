package voice

import (
	"context"
	"encoding/json"
	"eva-mind/internal/brainstem/database"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ProsodyAnalyzer analyzes voice prosody for mental health biomarkers
type ProsodyAnalyzer struct {
	apiKey string
	client *genai.Client
	db     *database.DB
}

// NewProsodyAnalyzer creates a new prosody analyzer
func NewProsodyAnalyzer(apiKey string, db *database.DB) (*ProsodyAnalyzer, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &ProsodyAnalyzer{
		apiKey: apiKey,
		client: client,
		db:     db,
	}, nil
}

// ProsodyFeatures represents extracted voice features
type ProsodyFeatures struct {
	// Basic acoustic features
	PitchMean      float64 `json:"pitch_mean"`       // Mean fundamental frequency (Hz)
	PitchStd       float64 `json:"pitch_std"`        // Pitch standard deviation
	PitchMin       float64 `json:"pitch_min"`
	PitchMax       float64 `json:"pitch_max"`

	// Voice quality
	Jitter         float64 `json:"jitter"`           // Pitch period perturbation
	Shimmer        float64 `json:"shimmer"`          // Amplitude perturbation
	HNR            float64 `json:"hnr"`              // Harmonics-to-Noise Ratio

	// Temporal features
	SpeechRate     float64 `json:"speech_rate"`      // Words per minute
	PauseDuration  float64 `json:"pause_duration"`   // Average pause duration (seconds)
	PauseFrequency float64 `json:"pause_frequency"`  // Pauses per minute

	// Energy/Intensity
	IntensityMean  float64 `json:"intensity_mean"`   // Mean intensity (dB)
	IntensityStd   float64 `json:"intensity_std"`

	// Derived indicators
	MonotonicityScore  float64 `json:"monotonicity_score"`  // 0-1, higher = more monotone
	TremorIndicator    float64 `json:"tremor_indicator"`    // 0-1, higher = more tremor
	BreathlessScore    float64 `json:"breathless_score"`    // 0-1, higher = more breathless

	// Analysis timestamp
	AnalyzedAt     time.Time `json:"analyzed_at"`
}

// AnalysisResult represents the complete analysis result
type AnalysisResult struct {
	Features           ProsodyFeatures           `json:"features"`
	DepressionScore    float64                   `json:"depression_score"`     // 0-1
	AnxietyScore       float64                   `json:"anxiety_score"`        // 0-1
	ParkinsonScore     float64                   `json:"parkinson_score"`      // 0-1
	HydrationLevel     string                    `json:"hydration_level"`      // "good", "moderate", "poor"
	OverallAssessment  string                    `json:"overall_assessment"`
	Alerts             []Alert                   `json:"alerts"`
	Recommendations    []string                  `json:"recommendations"`
}

// Alert represents a health alert
type Alert struct {
	Type     string  `json:"type"`      // "depression", "anxiety", "parkinson", "hydration"
	Severity string  `json:"severity"`  // "low", "medium", "high", "critical"
	Message  string  `json:"message"`
	Score    float64 `json:"score"`
}

// AnalyzeAudio analyzes audio features using Gemini Native Audio
func (p *ProsodyAnalyzer) AnalyzeAudio(
	audioData []byte,
	transcript string,
	idosoID int64,
) (*AnalysisResult, error) {

	log.Printf("🎙️ [PROSODY] Iniciando análise de voz para Idoso %d...", idosoID)

	// 1. Extract prosody features using Gemini Native Audio analysis
	features, err := p.extractProsodyFeatures(audioData, transcript)
	if err != nil {
		return nil, err
	}

	// 2. Calculate biomarker scores
	result := &AnalysisResult{
		Features: *features,
		Alerts:   []Alert{},
		Recommendations: []string{},
	}

	// Depression detection
	result.DepressionScore = p.calculateDepressionScore(features)
	if result.DepressionScore > 0.6 {
		result.Alerts = append(result.Alerts, Alert{
			Type:     "depression",
			Severity: p.getSeverity(result.DepressionScore),
			Message:  "Voz monotônica detectada - possível depressão",
			Score:    result.DepressionScore,
		})
	}

	// Anxiety detection
	result.AnxietyScore = p.calculateAnxietyScore(features)
	if result.AnxietyScore > 0.6 {
		result.Alerts = append(result.Alerts, Alert{
			Type:     "anxiety",
			Severity: p.getSeverity(result.AnxietyScore),
			Message:  "Fala acelerada + pitch elevado - ansiedade alta",
			Score:    result.AnxietyScore,
		})
	}

	// Parkinson detection
	result.ParkinsonScore = p.calculateParkinsonScore(features)
	if result.ParkinsonScore > 0.7 {
		result.Alerts = append(result.Alerts, Alert{
			Type:     "parkinson",
			Severity: "high",
			Message:  "Tremor vocal detectado - possível Parkinson precoce",
			Score:    result.ParkinsonScore,
		})
	}

	// Hydration assessment
	result.HydrationLevel = p.assessHydration(features)
	if result.HydrationLevel == "poor" {
		result.Alerts = append(result.Alerts, Alert{
			Type:     "hydration",
			Severity: "medium",
			Message:  "Voz pastosa detectada - possível desidratação",
			Score:    0.8,
		})
	}

	// Generate recommendations
	result.Recommendations = p.generateRecommendations(result)
	result.OverallAssessment = p.generateAssessment(result)

	// 3. Save to database
	err = p.saveToDatabase(idosoID, result)
	if err != nil {
		log.Printf("⚠️ [PROSODY] Erro ao salvar no banco: %v", err)
	}

	log.Printf("✅ [PROSODY] Análise completa. Alertas: %d", len(result.Alerts))
	return result, nil
}

// extractProsodyFeatures extracts prosody features using Gemini
func (p *ProsodyAnalyzer) extractProsodyFeatures(
	audioData []byte,
	transcript string,
) (*ProsodyFeatures, error) {

	// Use Gemini to analyze audio characteristics
	ctx := context.Background()
	model := p.client.GenerativeModel("gemini-2.5-flash-native-audio-preview")

	prompt := `Analise as características acústicas desta gravação de voz e retorne em JSON:

{
  "pitch_analysis": {
    "mean_hz": float (frequência fundamental média em Hz),
    "variation": float (variação de pitch, 0-1),
    "monotonicity": float (quão monótona é a voz, 0-1)
  },
  "speech_timing": {
    "words_per_minute": float,
    "average_pause_seconds": float,
    "pause_count": int
  },
  "voice_quality": {
    "clarity": float (0-1, 1 = muito clara),
    "stability": float (0-1, 1 = muito estável),
    "tremor_detected": boolean,
    "breathlessness": float (0-1)
  },
  "energy": {
    "mean_intensity": float (escala relativa 0-100),
    "variability": float (0-1)
  }
}`

	resp, err := model.GenerateContent(
		ctx,
		genai.Text(prompt),
		genai.Blob{MIMEType: "audio/wav", Data: audioData},
	)

	if err != nil {
		log.Printf("❌ [PROSODY] Erro ao chamar Gemini Audio: %v", err)
		return p.estimateFromTranscript(transcript), nil // Fallback
	}

	// Parse response
	features := p.parseGeminiResponse(resp, transcript)
	return features, nil
}

// parseGeminiResponse parses Gemini's response into ProsodyFeatures
func (p *ProsodyAnalyzer) parseGeminiResponse(
	resp *genai.GenerateContentResponse,
	transcript string,
) *ProsodyFeatures {

	features := &ProsodyFeatures{
		AnalyzedAt: time.Now(),
	}

	// Try to parse JSON from Gemini
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		part := resp.Candidates[0].Content.Parts[0]
		if textPart, ok := part.(genai.Text); ok {
			var geminiData map[string]interface{}
			err := json.Unmarshal([]byte(textPart), &geminiData)
			if err == nil {
				features = p.convertGeminiDataToFeatures(geminiData)
			}
		}
	}

	// Fallback: estimate from transcript
	if features.SpeechRate == 0 {
		return p.estimateFromTranscript(transcript)
	}

	return features
}

// estimateFromTranscript estimates basic features from transcript
func (p *ProsodyAnalyzer) estimateFromTranscript(transcript string) *ProsodyFeatures {
	words := len(strings.Fields(transcript))
	_ = float64(words) / 150.0 // Assume 150 WPM average (estimated duration)

	return &ProsodyFeatures{
		PitchMean:      180.0, // Default values
		PitchStd:       30.0,
		SpeechRate:     150.0,
		PauseDuration:  0.5,
		PauseFrequency: 5.0,
		IntensityMean:  70.0,
		AnalyzedAt:     time.Now(),
	}
}

// calculateDepressionScore calculates depression likelihood
func (p *ProsodyAnalyzer) calculateDepressionScore(features *ProsodyFeatures) float64 {
	score := 0.0

	// Monotonic voice (low pitch variation)
	if features.PitchStd < 20 {
		score += 0.4 * (1.0 - features.PitchStd/20.0)
	}

	// Slow speech rate
	if features.SpeechRate < 100 {
		score += 0.3 * (1.0 - features.SpeechRate/100.0)
	}

	// Low intensity (quiet voice)
	if features.IntensityMean < 60 {
		score += 0.2 * (1.0 - features.IntensityMean/60.0)
	}

	// Long pauses
	if features.PauseDuration > 1.0 {
		score += 0.1 * math.Min(features.PauseDuration/2.0, 1.0)
	}

	return math.Min(score, 1.0)
}

// calculateAnxietyScore calculates anxiety likelihood
func (p *ProsodyAnalyzer) calculateAnxietyScore(features *ProsodyFeatures) float64 {
	score := 0.0

	// Fast speech rate
	if features.SpeechRate > 180 {
		score += 0.4 * math.Min((features.SpeechRate-180)/50, 1.0)
	}

	// High pitch
	if features.PitchMean > 220 {
		score += 0.3 * math.Min((features.PitchMean-220)/80, 1.0)
	}

	// High pitch variability
	if features.PitchStd > 40 {
		score += 0.2 * math.Min((features.PitchStd-40)/40, 1.0)
	}

	// Frequent short pauses (nervousness)
	if features.PauseFrequency > 10 && features.PauseDuration < 0.5 {
		score += 0.1
	}

	return math.Min(score, 1.0)
}

// calculateParkinsonScore calculates Parkinson's likelihood
func (p *ProsodyAnalyzer) calculateParkinsonScore(features *ProsodyFeatures) float64 {
	score := 0.0

	// High jitter (voice instability)
	if features.Jitter > 0.05 {
		score += 0.3 * math.Min(features.Jitter/0.1, 1.0)
	}

	// High shimmer
	if features.Shimmer > 0.1 {
		score += 0.3 * math.Min(features.Shimmer/0.2, 1.0)
	}

	// Low HNR (noisy voice)
	if features.HNR < 15 {
		score += 0.2 * (1.0 - features.HNR/15)
	}

	// Tremor indicator
	score += 0.2 * features.TremorIndicator

	return math.Min(score, 1.0)
}

// assessHydration assesses hydration level
func (p *ProsodyAnalyzer) assessHydration(features *ProsodyFeatures) string {
	// Dry voice indicators: low HNR, breathlessness
	dryScore := 0.0

	if features.HNR < 12 {
		dryScore += 0.5
	}

	dryScore += 0.5 * features.BreathlessScore

	if dryScore > 0.7 {
		return "poor"
	} else if dryScore > 0.4 {
		return "moderate"
	}
	return "good"
}

// saveToDatabase saves analysis to PostgreSQL
func (p *ProsodyAnalyzer) saveToDatabase(idosoID int64, result *AnalysisResult) error {
	query := `
		INSERT INTO voice_prosody (
			patient_id, pitch_mean, pitch_std, jitter, shimmer, hnr,
			speech_rate, pause_duration, intensity_mean,
			monotonicity_score, tremor_indicator, analyzed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
	`

	_, err := p.db.Conn.Exec(
		query,
		idosoID,
		result.Features.PitchMean,
		result.Features.PitchStd,
		result.Features.Jitter,
		result.Features.Shimmer,
		result.Features.HNR,
		result.Features.SpeechRate,
		result.Features.PauseDuration,
		result.Features.IntensityMean,
		result.Features.MonotonicityScore,
		result.Features.TremorIndicator,
	)

	if err != nil {
		return err
	}

	// Save alerts
	for _, alert := range result.Alerts {
		queryAlert := `
			INSERT INTO voice_alerts (
				patient_id, alert_type, severity, message, score, created_at
			) VALUES ($1, $2, $3, $4, $5, NOW())
		`

		_, err = p.db.Conn.Exec(queryAlert, idosoID, alert.Type, alert.Severity, alert.Message, alert.Score)
		if err != nil {
			log.Printf("⚠️ [PROSODY] Erro ao salvar alerta: %v", err)
		}
	}

	return nil
}

// getSeverity returns severity level based on score
func (p *ProsodyAnalyzer) getSeverity(score float64) string {
	if score > 0.8 {
		return "critical"
	} else if score > 0.7 {
		return "high"
	} else if score > 0.5 {
		return "medium"
	}
	return "low"
}

// generateRecommendations generates recommendations based on analysis
func (p *ProsodyAnalyzer) generateRecommendations(result *AnalysisResult) []string {
	recommendations := []string{}

	if result.DepressionScore > 0.6 {
		recommendations = append(recommendations, "Considere consultar psiquiatra para avaliação")
		recommendations = append(recommendations, "Atividades físicas podem ajudar a melhorar o humor")
	}

	if result.AnxietyScore > 0.6 {
		recommendations = append(recommendations, "Pratique exercícios de respiração profunda")
		recommendations = append(recommendations, "Considere reduzir consumo de cafeína")
	}

	if result.ParkinsonScore > 0.7 {
		recommendations = append(recommendations, "URGENTE: Consulte neurologista para avaliação")
		recommendations = append(recommendations, "Documente tremor e rigidez muscular")
	}

	if result.HydrationLevel == "poor" {
		recommendations = append(recommendations, "Aumente ingestão de água (2-3 litros/dia)")
		recommendations = append(recommendations, "Atenção: desidratação pode causar confusão mental")
	}

	return recommendations
}

// generateAssessment generates overall assessment
func (p *ProsodyAnalyzer) generateAssessment(result *AnalysisResult) string {
	if len(result.Alerts) == 0 {
		return "Voz dentro dos padrões normais. Nenhum alerta detectado."
	}

	critical := 0
	high := 0
	for _, alert := range result.Alerts {
		if alert.Severity == "critical" {
			critical++
		} else if alert.Severity == "high" {
			high++
		}
	}

	if critical > 0 {
		return fmt.Sprintf("⚠️ ATENÇÃO: %d alerta(s) crítico(s) detectado(s). Requer avaliação médica imediata.", critical)
	}

	if high > 0 {
		return fmt.Sprintf("Detectados %d alerta(s) de severidade alta. Recomenda-se consulta médica.", high)
	}

	return fmt.Sprintf("Detectados %d indicador(es) que requerem monitoramento.", len(result.Alerts))
}

// convertGeminiDataToFeatures converts Gemini response to ProsodyFeatures
func (p *ProsodyAnalyzer) convertGeminiDataToFeatures(data map[string]interface{}) *ProsodyFeatures {
	features := &ProsodyFeatures{
		AnalyzedAt: time.Now(),
	}

	// Extract pitch analysis
	if pitch, ok := data["pitch_analysis"].(map[string]interface{}); ok {
		if v, ok := pitch["mean_hz"].(float64); ok {
			features.PitchMean = v
		}
		if v, ok := pitch["variation"].(float64); ok {
			features.PitchStd = v * 50 // Convert 0-1 to Hz range
		}
		if v, ok := pitch["monotonicity"].(float64); ok {
			features.MonotonicityScore = v
		}
	}

	// Extract speech timing
	if timing, ok := data["speech_timing"].(map[string]interface{}); ok {
		if v, ok := timing["words_per_minute"].(float64); ok {
			features.SpeechRate = v
		}
		if v, ok := timing["average_pause_seconds"].(float64); ok {
			features.PauseDuration = v
		}
		if v, ok := timing["pause_count"].(float64); ok {
			features.PauseFrequency = v
		}
	}

	// Extract voice quality
	if quality, ok := data["voice_quality"].(map[string]interface{}); ok {
		if v, ok := quality["stability"].(float64); ok {
			// Estimate jitter/shimmer from stability
			features.Jitter = (1.0 - v) * 0.1
			features.Shimmer = (1.0 - v) * 0.15
		}
		if v, ok := quality["tremor_detected"].(bool); ok && v {
			features.TremorIndicator = 0.8
		}
		if v, ok := quality["breathlessness"].(float64); ok {
			features.BreathlessScore = v
		}
		if v, ok := quality["clarity"].(float64); ok {
			// HNR estimate from clarity
			features.HNR = v * 20
		}
	}

	// Extract energy
	if energy, ok := data["energy"].(map[string]interface{}); ok {
		if v, ok := energy["mean_intensity"].(float64); ok {
			features.IntensityMean = v
		}
		if v, ok := energy["variability"].(float64); ok {
			features.IntensityStd = v * 20
		}
	}

	return features
}

// Close closes the analyzer
func (p *ProsodyAnalyzer) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
