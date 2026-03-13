// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package router

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// AgeGroup represents the developmental stage of the user
type AgeGroup string

const (
	AgeGroupKids   AgeGroup = "kids"   // 4-10 years
	AgeGroupTeens  AgeGroup = "teens"  // 11-19 years
	AgeGroupAdults AgeGroup = "adults" // 20+ years
)

// User represents a user with age information
type User struct {
	ID          string
	Age         int
	GuardianID  string // For minors
	Preferences map[string]interface{}
}

// GetAgeGroup determines the developmental stage based on age
func (u *User) GetAgeGroup() AgeGroup {
	if u.Age >= 4 && u.Age <= 10 {
		return AgeGroupKids
	} else if u.Age >= 11 && u.Age <= 19 {
		return AgeGroupTeens
	}
	return AgeGroupAdults
}

// ── WINNICOTT ENGINE (Holding Environment) ─────────────────────────────────

// HoldingScore represents the quality of the therapeutic holding environment (0-1)
type HoldingScore struct {
	Overall      float64 `json:"overall"`       // Composite score 0-1
	Containment  float64 `json:"containment"`   // Emotional containment quality
	Consistency  float64 `json:"consistency"`    // Interaction regularity over time
	Attunement   float64 `json:"attunement"`     // Response appropriateness
	PlayCapacity float64 `json:"play_capacity"`  // Child's capacity for creative play
	Pattern      string  `json:"pattern"`        // Detected pattern name
}

// WinnicottEngine assesses the holding environment for children (ages 4-10)
// Based on D.W. Winnicott's theory of the facilitating environment
type WinnicottEngine struct {
	// Interaction history for consistency tracking
	interactionTimes []time.Time
	// Sentiment history for containment assessment
	sentimentHistory []float64
}

// NewWinnicottEngine creates a Winnicott holding environment assessor
func NewWinnicottEngine() *WinnicottEngine {
	return &WinnicottEngine{
		interactionTimes: make([]time.Time, 0),
		sentimentHistory: make([]float64, 0),
	}
}

// Analyze performs Winnicott holding environment analysis on child input
func (w *WinnicottEngine) Analyze(input string, user *User) HoldingScore {
	w.interactionTimes = append(w.interactionTimes, time.Now())

	containment := w.measureContainment(input)
	consistency := w.measureConsistency()
	attunement := w.measureAttunement(input, user)
	playCapacity := w.measurePlayCapacity(input)

	w.sentimentHistory = append(w.sentimentHistory, containment)

	overall := containment*0.30 + consistency*0.20 + attunement*0.25 + playCapacity*0.25
	pattern := w.detectPattern(containment, playCapacity, input)

	return HoldingScore{
		Overall:      clamp01(overall),
		Containment:  clamp01(containment),
		Consistency:  clamp01(consistency),
		Attunement:   clamp01(attunement),
		PlayCapacity: clamp01(playCapacity),
		Pattern:      pattern,
	}
}

// measureContainment assesses emotional containment from sentiment in the text.
// High containment = child feels safe to express difficult emotions.
// Low containment = signs of emotional dysregulation or suppression.
func (w *WinnicottEngine) measureContainment(input string) float64 {
	lower := strings.ToLower(input)

	// Distress signals (child NOT contained — needs holding)
	distressWords := []string{
		"medo", "assustado", "sozinho", "ninguém", "chorar", "triste",
		"raiva", "ódio", "não gosto", "quero ir embora", "não quero",
		"dói", "machuca", "feio", "mau", "culpa", "vergonha",
	}
	// Safety signals (child feels contained — good holding)
	safetyWords := []string{
		"gosto", "brincar", "amigo", "mamãe", "papai", "casa",
		"feliz", "contente", "bonito", "legal", "divertido", "abraço",
		"protegido", "seguro", "junto", "amor", "carinho",
	}

	distressCount := 0
	safetyCount := 0
	for _, w := range distressWords {
		if strings.Contains(lower, w) {
			distressCount++
		}
	}
	for _, w := range safetyWords {
		if strings.Contains(lower, w) {
			safetyCount++
		}
	}

	total := distressCount + safetyCount
	if total == 0 {
		return 0.5 // neutral
	}

	// More safety words relative to distress = better containment
	// But distress expression itself is healthy IF the child feels safe enough to express it
	safetyRatio := float64(safetyCount) / float64(total)

	// If child expresses distress AND safety, containment is actually good
	// (they feel safe enough to share difficult feelings)
	if distressCount > 0 && safetyCount > 0 {
		return 0.7 + 0.3*safetyRatio
	}

	// Pure distress without any safety anchors = low containment
	if distressCount > 0 && safetyCount == 0 {
		return math.Max(0.1, 0.4-float64(distressCount)*0.05)
	}

	// Pure safety = good but maybe superficial
	return 0.5 + 0.4*safetyRatio
}

