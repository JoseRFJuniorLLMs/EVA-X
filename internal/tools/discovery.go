// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"eva/internal/brainstem/database"
)

// ToolDiscoveryService gerencia descoberta e combinação de ferramentas
type ToolDiscoveryService struct {
	repo       *ToolRepository
	db         *database.DB
	cache      *toolCache
	useDynamic bool // Se true, usa banco; se false, usa apenas código
}

// toolCache mantém cache das ferramentas para evitar queries repetidas
type toolCache struct {
	mu              sync.RWMutex
	tools           []FunctionDeclaration
	capabilities    []*CapabilityRecord
	toolsLastUpdate time.Time
	capsLastUpdate  time.Time
	ttl             time.Duration
}

// NewToolDiscoveryService cria o serviço de descoberta
func NewToolDiscoveryService(db *database.DB) *ToolDiscoveryService {
	service := &ToolDiscoveryService{
		db:         db,
		useDynamic: db != nil,
		cache: &toolCache{
			ttl: 5 * time.Minute, // Cache de 5 minutos
		},
	}

	if db != nil {
		service.repo = NewToolRepository(db)
	}

	return service
}

// GetToolDefinitions retorna todas as ferramentas disponíveis
// Combina ferramentas estáticas (código) com dinâmicas (banco)
func (s *ToolDiscoveryService) GetToolDefinitions(ctx context.Context) []FunctionDeclaration {
	// Tentar cache primeiro
	s.cache.mu.RLock()
	if time.Since(s.cache.toolsLastUpdate) < s.cache.ttl && len(s.cache.tools) > 0 {
		tools := s.cache.tools
		s.cache.mu.RUnlock()
		return tools
	}
	s.cache.mu.RUnlock()

	// Buscar ferramentas
	var tools []FunctionDeclaration

	if s.useDynamic && s.repo != nil {
		// Tentar buscar do banco primeiro
		dbTools, err := s.repo.GetToolsForGemini(ctx)
		if err != nil {
			log.Printf("⚠️ [DISCOVERY] Erro ao buscar ferramentas do banco: %v, usando estáticas", err)
			tools = GetToolDefinitions() // Fallback para estáticas
		} else if len(dbTools) > 0 {
			log.Printf("✅ [DISCOVERY] %d ferramentas carregadas do banco", len(dbTools))
			tools = dbTools
		} else {
			// Banco vazio, usar estáticas
			log.Printf("⚠️ [DISCOVERY] Banco vazio, usando ferramentas estáticas")
			tools = GetToolDefinitions()
		}
	} else {
		// Sem banco, usar apenas estáticas
		tools = GetToolDefinitions()
	}

	// Atualizar cache
	s.cache.mu.Lock()
	s.cache.tools = tools
	s.cache.toolsLastUpdate = time.Now()
	s.cache.mu.Unlock()

	return tools
}

// GetCapabilitiesPrompt gera texto de capacidades para incluir no prompt do Gemini
func (s *ToolDiscoveryService) GetCapabilitiesPrompt(ctx context.Context) string {
	// Tentar cache primeiro
	s.cache.mu.RLock()
	if time.Since(s.cache.capsLastUpdate) < s.cache.ttl && len(s.cache.capabilities) > 0 {
		caps := s.cache.capabilities
		s.cache.mu.RUnlock()
		return s.formatCapabilities(caps)
	}
	s.cache.mu.RUnlock()

	var capabilities []*CapabilityRecord

	if s.useDynamic && s.repo != nil {
		var err error
		capabilities, err = s.repo.GetCapabilities(ctx)
		if err != nil {
			log.Printf("⚠️ [DISCOVERY] Erro ao buscar capacidades: %v", err)
			return s.getDefaultCapabilitiesPrompt()
		}
	}

	if len(capabilities) == 0 {
		return s.getDefaultCapabilitiesPrompt()
	}

	// Atualizar cache
	s.cache.mu.Lock()
	s.cache.capabilities = capabilities
	s.cache.capsLastUpdate = time.Now()
	s.cache.mu.Unlock()

	return s.formatCapabilities(capabilities)
}

