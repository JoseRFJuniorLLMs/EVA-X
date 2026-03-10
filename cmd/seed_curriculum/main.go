// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// seed_curriculum populates the AutonomousLearner curriculum in NietzscheDB.
// Without topics in eva_curriculum, the learner cycle finds nothing to study.
// Run: go run cmd/seed_curriculum/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"github.com/joho/godotenv"
)

type TopicSeed struct {
	Topic    string
	Category string
	Priority int // 1-5, 5 = highest
}

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	nzClient, err := nietzscheInfra.NewClient(cfg.NietzscheGRPCAddr)
	if err != nil {
		log.Fatalf("NietzscheDB connect failed: %v", err)
	}
	defer nzClient.Close()

	db := database.NewNietzscheDB(nzClient.SDK())
	ctx := context.Background()

	topics := getTopics()
	now := time.Now().Format(time.RFC3339)

	inserted := 0
	skipped := 0

	for _, t := range topics {
		// Check if topic already exists
		rows, _ := db.QueryByLabel(ctx, "eva_curriculum",
			" AND n.topic = $topic AND n.status = $status",
			map[string]interface{}{"topic": t.Topic, "status": "pending"}, 1)

		if len(rows) > 0 {
			skipped++
			log.Printf("SKIP (exists): %s", t.Topic)
			continue
		}

		_, err := db.Insert(ctx, "eva_curriculum", map[string]interface{}{
			"topic":        t.Topic,
			"category":     t.Category,
			"priority":     t.Priority,
			"requested_by": "seed_curriculum",
			"status":       "pending",
			"created_at":   now,
			"study_count":  0,
		})
		if err != nil {
			log.Printf("ERROR inserting %s: %v", t.Topic, err)
			continue
		}
		inserted++
		log.Printf("OK [P%d] %s (%s)", t.Priority, t.Topic, t.Category)
	}

	fmt.Printf("\n=== Seed Curriculum Complete ===\n")
	fmt.Printf("Inserted: %d | Skipped: %d | Total: %d\n", inserted, skipped, len(topics))
}

func getTopics() []TopicSeed {
	return []TopicSeed{
		// === CLINICAL — Prioridade Máxima (core mission) ===
		{Topic: "Malária em Angola - epidemiologia, diagnóstico e tratamento 2024-2026", Category: "clinical", Priority: 5},
		{Topic: "Técnicas de microscopia para identificação de parasitas sanguíneos (Plasmodium)", Category: "clinical", Priority: 5},
		{Topic: "Doença do Sono (Tripanossomíase Africana) - diagnóstico e tratamento", Category: "clinical", Priority: 5},
		{Topic: "Esquistossomose em África - ciclo de vida, sintomas e tratamento", Category: "clinical", Priority: 4},
		{Topic: "Anemia falciforme - genética, prevalência em África e manejo clínico", Category: "clinical", Priority: 4},
		{Topic: "Tuberculose pulmonar - interpretação de raio-X e diagnóstico", Category: "clinical", Priority: 4},
		{Topic: "HIV/SIDA em Angola - protocolos de tratamento e prevenção", Category: "clinical", Priority: 4},
		{Topic: "Febre amarela e dengue - diferenciação clínica", Category: "clinical", Priority: 3},
		{Topic: "Desnutrição infantil em África - triagem e intervenção", Category: "clinical", Priority: 3},
		{Topic: "Cólera e doenças diarreicas - prevenção e tratamento em contexto africano", Category: "clinical", Priority: 3},

		// === PSYCHOLOGY — Modelo Lacaniano ===
		{Topic: "Psicologia Lacaniana - as três estruturas clínicas (neurose, psicose, perversão)", Category: "psychology", Priority: 4},
		{Topic: "Estádio do espelho de Lacan - formação do eu e identificação imaginária", Category: "psychology", Priority: 3},
		{Topic: "Transferência na relação terapêutica - conceitos fundamentais", Category: "psychology", Priority: 3},
		{Topic: "Luto e melancolia - abordagem psicanalítica e acompanhamento", Category: "psychology", Priority: 3},
		{Topic: "Demência e envelhecimento - abordagem psicológica centrada na pessoa", Category: "psychology", Priority: 4},
		{Topic: "Solidão e isolamento social em idosos - intervenções baseadas em evidência", Category: "psychology", Priority: 4},

		// === TECHNOLOGY — Infraestrutura EVA ===
		{Topic: "Grafos hiperbólicos e modelo de Poincaré para knowledge graphs", Category: "technology", Priority: 3},
		{Topic: "Embeddings semânticos com Gemini - text-embedding-004 vs embedding-001", Category: "technology", Priority: 3},
		{Topic: "Inteligência artificial em diagnóstico médico por imagem", Category: "technology", Priority: 3},
		{Topic: "YOLOv8 para detecção de objectos em imagens médicas", Category: "technology", Priority: 2},
		{Topic: "WebRTC e comunicação em tempo real para telemedicina", Category: "technology", Priority: 2},

		// === WELLNESS — Cuidados ao idoso ===
		{Topic: "Mindfulness e meditação baseada em evidência para idosos", Category: "wellness", Priority: 2},
		{Topic: "Exercício físico adaptado para idosos - prevenção de quedas", Category: "wellness", Priority: 2},
		{Topic: "Nutrição geriátrica - necessidades especiais e suplementação", Category: "wellness", Priority: 2},
		{Topic: "Higiene do sono em idosos - técnicas não-farmacológicas", Category: "wellness", Priority: 2},

		// === ANGOLA — Contexto local ===
		{Topic: "Sistema de saúde de Angola - estrutura, desafios e recursos", Category: "angola", Priority: 4},
		{Topic: "Geografia e etnias de Angola - contexto cultural para atendimento", Category: "angola", Priority: 2},
		{Topic: "Plantas medicinais tradicionais angolanas e suas evidências", Category: "angola", Priority: 2},

		// === EMERGENCY — Protocolos de urgência ===
		{Topic: "Suporte básico de vida (BLS) - protocolos actualizados 2025", Category: "emergency", Priority: 5},
		{Topic: "Sinais de alerta em idosos - quando chamar emergência", Category: "emergency", Priority: 5},
		{Topic: "Gestão de crises emocionais e ideação suicida - protocolos de segurança", Category: "emergency", Priority: 5},
	}
}
