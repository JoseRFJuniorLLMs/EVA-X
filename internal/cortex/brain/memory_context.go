// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package brain

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	hippocampusMemory "eva/internal/hippocampus/memory"
	retryPkg "eva/internal/brainstem/infrastructure/retry"
)

// MemoryContext contains metadata for memory storage
type MemoryContext struct {
	Emotion        string   // Detected emotion (e.g., "happy", "sad", "neutral")
	Urgency        string   // Urgency level (e.g., "high", "medium", "low", "ALTA", "MÉDIA", "CRÍTICA")
	Keywords       []string // Extracted keywords
	Importance     float64  // Calculated importance (0-1)
	AudioIntensity int      // Voice intensity 1-10 from AudioAnalysisService
}

// SaveEpisodicMemoryWithContext saves memory with full context metadata
// AUDIT FIX 2026-02-17: Now properly saves emotion, importance, and other metadata
// AUDIT FIX 2026-02-18: Returns error with retry logic instead of fire-and-forget
func (s *Service) SaveEpisodicMemoryWithContext(
	idosoID int64,
	role string,
	content string,
	eventDate time.Time,
	isAtomic bool,
	memCtx MemoryContext,
) error {
	ctx := context.Background()

	// 1. Calculate importance if not set
	if memCtx.Importance == 0 {
		memCtx.Importance = calculateImportance(content, memCtx)
	}

	log.Printf("🧠 [MEMORY-CTX] Importância calculada: %.2f | Emoção: %s | Urgência: %s | Intensidade: %d",
		memCtx.Importance, memCtx.Emotion, memCtx.Urgency, memCtx.AudioIntensity)

	// 2. Generate embedding (não bloqueia se falhar)
	var embedding []float32
	if s.embeddingService != nil {
		if emb, err := s.embeddingService.GenerateEmbedding(ctx, content); err == nil {
			embedding = emb
		}
	}

	// 3. Save via MemoryStore (Postgres + Neo4j + Qdrant com emotion/importance reais)
	if s.memoryStore == nil {
		return fmt.Errorf("memoryStore não inicializado — verifique conexão com banco de dados")
	}

	mem := &hippocampusMemory.Memory{
		IdosoID:    idosoID,
		Speaker:    role,
		Content:    content,
		Emotion:    memCtx.Emotion,
		Importance: memCtx.Importance,
		Topics:     memCtx.Keywords,
		SessionID:  fmt.Sprintf("session-%d", time.Now().Unix()),
		EventDate:  eventDate,
		IsAtomic:   isAtomic,
		Embedding:  embedding,
	}

	// Use retry with exponential backoff for transient failures
	err := retryPkg.Do(ctx, retryPkg.FastConfig(), func(ctx context.Context) error {
		return s.memoryStore.Store(ctx, mem)
	})
	if err != nil {
		log.Printf("❌ [MEMORY-CTX] Falha após retries ao salvar idoso=%d: %v", idosoID, err)
		return err
	}
	return nil
}

// summarizeText creates a simple summary of text
func summarizeText(text string) string {
	// Simple summarization: take first 200 chars or first 2 sentences
	text = strings.TrimSpace(text)

	if len(text) <= 200 {
		return text
	}

	// Try to find sentence boundaries
	sentences := strings.Split(text, ".")
	if len(sentences) >= 2 {
		summary := sentences[0] + "." + sentences[1] + "."
		if len(summary) <= 200 {
			return summary
		}
	}

	// Fallback: truncate at 200 chars
	return text[:200] + "..."
}