// measureConsistency evaluates regularity of interactions over time
func (w *WinnicottEngine) measureConsistency() float64 {
	n := len(w.interactionTimes)
	if n < 2 {
		return 0.5 // Not enough data yet
	}

	// Calculate intervals between interactions
	var intervals []float64
	for i := 1; i < n; i++ {
		gap := w.interactionTimes[i].Sub(w.interactionTimes[i-1]).Hours()
		intervals = append(intervals, gap)
	}

	// Calculate coefficient of variation (lower = more consistent)
	mean := 0.0
	for _, iv := range intervals {
		mean += iv
	}
	mean /= float64(len(intervals))

	if mean == 0 {
		return 1.0
	}

	variance := 0.0
	for _, iv := range intervals {
		diff := iv - mean
		variance += diff * diff
	}
	variance /= float64(len(intervals))
	stddev := math.Sqrt(variance)
	cv := stddev / mean // coefficient of variation

	// CV of 0 = perfectly consistent = score 1.0
	// CV of 2+ = very inconsistent = score ~0.1
	return clamp01(1.0 - cv/2.0)
}

// measureAttunement assesses response appropriateness based on child's emotional state
func (w *WinnicottEngine) measureAttunement(input string, user *User) float64 {
	lower := strings.ToLower(input)
	score := 0.5

	// Check if the conversation shows signs of being heard/understood
	validationMarkers := []string{
		"sim", "entendo", "obrigado", "legal", "fixe", "tá bem",
		"gosto de falar", "pode ser", "ok", "está bem",
	}
	for _, marker := range validationMarkers {
		if strings.Contains(lower, marker) {
			score += 0.1
		}
	}

	// Signs of NOT being attuned to (child feels misunderstood)
	misattuneMarkers := []string{
		"você não entende", "ninguém entende", "não é isso",
		"deixa", "tanto faz", "não importa", "esquece",
	}
	for _, marker := range misattuneMarkers {
		if strings.Contains(lower, marker) {
			score -= 0.15
		}
	}

	// Age-appropriate language check (very young children use simpler sentences)
	if user != nil && user.Age <= 7 {
		words := strings.Fields(input)
		avgWordLen := 0.0
		for _, w := range words {
			avgWordLen += float64(len(w))
		}
		if len(words) > 0 {
			avgWordLen /= float64(len(words))
		}
		// Short words from young child = natural, score is neutral
		// Very long complex words from young child = might be parroting/performing
		if avgWordLen > 8 {
			score -= 0.1
		}
	}

	return score
}

// measurePlayCapacity assesses the child's capacity for creative/transitional play
func (w *WinnicottEngine) measurePlayCapacity(input string) float64 {
	lower := strings.ToLower(input)

	playIndicators := []string{
		"brincar", "brincadeira", "faz de conta", "imaginar", "imaginação",
		"desenhar", "pintar", "história", "jogo", "jogar",
		"boneco", "bola", "parque", "correr", "pular",
		"inventar", "criar", "construir", "fingir", "sonhar",
	}

	inhibitionIndicators := []string{
		"chato", "não quero brincar", "cansado", "preguiça",
		"não posso", "proibido", "castigo", "errado", "feio",
		"parado", "quieto", "sentado",
	}

	playCount := 0
	inhibCount := 0
	for _, p := range playIndicators {
		if strings.Contains(lower, p) {
			playCount++
		}
	}
	for _, i := range inhibitionIndicators {
		if strings.Contains(lower, i) {
			inhibCount++
		}
	}

	total := playCount + inhibCount
	if total == 0 {
		return 0.5
	}

	return clamp01(float64(playCount) / float64(total))
}

// detectPattern identifies the primary Winnicottian pattern
func (w *WinnicottEngine) detectPattern(containment, playCapacity float64, input string) string {
	lower := strings.ToLower(input)

	// Transitional object attachment
	attachmentWords := []string{"ursinho", "cobertor", "boneco", "brinquedo favorito", "meu", "sempre levo"}
	for _, word := range attachmentWords {
		if strings.Contains(lower, word) {
			return "transitional_object"
		}
	}

	// Separation anxiety
	separationWords := []string{"não vai embora", "fica comigo", "volta", "saudade", "mamãe", "sozinho"}
	sepCount := 0
	for _, word := range separationWords {
		if strings.Contains(lower, word) {
			sepCount++
		}
	}
	if sepCount >= 2 {
		return "separation_anxiety"
	}

	if playCapacity > 0.7 {
		return "creative_play"
	}
	if containment < 0.3 {
		return "holding_deficit"
	}
	if containment > 0.7 && playCapacity > 0.5 {
		return "good_enough_environment"
	}

	return "developing"
}

