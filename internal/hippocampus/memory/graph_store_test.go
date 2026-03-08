package memory

import (
	"context"
	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"testing"
	"time"
)

func setupAdapter(t *testing.T) *nietzscheInfra.GraphAdapter {
	t.Helper()
	client, err := nietzscheInfra.NewClient("localhost:50051")
	if err != nil {
		t.Skip("Skipping NietzscheDB test: server not reachable")
		return nil
	}
	return nietzscheInfra.NewGraphAdapter(client, "patient_graph")
}

// TestNietzscheConnection verifica se a conexão com NietzscheDB está funcionando
func TestNietzscheConnection(t *testing.T) {
	adapter := setupAdapter(t)
	if adapter == nil {
		return
	}
	defer adapter.Client().Close()

	t.Log("✅ Conexão NietzscheDB OK")
}

// TestCountAllNodes conta todos os nós no banco
func TestCountAllNodes(t *testing.T) {
	adapter := setupAdapter(t)
	if adapter == nil {
		return
	}
	defer adapter.Client().Close()

	nql := `MATCH (n) RETURN labels(n) AS tipo, count(n) AS quantidade GROUP BY labels(n)`
	res, err := adapter.ExecuteNQL(context.Background(), nql, nil, "")
	if err != nil {
		t.Fatalf("Erro na query NQL: %v", err)
	}

	t.Log("📊 Contagem de nós por tipo:")
	for _, row := range res.ScalarRows {
		tipo := row["tipo"]
		qtd := row["quantidade"]
		t.Logf("  - %v: %v", tipo, qtd)
	}
}

// TestCreateSampleData cria dados de teste
func TestCreateSampleData(t *testing.T) {
	adapter := setupAdapter(t)
	if adapter == nil {
		return
	}
	defer adapter.Client().Close()

	store := NewGraphStore(adapter, &config.Config{NietzscheGRPCAddr: "localhost:50051"})

	// Criar memória de teste
	testMemory := &Memory{
		IdosoID:    1121,
		Content:    "Teste de salvamento de conversa no NietzscheDB",
		Speaker:    "user",
		Emotion:    "neutro",
		Importance: 0.5,
		SessionID:  "test-session-001",
		Timestamp:  time.Now(),
		Topics:     []string{"teste", "nietzschedb"},
	}

	err := store.AddEpisodicMemory(context.Background(), testMemory)
	if err != nil {
		t.Fatalf("Erro ao salvar memória: %v", err)
	}

	t.Log("✅ Memória de teste criada com sucesso no NietzscheDB")
}
