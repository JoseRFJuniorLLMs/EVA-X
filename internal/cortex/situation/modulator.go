// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package situation - Situational Modulator
// Detecta contexto situacional e modula pesos de personality em tempo real
// Baseado em mente.md - Performance-first design (<10ms latency)
package situation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"eva/internal/brainstem/infrastructure/cache"
)

// Situation representa o contexto situacional atual do usuário
type Situation struct {
	Stressors     []string  `json:"stressors"`       // "luto", "hospital", "aniversario", "crise"
	SocialContext string    `json:"social_context"`  // "sozinho", "familia", "publico"
	TimeOfDay     string    `json:"time_of_day"`     // "madrugada", "manha", "tarde", "noite"
	EmotionScore  float64   `json:"emotion_score"`   // -1.0 (negativo) to 1.0 (positivo)
	Intensity     float64   `json:"intensity"`       // 0.0-1.0 (força do contexto)
	DetectedAt    time.Time `json:"detected_at"`     // Timestamp de detecção
}

// Event representa um evento recente do usuário
type Event struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SituationalModulator detecta e modula contexto situacional
type SituationalModulator struct {
	cache          *cache.RedisClient
	cacheTTL       time.Duration
	stressorKeywords map[string][]string
}

// Config para o modulator
type Config struct {
	CacheTTL         time.Duration
	StressorKeywords map[string][]string
}

// NewModulator cria um novo Situational Modulator
func NewModulator(cache *cache.RedisClient, config *Config) *SituationalModulator {
	if config == nil {
		config = &Config{
			CacheTTL: 5 * time.Minute,
			StressorKeywords: getDefaultStressorKeywords(),
		}
	}

	return &SituationalModulator{
		cache:            cache,
		cacheTTL:         config.CacheTTL,
		stressorKeywords: config.StressorKeywords,
	}
}

// Infer detecta situação atual (<10ms)
func (m *SituationalModulator) Infer(ctx context.Context, userID string, recentText string, recentEvents []Event) (Situation, error) {
	// 1. Cache check (hit = <1ms)
	cacheKey := fmt.Sprintf("situation:%s", userID)
	if m.cache != nil {
		if cached, err := m.cache.Get(ctx, cacheKey); err == nil {
			var sit Situation
			if json.Unmarshal([]byte(cached), &sit) == nil {
				return sit, nil
			}
		}
	}

	// 2. Rules determinísticas rápidas (80% casos, ~5ms)
	sit := Situation{
		TimeOfDay:  getTimeOfDay(time.Now()),
		DetectedAt: time.Now(),
	}

	// Extrair stressors via keywords
	sit.Stressors = m.extractStressors(recentText)

	// Inferir contexto social
	sit.SocialContext = inferSocial(recentEvents)

	// Inferir emoção via sentiment analysis básico
	sit.EmotionScore = inferEmotion(recentText)

	// Calcular intensidade
	sit.Intensity = calculateIntensity(sit.Stressors, sit.EmotionScore)

	// 3. Cache result (5min TTL)
	if m.cache != nil {
		data, _ := json.Marshal(sit)
		m.cache.Set(ctx, cacheKey, string(data), m.cacheTTL)
	}

	return sit, nil
}

// ModulateWeights ajusta pesos de personality por contexto (<1ms)
func (m *SituationalModulator) ModulateWeights(baseWeights map[string]float64, sit Situation) map[string]float64 {
	modulated := copyMap(baseWeights)

	// Regras situacionais (baseado em psicologia)

	// LUTO: aumenta ansiedade, busca de segurança, reduz extroversão
	if contains(sit.Stressors, "luto") {
		modulated["ANSIEDADE"] = getOrDefault(modulated, "ANSIEDADE", 0.5) * 1.8
		modulated["BUSCA_SEGURANÇA"] = getOrDefault(modulated, "BUSCA_SEGURANÇA", 0.5) * 2.0
		modulated["EXTROVERSÃO"] = getOrDefault(modulated, "EXTROVERSÃO", 0.5) * 0.5
		modulated["TRISTEZA"] = getOrDefault(modulated, "TRISTEZA", 0.3) * 2.0
	}

	// HOSPITAL/DOENÇA: aumenta alerta, busca de segurança
	if contains(sit.Stressors, "hospital") || contains(sit.Stressors, "doença") {
		modulated["ALERTA"] = getOrDefault(modulated, "ALERTA", 0.5) * 2.0
		modulated["BUSCA_SEGURANÇA"] = getOrDefault(modulated, "BUSCA_SEGURANÇA", 0.5) * 1.5
		modulated["PREOCUPAÇÃO"] = getOrDefault(modulated, "PREOCUPAÇÃO", 0.5) * 1.8
	}

	// ANIVERSÁRIO/FESTA: NÃO inflar alegria (é esperado)
	// MAS se pessoa SÉRIA em festa → trait incomum → inflar
	if contains(sit.Stressors, "aniversário") || contains(sit.Stressors, "festa") {
		extroversao := getOrDefault(modulated, "EXTROVERSÃO", 0.5)
		if extroversao < 0.4 {
			// Comportamento incomum = mais informativo
			modulated["EXTROVERSÃO"] = extroversao * (1.0 + sit.Intensity)
		}
	}

	// MADRUGADA + SOZINHO: aumenta solidão, ansiedade
	if sit.SocialContext == "sozinho" && sit.TimeOfDay == "madrugada" {
		modulated["SOLIDÃO"] = getOrDefault(modulated, "SOLIDÃO", 0.3) * 1.5
		modulated["ANSIEDADE"] = getOrDefault(modulated, "ANSIEDADE", 0.5) * 1.3
	}

	// CRISE: dispara alerta máximo
	if contains(sit.Stressors, "crise") {
		modulated["ALERTA"] = getOrDefault(modulated, "ALERTA", 0.5) * 2.5
		modulated["ANSIEDADE"] = getOrDefault(modulated, "ANSIEDADE", 0.5) * 2.0
		modulated["DESESPERO"] = getOrDefault(modulated, "DESESPERO", 0.2) * 3.0
	}

	// EMOÇÃO NEGATIVA: aumenta depressão
	if sit.EmotionScore < -0.5 {
		modulated["DEPRESSÃO"] = getOrDefault(modulated, "DEPRESSÃO", 0.3) * 1.5
		modulated["TRISTEZA"] = getOrDefault(modulated, "TRISTEZA", 0.3) * 1.4
	}

	// EMOÇÃO POSITIVA: aumenta alegria (mas com cautela)
	if sit.EmotionScore > 0.5 {
		modulated["ALEGRIA"] = getOrDefault(modulated, "ALEGRIA", 0.5) * 1.3
	}

	return modulated
}