// ── ERIKSON ENGINE (Psychosocial Stages) ────────────────────────────────────

// EriksonStage represents one of Erikson's 8 psychosocial stages
type EriksonStage struct {
	Number      int     `json:"number"`       // 1-8
	Name        string  `json:"name"`         // Stage name
	AgeRange    string  `json:"age_range"`    // Typical age range
	Virtue      string  `json:"virtue"`       // Strength gained if resolved
	Crisis      string  `json:"crisis"`       // The central conflict
	Positive    string  `json:"positive"`     // Positive pole
	Negative    string  `json:"negative"`     // Negative pole
	Resolution  float64 `json:"resolution"`   // How well the crisis is being resolved (0-1)
	Confidence  float64 `json:"confidence"`   // Confidence in stage detection
	Indicators  []string `json:"indicators"`  // Detected thematic indicators
}

// EriksonEngine detects psychosocial developmental stage and crisis resolution
type EriksonEngine struct{}

// NewEriksonEngine creates a new Erikson psychosocial stage assessor
func NewEriksonEngine() *EriksonEngine {
	return &EriksonEngine{}
}

// eriksonStageDefinitions holds the 8 stages with their associated themes
var eriksonStageDefinitions = []struct {
	Number       int
	Name         string
	AgeRange     string
	AgeMin       int
	AgeMax       int
	Virtue       string
	Crisis       string
	Positive     string
	Negative     string
	PositiveKeys []string
	NegativeKeys []string
}{
	{1, "Trust vs Mistrust", "0-1", 0, 1, "Hope", "Can I trust the world?",
		"Trust", "Mistrust",
		[]string{"confiança", "seguro", "protegido", "cuidado", "amor"},
		[]string{"medo", "abandono", "inseguro", "desconfiança", "perigo"}},
	{2, "Autonomy vs Shame/Doubt", "2-3", 2, 3, "Will", "Is it okay to be me?",
		"Autonomy", "Shame",
		[]string{"sozinho", "eu consigo", "meu", "independente", "escolher"},
		[]string{"vergonha", "errado", "não consigo", "culpa", "dúvida"}},
	{3, "Initiative vs Guilt", "4-5", 4, 5, "Purpose", "Is it okay for me to do, move, and act?",
		"Initiative", "Guilt",
		[]string{"quero fazer", "inventar", "liderar", "plano", "ideia", "criar"},
		[]string{"culpa", "errado", "desculpa", "não devia", "proibido"}},
	{4, "Industry vs Inferiority", "6-11", 6, 11, "Competence", "Can I make it in the world?",
		"Industry", "Inferiority",
		[]string{"consegui", "aprendi", "escola", "nota", "orgulho", "competir", "ganhar"},
		[]string{"burro", "não consigo", "fracasso", "inferior", "pior", "difícil"}},
	{5, "Identity vs Role Confusion", "12-19", 12, 19, "Fidelity", "Who am I? Who can I be?",
		"Identity", "Role Confusion",
		[]string{"eu sou", "quero ser", "meu estilo", "diferente", "grupo", "valores", "futuro"},
		[]string{"confuso", "não sei quem sou", "pressão", "encaixar", "igual", "perdido"}},
	{6, "Intimacy vs Isolation", "20-39", 20, 39, "Love", "Can I love?",
		"Intimacy", "Isolation",
		[]string{"amor", "relação", "parceiro", "compromisso", "juntos", "intimidade", "casamento"},
		[]string{"sozinho", "isolado", "medo de amar", "abandonado", "distante", "solidão"}},
	{7, "Generativity vs Stagnation", "40-64", 40, 64, "Care", "Can I make my life count?",
		"Generativity", "Stagnation",
		[]string{"filhos", "ensinar", "legado", "contribuir", "comunidade", "mentor", "criar"},
		[]string{"estagnado", "preso", "sem propósito", "inútil", "vazio", "rotina"}},
	{8, "Integrity vs Despair", "65+", 65, 120, "Wisdom", "Is it okay to have been me?",
		"Integrity", "Despair",
		[]string{"satisfeito", "vida boa", "sabedoria", "aceitar", "paz", "gratidão", "memórias"},
		[]string{"arrependimento", "tempo perdido", "desespero", "tarde demais", "inútil", "morte"}},
}

