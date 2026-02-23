package memory

import (
	"context"
	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"testing"
	"time"
)

// TestNietzscheConnection verifica se a conexão com NietzscheDB está funcionando
func TestNietzscheConnection(t *testing.T) {
	cfg := &config.Config{
		NietzscheGRPCAddr: "localhost:50051",
	}

	adapter, err := nietzscheInfra.NewGraphAdapter(cfg)
	if err != nil {
		t.Skip("Skipping NietzscheDB test: server not reachable")
		return
	}
	defer adapter.Client().Close()

	t.Log("✅ Conexão NietzscheDB OK")
}

// TestCountAllNodes conta todos os nós no banco
func TestCountAllNodes(t *testing.T) {
	cfg := &config.Config{
		NietzscheGRPCAddr: "localhost:50051",
	}

	adapter, err := nietzscheInfra.NewGraphAdapter(cfg)
	if err != nil {
		t.Skip("Skipping test")
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
	cfg := &config.Config{
		NietzscheGRPCAddr: "localhost:50051",
	}

	adapter, err := nietzscheInfra.NewGraphAdapter(cfg)
	if err != nil {
		t.Skip("Skipping test")
		return
	}
	defer adapter.Client().Close()

	store := NewGraphStore(adapter, cfg)

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

	err = store.AddEpisodicMemory(context.Background(), testMemory)
	if err != nil {
		t.Fatalf("Erro ao salvar memória: %v", err)
	}

	t.Log("✅ Memória de teste criada com sucesso no NietzscheDB")
}
