// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// EnneagramService detects patient's Enneagram type from speech patterns
// PRINCIPLE: This identifies HOW the patient is trapped, not WHO they are
type EnneagramService struct {
	db       *database.DB
	types    map[int]*EnneagramType
	patterns map[int][]*regexp.Regexp
}

// NewEnneagramService creates a new Enneagram detection service
func NewEnneagramService(db *database.DB) *EnneagramService {
	svc := &EnneagramService{
		db:       db,
		types:    make(map[int]*EnneagramType),
		patterns: make(map[int][]*regexp.Regexp),
	}
	svc.loadTypes()
	return svc
}

// loadTypes loads Enneagram types and compiles keyword patterns
func (s *EnneagramService) loadTypes() {
	// Type 1 - The Perfectionist (Instinctive - Repressed Anger)
	s.types[1] = &EnneagramType{
		ID:                 1,
		Name:               "The Perfectionist",
		NamePT:             "O Perfeccionista",
		Center:             "instinctive",
		CenterPT:           "instintivo",
		RootEmotion:        "Anger (repressed)",
		RootEmotionPT:      "Raiva (reprimida)",
		ChiefFeature:       "Resentment and self-criticism",
		ChiefFeaturePT:     "Ressentimento e autocritica",
		DefenseMechanism:   "Reaction formation",
		DefenseMechanismPT: "Formacao reativa",
		KeywordsPT: []string{
			"certo", "errado", "deveria", "precisa", "correto",
			"responsabilidade", "dever", "perfeito", "falha", "erro",
		},
	}

	// Type 2 - The Helper (Emotional - Denied Shame)
	s.types[2] = &EnneagramType{
		ID:                 2,
		Name:               "The Helper",
		NamePT:             "O Ajudador",
		Center:             "emotional",
		CenterPT:           "emocional",
		RootEmotion:        "Shame (denied)",
		RootEmotionPT:      "Vergonha (negada)",
		ChiefFeature:       "Pride in being needed",
		ChiefFeaturePT:     "Orgulho de ser necessario",
		DefenseMechanism:   "Repression of own needs",
		DefenseMechanismPT: "Repressao das proprias necessidades",
		KeywordsPT: []string{
			"precisa de mim", "deixa eu ajudar", "faco por voce",
			"sempre estou", "cuido", "amor", "ajudo", "precisam",
		},
	}

	// Type 3 - The Achiever (Emotional - Avoided Shame)
	s.types[3] = &EnneagramType{
		ID:                 3,
		Name:               "The Achiever",
		NamePT:             "O Realizador",
		Center:             "emotional",
		CenterPT:           "emocional",
		RootEmotion:        "Shame (avoided)",
		RootEmotionPT:      "Vergonha (evitada)",
		ChiefFeature:       "Vanity and image manipulation",
		ChiefFeaturePT:     "Vaidade e manipulacao de imagem",
		DefenseMechanism:   "Identification with success",
		DefenseMechanismPT: "Identificacao com sucesso",
		KeywordsPT: []string{
			"consegui", "sucesso", "melhor", "eficiente", "resultado",
			"trabalho", "reconhecimento", "produtivo", "realizei",
		},
	}

	// Type 4 - The Individualist (Emotional - Internalized Shame)
	s.types[4] = &EnneagramType{
		ID:                 4,
		Name:               "The Individualist",
		NamePT:             "O Individualista",
		Center:             "emotional",
		CenterPT:           "emocional",
		RootEmotion:        "Shame (internalized)",
		RootEmotionPT:      "Vergonha (internalizada)",
		ChiefFeature:       "Envy and feeling deficient",
		ChiefFeaturePT:     "Inveja e sentir-se deficiente",
		DefenseMechanism:   "Introjection",
		DefenseMechanismPT: "Introjecao",
		KeywordsPT: []string{
			"ninguem entende", "diferente", "especial", "sinto profundamente",
			"vazio", "saudade", "falta", "unico", "incompreendido",
		},
	}

	// Type 5 - The Investigator (Mental - Fear of Intrusion)
	s.types[5] = &EnneagramType{
		ID:                 5,
		Name:               "The Investigator",
		NamePT:             "O Investigador",
		Center:             "mental",
		CenterPT:           "mental",
		RootEmotion:        "Fear (of intrusion)",
		RootEmotionPT:      "Medo (de intrusao)",
		ChiefFeature:       "Avarice of resources and energy",
		ChiefFeaturePT:     "Avareza de recursos e energia",
		DefenseMechanism:   "Isolation and compartmentalization",
		DefenseMechanismPT: "Isolamento e compartimentalizacao",
		KeywordsPT: []string{
			"penso", "estudo", "preciso entender", "sozinho",
			"observo", "analiso", "conhecimento", "pesquiso",
		},
	}

	// Type 6 - The Loyalist (Mental - Fear of Abandonment)
	s.types[6] = &EnneagramType{
		ID:                 6,
		Name:               "The Loyalist",
		NamePT:             "O Leal",
		Center:             "mental",
		CenterPT:           "mental",
		RootEmotion:        "Fear (of abandonment)",
		RootEmotionPT:      "Medo (de abandono)",
		ChiefFeature:       "Doubt and suspicion",
		ChiefFeaturePT:     "Duvida e suspeita",
		DefenseMechanism:   "Projection",
		DefenseMechanismPT: "Projecao",
		KeywordsPT: []string{
			"e se", "cuidado", "confianca", "seguro", "lealdade",
			"duvida", "preocupado", "sera que", "medo",
		},
	}

	// Type 7 - The Enthusiast (Mental - Fear of Pain)
	s.types[7] = &EnneagramType{
		ID:                 7,
		Name:               "The Enthusiast",
		NamePT:             "O Entusiasta",
		Center:             "mental",
		CenterPT:           "mental",
		RootEmotion:        "Fear (of pain)",
		RootEmotionPT:      "Medo (de dor)",
		ChiefFeature:       "Gluttony for experience",
		ChiefFeaturePT:     "Gula por experiencias",
		DefenseMechanism:   "Rationalization and reframing",
		DefenseMechanismPT: "Racionalizacao e reenquadramento",
		KeywordsPT: []string{
			"legal", "divertido", "plano", "opcao", "possibilidade",
			"vamos", "novo", "aventura", "interessante",
		},
	}

	// Type 8 - The Challenger (Instinctive - Expressed Anger)
	s.types[8] = &EnneagramType{
		ID:                 8,
		Name:               "The Challenger",
		NamePT:             "O Desafiador",
		Center:             "instinctive",
		CenterPT:           "instintivo",
		RootEmotion:        "Anger (expressed)",
		RootEmotionPT:      "Raiva (expressada)",
		ChiefFeature:       "Lust for power and control",
		ChiefFeaturePT:     "Luxuria por poder e controle",
		DefenseMechanism:   "Denial of vulnerability",
		DefenseMechanismPT: "Negacao da vulnerabilidade",
		KeywordsPT: []string{
			"forte", "luta", "controle", "poder", "proteger",
			"nao deixo", "ninguem manda", "enfrento", "decido",
		},
	}

	// Type 9 - The Peacemaker (Instinctive - Denied Anger)
	s.types[9] = &EnneagramType{
		ID:                 9,
		Name:               "The Peacemaker",
		NamePT:             "O Pacificador",
		Center:             "instinctive",
		CenterPT:           "instintivo",
		RootEmotion:        "Anger (denied)",
		RootEmotionPT:      "Raiva (negada)",
		ChiefFeature:       "Self-forgetting and merging",
		ChiefFeaturePT:     "Auto-esquecimento e fusao",
		DefenseMechanism:   "Narcotization (numbing)",
		DefenseMechanismPT: "Narcotizacao (anestesia)",
		KeywordsPT: []string{
			"tanto faz", "nao sei", "talvez", "deixa quieto",
			"nao quero incomodar", "paz", "calma", "tranquilo",
		},
	}

	// Compile regex patterns for each type
	for typeID, et := range s.types {
		patterns := make([]*regexp.Regexp, 0, len(et.KeywordsPT))
		for _, kw := range et.KeywordsPT {
			pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
			patterns = append(patterns, pattern)
		}
		s.patterns[typeID] = patterns
	}
}

