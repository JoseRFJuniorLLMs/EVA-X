package transnar

import (
	"strings"
	"unicode"
)

// SignifierChain representa a cadeia de significantes extraída de uma fala
type SignifierChain struct {
	Words           []string // Palavras principais
	Emotions        []string // Emoções detectadas
	Negations       []string // Negações ("não", "nunca", "jamais")
	Modals          []string // Modais ("quero", "devo", "preciso")
	Intensity       float64  // Intensidade emocional (0.0 a 1.0)
	RawText         string   // Texto original
	TemporalMarkers []string // NEW: Marcadores temporais
	Conditionals    []string // NEW: Condicionais (se, talvez)
	HasDiminisher   bool     // NEW: Tem diminuidor (pouco, meio)
	ContextScore    float64  // NEW: Score de contexto (0-1)
}

// Analyzer extrai a cadeia significante de um texto
type Analyzer struct {
	stopwords       map[string]bool
	emotionWords    map[string]string
	negationWords   map[string]bool
	modalWords      map[string]bool
	intensifiers    map[string]float64
	temporalMarkers map[string]string  // NEW
	conditionals    map[string]bool    // NEW
	diminishers     map[string]float64 // NEW
}

// NewAnalyzer cria um novo analisador de cadeia significante
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		stopwords: map[string]bool{
			"o": true, "a": true, "de": true, "que": true, "e": true,
			"do": true, "da": true, "em": true, "um": true, "para": true,
			"com": true, "uma": true, "os": true, "no": true, "se": true,
			"na": true, "por": true, "mais": true, "as": true, "dos": true,
			"como": true, "mas": true, "foi": true, "ao": true, "ele": true,
			"das": true, "tem": true, "à": true, "seu": true, "sua": true,
			"ou": true, "ser": true, "quando": true, "muito": true,
			"há": true, "nos": true, "já": true, "está": true, "eu": true,
			"também": true, "só": true, "pelo": true, "pela": true,
		},
		emotionWords: map[string]string{
			// Negativas
			"medo": "fear", "ansiedade": "anxiety", "tristeza": "sadness",
			"solidão": "loneliness", "raiva": "anger", "ódio": "hate",
			"culpa": "guilt", "vergonha": "shame", "desespero": "despair",
			"angústia": "anguish", "dor": "pain", "sofrimento": "suffering",
			// Positivas
			"alegria": "joy", "felicidade": "happiness", "amor": "love",
			"paz": "peace", "esperança": "hope", "gratidão": "gratitude",
			"alívio": "relief", "confiança": "trust",
		},
		negationWords: map[string]bool{
			"não": true, "nunca": true, "jamais": true, "nada": true,
			"nenhum": true, "nenhuma": true, "nem": true, "tampouco": true,
		},
		modalWords: map[string]bool{
			"quero": true, "queria": true, "devo": true, "devia": true,
			"preciso": true, "precisava": true, "posso": true, "podia": true,
			"tenho": true, "tinha": true, "vou": true, "ia": true,
		},
		intensifiers: map[string]float64{
			"muito": 1.5, "demais": 1.8, "extremamente": 2.0,
			"bastante": 1.3, "super": 1.6, "horrível": 1.7,
			"terrível": 1.8, "péssimo": 1.9, "ótimo": 1.5,
		},
		temporalMarkers: map[string]string{
			"sempre":    "frequency_high",
			"nunca":     "frequency_never",
			"todo dia":  "frequency_daily",
			"às vezes":  "frequency_sometimes",
			"raramente": "frequency_rare",
		},
		conditionals: map[string]bool{
			"se": true, "caso": true, "talvez": true,
			"pode ser": true, "quem sabe": true,
		},
		diminishers: map[string]float64{
			"pouco": 0.7, "meio": 0.8, "quase": 0.9,
			"um pouco": 0.75, "mais ou menos": 0.8,
		},
	}
}

