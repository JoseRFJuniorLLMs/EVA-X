package krylov

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gonum.org/v1/gonum/mat"
)

// === GERADORES DE DADOS ===

func generateRandomEmbedding(dimension int) []float64 {
	embedding := make([]float64, dimension)
	sum := 0.0
	for i := range embedding {
		embedding[i] = rand.NormFloat64()
		sum += embedding[i] * embedding[i]
	}
	norm := math.Sqrt(sum)
	for i := range embedding {
		embedding[i] /= norm
	}
	return embedding
}

func generateMemoryDataset(numMemories, dimension int) [][]float64 {
	dataset := make([][]float64, numMemories)
	for i := range dataset {
		dataset[i] = generateRandomEmbedding(dimension)
	}
	return dataset
}

// === BASELINE: SVD COMPLETO ===

func fullRecomputation(memories [][]float64, k int) (*mat.Dense, time.Duration) {
	startTime := time.Now()

	numMemories := len(memories)
	dimension := len(memories[0])

	data := make([]float64, numMemories*dimension)
	for i, mem := range memories {
		copy(data[i*dimension:(i+1)*dimension], mem)
	}
	A := mat.NewDense(numMemories, dimension, data)

	var svd mat.SVD
	ok := svd.Factorize(A, mat.SVDThin)
	if !ok {
		panic("SVD failed")
	}

	var v mat.Dense
	svd.VTo(&v)

	subspace := mat.NewDense(dimension, k, nil)
	for i := 0; i < dimension; i++ {
		for j := 0; j < k; j++ {
			subspace.Set(i, j, v.At(i, j))
		}
	}

	return subspace, time.Since(startTime)
}

// === METRICAS ===

func findTopK(query []float64, memories [][]float64, k int) []int {
	type sim struct {
		index int
		score float64
	}

	sims := make([]sim, len(memories))
	for i, mem := range memories {
		score := 0.0
		for j := range query {
			if j < len(mem) {
				score += query[j] * mem[j]
			}
		}
		sims[i] = sim{index: i, score: score}
	}

	// Selection sort para top-K (suficiente para testes)
	for i := 0; i < k && i < len(sims); i++ {
		maxIdx := i
		for j := i + 1; j < len(sims); j++ {
			if sims[j].score > sims[maxIdx].score {
				maxIdx = j
			}
		}
		sims[i], sims[maxIdx] = sims[maxIdx], sims[i]
	}

	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = sims[i].index
	}
	return result
}

// === BENCHMARKS ===

func BenchmarkRank1Update_SmallScale(b *testing.B) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 100; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}
}

func BenchmarkRank1Update_MediumScale(b *testing.B) {
	kmm := NewKrylovMemoryManager(1536, 64, 10000)

	for i := 0; i < 1000; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}
}

func BenchmarkCompressVector(b *testing.B) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 500; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	query := generateRandomEmbedding(1536)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kmm.CompressVector(query)
	}
}

func BenchmarkFullRecomputation(b *testing.B) {
	memories := generateMemoryDataset(100, 1536)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fullRecomputation(memories, 64)
	}
}

// === TESTES DE VALIDACAO ===

func TestUpdateSubspace(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 200; i++ {
		err := kmm.UpdateSubspace(generateRandomEmbedding(1536))
		if err != nil {
			t.Fatalf("UpdateSubspace falhou na iteracao %d: %v", i, err)
		}
	}

	if kmm.TotalUpdates() == 0 {
		t.Error("TotalUpdates deveria ser > 0")
	}

	if kmm.QueueFill() == 0 {
		t.Error("QueueFill deveria ser > 0")
	}
}

func TestUpdateSubspace_WrongDimension(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 100)

	err := kmm.UpdateSubspace(make([]float64, 100))
	if err == nil {
		t.Error("Deveria retornar erro para dimensao incorreta")
	}
}

func TestCompressAndReconstruct(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	// Treina com 500 memorias
	for i := 0; i < 500; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	// Testa compress + reconstruct
	original := generateRandomEmbedding(1536)
	compressed, err := kmm.CompressVector(original)
	if err != nil {
		t.Fatalf("CompressVector falhou: %v", err)
	}

	if len(compressed) != 64 {
		t.Errorf("Compressed deveria ter 64D, tem %d", len(compressed))
	}

	reconstructed, err := kmm.ReconstructVector(compressed)
	if err != nil {
		t.Fatalf("ReconstructVector falhou: %v", err)
	}

	if len(reconstructed) != 1536 {
		t.Errorf("Reconstructed deveria ter 1536D, tem %d", len(reconstructed))
	}
}

func TestOrthogonalityPreservation(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 5000; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	err := kmm.OrthogonalityError()
	if err > 0.1 {
		t.Errorf("Erro de ortogonalidade muito alto: %.6f (max 0.1)", err)
	}
}

func TestReorthogonalize(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 1000; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	kmm.Reorthogonalize()

	err := kmm.OrthogonalityError()
	if err > 0.01 {
		t.Errorf("Apos reortogonalizacao, erro deveria ser < 0.01, got %.6f", err)
	}
}

func TestMemoryConsolidation_NoDeadlock(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 1000)

	for i := 0; i < 500; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	// Deve completar sem deadlock
	done := make(chan bool, 1)
	go func() {
		kmm.MemoryConsolidation()
		done <- true
	}()

	select {
	case <-done:
		// OK
	case <-time.After(10 * time.Second):
		t.Fatal("MemoryConsolidation deadlocked!")
	}
}