// AnalyzeText analyzes patient speech for Enneagram evidence
func (s *EnneagramService) AnalyzeText(ctx context.Context, idosoID int64, text string, memoryID int64) ([]*EnneagramEvidence, error) {
	textLower := strings.ToLower(text)
	var evidences []*EnneagramEvidence

	for typeID, patterns := range s.patterns {
		for i, pattern := range patterns {
			if pattern.MatchString(textLower) {
				evidence := &EnneagramEvidence{
					IdosoID:       idosoID,
					MemoryID:      memoryID,
					Verbatim:      text,
					SuggestedType: typeID,
					Weight:        0.5, // Base weight
					Category:      "keyword",
					Context:       s.types[typeID].KeywordsPT[i],
					Timestamp:     time.Now(),
				}

				// Increase weight for chief feature indicators
				if s.detectChiefFeature(textLower, typeID) {
					evidence.Weight = 0.8
					evidence.Category = "chief_feature"
				}

				// Increase weight for defense mechanism indicators
				if s.detectDefenseMechanism(textLower, typeID) {
					evidence.Weight = 0.7
					evidence.Category = "defense_mechanism"
				}

				evidences = append(evidences, evidence)
			}
		}
	}

	// Save evidences
	for _, ev := range evidences {
		if err := s.saveEvidence(ctx, ev); err != nil {
			log.Printf("Error saving enneagram evidence: %v", err)
		}
	}

	return evidences, nil
}

