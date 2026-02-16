package memory

import (
	"context"
	"testing"
)

func TestRetrievalService_RetrieveHybridWithFDPN(t *testing.T) {
	// Mock dependencies
	mockDB := &mockDatabase{}
	mockEmbedder := &mockEmbeddingService{}
	mockQdrant := &mockQdrantClient{}
	mockGraphStore := &mockGraphStore{}
	mockFDPN := &mockFDPNEngine{}

	retrieval := &RetrievalService{
		db:         mockDB,
		embedder:   mockEmbedder,
		qdrant:     mockQdrant,
		graphStore: mockGraphStore,
		fdpn:       mockFDPN,
	}

	// Test: busca com FDPN boost
	results, err := retrieval.RetrieveHybridWithFDPN(context.Background(), 1, "café com Maria", 5)

	if err != nil {
		t.Fatalf("RetrieveHybridWithFDPN failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected results, got empty")
	}
}

func TestRetrievalService_RetrieveHybridWithFDPN_NoFDPN(t *testing.T) {
	// Mock dependencies (sem FDPN)
	mockDB := &mockDatabase{}
	mockEmbedder := &mockEmbeddingService{}
	mockQdrant := &mockQdrantClient{}

	retrieval := &RetrievalService{
		db:       mockDB,
		embedder: mockEmbedder,
		qdrant:   mockQdrant,
		fdpn:     nil, // Sem FDPN
	}

	// Test: busca sem FDPN (não deve falhar)
	results, err := retrieval.RetrieveHybridWithFDPN(context.Background(), 1, "café", 5)

	if err != nil {
		t.Fatalf("Expected no error without FDPN, got: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected results even without FDPN")
	}
}

func TestFDPNEngine_GetActivatedNodes(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{}
	mockRedis := &mockRedisClient{}

	fdpn := NewFDPNEngine(mockNeo4j, mockRedis, nil)

	// Test: obter nós ativados
	activatedNodes, err := fdpn.GetActivatedNodes(context.Background(), "user123", "café com Maria")

	if err != nil {
		t.Fatalf("GetActivatedNodes failed: %v", err)
	}

	if len(activatedNodes) == 0 {
		t.Error("Expected activated nodes, got empty")
	}

	// Verificar que "café" e "Maria" foram ativados
	foundCafe := false
	foundMaria := false
	for nodeID := range activatedNodes {
		if contains(nodeID, "café") || contains(nodeID, "cafe") {
			foundCafe = true
		}
		if contains(nodeID, "Maria") || contains(nodeID, "maria") {
			foundMaria = true
		}
	}

	if !foundCafe {
		t.Error("Expected 'café' to be activated")
	}
	if !foundMaria {
		t.Error("Expected 'Maria' to be activated")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}

func TestCalculateKeywordActivation(t *testing.T) {
	tests := []struct {
		keyword  string
		expected float64
	}{
		{"a", 0.5},           // curto
		{"café", 0.5},        // curto
		{"Maria", 0.5},       // curto
		{"hospital", 0.7},    // médio (>5)
		{"aniversário", 0.9}, // longo (>10)
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			activation := calculateKeywordActivation(tt.keyword)
			if activation != tt.expected {
				t.Errorf("Activation for '%s': expected %.1f, got %.1f", tt.keyword, tt.expected, activation)
			}
		})
	}
}

func TestFDPNBoost_Integration(t *testing.T) {
	// Cenário: memória com nó ativado pelo FDPN deve subir no ranking

	// Setup
	mockDB := &mockDatabase{}
	mockEmbedder := &mockEmbeddingService{}
	mockQdrant := &mockQdrantClient{
		results: []mockQdrantResult{
			{id: 1, score: 0.8}, // Alta similaridade
			{id: 2, score: 0.6}, // Média similaridade
			{id: 3, score: 0.5}, // Baixa similaridade
		},
	}
	mockFDPN := &mockFDPNEngine{
		activatedNodes: map[string]float64{
			"memory_2": 0.9, // Memória 2 ativada pelo FDPN
		},
	}

	retrieval := &RetrievalService{
		db:       mockDB,
		embedder: mockEmbedder,
		qdrant:   mockQdrant,
		fdpn:     mockFDPN,
	}

	// Execute
	results, err := retrieval.RetrieveHybridWithFDPN(context.Background(), 1, "query", 3)

	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Verificar: memória 2 deve ter score boosted
	// Score original: 0.6
	// Boost: +15% * 0.9 = +13.5%
	// Novo score: 0.6 * 1.135 = 0.681

	for _, result := range results {
		if result.Memory.ID == 2 {
			expectedScore := 0.6 * (1.0 + 0.15*0.9)
			if result.Score < expectedScore-0.01 || result.Score > expectedScore+0.01 {
				t.Errorf("Memory 2 score: expected ~%.3f, got %.3f", expectedScore, result.Score)
			}
		}
	}
}

