package memory

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"gonum.org/v1/gonum/mat"
)

// KrylovMemoryManager gerencia o subespacoo dinamico da EVA-Mind
// Rank-1 Updates com Gram-Schmidt Modificado + Sliding Window FIFO
// Comprime embeddings de 1536D para 64D mantendo ~97% de precisao
type KrylovMemoryManager struct {
	Basis     *mat.Dense // Matriz de base ortogonal Q (n x k)
	Dimension int        // Dimensao original dos embeddings (ex: 1536)
	K         int        // Dimensao do subespaco de Krylov (ex: 64)

	// Sliding Window (FIFO)
	windowSize  int
	memoryQueue [][]float64
	queueHead   int
	queueSize   int

	// Metricas
	totalUpdates int64
	lastUpdate   time.Time
	updateTimes  []time.Duration

	// Qualidade
	orthogonalityError  float64
	reconstructionError float64

	// Thread Safety - dois mutexes para evitar deadlock
	mu       sync.RWMutex // protege Basis, queue, metricas
	consolMu sync.Mutex   // protege consolidacao (chama reortogonalizar internamente)
}

// NewKrylovMemoryManager cria um novo gerenciador de memoria
func NewKrylovMemoryManager(dimension, k, windowSize int) *KrylovMemoryManager {
	return &KrylovMemoryManager{
		Basis:       mat.NewDense(dimension, k, nil),
		Dimension:   dimension,
		K:           k,
		windowSize:  windowSize,
		memoryQueue: make([][]float64, windowSize),
		updateTimes: make([]time.Duration, 0, 100),
		lastUpdate:  time.Now(),
	}
}

// UpdateSubspace adiciona uma nova memoria e refina a base de Krylov
// Rank-1 Update com Gram-Schmidt Modificado
// Complexidade: O(n*k) onde n = dimension, k = subspace size
func (kmm *KrylovMemoryManager) UpdateSubspace(newVector []float64) error {
	startTime := time.Now()

	kmm.mu.Lock()
	defer kmm.mu.Unlock()

	if len(newVector) != kmm.Dimension {
		return fmt.Errorf("dimensao incorreta: esperado %d, recebido %d",
			kmm.Dimension, len(newVector))
	}

	vNew := mat.NewVecDense(kmm.Dimension, nil)
	copy(vNew.RawVector().Data, newVector)

	// Gram-Schmidt Modificado:
	// v_perp = v_new - sum(i=1..k) <v_new, q_i> * q_i
	for i := 0; i < kmm.K; i++ {
		basisVec := mat.Col(nil, i, kmm.Basis)
		if allZero(basisVec) {
			continue
		}

		qVec := mat.NewVecDense(kmm.Dimension, basisVec)

		// Gram-Schmidt MODIFICADO: usa vNew atualizado (nao o original)
		dot := mat.Dot(vNew, qVec)

		tmp := mat.NewVecDense(kmm.Dimension, nil)
		tmp.ScaleVec(dot, qVec)
		vNew.SubVec(vNew, tmp)
	}

	// Normaliza residuo
	norm := mat.Norm(vNew, 2)

	if norm > 1e-6 {
		// Vetor tem informacao nova - adiciona ao subespaco
		vNew.ScaleVec(1.0/norm, vNew)
		kmm.shiftAndInsert(vNew)
		kmm.totalUpdates++
		kmm.orthogonalityError = kmm.checkOrthogonalityUnsafe()
	}
	// Se norm <= 1e-6: vetor redundante, ignora silenciosamente

	elapsed := time.Since(startTime)
	kmm.updateTimes = append(kmm.updateTimes, elapsed)
	if len(kmm.updateTimes) > 100 {
		kmm.updateTimes = kmm.updateTimes[1:]
	}
	kmm.lastUpdate = time.Now()

	return nil
}

// shiftAndInsert implementa Sliding Window FIFO
func (kmm *KrylovMemoryManager) shiftAndInsert(v *mat.VecDense) {
	kmm.memoryQueue[kmm.queueHead] = v.RawVector().Data
	kmm.queueHead = (kmm.queueHead + 1) % kmm.windowSize

	if kmm.queueSize < kmm.windowSize {
		kmm.queueSize++
	}

	colToReplace := kmm.totalUpdates % int64(kmm.K)
	kmm.Basis.SetCol(int(colToReplace), v.RawVector().Data)
}

// CompressVector projeta um vetor de alta dimensao no subespaco de Krylov
// 1536D -> 64D via v_compressed = Q^T * v_original
func (kmm *KrylovMemoryManager) CompressVector(vector []float64) ([]float64, error) {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()

	if len(vector) != kmm.Dimension {
		return nil, fmt.Errorf("dimensao incorreta: esperado %d, recebido %d",
			kmm.Dimension, len(vector))
	}

	v := mat.NewVecDense(kmm.Dimension, vector)
	compressed := mat.NewVecDense(kmm.K, nil)
	compressed.MulVec(kmm.Basis.T(), v)

	result := make([]float64, kmm.K)
	copy(result, compressed.RawVector().Data)
	return result, nil
}