// formatCapabilities formata capacidades para o prompt
func (s *ToolDiscoveryService) formatCapabilities(caps []*CapabilityRecord) string {
	var sb strings.Builder

	sb.WriteString("\n## MINHAS CAPACIDADES\n\n")
	sb.WriteString("Eu tenho as seguintes habilidades que posso usar para ajudá-lo:\n\n")

	for _, cap := range caps {
		sb.WriteString(fmt.Sprintf("### %s\n", cap.CapabilityName))
		sb.WriteString(fmt.Sprintf("%s\n", cap.ShortDescription))

		if cap.WhenToUse != "" {
			sb.WriteString(fmt.Sprintf("**Quando usar:** %s\n", cap.WhenToUse))
		}

		if len(cap.ExampleQueries) > 0 {
			sb.WriteString("**Exemplos de perguntas:**\n")
			for _, ex := range cap.ExampleQueries {
				sb.WriteString(fmt.Sprintf("  - \"%s\"\n", ex))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// getDefaultCapabilitiesPrompt retorna prompt padrão quando não há dados do banco
func (s *ToolDiscoveryService) getDefaultCapabilitiesPrompt() string {
	return `
## MINHAS CAPACIDADES

Eu tenho as seguintes habilidades que posso usar para ajudá-lo:

### Monitoramento de Saúde
Posso verificar seus sinais vitais e histórico de saúde.
**Quando usar:** Quando você perguntar sobre sua saúde física.
**Exemplos:** "como está minha pressão?", "minha glicose está normal?"

### Gestão de Agenda
Posso verificar sua agenda de consultas e horários de medicação.
**Quando usar:** Quando você perguntar sobre consultas ou remédios.
**Exemplos:** "tenho consulta hoje?", "que horas tomo meu remédio?"

### Identificação de Medicamentos
Posso usar a câmera para identificar seus medicamentos.
**Quando usar:** Quando você tiver dúvidas sobre qual remédio tomar.
**Exemplos:** "qual é esse remédio?", "não sei qual tomar agora"

### Avaliação Emocional
Posso fazer uma avaliação cuidadosa de como você está se sentindo.
**Quando usar:** Quando você expressar tristeza ou ansiedade persistente.

### Detecção de Crise
🚨 Posso ajudar em momentos de crise e acionar suporte imediato.
**Quando usar:** APENAS em situações de emergência emocional.
`
}

// GetToolByName retorna uma ferramenta pelo nome
func (s *ToolDiscoveryService) GetToolByName(ctx context.Context, name string) (*ToolRecord, error) {
	if !s.useDynamic || s.repo == nil {
		return nil, fmt.Errorf("dynamic tools not available")
	}
	return s.repo.GetByName(ctx, name)
}

// GetToolsByCategory retorna ferramentas de uma categoria
func (s *ToolDiscoveryService) GetToolsByCategory(ctx context.Context, category string) ([]*ToolRecord, error) {
	if !s.useDynamic || s.repo == nil {
		return nil, fmt.Errorf("dynamic tools not available")
	}
	return s.repo.GetByCategory(ctx, category)
}

// RegisterTool registra uma nova ferramenta
func (s *ToolDiscoveryService) RegisterTool(ctx context.Context, tool *ToolRecord) error {
	if !s.useDynamic || s.repo == nil {
		return fmt.Errorf("dynamic tools not available")
	}

	err := s.repo.Register(ctx, tool)
	if err != nil {
		return err
	}

	// Invalidar cache
	s.InvalidateCache()
	return nil
}

// EnableTool habilita uma ferramenta
func (s *ToolDiscoveryService) EnableTool(ctx context.Context, name string) error {
	if !s.useDynamic || s.repo == nil {
		return fmt.Errorf("dynamic tools not available")
	}

	err := s.repo.Enable(ctx, name)
	if err != nil {
		return err
	}

	s.InvalidateCache()
	return nil
}

// DisableTool desabilita uma ferramenta
func (s *ToolDiscoveryService) DisableTool(ctx context.Context, name string) error {
	if !s.useDynamic || s.repo == nil {
		return fmt.Errorf("dynamic tools not available")
	}

	err := s.repo.Disable(ctx, name)
	if err != nil {
		return err
	}

	s.InvalidateCache()
	return nil
}

// LogToolInvocation registra uma invocação de ferramenta
func (s *ToolDiscoveryService) LogToolInvocation(ctx context.Context, toolName string, idosoID int64,
	sessionID string, invokedBy string, params map[string]interface{}, triggerPhrase string) (int64, error) {

	if !s.useDynamic || s.repo == nil {
		return 0, nil // Silently skip if no dynamic tools
	}

	return s.repo.LogInvocation(ctx, toolName, idosoID, sessionID, invokedBy, params, triggerPhrase)
}

// CompleteToolInvocation marca invocação como concluída
func (s *ToolDiscoveryService) CompleteToolInvocation(ctx context.Context, logID int64, success bool,
	result map[string]interface{}, errorMsg string, executionTimeMs int) error {

	if !s.useDynamic || s.repo == nil || logID == 0 {
		return nil // Silently skip
	}

	return s.repo.CompleteInvocation(ctx, logID, success, result, errorMsg, executionTimeMs)
}

// InvalidateCache invalida o cache de ferramentas
func (s *ToolDiscoveryService) InvalidateCache() {
	s.cache.mu.Lock()
	s.cache.tools = nil
	s.cache.capabilities = nil
	s.cache.toolsLastUpdate = time.Time{}
	s.cache.capsLastUpdate = time.Time{}
	s.cache.mu.Unlock()
	log.Printf("🔄 [DISCOVERY] Cache de ferramentas invalidado")
}

// GetStats retorna estatísticas do serviço
func (s *ToolDiscoveryService) GetStats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"dynamic_enabled": s.useDynamic,
		"cache_ttl":       s.cache.ttl.String(),
	}

	if s.useDynamic && s.repo != nil {
		tools, _ := s.repo.GetAll(ctx)
		stats["total_tools"] = len(tools)

		enabledCount := 0
		criticalCount := 0
		totalInvocations := int64(0)

		for _, t := range tools {
			if t.Enabled {
				enabledCount++
			}
			if t.IsCritical {
				criticalCount++
			}
			totalInvocations += t.TotalInvocations
		}

		stats["enabled_tools"] = enabledCount
		stats["critical_tools"] = criticalCount
		stats["total_invocations"] = totalInvocations

		caps, _ := s.repo.GetCapabilities(ctx)
		stats["capabilities"] = len(caps)
	}

	return stats
}

// HealthCheck verifica se o serviço está funcionando
func (s *ToolDiscoveryService) HealthCheck(ctx context.Context) error {
	if !s.useDynamic {
		return nil // Static tools always work
	}

	tools, err := s.repo.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to query tools: %w", err)
	}

	if len(tools) == 0 {
		return fmt.Errorf("no tools available")
	}

	return nil
}

// SyncStaticTools sincroniza ferramentas estáticas com o banco
// Útil para garantir que novas ferramentas do código sejam registradas
func (s *ToolDiscoveryService) SyncStaticTools(ctx context.Context) error {
	if !s.useDynamic || s.repo == nil {
		return fmt.Errorf("dynamic tools not available")
	}

	staticTools := GetToolDefinitions()

	for _, static := range staticTools {
		// Verificar se já existe no banco
		existing, err := s.repo.GetByName(ctx, static.Name)
		if err == nil && existing != nil {
			// Já existe, pular
			continue
		}

		// Criar registro
		record := &ToolRecord{
			Name:           static.Name,
			Description:    static.Description,
			Category:       "legacy",
			HandlerType:    "internal",
			Enabled:        true,
			Priority:       50,
			Version:        "1.0.0",
		}

		if static.Parameters != nil {
			// Converter para JSON
			// Simplificado: seria necessário marshal correto
			record.Parameters = []byte("{}")
		}

		if err := s.repo.Register(ctx, record); err != nil {
			log.Printf("⚠️ [DISCOVERY] Erro ao sincronizar tool %s: %v", static.Name, err)
		}
	}

	log.Printf("✅ [DISCOVERY] Ferramentas estáticas sincronizadas")
	return nil
}
