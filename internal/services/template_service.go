// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"eva/internal/brainstem/database"
	"eva/internal/brainstem/logger"

	"github.com/rs/zerolog"
)

// TemplateService gerencia templates de prompts
type TemplateService struct {
	db  *database.DB
	log zerolog.Logger
}

// NewTemplateService cria nova instância
func NewTemplateService(db *database.DB) *TemplateService {
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
	ctx := context.Background()

	// Buscar dados do idoso
	m, err := s.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar dados do idoso: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("idoso nao encontrado: %d", idosoID)
	}

	data := &PromptData{
		NomeIdoso:           database.GetString(m, "nome"),
		NivelCognitivo:      database.GetString(m, "nivel_cognitivo"),
		TomVoz:              database.GetString(m, "tom_voz"),
		LimitacoesAuditivas: database.GetBool(m, "limitacoes_auditivas"),
		UsaAparelhoAuditivo: database.GetBool(m, "usa_aparelho_auditivo"),
	}

	// Calcular idade a partir de data_nascimento
	dataNasc := database.GetTime(m, "data_nascimento")
	if !dataNasc.IsZero() {
		data.Idade = int(time.Since(dataNasc).Hours() / 24 / 365.25)
	}

	// Verificar se é primeira interação (sem histórico de ligações)
	callCount, _ := s.db.Count(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID})
	data.PrimeiraInteracao = callCount == 0

	// Calcular taxa de adesão à medicação
	agendamentos, _ := s.db.QueryByLabel(ctx, "agendamentos",
		" AND n.idoso_id = $idoso AND n.tipo = $tipo",
		map[string]interface{}{"idoso": idosoID, "tipo": "lembrete_medicamento"}, 0)

	if len(agendamentos) > 0 {
		tomados := 0
		for _, a := range agendamentos {
			if database.GetBool(a, "medicamento_tomado") {
				tomados++
			}
		}
		data.TaxaAdesao = float64(tomados) / float64(len(agendamentos)) * 100
	}

	// Buscar última interação
	ligacoes, _ := s.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)

	if len(ligacoes) > 0 {
		// Find most recent by inicio_chamada
		var latest map[string]interface{}
		var latestTime time.Time
		for _, l := range ligacoes {
			t := database.GetTime(l, "inicio_chamada")
			if t.After(latestTime) {
				latestTime = t
				latest = l
			}
		}
		if latest != nil {
			data.UltimaInteracao = database.GetString(latest, "transcricao_resumo")
		}
	}

	// Buscar preocupações recentes
	s.getRecentConcerns(ctx, idosoID, data)

	return data, nil
}

// getRecentConcerns busca preocupações recentes
func (s *TemplateService) getRecentConcerns(ctx context.Context, idosoID int64, data *PromptData) {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "alertas",
		" AND n.idoso_id = $idoso AND n.criado_em > $since",
		map[string]interface{}{"idoso": idosoID, "since": sevenDaysAgo}, 0)
	if err != nil {
		return
	}

	for _, m := range rows {
		sev := database.GetString(m, "severidade")
		if sev == "critica" || sev == "alta" {
			msg := database.GetString(m, "mensagem")
			if msg != "" {
				data.PreocupacoesRecentes = append(data.PreocupacoesRecentes, msg)
			}
		}
	}

	// Limit to 3 most recent (already sorted by NQL but let's be safe)
	if len(data.PreocupacoesRecentes) > 3 {
		data.PreocupacoesRecentes = data.PreocupacoesRecentes[:3]
	}
}

// getTemplate busca template do banco
func (s *TemplateService) getTemplate(nome string) (string, error) {
	ctx := context.Background()
	rows, err := s.db.QueryByLabel(ctx, "prompt_templates",
		" AND n.nome = $nome AND n.ativo = $ativo",
		map[string]interface{}{"nome": nome, "ativo": true}, 1)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("template nao encontrado: %s", nome)
	}

	return database.GetString(rows[0], "template"), nil
}

// getFallbackTemplate retorna template básico em caso de erro
func (s *TemplateService) getFallbackTemplate(data *PromptData) string {
	return fmt.Sprintf(`Voce e a EVA, assistente de saude virtual.

O idoso se chama %s, %d anos.
Nivel cognitivo: %s
Tom de voz: %s

{{if .LimitacoesAuditivas}}
IMPORTANTE: O idoso tem limitacoes auditivas. Fale DEVAGAR, CLARA e pausadamente.
{{if .UsaAparelhoAuditivo}}
Ele usa aparelho auditivo, entao pergunte se esta conseguindo ouvir bem.
{{end}}
{{end}}

Seja calorosa, paciente e empatica.
Respostas CURTAS: 1-2 frases apenas.
Fale em portugues brasileiro natural.

{{if .PrimeiraInteracao}}
Esta e a primeira interacao. Apresente-se e pergunte como ele esta se sentindo.
{{else}}
{{if .UltimaInteracao}}
Ultima conversa: {{.UltimaInteracao}}
{{end}}
{{if .PreocupacoesRecentes}}
Preocupacoes recentes:
{{range .PreocupacoesRecentes}}
- {{.}}
{{end}}
{{end}}
{{end}}

Taxa de adesao a medicacao: {{printf "%%.0f" .TaxaAdesao}}%%
`, data.NomeIdoso, data.Idade, data.NivelCognitivo, data.TomVoz)
}

// SaveTemplate salva novo template no banco
func (s *TemplateService) SaveTemplate(nome, templateText string, variaveis []string) error {
	ctx := context.Background()
	variaveisJSON, _ := json.Marshal(variaveis)
	now := time.Now().UTC().Format(time.RFC3339)

	// Try upsert: find existing
	existing, _ := s.db.QueryByLabel(ctx, "prompt_templates",
		" AND n.nome = $nome",
		map[string]interface{}{"nome": nome}, 1)

	if len(existing) > 0 {
		err := s.db.Update(ctx, "prompt_templates",
			map[string]interface{}{"nome": nome},
			map[string]interface{}{
				"template":             templateText,
				"variaveis_esperadas":  string(variaveisJSON),
				"atualizado_em":        now,
			})
		if err != nil {
			s.log.Error().Err(err).Msg("Erro ao salvar template")
			return err
		}
	} else {
		_, err := s.db.Insert(ctx, "prompt_templates", map[string]interface{}{
			"nome":                nome,
			"template":            templateText,
			"variaveis_esperadas": string(variaveisJSON),
			"ativo":               true,
			"criado_em":           now,
			"atualizado_em":       now,
		})
		if err != nil {
			s.log.Error().Err(err).Msg("Erro ao salvar template")
			return err
		}
	}

	s.log.Info().Str("nome", nome).Msg("Template salvo com sucesso")
	return nil
}

// ValidateTemplate valida sintaxe do template
func (s *TemplateService) ValidateTemplate(templateText string) error {
	_, err := template.New("test").Parse(templateText)
	return err
}
