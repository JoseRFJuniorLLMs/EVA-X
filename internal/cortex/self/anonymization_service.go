package self

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// AnonymizationService remove informações pessoais identificáveis de transcrições
type AnonymizationService struct {
	client       *genai.Client
	modelName    string
	regexFilters []*regexp.Regexp
}

// AnonymizationConfig configurações do serviço
type AnonymizationConfig struct {
	GeminiAPIKey      string
	ModelName         string
	UseRegexFilters   bool   // Pré-filtro com regex antes do LLM
	PreserveStructure bool   // Mantém estrutura de turnos EVA/Usuário
}

// NewAnonymizationService cria o serviço de anonimização
func NewAnonymizationService(config AnonymizationConfig) (*AnonymizationService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}

	if config.ModelName == "" {
		config.ModelName = "gemini-2.0-flash-exp"
	}

	service := &AnonymizationService{
		client:    client,
		modelName: config.ModelName,
	}

	if config.UseRegexFilters {
		service.initRegexFilters()
	}

	return service, nil
}

// Close fecha a conexão com o LLM
func (as *AnonymizationService) Close() error {
	return as.client.Close()
}

// initRegexFilters inicializa filtros regex comuns
func (as *AnonymizationService) initRegexFilters() {
	as.regexFilters = []*regexp.Regexp{
		// CPF: 123.456.789-00
		regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`),
		// RG: 12.345.678-9
		regexp.MustCompile(`\b\d{2}\.\d{3}\.\d{3}-\d{1}\b`),
		// Telefone: (11) 98765-4321
		regexp.MustCompile(`\(\d{2}\)\s*\d{4,5}-\d{4}`),
		// Email: usuario@dominio.com
		regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		// CEP: 12345-678
		regexp.MustCompile(`\b\d{5}-\d{3}\b`),
		// Endereço com número: Rua X, 123
		regexp.MustCompile(`(?i)(rua|av|avenida|travessa)\s+[^,]+,\s*\d+`),
		// Cartão de crédito: 1234 5678 9012 3456
		regexp.MustCompile(`\b\d{4}\s\d{4}\s\d{4}\s\d{4}\b`),
		// Datas específicas: DD/MM/YYYY
		regexp.MustCompile(`\b\d{2}/\d{2}/\d{4}\b`),
	}
}

// Anonymize remove PII de uma transcrição
func (as *AnonymizationService) Anonymize(ctx context.Context, transcript string) (string, error) {
	// Passo 1: Regex filters (se habilitado)
	filtered := transcript
	if len(as.regexFilters) > 0 {
		filtered = as.applyRegexFilters(transcript)
	}

	// Passo 2: LLM-based anonymization (mais inteligente)
	anonymized, err := as.llmAnonymize(ctx, filtered)
	if err != nil {
		return "", fmt.Errorf("erro ao anonimizar com LLM: %w", err)
	}

	return anonymized, nil
}

// applyRegexFilters aplica substituições regex
func (as *AnonymizationService) applyRegexFilters(text string) string {
	result := text

	replacements := map[*regexp.Regexp]string{
		as.regexFilters[0]: "[CPF]",
		as.regexFilters[1]: "[RG]",
		as.regexFilters[2]: "[TELEFONE]",
		as.regexFilters[3]: "[EMAIL]",
		as.regexFilters[4]: "[CEP]",
		as.regexFilters[5]: "[ENDEREÇO]",
		as.regexFilters[6]: "[CARTÃO]",
		as.regexFilters[7]: "[DATA]",
	}

	for regex, replacement := range replacements {
		result = regex.ReplaceAllString(result, replacement)
	}

	return result
}

// llmAnonymize usa LLM para anonimização inteligente
func (as *AnonymizationService) llmAnonymize(ctx context.Context, text string) (string, error) {
	prompt := as.buildAnonymizationPrompt(text)

	model := as.client.GenerativeModel(as.modelName)
	model.SetTemperature(0.1) // Baixa temperatura para consistência
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(as.getSystemPrompt())},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("erro ao chamar LLM: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia do LLM")
	}

	anonymized := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return strings.TrimSpace(anonymized), nil
}

// getSystemPrompt define instruções para anonimização
func (as *AnonymizationService) getSystemPrompt() string {
	return `Você é um sistema de ANONIMIZAÇÃO de transcrições de terapia.

**Objetivo:**
Remover TODAS as informações pessoais identificáveis (PII) mantendo o conteúdo emocional e os padrões de comportamento.

**O que REMOVER:**
1. Nomes próprios (pessoas, pets, empresas)
   - "João" → "[PESSOA]"
   - "Maria da Silva" → "[PESSOA]"
   - "Empresa XYZ" → "[EMPREGADOR]"

2. Localizações específicas
   - "Moro em São Paulo" → "Moro em [CIDADE]"
   - "No bairro Jardins" → "No bairro [BAIRRO]"
   - "Rua das Flores, 123" → "[ENDEREÇO]"

3. Datas específicas
   - "Em 15 de março" → "Em [DATA]"
   - Exceção: Manter períodos genéricos ("há 3 meses", "na semana passada")

4. Profissões muito específicas
   - "Sou neurocirurgião" → "Trabalho na área de [PROFISSÃO_MÉDICA]"
   - Genéricas OK: "Sou professor" pode ficar

5. Detalhes que permitam identificação
   - "Minha filha de 7 anos chamada Ana" → "Minha filha de [IDADE] anos"
   - "Meu cachorro Totó" → "Meu cachorro"

**O que PRESERVAR:**
- Tom emocional: "Estou muito triste" → mantém
- Relações: "minha mãe", "meu chefe" → mantém
- Sentimentos: "me sinto sozinho" → mantém
- Padrões: "sempre faço isso" → mantém
- Estrutura do diálogo: "EVA:", "Usuário:" → mantém

**Formato de saída:**
Retorne APENAS o texto anonimizado, sem explicações ou comentários.`
}

// buildAnonymizationPrompt constrói o prompt
func (as *AnonymizationService) buildAnonymizationPrompt(text string) string {
	return fmt.Sprintf(`Anonimize esta transcrição:

---
%s
---

Regras:
- Substitua PII por placeholders [TIPO]
- Mantenha a emoção e o contexto
- Preserve "EVA:" e "Usuário:" se existirem
- Retorne APENAS o texto processado`, text)
}

// AnonymizeField anonimiza um campo específico (ex: nome de cidade)
func (as *AnonymizationService) AnonymizeField(fieldType, value string) string {
	// Mapeamento simples para campos conhecidos
	mapping := map[string]string{
		"name":     "[PESSOA]",
		"city":     "[CIDADE]",
		"company":  "[EMPRESA]",
		"email":    "[EMAIL]",
		"phone":    "[TELEFONE]",
		"address":  "[ENDEREÇO]",
		"cpf":      "[CPF]",
		"date":     "[DATA]",
	}

	if placeholder, ok := mapping[fieldType]; ok {
		return placeholder
	}

	return "[DADO_SENSÍVEL]"
}

// ValidateAnonymization verifica se ainda há PII no texto
func (as *AnonymizationService) ValidateAnonymization(ctx context.Context, text string) (bool, []string, error) {
	prompt := fmt.Sprintf(`Analise este texto e verifique se ainda contém PII:

---
%s
---

Se encontrar PII, liste-os. Se estiver seguro, retorne lista vazia.
Retorne JSON:
{
  "is_safe": true/false,
  "remaining_pii": ["item1", "item2", ...]
}`, text)

	model := as.client.GenerativeModel(as.modelName)
	model.SetTemperature(0.1)
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return false, nil, fmt.Errorf("erro ao validar: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return false, nil, fmt.Errorf("resposta vazia ao validar")
	}

	jsonText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	// Parse manual caso json.Unmarshal falhe
	isSafe := strings.Contains(jsonText, `"is_safe": true`) ||
	          strings.Contains(jsonText, `"is_safe":true`)

	// Extrai PII (se houver)
	var piiList []string
	if !isSafe {
		// Regex simples para extrair array de strings
		re := regexp.MustCompile(`"remaining_pii":\s*\[(.*?)\]`)
		matches := re.FindStringSubmatch(jsonText)
		if len(matches) > 1 {
			items := strings.Split(matches[1], ",")
			for _, item := range items {
				cleaned := strings.Trim(strings.TrimSpace(item), `"`)
				if cleaned != "" {
					piiList = append(piiList, cleaned)
				}
			}
		}
	}

	return isSafe, piiList, nil
}

// AnonymizeBatch processa múltiplos textos
func (as *AnonymizationService) AnonymizeBatch(ctx context.Context, texts []string) ([]string, error) {
	results := make([]string, 0, len(texts))

	for i, text := range texts {
		anonymized, err := as.Anonymize(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("erro ao anonimizar texto %d: %w", i, err)
		}
		results = append(results, anonymized)
	}

	return results, nil
}

// GetAnonymizationStats retorna estatísticas sobre o processo
func (as *AnonymizationService) GetAnonymizationStats(original, anonymized string) map[string]interface{} {
	// Conta placeholders inseridos
	placeholders := []string{
		"[PESSOA]", "[CIDADE]", "[EMPRESA]", "[EMAIL]",
		"[TELEFONE]", "[ENDEREÇO]", "[CPF]", "[DATA]",
	}

	counts := make(map[string]int)
	for _, ph := range placeholders {
		counts[ph] = strings.Count(anonymized, ph)
	}

	return map[string]interface{}{
		"original_length":    len(original),
		"anonymized_length":  len(anonymized),
		"reduction_percent":  float64(len(original)-len(anonymized)) / float64(len(original)) * 100,
		"placeholders_added": counts,
	}
}
