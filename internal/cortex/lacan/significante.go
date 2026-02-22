// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"
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
			"first_occurrence": now,
			"last_occurrence":  now,
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

// FindSemanticNeighbors localiza significantes proximos no manifold hiperbolico (associações livres)
func (s *SignifierService) FindSemanticNeighbors(ctx context.Context, idosoID int64, word string, limit int) ([]string, error) {
	if s.client == nil {
		return nil, nil
	}

	// 1. Encontrar o nó do significante alvo
	nqlFind := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId AND s.word = $word RETURN s`
	qr, err := s.client.ExecuteNQL(ctx, nqlFind, map[string]interface{}{
		"idosoId": idosoID,
		"word":    word,
	}, "")
	if err != nil || len(qr.Nodes) == 0 {
		return nil, err
	}
	targetID := qr.Nodes[0].ID

	// 2. Buscar vizinhos por proximidade no manifold (HYPERBOLIC_DIST)
	// Isso simula a "associação livre" lacaniana através da estrutura do grafo.
	nqlNeighbors := `
		MATCH (s:Significante) 
		WHERE s.idoso_id = $idosoId AND s.id != $targetId
		RETURN s.word
		ORDER BY HYPERBOLIC_DIST(s, $targetId) ASC
		LIMIT $limit
	`
	qrNeighbors, err := s.client.ExecuteNQL(ctx, nqlNeighbors, map[string]interface{}{
		"idosoId":  idosoID,
		"targetId": targetID,
		"limit":    limit,
	}, "")
	if err != nil {
		return nil, err
	}

	var neighbors []string
	for _, node := range qrNeighbors.Nodes {
		if w, ok := node.Content["word"].(string); ok {
			neighbors = append(neighbors, w)
		}
	}
	return neighbors, nil
}

// IdentifySessionDominant identifica o significante central (Ponto de Almofada) usando PageRank
func (s *SignifierService) IdentifySessionDominant(ctx context.Context, idosoID int64) (string, error) {
	if s.client == nil {
		return "", nil
	}

	// 1. Executar PageRank na coleção (o NietzscheDB já computa centralidade estrutural)
	// Usamos o SDK interno via GraphAdapter
	sdkClient := s.client.SDK()
	if sdkClient == nil {
		return "", fmt.Errorf("nietzsche sdk client not available")
	}

	// PageRank focado na estrutura de significantes experimentados
	// Nota: Em produção, o PageRank pode ser agendado, aqui fazemos sob demanda para a sessão.
	result, err := sdkClient.RunPageRank(ctx, "patient_graph", 0.85, 20)
	if err != nil {
		return "", err
	}

	// 2. Filtrar os resultados por idosoID e escolher o maior score (Significante)
	var bestWord string
	var maxScore float64

	for _, score := range result.Scores {
		node, err := s.client.GetNode(ctx, score.NodeID, "")
		if err != nil || node.NodeType != "Significante" {
			continue
		}

		if id, ok := node.Content["idoso_id"].(float64); ok && int64(id) == idosoID {
			if score.Score > maxScore {
				maxScore = score.Score
				if w, ok := node.Content["word"].(string); ok {
					bestWord = w
				}
			}
		}
	}

	return bestWord, nil
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
// ── L-SYSTEMS (GROWTH) ────────────────────────────────────────────────────────

// LSystemRules define como o grafo cresce organicamente (Fase 11)
var LSystemRules = map[string]string{
	"A": "AB",   // Associação primária gera ramificação
	"B": "A[C]", // Ramificação gera conceito lateral
	"C": "S",    // Conceito se estabiliza como Simbólico
}

// ApplyLSystemGrowth aplica regras de crescimento ao grafo de um idoso.
// Simula o 'processo primário' de Freud/Lacan onde os significantes se multiplicam.
func (s *SignifierService) ApplyLSystemGrowth(ctx context.Context, idosoID int64, iterations int) error {
	log.Printf("🌿 [L-SYSTEM] Iniciando crescimento orgânico do grafo (Idoso: %d)", idosoID)

	// Localizar os 5 significantes com maior energia (semente)
	nqlSeed := `
		MATCH (s:Significante) 
		WHERE s.idoso_id = $idosoId
		RETURN s.id, s.word
		ORDER BY s.energy DESC
		LIMIT 5
	`
	res, err := s.client.ExecuteNQL(ctx, nqlSeed, map[string]interface{}{"idosoId": idosoID}, "")
	if err != nil || len(res.Nodes) == 0 {
		return err
	}

	for _, node := range res.Nodes {
		word, _ := node.Content["word"].(string)
		// Aplicar regra simplificada: se a palavra contém padrão 'A', criar associação 'B'
		if strings.Contains(strings.ToLower(word), "dor") || strings.Contains(strings.ToLower(word), "medo") {
			// Simular gramática: Criar um nó de 'Angústia' e associar
			assocWord := "Angústia"
			// No NietzscheDB, MergeNode garante a existência do nó
			mergeRes, err := s.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
				NodeType: "Significante",
				MatchKeys: map[string]interface{}{
					"idoso_id": idosoID,
					"word":     assocWord,
				},
				OnCreateSet: map[string]interface{}{
					"energy": 0.8,
				},
			})
			if err == nil {
				s.client.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
					FromNodeID: node.ID,
					ToNodeID:   mergeRes.NodeID,
					EdgeType:   "L_SYSTEM_ASSOCIATION",
				})
			}
		}
	}

	return nil
}
