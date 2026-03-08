// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"eva/internal/brainstem/database"
)

// LacanDBConfig gerencia configurações lacanianas do banco de dados
type LacanDBConfig struct {
	db    *database.DB
	cache *lacanConfigCache
	mu    sync.RWMutex
}

// lacanConfigCache armazena configurações em memória
type lacanConfigCache struct {
	transferenciaPatterns map[string][]string
	transferenciaGuidance map[string]TransferenciaGuidance
	desirePatterns        map[string]DesirePattern
	desireResponses       map[string]DesireResponse
	emotionalKeywords     map[string]EmotionalKeyword
	addresseePatterns     map[string]AddresseePattern
	addresseeGuidance     map[string]AddresseeGuidance
	ethicalPrinciples     []EthicalPrinciple
	elaborationMarkers    map[string]string
	config                map[string]string
	loadedAt              time.Time
}

// TransferenciaGuidance representa orientação de transferência
type TransferenciaGuidance struct {
	GuidanceText         string
	ClinicalImplications string
	TherapeuticApproach  string
}

// DesirePattern representa um padrão de desejo latente
type DesirePattern struct {
	LatentDesire string
	Keywords     []string
	Confidence   float64
	Description  string
}

// DesireResponse representa uma resposta sugerida para desejo
type DesireResponse struct {
	SuggestedResponse string
	ClinicalGuidance  string
	DialogueStrategy  string
	NeverDo           string
}

// EmotionalKeyword representa uma palavra emocional
type EmotionalKeyword struct {
	Keyword                    string
	EmotionalCharge            string // 'normal', 'high', 'critical'
	Category                   string
	PsychoanalyticSignificance string
	RequiresAttention          bool
}

// AddresseePattern representa padrões de destinatário
type AddresseePattern struct {
	AddresseeType    string
	DetectionKeywords []string
	SymbolicFunction string
	TypicalDemands   []string
}

// AddresseeGuidance representa orientação para destinatário
type AddresseeGuidance struct {
	GuidanceText         string
	InterventionStrategy string
	ClinicalCaveats      string
}

// EthicalPrinciple representa um princípio ético lacaniano
type EthicalPrinciple struct {
	PrincipleCode        string
	PrincipleText        string
	ClinicalInstruction  string
	PortugueseRationale  string
	Priority             int
}

// NewLacanDBConfig cria um novo gerenciador de configurações
func NewLacanDBConfig(db *database.DB) *LacanDBConfig {
	config := &LacanDBConfig{
		db:    db,
		cache: &lacanConfigCache{},
	}

	// Carregar configurações na inicialização
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := config.LoadAll(ctx); err != nil {
		log.Printf("⚠️ [LacanDBConfig] Erro ao carregar configurações: %v (usando fallback)", err)
	}

	return config
}

// LoadAll carrega todas as configurações do banco
func (c *LacanDBConfig) LoadAll(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = &lacanConfigCache{
		transferenciaPatterns: make(map[string][]string),
		transferenciaGuidance: make(map[string]TransferenciaGuidance),
		desirePatterns:        make(map[string]DesirePattern),
		desireResponses:       make(map[string]DesireResponse),
		emotionalKeywords:     make(map[string]EmotionalKeyword),
		addresseePatterns:     make(map[string]AddresseePattern),
		addresseeGuidance:     make(map[string]AddresseeGuidance),
		elaborationMarkers:    make(map[string]string),
		config:                make(map[string]string),
	}

	// Carregar cada categoria
	if err := c.loadTransferenciaPatterns(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar transferencia patterns: %v", err)
	}

	if err := c.loadDesirePatterns(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar desire patterns: %v", err)
	}

	if err := c.loadEmotionalKeywords(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar emotional keywords: %v", err)
	}

	if err := c.loadAddresseePatterns(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar addressee patterns: %v", err)
	}

	if err := c.loadEthicalPrinciples(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar ethical principles: %v", err)
	}

	if err := c.loadConfig(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar config: %v", err)
	}

	c.cache.loadedAt = time.Now()
	log.Printf("✅ [LacanDBConfig] Configurações carregadas do NietzscheDB")

	return nil
}