// Analyze extrai a cadeia significante de um texto
func (a *Analyzer) Analyze(text string) *SignifierChain {
	chain := &SignifierChain{
		Words:           []string{},
		Emotions:        []string{},
		Negations:       []string{},
		Modals:          []string{},
		TemporalMarkers: []string{},
		Conditionals:    []string{},
		Intensity:       0.5, // Base
		HasDiminisher:   false,
		ContextScore:    0.5,
		RawText:         text,
	}

	// Normalizar e tokenizar
	words := a.tokenize(text)
	textLower := strings.ToLower(text)

	// Processar cada palavra
	intensityMultiplier := 1.0
	for i, word := range words {
		wordLower := strings.ToLower(word)

		// Verificar intensificadores
		if mult, ok := a.intensifiers[wordLower]; ok {
			intensityMultiplier *= mult
		}

		// Verificar diminuidores (NEW)
		if mult, ok := a.diminishers[wordLower]; ok {
			intensityMultiplier *= mult
			chain.HasDiminisher = true
		}

		// Verificar negações
		if a.negationWords[wordLower] {
			chain.Negations = append(chain.Negations, word)
			intensityMultiplier *= 1.2 // Negação aumenta intensidade
		}

		// Verificar modais
		if a.modalWords[wordLower] {
			chain.Modals = append(chain.Modals, word)
		}

		// Verificar emoções
		if emotion, ok := a.emotionWords[wordLower]; ok {
			chain.Emotions = append(chain.Emotions, emotion)
			intensityMultiplier *= 1.3
		}

		// Verificar marcadores temporais (NEW)
		if marker, ok := a.temporalMarkers[wordLower]; ok {
			chain.TemporalMarkers = append(chain.TemporalMarkers, marker)
			// "sempre" e "nunca" aumentam intensidade
			if marker == "frequency_high" || marker == "frequency_never" {
				intensityMultiplier *= 1.4
			}
		}

		// Verificar condicionais (NEW)
		if a.conditionals[wordLower] {
			chain.Conditionals = append(chain.Conditionals, word)
			intensityMultiplier *= 0.9 // Condicional reduz certeza
		}

		// Adicionar palavras significativas (não stopwords)
		if !a.stopwords[wordLower] && len(wordLower) > 2 {
			chain.Words = append(chain.Words, word)
		}

		// Detectar padrões específicos
		if i > 0 {
			// Padrão: negação + modal (ex: "não quero")
			prevWord := strings.ToLower(words[i-1])
			if a.negationWords[prevWord] && a.modalWords[wordLower] {
				intensityMultiplier *= 1.5 // Padrão forte
			}
		}
	}

	// Detectar frases compostas (NEW)
	for phrase, marker := range a.temporalMarkers {
		if strings.Contains(textLower, phrase) && len(phrase) > 5 {
			chain.TemporalMarkers = append(chain.TemporalMarkers, marker)
		}
	}

	for phrase := range a.conditionals {
		if strings.Contains(textLower, phrase) && len(phrase) > 3 {
			chain.Conditionals = append(chain.Conditionals, phrase)
		}
	}

	// Calcular intensidade final (clampar entre 0 e 1)
	chain.Intensity = minFloat(1.0, 0.5*intensityMultiplier)

	// Calcular context score (NEW)
	chain.ContextScore = a.calculateContextScore(chain)

	return chain
}

// calculateContextScore calcula score de contexto baseado em padrões
func (a *Analyzer) calculateContextScore(chain *SignifierChain) float64 {
	score := 0.5 // Base

	// Mais emoções = mais contexto
	if len(chain.Emotions) > 0 {
		score += 0.1 * float64(min(len(chain.Emotions), 3))
	}

	// Marcadores temporais = mais específico
	if len(chain.TemporalMarkers) > 0 {
		score += 0.15
	}

	// Condicionais = menos certeza
	if len(chain.Conditionals) > 0 {
		score -= 0.1
	}

	// Diminuidores = menos intensidade mas mais nuance
	if chain.HasDiminisher {
		score += 0.05
	}

	// Clampar entre 0 e 1
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// tokenize divide o texto em palavras
func (a *Analyzer) tokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// GetEvidences retorna evidências para inferência bayesiana
func (c *SignifierChain) GetEvidences() []string {
	evidences := []string{}

	// Evidência: presença de negação
	if len(c.Negations) > 0 {
		evidences = append(evidences, "negation_present")
	}

	// Evidência: presença de modal
	if len(c.Modals) > 0 {
		evidences = append(evidences, "modal_present")
	}

	// Evidência: emoção negativa
	for _, emotion := range c.Emotions {
		if isNegativeEmotion(emotion) {
			evidences = append(evidences, "negative_emotion")
			break
		}
	}

	// Evidência: alta intensidade
	if c.Intensity > 0.7 {
		evidences = append(evidences, "high_intensity")
	}

	// Evidência: padrão negação + modal
	if len(c.Negations) > 0 && len(c.Modals) > 0 {
		evidences = append(evidences, "negation_modal_pattern")
	}

	return evidences
}

// HasPattern verifica se a cadeia contém um padrão específico
func (c *SignifierChain) HasPattern(pattern string) bool {
	switch pattern {
	case "negation":
		return len(c.Negations) > 0
	case "modal":
		return len(c.Modals) > 0
	case "negation_modal":
		return len(c.Negations) > 0 && len(c.Modals) > 0
	case "high_emotion":
		return len(c.Emotions) > 0 && c.Intensity > 0.7
	default:
		return false
	}
}

func isNegativeEmotion(emotion string) bool {
	negative := map[string]bool{
		"fear": true, "anxiety": true, "sadness": true,
		"loneliness": true, "anger": true, "hate": true,
		"guilt": true, "shame": true, "despair": true,
		"anguish": true, "pain": true, "suffering": true,
	}
	return negative[emotion]
}
