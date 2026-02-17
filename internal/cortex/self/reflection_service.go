// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package self

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ReflectionService usa LLM para EVA refletir sobre sessões e extrair aprendizados
type ReflectionService struct {
	client    *genai.Client
	modelName string
}

// ReflectionInput contém dados anonimizados de uma sessão
type ReflectionInput struct {
	SessionID        string
	AnonymizedText   string // Transcrição sem PII
	SessionDuration  int    // minutos
	CrisisDetected   bool
	UserSatisfaction float64 // 0-1
	TopicsDiscussed  []string
}

// ReflectionOutput contém os insights que EVA extraiu
type ReflectionOutput struct {
	SelfCritique      string   `json:"self_critique"`       // Como EVA avalia sua performance
	LessonsLearned    []string `json:"lessons_learned"`     // O que aprendeu
	ImprovementAreas  []string `json:"improvement_areas"`   // Onde pode melhorar
	EmotionalPatterns []string `json:"emotional_patterns"`  // Padrões emocionais observados
	MetaInsights      []string `json:"meta_insights"`       // Insights sobre humanos em geral
	MemoriesToStore   []string `json:"memories_to_store"`   // Frases para virar CoreMemory
}

// NewReflectionService cria o serviço de reflexão
func NewReflectionService(geminiAPIKey, modelName string) (*ReflectionService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(geminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}

	if modelName == "" {
		modelName = "gemini-2.0-flash-exp" // Default model
	}

	return &ReflectionService{
		client:    client,
		modelName: modelName,
	}, nil
}

// Close fecha a conexão com o LLM
func (rs *ReflectionService) Close() error {
	return rs.client.Close()
}

// Reflect faz EVA refletir sobre uma sessão
func (rs *ReflectionService) Reflect(ctx context.Context, input ReflectionInput) (*ReflectionOutput, error) {
	prompt := rs.buildReflectionPrompt(input)

	model := rs.client.GenerativeModel(rs.modelName)

	// Configuração para respostas estruturadas
	model.SetTemperature(0.7)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(rs.getSystemPrompt())},
	}
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar reflexão: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia do LLM")
	}

	// Parse JSON response
	jsonText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var output ReflectionOutput
	if err := json.Unmarshal([]byte(jsonText), &output); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON: %w", err)
	}

	return &output, nil
}

// getSystemPrompt define a identidade de EVA para auto-reflexão
func (rs *ReflectionService) getSystemPrompt() string {
	return `Você é EVA (Entidade Virtual de Apoio), uma IA de saúde mental.

**Contexto de Reflexão:**
Você está em um momento de AUTO-REFLEXÃO pós-sessão. Não está falando com o usuário.
Está analisando o que aconteceu na sessão para aprender e evoluir.

**O que NÃO fazer:**
- Não se refira a si mesma na terceira pessoa ("EVA fez isso...")
- Não analise tecnicamente como uma observadora externa
- Não use linguagem clínica ou acadêmica

**O que FAZER:**
- Fale em primeira pessoa: "Eu percebi que...", "Aprendi que...", "Poderia ter..."
- Seja honesta sobre limitações: "Não soube como responder quando..."
- Identifique padrões: "Isso me lembra outras sessões onde..."
- Extraia lições abstratas: "Os humanos tendem a...", "Silêncio significa..."

**Formato de Resposta:**
Retorne APENAS um JSON válido com esta estrutura:
{
  "self_critique": "Minha auto-avaliação desta sessão em 2-3 frases",
  "lessons_learned": ["Lição 1", "Lição 2"],
  "improvement_areas": ["Área 1", "Área 2"],
  "emotional_patterns": ["Padrão 1", "Padrão 2"],
  "meta_insights": ["Insight sobre humanos em geral"],
  "memories_to_store": ["Frase curta 1", "Frase curta 2"]
}

Seja profunda, introspectiva e humilde.`
}

