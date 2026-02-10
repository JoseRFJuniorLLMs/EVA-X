package lacan

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// LacanDBConfig gerencia configurações lacanianas do banco de dados
type LacanDBConfig struct {
	db    *sql.DB
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
func NewLacanDBConfig(db *sql.DB) *LacanDBConfig {
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
	log.Printf("✅ [LacanDBConfig] Configurações carregadas do PostgreSQL")

	return nil
}

// loadTransferenciaPatterns carrega padrões de transferência
func (c *LacanDBConfig) loadTransferenciaPatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryContext(ctx, `
		SELECT transferencia_type, keywords
		FROM lacan_transferencia_patterns
		WHERE active = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tType string
		var keywords []string
		if err := rows.Scan(&tType, &keywords); err != nil {
			continue
		}
		c.cache.transferenciaPatterns[tType] = keywords
	}

	// Carregar guidance
	guidanceRows, err := c.db.QueryContext(ctx, `
		SELECT transferencia_type, guidance_text,
		       COALESCE(clinical_implications, ''),
		       COALESCE(therapeutic_approach, '')
		FROM lacan_transferencia_guidance
	`)
	if err != nil {
		return err
	}
	defer guidanceRows.Close()

	for guidanceRows.Next() {
		var tType, guidance, implications, approach string
		if err := guidanceRows.Scan(&tType, &guidance, &implications, &approach); err != nil {
			continue
		}
		c.cache.transferenciaGuidance[tType] = TransferenciaGuidance{
			GuidanceText:         guidance,
			ClinicalImplications: implications,
			TherapeuticApproach:  approach,
		}
	}

	return nil
}

// loadDesirePatterns carrega padrões de desejo
func (c *LacanDBConfig) loadDesirePatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryContext(ctx, `
		SELECT latent_desire, keywords, confidence, COALESCE(description, '')
		FROM lacan_desire_patterns
		WHERE active = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pattern DesirePattern
		var keywords []string
		if err := rows.Scan(&pattern.LatentDesire, &keywords, &pattern.Confidence, &pattern.Description); err != nil {
			continue
		}
		pattern.Keywords = keywords
		c.cache.desirePatterns[pattern.LatentDesire] = pattern
	}

	// Carregar responses
	respRows, err := c.db.QueryContext(ctx, `
		SELECT latent_desire, suggested_response, clinical_guidance,
		       COALESCE(dialogue_strategy, ''), COALESCE(never_do, '')
		FROM lacan_desire_responses
	`)
	if err != nil {
		return err
	}
	defer respRows.Close()

	for respRows.Next() {
		var desire string
		var resp DesireResponse
		if err := respRows.Scan(&desire, &resp.SuggestedResponse, &resp.ClinicalGuidance, &resp.DialogueStrategy, &resp.NeverDo); err != nil {
			continue
		}
		c.cache.desireResponses[desire] = resp
	}

	return nil
}

// loadEmotionalKeywords carrega palavras emocionais
func (c *LacanDBConfig) loadEmotionalKeywords(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT keyword, emotional_charge, COALESCE(category, ''),
		       COALESCE(psychoanalytic_significance, ''), requires_attention
		FROM lacan_emotional_keywords
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var kw EmotionalKeyword
		if err := rows.Scan(&kw.Keyword, &kw.EmotionalCharge, &kw.Category, &kw.PsychoanalyticSignificance, &kw.RequiresAttention); err != nil {
			continue
		}
		c.cache.emotionalKeywords[kw.Keyword] = kw
	}

	return nil
}

// loadAddresseePatterns carrega padrões de destinatário
func (c *LacanDBConfig) loadAddresseePatterns(ctx context.Context) error {
	// Carregar patterns
	rows, err := c.db.QueryContext(ctx, `
		SELECT addressee_type, detection_keywords,
		       COALESCE(symbolic_function, ''), typical_demands
		FROM lacan_addressee_patterns
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pattern AddresseePattern
		var keywords, demands []string
		if err := rows.Scan(&pattern.AddresseeType, &keywords, &pattern.SymbolicFunction, &demands); err != nil {
			continue
		}
		pattern.DetectionKeywords = keywords
		pattern.TypicalDemands = demands
		c.cache.addresseePatterns[pattern.AddresseeType] = pattern
	}

	// Carregar guidance
	guidanceRows, err := c.db.QueryContext(ctx, `
		SELECT addressee_type, guidance_text,
		       COALESCE(intervention_strategy, ''), COALESCE(clinical_caveats, '')
		FROM lacan_addressee_guidance
	`)
	if err != nil {
		return err
	}
	defer guidanceRows.Close()

	for guidanceRows.Next() {
		var aType string
		var guidance AddresseeGuidance
		if err := guidanceRows.Scan(&aType, &guidance.GuidanceText, &guidance.InterventionStrategy, &guidance.ClinicalCaveats); err != nil {
			continue
		}
		c.cache.addresseeGuidance[aType] = guidance
	}

	return nil
}

// loadEthicalPrinciples carrega princípios éticos
func (c *LacanDBConfig) loadEthicalPrinciples(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT principle_code, principle_text, clinical_instruction,
		       COALESCE(portuguese_rationale, ''), priority
		FROM lacan_ethical_principles
		ORDER BY priority DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	c.cache.ethicalPrinciples = nil
	for rows.Next() {
		var p EthicalPrinciple
		if err := rows.Scan(&p.PrincipleCode, &p.PrincipleText, &p.ClinicalInstruction, &p.PortugueseRationale, &p.Priority); err != nil {
			continue
		}
		c.cache.ethicalPrinciples = append(c.cache.ethicalPrinciples, p)
	}

	return nil
}

// loadConfig carrega configurações gerais
func (c *LacanDBConfig) loadConfig(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT config_key, config_value FROM lacan_config
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		c.cache.config[key] = value
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
