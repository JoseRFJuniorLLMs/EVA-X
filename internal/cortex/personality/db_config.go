// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"eva/internal/brainstem/database"
)

// EnneagramDBConfig gerencia configurações do Enneagram do banco de dados
type EnneagramDBConfig struct {
	db    *database.DB
	cache *enneagramCache
	mu    sync.RWMutex
}

// enneagramCache armazena configurações em memória
type enneagramCache struct {
	types              map[int]EnneagramTypeInfo
	attentionWeights   map[int]map[string]float64
	stressPoints       map[int]int
	growthPoints       map[int]int
	relationshipLevels map[int]RelationshipLevel
	config             map[string]string
	loadedAt           time.Time
}

// EnneagramTypeInfo representa informações completas de um tipo
type EnneagramTypeInfo struct {
	TypeID                 int
	TypeName               string
	TypeNameEN             string
	Archetype              string
	CoreMotivation         string
	CoreFear               string
	PersonalityDescription string
	LLMInstruction         string
	StressPoint            int
	GrowthPoint            int
	WingOptions            []int
}

// RelationshipLevel representa um nível de relacionamento
type RelationshipLevel struct {
	Level            int
	LevelName        string
	Description      string
	InteractionStyle string
	AutonomyDegree   float64
	MinConversations int
}

// NewEnneagramDBConfig cria um novo gerenciador de configurações
func NewEnneagramDBConfig(db *database.DB) *EnneagramDBConfig {
	config := &EnneagramDBConfig{
		db:    db,
		cache: &enneagramCache{},
	}

	// Carregar configurações na inicialização
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := config.LoadAll(ctx); err != nil {
		log.Printf("⚠️ [EnneagramDBConfig] Erro ao carregar configurações: %v (usando fallback)", err)
	}

	return config
}

// LoadAll carrega todas as configurações do banco
func (c *EnneagramDBConfig) LoadAll(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = &enneagramCache{
		types:              make(map[int]EnneagramTypeInfo),
		attentionWeights:   make(map[int]map[string]float64),
		stressPoints:       make(map[int]int),
		growthPoints:       make(map[int]int),
		relationshipLevels: make(map[int]RelationshipLevel),
		config:             make(map[string]string),
	}

	if err := c.loadTypes(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar tipos: %v", err)
	}

	if err := c.loadAttentionWeights(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar pesos de atenção: %v", err)
	}

	if err := c.loadMovements(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar movimentos: %v", err)
	}

	if err := c.loadRelationshipLevels(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar níveis de relacionamento: %v", err)
	}

	if err := c.loadConfig(ctx); err != nil {
		log.Printf("⚠️ Erro ao carregar config: %v", err)
	}

	c.cache.loadedAt = time.Now()
	log.Printf("✅ [EnneagramDBConfig] Configurações carregadas do NietzscheDB")

	return nil
}

// loadTypes carrega os tipos do Enneagram
func (c *EnneagramDBConfig) loadTypes(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "enneagram_types", " AND n.active = $active", map[string]interface{}{"active": true}, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		t := EnneagramTypeInfo{
			TypeID:                 int(database.GetInt64(row, "type_id")),
			TypeName:               database.GetString(row, "type_name"),
			TypeNameEN:             database.GetString(row, "type_name_en"),
			Archetype:              database.GetString(row, "archetype"),
			CoreMotivation:         database.GetString(row, "core_motivation"),
			CoreFear:               database.GetString(row, "core_fear"),
			PersonalityDescription: database.GetString(row, "personality_description"),
			LLMInstruction:         database.GetString(row, "llm_instruction"),
			StressPoint:            int(database.GetInt64(row, "stress_point")),
			GrowthPoint:            int(database.GetInt64(row, "growth_point")),
		}

		// Parse wing_options from []interface{} to []int
		if wRaw, ok := row["wing_options"]; ok && wRaw != nil {
			if wSlice, ok := wRaw.([]interface{}); ok {
				for _, v := range wSlice {
					switch w := v.(type) {
					case float64:
						t.WingOptions = append(t.WingOptions, int(w))
					case int64:
						t.WingOptions = append(t.WingOptions, int(w))
					case int:
						t.WingOptions = append(t.WingOptions, w)
					}
				}
			}
		}

		c.cache.types[t.TypeID] = t
	}

	return nil
}

// loadAttentionWeights carrega os pesos de atenção
func (c *EnneagramDBConfig) loadAttentionWeights(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "enneagram_attention_weights", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		typeID := int(database.GetInt64(row, "type_id"))
		concept := database.GetString(row, "attention_concept")
		weight := database.GetFloat64(row, "weight_multiplier")

		if c.cache.attentionWeights[typeID] == nil {
			c.cache.attentionWeights[typeID] = make(map[string]float64)
		}
		c.cache.attentionWeights[typeID][concept] = weight
	}

	return nil
}

// loadMovements carrega os movimentos de estresse e crescimento
func (c *EnneagramDBConfig) loadMovements(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "enneagram_movements", "", nil, 0)
	if err != nil {
		return err
	}

	for _, row := range rows {
		fromType := int(database.GetInt64(row, "from_type"))
		toType := int(database.GetInt64(row, "to_type"))
		movementType := database.GetString(row, "movement_type")

		if movementType == "stress" {
			c.cache.stressPoints[fromType] = toType
		} else if movementType == "growth" {
			c.cache.growthPoints[fromType] = toType
		}
	}

	return nil
}