// detectChiefFeature checks for chief feature indicators
func (s *EnneagramService) detectChiefFeature(text string, typeID int) bool {
	chiefFeaturePatterns := map[int][]string{
		1: {"deveria ter feito", "nao esta certo", "precisa ser"},
		2: {"preciso ajudar", "sem mim", "depende de mim"},
		3: {"preciso mostrar", "tenho que conseguir", "parecer"},
		4: {"ninguem me entende", "sou diferente", "algo falta"},
		5: {"preciso pensar", "deixa eu analisar", "nao sei o suficiente"},
		6: {"e se der errado", "nao confio", "tenho medo"},
		7: {"vamos fazer algo", "tem outras opcoes", "nao quero ficar parado"},
		8: {"nao vou deixar", "tenho que controlar", "sou forte"},
		9: {"tanto faz", "nao quero conflito", "deixa como esta"},
	}

	patterns, ok := chiefFeaturePatterns[typeID]
	if !ok {
		return false
	}

	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// detectDefenseMechanism checks for defense mechanism indicators
func (s *EnneagramService) detectDefenseMechanism(text string, typeID int) bool {
	defensePatterns := map[int][]string{
		1: {"mas eu estava certo", "o certo seria"},
		2: {"eu so quero ajudar", "faco por amor"},
		3: {"estou muito ocupado", "trabalhando muito"},
		4: {"voce nao entenderia", "e complicado"},
		5: {"preciso de mais tempo", "deixa eu ver"},
		6: {"e se eles", "acho que querem"},
		7: {"mas olha o lado bom", "podia ser pior"},
		8: {"nao tenho medo", "nao me afeta"},
		9: {"nao tem problema", "esta tudo bem"},
	}

	patterns, ok := defensePatterns[typeID]
	if !ok {
		return false
	}

	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// saveEvidence persists evidence to database
func (s *EnneagramService) saveEvidence(ctx context.Context, ev *EnneagramEvidence) error {
	now := ev.Timestamp.Format(time.RFC3339)

	content := map[string]interface{}{
		"idoso_id":       ev.IdosoID,
		"verbatim":       ev.Verbatim,
		"suggested_type": ev.SuggestedType,
		"weight":         ev.Weight,
		"category":       ev.Category,
		"context":        ev.Context,
		"timestamp":      now,
		"created_at":     now,
	}

	if ev.MemoryID > 0 {
		content["memory_id"] = ev.MemoryID
	}

	id, err := s.db.Insert(ctx, "enneagram_evidence", content)
	if err != nil {
		return err
	}
	ev.ID = id
	return nil
}

// GetPatientEnneagram retrieves patient's Enneagram assessment
func (s *EnneagramService) GetPatientEnneagram(ctx context.Context, idosoID int64) (*PatientEnneagram, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_enneagram",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		// Return empty assessment
		return &PatientEnneagram{
			IdosoID:    idosoID,
			TypeScores: make(map[string]float64),
		}, nil
	}

	m := rows[0]
	pe := &PatientEnneagram{
		IdosoID:              idosoID,
		PrimaryType:          int(database.GetInt64(m, "primary_type")),
		PrimaryTypeConfidence: database.GetFloat64(m, "primary_type_confidence"),
		DominantWing:         int(database.GetInt64(m, "dominant_wing")),
		WingInfluence:        database.GetFloat64(m, "wing_influence"),
		HealthLevel:          int(database.GetInt64(m, "health_level")),
		InstinctualVariant:   database.GetString(m, "instinctual_variant"),
		EvidenceCount:        int(database.GetInt64(m, "evidence_count")),
		IdentificationMethod: database.GetString(m, "identification_method"),
		LastEvidenceAt:       database.GetTime(m, "last_evidence_at"),
		IdentifiedAt:         database.GetTime(m, "identified_at"),
	}

	// Parse type scores
	pe.TypeScores = make(map[string]float64)
	if raw, ok := m["type_scores"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			json.Unmarshal([]byte(v), &pe.TypeScores)
		case map[string]interface{}:
			for k, val := range v {
				if f, ok := val.(float64); ok {
					pe.TypeScores[k] = f
				}
			}
		}
	}

	return pe, nil
}

