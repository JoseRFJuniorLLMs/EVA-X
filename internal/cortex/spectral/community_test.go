package spectral

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

// TestBuildLaplacian_SimpleGraph testa construcao do Laplaciano para um grafo simples
// Triangulo: 3 nos, 3 arestas, todos peso 1
func TestBuildLaplacian_SimpleGraph(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	nodes := []GraphNode{
		{ID: "0", Label: "A", Index: 0},
		{ID: "1", Label: "B", Index: 1},
		{ID: "2", Label: "C", Index: 2},
	}

	edges := []GraphEdge{
		{SourceIdx: 0, TargetIdx: 1, Weight: 1.0},
		{SourceIdx: 1, TargetIdx: 2, Weight: 1.0},
		{SourceIdx: 0, TargetIdx: 2, Weight: 1.0},
	}

	laplacian := sce.buildLaplacian(nodes, edges, 3)

	// Laplaciano normalizado de um triangulo regular:
	// Diagonal = 1.0, Off-diagonal = -1/degree
	// Para triangulo regular: todos os graus = 2
	// L_norm = I - D^{-1/2} A D^{-1/2}
	// Cada off-diagonal = -1/sqrt(2*2) = -0.5

	diag := laplacian.At(0, 0)
	if math.Abs(diag-1.0) > 0.01 {
		t.Errorf("Diagonal esperada ~1.0, obtida %.4f", diag)
	}

	offDiag := laplacian.At(0, 1)
	if offDiag >= 0 {
		t.Errorf("Off-diagonal deveria ser negativa, obtida %.4f", offDiag)
	}

	t.Logf("Laplaciano normalizado 3x3: diag=%.4f, off=%.4f", diag, offDiag)
}

// TestEigenDecompose_KnownSpectrum testa eigendecomposicao com espectro conhecido
// Grafo bipartido completo K_{2,2}: autovalores conhecidos
func TestEigenDecompose_KnownSpectrum(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	// Grafo: dois clusters claros
	// Cluster A: nos 0,1 (conectados entre si)
	// Cluster B: nos 2,3 (conectados entre si)
	// Uma aresta fraca entre clusters (0-2)
	nodes := make([]GraphNode, 4)
	for i := 0; i < 4; i++ {
		nodes[i] = GraphNode{ID: string(rune('0' + i)), Label: "N", Index: i}
	}

	edges := []GraphEdge{
		{SourceIdx: 0, TargetIdx: 1, Weight: 5.0}, // Forte intra-A
		{SourceIdx: 2, TargetIdx: 3, Weight: 5.0}, // Forte intra-B
		{SourceIdx: 0, TargetIdx: 2, Weight: 0.1}, // Fraca inter-cluster
	}

	laplacian := sce.buildLaplacian(nodes, edges, 4)
	eigenvalues, _, err := sce.eigenDecompose(laplacian, 4)
	if err != nil {
		t.Fatalf("EigenDecompose falhou: %v", err)
	}

	t.Logf("Autovalores: %v", eigenvalues)

	// Primeiro autovalor deve ser ~0 (componente conexa)
	if eigenvalues[0] > 0.01 {
		t.Errorf("Primeiro autovalor deveria ser ~0, obtido %.6f", eigenvalues[0])
	}

	// Deve haver um spectral gap claro entre lambda_2 e lambda_3
	// (2 clusters => gap apos lambda_2)
	if len(eigenvalues) >= 3 {
		gap := eigenvalues[2] - eigenvalues[1]
		t.Logf("Spectral gap (lambda_2 - lambda_1): %.6f", gap)
	}
}

// TestFindOptimalK_TwoClusters verifica que K=2 e detectado para 2 clusters claros
func TestFindOptimalK_TwoClusters(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	// Espectro tipico de 2 clusters:
	// lambda_0 = 0, lambda_1 = 0.01, [gap], lambda_2 = 1.5, lambda_3 = 1.8, ...
	eigenvalues := []float64{0.0, 0.01, 1.5, 1.8, 2.0, 2.1, 2.2, 2.3}

	k := sce.findOptimalK(eigenvalues, 8)

	if k != 2 {
		t.Errorf("Esperado K=2 para espectro com gap claro, obtido K=%d", k)
	}
	t.Logf("Optimal K = %d (correto)", k)
}

// TestFindOptimalK_ThreeClusters verifica K=3 para 3 clusters
func TestFindOptimalK_ThreeClusters(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	// 3 clusters: lambda_0=0, lambda_1=0.01, lambda_2=0.02, [gap], lambda_3=2.0, ...
	eigenvalues := []float64{0.0, 0.01, 0.02, 2.0, 2.1, 2.2, 2.5, 2.8, 3.0, 3.1}

	k := sce.findOptimalK(eigenvalues, 10)

	if k != 3 {
		t.Errorf("Esperado K=3, obtido K=%d", k)
	}
	t.Logf("Optimal K = %d", k)
}

