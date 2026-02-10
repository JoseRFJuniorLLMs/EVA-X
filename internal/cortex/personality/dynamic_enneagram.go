package personality

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// DynamicEnneagram implementa personalidade evolutiva como distribuicao probabilistica
// Em vez de tipo fixo, mantém [9]float64 que evolui com interacoes
// Ciencia: Personalidade como sistema dinamico que muda ao longo da vida
type DynamicEnneagram struct {
	distributions map[int64]*PersonalityDistribution // patientID -> distribuicao
	rules         []TransitionRule
	mu            sync.RWMutex
}

// PersonalityDistribution distribuicao sobre os 9 tipos Enneagram
type PersonalityDistribution struct {
	Types     [9]float64            `json:"types"`      // Probabilidade de cada tipo (soma = 1.0)
	History   []PersonalitySnapshot `json:"history"`     // Historico de snapshots
	BaseType  int                   `json:"base_type"`   // Tipo base original (1-9)
	PatientID int64                 `json:"patient_id"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// PersonalitySnapshot snapshot de um momento da personalidade
type PersonalitySnapshot struct {
	Types     [9]float64 `json:"types"`
	Timestamp time.Time  `json:"timestamp"`
	Trigger   string     `json:"trigger"` // O que causou a mudanca
}

// TransitionRule regra de transicao de personalidade
type TransitionRule struct {
	FromType  int     `json:"from_type"`
	ToType    int     `json:"to_type"`
	Trigger   string  `json:"trigger"`   // "grief", "love", "anxiety", "growth", "stress"
	Intensity float64 `json:"intensity"` // 0-1 quanto transfere
}

// PersonalityInsight insight sobre evolucao da personalidade
type PersonalityInsight struct {
	DominantType    int     `json:"dominant_type"`
	DominantWeight  float64 `json:"dominant_weight"`
	SecondaryType   int     `json:"secondary_type"`
	SecondaryWeight float64 `json:"secondary_weight"`
	Stability       float64 `json:"stability"` // 0-1 quao estavel
	Trend           string  `json:"trend"`     // "stable", "transitioning", "volatile"
}

// NewDynamicEnneagram cria o motor de personalidade dinamica
func NewDynamicEnneagram() *DynamicEnneagram {
	de := &DynamicEnneagram{
		distributions: make(map[int64]*PersonalityDistribution),
		rules: []TransitionRule{
			// Regras de Desintegracao (Stress)
			{FromType: 1, ToType: 4, Trigger: "stress", Intensity: 0.15},
			{FromType: 2, ToType: 8, Trigger: "stress", Intensity: 0.15},
			{FromType: 3, ToType: 9, Trigger: "stress", Intensity: 0.15},
			{FromType: 4, ToType: 2, Trigger: "stress", Intensity: 0.15},
			{FromType: 5, ToType: 7, Trigger: "stress", Intensity: 0.15},
			{FromType: 6, ToType: 3, Trigger: "stress", Intensity: 0.15},
			{FromType: 7, ToType: 1, Trigger: "stress", Intensity: 0.15},
			{FromType: 8, ToType: 5, Trigger: "stress", Intensity: 0.15},
			{FromType: 9, ToType: 6, Trigger: "stress", Intensity: 0.15},

			// Regras de Integracao (Growth)
			{FromType: 1, ToType: 7, Trigger: "growth", Intensity: 0.10},
			{FromType: 2, ToType: 4, Trigger: "growth", Intensity: 0.10},
			{FromType: 3, ToType: 6, Trigger: "growth", Intensity: 0.10},
			{FromType: 4, ToType: 1, Trigger: "growth", Intensity: 0.10},
			{FromType: 5, ToType: 8, Trigger: "growth", Intensity: 0.10},
			{FromType: 6, ToType: 9, Trigger: "growth", Intensity: 0.10},
			{FromType: 7, ToType: 5, Trigger: "growth", Intensity: 0.10},
			{FromType: 8, ToType: 2, Trigger: "growth", Intensity: 0.10},
			{FromType: 9, ToType: 3, Trigger: "growth", Intensity: 0.10},

			// Regras emocionais especificas
			{FromType: 2, ToType: 4, Trigger: "grief", Intensity: 0.20},
			{FromType: 6, ToType: 9, Trigger: "love", Intensity: 0.12},
			{FromType: 7, ToType: 5, Trigger: "anxiety", Intensity: 0.18},
			{FromType: 3, ToType: 4, Trigger: "grief", Intensity: 0.15},
			{FromType: 8, ToType: 2, Trigger: "love", Intensity: 0.10},
		},
	}

	return de
}

// InitializePatient inicializa distribuicao para um novo paciente
func (de *DynamicEnneagram) InitializePatient(patientID int64, baseType int) {
	de.mu.Lock()
	defer de.mu.Unlock()

	if baseType < 1 || baseType > 9 {
		baseType = 9 // Default: Tipo 9 (Mediador)
	}

	dist := &PersonalityDistribution{
		BaseType:  baseType,
		PatientID: patientID,
		UpdatedAt: time.Now(),
	}

	// 60% no tipo base, 40% distribuido entre os outros
	remainingWeight := 0.40 / 8.0
	for i := 0; i < 9; i++ {
		if i == baseType-1 {
			dist.Types[i] = 0.60
		} else {
			dist.Types[i] = remainingWeight
		}
	}

	// Snapshot inicial
	dist.History = []PersonalitySnapshot{{
		Types:     dist.Types,
		Timestamp: time.Now(),
		Trigger:   "initialization",
	}}

	de.distributions[patientID] = dist

	log.Printf("[ENNEAGRAM] Paciente %d inicializado: Tipo base %d (%.0f%%)",
		patientID, baseType, dist.Types[baseType-1]*100)
}

// Evolve evolui a personalidade baseado em um evento emocional
func (de *DynamicEnneagram) Evolve(patientID int64, trigger string) error {
	de.mu.Lock()
	defer de.mu.Unlock()

	dist, ok := de.distributions[patientID]
	if !ok {
		return fmt.Errorf("paciente %d nao inicializado no DynamicEnneagram", patientID)
	}

	// Encontrar tipo dominante atual
	dominantType := de.getDominantTypeUnsafe(dist)

	// Aplicar regras de transicao matching
	transitioned := false
	for _, rule := range de.rules {
		if rule.FromType == dominantType && rule.Trigger == trigger {
			de.applyTransition(dist, rule)
			transitioned = true
		}
	}

	if !transitioned {
		return nil // Sem regra aplicavel
	}

	// Snapshot
	dist.History = append(dist.History, PersonalitySnapshot{
		Types:     dist.Types,
		Timestamp: time.Now(),
		Trigger:   trigger,
	})

	// Limitar historico a ultimos 100 snapshots
	if len(dist.History) > 100 {
		dist.History = dist.History[len(dist.History)-100:]
	}

	dist.UpdatedAt = time.Now()

	newDominant := de.getDominantTypeUnsafe(dist)
	log.Printf("[ENNEAGRAM] Paciente %d evoluiu: trigger='%s', tipo %d(%.0f%%) -> %d(%.0f%%)",
		patientID, trigger, dominantType, dist.Types[dominantType-1]*100,
		newDominant, dist.Types[newDominant-1]*100)

	return nil
}

// applyTransition aplica uma regra de transicao
func (de *DynamicEnneagram) applyTransition(dist *PersonalityDistribution, rule TransitionRule) {
	fromIdx := rule.FromType - 1
	toIdx := rule.ToType - 1

	if fromIdx < 0 || fromIdx >= 9 || toIdx < 0 || toIdx >= 9 {
		return
	}

	// Transfere probabilidade do tipo de origem para o destino
	transfer := dist.Types[fromIdx] * rule.Intensity
	dist.Types[fromIdx] -= transfer
	dist.Types[toIdx] += transfer

	// Normalizar (soma = 1.0)
	de.normalizeDistribution(dist)
}

// normalizeDistribution garante que soma = 1.0 e nenhum valor < 0.01
func (de *DynamicEnneagram) normalizeDistribution(dist *PersonalityDistribution) {
	sum := 0.0
	for i := range dist.Types {
		if dist.Types[i] < 0.01 {
			dist.Types[i] = 0.01 // Minimo 1%
		}
		sum += dist.Types[i]
	}

	if sum > 0 {
		for i := range dist.Types {
			dist.Types[i] /= sum
		}
	}
}

// GetInsight retorna insight sobre a personalidade de um paciente
func (de *DynamicEnneagram) GetInsight(patientID int64) (*PersonalityInsight, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	dist, ok := de.distributions[patientID]
	if !ok {
		return nil, fmt.Errorf("paciente %d nao inicializado", patientID)
	}

	dominant := de.getDominantTypeUnsafe(dist)
	secondary := de.getSecondaryTypeUnsafe(dist)

	// Calcular estabilidade (entropia inversa: alta concentracao = estavel)
	entropy := 0.0
	for _, p := range dist.Types {
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	maxEntropy := math.Log2(9.0) // Entropia maxima (distribuicao uniforme)
	stability := 1.0 - (entropy / maxEntropy)

	// Determinar tendencia
	trend := "stable"
	if len(dist.History) >= 5 {
		recent := dist.History[len(dist.History)-5:]
		changes := 0
		for i := 1; i < len(recent); i++ {
			prevDom := de.getDominantTypeFromSnapshot(recent[i-1])
			currDom := de.getDominantTypeFromSnapshot(recent[i])
			if prevDom != currDom {
				changes++
			}
		}
		if changes >= 3 {
			trend = "volatile"
		} else if changes >= 1 {
			trend = "transitioning"
		}
	}

	return &PersonalityInsight{
		DominantType:    dominant,
		DominantWeight:  dist.Types[dominant-1],
		SecondaryType:   secondary,
		SecondaryWeight: dist.Types[secondary-1],
		Stability:       stability,
		Trend:           trend,
	}, nil
}

// GetDistribution retorna a distribuicao completa de um paciente
func (de *DynamicEnneagram) GetDistribution(patientID int64) (*PersonalityDistribution, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	dist, ok := de.distributions[patientID]
	if !ok {
		return nil, fmt.Errorf("paciente %d nao inicializado", patientID)
	}

	// Copia para evitar race conditions
	copy := *dist
	return &copy, nil
}

// GetDominantType retorna o tipo dominante atual
func (de *DynamicEnneagram) GetDominantType(patientID int64) (int, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	dist, ok := de.distributions[patientID]
	if !ok {
		return 0, fmt.Errorf("paciente %d nao inicializado", patientID)
	}

	return de.getDominantTypeUnsafe(dist), nil
}

// BlendVoice retorna uma mistura de vozes de personalidade
func (de *DynamicEnneagram) BlendVoice(patientID int64) (map[int]float64, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	dist, ok := de.distributions[patientID]
	if !ok {
		return nil, fmt.Errorf("paciente %d nao inicializado", patientID)
	}

	// Retornar tipos com peso > 10%
	blend := make(map[int]float64)
	for i, weight := range dist.Types {
		if weight > 0.10 {
			blend[i+1] = weight
		}
	}

	return blend, nil
}

// getDominantTypeUnsafe retorna tipo com maior peso (chamador deve ter lock)
func (de *DynamicEnneagram) getDominantTypeUnsafe(dist *PersonalityDistribution) int {
	maxWeight := -1.0
	dominant := 1
	for i, w := range dist.Types {
		if w > maxWeight {
			maxWeight = w
			dominant = i + 1
		}
	}
	return dominant
}

// getSecondaryTypeUnsafe retorna segundo tipo com maior peso
func (de *DynamicEnneagram) getSecondaryTypeUnsafe(dist *PersonalityDistribution) int {
	dominant := de.getDominantTypeUnsafe(dist)

	maxWeight := -1.0
	secondary := 1
	for i, w := range dist.Types {
		if i+1 != dominant && w > maxWeight {
			maxWeight = w
			secondary = i + 1
		}
	}
	return secondary
}

func (de *DynamicEnneagram) getDominantTypeFromSnapshot(snap PersonalitySnapshot) int {
	maxWeight := -1.0
	dominant := 1
	for i, w := range snap.Types {
		if w > maxWeight {
			maxWeight = w
			dominant = i + 1
		}
	}
	return dominant
}

// GetStatistics retorna estatisticas do motor
func (de *DynamicEnneagram) GetStatistics() map[string]interface{} {
	de.mu.RLock()
	defer de.mu.RUnlock()

	return map[string]interface{}{
		"engine":          "dynamic_enneagram",
		"patients_loaded": len(de.distributions),
		"transition_rules": len(de.rules),
		"status":          "active",
	}
}