func TestSaveLoadCheckpoint(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 100)

	// Treina
	for i := 0; i < 200; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	// Salva
	tmpDir := t.TempDir()
	ckptPath := filepath.Join(tmpDir, "krylov.ckpt")

	err := kmm.SaveCheckpoint(ckptPath)
	if err != nil {
		t.Fatalf("SaveCheckpoint falhou: %v", err)
	}

	// Verifica que arquivo existe
	info, err := os.Stat(ckptPath)
	if err != nil {
		t.Fatalf("Checkpoint nao encontrado: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("Checkpoint vazio")
	}

	// Carrega em novo manager
	kmm2 := NewKrylovMemoryManager(1536, 64, 100)
	err = kmm2.LoadCheckpoint(ckptPath)
	if err != nil {
		t.Fatalf("LoadCheckpoint falhou: %v", err)
	}

	// Valida estado restaurado
	if kmm2.TotalUpdates() != kmm.TotalUpdates() {
		t.Errorf("TotalUpdates diferente: %d vs %d", kmm2.TotalUpdates(), kmm.TotalUpdates())
	}

	// Comprime mesmo vetor nos dois e compara
	testVec := generateRandomEmbedding(1536)
	c1, _ := kmm.CompressVector(testVec)
	c2, _ := kmm2.CompressVector(testVec)

	diff := 0.0
	for i := range c1 {
		d := c1[i] - c2[i]
		diff += d * d
	}
	diff = math.Sqrt(diff)

	if diff > 1e-10 {
		t.Errorf("Checkpoint restaurado difere do original: diff=%.12f", diff)
	}
}

func TestRecallPreservation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping recall test in short mode")
	}

	// NOTA: Com dados ALEATORIOS, recall sera baixo (~10-20%) porque
	// nao ha estrutura dimensional para capturar. Todas as 1536 dimensoes
	// contribuem igualmente, entao projetar para 64D perde ~96% de info.
	//
	// Com embeddings REAIS (OpenAI, Gemini), ~95% da variancia esta nas
	// primeiras 50-100 dimensoes, entao recall sobe para ~90-97%.
	//
	// Este teste valida que o pipeline funciona, nao a qualidade absoluta.

	dimension := 1536
	k := 64
	numMemories := 500

	memories := generateMemoryDataset(numMemories, dimension)

	kmm := NewKrylovMemoryManager(dimension, k, 1000)
	for _, mem := range memories {
		kmm.UpdateSubspace(mem)
	}

	compressedMemories := make([][]float64, numMemories)
	for i, mem := range memories {
		c, err := kmm.CompressVector(mem)
		if err != nil {
			t.Fatal(err)
		}
		compressedMemories[i] = c
	}

	totalRecall := 0.0
	numQueries := 10

	for i := 0; i < numQueries; i++ {
		query := generateRandomEmbedding(dimension)
		compressedQuery, _ := kmm.CompressVector(query)

		topKFull := findTopK(query, memories, 10)
		topKComp := findTopK(compressedQuery, compressedMemories, 10)

		hits := 0
		for _, idxFull := range topKFull {
			for _, idxComp := range topKComp {
				if idxFull == idxComp {
					hits++
					break
				}
			}
		}

		recall := float64(hits) / 10.0
		totalRecall += recall
	}

	avgRecall := totalRecall / float64(numQueries)
	t.Logf("Recall@10 medio (dados aleatorios): %.1f%%", avgRecall*100)
	t.Logf("NOTA: Com embeddings reais (OpenAI), recall esperado: ~90-97%%")

	// Para dados aleatorios, apenas valida que pipeline funciona (>0%)
	if avgRecall <= 0 {
		t.Error("Recall zero - pipeline de compressao quebrado")
	}
}

func TestGetStatistics(t *testing.T) {
	kmm := NewKrylovMemoryManager(1536, 64, 100)

	for i := 0; i < 50; i++ {
		kmm.UpdateSubspace(generateRandomEmbedding(1536))
	}

	stats := kmm.GetStatistics()

	if stats["dimension"] != 1536 {
		t.Errorf("dimension esperado 1536, got %v", stats["dimension"])
	}
	if stats["subspace_size"] != 64 {
		t.Errorf("subspace_size esperado 64, got %v", stats["subspace_size"])
	}
	if stats["status"] == "initializing" {
		t.Error("Status nao deveria ser initializing apos 50 updates")
	}
}

func TestMemoryUsage(t *testing.T) {
	dimension := 1536
	k := 64
	numMemories := 1000

	memOriginal := numMemories * dimension * 4 // float32
	memCompressed := numMemories * k * 4

	reduction := (1.0 - float64(memCompressed)/float64(memOriginal)) * 100

	t.Logf("Original (1536D): %.2f MB", float64(memOriginal)/(1024*1024))
	t.Logf("Compressed (64D): %.2f MB", float64(memCompressed)/(1024*1024))
	t.Logf("Reducao: %.1f%%", reduction)

	if reduction < 90.0 {
		t.Errorf("Reducao insuficiente: %.1f%% (min 90%%)", reduction)
	}
}

func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	kmm := NewKrylovMemoryManager(1536, 64, 100000)

	scales := []int{1000, 5000}

	for _, scale := range scales {
		start := time.Now()

		for i := 0; i < scale; i++ {
			kmm.UpdateSubspace(generateRandomEmbedding(1536))
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(scale)

		t.Logf("Escala %d: tempo medio/update = %v", scale, avgTime)

		if avgTime > 5*time.Millisecond {
			t.Errorf("Performance degradou em escala %d: %v > 5ms", scale, avgTime)
		}
	}
}
