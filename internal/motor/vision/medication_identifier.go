package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"eva-mind/internal/brainstem/database"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// MedicationIdentifier handles visual identification of medications using Gemini Vision
type MedicationIdentifier struct {
	apiKey string
	client *genai.Client
}

// NewMedicationIdentifier creates a new medication identifier
func NewMedicationIdentifier(apiKey string) (*MedicationIdentifier, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &MedicationIdentifier{
		apiKey: apiKey,
		client: client,
	}, nil
}

// IdentificationResult represents the result of medication identification
type IdentificationResult struct {
	MedicationName     string                          `json:"medication_name"`
	GenericName        string                          `json:"generic_name"`
	Dosage             string                          `json:"dosage"`
	PharmaceuticalForm string                          `json:"pharmaceutical_form"`
	Color              string                          `json:"color"`
	Manufacturer       string                          `json:"manufacturer"`
	ExpiryDate         *time.Time                      `json:"expiry_date"`
	BatchNumber        string                          `json:"batch_number"`
	Confidence         float64                         `json:"confidence"`
	Reasoning          string                          `json:"reasoning"`
	MatchedMedication  *database.Medicamento           `json:"matched_medication"`
	SafetyCheck        *database.MedicationSafetyCheck `json:"safety_check"`
}

// IdentifyFromImage analyzes an image and identifies the medication
func (m *MedicationIdentifier) IdentifyFromImage(
	imageBase64 string,
	candidateMeds []database.Medicamento,
	db *database.DB,
) (*IdentificationResult, error) {

	log.Printf("🔍 [VISION] Iniciando identificação visual de medicamento...")

	// 1. Decode base64 image
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// 2. Create prompt for Gemini Vision
	prompt := m.createPrompt(candidateMeds)

	// 3. Call Gemini Vision API
	ctx := context.Background()
	model := m.client.GenerativeModel("gemini-2.0-flash-exp")

	// Configure model
	model.SetTemperature(0.2) // Lower temperature for more factual responses
	model.ResponseMIMEType = "application/json"

	// Create content parts
	resp, err := model.GenerateContent(
		ctx,
		genai.Text(prompt),
		genai.ImageData("image/jpeg", imageData),
	)

	if err != nil {
		log.Printf("❌ [VISION] Erro ao chamar Gemini Vision: %v", err)
		return nil, fmt.Errorf("Gemini Vision API error: %w", err)
	}

	// 4. Parse response
	result, err := m.parseVisionResponse(resp)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ [VISION] Medicamento identificado: %s (confiança: %.2f%%)", result.MedicationName, result.Confidence*100)

	// 5. Match with database
	matchedMed := m.findBestMatch(result, candidateMeds)
	result.MatchedMedication = matchedMed

	// 6. Perform safety checks if matched
	if matchedMed != nil && db != nil {
		safetyCheck, err := db.CheckMedicationSafety(matchedMed.ID)
		if err != nil {
			log.Printf("⚠️ [VISION] Erro ao verificar segurança: %v", err)
		} else {
			result.SafetyCheck = safetyCheck
		}
	}

	return result, nil
}

// createPrompt creates the prompt for Gemini Vision
func (m *MedicationIdentifier) createPrompt(candidateMeds []database.Medicamento) string {
	candidatesStr := ""
	if len(candidateMeds) > 0 {
		candidatesStr = "\n\n**Medicamentos possíveis deste paciente:**\n"
		for _, med := range candidateMeds {
			candidatesStr += fmt.Sprintf("- %s (%s) - %s - Cor: %s\n",
				med.Nome, med.Dosagem, med.Fabricante, med.CorEmbalagem)
		}
	}

	prompt := fmt.Sprintf(`Você é um especialista farmacêutico em identificação de medicamentos.

Analise esta imagem de medicamento e extraia as seguintes informações:

1. **Nome do medicamento** (comercial)
2. **Nome genérico** (princípio ativo)
3. **Dosagem exata** (ex: "20mg", "500mg/ml")
4. **Forma farmacêutica** (comprimido, cápsula, xarope, injeção, etc)
5. **Cor predominante** da embalagem ou pílula
6. **Marca/laboratório**
7. **Data de validade** (se visível - formato: YYYY-MM-DD)
8. **Número de lote** (se visível)
9. **Nível de confiança** (0.0 a 1.0)
10. **Raciocínio** sobre como identificou

%s

**IMPORTANTE:**
- Seja preciso na dosagem
- Se não tiver certeza, indique confiança baixa
- Procure por códigos impressos nas pílulas
- Verifique se o medicamento corresponde à lista de candidatos

Retorne a resposta em formato JSON com os campos:
{
  "medication_name": "string",
  "generic_name": "string",
  "dosage": "string",
  "pharmaceutical_form": "string",
  "color": "string",
  "manufacturer": "string",
  "expiry_date": "YYYY-MM-DD ou null",
  "batch_number": "string ou null",
  "confidence": 0.95,
  "reasoning": "string"
}`, candidatesStr)

	return prompt
}