// GetEnneagramType returns details for a specific type
func (s *EnneagramService) GetEnneagramType(typeID int) *EnneagramType {
	return s.types[typeID]
}

// GenerateMirrorOutput creates objective output about patient's Enneagram
func (s *EnneagramService) GenerateMirrorOutput(ctx context.Context, idosoID int64) (*MirrorOutput, error) {
	pe, err := s.GetPatientEnneagram(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	if pe.PrimaryType == 0 || pe.PrimaryTypeConfidence < 0.3 {
		return nil, nil // Not enough data
	}

	et := s.types[pe.PrimaryType]
	if et == nil {
		return nil, fmt.Errorf("unknown enneagram type: %d", pe.PrimaryType)
	}

	// Count evidences by category
	evidenceRows, err := s.db.QueryByLabel(ctx, "enneagram_evidence",
		" AND n.idoso_id = $idoso AND n.suggested_type = $stype",
		map[string]interface{}{"idoso": idosoID, "stype": pe.PrimaryType}, 0)
	if err != nil {
		return nil, err
	}

	categoryCounts := make(map[string]int)
	for _, m := range evidenceRows {
		cat := database.GetString(m, "category")
		if cat != "" {
			categoryCounts[cat]++
		}
	}

	// Build mirror output - objective data, no interpretation
	dataPoints := []string{
		fmt.Sprintf("Padrao de fala identificado: Centro %s", et.CenterPT),
		fmt.Sprintf("Emocao raiz associada: %s", et.RootEmotionPT),
		fmt.Sprintf("Evidencias coletadas: %d falas", pe.EvidenceCount),
	}

	if cnt, ok := categoryCounts["chief_feature"]; ok && cnt > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Traco principal detectado %d vezes: %s", cnt, et.ChiefFeaturePT))
	}

	return &MirrorOutput{
		Type:       "enneagram_pattern",
		DataPoints: dataPoints,
		Frequency:  &pe.EvidenceCount,
		Question:   "Voce percebe esse padrao em si mesmo? O que voce acha que isso significa?",
		RawData: map[string]interface{}{
			"type_id":        pe.PrimaryType,
			"type_name":      et.NamePT,
			"confidence":     pe.PrimaryTypeConfidence,
			"center":         et.CenterPT,
			"root_emotion":   et.RootEmotionPT,
			"chief_feature":  et.ChiefFeaturePT,
			"category_counts": categoryCounts,
		},
	}, nil
}
