// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"strings"
	"unicode/utf8"
)

// ComplexitySignal represents a single scoring signal.
type ComplexitySignal struct {
	Name  string
	Score float64
}

// ComplexityAssessment is the result of assessing a message.
type ComplexityAssessment struct {
	NeedsSystem2 bool
	Score        float64            // 0.0 → 1.0
	Signals      []ComplexitySignal // breakdown for logging/debugging
}

// complexityThreshold is the minimum score to activate System 2.
// Tune this to balance quality vs. latency.
const complexityThreshold = 0.45

// ──────────────────────────────────────────────────────────────────────────────
// Clinical signal lexicons
// ──────────────────────────────────────────────────────────────────────────────

// highComplexityTerms — polypharmacy, acute symptoms, neurological, psychiatric
var highComplexityTerms = []string{
	// Acute critical
	"dor no peito", "falta de ar", "pressão no peito", "desmaio", "convulsão",
	"paralisia", "perda de consciência", "vômito com sangue", "sangramento intenso",
	"dor de cabeça súbita", "visão dupla", "confusão mental",

	// Medication interactions
	"interação medicamentosa", "efeito colateral", "overdose", "dose errada",
	"esqueci o remédio", "mudei o remédio", "parei o remédio",

	// Psychiatric/cognitive
	"suicídio", "não quero viver", "me machucar", "depressão", "ansiedade severa",
	"alucinação", "delírio", "paranoia", "demência", "alzheimer",

	// Diagnosis-seeking
	"diagnóstico", "exame alterado", "resultado ruim", "câncer", "tumor",
	"marcador elevado", "glicose alta", "pressão alta crônica",
}

// mediumComplexityTerms — general health concerns
var mediumComplexityTerms = []string{
	"dor", "febre", "tontura", "cansaço", "fraqueza", "enjoo", "náusea",
	"vômito", "diarreia", "inchaço", "dormência", "formigamento",
	"palpitação", "coração", "respiração", "cabeça", "memória",
	"remédio", "comprimido", "medicamento", "dose", "tratamento",
	"médico", "hospital", "consulta", "exame", "resultado",
}

// urgencyPhrases — expressions that signal the patient is worried/uncertain
var urgencyPhrases = []string{
	"é grave", "é sério", "preciso de ajuda", "o que faço", "o que eu faço",
	"devo ir", "preciso ir", "urgente", "emergência", "socorro",
	"preocupado", "preocupada", "com medo", "assustado", "não sei",
	"será que é", "pode ser", "está piorando", "ficou pior",
}

// repetitionMarkers — ruminative patterns suggesting chronic/complex issues
var repetitionMarkers = []string{
	"sempre", "todo dia", "há semanas", "há meses", "faz tempo",
	"ainda", "continua", "não melhora", "não passa", "volta sempre",
}

// ──────────────────────────────────────────────────────────────────────────────
// AssessComplexity
// ──────────────────────────────────────────────────────────────────────────────

// AssessComplexity determines whether a patient message warrants System 2 reasoning.
//
// Scoring rubric:
//   - High-complexity clinical term:  +0.35 each (capped at 0.70)
//   - Medium-complexity health term:  +0.10 each (capped at 0.30)
//   - Urgency phrase:                 +0.20 each (capped at 0.40)
//   - Repetition/chronic marker:      +0.10 each (capped at 0.20)
//   - Message length > 80 chars:      +0.05 (patient is elaborating)
//   - Message length > 200 chars:     +0.10 (patient is very descriptive)
func AssessComplexity(message string) ComplexityAssessment {
	lower := strings.ToLower(message)
	var signals []ComplexitySignal
	total := 0.0

	// ── High-complexity terms ─────────────────────────────────────────────────
	highScore := 0.0
	for _, term := range highComplexityTerms {
		if strings.Contains(lower, term) {
			highScore += 0.35
		}
	}
	if highScore > 0.70 {
		highScore = 0.70
	}
	if highScore > 0 {
		signals = append(signals, ComplexitySignal{Name: "high_clinical_terms", Score: highScore})
		total += highScore
	}

	// ── Medium-complexity terms ───────────────────────────────────────────────
	medScore := 0.0
	for _, term := range mediumComplexityTerms {
		if strings.Contains(lower, term) {
			medScore += 0.10
		}
	}
	if medScore > 0.30 {
		medScore = 0.30
	}
	if medScore > 0 {
		signals = append(signals, ComplexitySignal{Name: "medium_health_terms", Score: medScore})
		total += medScore
	}

	// ── Urgency phrases ───────────────────────────────────────────────────────
	urgScore := 0.0
	for _, phrase := range urgencyPhrases {
		if strings.Contains(lower, phrase) {
			urgScore += 0.20
		}
	}
	if urgScore > 0.40 {
		urgScore = 0.40
	}
	if urgScore > 0 {
		signals = append(signals, ComplexitySignal{Name: "urgency_phrases", Score: urgScore})
		total += urgScore
	}

	// ── Repetition/chronic markers ────────────────────────────────────────────
	repScore := 0.0
	for _, marker := range repetitionMarkers {
		if strings.Contains(lower, marker) {
			repScore += 0.10
		}
	}
	if repScore > 0.20 {
		repScore = 0.20
	}
	if repScore > 0 {
		signals = append(signals, ComplexitySignal{Name: "repetition_markers", Score: repScore})
		total += repScore
	}

	// ── Message length ────────────────────────────────────────────────────────
	charCount := utf8.RuneCountInString(message)
	lenScore := 0.0
	if charCount > 200 {
		lenScore = 0.10
	} else if charCount > 80 {
		lenScore = 0.05
	}
	if lenScore > 0 {
		signals = append(signals, ComplexitySignal{Name: "message_length", Score: lenScore})
		total += lenScore
	}

	// ── Cap at 1.0 ────────────────────────────────────────────────────────────
	if total > 1.0 {
		total = 1.0
	}

	return ComplexityAssessment{
		NeedsSystem2: total >= complexityThreshold,
		Score:        total,
		Signals:      signals,
	}
}

// IsClinicallyComplex is a convenience wrapper for the orchestrator.
func IsClinicallyComplex(message string) bool {
	return AssessComplexity(message).NeedsSystem2
}