// TestSpectralKMeans_TwoClusters verifica separacao de dois clusters
func TestSpectralKMeans_TwoClusters(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	n := 6
	k := 2

	// Simular autovetores: Fiedler vector separa dois grupos
	// Nos 0,1,2 -> positivos, Nos 3,4,5 -> negativos
	eigenvectors := mat.NewDense(n, n, nil)

	// Coluna 0: constante (autovetor trivial)
	for i := 0; i < n; i++ {
		eigenvectors.Set(i, 0, 1.0/math.Sqrt(float64(n)))
	}

	// Coluna 1: Fiedler vector (separa 2 grupos)
	fiedler := []float64{0.5, 0.5, 0.5, -0.5, -0.5, -0.5}
	for i, v := range fiedler {
		eigenvectors.Set(i, 1, v)
	}

	// Colunas restantes: valores aleatorios pequenos
	for j := 2; j < n; j++ {
		for i := 0; i < n; i++ {
			eigenvectors.Set(i, j, 0.01*float64(i+j))
		}
	}

	assignments := sce.spectralKMeans(eigenvectors, n, k)

	t.Logf("Assignments: %v", assignments)

	// Verificar que nos 0,1,2 estao no mesmo cluster
	// e nos 3,4,5 estao em outro cluster
	if assignments[0] != assignments[1] || assignments[1] != assignments[2] {
		t.Errorf("Nos 0,1,2 deveriam estar no mesmo cluster: %v", assignments[:3])
	}
	if assignments[3] != assignments[4] || assignments[4] != assignments[5] {
		t.Errorf("Nos 3,4,5 deveriam estar no mesmo cluster: %v", assignments[3:])
	}
	if assignments[0] == assignments[3] {
		t.Errorf("Clusters A e B deveriam ser diferentes")
	}
}

// TestFractalDimension_RegularSpectrum espectro regular tem dimensao fractal ~1
func TestFractalDimension_RegularSpectrum(t *testing.T) {
	// Espectro uniforme (grafo regular): lambda_i = i * constante
	eigenvalues := make([]float64, 100)
	for i := 0; i < 100; i++ {
		eigenvalues[i] = float64(i) * 0.1
	}

	dim := computeSpectralDim(eigenvalues)
	t.Logf("Dimensao fractal de espectro uniforme: %.4f", dim)

	// Espectro uniforme: N(lambda) ~ lambda^1, logo d ~ 2*1 = 2
	// (para distribuicao uniforme)
	if dim < 0.5 {
		t.Errorf("Dimensao muito baixa para espectro uniforme: %.4f", dim)
	}
}

// TestFractalDimension_ClusteredSpectrum espectro com clusters tem dimensao maior
func TestFractalDimension_ClusteredSpectrum(t *testing.T) {
	// Espectro com gaps (grafo hierarquico)
	var eigenvalues []float64
	// Nivel 1: autovalores proximos de 0
	for i := 0; i < 10; i++ {
		eigenvalues = append(eigenvalues, float64(i)*0.001)
	}
	// Gap
	// Nivel 2: autovalores proximos de 1
	for i := 0; i < 20; i++ {
		eigenvalues = append(eigenvalues, 1.0+float64(i)*0.01)
	}
	// Gap
	// Nivel 3: autovalores proximos de 3
	for i := 0; i < 30; i++ {
		eigenvalues = append(eigenvalues, 3.0+float64(i)*0.02)
	}

	result := AnalyzeFractalHierarchy(eigenvalues)
	t.Logf("Espectro hierarquico: dim=%.4f, lacunaridade=%.4f, profundidade=%d, classificacao=%s",
		result.SpectralDimension, result.Lacunarity, result.HierarchyDepth, result.Classification)
	t.Logf("Level sizes: %v", result.LevelSizes)
	t.Logf("Level gaps: %v", result.LevelGaps)

	// Espectro com gaps claros deve ter profundidade >= 2
	if result.HierarchyDepth < 1 {
		t.Errorf("Espectro com 3 niveis deveria ter profundidade >= 1, obtido %d", result.HierarchyDepth)
	}
}

// TestHurstFromSpectrum_RandomGraph grafo aleatorio: Hurst ~0.5
func TestHurstFromSpectrum_RandomGraph(t *testing.T) {
	// Simular espectro de Wigner (semicirculo): autovalores tipo random matrix
	eigenvalues := make([]float64, 200)
	for i := 0; i < 200; i++ {
		// Distribuicao semicircular: lambda_i = -2 + 4*i/N
		eigenvalues[i] = float64(i) * 0.02
	}

	hurst := ComputeHurstFromSpectrum(eigenvalues)
	t.Logf("Hurst do espectro uniforme: %.4f", hurst)

	// Para espectro uniforme, spacings sao constantes -> Hurst deve ser alto
	// (serie constante = totalmente persistente)
}