// loadRelationshipLevels carrega os níveis de relacionamento
func (c *EnneagramDBConfig) loadRelationshipLevels(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "relationship_levels", "", nil, 0)
	if err != nil {
		return err
	}

	// Sort by level ASC (NietzscheDB has no ORDER BY)
	sort.Slice(rows, func(i, j int) bool {
		return database.GetInt64(rows[i], "level") < database.GetInt64(rows[j], "level")
	})

	for _, row := range rows {
		rl := RelationshipLevel{
			Level:            int(database.GetInt64(row, "level")),
			LevelName:        database.GetString(row, "level_name"),
			Description:      database.GetString(row, "description"),
			InteractionStyle: database.GetString(row, "interaction_style"),
			AutonomyDegree:   database.GetFloat64(row, "autonomy_degree"),
			MinConversations: int(database.GetInt64(row, "min_conversations")),
		}
		c.cache.relationshipLevels[rl.Level] = rl
	}

	return nil
}

// loadConfig carrega configurações gerais
func (c *EnneagramDBConfig) loadConfig(ctx context.Context) error {
	rows, err := c.db.QueryByLabel(ctx, "enneagram_config", "", nil, 0)
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

// ============================================================================
// MÉTODOS DE ACESSO
// ============================================================================

// GetTypeInfo retorna informações de um tipo
func (c *EnneagramDBConfig) GetTypeInfo(typeID int) *EnneagramTypeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if t, ok := c.cache.types[typeID]; ok {
		return &t
	}
	return nil
}

// GetLLMInstruction retorna a instrução LLM para um tipo
func (c *EnneagramDBConfig) GetLLMInstruction(typeID int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if t, ok := c.cache.types[typeID]; ok {
		return t.LLMInstruction
	}
	// Fallback para Tipo 9
	return "Você está no modo PACIFICADOR (Tipo 9). Seja calma, aceitadora e harmoniosa."
}

// GetAttentionWeights retorna os pesos de atenção para um tipo
func (c *EnneagramDBConfig) GetAttentionWeights(typeID int) map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if weights, ok := c.cache.attentionWeights[typeID]; ok {
		result := make(map[string]float64)
		for k, v := range weights {
			result[k] = v
		}
		return result
	}

	// Fallback para Tipo 9
	return map[string]float64{
		"HARMONIA": 1.9,
		"PAZ":      1.85,
		"UNIÃO":    1.8,
		"CONFLITO": 0.5,
	}
}

// GetStressPoint retorna o ponto de estresse para um tipo
func (c *EnneagramDBConfig) GetStressPoint(typeID int) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if point, ok := c.cache.stressPoints[typeID]; ok {
		return point
	}
	return typeID // Fallback: retorna o próprio tipo
}

// GetGrowthPoint retorna o ponto de crescimento para um tipo
func (c *EnneagramDBConfig) GetGrowthPoint(typeID int) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if point, ok := c.cache.growthPoints[typeID]; ok {
		return point
	}
	return typeID // Fallback: retorna o próprio tipo
}

// GetRelationshipLevel retorna informações do nível de relacionamento
func (c *EnneagramDBConfig) GetRelationshipLevel(level int) *RelationshipLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if rl, ok := c.cache.relationshipLevels[level]; ok {
		return &rl
	}
	return nil
}

// GetRelationshipLevelByConversations calcula o nível baseado em conversas
func (c *EnneagramDBConfig) GetRelationshipLevelByConversations(conversations int) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Encontrar o nível apropriado baseado em conversas
	resultLevel := 1
	for level, rl := range c.cache.relationshipLevels {
		if conversations >= rl.MinConversations && level > resultLevel {
			resultLevel = level
		}
	}

	if resultLevel > 10 {
		resultLevel = 10
	}

	return resultLevel
}

// GetRelationshipLevelName retorna o nome do nível
func (c *EnneagramDBConfig) GetRelationshipLevelName(level int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if rl, ok := c.cache.relationshipLevels[level]; ok {
		return rl.LevelName
	}

	// Fallback hardcoded
	names := map[int]string{
		1: "Nos conhecendo", 2: "Conhecidas", 3: "Amigas",
		4: "Boas amigas", 5: "Amigas próximas", 6: "Confidentes",
		7: "Muito próximas", 8: "Inseparáveis", 9: "Como família",
		10: "Família do coração",
	}
	if name, ok := names[level]; ok {
		return name
	}
	return "Desconhecido"
}

// GetConfig retorna configuração por chave
func (c *EnneagramDBConfig) GetConfig(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache.config[key]
}

// GetConfigInt retorna configuração como inteiro
func (c *EnneagramDBConfig) GetConfigInt(key string, defaultVal int) int {
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

// GetDefaultType retorna o tipo padrão da EVA
func (c *EnneagramDBConfig) GetDefaultType() int {
	return c.GetConfigInt("default_type", 9) // Pacificador
}

// GetDefaultWing retorna a asa padrão da EVA
func (c *EnneagramDBConfig) GetDefaultWing() int {
	return c.GetConfigInt("default_wing", 8) // Asa 8
}

// Reload recarrega todas as configurações
func (c *EnneagramDBConfig) Reload(ctx context.Context) error {
	return c.LoadAll(ctx)
}

// LastLoaded retorna quando as configurações foram carregadas
func (c *EnneagramDBConfig) LastLoaded() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.loadedAt
}
