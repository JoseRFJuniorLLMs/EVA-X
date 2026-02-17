package personality

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CreatorProfile representa o perfil especial do Criador
type CreatorProfile struct {
	// Identificação
	CPF       string
	IsCreator bool

	// Personalidade (de eva_personalidade_criador)
	Personality map[string]PersonalityTrait

	// Conhecimento do Projeto (de eva_conhecimento_projeto)
	ProjectKnowledge []ProjectItem

	// Memórias (de eva_memorias_criador)
	Memories []CreatorMemory

	// Enneagram
	EneagramType EneagramType
	EneagramWing int
}

// PersonalityTrait representa um aspecto da personalidade
type PersonalityTrait struct {
	Aspecto    string
	Valor      string
	Contexto   string
	Prioridade int
}

// ProjectItem representa conhecimento do projeto
type ProjectItem struct {
	Categoria   string
	Item        string
	Descricao   string
	Localizacao string
	Importancia int
}

// CreatorMemory representa uma memória do Criador
type CreatorMemory struct {
	Tipo        string
	Conteudo    string
	DataEvento  time.Time
	Importancia int
}

// CreatorCPF é o CPF do Criador
const CreatorCPF = "64525430249"

// CreatorProfileService gerencia o perfil do Criador
type CreatorProfileService struct {
	db *sql.DB
}

// NewCreatorProfileService cria um novo serviço
func NewCreatorProfileService(db *sql.DB) *CreatorProfileService {
	return &CreatorProfileService{db: db}
}

// IsCreator verifica se o CPF é do Criador
func (s *CreatorProfileService) IsCreator(cpf string) bool {
	// Remove pontuação do CPF
	cleanCPF := strings.ReplaceAll(cpf, ".", "")
	cleanCPF = strings.ReplaceAll(cleanCPF, "-", "")
	return cleanCPF == CreatorCPF
}

// LoadCreatorProfile carrega o perfil completo do Criador
func (s *CreatorProfileService) LoadCreatorProfile(ctx context.Context) (*CreatorProfile, error) {
	profile := &CreatorProfile{
		CPF:         CreatorCPF,
		IsCreator:   true,
		Personality: make(map[string]PersonalityTrait),
	}

	// 1. Carregar Personalidade
	if err := s.loadPersonality(ctx, profile); err != nil {
		return nil, fmt.Errorf("erro ao carregar personalidade: %w", err)
	}

	// 2. Carregar Conhecimento do Projeto
	if err := s.loadProjectKnowledge(ctx, profile); err != nil {
		return nil, fmt.Errorf("erro ao carregar conhecimento: %w", err)
	}

	// 3. Carregar Memórias
	if err := s.loadMemories(ctx, profile); err != nil {
		return nil, fmt.Errorf("erro ao carregar memórias: %w", err)
	}

	// 4. Extrair tipo Enneagram
	if trait, ok := profile.Personality["eneagrama_tipo"]; ok {
		var t int
		fmt.Sscanf(trait.Valor, "%d", &t)
		profile.EneagramType = EneagramType(t)
	} else {
		profile.EneagramType = Type9 // Default: Pacificador
	}

	if trait, ok := profile.Personality["eneagrama_asa"]; ok {
		fmt.Sscanf(trait.Valor, "%d", &profile.EneagramWing)
	}

	return profile, nil
}

// loadPersonality carrega os traits de personalidade do PostgreSQL
func (s *CreatorProfileService) loadPersonality(ctx context.Context, profile *CreatorProfile) error {
	query := `
		SELECT aspecto, valor, contexto, prioridade
		FROM eva_personalidade_criador
		WHERE ativo = true
		ORDER BY prioridade DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var trait PersonalityTrait
		var contexto sql.NullString

		if err := rows.Scan(&trait.Aspecto, &trait.Valor, &contexto, &trait.Prioridade); err != nil {
			continue
		}

		if contexto.Valid {
			trait.Contexto = contexto.String
		}

		profile.Personality[trait.Aspecto] = trait
	}

	return rows.Err()
}

// loadProjectKnowledge carrega conhecimento do projeto
func (s *CreatorProfileService) loadProjectKnowledge(ctx context.Context, profile *CreatorProfile) error {
	query := `
		SELECT categoria, item, descricao, COALESCE(localizacao, ''), importancia
		FROM eva_conhecimento_projeto
		ORDER BY importancia DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var item ProjectItem
		if err := rows.Scan(&item.Categoria, &item.Item, &item.Descricao, &item.Localizacao, &item.Importancia); err != nil {
			continue
		}
		profile.ProjectKnowledge = append(profile.ProjectKnowledge, item)
	}

	return rows.Err()
}

// loadMemories carrega memórias do Criador
func (s *CreatorProfileService) loadMemories(ctx context.Context, profile *CreatorProfile) error {
	query := `
		SELECT tipo, conteudo, data_evento, importancia
		FROM eva_memorias_criador
		ORDER BY importancia DESC, data_evento DESC
		LIMIT 50
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mem CreatorMemory
		if err := rows.Scan(&mem.Tipo, &mem.Conteudo, &mem.DataEvento, &mem.Importancia); err != nil {
			continue
		}
		profile.Memories = append(profile.Memories, mem)
	}

	return rows.Err()
}

// GenerateSystemPrompt gera o prompt de sistema para o Criador
// Apenas injeta dados dinamicos do banco — sem instrucoes hardcoded.
func (s *CreatorProfileService) GenerateSystemPrompt(profile *CreatorProfile) string {
	var sb strings.Builder

	// Traits do banco (eva_personalidade_criador)
	if len(profile.Personality) > 0 {
		sb.WriteString("[PERFIL]\n")
		for _, trait := range profile.Personality {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", trait.Aspecto, trait.Valor))
		}
	}

	// Conhecimento do Projeto (eva_conhecimento_projeto)
	if len(profile.ProjectKnowledge) > 0 {
		sb.WriteString("\n[CONHECIMENTO DO PROJETO]\n")
		categories := make(map[string][]string)
		for _, item := range profile.ProjectKnowledge {
			categories[item.Categoria] = append(categories[item.Categoria], item.Item)
		}
		for cat, items := range categories {
			if len(items) > 5 {
				items = items[:5]
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", cat, strings.Join(items, ", ")))
		}
	}

	// Memorias (eva_memorias_criador)
	if len(profile.Memories) > 0 {
		sb.WriteString("\n[MEMORIAS]\n")
		for i, mem := range profile.Memories {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Tipo, mem.Conteudo))
		}
	}

	return sb.String()
}

// SaveMemory salva uma nova memória do Criador
func (s *CreatorProfileService) SaveMemory(ctx context.Context, tipo, conteudo string, importancia int, tags []string) error {
	tagsJSON, _ := json.Marshal(tags)

	query := `
		INSERT INTO eva_memorias_criador (tipo, conteudo, importancia, tags)
		VALUES ($1, $2, $3, $4)
	`

	_, err := s.db.ExecContext(ctx, query, tipo, conteudo, importancia, tagsJSON)
	return err
}
