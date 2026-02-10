package memory

import (
	"context"
	"log"
	"strings"
)

// MetadataAnalyzer extrai metadados de texto usando LLM
type MetadataAnalyzer struct {
	geminiAPIKey string
}

// NewMetadataAnalyzer cria um novo analisador
func NewMetadataAnalyzer(apiKey string) *MetadataAnalyzer {
	return &MetadataAnalyzer{geminiAPIKey: apiKey}
}

// Metadata representa os metadados extraídos
type Metadata struct {
	Emotion    string   `json:"emotion"`
	Importance float64  `json:"importance"`
	Topics     []string `json:"topics"`
}

// Analyze extrai emoção, importância e tópicos de um texto
func (m *MetadataAnalyzer) Analyze(ctx context.Context, text string) (*Metadata, error) {
	// TODO: Implementar análise via Gemini API quando necessário
	// Por enquanto, usar apenas análise heurística
	return m.analyzeHeuristic(text), nil
}

// analyzeHeuristic usa regras simples (fallback se LLM falhar)
func (m *MetadataAnalyzer) analyzeHeuristic(text string) *Metadata {
	text = strings.ToLower(text)

	// Detectar emoção por palavras-chave
	emotion := "neutro"
	if strings.Contains(text, "feliz") || strings.Contains(text, "alegr") || strings.Contains(text, "ador") {
		emotion = "feliz"
	} else if strings.Contains(text, "trist") || strings.Contains(text, "chora") || strings.Contains(text, "solid") {
		emotion = "triste"
	} else if strings.Contains(text, "nervos") || strings.Contains(text, "ansios") || strings.Contains(text, "preocup") {
		emotion = "ansioso"
	} else if strings.Contains(text, "confus") || strings.Contains(text, "esquec") || strings.Contains(text, "não lembr") {
		emotion = "confuso"
	}

	// Detectar importância
	importance := 0.5 // Padrão médio
	if strings.Contains(text, "dor") || strings.Contains(text, "médico") || strings.Contains(text, "remédio") {
		importance = 0.8
	} else if strings.Contains(text, "emergência") || strings.Contains(text, "socorro") || strings.Contains(text, "caí") {
		importance = 1.0
	} else if strings.Contains(text, "tempo") || strings.Contains(text, "hora") {
		importance = 0.3
	}

	// Detectar tópicos
	topics := []string{}
	if strings.Contains(text, "médico") || strings.Contains(text, "dor") || strings.Contains(text, "saúde") || strings.Contains(text, "remédio") {
		topics = append(topics, "saúde")
	}
	if strings.Contains(text, "filho") || strings.Contains(text, "neto") || strings.Contains(text, "família") {
		topics = append(topics, "família")
	}
	if strings.Contains(text, "medicamento") || strings.Contains(text, "remédio") || strings.Contains(text, "tomar") {
		topics = append(topics, "medicamento")
	}
	if strings.Contains(text, "passeio") || strings.Contains(text, "música") || strings.Contains(text, "tv") {
		topics = append(topics, "lazer")
	}

	if len(topics) == 0 {
		topics = []string{"geral"}
	}

	return &Metadata{
		Emotion:    emotion,
		Importance: importance,
		Topics:     topics,
	}
}

// AnalyzeBatch processa múltiplos textos
func (m *MetadataAnalyzer) AnalyzeBatch(ctx context.Context, texts []string) ([]*Metadata, error) {
	results := make([]*Metadata, len(texts))

	for i, text := range texts {
		meta, err := m.Analyze(ctx, text)
		if err != nil {
			log.Printf("⚠️ Erro ao analisar texto %d: %v", i, err)
			meta = m.analyzeHeuristic(text) // Fallback
		}
		results[i] = meta
	}

	return results, nil
}
