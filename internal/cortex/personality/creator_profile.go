package personality

import (
	"context"
	"encoding/json"
	"eva/internal/brainstem/database"
	"fmt"
	"sort"
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
	db *database.DB
}

// NewCreatorProfileService cria um novo serviço
func NewCreatorProfileService(db *database.DB) *CreatorProfileService {
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

// loadPersonality carrega os traits de personalidade do NietzscheDB
func (s *CreatorProfileService) loadPersonality(ctx context.Context, profile *CreatorProfile) error {
	rows, err := s.db.QueryByLabel(ctx, "eva_personalidade_criador", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return err
	}

	// Sort by prioridade DESC in Go
	sort.Slice(rows, func(i, j int) bool {
		return database.GetInt64(rows[i], "prioridade") > database.GetInt64(rows[j], "prioridade")
	})

	for _, row := range rows {
		trait := PersonalityTrait{
			Aspecto:    database.GetString(row, "aspecto"),
			Valor:      database.GetString(row, "valor"),
			Contexto:   database.GetString(row, "contexto"),
			Prioridade: int(database.GetInt64(row, "prioridade")),
		}
		profile.Personality[trait.Aspecto] = trait
	}

	return nil
}

// loadProjectKnowledge carrega conhecimento do projeto
func (s *CreatorProfileService) loadProjectKnowledge(ctx context.Context, profile *CreatorProfile) error {
	rows, err := s.db.QueryByLabel(ctx, "eva_conhecimento_projeto", "", nil, 0)
	if err != nil {
		return err
	}

	// Sort by importancia DESC in Go
	sort.Slice(rows, func(i, j int) bool {
		return database.GetInt64(rows[i], "importancia") > database.GetInt64(rows[j], "importancia")
	})

	for _, row := range rows {
		item := ProjectItem{
			Categoria:   database.GetString(row, "categoria"),
			Item:        database.GetString(row, "item"),
			Descricao:   database.GetString(row, "descricao"),
			Localizacao: database.GetString(row, "localizacao"),
			Importancia: int(database.GetInt64(row, "importancia")),
		}
		profile.ProjectKnowledge = append(profile.ProjectKnowledge, item)
	}

	return nil
}

// loadMemories carrega memórias do Criador
func (s *CreatorProfileService) loadMemories(ctx context.Context, profile *CreatorProfile) error {
	rows, err := s.db.QueryByLabel(ctx, "eva_memorias_criador", "", nil, 0)
	if err != nil {
		return err
	}

	// Sort by importancia DESC, data_evento DESC in Go
	sort.Slice(rows, func(i, j int) bool {
		impI := database.GetInt64(rows[i], "importancia")
		impJ := database.GetInt64(rows[j], "importancia")
		if impI != impJ {
			return impI > impJ
		}
		return database.GetTime(rows[i], "data_evento").After(database.GetTime(rows[j], "data_evento"))
	})

	// Limit to 50
	if len(rows) > 50 {
		rows = rows[:50]
	}

	for _, row := range rows {
		mem := CreatorMemory{
			Tipo:        database.GetString(row, "tipo"),
			Conteudo:    database.GetString(row, "conteudo"),
			DataEvento:  database.GetTime(row, "data_evento"),
			Importancia: int(database.GetInt64(row, "importancia")),
		}
		profile.Memories = append(profile.Memories, mem)
	}

	return nil
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

	_, err := s.db.Insert(ctx, "eva_memorias_criador", map[string]interface{}{
		"tipo":        tipo,
		"conteudo":    conteudo,
		"importancia": importancia,
		"tags":        string(tagsJSON),
		"created_at":  time.Now().Format(time.RFC3339),
	})
	return err
}
