package krylov

import (
	"fmt"
	"log"
	"math"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// HierarchicalKrylov implementa compressao multi-escala inspirada na hierarquia cortical
// 4 niveis: Features(16D) -> Concepts(64D) -> Themes(256D) -> Schemas(1024D)
// Ciencia: Felleman & Van Essen (1991) - "Distributed hierarchical processing in the primate cerebral cortex"
type HierarchicalKrylov struct {
	levels    []KrylovLevel
	dimension int // Dimensao original (1536)
	mu        sync.RWMutex
}

// KrylovLevel um nivel da hierarquia cortical
type KrylovLevel struct {
	Name      string     // "features", "concepts", "themes", "schemas"
	Dimension int        // 16, 64, 256, 1024
	Basis     *mat.Dense // Matriz de base ortogonal Q (n x k)
	TimeConst float64    // Constante temporal em minutos (5, 60, 1440, 10080)
	Updates   int64
	mu        sync.RWMutex
}

// MultiScaleResult resultado de compressao multi-escala
type MultiScaleResult struct {
	Features []float64 `json:"features"` // 16D - detalhes imediatos (gato preto)
	Concepts []float64 `json:"concepts"` // 64D - objetos/acoes (passear com pet)
	Themes   []float64 `json:"themes"`   // 256D - situacoes (atividade de lazer)
	Schemas  []float64 `json:"schemas"`  // 1024D - scripts sociais (rotina de cuidado)
}

// NewHierarchicalKrylov cria um Krylov hierarquico com 4 niveis
func NewHierarchicalKrylov(dimension int) *HierarchicalKrylov {
	hk := &HierarchicalKrylov{
		dimension: dimension,
		levels: []KrylovLevel{
			{
				Name:      "features",
				Dimension: 16,
				Basis:     mat.NewDense(dimension, 16, nil),
				TimeConst: 5.0, // 5 minutos
			},
			{
				Name:      "concepts",
				Dimension: 64,
				Basis:     mat.NewDense(dimension, 64, nil),
				TimeConst: 60.0, // 1 hora
			},
			{
				Name:      "themes",
				Dimension: 256,
				Basis:     mat.NewDense(dimension, 256, nil),
				TimeConst: 1440.0, // 1 dia
			},
			{
				Name:      "schemas",
				Dimension: 1024,
				Basis:     mat.NewDense(dimension, 1024, nil),
				TimeConst: 10080.0, // 1 semana
			},
		},
	}

	return hk
}

// UpdateAllLevels adiciona um novo vetor a todos os niveis da hierarquia
func (hk *HierarchicalKrylov) UpdateAllLevels(vector []float64) error {
	hk.mu.Lock()
	defer hk.mu.Unlock()

	if len(vector) != hk.dimension {
		return fmt.Errorf("dimensao incorreta: esperado %d, recebido %d", hk.dimension, len(vector))
	}

	for i := range hk.levels {
		hk.updateLevel(&hk.levels[i], vector)
	}

	return nil
}

// updateLevel atualiza um nivel especifico com Gram-Schmidt modificado
func (hk *HierarchicalKrylov) updateLevel(level *KrylovLevel, vector []float64) {
	level.mu.Lock()
	defer level.mu.Unlock()

	vNew := mat.NewVecDense(hk.dimension, nil)
	copy(vNew.RawVector().Data, vector)

	// Gram-Schmidt Modificado contra a base deste nivel
	for j := 0; j < level.Dimension; j++ {
		basisVec := mat.Col(nil, j, level.Basis)
		if allZeroSlice(basisVec) {
			continue
		}

		qVec := mat.NewVecDense(hk.dimension, basisVec)
		dot := mat.Dot(vNew, qVec)

		tmp := mat.NewVecDense(hk.dimension, nil)
		tmp.ScaleVec(dot, qVec)
		vNew.SubVec(vNew, tmp)
	}

	norm := mat.Norm(vNew, 2)
	if norm > 1e-6 {
		vNew.ScaleVec(1.0/norm, vNew)
		colToReplace := int(level.Updates % int64(level.Dimension))
		level.Basis.SetCol(colToReplace, vNew.RawVector().Data)
		level.Updates++
	}
}

// CompressMultiLevel comprime um vetor em todos os 4 niveis
func (hk *HierarchicalKrylov) CompressMultiLevel(vector []float64) (*MultiScaleResult, error) {
	hk.mu.RLock()
	defer hk.mu.RUnlock()

	if len(vector) != hk.dimension {
		return nil, fmt.Errorf("dimensao incorreta: esperado %d, recebido %d", hk.dimension, len(vector))
	}

	result := &MultiScaleResult{}
	v := mat.NewVecDense(hk.dimension, vector)

	for i := range hk.levels {
		hk.levels[i].mu.RLock()
		compressed := mat.NewVecDense(hk.levels[i].Dimension, nil)
		compressed.MulVec(hk.levels[i].Basis.T(), v)

		data := make([]float64, hk.levels[i].Dimension)
		copy(data, compressed.RawVector().Data)
		hk.levels[i].mu.RUnlock()

		switch i {
		case 0:
			result.Features = data
		case 1:
			result.Concepts = data
		case 2:
			result.Themes = data
		case 3:
			result.Schemas = data
		}
	}

	return result, nil
}

// CompressToLevel comprime um vetor para um nivel especifico
func (hk *HierarchicalKrylov) CompressToLevel(vector []float64, levelName string) ([]float64, error) {
	hk.mu.RLock()
	defer hk.mu.RUnlock()

	if len(vector) != hk.dimension {
		return nil, fmt.Errorf("dimensao incorreta: esperado %d, recebido %d", hk.dimension, len(vector))
	}

	for i := range hk.levels {
		if hk.levels[i].Name == levelName {
			hk.levels[i].mu.RLock()
			defer hk.levels[i].mu.RUnlock()

			v := mat.NewVecDense(hk.dimension, vector)
			compressed := mat.NewVecDense(hk.levels[i].Dimension, nil)
			compressed.MulVec(hk.levels[i].Basis.T(), v)

			data := make([]float64, hk.levels[i].Dimension)
			copy(data, compressed.RawVector().Data)
			return data, nil
		}
	}

	return nil, fmt.Errorf("nivel '%s' nao encontrado", levelName)
}

// ReconstructFromLevel reconstroi vetor aproximado a partir de um nivel comprimido
func (hk *HierarchicalKrylov) ReconstructFromLevel(compressed []float64, levelName string) ([]float64, error) {
	hk.mu.RLock()
	defer hk.mu.RUnlock()

	for i := range hk.levels {
		if hk.levels[i].Name == levelName {
			hk.levels[i].mu.RLock()
			defer hk.levels[i].mu.RUnlock()

			if len(compressed) != hk.levels[i].Dimension {
				return nil, fmt.Errorf("dimensao comprimida incorreta para %s: esperado %d, recebido %d",
					levelName, hk.levels[i].Dimension, len(compressed))
			}

			c := mat.NewVecDense(hk.levels[i].Dimension, compressed)
			reconstructed := mat.NewVecDense(hk.dimension, nil)
			reconstructed.MulVec(hk.levels[i].Basis, c)

			data := make([]float64, hk.dimension)
			copy(data, reconstructed.RawVector().Data)
			return data, nil
		}
	}

	return nil, fmt.Errorf("nivel '%s' nao encontrado", levelName)
}

// SimilarityAtLevel calcula similaridade coseno entre dois vetores num nivel especifico
func (hk *HierarchicalKrylov) SimilarityAtLevel(a, b []float64, levelName string) (float64, error) {
	compA, err := hk.CompressToLevel(a, levelName)
	if err != nil {
		return 0, err
	}

	compB, err := hk.CompressToLevel(b, levelName)
	if err != nil {
		return 0, err
	}

	return cosineSim(compA, compB), nil
}

// Reorthogonalize reortogonaliza todas as bases via QR
func (hk *HierarchicalKrylov) Reorthogonalize() {
	hk.mu.Lock()
	defer hk.mu.Unlock()

	for i := range hk.levels {
		hk.levels[i].mu.Lock()
		reorthogonalizeMatrix(hk.levels[i].Basis, hk.dimension, hk.levels[i].Dimension)
		hk.levels[i].mu.Unlock()
	}

	log.Println("[HIERARCHICAL_KRYLOV] Reortogonalizacao completa em todos os 4 niveis")
}

// GetStatistics retorna estatisticas de todos os niveis
func (hk *HierarchicalKrylov) GetStatistics() map[string]interface{} {
	hk.mu.RLock()
	defer hk.mu.RUnlock()

	levels := make([]map[string]interface{}, len(hk.levels))
	for i := range hk.levels {
		hk.levels[i].mu.RLock()
		levels[i] = map[string]interface{}{
			"name":       hk.levels[i].Name,
			"dimension":  hk.levels[i].Dimension,
			"updates":    hk.levels[i].Updates,
			"time_const": hk.levels[i].TimeConst,
			"compression": fmt.Sprintf("%dD -> %dD (%.0fx)",
				hk.dimension, hk.levels[i].Dimension,
				float64(hk.dimension)/float64(hk.levels[i].Dimension)),
		}
		hk.levels[i].mu.RUnlock()
	}

	return map[string]interface{}{
		"engine":       "hierarchical_krylov",
		"original_dim": hk.dimension,
		"num_levels":   len(hk.levels),
		"levels":       levels,
		"status":       "active",
	}
}

// reorthogonalizeMatrix reortogonaliza uma matriz via QR decomposition
func reorthogonalizeMatrix(basis *mat.Dense, rows, cols int) {
	var qr mat.QR
	qr.Factorize(basis)

	var q mat.Dense
	qr.QTo(&q)

	qRows, qCols := q.Dims()
	if qRows == rows && qCols >= cols {
		for i := 0; i < rows; i++ {
			for j := 0; j < cols; j++ {
				basis.Set(i, j, q.At(i, j))
			}
		}
	}
}

// cosineSim calcula similaridade coseno entre dois slices
func cosineSim(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	dot := 0.0
	normA := 0.0
	normB := 0.0

	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}

// allZeroSlice verifica se todos os valores sao zero
func allZeroSlice(v []float64) bool {
	for _, val := range v {
		if val != 0 {
			return false
		}
	}
	return true
}
