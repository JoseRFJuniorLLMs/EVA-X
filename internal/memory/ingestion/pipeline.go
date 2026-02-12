package ingestion

import (
	"context"
	"encoding/json"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/cortex/gemini"
	"fmt"
	"log"
	"time"
)

// IngestionPipeline transforms raw text into structured atomic facts
type IngestionPipeline struct {
	cfg *config.Config
}

// NewIngestionPipeline creates a new pipeline service
func NewIngestionPipeline(cfg *config.Config) *IngestionPipeline {
	return &IngestionPipeline{cfg: cfg}
}

// ProcessText extracts atomic facts from a raw string
func (p *IngestionPipeline) ProcessText(ctx context.Context, text string) ([]AtomicFact, error) {
	log.Printf("🧪 [Ingestion] Extracting atomic facts from: %d chars", len(text))

	now := time.Now()
	prompt := fmt.Sprintf(`
		Você é um extractor de fatos atômicos para o sistema EVA-Mind.
		Sua tarefa é quebrar o texto abaixo em uma lista de FATOS ATÔMICOS independentes.

		REGRAS:
		1. Resolva ambiguidades: Substitua pronomes ("ele", "ela") pelos nomes reais se o contexto permitir.
		2. Grounding Temporal: Identifique quando o evento ocorreu.
		   - Se o texto diz "ontem", "há um mês", ou uma data específica, calcule o event_date.
		   - Data atual de referência: %s
		3. Estrutura: Cada fato deve ter Sujeito, Predicado e Objeto.
		4. Output: Retorne estritamente um JSON array seguindo o esquema abaixo.

		ESQUEMA JSON:
		{
			"resolved_text": "Texto completo e claro do fato",
			"subject": "Sujeito",
			"predicate": "Ação/Verbo",
			"object": "Objeto da ação",
			"event_date": "ISO8601 string",
			"confidence": 0.95
		}

		TEXTO:
		"%s"
	`, now.Format("2006-01-02 15:04:05"), text)

	resp, err := gemini.AnalyzeText(p.cfg, prompt)
	if err != nil {
		return nil, fmt.Errorf("gemini analysis failed: %w", err)
	}

	// Clean JSON markdown if present
	resp = p.cleanJSON(resp)

	var facts []AtomicFact
	if err := json.Unmarshal([]byte(resp), &facts); err != nil {
		return nil, fmt.Errorf("failed to parse atomic facts JSON: %w", err)
	}

	// Set metadata
	for i := range facts {
		facts[i].DocumentDate = now
		facts[i].IsAtomic = true
		if facts[i].EventDate.IsZero() {
			facts[i].EventDate = now // Default to now if not extracted
		}
	}

	log.Printf("✅ [Ingestion] Extracted %d atomic facts", len(facts))
	return facts, nil
}

func (p *IngestionPipeline) cleanJSON(input string) string {
	// Simple check for markdown code blocks
	if start := fmt.Sprint("```json\n"); len(input) > len(start) && input[:len(start)] == start {
		input = input[len(start):]
		if end := "```"; input[len(input)-len(end):] == end {
			input = input[:len(input)-len(end)]
		}
	} else if start := "```"; len(input) > len(start) && input[:len(start)] == start {
		input = input[len(start):]
		if end := "```"; input[len(input)-len(end):] == end {
			input = input[:len(input)-len(end)]
		}
	}
	return input
}