// loadTransferenciaPatterns carrega padrões de transferência
func (c *LacanDBConfig) loadTransferenciaPatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryByLabel(ctx, "lacan_transferencia_patterns", " AND n.active = $active", map[string]interface{}{"active": true}, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		tType := database.GetString(row, "transferencia_type")
		keywords := getStringSlice(row, "keywords")
		if tType != "" {
			c.cache.transferenciaPatterns[tType] = keywords
		}
	}

	// Carregar guidance
	guidanceRows, err := c.db.QueryByLabel(ctx, "lacan_transferencia_guidance", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range guidanceRows {
		tType := database.GetString(row, "transferencia_type")
		if tType != "" {
			c.cache.transferenciaGuidance[tType] = TransferenciaGuidance{
				GuidanceText:         database.GetString(row, "guidance_text"),
				ClinicalImplications: database.GetString(row, "clinical_implications"),
				TherapeuticApproach:  database.GetString(row, "therapeutic_approach"),
			}
		}
	}

	return nil
}

// loadDesirePatterns carrega padrões de desejo
func (c *LacanDBConfig) loadDesirePatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryByLabel(ctx, "lacan_desire_patterns", " AND n.active = $active", map[string]interface{}{"active": true}, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		pattern := DesirePattern{
			LatentDesire: database.GetString(row, "latent_desire"),
			Keywords:     getStringSlice(row, "keywords"),
			Confidence:   database.GetFloat64(row, "confidence"),
			Description:  database.GetString(row, "description"),
		}
		if pattern.LatentDesire != "" {
			c.cache.desirePatterns[pattern.LatentDesire] = pattern
		}
	}

	// Carregar responses
	respRows, err := c.db.QueryByLabel(ctx, "lacan_desire_responses", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range respRows {
		desire := database.GetString(row, "latent_desire")
		if desire != "" {
			c.cache.desireResponses[desire] = DesireResponse{
				SuggestedResponse: database.GetString(row, "suggested_response"),
				ClinicalGuidance:  database.GetString(row, "clinical_guidance"),
				DialogueStrategy:  database.GetString(row, "dialogue_strategy"),
				NeverDo:           database.GetString(row, "never_do"),
			}
		}
	}

	return nil
}

// loadEmotionalKeywords carrega palavras emocionais
func (c *LacanDBConfig) loadEmotionalKeywords(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "lacan_emotional_keywords", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		kw := EmotionalKeyword{
			Keyword:                    database.GetString(row, "keyword"),
			EmotionalCharge:            database.GetString(row, "emotional_charge"),
			Category:                   database.GetString(row, "category"),
			PsychoanalyticSignificance: database.GetString(row, "psychoanalytic_significance"),
			RequiresAttention:          database.GetBool(row, "requires_attention"),
		}
		if kw.Keyword != "" {
			c.cache.emotionalKeywords[kw.Keyword] = kw
		}
	}

	return nil
}

// loadAddresseePatterns carrega padrões de destinatário
func (c *LacanDBConfig) loadAddresseePatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryByLabel(ctx, "lacan_addressee_patterns", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		pattern := AddresseePattern{
			AddresseeType:     database.GetString(row, "addressee_type"),
			DetectionKeywords: getStringSlice(row, "detection_keywords"),
			SymbolicFunction:  database.GetString(row, "symbolic_function"),
			TypicalDemands:    getStringSlice(row, "typical_demands"),
		}
		if pattern.AddresseeType != "" {
			c.cache.addresseePatterns[pattern.AddresseeType] = pattern
		}
	}

	// Carregar guidance
	guidanceRows, err := c.db.QueryByLabel(ctx, "lacan_addressee_guidance", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range guidanceRows {
		aType := database.GetString(row, "addressee_type")
		if aType != "" {
			c.cache.addresseeGuidance[aType] = AddresseeGuidance{
				GuidanceText:         database.GetString(row, "guidance_text"),
				InterventionStrategy: database.GetString(row, "intervention_strategy"),
				ClinicalCaveats:      database.GetString(row, "clinical_caveats"),
			}
		}
	}

	return nil
}

