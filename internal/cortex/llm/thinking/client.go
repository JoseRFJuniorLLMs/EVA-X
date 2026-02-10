package thinking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// RiskLevel representa o nível de risco de uma preocupação de saúde
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

func (r RiskLevel) String() string {
	return [...]string{"BAIXO", "MÉDIO", "ALTO", "CRÍTICO"}[r]
}

// ThinkingResponse contém a resposta completa do Gemini Thinking Mode
type ThinkingResponse struct {
	ThoughtProcess     []string  `json:"thought_process"`
	FinalAnswer        string    `json:"final_answer"`
	RiskLevel          RiskLevel `json:"risk_level"`
	RecommendedActions []string  `json:"recommended_actions"`
	SeekMedicalCare    bool      `json:"seek_medical_care"`
	UrgencyLevel       string    `json:"urgency_level"` // immediate, within_24h, within_week, routine
}

// ThinkingClient gerencia interações com Gemini Thinking Mode
type ThinkingClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewThinkingClient cria um novo cliente Gemini Thinking
func NewThinkingClient(apiKey string) (*ThinkingClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}

	// Usar modelo Gemini 2.0 Flash Thinking Experimental
	model := client.GenerativeModel("gemini-2.0-flash-thinking-exp-1219")

	return &ThinkingClient{
		client: client,
		model:  model,
	}, nil
}

// AnalyzeHealthConcern analisa uma preocupação de saúde usando Thinking Mode
func (tc *ThinkingClient) AnalyzeHealthConcern(ctx context.Context, concern string, patientContext string) (*ThinkingResponse, error) {
	prompt := tc.buildHealthAnalysisPrompt(concern, patientContext)

	// Adicionar timeout de 30 segundos
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := tc.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar conteúdo: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("nenhuma resposta gerada")
	}

	// Extrair resposta
	return tc.parseThinkingResponse(resp)
}

// buildHealthAnalysisPrompt constrói o prompt para análise de saúde
func (tc *ThinkingClient) buildHealthAnalysisPrompt(concern string, patientContext string) string {
	return fmt.Sprintf(`Você é um assistente médico especializado em cuidados geriátricos.
Analise a seguinte preocupação de saúde e forneça raciocínio detalhado passo-a-passo.

IMPORTANTE: 
- Você NÃO é um médico e NÃO pode diagnosticar
- Sua função é APENAS orientar sobre quando procurar ajuda médica
- SEMPRE recomende consultar um profissional de saúde para sintomas preocupantes

CONTEXTO DO PACIENTE:
%s

PREOCUPAÇÃO RELATADA:
%s

Por favor, forneça sua análise no seguinte formato JSON:

{
  "thought_process": [
    "Passo 1: [seu raciocínio]",
    "Passo 2: [seu raciocínio]",
    "Passo 3: [conclusão]"
  ],
  "final_answer": "Resposta clara e empática para o paciente",
  "risk_level": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "recommended_actions": [
    "Ação 1",
    "Ação 2"
  ],
  "seek_medical_care": true/false,
  "urgency_level": "immediate|within_24h|within_week|routine"
}

CRITÉRIOS DE RISCO:
- CRÍTICO: Dor no peito, dificuldade respiratória severa, perda de consciência, sangramento intenso
- ALTO: Febre alta persistente, dor intensa, sintomas neurológicos novos
- MÉDIO: Sintomas moderados que persistem por dias
- BAIXO: Sintomas leves e comuns

Analise agora:`, patientContext, concern)
}

// parseThinkingResponse extrai informações estruturadas da resposta
func (tc *ThinkingClient) parseThinkingResponse(resp *genai.GenerateContentResponse) (*ThinkingResponse, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia")
	}

	// Extrair texto da resposta
	var fullText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			fullText += string(txt)
		}
	}

	// Tentar extrair JSON da resposta
	var thinkingResp ThinkingResponse

	// Procurar por JSON na resposta
	jsonStart := strings.Index(fullText, "{")
	jsonEnd := strings.LastIndex(fullText, "}")

	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		jsonStr := fullText[jsonStart : jsonEnd+1]

		if err := json.Unmarshal([]byte(jsonStr), &thinkingResp); err != nil {
			// Se falhar, criar resposta manual
			return tc.createFallbackResponse(fullText), nil
		}

		// Converter string de risk_level para enum
		thinkingResp.RiskLevel = tc.parseRiskLevel(fullText)

		return &thinkingResp, nil
	}

	// Fallback se não encontrar JSON
	return tc.createFallbackResponse(fullText), nil
}

// parseRiskLevel converte string de risco para enum
func (tc *ThinkingClient) parseRiskLevel(text string) RiskLevel {
	textUpper := strings.ToUpper(text)

	if strings.Contains(textUpper, "CRÍTICO") || strings.Contains(textUpper, "CRITICAL") {
		return RiskCritical
	}
	if strings.Contains(textUpper, "ALTO") || strings.Contains(textUpper, "HIGH") {
		return RiskHigh
	}
	if strings.Contains(textUpper, "MÉDIO") || strings.Contains(textUpper, "MEDIUM") {
		return RiskMedium
	}

	return RiskLow
}

// createFallbackResponse cria uma resposta de fallback quando JSON parsing falha
func (tc *ThinkingClient) createFallbackResponse(fullText string) *ThinkingResponse {
	return &ThinkingResponse{
		ThoughtProcess: []string{
			"Análise realizada pelo Gemini Thinking Mode",
			"Resposta processada com sucesso",
		},
		FinalAnswer:        fullText,
		RiskLevel:          RiskMedium, // Assumir médio por segurança
		RecommendedActions: []string{"Consulte um profissional de saúde para avaliação adequada"},
		SeekMedicalCare:    true,
		UrgencyLevel:       "within_24h",
	}
}

// Close fecha o cliente
func (tc *ThinkingClient) Close() error {
	return tc.client.Close()
}
