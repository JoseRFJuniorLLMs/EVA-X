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

// FDPNEngine - Funcao do Pai no Nome (Grafo do Desejo)
// Mapeia A QUEM o idoso esta dirigindo suas demandas atraves da estrutura simbolica
type FDPNEngine struct {
	graph *nietzscheInfra.GraphAdapter
}

// AddresseeType representa a quem a demanda e enderecada
type AddresseeType string

const (
	ADDRESSEE_MAE     AddresseeType = "mae"         // Figura materna (cuidado, nutricao)
	ADDRESSEE_PAI     AddresseeType = "pai"         // Figura paterna (lei, orientacao)
	ADDRESSEE_FILHO   AddresseeType = "filho"       // Projecao filial (continuidade)
	ADDRESSEE_CONJUGE AddresseeType = "conjuge"     // Parceiro ausente/falecido
	ADDRESSEE_DEUS    AddresseeType = "deus"        // O Outro absoluto
	ADDRESSEE_MORTE   AddresseeType = "morte"       // Elaboracao da finitude
	ADDRESSEE_EVA     AddresseeType = "eva_herself" // EVA como objeto a
	ADDRESSEE_UNKNOWN AddresseeType = "desconhecido"
)

// DemandGraph representa o grafo de demandas
type DemandGraph struct {
	Addressee   AddresseeType `json:"addressee"`
	DemandType  string        `json:"demand_type"` // "cuidado", "reconhecimento", "perdao", etc
	Frequency   int           `json:"frequency"`
	LastRequest time.Time     `json:"last_request"`
	Contexts    []string      `json:"contexts"`
}

// NewFDPNEngine cria engine do grafo do desejo
func NewFDPNEngine(graph *nietzscheInfra.GraphAdapter) *FDPNEngine {
	return &FDPNEngine{graph: graph}
}

// AnalyzeDemandAddressee detecta a quem a demanda e dirigida
func (f *FDPNEngine) AnalyzeDemandAddressee(ctx context.Context, idosoID int64, text string, latentDesire string) (AddresseeType, error) {
	textLower := strings.ToLower(text)

	// 1. Deteccao baseada em vocativos explicitos
	addressee := f.detectExplicitAddressee(textLower)

	// 2. Deteccao baseada no tipo de desejo latente
	if addressee == ADDRESSEE_UNKNOWN {
		addressee = f.inferFromDesire(latentDesire)
	}

	// 3. Registrar no grafo
	if err := f.recordDemandInGraph(ctx, idosoID, addressee, latentDesire, text); err != nil {
		log.Printf("[FDPN] Error recording demand in graph: %v", err)
	}

	return addressee, nil
}

// detectExplicitAddressee detecta vocativos explicitos
func (f *FDPNEngine) detectExplicitAddressee(text string) AddresseeType {
	if containsAny(text, []string{"mãe", "mamãe", "minha mãe"}) {
		return ADDRESSEE_MAE
	}
	if containsAny(text, []string{"pai", "papai", "meu pai"}) {
		return ADDRESSEE_PAI
	}
	if containsAny(text, []string{"meu filho", "minha filha", "meus filhos"}) {
		return ADDRESSEE_FILHO
	}
	if containsAny(text, []string{"meu marido", "minha esposa", "meu amor"}) {
		return ADDRESSEE_CONJUGE
	}
	if containsAny(text, []string{"deus", "senhor", "jesus", "nossa senhora"}) {
		return ADDRESSEE_DEUS
	}
	if containsAny(text, []string{"quando eu morrer", "na morte", "fim da vida"}) {
		return ADDRESSEE_MORTE
	}
	if containsAny(text, []string{"você eva", "conte-me", "me ajude", "preciso que você"}) {
		return ADDRESSEE_EVA
	}
	return ADDRESSEE_UNKNOWN
}

