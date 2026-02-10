package personality

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// EnneagramDBConfig gerencia configurações do Enneagram do banco de dados
type EnneagramDBConfig struct {
	db    *sql.DB
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
func NewEnneagramDBConfig(db *sql.DB) *EnneagramDBConfig {
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
	log.Printf("✅ [EnneagramDBConfig] Configurações carregadas do PostgreSQL")

	return nil
}

// loadTypes carrega os tipos do Enneagram
func (c *EnneagramDBConfig) loadTypes(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT type_id, type_name, COALESCE(type_name_en, ''), archetype,
		       COALESCE(core_motivation, ''), COALESCE(core_fear, ''),
		       personality_description, llm_instruction,
		       stress_point, growth_point, wing_options
		FROM enneagram_types
		WHERE active = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t EnneagramTypeInfo
		var wings []int
		if err := rows.Scan(
			&t.TypeID, &t.TypeName, &t.TypeNameEN, &t.Archetype,
			&t.CoreMotivation, &t.CoreFear,
			&t.PersonalityDescription, &t.LLMInstruction,
			&t.StressPoint, &t.GrowthPoint, &wings,
		); err != nil {
			continue
		}
		t.WingOptions = wings
		c.cache.types[t.TypeID] = t
	}

	return nil
}

// loadAttentionWeights carrega os pesos de atenção
func (c *EnneagramDBConfig) loadAttentionWeights(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT type_id, attention_concept, weight_multiplier
		FROM enneagram_attention_weights
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var typeID int
		var concept string
		var weight float64
		if err := rows.Scan(&typeID, &concept, &weight); err != nil {
			continue
		}

		if c.cache.attentionWeights[typeID] == nil {
			c.cache.attentionWeights[typeID] = make(map[string]float64)
		}
		c.cache.attentionWeights[typeID][concept] = weight
	}

	return nil
}

// loadMovements carrega os movimentos de estresse e crescimento
func (c *EnneagramDBConfig) loadMovements(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT from_type, to_type, movement_type
		FROM enneagram_movements
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var fromType, toType int
		var movementType string
		if err := rows.Scan(&fromType, &toType, &movementType); err != nil {
			continue
		}

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
	rows, err := c.db.QueryContext(ctx, `
		SELECT level, level_name, COALESCE(description, ''),
		       COALESCE(interaction_style, ''), autonomy_degree, min_conversations
		FROM relationship_levels
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rl RelationshipLevel
		if err := rows.Scan(
			&rl.Level, &rl.LevelName, &rl.Description,
			&rl.InteractionStyle, &rl.AutonomyDegree, &rl.MinConversations,
		); err != nil {
			continue
		}
		c.cache.relationshipLevels[rl.Level] = rl
	}

	return nil
}

// loadConfig carrega configurações gerais
func (c *EnneagramDBConfig) loadConfig(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `
		SELECT config_key, config_value FROM enneagram_config
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