// Analyze performs Erikson psychosocial stage analysis
func (e *EriksonEngine) Analyze(input string, user *User) EriksonStage {
	// 1. Determine primary stage from age
	primaryStage := e.stageFromAge(user.Age)

	// 2. Detect thematic indicators in text
	lower := strings.ToLower(input)
	positiveHits := 0
	negativeHits := 0
	var indicators []string

	def := eriksonStageDefinitions[primaryStage-1]

	for _, key := range def.PositiveKeys {
		if strings.Contains(lower, key) {
			positiveHits++
			indicators = append(indicators, "+"+key)
		}
	}
	for _, key := range def.NegativeKeys {
		if strings.Contains(lower, key) {
			negativeHits++
			indicators = append(indicators, "-"+key)
		}
	}

	// 3. Also check adjacent stages (people can regress or advance)
	adjacentConfidence := 0.0
	adjacentStage := primaryStage
	for _, adjIdx := range []int{primaryStage - 2, primaryStage} { // previous and next
		if adjIdx < 0 || adjIdx >= len(eriksonStageDefinitions) {
			continue
		}
		adjDef := eriksonStageDefinitions[adjIdx]
		adjScore := 0
		for _, key := range adjDef.PositiveKeys {
			if strings.Contains(lower, key) {
				adjScore++
			}
		}
		for _, key := range adjDef.NegativeKeys {
			if strings.Contains(lower, key) {
				adjScore++
			}
		}
		adjConf := float64(adjScore) / float64(len(adjDef.PositiveKeys)+len(adjDef.NegativeKeys))
		if adjConf > adjacentConfidence {
			adjacentConfidence = adjConf
			adjacentStage = adjIdx + 1
		}
	}

	// 4. Calculate resolution score
	totalHits := positiveHits + negativeHits
	resolution := 0.5 // default neutral
	if totalHits > 0 {
		resolution = float64(positiveHits) / float64(totalHits)
	}

	// 5. Calculate confidence in primary stage vs adjacent
	primaryConfidence := 0.6 // base confidence from age match
	if totalHits > 0 {
		primaryConfidence += 0.4 * (float64(totalHits) / float64(len(def.PositiveKeys)+len(def.NegativeKeys)))
	}
	// If adjacent stage has stronger signal, note it but keep primary
	if adjacentConfidence > primaryConfidence*0.8 && adjacentStage != primaryStage {
		indicators = append(indicators, fmt.Sprintf("adjacent_stage_%d_signal", adjacentStage))
	}

	return EriksonStage{
		Number:     def.Number,
		Name:       def.Name,
		AgeRange:   def.AgeRange,
		Virtue:     def.Virtue,
		Crisis:     def.Crisis,
		Positive:   def.Positive,
		Negative:   def.Negative,
		Resolution: clamp01(resolution),
		Confidence: clamp01(primaryConfidence),
		Indicators: indicators,
	}
}

// stageFromAge maps age to Erikson's stage number (1-8)
func (e *EriksonEngine) stageFromAge(age int) int {
	for _, def := range eriksonStageDefinitions {
		if age >= def.AgeMin && age <= def.AgeMax {
			return def.Number
		}
	}
	return 8 // default to final stage for very old
}

// ── LACAN ENGINE (Symbolic Analysis) ────────────────────────────────────────

// LacanAnalysis represents structured Lacanian symbolic analysis
type LacanAnalysis struct {
	MasterSignifiers []MasterSignifier `json:"master_signifiers"` // S1 - recurring anchoring themes
	DemandVsDesire   DemandDesire      `json:"demand_vs_desire"`  // What they ask for vs what they need
	Register         string            `json:"register"`          // Dominant register: Imaginary/Symbolic/Real
	SubjectPosition  string            `json:"subject_position"`  // Hysteric/Obsessional/Psychotic/Perverse
	Confidence       float64           `json:"confidence"`
}

// MasterSignifier represents a S1 (signifiant-maitre) — a word/theme that anchors the subject's discourse
type MasterSignifier struct {
	Word            string  `json:"word"`
	EmotionalCharge float64 `json:"emotional_charge"` // 0-1
	IsRepressed     bool    `json:"is_repressed"`     // Appears through negation/avoidance
}

// DemandDesire distinguishes explicit demand from latent desire
type DemandDesire struct {
	ExplicitDemand string `json:"explicit_demand"` // What they literally ask for
	LatentDesire   string `json:"latent_desire"`   // What they actually need (inferred)
	Gap            string `json:"gap"`             // Description of the gap between demand and desire
}

// LacanEngine performs structured symbolic analysis for adult patients
type LacanEngine struct{}

// NewLacanEngine creates a new Lacan symbolic analysis engine
func NewLacanEngine() *LacanEngine {
	return &LacanEngine{}
}

// Analyze performs Lacanian symbolic analysis on adult input
func (l *LacanEngine) Analyze(input string) LacanAnalysis {
	lower := strings.ToLower(input)

	signifiers := l.detectMasterSignifiers(lower)
	demandDesire := l.analyzeDemandVsDesire(lower)
	register := l.detectDominantRegister(lower)
	position := l.detectSubjectPosition(lower)

	confidence := 0.3 // base
	if len(signifiers) > 0 {
		confidence += 0.2
	}
	if demandDesire.LatentDesire != "" {
		confidence += 0.2
	}
	if register != "indefinido" {
		confidence += 0.15
	}
	if position != "indefinido" {
		confidence += 0.15
	}

	return LacanAnalysis{
		MasterSignifiers: signifiers,
		DemandVsDesire:   demandDesire,
		Register:         register,
		SubjectPosition:  position,
		Confidence:       clamp01(confidence),
	}
}