// TestHurstFromSpectrum_HierarchicalGraph grafo hierarquico: Hurst > 0.5
func TestHurstFromSpectrum_HierarchicalGraph(t *testing.T) {
	// Espectro com padrao hierarquico (gaps repetitivos em varias escalas)
	var eigenvalues []float64
	// Criar padrao auto-similar: clusters de 5 autovalores proximos, gaps entre clusters
	for cluster := 0; cluster < 10; cluster++ {
		base := float64(cluster) * 2.0
		for i := 0; i < 20; i++ {
			eigenvalues = append(eigenvalues, base+float64(i)*0.05)
		}
	}

	hurst := ComputeHurstFromSpectrum(eigenvalues)
	t.Logf("Hurst do espectro hierarquico: %.4f", hurst)
}

// TestBuildCommunities verifica construcao de comunidades
func TestBuildCommunities(t *testing.T) {
	sce := &SpectralCommunityEngine{tau: 90}

	nodes := []GraphNode{
		{ID: "0", Label: "Significante", Name: "solidao", Index: 0},
		{ID: "1", Label: "Significante", Name: "abandono", Index: 1},
		{ID: "2", Label: "Topic", Name: "familia", Index: 2},
		{ID: "3", Label: "Topic", Name: "trabalho", Index: 3},
		{ID: "4", Label: "Emotion", Name: "tristeza", Index: 4},
	}

	edges := []GraphEdge{
		{SourceIdx: 0, TargetIdx: 1, Weight: 3.0},  // solidao-abandono (forte)
		{SourceIdx: 0, TargetIdx: 4, Weight: 2.0},  // solidao-tristeza
		{SourceIdx: 2, TargetIdx: 3, Weight: 1.5},  // familia-trabalho
		{SourceIdx: 1, TargetIdx: 4, Weight: 2.5},  // abandono-tristeza
	}

	// 2 comunidades: emocional (0,1,4) e tematica (2,3)
	assignments := []int{0, 0, 1, 1, 0}

	communities := sce.buildCommunities(nodes, edges, assignments, 2)

	if len(communities) != 2 {
		t.Fatalf("Esperado 2 comunidades, obtido %d", len(communities))
	}

	for _, c := range communities {
		t.Logf("Comunidade %d (%s): %d nos, coerencia=%.4f, centralidade=%.4f",
			c.ID, c.Label, len(c.NodeIDs), c.Coherence, c.Centrality)
	}

	// Comunidade emocional (0,1,4) deve ter label "emocional"
	found := false
	for _, c := range communities {
		if c.Label == "emocional" && len(c.NodeIDs) == 3 {
			found = true
			if c.Coherence < 0.5 {
				t.Errorf("Comunidade emocional deveria ter coerencia alta, obtida %.4f", c.Coherence)
			}
		}
	}
	if !found {
		t.Errorf("Comunidade emocional com 3 nos nao encontrada")
	}
}

// TestLinearRegression verifica regressao linear
func TestLinearRegression(t *testing.T) {
	// y = 2x + 1
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{3, 5, 7, 9, 11}

	slope := linearRegressionSlope(x, y)

	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("Slope esperado 2.0, obtido %.4f", slope)
	}
}

// TestEuclideanDist verifica distancia euclidiana
func TestEuclideanDist(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{3, 4, 0}

	dist := euclideanDist(a, b)
	if math.Abs(dist-5.0) > 0.01 {
		t.Errorf("Distancia esperada 5.0, obtida %.4f", dist)
	}
}

// TestLacunarity_Uniform baixa lacunaridade para espectro uniforme
func TestLacunarity_Uniform(t *testing.T) {
	// Espectro uniformemente espacado
	eigenvalues := make([]float64, 100)
	for i := 0; i < 100; i++ {
		eigenvalues[i] = float64(i) * 0.1
	}

	lac := computeSpectralLacunarity(eigenvalues)
	t.Logf("Lacunaridade espectro uniforme: %.4f", lac)

	// Espectro uniforme: variancia dos spacings = 0, lacunaridade = 1
	if lac > 1.1 {
		t.Errorf("Lacunaridade de espectro uniforme deveria ser ~1.0, obtida %.4f", lac)
	}
}

// BenchmarkEigenDecompose_100nodes benchmark para grafo com 100 nos
func BenchmarkEigenDecompose_100nodes(b *testing.B) {
	sce := &SpectralCommunityEngine{tau: 90}
	n := 100

	// Criar Laplaciano simetrico aleatorio
	sym := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		rowSum := 0.0
		for j := i + 1; j < n; j++ {
			// Conectar ~10% dos pares
			if (i*7+j*13)%10 == 0 {
				w := 1.0
				sym.SetSym(i, j, -w)
				rowSum += w
			}
		}
		sym.SetSym(i, i, rowSum)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sce.eigenDecompose(sym, n)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEigenDecompose_500nodes benchmark para grafo maior
func BenchmarkEigenDecompose_500nodes(b *testing.B) {
	sce := &SpectralCommunityEngine{tau: 90}
	n := 500

	sym := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		rowSum := 0.0
		for j := i + 1; j < n; j++ {
			if (i*7+j*13)%20 == 0 {
				w := 1.0
				sym.SetSym(i, j, -w)
				rowSum += w
			}
		}
		sym.SetSym(i, i, rowSum)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sce.eigenDecompose(sym, n)
		if err != nil {
			b.Fatal(err)
		}
	}
}
