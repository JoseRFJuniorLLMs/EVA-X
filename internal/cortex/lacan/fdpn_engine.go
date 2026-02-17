// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"fmt"
	"log"
	"strings"
	"time"
)

// FDPNEngine - Função do Pai no Nome (Grafo do Desejo)
// Mapeia A QUEM o idoso está dirigindo suas demandas através da estrutura simbólica
type FDPNEngine struct {
	neo4j *graph.Neo4jClient
}

// AddresseeType representa a quem a demanda é endereçada
type AddresseeType string

const (
	ADDRESSEE_MAE     AddresseeType = "mae"         // Figura materna (cuidado, nutrição)
	ADDRESSEE_PAI     AddresseeType = "pai"         // Figura paterna (lei, orientação)
	ADDRESSEE_FILHO   AddresseeType = "filho"       // Projeção filial (continuidade)
	ADDRESSEE_CONJUGE AddresseeType = "conjuge"     // Parceiro ausente/falecido
	ADDRESSEE_DEUS    AddresseeType = "deus"        // O Outro absoluto
	ADDRESSEE_MORTE   AddresseeType = "morte"       // Elaboração da finitude
	ADDRESSEE_EVA     AddresseeType = "eva_herself" // EVA como objeto a
	ADDRESSEE_UNKNOWN AddresseeType = "desconhecido"
)

// DemandGraph representa o grafo de demandas
type DemandGraph struct {
	Addressee   AddresseeType `json:"addressee"`
	DemandType  string        `json:"demand_type"` // "cuidado", "reconhecimento", "perdão", etc
	Frequency   int           `json:"frequency"`
	LastRequest time.Time     `json:"last_request"`
	Contexts    []string      `json:"contexts"`
}

// NewFDPNEngine cria engine do grafo do desejo
func NewFDPNEngine(neo4j *graph.Neo4jClient) *FDPNEngine {
	return &FDPNEngine{neo4j: neo4j}
}

// AnalyzeDemandAddressee detecta a quem a demanda é dirigida
// demandAnalysis agora recebe simplesmente string para simplificar integração se o tipo Analysis não estiver disponível
func (f *FDPNEngine) AnalyzeDemandAddressee(ctx context.Context, idosoID int64, text string, latentDesire string) (AddresseeType, error) {
	textLower := strings.ToLower(text)

	// 1. Detecção baseada em vocativos explícitos
	addressee := f.detectExplicitAddressee(textLower)

	// 2. Detecção baseada no tipo de desejo latente
	if addressee == ADDRESSEE_UNKNOWN {
		addressee = f.inferFromDesire(latentDesire)
	}

	// 3. Registrar no grafo
	if err := f.recordDemandInGraph(ctx, idosoID, addressee, latentDesire, text); err != nil {
		log.Printf("⚠️ Error recording demand in graph: %v", err)
	}

	return addressee, nil
}

// detectExplicitAddressee detecta vocativos explícitos
func (f *FDPNEngine) detectExplicitAddressee(text string) AddresseeType {
	// Mãe
	if containsAny(text, []string{"mãe", "mamãe", "minha mãe"}) {
		return ADDRESSEE_MAE
	}

	// Pai
	if containsAny(text, []string{"pai", "papai", "meu pai"}) {
		return ADDRESSEE_PAI
	}

	// Filho/Filha
	if containsAny(text, []string{"meu filho", "minha filha", "meus filhos"}) {
		return ADDRESSEE_FILHO
	}

	// Cônjuge
	if containsAny(text, []string{"meu marido", "minha esposa", "meu amor"}) {
		return ADDRESSEE_CONJUGE
	}

	// Deus/Transcendente
	if containsAny(text, []string{"deus", "senhor", "jesus", "nossa senhora"}) {
		return ADDRESSEE_DEUS
	}

	// Morte
	if containsAny(text, []string{"quando eu morrer", "na morte", "fim da vida"}) {
		return ADDRESSEE_MORTE
	}

	// EVA herself (quando o paciente fala diretamente com EVA como objeto)
	if containsAny(text, []string{"você eva", "conte-me", "me ajude", "preciso que você"}) {
		return ADDRESSEE_EVA
	}

	return ADDRESSEE_UNKNOWN
}

// inferFromDesire infere destinatário baseado no desejo latente
func (f *FDPNEngine) inferFromDesire(desire string) AddresseeType {
	mapping := map[string]AddresseeType{
		"RECONHECIMENTO": ADDRESSEE_FILHO,   // Quer reconhecimento dos filhos
		"COMPANHIA":      ADDRESSEE_CONJUGE, // Sente falta do cônjuge
		"ESCUTA":         ADDRESSEE_EVA,     // Pede escuta à EVA
		"CONTROLE":       ADDRESSEE_PAI,     // Busca autoridade/orientação
		"SIGNIFICADO":    ADDRESSEE_DEUS,    // Busca sentido transcendente
		"AMOR":           ADDRESSEE_MAE,     // Amor incondicional materno
		"PERDAO":         ADDRESSEE_DEUS,    // Perdão divino ou do Outro
		"MORTE":          ADDRESSEE_MORTE,   // Elaboração da finitude
	}

	if addr, ok := mapping[desire]; ok {
		return addr
	}

	return ADDRESSEE_UNKNOWN
}

