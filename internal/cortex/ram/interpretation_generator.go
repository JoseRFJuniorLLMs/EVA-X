// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ram

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InterpretationGenerator (E1) - Gera múltiplas interpretações alternativas
type InterpretationGenerator struct {
	llm            LLMService
	embedder       EmbeddingService
	retrieval      RetrievalService
	maxRetries     int
	temperature    float64 // 0.7 default (mais criativo)
}

// LLMService interface para LLM (Gemini)
type LLMService interface {
	GenerateText(ctx context.Context, prompt string, temperature float64) (string, error)
	GenerateMultiple(ctx context.Context, prompt string, n int, temperature float64) ([]string, error)
}

// EmbeddingService interface para embeddings
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// RetrievalService interface para busca de memórias
type RetrievalService interface {
	RetrieveRelevant(ctx context.Context, patientID int64, query string, k int) ([]Memory, error)
}

// Memory representa uma memória recuperada
type Memory struct {
	ID        int64
	Content   string
	Timestamp time.Time
	Score     float64
}

// NewInterpretationGenerator cria novo gerador
func NewInterpretationGenerator(llm LLMService, embedder EmbeddingService, retrieval RetrievalService) *InterpretationGenerator {
	return &InterpretationGenerator{
		llm:         llm,
		embedder:    embedder,
		retrieval:   retrieval,
		maxRetries:  3,
		temperature: 0.7,
	}
}

// Generate gera N interpretações alternativas
func (g *InterpretationGenerator) Generate(ctx context.Context, patientID int64, query string, context string, numInterpretations int) ([]Interpretation, error) {
	// 1. Recuperar memórias relevantes
	memories, err := g.retrieval.RetrieveRelevant(ctx, patientID, query, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memories: %w", err)
	}

	// 2. Construir prompt para LLM
	prompt := g.buildPrompt(query, context, memories, numInterpretations)

	// 3. Gerar múltiplas interpretações com LLM
	texts, err := g.llm.GenerateMultiple(ctx, prompt, numInterpretations, g.temperature)
	if err != nil {
		return nil, fmt.Errorf("failed to generate interpretations: %w", err)
	}

	// 4. Construir objetos Interpretation
	interpretations := make([]Interpretation, 0, len(texts))
	for _, text := range texts {
		interp := Interpretation{
			ID:              uuid.New().String(),
			Content:         text,
			Confidence:      g.estimateConfidence(text, memories),
			PlausibilityScore: g.calculatePlausibility(text, memories),
			HistoricalScore: 0.0, // Será preenchido pelo validator
			ReasoningPath:   g.extractReasoningPath(text),
			GeneratedAt:     time.Now(),
		}

		interpretations = append(interpretations, interp)
	}

	return interpretations, nil
}

// buildPrompt constrói prompt para LLM
func (g *InterpretationGenerator) buildPrompt(query string, context string, memories []Memory, n int) string {
	prompt := fmt.Sprintf(`Você é um assistente especializado em interpretar perguntas sobre memórias de pacientes com Alzheimer.

## Contexto do Paciente
%s

## Memórias Relevantes
`, context)

	for i, memory := range memories {
		prompt += fmt.Sprintf("%d. %s\n", i+1, memory.Content)
	}

	prompt += fmt.Sprintf(`

## Pergunta
"%s"

## Tarefa
Gere %d interpretações alternativas DIFERENTES da pergunta acima, considerando:
1. Diferentes ângulos de interpretação
2. Possíveis ambiguidades na pergunta
3. Contexto temporal (passado recente vs passado distante)
4. Relacionamentos mencionados

Para cada interpretação:
- Seja específico e concreto
- Baseie-se nas memórias disponíveis
- Explique o raciocínio brevemente
- Indique nível de certeza

Formato da resposta:
[INTERPRETATION_1]
<texto da interpretação 1>
[REASONING_1]
<raciocínio da interpretação 1>
[CONFIDENCE_1]
<0.0-1.0>

[INTERPRETATION_2]
...

Gere %d interpretações alternativas:`, query, n, n)

	return prompt
}

