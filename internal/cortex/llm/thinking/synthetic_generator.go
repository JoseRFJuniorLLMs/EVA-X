// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	nietzsche "nietzsche-sdk"
)

// SyntheticPatient representa um perfil de paciente gerado para treino
type SyntheticPatient struct {
	Name        string                 `json:"name"`
	Age         int                    `json:"age"`
	Symptoms    []string               `json:"symptoms"`
	Conditions  []string               `json:"conditions"`
	Metadata    map[string]interface{} `json:"metadata"`
	CausalScore float32                `json:"causal_score"`
}

// SyntheticGenerator gera dados clínicos sintéticos para alimentar o treino neuro-simbólico.
type SyntheticGenerator struct {
	thinkingClient *ThinkingClient
	graphAdapter   *nietzscheInfra.GraphAdapter
}

func NewSyntheticGenerator(tc *ThinkingClient, ga *nietzscheInfra.GraphAdapter) *SyntheticGenerator {
	return &SyntheticGenerator{
		thinkingClient: tc,
		graphAdapter:   ga,
	}
}

// GenerateClinicalData gera e injeta N pacientes sintéticos no NietzscheDB.
func (g *SyntheticGenerator) GenerateClinicalData(ctx context.Context, count int) error {
	log.Printf("[SYNTHETIC] Iniciando geração de %d perfis clínicos...", count)

	for i := 0; i < count; i++ {
		// 1. Pedir ao Gemini para gerar um perfil realista (Angola/Malária context)
		prompt := `Gere um perfil de paciente sintético para um sistema de saúde em Angola.
Foque em casos variados (Malária, Hipertensão, Ansiedade, ou casos saudáveis).
Retorne APENAS um JSON no formato:
{
  "name": "Nome Completo",
  "age": 30,
  "symptoms": ["Febre", "Dor de cabeça"],
  "conditions": ["Malária Suspeita"],
  "metadata": {"location": "Luanda", "risk_factor": "alto"},
  "causal_score": 0.85
}`

		// Usamos AnalyzeHealthConcern mas para geração (reutilizando o pipeline de prompt)
		resp, err := g.thinkingClient.AnalyzeHealthConcern(ctx, prompt, "Contexto: Angola Healthcare Training")
		if err != nil {
			log.Printf("[SYNTHETIC] Erro ao gerar paciente %d: %v", i, err)
			continue
		}

		// Parse do JSON gerado do FinalAnswer (Gemini Thinking costuma colocar JSON lá se pedido)
		var patient SyntheticPatient
		jsonStr := g.extractJSON(resp.FinalAnswer)
		if err := json.Unmarshal([]byte(jsonStr), &patient); err != nil {
			log.Printf("[SYNTHETIC] Erro parse JSON paciente %d: %v", i, err)
			continue
		}

		// 2. Injetar no NietzscheDB
		nodeOpts := nietzsche.InsertNodeOpts{
			Collection: "Patient",
			NodeType:   "SyntheticPatient",
			Content: map[string]interface{}{
				"name":         patient.Name,
				"age":          patient.Age,
				"symptoms":     patient.Symptoms,
				"conditions":   patient.Conditions,
				"metadata":     patient.Metadata,
				"causal_score": patient.CausalScore,
				"synthetic":    true,
				"created_at":   time.Now().Unix(),
			},
			// Gerar embedding aleatório ou semi-estruturado se o server não auto-embed
			// (NietzscheDB v2.0 costuma auto-embed se NIETZSCHE_VECTOR_BACKEND=embedded)
		}

		res, err := g.graphAdapter.InsertNode(ctx, nodeOpts)
		if err != nil {
			log.Printf("[SYNTHETIC] Erro ao salvar paciente %d no banco: %v", i, err)
			continue
		}

		// 3. Criar arestas de "Sintoma" (Conexão estrutural para o GNN aprender)
		for _, symptom := range patient.Symptoms {
			symptomNode, _ := g.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
				NodeType:  "Symptom",
				MatchKeys: map[string]interface{}{"name": symptom},
			})

			g.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: res.ID,
				ToNodeID:   symptomNode.NodeID,
				EdgeType:   "HAS_SYMPTOM",
			})
		}

		if i%10 == 0 {
			log.Printf("[SYNTHETIC] Progresso: %d/%d injetados", i, count)
		}
	}

	log.Printf("[SYNTHETIC] Geração concluída com sucesso!")
	return nil
}

func (g *SyntheticGenerator) extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return text
}
