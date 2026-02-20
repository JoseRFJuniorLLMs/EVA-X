// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"
)

// SignifierService rastreia significantes recorrentes (palavras-chave que se repetem) no Grafo
type SignifierService struct {
	client *nietzscheInfra.GraphAdapter
}

// NewSignifierService cria novo servico com NietzscheDB
func NewSignifierService(client *nietzscheInfra.GraphAdapter) *SignifierService {
	return &SignifierService{client: client}
}

// Signifier representa um significante rastreado
type Signifier struct {
	Word            string
	Frequency       int
	Contexts        []string
	FirstOccurrence time.Time
	LastOccurrence  time.Time
	EmotionalCharge float64 // 0.0 (neutro) a 1.0 (altamente carregado)
}

// TrackSignifiers extrai e rastreia significantes emocionalmente carregados
func (s *SignifierService) TrackSignifiers(ctx context.Context, idosoID int64, text string) error {
	keywords := extractEmotionalKeywords(text)

	for _, word := range keywords {
		err := s.incrementSignifier(ctx, idosoID, word, text)
		if err != nil {
			return err
		}
	}

	return nil
}

// incrementSignifier incrementa frequencia de um significante no Grafo
func (s *SignifierService) incrementSignifier(ctx context.Context, idosoID int64, word, contextStr string) error {
	if s.client == nil {
		return nil
	}

	now := nietzscheInfra.NowUnix()

	// 1. MERGE Person node
	personResult, err := s.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Person",
		MatchKeys: map[string]interface{}{"id": idosoID},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Person: %w", err)
	}

	// 2. MERGE Significante node
	sigResult, err := s.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Significante",
		MatchKeys: map[string]interface{}{
			"word":     word,
			"idoso_id": idosoID,
		},
		OnCreateSet: map[string]interface{}{
			"frequency":        1,
			"first_occurrence":  now,
			"last_occurrence":   now,
			"contexts":         contextStr,
		},
		OnMatchSet: map[string]interface{}{
			"frequency":       "INCREMENT",
			"last_occurrence": now,
			"contexts":        contextStr,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Significante: %w", err)
	}

	// 3. CREATE Event node
	eventResult, err := s.client.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType: "Event",
		Content: map[string]interface{}{
			"type":      "utterance",
			"content":   contextStr,
			"timestamp": now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Event: %w", err)
	}

	// 4. Create edges: Event -EVOCA-> Significante, Person -EXPERIENCED-> Event
	_, err = s.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: eventResult.ID,
		ToNodeID:   sigResult.NodeID,
		EdgeType:   "EVOCA",
	})
	if err != nil {
		return fmt.Errorf("failed to create EVOCA edge: %w", err)
	}

	_, err = s.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: personResult.NodeID,
		ToNodeID:   eventResult.ID,
		EdgeType:   "EXPERIENCED",
	})
	if err != nil {
		return fmt.Errorf("failed to create EXPERIENCED edge: %w", err)
	}

	return nil
}

// GetKeySignifiers retorna os N significantes mais frequentes
func (s *SignifierService) GetKeySignifiers(ctx context.Context, idosoID int64, topN int) ([]Signifier, error) {
	if s.client == nil {
		return []Signifier{}, nil
	}

	nql := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId AND s.frequency >= 3 RETURN s ORDER BY s.frequency DESC LIMIT $limit`

	result, err := s.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
		"limit":   topN,
	}, "")
	if err != nil {
		return nil, err
	}

	var signifiers []Signifier
	for _, node := range result.Nodes {
		var sig Signifier

		if w, ok := node.Content["word"].(string); ok {
			sig.Word = w
		}
		if f, ok := node.Content["frequency"].(float64); ok {
			sig.Frequency = int(f)
		}
		if ctxs, ok := node.Content["contexts"].(string); ok {
			sig.Contexts = append(sig.Contexts, ctxs)
		}
		if ctxs, ok := node.Content["contexts"].([]interface{}); ok {
			for _, c := range ctxs {
				sig.Contexts = append(sig.Contexts, fmt.Sprintf("%v", c))
			}
		}

		sig.EmotionalCharge = calculateEmotionalCharge(sig.Word)
		signifiers = append(signifiers, sig)
	}

	return signifiers, nil
}

// ShouldInterpelSignifier decide se e momento de interpelar o significante
func (s *SignifierService) ShouldInterpelSignifier(ctx context.Context, idosoID int64, word string) (bool, error) {
	if s.client == nil {
		return false, nil
	}

	nql := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId AND s.word = $word RETURN s`
	result, err := s.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
		"word":    word,
	}, "")
	if err != nil || len(result.Nodes) == 0 {
		return false, err
	}

	node := result.Nodes[0]
	frequency := 0
	if f, ok := node.Content["frequency"].(float64); ok {
		frequency = int(f)
	}

	if frequency >= 5 {
		_, hasInterp := node.Content["last_interpellation"]
		if !hasInterp {
			return true, nil
		}
		return true, nil
	}

	return false, nil
}

// MarkAsInterpelled marca que o significante foi interpelado
func (s *SignifierService) MarkAsInterpelled(ctx context.Context, idosoID int64, word string) error {
	if s.client == nil {
		return nil
	}

	_, err := s.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Significante",
		MatchKeys: map[string]interface{}{
			"idoso_id": idosoID,
			"word":     word,
		},
		OnMatchSet: map[string]interface{}{
			"last_interpellation": nietzscheInfra.NowUnix(),
		},
	})
	return err
}

// GenerateInterpellation gera frase para interpelar o significante
func GenerateInterpellation(word string, frequency int) string {
	return "Percebi que voce frequentemente menciona a palavra '" + word + "'. " +
		"Ela apareceu " + string(rune(frequency)) + " vezes em nossas conversas. " +
		"O que essa palavra representa para voce?"
}

// Helper functions (Mantidas)

func extractEmotionalKeywords(text string) []string {
	// Palavras com carga emocional (extracao simples)
	emotionalWords := map[string]bool{
		"solidão":    true,
		"tristeza":   true,
		"medo":       true,
		"saudade":    true,
		"abandono":   true,
		"dor":        true,
		"sofrimento": true,
		"angústia":   true,
		"ansiedade":  true,
		"depressão":  true,
		"alegria":    true,
		"felicidade": true,
		"amor":       true,
		"morte":      true,
		"vida":       true,
		"família":    true,
		"filho":      true,
		"filha":      true,
		"esposa":     true,
		"marido":     true,
		"vazio":      true,
		"falta":      true,
		"perda":      true,
		"culpa":      true,
		"raiva":      true,
		"ódio":       true,
		"perdão":     true,
		"esperança":  true,
		"desespero":  true,
	}

	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	for _, word := range words {
		// Remove pontuacao
		cleaned := strings.Trim(word, ".,!?;:")
		if emotionalWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}

	return keywords
}

func calculateEmotionalCharge(word string) float64 {
	// Palavras de alta carga emocional (Exemplo)
	highCharge := map[string]bool{
		"morte": true, "abandono": true, "solidão": true,
		"desespero": true, "ódio": true, "culpa": true,
		"vazio": true, "perda": true,
	}

	if highCharge[word] {
		return 1.0
	}

	return 0.5 // Carga media por padrao
}

// Keeping sql import just to mock the signature if needed but we removed it from struct
var _ = sql.ErrNoRows