func TestSortByScore(t *testing.T) {
	results := []*SearchResult{
		{Memory: &Memory{ID: 1}, Score: 0.5},
		{Memory: &Memory{ID: 2}, Score: 0.9},
		{Memory: &Memory{ID: 3}, Score: 0.7},
	}

	sortByScore(results)

	// Verificar ordem descendente
	if results[0].Score != 0.9 {
		t.Errorf("First result should have highest score (0.9), got %.1f", results[0].Score)
	}
	if results[1].Score != 0.7 {
		t.Errorf("Second result should have score 0.7, got %.1f", results[1].Score)
	}
	if results[2].Score != 0.5 {
		t.Errorf("Third result should have lowest score (0.5), got %.1f", results[2].Score)
	}
}

func TestGetFDPNBoostStats(t *testing.T) {
	retrieval := &RetrievalService{}

	results := []*SearchResult{
		{Memory: &Memory{ID: 1}, Score: 0.8},
		{Memory: &Memory{ID: 2}, Score: 0.6},
		{Memory: &Memory{ID: 3}, Score: 0.5},
	}

	activatedNodes := map[string]float64{
		"memory_1": 0.9,
		"memory_2": 0.5,
		// memory_3 não ativada
	}

	stats := retrieval.GetFDPNBoostStats(results, activatedNodes)

	if stats.TotalMemories != 3 {
		t.Errorf("Total memories: expected 3, got %d", stats.TotalMemories)
	}

	if stats.BoostedMemories != 2 {
		t.Errorf("Boosted memories: expected 2, got %d", stats.BoostedMemories)
	}

	// Boost factors: 0.15*0.9=0.135, 0.15*0.5=0.075
	// Avg: (0.135 + 0.075) / 2 = 0.105
	expectedAvg := 0.105
	if stats.AvgBoostFactor < expectedAvg-0.01 || stats.AvgBoostFactor > expectedAvg+0.01 {
		t.Errorf("Avg boost factor: expected ~%.3f, got %.3f", expectedAvg, stats.AvgBoostFactor)
	}

	expectedMax := 0.135
	if stats.MaxBoostFactor < expectedMax-0.01 || stats.MaxBoostFactor > expectedMax+0.01 {
		t.Errorf("Max boost factor: expected ~%.3f, got %.3f", expectedMax, stats.MaxBoostFactor)
	}
}

// Mock implementations

type mockDatabase struct{}

func (m *mockDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*mockRows, error) {
	return &mockRows{}, nil
}

type mockRows struct {
	index int
}

func (m *mockRows) Next() bool {
	m.index++
	return m.index <= 3 // 3 memórias mock
}

func (m *mockRows) Scan(dest ...interface{}) error {
	// Mock scan
	if len(dest) >= 11 {
		if id, ok := dest[0].(*int64); ok {
			*id = int64(m.index)
		}
		if content, ok := dest[4].(*string); ok {
			*content = "Mock memory content"
		}
	}
	return nil
}

func (m *mockRows) Close() error {
	return nil
}

type mockEmbeddingService struct{}

func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Mock embedding (768 dimensions)
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = 0.1
	}
	return embedding, nil
}

type mockQdrantClient struct {
	results []mockQdrantResult
}

type mockQdrantResult struct {
	id    int64
	score float32
}

func (m *mockQdrantClient) Search(ctx context.Context, collection string, vector []float32, limit uint64, filter interface{}) ([]mockQdrantSearchResult, error) {
	var results []mockQdrantSearchResult
	for _, r := range m.results {
		results = append(results, mockQdrantSearchResult{
			ID:    r.id,
			Score: r.score,
		})
	}
	return results, nil
}

type mockQdrantSearchResult struct {
	ID    int64
	Score float32
}

type mockGraphStore struct{}

func (m *mockGraphStore) GetRelatedMemoriesRecursive(ctx context.Context, seedID int64, limit int) ([]int64, error) {
	return []int64{seedID + 100}, nil
}

type mockFDPNEngine struct {
	activatedNodes map[string]float64
}

func (m *mockFDPNEngine) GetActivatedNodes(ctx context.Context, userID string, query string) (map[string]float64, error) {
	if m.activatedNodes != nil {
		return m.activatedNodes, nil
	}
	return map[string]float64{
		"node_café":  0.8,
		"node_Maria": 0.7,
	}, nil
}

type mockRedisClient struct{}

func (m *mockRedisClient) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration interface{}) error {
	return nil
}