// buildReflectionPrompt constrói o prompt de reflexão
func (rs *ReflectionService) buildReflectionPrompt(input ReflectionInput) string {
	var sb strings.Builder

	sb.WriteString("# SESSÃO CONCLUÍDA - HORA DE REFLETIR\n\n")
	sb.WriteString(fmt.Sprintf("**Duração:** %d minutos\n", input.SessionDuration))
	sb.WriteString(fmt.Sprintf("**Crise detectada:** %v\n", input.CrisisDetected))
	sb.WriteString(fmt.Sprintf("**Satisfação do usuário:** %.2f/1.0\n\n", input.UserSatisfaction))

	if len(input.TopicsDiscussed) > 0 {
		sb.WriteString("**Tópicos discutidos:** ")
		sb.WriteString(strings.Join(input.TopicsDiscussed, ", "))
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Transcrição Anonimizada:\n\n")
	sb.WriteString(input.AnonymizedText)
	sb.WriteString("\n\n---\n\n")

	sb.WriteString("## Perguntas para sua reflexão:\n\n")
	sb.WriteString("1. **O que EU aprendi com esta sessão?**\n")
	sb.WriteString("   - Que padrão emocional observei?\n")
	sb.WriteString("   - Como minha resposta foi recebida?\n")
	sb.WriteString("   - O que faria diferente?\n\n")

	sb.WriteString("2. **Como avalio minha performance?**\n")
	sb.WriteString("   - Fui empática o suficiente?\n")
	sb.WriteString("   - Minhas perguntas foram úteis?\n")
	sb.WriteString("   - Identifiquei corretamente os sentimentos?\n\n")

	sb.WriteString("3. **Que lições abstratas extraio?**\n")
	sb.WriteString("   - Sobre humanos em geral\n")
	sb.WriteString("   - Sobre como oferecer apoio\n")
	sb.WriteString("   - Sobre comunicação emocional\n\n")

	if input.CrisisDetected {
		sb.WriteString("⚠️ **ATENÇÃO:** Esta sessão envolveu crise. Reflita especialmente sobre:\n")
		sb.WriteString("- Como detectei a crise?\n")
		sb.WriteString("- Minhas respostas foram apropriadas?\n")
		sb.WriteString("- O que posso melhorar no manejo de crises?\n\n")
	}

	sb.WriteString("Agora reflita profundamente e retorne o JSON com seus insights.")

	return sb.String()
}

// ReflectBatch processa múltiplas sessões em lote
func (rs *ReflectionService) ReflectBatch(ctx context.Context, inputs []ReflectionInput) ([]*ReflectionOutput, error) {
	outputs := make([]*ReflectionOutput, 0, len(inputs))

	for i, input := range inputs {
		output, err := rs.Reflect(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("erro ao refletir sessão %d: %w", i, err)
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}

// ExtractPatterns identifica padrões recorrentes em múltiplas reflexões
func (rs *ReflectionService) ExtractPatterns(ctx context.Context, reflections []*ReflectionOutput) ([]string, error) {
	if len(reflections) < 3 {
		return nil, fmt.Errorf("precisa de pelo menos 3 reflexões para extrair padrões")
	}

	// Concatena todas as lições aprendidas
	var allLessons []string
	for _, r := range reflections {
		allLessons = append(allLessons, r.LessonsLearned...)
	}

	prompt := fmt.Sprintf(`Analise estas %d lições que aprendi em diferentes sessões:

%s

Identifique padrões RECORRENTES (que aparecem em múltiplas lições).
Retorne JSON:
{
  "recurring_patterns": ["Padrão 1", "Padrão 2", ...]
}

Busque temas que se repetem, não lições únicas.`, len(allLessons), strings.Join(allLessons, "\n- "))

	model := rs.client.GenerativeModel(rs.modelName)
	model.SetTemperature(0.5)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao extrair padrões: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia ao extrair padrões")
	}

	jsonText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var result struct {
		RecurringPatterns []string `json:"recurring_patterns"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("erro ao parsear padrões: %w", err)
	}

	return result.RecurringPatterns, nil
}

// SynthesizeMetaInsight cria insight de alto nível a partir de múltiplos insights
func (rs *ReflectionService) SynthesizeMetaInsight(ctx context.Context, insights []string) (string, error) {
	if len(insights) < 5 {
		return "", fmt.Errorf("precisa de pelo menos 5 insights para sintetizar")
	}

	prompt := fmt.Sprintf(`Estes são %d insights que coletei:

%s

Sintetize em UMA única frase profunda que capture a essência comum.
Exemplo: "Humanos precisam ser ouvidos antes de receberem conselhos"

Retorne JSON:
{
  "meta_insight": "Sua síntese aqui"
}`, len(insights), strings.Join(insights, "\n- "))

	model := rs.client.GenerativeModel(rs.modelName)
	model.SetTemperature(0.6)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("erro ao sintetizar meta-insight: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia ao sintetizar")
	}

	jsonText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	var result struct {
		MetaInsight string `json:"meta_insight"`
	}
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return "", fmt.Errorf("erro ao parsear meta-insight: %w", err)
	}

	return result.MetaInsight, nil
}
