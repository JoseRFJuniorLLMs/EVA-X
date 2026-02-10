package configurator

import (
	"encoding/json"
	"fmt"
	"os"
)

// EvaBehaviorConfig defines complete behavior configuration
type EvaBehaviorConfig struct {
	TTS         TTSConfig         `json:"tts"`
	UI          UIConfig          `json:"ui"`
	Content     ContentConfig     `json:"content"`
	Interaction InteractionConfig `json:"interaction"`
}

// TTSConfig defines text-to-speech settings
type TTSConfig struct {
	Rate  float64 `json:"rate"`  // 0.5 to 2.0
	Pitch float64 `json:"pitch"` // -3.0 to 3.0
	Tone  string  `json:"tone"`  // "animated", "monotone", "energetic", "empathetic", "clear"
}

// UIConfig defines user interface settings
type UIConfig struct {
	FontFamily       string  `json:"fontFamily"`
	FontSize         int     `json:"fontSize"`
	LetterSpacing    float64 `json:"letterSpacing,omitempty"`
	BackgroundColor  string  `json:"backgroundColor"`
	PrimaryColor     string  `json:"primaryColor"`
	EnableAnimations bool    `json:"enableAnimations,omitempty"`
}

// ContentConfig defines content filtering and selection
type ContentConfig struct {
	AllowMetaphors       bool     `json:"allowMetaphors"`
	MaxComplexity        int      `json:"maxComplexity"` // 1-10
	PreferredCollections []string `json:"preferredCollections"`
}

// InteractionConfig defines interaction preferences
type InteractionConfig struct {
	PrimaryInput       string `json:"primaryInput"` // "voice", "text"
	EnableGamification bool   `json:"enableGamification,omitempty"`
	EnableKaraoke      bool   `json:"enableKaraoke,omitempty"`
	ShowVisualTimer    bool   `json:"showVisualTimer,omitempty"`
}

// NeuroConfigurator manages behavior configurations
type NeuroConfigurator struct {
	profiles map[string]EvaBehaviorConfig
}

// NewNeuroConfigurator creates a new configurator from JSON file
func NewNeuroConfigurator(configPath string) (*NeuroConfigurator, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config struct {
		Profiles map[string]EvaBehaviorConfig `json:"profiles"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &NeuroConfigurator{
		profiles: config.Profiles,
	}, nil
}

// GetConfig returns configuration for a user
func (nc *NeuroConfigurator) GetConfig(ageGroup, neuroType string) *EvaBehaviorConfig {
	// Generate profile key
	profileKey := fmt.Sprintf("%s_%s", ageGroup, neuroType)

	// Lookup configuration
	if config, exists := nc.profiles[profileKey]; exists {
		return &config
	}

	// Fallback to standard for age group
	fallbackKey := fmt.Sprintf("%s_standard", ageGroup)
	if config, exists := nc.profiles[fallbackKey]; exists {
		return &config
	}

	// Ultimate fallback
	if config, exists := nc.profiles["adults_standard"]; exists {
		return &config
	}

	// Should never reach here if config is valid
	return nil
}

// ListProfiles returns all available profile keys
func (nc *NeuroConfigurator) ListProfiles() []string {
	keys := make([]string, 0, len(nc.profiles))
	for key := range nc.profiles {
		keys = append(keys, key)
	}
	return keys
}

// ValidateConfig checks if configuration is valid
func (nc *NeuroConfigurator) ValidateConfig() error {
	requiredProfiles := []string{
		"kids_standard", "kids_autism", "kids_adhd", "kids_dyslexia",
		"teens_standard", "teens_autism", "teens_adhd", "teens_dyslexia",
		"adults_standard", "adults_autism", "adults_adhd", "adults_dyslexia",
	}

	for _, profile := range requiredProfiles {
		if _, exists := nc.profiles[profile]; !exists {
			return fmt.Errorf("missing required profile: %s", profile)
		}
	}

	return nil
}