// detectMasterSignifiers finds recurring emotionally-charged words that function as S1
func (l *LacanEngine) detectMasterSignifiers(input string) []MasterSignifier {
	// Emotional keyword categories with charge levels
	signifierMap := map[string]float64{
		// High charge (0.9-1.0)
		"morte": 1.0, "abandono": 1.0, "solidão": 0.95, "desespero": 0.95,
		"ódio": 0.9, "culpa": 0.9, "vazio": 0.9, "perda": 0.9, "trauma": 1.0,
		// Medium-high charge (0.7-0.85)
		"medo": 0.85, "raiva": 0.8, "tristeza": 0.8, "angústia": 0.85,
		"ansiedade": 0.8, "dor": 0.75, "sofrimento": 0.8, "vergonha": 0.75,
		"saudade": 0.7, "falta": 0.7,
		// Medium charge (0.5-0.65)
		"família": 0.6, "filho": 0.6, "filha": 0.6, "pai": 0.65, "mãe": 0.65,
		"amor": 0.6, "vida": 0.55, "trabalho": 0.5, "casa": 0.5,
		// Relational (0.5-0.7)
		"esposa": 0.55, "marido": 0.55, "namorado": 0.55, "namorada": 0.55,
	}

	words := strings.Fields(input)
	var signifiers []MasterSignifier

	found := make(map[string]bool)
	for _, word := range words {
		cleaned := strings.Trim(word, ".,!?;:\"'()")
		if charge, ok := signifierMap[cleaned]; ok && !found[cleaned] {
			found[cleaned] = true
			signifiers = append(signifiers, MasterSignifier{
				Word:            cleaned,
				EmotionalCharge: charge,
				IsRepressed:     false,
			})
		}
	}

	// Detect repressed signifiers (appear through negation: "nao tenho medo" = medo IS the S1)
	negationPatterns := []struct {
		pattern string
		word    string
	}{
		{"não tenho medo", "medo"},
		{"não sinto falta", "falta"},
		{"não me importo", "importar"},
		{"não estou triste", "tristeza"},
		{"sem problemas", "problemas"},
		{"não é nada", "nada"},
		{"não preciso", "necessidade"},
		{"não me afeta", "afeto"},
		{"tudo bem", "negação_global"},
	}

	for _, np := range negationPatterns {
		if strings.Contains(input, np.pattern) {
			// Denegation (Verneinung): negating something reveals it as the repressed S1
			if !found[np.word] {
				found[np.word] = true
				signifiers = append(signifiers, MasterSignifier{
					Word:            np.word,
					EmotionalCharge: 0.85, // Repressed signifiers carry high charge
					IsRepressed:     true,
				})
			}
		}
	}

	return signifiers
}

// analyzeDemandVsDesire distinguishes what the subject explicitly asks for from what they latently desire
func (l *LacanEngine) analyzeDemandVsDesire(input string) DemandDesire {
	dd := DemandDesire{}

	// Detect explicit demands (surface-level requests)
	demandPatterns := []struct {
		pattern string
		demand  string
	}{
		{"quero", "obter algo específico"},
		{"preciso", "suprir uma necessidade"},
		{"me ajuda", "receber ajuda concreta"},
		{"não aguento", "alívio do sofrimento"},
		{"quero saber", "obter informação"},
		{"me diz", "receber orientação"},
		{"pode fazer", "ação concreta"},
	}

	for _, dp := range demandPatterns {
		if strings.Contains(input, dp.pattern) {
			dd.ExplicitDemand = dp.demand
			break
		}
	}
	if dd.ExplicitDemand == "" {
		dd.ExplicitDemand = "expressão sem demanda explícita"
	}

	// Infer latent desire from emotional subtext
	// In Lacan: desire is always the desire of the Other — what do they want from the listener?
	desireSignals := map[string]string{
		// Loneliness/abandonment → desire for the Other's presence
		"sozinho":  "ser reconhecido pelo Outro",
		"solidão":  "ser reconhecido pelo Outro",
		"abandono": "ser reconhecido pelo Outro",
		"ninguém":  "ser reconhecido pelo Outro",
		// Anger/frustration → desire for mastery/agency
		"raiva":         "recuperar agência sobre a própria vida",
		"ódio":          "recuperar agência sobre a própria vida",
		"não aguento":   "recuperar agência sobre a própria vida",
		"não consigo":   "recuperar agência sobre a própria vida",
		// Fear/anxiety → desire for symbolic anchoring
		"medo":      "encontrar um ponto de ancoragem simbólica",
		"ansiedade": "encontrar um ponto de ancoragem simbólica",
		"angústia":  "encontrar um ponto de ancoragem simbólica",
		// Loss/grief → desire to symbolize the unspeakable
		"perda":    "simbolizar o que não pode ser dito",
		"saudade":  "simbolizar o que não pode ser dito",
		"falta":    "simbolizar o que não pode ser dito",
		"vazio":    "simbolizar o que não pode ser dito",
		// Identity → desire for recognition in the Symbolic order
		"quem sou":       "inscrever-se na ordem simbólica",
		"não sei":        "inscrever-se na ordem simbólica",
		"confuso":        "inscrever-se na ordem simbólica",
		"perdido":        "inscrever-se na ordem simbólica",
	}

	for signal, desire := range desireSignals {
		if strings.Contains(input, signal) {
			dd.LatentDesire = desire
			break
		}
	}
	if dd.LatentDesire == "" {
		dd.LatentDesire = "desejo não articulado"
	}

	// Describe the gap
	if dd.ExplicitDemand != "expressão sem demanda explícita" && dd.LatentDesire != "desejo não articulado" {
		dd.Gap = fmt.Sprintf("O sujeito demanda '%s' mas o desejo latente aponta para '%s'",
			dd.ExplicitDemand, dd.LatentDesire)
	} else {
		dd.Gap = "gap não determinável com os dados disponíveis"
	}

	return dd
}

