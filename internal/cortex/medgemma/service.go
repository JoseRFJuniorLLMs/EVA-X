package medgemma

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// MedGemmaService gerencia análises de imagens médicas
type MedGemmaService struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewMedGemmaService cria um novo serviço MedGemma
func NewMedGemmaService(apiKey string) (*MedGemmaService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}

	// Usar Gemini 2.0 Flash com capacidades de visão
	// Nota: MedGemma específico pode não estar disponível via API pública ainda
	// Usando Gemini Flash com prompts médicos especializados
	model := client.GenerativeModel("gemini-2.0-flash-exp")

	return &MedGemmaService{
		client: client,
		model:  model,
	}, nil
}

// AnalyzePrescription analisa uma imagem de receita médica
func (ms *MedGemmaService) AnalyzePrescription(ctx context.Context, imageData []byte, mimeType string) (*PrescriptionAnalysis, error) {
	prompt := `Você é um assistente médico especializado em análise de receitas médicas.
Analise esta imagem de receita e extraia as seguintes informações:

1. Lista completa de medicamentos prescritos
2. Para cada medicamento:
   - Nome completo
   - Dosagem (mg, ml, etc)
   - Frequência de uso (quantas vezes por dia)
   - Horários recomendados
   - Duração do tratamento (se especificado)
3. Nome do médico
4. CRM do médico
5. Data da receita
6. Instruções especiais ou observações

IMPORTANTE:
- Se não conseguir ler alguma informação, indique "não legível"
- Seja preciso com dosagens e frequências
- Identifique se há medicamentos controlados

Retorne a resposta no seguinte formato JSON:
{
  "medications": [
    {
      "name": "nome do medicamento",
      "dosage": "dosagem",
      "frequency": "frequência",
      "schedule": "horários",
      "duration": "duração"
    }
  ],
  "doctor_name": "nome do médico",
  "doctor_crm": "CRM",
  "prescription_date": "data",
  "special_instructions": "instruções especiais",
  "controlled_medications": true/false
}`

	// Adicionar timeout de 30 segundos
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := ms.model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData(mimeType, imageData),
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao gerar análise: %w", err)
	}

	return ms.parsePrescriptionResponse(resp)
}

// AnalyzeWound analisa uma imagem de ferida ou lesão
func (ms *MedGemmaService) AnalyzeWound(ctx context.Context, imageData []byte, mimeType string) (*WoundAnalysis, error) {
	prompt := `Você é um assistente médico especializado em dermatologia e análise de feridas.
Analise esta imagem de ferida/lesão e forneça:

1. Tipo de lesão (corte, queimadura, úlcera, abrasão, etc)
2. Tamanho aproximado (em cm)
3. Aparência (cor, textura, bordas)
4. Sinais de infecção:
   - Vermelhidão excessiva
   - Inchaço
   - Presença de pus ou secreção
   - Calor local
5. Nível de gravidade: BAIXO, MÉDIO, ALTO ou CRÍTICO
6. Recomendações de cuidado imediato
7. Necessidade de atendimento médico (sim/não)
8. Urgência (immediate, within_24h, within_week, routine)

CRITÉRIOS DE GRAVIDADE:
- CRÍTICO: Sangramento intenso, queimadura de 3º grau, sinais severos de infecção
- ALTO: Ferida profunda, sinais moderados de infecção, área extensa
- MÉDIO: Ferida superficial com sinais leves de infecção
- BAIXO: Ferida superficial limpa, pequena

IMPORTANTE: Sempre recomendar consulta médica para lesões graves ou com sinais de infecção.

Retorne no formato JSON:
{
  "type": "tipo de lesão",
  "size": "tamanho aproximado",
  "appearance": "descrição da aparência",
  "infection_signs": ["sinal1", "sinal2"],
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "recommendations": ["recomendação1", "recomendação2"],
  "seek_medical_care": true/false,
  "urgency": "immediate|within_24h|within_week|routine"
}`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := ms.model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData(mimeType, imageData),
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao gerar análise: %w", err)
	}

	return ms.parseWoundResponse(resp)
}

// parsePrescriptionResponse extrai análise de receita da resposta
func (ms *MedGemmaService) parsePrescriptionResponse(resp *genai.GenerateContentResponse) (*PrescriptionAnalysis, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia")
	}

	var fullText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			fullText += string(txt)
		}
	}

	// Procurar por JSON na resposta
	jsonStart := strings.Index(fullText, "{")
	jsonEnd := strings.LastIndex(fullText, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("JSON não encontrado na resposta")
	}

	jsonStr := fullText[jsonStart : jsonEnd+1]

	var analysis PrescriptionAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON: %w", err)
	}

	return &analysis, nil
}

// parseWoundResponse extrai análise de ferida da resposta
func (ms *MedGemmaService) parseWoundResponse(resp *genai.GenerateContentResponse) (*WoundAnalysis, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia")
	}

	var fullText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			fullText += string(txt)
		}
	}

	jsonStart := strings.Index(fullText, "{")
	jsonEnd := strings.LastIndex(fullText, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("JSON não encontrado na resposta")
	}

	jsonStr := fullText[jsonStart : jsonEnd+1]

	var analysis WoundAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON: %w", err)
	}

	return &analysis, nil
}

// Close fecha o cliente
func (ms *MedGemmaService) Close() error {
	return ms.client.Close()
}

// PrescriptionAnalysis representa análise de receita médica
type PrescriptionAnalysis struct {
	Medications           []Medication `json:"medications"`
	DoctorName            string       `json:"doctor_name"`
	DoctorCRM             string       `json:"doctor_crm"`
	PrescriptionDate      string       `json:"prescription_date"`
	SpecialInstructions   string       `json:"special_instructions"`
	ControlledMedications bool         `json:"controlled_medications"`
}

// Medication representa um medicamento prescrito
type Medication struct {
	Name      string `json:"name"`
	Dosage    string `json:"dosage"`
	Frequency string `json:"frequency"`
	Schedule  string `json:"schedule"`
	Duration  string `json:"duration"`
}

// WoundAnalysis representa análise de ferida
type WoundAnalysis struct {
	Type            string   `json:"type"`
	Size            string   `json:"size"`
	Appearance      string   `json:"appearance"`
	InfectionSigns  []string `json:"infection_signs"`
	Severity        string   `json:"severity"`
	Recommendations []string `json:"recommendations"`
	SeekMedicalCare bool     `json:"seek_medical_care"`
	Urgency         string   `json:"urgency"`
}
