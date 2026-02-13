package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// SelfKnowledgeService permite a EVA consultar conhecimento sobre si mesma
type SelfKnowledgeService struct {
	db *sql.DB
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
func NewSelfKnowledgeService(db *sql.DB) *SelfKnowledgeService {
	return &SelfKnowledgeService{db: db}
}

// SearchByQuery busca conhecimento por texto livre
func (s *SelfKnowledgeService) SearchByQuery(ctx context.Context, query string, limit int) ([]*KnowledgeEntry, error) {
	if limit <= 0 {
		limit = 5
	}

	// Busca por título, summary ou tags
	sqlQuery := `
		SELECT
			id, knowledge_type, knowledge_key, title, summary,
			detailed_content, COALESCE(code_location, ''),
			COALESCE(parent_key, ''), COALESCE(related_keys, '[]'),
			COALESCE(tags, '[]'), importance
		FROM eva_self_knowledge
		WHERE
			title ILIKE $1 OR
			summary ILIKE $1 OR
			detailed_content ILIKE $1 OR
			tags::text ILIKE $1
		ORDER BY importance DESC
		LIMIT $2
	`

	searchPattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, sqlQuery, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// GetByKey busca conhecimento por chave específica
func (s *SelfKnowledgeService) GetByKey(ctx context.Context, key string) (*KnowledgeEntry, error) {
	query := `
		SELECT
			id, knowledge_type, knowledge_key, title, summary,
			detailed_content, COALESCE(code_location, ''),
			COALESCE(parent_key, ''), COALESCE(related_keys, '[]'),
			COALESCE(tags, '[]'), importance
		FROM eva_self_knowledge
		WHERE knowledge_key = $1
	`

	row := s.db.QueryRowContext(ctx, query, key)
	return s.scanEntry(row)
}

// GetByType busca conhecimento por tipo
func (s *SelfKnowledgeService) GetByType(ctx context.Context, knowledgeType string) ([]*KnowledgeEntry, error) {
	query := `
		SELECT
			id, knowledge_type, knowledge_key, title, summary,
			detailed_content, COALESCE(code_location, ''),
			COALESCE(parent_key, ''), COALESCE(related_keys, '[]'),
			COALESCE(tags, '[]'), importance
		FROM eva_self_knowledge
		WHERE knowledge_type = $1
		ORDER BY importance DESC
	`

	rows, err := s.db.QueryContext(ctx, query, knowledgeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge by type: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
}

// GetChildren busca filhos de uma entrada
func (s *SelfKnowledgeService) GetChildren(ctx context.Context, parentKey string) ([]*KnowledgeEntry, error) {
	query := `
		SELECT
			id, knowledge_type, knowledge_key, title, summary,
			detailed_content, COALESCE(code_location, ''),
			COALESCE(parent_key, ''), COALESCE(related_keys, '[]'),
			COALESCE(tags, '[]'), importance
		FROM eva_self_knowledge
		WHERE parent_key = $1
		ORDER BY importance DESC
	`

	rows, err := s.db.QueryContext(ctx, query, parentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	return s.scanEntries(rows)
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

// Helper: scan múltiplas entradas
func (s *SelfKnowledgeService) scanEntries(rows *sql.Rows) ([]*KnowledgeEntry, error) {
	var entries []*KnowledgeEntry

	for rows.Next() {
		entry := &KnowledgeEntry{}
		var relatedJSON, tagsJSON []byte

		err := rows.Scan(
			&entry.ID, &entry.KnowledgeType, &entry.KnowledgeKey,
			&entry.Title, &entry.Summary, &entry.DetailedContent,
			&entry.CodeLocation, &entry.ParentKey, &relatedJSON,
			&tagsJSON, &entry.Importance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		json.Unmarshal(relatedJSON, &entry.RelatedKeys)
		json.Unmarshal(tagsJSON, &entry.Tags)

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// Helper: scan uma entrada
func (s *SelfKnowledgeService) scanEntry(row *sql.Row) (*KnowledgeEntry, error) {
	entry := &KnowledgeEntry{}
	var relatedJSON, tagsJSON []byte

	err := row.Scan(
		&entry.ID, &entry.KnowledgeType, &entry.KnowledgeKey,
		&entry.Title, &entry.Summary, &entry.DetailedContent,
		&entry.CodeLocation, &entry.ParentKey, &relatedJSON,
		&tagsJSON, &entry.Importance,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan entry: %w", err)
	}

	json.Unmarshal(relatedJSON, &entry.RelatedKeys)
	json.Unmarshal(tagsJSON, &entry.Tags)

	return entry, nil
}

// LogCreatorQuery registra consulta do criador sobre arquitetura
func (s *SelfKnowledgeService) LogCreatorQuery(ctx context.Context) error {
	query := `
		UPDATE creator_knowledge_access
		SET last_architecture_query = NOW(),
		    total_architecture_queries = total_architecture_queries + 1
		WHERE creator_cpf = '64525430249'
	`
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		log.Printf("⚠️ [SELF-KNOWLEDGE] Erro ao registrar consulta do criador: %v", err)
	}
	return err
}
