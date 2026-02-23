// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/llm/thinking"
)

func main() {
	count := flag.Int("count", 100, "Número de pacientes sintéticos a gerar")
	distill := flag.Bool("distill", true, "Executar destilação após geração")
	output := flag.String("output", "distilled_data.json", "Arquivo de saída para o dataset")
	flag.Parse()

	log.Println("🚀 Iniciando Pipeline Zero-Data de Treino Neuro-Simbólico...")

	// 1. Setup Infra
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erro ao carregar config: %v", err)
	}
	nietzscheClient, err := nietzscheInfra.NewClient(cfg.NietzscheGRPCAddr)
	if err != nil {
		log.Fatalf("Erro ao criar Nietzsche client: %v", err)
	}
	graphAdapter := nietzscheInfra.NewGraphAdapter(nietzscheClient, "Patient")

	// Gemini Thinking Client
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY não configurada")
	}
	thinkingClient, err := thinking.NewThinkingClient(apiKey)
	if err != nil {
		log.Fatalf("Erro ao criar ThinkingClient: %v", err)
	}
	defer thinkingClient.Close()

	ctx := context.Background()

	// 2. Geração
	generator := thinking.NewSyntheticGenerator(thinkingClient, graphAdapter)
	if err := generator.GenerateClinicalData(ctx, *count); err != nil {
		log.Printf("⚠️ Erro na geração: %v", err)
	}

	// 3. Destilação (O Professor ensina o Aluno)
	if *distill {
		distiller := thinking.NewDistiller(graphAdapter)
		if err := distiller.DistillKnowledge(ctx, *output); err != nil {
			log.Fatalf("❌ Erro na destilação: %v", err)
		}
	}

	log.Println("✅ Pipeline finalizado com sucesso!")
	log.Printf("Dataset pronto para treino em: %s", *output)
}