// extractStressors extrai stressors do texto via keywords
func (m *SituationalModulator) extractStressors(text string) []string {
	stressors := []string{}
	textLower := strings.ToLower(text)

	for stressor, keywords := range m.stressorKeywords {
		for _, keyword := range keywords {
			if strings.Contains(textLower, keyword) {
				if !contains(stressors, stressor) {
					stressors = append(stressors, stressor)
				}
				break
			}
		}
	}

	return stressors
}

// Helper functions

func getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 0 && hour < 6:
		return "madrugada"
	case hour >= 6 && hour < 12:
		return "manha"
	case hour >= 12 && hour < 18:
		return "tarde"
	default:
		return "noite"
	}
}

func inferSocial(events []Event) string {
	if len(events) == 0 {
		return "sozinho"
	}

	// Lógica: se mencionou pessoas recentemente = "familia" ou "publico"
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Content), "familia") ||
			strings.Contains(strings.ToLower(event.Content), "filho") ||
			strings.Contains(strings.ToLower(event.Content), "esposa") {
			return "familia"
		}
	}

	return "publico"
}

func inferEmotion(text string) float64 {
	// Sentiment analysis básico (keywords)
	positive := []string{"feliz", "alegre", "bom", "ótimo", "maravilhoso", "contente", "satisfeito"}
	negative := []string{"triste", "mal", "ruim", "péssimo", "horrível", "deprimido", "ansioso", "preocupado"}

	score := 0.0
	textLower := strings.ToLower(text)

	for _, word := range positive {
		if strings.Contains(textLower, word) {
			score += 0.3
		}
	}

	for _, word := range negative {
		if strings.Contains(textLower, word) {
			score -= 0.3
		}
	}

	// Limitar entre -1.0 e 1.0
	return math.Max(-1.0, math.Min(1.0, score))
}

func calculateIntensity(stressors []string, emotionScore float64) float64 {
	// Intensidade baseada em número de stressors + magnitude emocional
	intensity := float64(len(stressors)) * 0.3
	intensity += math.Abs(emotionScore) * 0.5

	return math.Min(1.0, intensity)
}

func copyMap(m map[string]float64) map[string]float64 {
	copy := make(map[string]float64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getOrDefault(m map[string]float64, key string, defaultValue float64) float64 {
	if val, exists := m[key]; exists {
		return val
	}
	return defaultValue
}

func getDefaultStressorKeywords() map[string][]string {
	return map[string][]string{
		"luto":        {"faleceu", "morreu", "morte", "velório", "funeral", "falecimento"},
		"hospital":    {"hospital", "internado", "internação", "uti", "emergência"},
		"doença":      {"doente", "doença", "enfermo", "mal", "sintoma"},
		"aniversário": {"aniversário", "niver", "parabéns"},
		"festa":       {"festa", "celebração", "comemoração", "confraternização"},
		"crise":       {"crise", "pânico", "desespero", "emergência", "ajuda"},
	}
}

// GetEnneagramBaseWeights returns base cognitive weights for an Enneagram type (1-9)
// Migrated from internal/cortex/personality/situation_modulator.go (consolidation)
func GetEnneagramBaseWeights(enneaType int) map[string]float64 {
	baseWeights := map[int]map[string]float64{
		1: {"PERFECCIONISMO": 0.9, "FRUSTRAÇÃO": 0.7, "RESPONSABILIDADE": 0.8},
		2: {"BUSCA_CONEXAO": 0.9, "EMPATIA": 0.8, "NECESSIDADE_APROVACAO": 0.7},
		3: {"AMBIÇÃO": 0.9, "IMAGEM": 0.8, "EFICIÊNCIA": 0.8},
		4: {"PROFUNDIDADE_EMOCIONAL": 0.9, "AUTENTICIDADE": 0.8, "TRISTEZA": 0.6},
		5: {"AUTONOMIA": 0.9, "ANÁLISE": 0.8, "ISOLAMENTO": 0.6},
		6: {"ANSIEDADE": 0.9, "BUSCA_SEGURANÇA": 0.8, "LEALDADE": 0.7},
		7: {"ENTUSIASMO": 0.9, "INQUIETAÇÃO": 0.7, "EVITAÇÃO_DOR": 0.6},
		8: {"CONTROLE": 0.9, "RESISTÊNCIA": 0.8, "PROTEÇÃO": 0.7},
		9: {"HARMONIA": 0.9, "APATIA": 0.6, "EVITAÇÃO_CONFLITO": 0.8},
	}

	if weights, exists := baseWeights[enneaType]; exists {
		result := make(map[string]float64)
		for k, v := range weights {
			result[k] = v
		}
		return result
	}

	return map[string]float64{}
}