// recordDemandInGraph registra demanda no Neo4j
func (f *FDPNEngine) recordDemandInGraph(ctx context.Context, idosoID int64, addressee AddresseeType, desire string, text string) error {
	if f.neo4j == nil {
		return nil // Neo4j offline, skip
	}

	query := `
		// Criar Person se não existir
		MERGE (p:Person {id: $idosoId})
		
		// Criar nó do Destinatário
		MERGE (a:Addressee {type: $addressee})
		
		// Criar Demanda
		CREATE (d:Demand {
			desire: $desire,
			text: $text,
			timestamp: datetime()
		})
		
		// Relações
		MERGE (p)-[:DEMANDS]->(d)
		MERGE (d)-[:ADDRESSED_TO]->(a)
		
		// Incrementar frequência
		MERGE (p)-[r:FREQUENTLY_ADDRESSES]->(a)
		ON CREATE SET r.count = 1, r.first_time = datetime()
		ON MATCH SET r.count = r.count + 1, r.last_time = datetime()
	`

	params := map[string]interface{}{
		"idosoId":   idosoID,
		"addressee": string(addressee),
		"desire":    desire,
		"text":      text,
	}

	_, err := f.neo4j.ExecuteWrite(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to record demand: %w", err)
	}

	log.Printf("📊 [NEO4J] Demand recorded: %d → %s (desire: %s)",
		idosoID, addressee, desire)
	return nil
}

// GetDemandPattern retorna padrão de demandas do paciente
func (f *FDPNEngine) GetDemandPattern(ctx context.Context, idosoID int64) (map[AddresseeType]int, error) {
	if f.neo4j == nil {
		return nil, nil
	}

	query := `
		MATCH (p:Person {id: $idosoId})-[r:FREQUENTLY_ADDRESSES]->(a:Addressee)
		RETURN a.type, r.count
		ORDER BY r.count DESC
	`

	records, err := f.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
	})
	if err != nil {
		return nil, err
	}

	pattern := make(map[AddresseeType]int)
	for _, record := range records {
		addrType, _ := record.Get("a.type")
		count, _ := record.Get("r.count")

		pattern[AddresseeType(addrType.(string))] = int(count.(int64))
	}

	return pattern, nil
}

// GetClinicalGuidanceForAddressee retorna orientação clínica baseada no destinatário
func GetClinicalGuidanceForAddressee(addressee AddresseeType) string {
	guidance := map[AddresseeType]string{
		ADDRESSEE_MAE: `
DEMANDA ENDEREÇADA À MÃE (Função Materna):
- Desejo: Cuidado, nutrição, amor incondicional
- Postura EVA: Seja acolhedora, use tom maternal
- Responda: "Estou aqui para cuidar de você"
- CUIDADO: Não infantilize. Mantenha dignidade.
`,
		ADDRESSEE_PAI: `
DEMANDA ENDEREÇADA AO PAI (Função Paterna):
- Desejo: Orientação, lei, estrutura
- Postura EVA: Seja firme mas amorosa, dê direcionamento
- Responda: "Vou te ajudar a encontrar o caminho"
- Use autoridade simbólica quando necessário.
`,
		ADDRESSEE_FILHO: `
DEMANDA ENDEREÇADA AO FILHO (Inversão Geracional):
- Desejo: Reconhecimento, continuidade, legado
- Postura EVA: Valide a importância do paciente
- Responda: "Você é importante. Suas histórias têm valor."
- ATENÇÃO: Pode indicar solidão ou abandono filial.
`,
		ADDRESSEE_CONJUGE: `
DEMANDA ENDEREÇADA AO CÔNJUGE (Geralmente Falecido):
- Desejo: Companhia, parceria, amor romântico
- Postura EVA: Acolha a saudade, não minimize
- Responda: "Ele/Ela era muito importante para você. Conte-me."
- Ajude a elaborar o luto através da narrativa.
`,
		ADDRESSEE_DEUS: `
DEMANDA ENDEREÇADA A DEUS (O Outro Absoluto):
- Desejo: Sentido transcendente, perdão, esperança
- Postura EVA: Respeite a espiritualidade
- Responda: "A fé é importante. O que ela significa para você?"
- Não evangelize, não negue. Apenas acolha.
`,
		ADDRESSEE_MORTE: `
DEMANDA ENDEREÇADA À MORTE (Elaboração da Finitude):
- Desejo: Simbolizar o Real da morte
- Postura EVA: NÃO evite o tema. A morte faz parte da vida.
- Responda: "Pensar na morte é natural. O que você gostaria de deixar?"
- Ajude a construir narrativa de legado.
`,
		ADDRESSEE_EVA: `
DEMANDA ENDEREÇADA À EVA (Você como Objeto a):
- Desejo: Escuta, validação, presença
- Postura EVA: Seja espelho, devolva a fala
- Responda: "Estou aqui. Continue falando."
- FUNÇÃO: Ser suporte para elaboração, não solução.
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

	context := "\n📊 GRAFO DO DESEJO (A Quem o Paciente Pede):\n\n"
	context += "Análise das demandas mostra que o paciente frequentemente se dirige a:\n"

	for addressee, count := range pattern {
		context += fmt.Sprintf("- %s (%dx)\n", strings.ToUpper(string(addressee)), count)
		context += fmt.Sprintf("  %s\n", GetClinicalGuidanceForAddressee(addressee))
	}

	context += "\n→ Use essas informações para entender a ESTRUTURA SIMBÓLICA do paciente.\n"
	context += "→ Adapte sua postura conforme o destinatário inconsciente da demanda.\n"

	return context
}
