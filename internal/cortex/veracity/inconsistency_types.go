package veracity

import (
	"time"
)

// InconsistencyType tipos de inconsistências detectadas
type InconsistencyType string

const (
	DirectContradiction    InconsistencyType = "direct_contradiction"
	TemporalInconsistency  InconsistencyType = "temporal_inconsistency"
	EmotionalInconsistency InconsistencyType = "emotional_inconsistency"
	NarrativeGap           InconsistencyType = "narrative_gap"
	BehavioralChange       InconsistencyType = "behavioral_change"
)

// Severity níveis de gravidade
type Severity string

const (
	SeverityLow      Severity = "low"      // Memória imprecisa, normal
	SeverityMedium   Severity = "medium"   // Inconsistência notável
	SeverityHigh     Severity = "high"     // Contradição clara
	SeverityCritical Severity = "critical" // Omissão perigosa
)

// Evidence evidência do grafo que contradiz a afirmação
type Evidence struct {
	Fact      string                 // Fato do grafo
	Timestamp time.Time              // Quando foi registrado
	Source    string                 // Query Cypher que encontrou
	Metadata  map[string]interface{} // Dados adicionais
}

// Inconsistency representa uma inconsistência detectada
type Inconsistency struct {
	Type          InconsistencyType
	Confidence    float64    // 0-1: quão certo estamos da inconsistência
	Statement     string     // O que o usuário disse agora
	GraphEvidence []Evidence // Evidências do grafo que contradizem
	Reasoning     string     // Explicação da inconsistência
	Severity      Severity   // Gravidade
	Timestamp     time.Time  // Quando foi detectado
}

// GetDescription retorna descrição humana do tipo
func (t InconsistencyType) GetDescription() string {
	descriptions := map[InconsistencyType]string{
		DirectContradiction:    "Contradição direta com fato registrado",
		TemporalInconsistency:  "Inconsistência temporal (datas não batem)",
		EmotionalInconsistency: "Negação de emoção presente no histórico",
		NarrativeGap:           "Omissão de evento importante",
		BehavioralChange:       "Mudança atípica de comportamento",
	}
	return descriptions[t]
}

// ShouldConfront decide se EVA deve confrontar suavemente
func (i *Inconsistency) ShouldConfront() bool {
	// Confrontar se:
	// 1. Confiança alta (> 0.7)
	// 2. Severidade média ou alta
	// 3. Não é apenas memória imprecisa

	if i.Confidence < 0.7 {
		return false // Baixa confiança - pode ser memória
	}

	if i.Severity == SeverityCritical {
		return true // Sempre confrontar casos críticos
	}

	if i.Severity == SeverityHigh && i.Confidence > 0.8 {
		return true
	}

	return false
}

// GetSeverityScore retorna score numérico de gravidade
func (s Severity) GetSeverityScore() int {
	scores := map[Severity]int{
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}
	return scores[s]
}