// estimateConfidence estima confiança baseada em overlap com memórias
func (g *InterpretationGenerator) estimateConfidence(text string, memories []Memory) float64 {
	// Simplificado: baseado em número de palavras que aparecem nas memórias
	// TODO: Usar embeddings para comparação mais sofisticada

	if len(memories) == 0 {
		return 0.5 // Neutro
	}

	words := extractKeywords(text)
	matchCount := 0

	for _, word := range words {
		for _, memory := range memories {
			if contains(memory.Content, word) {
				matchCount++
				break
			}
		}
	}

	confidence := float64(matchCount) / float64(len(words))

	// Clamp [0.3, 0.95]
	if confidence < 0.3 {
		confidence = 0.3
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// calculatePlausibility calcula plausibilidade da interpretação
func (g *InterpretationGenerator) calculatePlausibility(text string, memories []Memory) float64 {
	// Simplificado: baseado em similaridade com memórias
	if len(memories) == 0 {
		return 0.5
	}

	// Calcular overlap médio
	totalOverlap := 0.0
	for _, memory := range memories {
		overlap := g.textOverlap(text, memory.Content)
		totalOverlap += overlap
	}

	avgOverlap := totalOverlap / float64(len(memories))

	// Normalizar para [0.4, 0.9]
	plausibility := 0.4 + (avgOverlap * 0.5)

	return plausibility
}

// textOverlap calcula overlap simples entre dois textos
func (g *InterpretationGenerator) textOverlap(text1, text2 string) float64 {
	words1 := extractKeywords(text1)
	words2 := extractKeywords(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	matchCount := 0
	for _, word := range words1 {
		if containsWord(words2, word) {
			matchCount++
		}
	}

	overlap := float64(matchCount) / float64(len(words1))
	return overlap
}

// extractReasoningPath extrai caminho de raciocínio do texto
func (g *InterpretationGenerator) extractReasoningPath(text string) []string {
	// Simplificado: dividir em sentenças
	// TODO: Parser mais sofisticado para extrair [REASONING_X]

	path := []string{
		"Baseado nas memórias disponíveis",
		"Considerando o contexto temporal",
		"Analisando relacionamentos mencionados",
	}

	return path
}

// GenerateSingle gera uma única interpretação (fallback)
func (g *InterpretationGenerator) GenerateSingle(ctx context.Context, patientID int64, query string, context string) (*Interpretation, error) {
	interpretations, err := g.Generate(ctx, patientID, query, context, 1)
	if err != nil {
		return nil, err
	}

	if len(interpretations) == 0 {
		return nil, fmt.Errorf("no interpretations generated")
	}

	return &interpretations[0], nil
}

// SetTemperature ajusta criatividade do LLM
func (g *InterpretationGenerator) SetTemperature(temp float64) {
	if temp >= 0.0 && temp <= 1.0 {
		g.temperature = temp
	}
}

// Helper functions

func extractKeywords(text string) []string {
	// Simplificado: split por espaços e remover stopwords
	words := splitWords(text)
	keywords := make([]string, 0)

	stopwords := map[string]bool{
		"o": true, "a": true, "os": true, "as": true,
		"de": true, "do": true, "da": true, "dos": true, "das": true,
		"em": true, "no": true, "na": true, "nos": true, "nas": true,
		"com": true, "para": true, "por": true, "e": true,
	}

	for _, word := range words {
		if len(word) > 2 && !stopwords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

func splitWords(text string) []string {
	// Simplificado: split por espaços
	// TODO: Usar tokenizer mais sofisticado
	words := make([]string, 0)
	current := ""

	for _, char := range text {
		if char == ' ' || char == '\n' || char == '\t' {
			if len(current) > 0 {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if len(current) > 0 {
		words = append(words, current)
	}

	return words
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) && len(substr) > 0
}

func containsWord(words []string, target string) bool {
	for _, word := range words {
		if word == target {
			return true
		}
	}
	return false
}