// calculateImportance calculates memory importance based on multi-factor analysis
// AUDIT FIX 2026-02-17: Sophisticated multi-factor scoring
// Base 0.5 + emotion +urgency +intensity +content features, capped at 1.0
func calculateImportance(content string, ctx MemoryContext) float64 {
	importance := 0.5 // Base importance
	lower := strings.ToLower(content)

	// ─────────────────────────────────────────────────────────────
	// FATOR 1: EMOÇÃO (PT + EN) — max +0.25
	// ─────────────────────────────────────────────────────────────
	emotionNorm := strings.ToLower(ctx.Emotion)
	switch emotionNorm {
	case "pânico", "crisis", "crise", "emergência", "desespero":
		importance += 0.25
	case "sad", "triste", "tristeza", "angry", "raiva", "fearful", "medo",
		"ansioso", "ansiedade", "sozinho", "solidão", "melancolia":
		importance += 0.20
	case "happy", "alegre", "excited", "feliz", "satisfeito":
		importance += 0.10
	}

	// ─────────────────────────────────────────────────────────────
	// FATOR 2: URGÊNCIA (PT + EN, case-insensitive) — max +0.25
	// ─────────────────────────────────────────────────────────────
	urgencyNorm := strings.ToUpper(strings.TrimSpace(ctx.Urgency))
	switch urgencyNorm {
	case "CRITICA", "CRÍTICA", "CRITICAL":
		importance += 0.25
	case "ALTA", "HIGH":
		importance += 0.20
	case "MEDIA", "MÉDIA", "MEDIUM":
		importance += 0.10
	case "BAIXA", "LOW":
		// no boost
	}

	// ─────────────────────────────────────────────────────────────
	// FATOR 3: INTENSIDADE DE VOZ (1-10) — max +0.15
	// ─────────────────────────────────────────────────────────────
	switch {
	case ctx.AudioIntensity >= 9:
		importance += 0.15
	case ctx.AudioIntensity >= 7:
		importance += 0.10
	case ctx.AudioIntensity >= 5:
		importance += 0.05
	}

	// ─────────────────────────────────────────────────────────────
	// FATOR 4: CONTEÚDO — cumulativo até +0.70
	// ─────────────────────────────────────────────────────────────

	// 4a: Palavras Lacanianas (alta carga emocional/clínica) → +0.20
	lacanKeywords := []string{
		"morte", "morrer", "suicídio", "suicidar", "não aguento",
		"não quero viver", "abandono", "abandonado", "solidão",
		"desespero", "ódio", "culpa", "vazio", "perda", "luto", "trauma",
	}
	for _, kw := range lacanKeywords {
		if strings.Contains(lower, kw) {
			importance += 0.20
			break
		}
	}

	// 4b: Relações pessoais → +0.15
	personalKeywords := []string{
		"filha", "filho", "esposa", "marido", "mãe", "pai",
		"neto", "neta", "irmão", "irmã", "avó", "avô",
		"bisavó", "bisavô",
		"daughter", "son", "wife", "husband", "mother", "father",
		"gosto", "amo", "prefiro", "like", "love", "prefer",
		"nome", "chama", "called", "me chamo", "meu nome",
	}
	for _, kw := range personalKeywords {
		if strings.Contains(lower, kw) {
			importance += 0.15
			break
		}
	}

	// 4c: Urgência médica → +0.15
	medicalKeywords := []string{
		"hospital", "internação", "uti", "cirurgia", "operação",
		"remédio", "medicamento", "dor", "doença", "sintoma", "febre",
		"emergência", "socorro", "ajuda", "urgente",
	}
	for _, kw := range medicalKeywords {
		if strings.Contains(lower, kw) {
			importance += 0.15
			break
		}
	}

	// 4d: Referências temporais (lembrar datas, consultas) → +0.10
	temporalKeywords := []string{
		"hoje", "amanhã", "não esqueça", "lembre", "preciso lembrar",
		"importante lembrar", "consulta", "agendamento", "compromisso",
		"aniversário",
	}
	for _, kw := range temporalKeywords {
		if strings.Contains(lower, kw) {
			importance += 0.10
			break
		}
	}

	// 4e: Localização de objetos (importante para idosos) → +0.10
	locationKeywords := []string{
		"guardei", "coloquei", "está na", "está no", "onde está",
		"endereço", "chave", "documento", "cartão",
	}
	for _, kw := range locationKeywords {
		if strings.Contains(lower, kw) {
			importance += 0.10
			break
		}
	}

	// Cap at 1.0
	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}