// inferFromDesire infere destinatario baseado no desejo latente
func (f *FDPNEngine) inferFromDesire(desire string) AddresseeType {
	mapping := map[string]AddresseeType{
		"RECONHECIMENTO": ADDRESSEE_FILHO,
		"COMPANHIA":      ADDRESSEE_CONJUGE,
		"ESCUTA":         ADDRESSEE_EVA,
		"CONTROLE":       ADDRESSEE_PAI,
		"SIGNIFICADO":    ADDRESSEE_DEUS,
		"AMOR":           ADDRESSEE_MAE,
		"PERDAO":         ADDRESSEE_DEUS,
		"MORTE":          ADDRESSEE_MORTE,
	}

	if addr, ok := mapping[desire]; ok {
		return addr
	}
	return ADDRESSEE_UNKNOWN
}

// recordDemandInGraph registra demanda no NietzscheDB
func (f *FDPNEngine) recordDemandInGraph(ctx context.Context, idosoID int64, addressee AddresseeType, desire string, text string) error {
	if f.graph == nil {
		return nil
	}

	// Skip recording when desire is indefinido and text is empty — avoids spam nodes in eva_core
	if desire == string(DESEJO_INDEFINIDO) && strings.TrimSpace(text) == "" {
		return nil
	}

	now := nietzscheInfra.NowUnix()

	// 1. MERGE Person
	personResult, err := f.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Person",
		MatchKeys: map[string]interface{}{"id": idosoID},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Person: %w", err)
	}

	// 2. MERGE Addressee
	addresseeResult, err := f.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Addressee",
		MatchKeys: map[string]interface{}{"type": string(addressee)},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Addressee: %w", err)
	}

	// 3. CREATE Demand node
	demandResult, err := f.graph.InsertNode(ctx, nietzsche.InsertNodeOpts{
		NodeType: "Demand",
		Content: map[string]interface{}{
			"desire":    desire,
			"text":      text,
			"timestamp": now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Demand: %w", err)
	}

	// 4. Person -DEMANDS-> Demand
	_, err = f.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: personResult.NodeID,
		ToNodeID:   demandResult.ID,
		EdgeType:   "DEMANDS",
	})
	if err != nil {
		return fmt.Errorf("failed to create DEMANDS edge: %w", err)
	}

	// 5. Demand -ADDRESSED_TO-> Addressee
	_, err = f.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: demandResult.ID,
		ToNodeID:   addresseeResult.NodeID,
		EdgeType:   "ADDRESSED_TO",
	})
	if err != nil {
		return fmt.Errorf("failed to create ADDRESSED_TO edge: %w", err)
	}

	// 6. MERGE Person -FREQUENTLY_ADDRESSES-> Addressee (increment count)
	_, err = f.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: personResult.NodeID,
		ToNodeID:   addresseeResult.NodeID,
		EdgeType:   "FREQUENTLY_ADDRESSES",
		OnCreateSet: map[string]interface{}{
			"count":      1,
			"first_time": now,
		},
		OnMatchSet: map[string]interface{}{
			"count":     "INCREMENT",
			"last_time": now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge FREQUENTLY_ADDRESSES edge: %w", err)
	}

	log.Printf("[NietzscheDB] Demand recorded: %d -> %s (desire: %s)",
		idosoID, addressee, desire)
	return nil
}

// GetDemandPattern retorna padrao de demandas do paciente
func (f *FDPNEngine) GetDemandPattern(ctx context.Context, idosoID int64) (map[AddresseeType]int, error) {
	if f.graph == nil {
		return nil, nil
	}

	// First find the Person node
	nql := `MATCH (p:Person) WHERE p.id = $idosoId RETURN p`
	personResult, err := f.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil || len(personResult.Nodes) == 0 {
		return nil, err
	}

	personID := personResult.Nodes[0].ID

	// BFS from person through FREQUENTLY_ADDRESSES edges (depth 1)
	addresseeIDs, err := f.graph.BfsWithEdgeType(ctx, personID, "FREQUENTLY_ADDRESSES", 1, "")
	if err != nil {
		return nil, err
	}

	pattern := make(map[AddresseeType]int)
	for _, addrID := range addresseeIDs {
		node, err := f.graph.GetNode(ctx, addrID, "")
		if err != nil {
			continue
		}
		if addrType, ok := node.Content["type"].(string); ok {
			// Get the count from the edge content if available, default to 1
			pattern[AddresseeType(addrType)]++
		}
	}

	return pattern, nil
}