// detectDominantRegister identifies which Lacanian register dominates: Imaginary, Symbolic, or Real
func (l *LacanEngine) detectDominantRegister(input string) string {
	imaginaryScore := 0
	symbolicScore := 0
	realScore := 0

	// Imaginary: mirror, image, appearance, comparison, ego
	imaginaryWords := []string{
		"pareço", "imagem", "espelho", "bonito", "feio", "gordo", "magro",
		"melhor que", "pior que", "igual", "comparar", "aparência", "olhar",
		"selfie", "foto", "como os outros", "normal",
	}
	// Symbolic: law, rules, language, names, structure, father
	symbolicWords := []string{
		"lei", "regra", "palavra", "nome", "pai", "autoridade", "dever",
		"promessa", "contrato", "acordo", "proibido", "permitido", "certo",
		"errado", "verdade", "mentira", "significado",
	}
	// Real: trauma, impossible, unspeakable, body, repetition
	realWords := []string{
		"não consigo explicar", "sem palavras", "corpo", "dor física",
		"pesadelo", "repete", "sempre igual", "impossível", "insuportável",
		"não tem nome", "horrível", "trauma", "pânico",
	}

	for _, w := range imaginaryWords {
		if strings.Contains(input, w) {
			imaginaryScore++
		}
	}
	for _, w := range symbolicWords {
		if strings.Contains(input, w) {
			symbolicScore++
		}
	}
	for _, w := range realWords {
		if strings.Contains(input, w) {
			realScore++
		}
	}

	maxScore := max3(imaginaryScore, symbolicScore, realScore)
	if maxScore == 0 {
		return "indefinido"
	}

	switch maxScore {
	case imaginaryScore:
		return "Imaginário"
	case symbolicScore:
		return "Simbólico"
	default:
		return "Real"
	}
}

// detectSubjectPosition identifies the clinical structure position
func (l *LacanEngine) detectSubjectPosition(input string) string {
	// Hysteric: questions addressed to the Other, "why?", dramatic affect
	hystericWords := []string{"por que", "por quê", "como pode", "injusto", "dramático", "não é justo"}
	// Obsessional: control, doubt, ritual, thinking
	obsessionalWords := []string{"tenho que", "devo", "controle", "certeza", "dúvida", "pensar", "organizar", "correto"}
	// Note: Psychotic/Perverse positions are clinical diagnoses — we only flag strong signals
	psychoticWords := []string{"vozes", "perseguição", "conspiração", "mensagem secreta"}

	hystericScore := 0
	obsessionalScore := 0
	psychoticScore := 0

	for _, w := range hystericWords {
		if strings.Contains(input, w) {
			hystericScore++
		}
	}
	for _, w := range obsessionalWords {
		if strings.Contains(input, w) {
			obsessionalScore++
		}
	}
	for _, w := range psychoticWords {
		if strings.Contains(input, w) {
			psychoticScore++
		}
	}

	maxScore := max3(hystericScore, obsessionalScore, psychoticScore)
	if maxScore == 0 {
		return "indefinido"
	}

	switch maxScore {
	case hystericScore:
		return "Histérico"
	case obsessionalScore:
		return "Obsessivo"
	case psychoticScore:
		return "Psicótico (flag clínico — requer avaliação)"
	default:
		return "indefinido"
	}
}