// parseVisionResponse parses the Gemini Vision API response
func (m *MedicationIdentifier) parseVisionResponse(resp *genai.GenerateContentResponse) (*IdentificationResult, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini Vision")
	}

	// Extract JSON from response
	part := resp.Candidates[0].Content.Parts[0]
	textPart, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from Gemini")
	}

	jsonStr := string(textPart)

	// Clean JSON (remove markdown code blocks if present)
	jsonStr = strings.TrimPrefix(jsonStr, "```json\n")
	jsonStr = strings.TrimSuffix(jsonStr, "\n```")
	jsonStr = strings.TrimSpace(jsonStr)

	// Parse JSON
	var result IdentificationResult
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		log.Printf("❌ [VISION] Erro ao parsear JSON: %v\nJSON: %s", err, jsonStr)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// findBestMatch finds the best matching medication from candidates
func (m *MedicationIdentifier) findBestMatch(
	result *IdentificationResult,
	candidates []database.Medicamento,
) *database.Medicamento {

	if len(candidates) == 0 {
		return nil
	}

	var bestMatch *database.Medicamento
	var bestScore float64

	for i := range candidates {
		candidate := &candidates[i]
		score := m.calculateMatchScore(result, candidate)

		log.Printf("📊 [VISION] Match score para %s: %.2f", candidate.Nome, score)

		if score > bestScore {
			bestScore = score
			bestMatch = candidate
		}
	}

	// Only return match if score is above threshold
	if bestScore < 0.6 {
		log.Printf("⚠️ [VISION] Melhor match tem score %.2f < 0.6, retornando nil", bestScore)
		return nil
	}

	log.Printf("✅ [VISION] Melhor match: %s (score: %.2f)", bestMatch.Nome, bestScore)
	return bestMatch
}

// calculateMatchScore calculates similarity score between identified and candidate medication
func (m *MedicationIdentifier) calculateMatchScore(
	result *IdentificationResult,
	candidate *database.Medicamento,
) float64 {

	score := 0.0

	// Name matching (40% weight) - fuzzy matching
	nameScore := m.fuzzyMatch(result.MedicationName, candidate.Nome)
	score += nameScore * 0.4

	// Dosage matching (30% weight) - exact match
	if strings.EqualFold(result.Dosage, candidate.Dosagem) {
		score += 0.3
	}

	// Manufacturer matching (15% weight) - fuzzy matching
	if candidate.Fabricante != "" {
		mfgScore := m.fuzzyMatch(result.Manufacturer, candidate.Fabricante)
		score += mfgScore * 0.15
	}

	// Color matching (15% weight) - fuzzy matching
	if candidate.CorEmbalagem != "" {
		colorScore := m.fuzzyMatch(result.Color, candidate.CorEmbalagem)
		score += colorScore * 0.15
	}

	return score
}

// fuzzyMatch performs fuzzy string matching (simple version)
func (m *MedicationIdentifier) fuzzyMatch(str1, str2 string) float64 {
	if str1 == "" || str2 == "" {
		return 0.0
	}

	// Normalize strings
	s1 := strings.ToLower(strings.TrimSpace(str1))
	s2 := strings.ToLower(strings.TrimSpace(str2))

	// Exact match
	if s1 == s2 {
		return 1.0
	}

	// Contains match
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 0.8
	}

	// Calculate Levenshtein-like similarity (simplified)
	maxLen := max(len(s1), len(s2))
	if maxLen == 0 {
		return 1.0
	}

	// Count matching characters at same position
	matches := 0
	minLen := min(len(s1), len(s2))
	for i := 0; i < minLen; i++ {
		if s1[i] == s2[i] {
			matches++
		}
	}

	return float64(matches) / float64(maxLen)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Close closes the Gemini client
func (m *MedicationIdentifier) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