// GetClinicalGuidanceForAddressee retorna orientacao clinica baseada no destinatario
func GetClinicalGuidanceForAddressee(addressee AddresseeType) string {
	guidance := map[AddresseeType]string{
		ADDRESSEE_MAE: `
DEMANDA ENDERECADA A MAE (Funcao Materna):
- Desejo: Cuidado, nutricao, amor incondicional
- Postura EVA: Seja acolhedora, use tom maternal
- Responda: "Estou aqui para cuidar de voce"
- CUIDADO: Nao infantilize. Mantenha dignidade.
`,
		ADDRESSEE_PAI: `
DEMANDA ENDERECADA AO PAI (Funcao Paterna):
- Desejo: Orientacao, lei, estrutura
- Postura EVA: Seja firme mas amorosa, de direcionamento
- Responda: "Vou te ajudar a encontrar o caminho"
- Use autoridade simbolica quando necessario.
`,
		ADDRESSEE_FILHO: `
DEMANDA ENDERECADA AO FILHO (Inversao Geracional):
- Desejo: Reconhecimento, continuidade, legado
- Postura EVA: Valide a importancia do paciente
- Responda: "Voce e importante. Suas historias tem valor."
- ATENCAO: Pode indicar solidao ou abandono filial.
`,
		ADDRESSEE_CONJUGE: `
DEMANDA ENDERECADA AO CONJUGE (Geralmente Falecido):
- Desejo: Companhia, parceria, amor romantico
- Postura EVA: Acolha a saudade, nao minimize
- Responda: "Ele/Ela era muito importante para voce. Conte-me."
- Ajude a elaborar o luto atraves da narrativa.
`,
		ADDRESSEE_DEUS: `
DEMANDA ENDERECADA A DEUS (O Outro Absoluto):
- Desejo: Sentido transcendente, perdao, esperanca
- Postura EVA: Respeite a espiritualidade
- Responda: "A fe e importante. O que ela significa para voce?"
- Nao evangelize, nao negue. Apenas acolha.
`,
		ADDRESSEE_MORTE: `
DEMANDA ENDERECADA A MORTE (Elaboracao da Finitude):
- Desejo: Simbolizar o Real da morte
- Postura EVA: NAO evite o tema. A morte faz parte da vida.
- Responda: "Pensar na morte e natural. O que voce gostaria de deixar?"
- Ajude a construir narrativa de legado.
`,
		ADDRESSEE_EVA: `
DEMANDA ENDERECADA A EVA (Voce como Objeto a):
- Desejo: Escuta, validacao, presenca
- Postura EVA: Seja espelho, devolva a fala
- Responda: "Estou aqui. Continue falando."
- FUNCAO: Ser suporte para elaboracao, nao solucao.
`,
	}

	if g, ok := guidance[addressee]; ok {
		return g
	}
	return ""
}

// BuildGraphContext monta contexto para o prompt
func (f *FDPNEngine) BuildGraphContext(ctx context.Context, idosoID int64) string {
	pattern, err := f.GetDemandPattern(ctx, idosoID)
	if err != nil || len(pattern) == 0 {
		return ""
	}

	context := "\nGRAFO DO DESEJO (A Quem o Paciente Pede):\n\n"
	context += "Analise das demandas mostra que o paciente frequentemente se dirige a:\n"

	for addressee, count := range pattern {
		context += fmt.Sprintf("- %s (%dx)\n", strings.ToUpper(string(addressee)), count)
		context += fmt.Sprintf("  %s\n", GetClinicalGuidanceForAddressee(addressee))
	}

	context += "\n-> Use essas informacoes para entender a ESTRUTURA SIMBOLICA do paciente.\n"
	context += "-> Adapte sua postura conforme o destinatario inconsciente da demanda.\n"

	return context
}