// ── DEVELOPMENTAL ROUTER (Updated) ─────────────────────────────────────────

// DevelopmentalRouter routes interventions based on developmental stage
type DevelopmentalRouter struct {
	winnicottEngine *WinnicottEngine
	eriksonEngine   *EriksonEngine
	lacanEngine     *LacanEngine

	// Vector DB client
	vectorClient interface{} // Placeholder (NietzscheDB vector)
}

// NewDevelopmentalRouter creates a new developmental router
func NewDevelopmentalRouter() *DevelopmentalRouter {
	return &DevelopmentalRouter{
		winnicottEngine: NewWinnicottEngine(),
		eriksonEngine:   NewEriksonEngine(),
		lacanEngine:     NewLacanEngine(),
	}
}

// AnalysisResult represents the output of psychological analysis
type AnalysisResult struct {
	Vector     []float64
	Confidence float64
	Pattern    string
	AgeGroup   AgeGroup
}

// Intervention represents a therapeutic intervention
type Intervention struct {
	ID              string
	Title           string
	Content         string
	TargetAudience  []AgeGroup
	VoiceSettings   VoiceSettings
	MoralAdaptation map[AgeGroup]string
}

// VoiceSettings represents TTS configuration
type VoiceSettings struct {
	SpeakingRate float64
	Pitch        float64
	Tone         string
}

// SelectIntervention chooses the appropriate intervention based on age and input
func (r *DevelopmentalRouter) SelectIntervention(user *User, input string) (*Intervention, error) {
	// 1. Determine age group
	ageGroup := user.GetAgeGroup()

	// 2. Perform age-appropriate psychological analysis
	var analysis AnalysisResult

	switch ageGroup {
	case AgeGroupKids:
		// Winnicott analysis: focus on play, holding, fear of abandonment
		analysis = r.analyzeKids(input)

	case AgeGroupTeens:
		// Erikson analysis: focus on identity, peer pressure, autonomy
		analysis = r.analyzeTeens(input)

	case AgeGroupAdults:
		// Lacan + Gurdjieff analysis: full system
		analysis = r.analyzeAdults(input)
	}

	// 3. Search NietzscheDB vector with age filter
	intervention, err := r.searchVector(analysis, ageGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to search interventions: %w", err)
	}

	// 4. Adapt response for age group
	return r.adaptIntervention(intervention, ageGroup), nil
}

// analyzeKids performs Winnicott-based analysis for children
func (r *DevelopmentalRouter) analyzeKids(input string) AnalysisResult {
	return r.analyzeKidsForUser(input, nil)
}

// analyzeKidsForUser performs Winnicott analysis with user context
func (r *DevelopmentalRouter) analyzeKidsForUser(input string, user *User) AnalysisResult {
	holding := r.winnicottEngine.Analyze(input, user)

	// Build a feature vector from the holding scores
	// First 4 dimensions encode the holding environment, rest zeroed for vector search compatibility
	vector := make([]float64, 768)
	vector[0] = holding.Containment
	vector[1] = holding.Consistency
	vector[2] = holding.Attunement
	vector[3] = holding.PlayCapacity

	return AnalysisResult{
		Vector:     vector,
		Confidence: holding.Overall,
		Pattern:    holding.Pattern,
		AgeGroup:   AgeGroupKids,
	}
}

// analyzeTeens performs Erikson-based analysis for adolescents
func (r *DevelopmentalRouter) analyzeTeens(input string) AnalysisResult {
	return r.analyzeTeensForUser(input, nil)
}

// analyzeTeensForUser performs Erikson analysis with user context
func (r *DevelopmentalRouter) analyzeTeensForUser(input string, user *User) AnalysisResult {
	if user == nil {
		user = &User{Age: 15} // default teen age if unknown
	}

	stage := r.eriksonEngine.Analyze(input, user)

	// Build feature vector: encode stage info for vector search
	vector := make([]float64, 768)
	vector[0] = float64(stage.Number) / 8.0 // Normalized stage number
	vector[1] = stage.Resolution             // Crisis resolution score
	vector[2] = stage.Confidence             // Detection confidence

	pattern := fmt.Sprintf("stage_%d_%s", stage.Number, strings.ReplaceAll(
		strings.ToLower(stage.Name), " ", "_"))

	return AnalysisResult{
		Vector:     vector,
		Confidence: stage.Confidence,
		Pattern:    pattern,
		AgeGroup:   AgeGroupTeens,
	}
}