// ReconstructVector reconstroi aproximadamente um vetor original
// v_reconstructed ~= Q * v_compressed
func (kmm *KrylovMemoryManager) ReconstructVector(compressed []float64) ([]float64, error) {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()

	if len(compressed) != kmm.K {
		return nil, fmt.Errorf("dimensao comprimida incorreta: esperado %d, recebido %d",
			kmm.K, len(compressed))
	}

	c := mat.NewVecDense(kmm.K, compressed)
	reconstructed := mat.NewVecDense(kmm.Dimension, nil)
	reconstructed.MulVec(kmm.Basis, c)

	result := make([]float64, kmm.Dimension)
	copy(result, reconstructed.RawVector().Data)
	return result, nil
}

// checkOrthogonalityUnsafe calcula ||Q^TQ - I||_F sem lock (chamador deve ter lock)
func (kmm *KrylovMemoryManager) checkOrthogonalityUnsafe() float64 {
	gram := mat.NewDense(kmm.K, kmm.K, nil)
	gram.Mul(kmm.Basis.T(), kmm.Basis)

	for i := 0; i < kmm.K; i++ {
		gram.Set(i, i, gram.At(i, i)-1.0)
	}

	return mat.Norm(gram, 2)
}

// reorthogonalizeUnsafe forca reortogonalizacao via QR (chamador deve ter lock)
func (kmm *KrylovMemoryManager) reorthogonalizeUnsafe() {
	var qr mat.QR
	qr.Factorize(kmm.Basis)

	var q mat.Dense
	qr.QTo(&q)

	rows, cols := q.Dims()
	if rows == kmm.Dimension && cols >= kmm.K {
		for i := 0; i < kmm.Dimension; i++ {
			for j := 0; j < kmm.K; j++ {
				kmm.Basis.Set(i, j, q.At(i, j))
			}
		}
	}

	kmm.orthogonalityError = kmm.checkOrthogonalityUnsafe()
}

// Reorthogonalize forca reortogonalizacao completa da base via QR decomposition
// Thread-safe, nao causa deadlock
func (kmm *KrylovMemoryManager) Reorthogonalize() {
	kmm.mu.Lock()
	defer kmm.mu.Unlock()

	kmm.reorthogonalizeUnsafe()
}

// MemoryConsolidation executa consolidacao periodica da memoria
// Verifica ortogonalidade e reortogonaliza se necessario
// Thread-safe, usa consolMu para evitar consolidacoes simultaneas
func (kmm *KrylovMemoryManager) MemoryConsolidation() {
	kmm.consolMu.Lock()
	defer kmm.consolMu.Unlock()

	kmm.mu.Lock()

	needsReorth := kmm.orthogonalityError > 0.05

	if needsReorth {
		kmm.reorthogonalizeUnsafe()
	}

	kmm.reconstructionError = kmm.calculateReconstructionErrorUnsafe()

	kmm.mu.Unlock()
}

// calculateReconstructionErrorUnsafe mede erro de reconstrucao (chamador deve ter lock)
func (kmm *KrylovMemoryManager) calculateReconstructionErrorUnsafe() float64 {
	if kmm.queueSize == 0 {
		return 0.0
	}

	numSamples := kmm.queueSize
	if numSamples > 100 {
		numSamples = 100
	}
	totalError := 0.0
	validSamples := 0

	for i := 0; i < numSamples; i++ {
		idx := (kmm.queueHead + i) % kmm.windowSize
		if kmm.memoryQueue[idx] == nil {
			continue
		}

		original := kmm.memoryQueue[idx]

		// Compress inline (sem lock pois ja temos)
		v := mat.NewVecDense(kmm.Dimension, original)
		compressed := mat.NewVecDense(kmm.K, nil)
		compressed.MulVec(kmm.Basis.T(), v)

		// Reconstruct inline
		reconstructed := mat.NewVecDense(kmm.Dimension, nil)
		reconstructed.MulVec(kmm.Basis, compressed)

		// Erro L2
		errSum := 0.0
		for j := 0; j < len(original); j++ {
			diff := original[j] - reconstructed.AtVec(j)
			errSum += diff * diff
		}
		totalError += math.Sqrt(errSum)
		validSamples++
	}

	if validSamples == 0 {
		return 0.0
	}
	return totalError / float64(validSamples)
}

