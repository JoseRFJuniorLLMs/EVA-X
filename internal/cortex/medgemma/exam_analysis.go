package medgemma

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
)

// AnalyzeExam analisa qualquer tipo de exame usando prompt especializado
func (ms *MedGemmaService) AnalyzeExam(ctx context.Context, examType ExamType, imageData []byte, mimeType string, metadata map[string]string) (map[string]interface{}, error) {
	// Obter prompt especializado
	prompt := GetPromptForExam(examType, metadata)

	// Adicionar timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := ms.model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData(mimeType, imageData),
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao gerar análise: %w", err)
	}

	return ms.parseGenericResponse(resp)
}

// parseGenericResponse extrai JSON de qualquer tipo de resposta
func (ms *MedGemmaService) parseGenericResponse(resp *genai.GenerateContentResponse) (map[string]interface{}, error) {
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("erro ao parsear JSON: %w", err)
	}

	return result, nil
}

// MalariaAnalysis representa análise de malária
type MalariaAnalysis struct {
	Result             string   `json:"result"`
	Species            string   `json:"species"`
	Parasitemia        string   `json:"parasitemia"`
	InfectedCellsCount int      `json:"infected_cells_count"`
	TotalCellsCounted  int      `json:"total_cells_counted"`
	Severity           string   `json:"severity"`
	Confidence         float64  `json:"confidence"`
	Recommendations    []string `json:"recommendations"`
}

// TBScreeningAnalysis representa triagem de TB
type TBScreeningAnalysis struct {
	TBProbability         string   `json:"tb_probability"`
	Findings              []string `json:"findings"`
	Severity              string   `json:"severity"`
	RequiresConfirmation  bool     `json:"requires_confirmation"`
	Urgency               string   `json:"urgency"`
	DifferentialDiagnosis []string `json:"differential_diagnosis"`
	Recommendations       []string `json:"recommendations"`
}

// RapidTestAnalysis representa análise de teste rápido
type RapidTestAnalysis struct {
	TestValid          bool     `json:"test_valid"`
	ControlLinePresent bool     `json:"control_line_present"`
	TestLinePresent    bool     `json:"test_line_present"`
	TestLineIntensity  string   `json:"test_line_intensity"`
	Result             string   `json:"result"`
	Confidence         float64  `json:"confidence"`
	Recommendations    []string `json:"recommendations"`
}

// SkinLesionAnalysis representa análise de lesão cutânea
type SkinLesionAnalysis struct {
	LesionType      string          `json:"lesion_type"`
	MelanomaRisk    string          `json:"melanoma_risk"`
	ABCDEScore      map[string]bool `json:"abcde_score"`
	MpoxProbability float64         `json:"mpox_probability"`
	MpoxFeatures    []string        `json:"mpox_features"`
	InfectionSigns  []string        `json:"infection_signs"`
	Severity        string          `json:"severity"`
	Recommendations []string        `json:"recommendations"`
}

// PressureUlcerAnalysis representa análise de úlcera de pressão
type PressureUlcerAnalysis struct {
	Stage           string   `json:"stage"`
	Size            string   `json:"size"`
	Location        string   `json:"location"`
	TissueType      string   `json:"tissue_type"`
	InfectionSigns  []string `json:"infection_signs"`
	Severity        string   `json:"severity"`
	Recommendations []string `json:"recommendations"`
}

// DiabeticFootAnalysis representa análise de pé diabético
type DiabeticFootAnalysis struct {
	WagnerGrade     string   `json:"wagner_grade"`
	UlcerPresent    bool     `json:"ulcer_present"`
	UlcerDepth      string   `json:"ulcer_depth"`
	NecrosisPresent bool     `json:"necrosis_present"`
	InfectionSigns  []string `json:"infection_signs"`
	VascularStatus  string   `json:"vascular_status"`
	Severity        string   `json:"severity"`
	AmputationRisk  string   `json:"amputation_risk"`
	Recommendations []string `json:"recommendations"`
}