// analyzeAdults performs Lacan-based analysis for adults
func (r *DevelopmentalRouter) analyzeAdults(input string) AnalysisResult {
	analysis := r.lacanEngine.Analyze(input)

	// Build feature vector from Lacanian analysis
	vector := make([]float64, 768)

	// Encode master signifiers into vector dimensions
	for i, sig := range analysis.MasterSignifiers {
		if i >= 10 {
			break // max 10 signifier slots
		}
		vector[i] = sig.EmotionalCharge
		if sig.IsRepressed {
			vector[10+i] = 1.0 // repression flag
		}
	}

	// Encode register as numeric
	switch analysis.Register {
	case "Imaginário":
		vector[20] = 1.0
	case "Simbólico":
		vector[21] = 1.0
	case "Real":
		vector[22] = 1.0
	}

	// Encode subject position
	switch analysis.SubjectPosition {
	case "Histérico":
		vector[23] = 1.0
	case "Obsessivo":
		vector[24] = 1.0
	}

	// Build pattern string
	pattern := fmt.Sprintf("register_%s_position_%s", analysis.Register, analysis.SubjectPosition)
	if len(analysis.MasterSignifiers) > 0 {
		pattern += "_s1_" + analysis.MasterSignifiers[0].Word
	}

	return AnalysisResult{
		Vector:     vector,
		Confidence: analysis.Confidence,
		Pattern:    pattern,
		AgeGroup:   AgeGroupAdults,
	}
}

// searchVector searches for interventions with age filtering
func (r *DevelopmentalRouter) searchVector(analysis AnalysisResult, ageGroup AgeGroup) (*Intervention, error) {
	// TODO: Implement NietzscheDB vector search with filter
	// Filter: target_audience must include ageGroup

	return &Intervention{
		ID:             "placeholder",
		Title:          "Placeholder Intervention",
		Content:        "Placeholder content",
		TargetAudience: []AgeGroup{ageGroup},
		VoiceSettings: VoiceSettings{
			SpeakingRate: 1.0,
			Pitch:        0.0,
			Tone:         "neutral",
		},
		MoralAdaptation: make(map[AgeGroup]string),
	}, nil
}

// adaptIntervention adapts the intervention for the specific age group
func (r *DevelopmentalRouter) adaptIntervention(intervention *Intervention, ageGroup AgeGroup) *Intervention {
	// Adapt voice settings based on age
	switch ageGroup {
	case AgeGroupKids:
		intervention.VoiceSettings = VoiceSettings{
			SpeakingRate: 1.0,
			Pitch:        2.0, // Higher pitch
			Tone:         "animated",
		}

	case AgeGroupTeens:
		intervention.VoiceSettings = VoiceSettings{
			SpeakingRate: 1.1, // Slightly faster
			Pitch:        0.0, // Neutral
			Tone:         "casual",
		}

	case AgeGroupAdults:
		intervention.VoiceSettings = VoiceSettings{
			SpeakingRate: 0.9,  // Slower
			Pitch:        -1.5, // Lower pitch
			Tone:         "empathetic",
		}
	}

	return intervention
}

// SelectInterventionFull chooses intervention with full engine analysis results
func (r *DevelopmentalRouter) SelectInterventionFull(user *User, input string) (*Intervention, interface{}, error) {
	ageGroup := user.GetAgeGroup()

	var analysis AnalysisResult
	var engineResult interface{}

	switch ageGroup {
	case AgeGroupKids:
		holding := r.winnicottEngine.Analyze(input, user)
		engineResult = holding
		analysis = r.analyzeKidsForUser(input, user)
	case AgeGroupTeens:
		stage := r.eriksonEngine.Analyze(input, user)
		engineResult = stage
		analysis = r.analyzeTeensForUser(input, user)
	case AgeGroupAdults:
		lacanResult := r.lacanEngine.Analyze(input)
		engineResult = lacanResult
		analysis = r.analyzeAdults(input)
	}

	intervention, err := r.searchVector(analysis, ageGroup)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search interventions: %w", err)
	}

	return r.adaptIntervention(intervention, ageGroup), engineResult, nil
}

// GetWinnicottEngine returns the Winnicott engine for direct access
func (r *DevelopmentalRouter) GetWinnicottEngine() *WinnicottEngine {
	return r.winnicottEngine
}

// GetEriksonEngine returns the Erikson engine for direct access
func (r *DevelopmentalRouter) GetEriksonEngine() *EriksonEngine {
	return r.eriksonEngine
}

// GetLacanEngine returns the Lacan engine for direct access
func (r *DevelopmentalRouter) GetLacanEngine() *LacanEngine {
	return r.lacanEngine
}

// ── HELPER FUNCTIONS ────────────────────────────────────────────────────────

// clamp01 clamps a float64 to [0, 1]
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// max3 returns the maximum of three ints
func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// IsMinor checks if user is under 18
func (u *User) IsMinor() bool {
	return u.Age < 18
}

// RequiresGuardian checks if user requires guardian consent
func (u *User) RequiresGuardian() bool {
	return u.Age < 13 // COPPA requirement
}
