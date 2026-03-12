// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package perception

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

// PerceptionEngine analyzes camera frames using Gemini Vision to detect objects
// and spatial relationships, producing structured 2D scene descriptions.
type PerceptionEngine struct {
	client *genai.Client
	apiKey string

	// Rate limiting: avoid hammering Gemini with every frame
	lastAnalysis time.Time
	minInterval  time.Duration
	mu           sync.Mutex
}

// DetectedObject represents a single object found in a camera frame.
type DetectedObject struct {
	Label      string  `json:"label"`       // e.g. "coffee_mug", "book", "medicine_bottle"
	Confidence float64 `json:"confidence"`  // 0.0-1.0
	X          float64 `json:"x"`           // normalized position (0.0-1.0 from left)
	Y          float64 `json:"y"`           // normalized position (0.0-1.0 from top)
	Width      float64 `json:"width"`       // normalized bounding box width
	Height     float64 `json:"height"`      // normalized bounding box height
	Category   string  `json:"category"`    // semantic category: "furniture", "person", "medication", "food", "device"
}

// SceneAnalysis represents the full 2D semantic analysis of a camera frame.
type SceneAnalysis struct {
	Objects       []DetectedObject `json:"objects"`
	SceneType     string           `json:"scene_type"`     // "bedroom", "kitchen", "living_room", "bathroom", "outdoor", "unknown"
	Lighting      string           `json:"lighting"`       // "bright", "dim", "dark", "natural", "artificial"
	Activity      string           `json:"activity"`       // "resting", "eating", "walking", "reading", "watching_tv", "unknown"
	PeopleCount   int              `json:"people_count"`   // number of people detected
	RiskFactors   []string         `json:"risk_factors"`   // e.g. "fall_hazard", "medication_on_floor", "poor_lighting"
	Timestamp     int64            `json:"timestamp"`
	FrameHash     string           `json:"frame_hash"`     // deduplicate identical frames
}

// NewPerceptionEngine creates a new engine with the given Gemini API key.
// minInterval controls how frequently frames are analyzed (default 2s).
func NewPerceptionEngine(apiKey string, minInterval time.Duration) (*PerceptionEngine, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("perception engine: failed to create Gemini client: %w", err)
	}

	if minInterval <= 0 {
		minInterval = 2 * time.Second
	}

	return &PerceptionEngine{
		client:      client,
		apiKey:      apiKey,
		minInterval: minInterval,
	}, nil
}

// ShouldAnalyze returns true if enough time has passed since the last analysis.
// Thread-safe.
func (pe *PerceptionEngine) ShouldAnalyze() bool {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	return time.Since(pe.lastAnalysis) >= pe.minInterval
}

// AnalyzeFrame takes a JPEG frame (raw bytes) and returns a structured scene analysis.
// Returns nil if rate-limited (call ShouldAnalyze first to check).
func (pe *PerceptionEngine) AnalyzeFrame(ctx context.Context, jpegData []byte) (*SceneAnalysis, error) {
	pe.mu.Lock()
	if time.Since(pe.lastAnalysis) < pe.minInterval {
		pe.mu.Unlock()
		return nil, nil // rate limited
	}
	pe.lastAnalysis = time.Now()
	pe.mu.Unlock()

	if len(jpegData) == 0 {
		return nil, fmt.Errorf("perception engine: empty frame")
	}

	model := pe.client.GenerativeModel("gemini-2.0-flash-exp")
	model.SetTemperature(0.1)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx,
		genai.Text(perceptionPrompt),
		genai.ImageData("image/jpeg", jpegData),
	)
	if err != nil {
		log.Warn().Err(err).Msg("[PERCEPTION] Gemini Vision analysis failed")
		return nil, fmt.Errorf("perception engine: gemini error: %w", err)
	}

	scene, err := parseSceneResponse(resp)
	if err != nil {
		return nil, err
	}

	scene.Timestamp = time.Now().Unix()
	scene.FrameHash = frameHash(jpegData)

	log.Debug().
		Int("objects", len(scene.Objects)).
		Str("scene", scene.SceneType).
		Str("activity", scene.Activity).
		Int("people", scene.PeopleCount).
		Msg("[PERCEPTION] Frame analyzed")

	return scene, nil
}

// AnalyzeFrameBase64 is a convenience wrapper that accepts base64-encoded JPEG.
func (pe *PerceptionEngine) AnalyzeFrameBase64(ctx context.Context, b64Data string) (*SceneAnalysis, error) {
	jpegData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, fmt.Errorf("perception engine: invalid base64: %w", err)
	}
	return pe.AnalyzeFrame(ctx, jpegData)
}

// Close releases the Gemini client.
func (pe *PerceptionEngine) Close() error {
	if pe.client != nil {
		return pe.client.Close()
	}
	return nil
}

// perceptionPrompt is the system prompt for Gemini Vision scene analysis.
const perceptionPrompt = `You are a visual perception system for an elderly care AI assistant.
Analyze this camera frame and identify ALL objects, people, and spatial context.

Return a JSON object with this exact structure:
{
  "objects": [
    {
      "label": "object_name_in_english_snake_case",
      "confidence": 0.95,
      "x": 0.5,
      "y": 0.3,
      "width": 0.2,
      "height": 0.15,
      "category": "furniture"
    }
  ],
  "scene_type": "living_room",
  "lighting": "bright",
  "activity": "resting",
  "people_count": 1,
  "risk_factors": ["poor_lighting", "fall_hazard"]
}

Rules:
- x, y, width, height are normalized 0.0-1.0 (x=0 left, y=0 top)
- categories: "person", "furniture", "medication", "food", "drink", "device", "clothing", "pet", "hazard", "other"
- scene_type: "bedroom", "kitchen", "living_room", "bathroom", "office", "outdoor", "unknown"
- lighting: "bright", "dim", "dark", "natural", "artificial"
- activity: "resting", "eating", "walking", "reading", "cooking", "watching_tv", "exercising", "sleeping", "talking", "unknown"
- risk_factors: "fall_hazard", "medication_on_floor", "poor_lighting", "sharp_object", "wet_floor", "obstacle", "no_risk"
- Be concise. Only list objects with confidence >= 0.5
- Maximum 20 objects per frame
- Focus on safety-relevant objects for elderly care`

// parseSceneResponse extracts a SceneAnalysis from the Gemini response.
func parseSceneResponse(resp *genai.GenerateContentResponse) (*SceneAnalysis, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("perception engine: empty Gemini response")
	}

	part := resp.Candidates[0].Content.Parts[0]
	textPart, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("perception engine: unexpected response type")
	}

	jsonStr := string(textPart)
	jsonStr = strings.TrimPrefix(jsonStr, "```json\n")
	jsonStr = strings.TrimSuffix(jsonStr, "\n```")
	jsonStr = strings.TrimSpace(jsonStr)

	var scene SceneAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &scene); err != nil {
		log.Warn().Str("json", jsonStr).Err(err).Msg("[PERCEPTION] Failed to parse scene JSON")
		return nil, fmt.Errorf("perception engine: JSON parse error: %w", err)
	}

	return &scene, nil
}

// frameHash produces a simple hash to deduplicate identical frames.
// Uses first+last 64 bytes + length as a fast fingerprint (not cryptographic).
func frameHash(data []byte) string {
	if len(data) < 128 {
		return fmt.Sprintf("%x_%d", data[:min(16, len(data))], len(data))
	}
	return fmt.Sprintf("%x%x_%d", data[:32], data[len(data)-32:], len(data))
}
