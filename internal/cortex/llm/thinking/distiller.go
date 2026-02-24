// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	nietzsche "nietzsche-sdk"
)

// DistillationSample representa um par entrada-saída para treino
type DistillationSample struct {
	NodeID    string                 `json:"node_id"`
	Embedding []float64              `json:"embedding"`
	Target    []float32              `json:"target_diffusion"` // Resultado do Professor
	Energy    float32                `json:"energy"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// Distiller executa o loop "Professor-Aluno".
// Ele usa os algoritmos clássicos do NietzscheDB (Chebyshev/Minkowski) para gerar
// labels para treinar os "monstrinhos" neurais.
type Distiller struct {
	graphAdapter *nietzscheInfra.GraphAdapter
}

func NewDistiller(ga *nietzscheInfra.GraphAdapter) *Distiller {
	return &Distiller{graphAdapter: ga}
}

// DistillKnowledge procesa a coleção e gera um dataset rotulado.
func (d *Distiller) DistillKnowledge(ctx context.Context, outputFile string) error {
	log.Printf("[DISTILLER] Iniciando destilação de conhecimento do motor clássico...")

	// 1. Buscar todos os pacientes (os "alunos" vão aprender sobre eles)
	qr, err := d.graphAdapter.ExecuteNQL(ctx, "MATCH (n) RETURN n", nil, "Patient")
	if err != nil {
		return fmt.Errorf("erro ao buscar pacientes: %w", err)
	}

	samples := make([]DistillationSample, 0, len(qr.Nodes))

	for _, node := range qr.Nodes {
		// 2. O PROFESSOR FALA: Rodar difusão de calor (Chebyshev) a partir deste nó
		// Isso define quais outros nós são semanticamente importantes.
		diffusion, err := d.graphAdapter.Diffuse(ctx, []string{node.ID}, nietzsche.DiffuseOpts{
			TValues: []float64{0.1, 0.5, 1.0, 2.0, 5.0},
		})
		if err != nil {
			log.Printf("[DISTILLER] Erro na difusão do nó %s: %v", node.ID, err)
			continue
		}

		// Consolidar resultado da difusão como um target tensor (simplificado)
		target := make([]float32, 128) // Imaginando um embedding comprimido de 128d
		if len(diffusion) > 0 {
			for i, scale := range diffusion {
				if i >= 128 {
					break
				}
				if len(scale.Scores) > 0 {
					target[i] = float32(scale.Scores[0])
				}
			}
		}

		samples = append(samples, DistillationSample{
			NodeID:    node.ID,
			Embedding: node.Embedding,
			Target:    target,
			Energy:    node.Energy,
			Metadata:  node.Content,
		})
	}

	// 3. Salvar em JSON para o Python ingerir
	file, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, file, 0644); err != nil {
		return err
	}

	log.Printf("[DISTILLER] ✅ Dataset destilado com %d amostras salvo em %s", len(samples), outputFile)
	return nil
}