// loadEthicalPrinciples carrega princípios éticos
func (c *LacanDBConfig) loadEthicalPrinciples(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "lacan_ethical_principles", "", nil, 0)
	if err != nil {
		return err
	}

	c.cache.ethicalPrinciples = nil
	for _, row := range rows {
		p := EthicalPrinciple{
			PrincipleCode:       database.GetString(row, "principle_code"),
			PrincipleText:       database.GetString(row, "principle_text"),
			ClinicalInstruction: database.GetString(row, "clinical_instruction"),
			PortugueseRationale: database.GetString(row, "portuguese_rationale"),
			Priority:            int(database.GetInt64(row, "priority")),
		}
		c.cache.ethicalPrinciples = append(c.cache.ethicalPrinciples, p)
	}

	// Sort by priority DESC (NQL has no ORDER BY)
	sort.Slice(c.cache.ethicalPrinciples, func(i, j int) bool {
		return c.cache.ethicalPrinciples[i].Priority > c.cache.ethicalPrinciples[j].Priority
	})

	return nil
}

// loadConfig carrega configurações gerais
func (c *LacanDBConfig) loadConfig(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "lacan_config", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		key := database.GetString(row, "config_key")
		value := database.GetString(row, "config_value")
		if key != "" {
			c.cache.config[key] = value
		}
	}

	return nil
}

// getStringSlice extracts a []string from a NietzscheDB content map field.
func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}

// ============================================================================
// MÉTODOS DE ACESSO (leitura do cache)
// ============================================================================

// GetTransferenciaPatterns retorna padrões de transferência por tipo
func (c *LacanDBConfig) GetTransferenciaPatterns(tType string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if patterns, ok := c.cache.transferenciaPatterns[tType]; ok {
		return patterns
	}
	return nil
}

// GetTransferenciaGuidance retorna orientação de transferência
func (c *LacanDBConfig) GetTransferenciaGuidance(tType string) *TransferenciaGuidance {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if guidance, ok := c.cache.transferenciaGuidance[tType]; ok {
		return &guidance
	}
	return nil
}

// GetDesirePattern retorna padrão de desejo por tipo
func (c *LacanDBConfig) GetDesirePattern(desire string) *DesirePattern {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if pattern, ok := c.cache.desirePatterns[desire]; ok {
		return &pattern
	}
	return nil
}

// GetDesireResponse retorna resposta sugerida para desejo
func (c *LacanDBConfig) GetDesireResponse(desire string) *DesireResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if resp, ok := c.cache.desireResponses[desire]; ok {
		return &resp
	}
	return nil
}

// GetAllDesirePatterns retorna todos os padrões de desejo
func (c *LacanDBConfig) GetAllDesirePatterns() map[string]DesirePattern {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]DesirePattern)
	for k, v := range c.cache.desirePatterns {
		result[k] = v
	}
	return result
}

// IsEmotionalKeyword verifica se é palavra emocional
func (c *LacanDBConfig) IsEmotionalKeyword(word string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.cache.emotionalKeywords[word]
	return ok
}

// IsHighEmotionalCharge verifica se palavra tem alta carga
func (c *LacanDBConfig) IsHighEmotionalCharge(word string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if kw, ok := c.cache.emotionalKeywords[word]; ok {
		return kw.EmotionalCharge == "high" || kw.EmotionalCharge == "critical"
	}
	return false
}

// GetEmotionalKeyword retorna informações da palavra emocional
func (c *LacanDBConfig) GetEmotionalKeyword(word string) *EmotionalKeyword {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if kw, ok := c.cache.emotionalKeywords[word]; ok {
		return &kw
	}
	return nil
}

// GetAddresseeGuidance retorna orientação para destinatário
func (c *LacanDBConfig) GetAddresseeGuidance(addresseeType string) *AddresseeGuidance {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if guidance, ok := c.cache.addresseeGuidance[addresseeType]; ok {
		return &guidance
	}
	return nil
}

// GetEthicalPrinciples retorna princípios éticos
func (c *LacanDBConfig) GetEthicalPrinciples() []EthicalPrinciple {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]EthicalPrinciple, len(c.cache.ethicalPrinciples))
	copy(result, c.cache.ethicalPrinciples)
	return result
}

// GetConfig retorna configuração por chave
func (c *LacanDBConfig) GetConfig(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache.config[key]
}

// GetConfigInt retorna configuração como inteiro
func (c *LacanDBConfig) GetConfigInt(key string, defaultVal int) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache.config[key]; ok {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

// Reload recarrega todas as configurações
func (c *LacanDBConfig) Reload(ctx context.Context) error {
	return c.LoadAll(ctx)
}

// LastLoaded retorna quando as configurações foram carregadas
func (c *LacanDBConfig) LastLoaded() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.loadedAt
}
