// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"testing"
	"time"
)

// TestNeo4jConnection verifica se a conexão com Neo4j está funcionando
func TestNeo4jConnection(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar ao Neo4j: %v", err)
	}
	defer client.Close(context.Background())

	t.Log("✅ Conexão Neo4j OK")
}

// TestCountAllNodes conta todos os nós no banco
func TestCountAllNodes(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar: %v", err)
	}
	defer client.Close(context.Background())

	query := `MATCH (n) RETURN labels(n) AS tipo, count(n) AS quantidade`
	records, err := client.ExecuteRead(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("Erro na query: %v", err)
	}

	t.Log("📊 Contagem de nós por tipo:")
	for _, record := range records {
		tipo, _ := record.Get("tipo")
		qtd, _ := record.Get("quantidade")
		t.Logf("  - %v: %v", tipo, qtd)
	}

	if len(records) == 0 {
		t.Log("⚠️ Banco está VAZIO - nenhum nó encontrado")
	}
}

// TestGetAllConversations busca todas as conversas
func TestGetAllConversations(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar: %v", err)
	}
	defer client.Close(context.Background())

	query := `
		MATCH (p:Person)-[:EXPERIENCED]->(e:Event)
		RETURN p.id AS idoso_id,
		       e.content AS mensagem,
		       e.speaker AS quem_falou,
		       e.timestamp AS quando
		ORDER BY e.timestamp DESC
		LIMIT 50
	`
	records, err := client.ExecuteRead(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("Erro na query: %v", err)
	}

	t.Logf("📝 Total de conversas encontradas: %d", len(records))
	for i, record := range records {
		idoso, _ := record.Get("idoso_id")
		msg, _ := record.Get("mensagem")
		speaker, _ := record.Get("quem_falou")
		t.Logf("  [%d] Idoso %v (%v): %v", i+1, idoso, speaker, msg)
	}
}

// TestGetDataForUser busca todos os dados de um usuário específico
func TestGetDataForUser(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar: %v", err)
	}
	defer client.Close(context.Background())

	// Testar com ID 1121
	idosoID := int64(1121)

	// Query 1: Eventos/Conversas
	query1 := `
		MATCH (p:Person {id: $idosoId})-[:EXPERIENCED]->(e:Event)
		RETURN e.content AS content, e.speaker AS speaker, e.timestamp AS timestamp
		ORDER BY e.timestamp DESC
		LIMIT 20
	`
	records, err := client.ExecuteRead(context.Background(), query1, map[string]interface{}{"idosoId": idosoID})
	if err != nil {
		t.Fatalf("Erro na query eventos: %v", err)
	}
	t.Logf("📝 Eventos do usuário %d: %d encontrados", idosoID, len(records))

	// Query 2: Significantes
	query2 := `
		MATCH (s:Significante {idoso_id: $idosoId})
		RETURN s.word AS palavra, s.frequency AS frequencia, s.emotional_valence AS valencia
		ORDER BY s.frequency DESC
		LIMIT 10
	`
	records2, err := client.ExecuteRead(context.Background(), query2, map[string]interface{}{"idosoId": idosoID})
	if err != nil {
		t.Logf("⚠️ Erro na query significantes: %v", err)
	} else {
		t.Logf("🔤 Significantes do usuário %d: %d encontrados", idosoID, len(records2))
	}

	// Query 3: Demandas
	query3 := `
		MATCH (p:Person {id: $idosoId})-[:DEMANDS]->(d:Demand)
		RETURN d.type AS tipo, d.text AS texto, d.timestamp AS quando
		ORDER BY d.timestamp DESC
		LIMIT 10
	`
	records3, err := client.ExecuteRead(context.Background(), query3, map[string]interface{}{"idosoId": idosoID})
	if err != nil {
		t.Logf("⚠️ Erro na query demandas: %v", err)
	} else {
		t.Logf("📢 Demandas do usuário %d: %d encontradas", idosoID, len(records3))
	}
}

// TestCreateSampleData cria dados de teste
func TestCreateSampleData(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar: %v", err)
	}
	defer client.Close(context.Background())

	store := NewGraphStore(client, cfg)

	// Criar memória de teste
	testMemory := &Memory{
		IdosoID:    1121,
		Content:    "Teste de salvamento de conversa",
		Speaker:    "user",
		Emotion:    "neutro",
		Importance: 0.5,
		SessionID:  "test-session-001",
		Timestamp:  time.Now(),
		Topics:     []string{"teste", "memoria"},
	}

	err = store.AddEpisodicMemory(context.Background(), testMemory)
	if err != nil {
		t.Fatalf("Erro ao salvar memória: %v", err)
	}

	t.Log("✅ Memória de teste criada com sucesso")
}

// TestVerifySchemaExists verifica se o schema existe
func TestVerifySchemaExists(t *testing.T) {
	cfg := &config.Config{
		Neo4jURI:      "bolt://104.248.219.200:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "Debian23",
	}

	client, err := graph.NewNeo4jClient(cfg)
	if err != nil {
		t.Fatalf("Falha ao conectar: %v", err)
	}
	defer client.Close(context.Background())

	// Verificar labels existentes
	query := `CALL db.labels() YIELD label RETURN label`
	records, err := client.ExecuteRead(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("Erro ao buscar labels: %v", err)
	}

	t.Log("📋 Labels existentes no banco:")
	expectedLabels := map[string]bool{
		"Person": false, "Event": false, "Topic": false,
		"Emotion": false, "Significante": false, "Demand": false,
	}

	for _, record := range records {
		label, _ := record.Get("label")
		labelStr := label.(string)
		t.Logf("  - %s", labelStr)
		if _, ok := expectedLabels[labelStr]; ok {
			expectedLabels[labelStr] = true
		}
	}

	t.Log("\n📊 Status dos labels esperados:")
	for label, exists := range expectedLabels {
		status := "❌ NÃO EXISTE"
		if exists {
			status = "✅ OK"
		}
		t.Logf("  - %s: %s", label, status)
	}
}