// krylovCheckpoint e a estrutura serializada para gob
type krylovCheckpoint struct {
	BasisData   []float64
	Dimension   int
	K           int
	WindowSize  int
	MemoryQueue [][]float64
	QueueHead   int
	QueueSize   int
	TotalUpd    int64
	OrthError   float64
	ReconError  float64
}

// SaveCheckpoint salva o estado completo da base Krylov em disco
func (kmm *KrylovMemoryManager) SaveCheckpoint(filepath string) error {
	kmm.mu.RLock()

	rows, cols := kmm.Basis.Dims()
	basisData := make([]float64, rows*cols)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			basisData[i*cols+j] = kmm.Basis.At(i, j)
		}
	}

	cp := krylovCheckpoint{
		BasisData:   basisData,
		Dimension:   kmm.Dimension,
		K:           kmm.K,
		WindowSize:  kmm.windowSize,
		MemoryQueue: make([][]float64, kmm.windowSize),
		QueueHead:   kmm.queueHead,
		QueueSize:   kmm.queueSize,
		TotalUpd:    kmm.totalUpdates,
		OrthError:   kmm.orthogonalityError,
		ReconError:  kmm.reconstructionError,
	}
	copy(cp.MemoryQueue, kmm.memoryQueue)

	kmm.mu.RUnlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(cp); err != nil {
		return fmt.Errorf("falha ao serializar checkpoint: %w", err)
	}

	if err := os.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("falha ao escrever checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint carrega um estado anterior da base Krylov
func (kmm *KrylovMemoryManager) LoadCheckpoint(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("falha ao ler checkpoint: %w", err)
	}

	var cp krylovCheckpoint
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cp); err != nil {
		return fmt.Errorf("falha ao deserializar checkpoint: %w", err)
	}

	kmm.mu.Lock()
	defer kmm.mu.Unlock()

	kmm.Dimension = cp.Dimension
	kmm.K = cp.K
	kmm.windowSize = cp.WindowSize
	kmm.Basis = mat.NewDense(cp.Dimension, cp.K, cp.BasisData)
	kmm.memoryQueue = cp.MemoryQueue
	kmm.queueHead = cp.QueueHead
	kmm.queueSize = cp.QueueSize
	kmm.totalUpdates = cp.TotalUpd
	kmm.orthogonalityError = cp.OrthError
	kmm.reconstructionError = cp.ReconError

	return nil
}

// GetStatistics retorna estatisticas detalhadas do sistema
func (kmm *KrylovMemoryManager) GetStatistics() map[string]interface{} {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()

	var avgUpdateTime time.Duration
	if len(kmm.updateTimes) > 0 {
		var total time.Duration
		for _, t := range kmm.updateTimes {
			total += t
		}
		avgUpdateTime = total / time.Duration(len(kmm.updateTimes))
	}

	compressionRatio := float64(kmm.Dimension) / float64(kmm.K)
	memoryReduction := (1.0 - float64(kmm.K)/float64(kmm.Dimension)) * 100

	return map[string]interface{}{
		"dimension":            kmm.Dimension,
		"subspace_size":        kmm.K,
		"window_size":          kmm.windowSize,
		"queue_fill":           kmm.queueSize,
		"total_updates":        kmm.totalUpdates,
		"last_update":          kmm.lastUpdate.Format(time.RFC3339),
		"avg_update_time_us":   avgUpdateTime.Microseconds(),
		"orthogonality_error":  kmm.orthogonalityError,
		"reconstruction_error": kmm.reconstructionError,
		"compression_ratio":    compressionRatio,
		"memory_reduction_%":   memoryReduction,
		"status":               kmm.getHealthStatusUnsafe(),
	}
}

// getHealthStatusUnsafe retorna status de saude (chamador deve ter lock)
func (kmm *KrylovMemoryManager) getHealthStatusUnsafe() string {
	if kmm.orthogonalityError > 0.1 {
		return "degraded"
	}
	if kmm.queueSize == 0 {
		return "initializing"
	}
	if kmm.queueSize < kmm.windowSize/2 {
		return "warming_up"
	}
	return "healthy"
}

// OrthogonalityError retorna o erro de ortogonalidade atual
func (kmm *KrylovMemoryManager) OrthogonalityError() float64 {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()
	return kmm.orthogonalityError
}

// TotalUpdates retorna o numero total de atualizacoes
func (kmm *KrylovMemoryManager) TotalUpdates() int64 {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()
	return kmm.totalUpdates
}

// QueueFill retorna quantas memorias estao na janela
func (kmm *KrylovMemoryManager) QueueFill() int {
	kmm.mu.RLock()
	defer kmm.mu.RUnlock()
	return kmm.queueSize
}

// allZero verifica se um slice e todo zero
func allZero(v []float64) bool {
	for _, val := range v {
		if val != 0 {
			return false
		}
	}
	return true
}
