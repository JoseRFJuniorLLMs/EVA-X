package services

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"text/template"

	"eva-mind/internal/brainstem/logger"

	"github.com/rs/zerolog"
)

// TemplateService gerencia templates de prompts
type TemplateService struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewTemplateService cria nova instância
func NewTemplateService(db *sql.DB) *TemplateService {
	return &TemplateService{
		db:  db,
		log: logger.Logger.With().Str("service", "template").Logger(),
	}
}

// PromptData dados para popular o template
type PromptData struct {
	NomeIdoso            string
	Idade                int
	NivelCognitivo       string
	TomVoz               string
	LimitacoesAuditivas  bool
	UsaAparelhoAuditivo  bool
	PrimeiraInteracao    bool
	TaxaAdesao           float64
	UltimaInteracao      string
	PreocupacoesRecentes []string
}

// BuildInstructions constrói instruções do prompt usando template
func (s *TemplateService) BuildInstructions(idosoID int64) (string, error) {
	// 1. Buscar dados do idoso
	data, err := s.getPromptData(idosoID)
	if err != nil {
		s.log.Error().Err(err).Int64("idoso_id", idosoID).Msg("Erro ao buscar dados")
		return "", err
	}

	// 2. Buscar template do banco
	templateText, err := s.getTemplate("eva_base_v2")
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao buscar template")
		return s.getFallbackTemplate(data), nil
	}

	// 3. Processar template
	tmpl, err := template.New("prompt").Parse(templateText)
	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao parsear template")
		return s.getFallbackTemplate(data), nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		s.log.Error().Err(err).Msg("Erro ao executar template")
		return s.getFallbackTemplate(data), nil
	}

	result := buf.String()
	s.log.Debug().Int("length", len(result)).Msg("Template processado com sucesso")

	return result, nil
}

// getPromptData busca dados necessários para o template
func (s *TemplateService) getPromptData(idosoID int64) (*PromptData, error) {
	query := `
		SELECT 
			i.nome, 
			EXTRACT(YEAR FROM AGE(i.data_nascimento)) as idade,
			i.nivel_cognitivo, 
			i.tom_voz,
			i.limitacoes_auditivas, 
			i.usa_aparelho_auditivo,
			COALESCE(
				(SELECT COUNT(*) = 0 FROM historico_ligacoes WHERE idoso_id = i.id),
				true
			) as primeira_interacao,
			COALESCE(
				(SELECT 
					COUNT(CASE WHEN medicamento_tomado THEN 1 END)::float / 
					NULLIF(COUNT(*)::float, 0) * 100
				FROM agendamentos 
				WHERE idoso_id = i.id AND tipo = 'lembrete_medicamento'),
				0
			) as taxa_adesao
		FROM idosos i
		WHERE i.id = $1
	`

	var data PromptData
	err := s.db.QueryRow(query, idosoID).Scan(
		&data.NomeIdoso,
		&data.Idade,
		&data.NivelCognitivo,
		&data.TomVoz,
		&data.LimitacoesAuditivas,
		&data.UsaAparelhoAuditivo,
		&data.PrimeiraInteracao,
		&data.TaxaAdesao,
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao buscar dados do idoso: %w", err)
	}

	// Buscar última interação
	var ultimaInteracao sql.NullString
	s.db.QueryRow(`
		SELECT transcricao_resumo 
		FROM historico_ligacoes 
		WHERE idoso_id = $1 
		ORDER BY inicio_chamada DESC 
		LIMIT 1
	`, idosoID).Scan(&ultimaInteracao)

	if ultimaInteracao.Valid {
		data.UltimaInteracao = ultimaInteracao.String
	}

	// Buscar preocupações recentes
	s.getRecentConcerns(idosoID, &data)

	return &data, nil
}

// getRecentConcerns busca preocupações recentes
func (s *TemplateService) getRecentConcerns(idosoID int64, data *PromptData) {
	query := `
		SELECT mensagem
		FROM alertas
		WHERE idoso_id = $1
		  AND severidade IN ('critica', 'alta')
		  AND criado_em > NOW() - INTERVAL '7 days'
		ORDER BY criado_em DESC
		LIMIT 3
	`

	rows, err := s.db.Query(query, idosoID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var concern string
		if err := rows.Scan(&concern); err != nil {
			continue
		}
		data.PreocupacoesRecentes = append(data.PreocupacoesRecentes, concern)
	}
}

// getTemplate busca template do banco
func (s *TemplateService) getTemplate(nome string) (string, error) {
	query := `
		SELECT template
		FROM prompt_templates
		WHERE nome = $1 AND ativo = true
		LIMIT 1
	`

	var template string
	err := s.db.QueryRow(query, nome).Scan(&template)
	if err != nil {
		return "", err
	}

	return template, nil
}

// getFallbackTemplate retorna template básico em caso de erro
func (s *TemplateService) getFallbackTemplate(data *PromptData) string {
	return fmt.Sprintf(`Você é a EVA, assistente de saúde virtual.

O idoso se chama %s, %d anos.
Nível cognitivo: %s
Tom de voz: %s

{{if .LimitacoesAuditivas}}
IMPORTANTE: O idoso tem limitações auditivas. Fale DEVAGAR, CLARA e pausadamente.
{{if .UsaAparelhoAuditivo}}
Ele usa aparelho auditivo, então pergunte se está conseguindo ouvir bem.
{{end}}
{{end}}

Seja calorosa, paciente e empática.
Respostas CURTAS: 1-2 frases apenas.
Fale em português brasileiro natural.

{{if .PrimeiraInteracao}}
Esta é a primeira interação. Apresente-se e pergunte como ele está se sentindo.
{{else}}
{{if .UltimaInteracao}}
Última conversa: {{.UltimaInteracao}}
{{end}}
{{if .PreocupacoesRecentes}}
Preocupações recentes:
{{range .PreocupacoesRecentes}}
- {{.}}
{{end}}
{{end}}
{{end}}

Taxa de adesão à medicação: {{printf "%%.0f" .TaxaAdesao}}%%
`, data.NomeIdoso, data.Idade, data.NivelCognitivo, data.TomVoz)
}

// SaveTemplate salva novo template no banco
func (s *TemplateService) SaveTemplate(nome, templateText string, variaveis []string) error {
	query := `
		INSERT INTO prompt_templates (nome, template, variaveis_esperadas, ativo)
		VALUES ($1, $2, $3, true)
		ON CONFLICT (nome) 
		DO UPDATE SET 
			template = EXCLUDED.template,
			variaveis_esperadas = EXCLUDED.variaveis_esperadas,
			atualizado_em = NOW()
	`

	variaveisJSON, _ := json.Marshal(variaveis)
	_, err := s.db.Exec(query, nome, templateText, string(variaveisJSON))

	if err != nil {
		s.log.Error().Err(err).Msg("Erro ao salvar template")
		return err
	}

	s.log.Info().Str("nome", nome).Msg("Template salvo com sucesso")
	return nil
}

// ValidateTemplate valida sintaxe do template
func (s *TemplateService) ValidateTemplate(templateText string) error {
	_, err := template.New("test").Parse(templateText)
	return err
}
