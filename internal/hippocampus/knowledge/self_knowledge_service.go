// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"eva/internal/brainstem/database"
)

// creatorCPF returns the creator CPF from env or fallback
func creatorCPF() string {
	if cpf := os.Getenv("CREATOR_CPF"); cpf != "" {
		return cpf
	}
	return "64525430249"
}

// SelfKnowledgeService permite a EVA consultar conhecimento sobre si mesma
type SelfKnowledgeService struct {
	db *database.DB
}

// KnowledgeEntry representa um registro de conhecimento
type KnowledgeEntry struct {
	ID              int64    `json:"id"`
	KnowledgeType   string   `json:"knowledge_type"`
	KnowledgeKey    string   `json:"knowledge_key"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	DetailedContent string   `json:"detailed_content"`
	CodeLocation    string   `json:"code_location"`
	ParentKey       string   `json:"parent_key"`
	RelatedKeys     []string `json:"related_keys"`
	Tags            []string `json:"tags"`
	Importance      int      `json:"importance"`
}

// NewSelfKnowledgeService cria o serviço de autoconhecimento
func NewSelfKnowledgeService(db *database.DB) *SelfKnowledgeService {
	return &SelfKnowledgeService{db: db}
}

// SearchByQuery busca conhecimento por texto livre
func (s *SelfKnowledgeService) SearchByQuery(ctx context.Context, query string, limit int) ([]*KnowledgeEntry, error) {
	if limit <= 0 {
		limit = 5
	}

	// Buscar todos os registros de self_knowledge e filtrar localmente
	rows, err := s.db.QueryByLabel(ctx, "EvaSelfKnowledge", "", nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}

	searchLower := strings.ToLower(query)
	var entries []*KnowledgeEntry

	for _, row := range rows {
		title := database.GetString(row, "title")
		summary := database.GetString(row, "summary")
		detailedContent := database.GetString(row, "detailed_content")
		tagsStr := database.GetString(row, "tags")

		// Check if any field contains the search query (case-insensitive)
		if strings.Contains(strings.ToLower(title), searchLower) ||
			strings.Contains(strings.ToLower(summary), searchLower) ||
			strings.Contains(strings.ToLower(detailedContent), searchLower) ||
			strings.Contains(strings.ToLower(tagsStr), searchLower) {

			entry := s.rowToEntry(row)
			entries = append(entries, entry)

			if len(entries) >= limit {
				break
			}
		}
	}

	// Sort by importance (descending) - entries are already filtered
	sortEntriesByImportance(entries)

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// GetByKey busca conhecimento por chave específica
func (s *SelfKnowledgeService) GetByKey(ctx context.Context, key string) (*KnowledgeEntry, error) {
	rows, err := s.db.QueryByLabel(ctx, "EvaSelfKnowledge",
		" AND n.knowledge_key = $knowledge_key",
		map[string]interface{}{"knowledge_key": key}, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge by key: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	return s.rowToEntry(rows[0]), nil
}

// GetByType busca conhecimento por tipo
func (s *SelfKnowledgeService) GetByType(ctx context.Context, knowledgeType string) ([]*KnowledgeEntry, error) {
	rows, err := s.db.QueryByLabel(ctx, "EvaSelfKnowledge",
		" AND n.knowledge_type = $knowledge_type",
		map[string]interface{}{"knowledge_type": knowledgeType}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge by type: %w", err)
	}

	var entries []*KnowledgeEntry
	for _, row := range rows {
		entries = append(entries, s.rowToEntry(row))
	}

	sortEntriesByImportance(entries)
	return entries, nil
}

// GetChildren busca filhos de uma entrada
func (s *SelfKnowledgeService) GetChildren(ctx context.Context, parentKey string) ([]*KnowledgeEntry, error) {
	rows, err := s.db.QueryByLabel(ctx, "EvaSelfKnowledge",
		" AND n.parent_key = $parent_key",
		map[string]interface{}{"parent_key": parentKey}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}

	var entries []*KnowledgeEntry
	for _, row := range rows {
		entries = append(entries, s.rowToEntry(row))
	}

	sortEntriesByImportance(entries)
	return entries, nil
}

// GetArchitectureOverview retorna visão geral da arquitetura
func (s *SelfKnowledgeService) GetArchitectureOverview(ctx context.Context) (string, error) {
	entries, err := s.GetByType(ctx, "architecture")
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "Conhecimento de arquitetura não disponível.", nil
	}

	return entries[0].DetailedContent, nil
}

// GetModuleInfo retorna informações sobre um módulo específico
func (s *SelfKnowledgeService) GetModuleInfo(ctx context.Context, moduleName string) (*KnowledgeEntry, error) {
	key := "module:" + moduleName
	return s.GetByKey(ctx, key)
}

// GetConceptExplanation retorna explicação de um conceito
func (s *SelfKnowledgeService) GetConceptExplanation(ctx context.Context, conceptName string) (*KnowledgeEntry, error) {
	key := "concept:" + conceptName
	return s.GetByKey(ctx, key)
}

// GenerateContextForCreator gera contexto especial para o criador
func (s *SelfKnowledgeService) GenerateContextForCreator(ctx context.Context) (string, error) {
	var sb strings.Builder

	sb.WriteString("\n## 🏗️ MODO ARQUITETO ATIVADO\n\n")
	sb.WriteString("Pai, você tem acesso total ao conhecimento interno do sistema.\n\n")

	// Buscar módulos principais
	modules, err := s.GetByType(ctx, "module")
	if err == nil && len(modules) > 0 {
		sb.WriteString("### Módulos Disponíveis:\n")
		for _, m := range modules {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Title, m.Summary))
		}
		sb.WriteString("\n")
	}

	// Buscar conceitos
	concepts, err := s.GetByType(ctx, "concept")
	if err == nil && len(concepts) > 0 {
		sb.WriteString("### Conceitos Implementados:\n")
		for _, c := range concepts {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", c.Title, c.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Posso explicar qualquer parte do código em detalhes.\n")

	return sb.String(), nil
}

// GetPromptContext retorna conhecimento formatado para incluir no prompt
func (s *SelfKnowledgeService) GetPromptContext(ctx context.Context, query string) (string, error) {
	entries, err := s.SearchByQuery(ctx, query, 3)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n## Conhecimento Interno Relevante:\n\n")

	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("### %s\n", entry.Title))
		sb.WriteString(fmt.Sprintf("%s\n", entry.Summary))
		if entry.CodeLocation != "" {
			sb.WriteString(fmt.Sprintf("📁 Localização: `%s`\n", entry.CodeLocation))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// IsCreatorAsking verifica se a consulta é sobre arquitetura (para o criador)
func (s *SelfKnowledgeService) IsCreatorAsking(query string) bool {
	architectureKeywords := []string{
		"código", "code", "arquitetura", "architecture",
		"módulo", "module", "como funciona", "how it works",
		"implementação", "implementation", "banco", "database",
		"serviço", "service", "lacan", "fdpn", "memória", "memory",
		"cortex", "hippocampus", "brainstem", "motor", "senses",
	}

	lowerQuery := strings.ToLower(query)
	for _, keyword := range architectureKeywords {
		if strings.Contains(lowerQuery, keyword) {
			return true
		}
	}
	return false
}

// rowToEntry converts a NietzscheDB row to a KnowledgeEntry
func (s *SelfKnowledgeService) rowToEntry(row map[string]interface{}) *KnowledgeEntry {
	entry := &KnowledgeEntry{
		ID:              database.GetInt64(row, "pg_id"),
		KnowledgeType:   database.GetString(row, "knowledge_type"),
		KnowledgeKey:    database.GetString(row, "knowledge_key"),
		Title:           database.GetString(row, "title"),
		Summary:         database.GetString(row, "summary"),
		DetailedContent: database.GetString(row, "detailed_content"),
		CodeLocation:    database.GetString(row, "code_location"),
		ParentKey:       database.GetString(row, "parent_key"),
		Importance:      int(database.GetInt64(row, "importance")),
	}

	if entry.ID == 0 {
		entry.ID = database.GetInt64(row, "id")
	}

	// Parse JSON arrays for related_keys and tags
	relatedStr := database.GetString(row, "related_keys")
	if relatedStr != "" {
		json.Unmarshal([]byte(relatedStr), &entry.RelatedKeys)
	}

	tagsStr := database.GetString(row, "tags")
	if tagsStr != "" {
		json.Unmarshal([]byte(tagsStr), &entry.Tags)
	}

	return entry
}

// sortEntriesByImportance sorts entries by importance descending
func sortEntriesByImportance(entries []*KnowledgeEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Importance > entries[i].Importance {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// LogCreatorQuery registra consulta do criador sobre arquitetura
func (s *SelfKnowledgeService) LogCreatorQuery(ctx context.Context) error {
	// Update creator knowledge access in NietzscheDB
	cpf := creatorCPF()
	err := s.db.Update(ctx, "CreatorKnowledgeAccess",
		map[string]interface{}{"creator_cpf": cpf},
		map[string]interface{}{
			"last_architecture_query":    fmt.Sprintf("%v", strings.Replace(fmt.Sprintf("%v", ctx), " ", "", -1)),
			"total_architecture_queries": "increment", // Note: NietzscheDB doesn't support increment natively
		})
	if err != nil {
		// Fallback: try to get current value and update
		rows, qerr := s.db.QueryByLabel(ctx, "CreatorKnowledgeAccess",
			" AND n.creator_cpf = $cpf",
			map[string]interface{}{"cpf": cpf}, 1)
		if qerr == nil && len(rows) > 0 {
			currentCount := database.GetInt64(rows[0], "total_architecture_queries")
			err = s.db.Update(ctx, "CreatorKnowledgeAccess",
				map[string]interface{}{"creator_cpf": cpf},
				map[string]interface{}{
					"total_architecture_queries": currentCount + 1,
				})
		}
		if err != nil {
			log.Printf("⚠️ [SELF-KNOWLEDGE] Erro ao registrar consulta do criador: %v", err)
		}
	}
	return err
}
