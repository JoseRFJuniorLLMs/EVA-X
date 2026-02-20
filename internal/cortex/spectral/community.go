// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package spectral

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"gonum.org/v1/gonum/mat"
)

// MemoryCommunity representa uma comunidade de memorias descoberta por clustering espectral
type MemoryCommunity struct {
	ID          int      `json:"id"`
	Label       string   `json:"label"`       // Nome inferido da comunidade
	NodeIDs     []string `json:"node_ids"`     // IDs dos nos nesta comunidade
	NodeLabels  []string `json:"node_labels"`  // Labels dos nos (Topic, Emotion, etc.)
	Coherence   float64  `json:"coherence"`    // Coesao interna (0-1)
	Centrality  float64  `json:"centrality"`   // Importancia relativa no grafo
	DominantAge float64  `json:"dominant_age"` // Idade dominante em dias
}

// SpectralAnalysis resultado completo da analise espectral
type SpectralAnalysis struct {
	IdosoID            int64             `json:"idoso_id"`
	Communities        []MemoryCommunity `json:"communities"`
	OptimalK           int               `json:"optimal_k"`           // Numero otimo de comunidades
	FiedlerValue       float64           `json:"fiedler_value"`       // 2o menor autovalor (conectividade algebrica)
	SpectralGap        float64           `json:"spectral_gap"`        // Gap entre lambda_k e lambda_{k+1}
	FractalDimension   float64           `json:"fractal_dimension"`   // Dimensao fractal do espectro
	GraphConnectedness float64           `json:"graph_connectedness"` // Quao conectado e o grafo (0-1)
	Eigenvalues        []float64         `json:"eigenvalues"`         // Primeiros autovalores para debug
	NodeCount          int               `json:"node_count"`
	EdgeCount          int               `json:"edge_count"`
}

// GraphNode no do grafo de memoria
type GraphNode struct {
	ID    string
	Label string // Person, Event, Significante, Topic, Emotion, etc.
	Name  string
	Index int // Indice na matriz de adjacencia
}

// GraphEdge aresta com peso temporal
type GraphEdge struct {
	SourceIdx int
	TargetIdx int
	Weight    float64 // Peso com decay temporal aplicado
	Type      string
}

// SpectralCommunityEngine motor de clustering espectral para o grafo de memorias
// Usa o Laplaciano do grafo (L = D - A) + autovetores para descobrir comunidades naturais
// Krylov e o coracao: os autovetores do Laplaciano vivem num subespaco de Krylov
type SpectralCommunityEngine struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	tau          float64 // Constante de decay temporal (mesma do TemporalDecayService)

	mu sync.RWMutex
}

// NewSpectralCommunityEngine cria o motor de clustering espectral
func NewSpectralCommunityEngine(graphAdapter *nietzscheInfra.GraphAdapter, tauDays float64) *SpectralCommunityEngine {
	if tauDays <= 0 {
		tauDays = 90
	}
	return &SpectralCommunityEngine{
		graphAdapter: graphAdapter,
		tau:          tauDays,
	}
}

// AnalyzeCommunities executa analise espectral completa do grafo de memorias de um paciente
// Pipeline: NietzscheDB -> Adjacency Matrix -> Laplacian -> Eigendecomposition -> Clustering -> Fractal Analysis
func (sce *SpectralCommunityEngine) AnalyzeCommunities(ctx context.Context, idosoID int64) (*SpectralAnalysis, error) {
	sce.mu.Lock()
	defer sce.mu.Unlock()

	// 1. Buscar grafo (nos + arestas com decay)
	nodes, edges, err := sce.fetchGraph(ctx, idosoID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar grafo: %w", err)
	}

	n := len(nodes)
	if n < 3 {
		return &SpectralAnalysis{
			IdosoID:   idosoID,
			NodeCount: n,
			EdgeCount: len(edges),
			OptimalK:  1,
			Communities: []MemoryCommunity{{
				ID: 0, Label: "unica", NodeIDs: nodeIDs(nodes),
				NodeLabels: nodeLabels(nodes), Coherence: 1.0,
			}},
		}, nil
	}

	// 2. Construir Laplaciano L = D - A (com pesos temporais)
	laplacian := sce.buildLaplacian(nodes, edges, n)

	// 3. Eigendecomposition (Laplaciano e simetrico -> EigenSym)
	eigenvalues, eigenvectors, err := sce.eigenDecompose(laplacian, n)
	if err != nil {
		return nil, fmt.Errorf("falha na decomposicao espectral: %w", err)
	}

	// 4. Determinar numero otimo de comunidades (spectral gap)
	optK := sce.findOptimalK(eigenvalues, n)

	// 5. Clustering via k-means nos autovetores do Fiedler
	assignments := sce.spectralKMeans(eigenvectors, n, optK)

	// 6. Montar comunidades
	communities := sce.buildCommunities(nodes, edges, assignments, optK)

	// 7. Dimensao fractal do espectro de autovalores
	fractalDim := sce.computeSpectralFractalDimension(eigenvalues)

	// 8. Metricas globais
	fiedlerValue := 0.0
	spectralGap := 0.0
	if len(eigenvalues) > 1 {
		fiedlerValue = eigenvalues[1] // 2o menor = conectividade algebrica
	}
	if optK < len(eigenvalues) {
		spectralGap = eigenvalues[optK] - eigenvalues[optK-1]
	}

	// Conectividade normalizada
	connectedness := 0.0
	if n > 1 {
		connectedness = fiedlerValue / float64(n)
	}

	// Retornar primeiros 20 autovalores para debug
	topEigen := eigenvalues
	if len(topEigen) > 20 {
		topEigen = topEigen[:20]
	}

	analysis := &SpectralAnalysis{
		IdosoID:            idosoID,
		Communities:        communities,
		OptimalK:           optK,
		FiedlerValue:       fiedlerValue,
		SpectralGap:        spectralGap,
		FractalDimension:   fractalDim,
		GraphConnectedness: connectedness,
		Eigenvalues:        topEigen,
		NodeCount:          n,
		EdgeCount:          len(edges),
	}

	log.Printf("[SPECTRAL] Paciente %d: %d nos, %d arestas, %d comunidades, fractal_dim=%.4f, fiedler=%.6f",
		idosoID, n, len(edges), optK, fractalDim, fiedlerValue)

	return analysis, nil
}

// fetchGraph busca todos os nos e arestas relevantes do grafo de um paciente
// Rewritten: uses BFS from patient node to discover neighbors (replaces *1..2 path)
func (sce *SpectralCommunityEngine) fetchGraph(ctx context.Context, idosoID int64) ([]GraphNode, []GraphEdge, error) {
	// Find patient node
	patientResult, err := sce.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": idosoID,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("falha ao encontrar paciente: %w", err)
	}

	// BFS from patient up to depth 2
	neighborIDs, err := sce.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return nil, nil, fmt.Errorf("falha ao buscar nos: %w", err)
	}

	nodeIndex := make(map[string]int)
	var nodes []GraphNode

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue // Skip patient node itself
		}
		if _, exists := nodeIndex[nID]; exists {
			continue
		}

		// Get node details
		nodeResult, err := sce.graphAdapter.GetNode(ctx, nID, "")
		if err != nil {
			continue
		}

		idx := len(nodes)
		nodeIndex[nID] = idx

		nodeLabel := "Unknown"
		if nt, ok := nodeResult.Content["node_type"].(string); ok {
			nodeLabel = nt
		} else if nodeResult.NodeType != "" {
			nodeLabel = nodeResult.NodeType
		}

		nodeName := nID
		if name, ok := nodeResult.Content["name"].(string); ok {
			nodeName = name
		} else if word, ok := nodeResult.Content["word"].(string); ok {
			nodeName = word
		} else if content, ok := nodeResult.Content["content"].(string); ok {
			nodeName = content
		}

		nodes = append(nodes, GraphNode{
			ID:    nID,
			Label: nodeLabel,
			Name:  nodeName,
			Index: idx,
		})
	}

	// Build edges: for each node, find its direct neighbors and record edges
	var edges []GraphEdge
	edgeSeen := make(map[string]bool)

	for _, node := range nodes {
		directNeighbors, err := sce.graphAdapter.Bfs(ctx, node.ID, 1, "")
		if err != nil {
			continue
		}
		for _, neighborID := range directNeighbors {
			if neighborID == node.ID || neighborID == patientResult.NodeID {
				continue
			}
			dstIdx, dstOK := nodeIndex[neighborID]
			srcIdx := nodeIndex[node.ID]
			if !dstOK {
				continue
			}

			// Avoid duplicate edges
			edgeKey := fmt.Sprintf("%d-%d", srcIdx, dstIdx)
			reverseKey := fmt.Sprintf("%d-%d", dstIdx, srcIdx)
			if edgeSeen[edgeKey] || edgeSeen[reverseKey] {
				continue
			}
			edgeSeen[edgeKey] = true

			// Default weight with decay approximation
			w := 1.0

			edges = append(edges, GraphEdge{
				SourceIdx: srcIdx,
				TargetIdx: dstIdx,
				Weight:    w,
				Type:      "RELATED",
			})
		}
	}

	return nodes, edges, nil
}

// buildLaplacian constroi Laplaciano normalizado L = D^{-1/2} (D - A) D^{-1/2}
// Usa pesos com decay temporal: arestas recentes pesam mais
func (sce *SpectralCommunityEngine) buildLaplacian(nodes []GraphNode, edges []GraphEdge, n int) *mat.SymDense {
	// Matriz de adjacencia ponderada
	adj := mat.NewDense(n, n, nil)
	for _, e := range edges {
		// Grafo nao-direcionado: simetrico
		current := adj.At(e.SourceIdx, e.TargetIdx)
		adj.Set(e.SourceIdx, e.TargetIdx, current+e.Weight)
		adj.Set(e.TargetIdx, e.SourceIdx, current+e.Weight)
	}

	// Grau de cada no (soma dos pesos das arestas)
	degree := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			degree[i] += adj.At(i, j)
		}
	}

	// L = D - A (Laplaciano combinatorio)
	laplacianData := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			idx := i*n + j
			if i == j {
				laplacianData[idx] = degree[i]
			} else {
				laplacianData[idx] = -adj.At(i, j)
			}
		}
	}

	// Normalizacao: L_norm = D^{-1/2} L D^{-1/2}
	// Melhor para clustering (autovalores em [0, 2])
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			idx := i*n + j
			di := degree[i]
			dj := degree[j]
			if di > 1e-10 && dj > 1e-10 {
				laplacianData[idx] /= math.Sqrt(di * dj)
			}
		}
	}

	// Converter para SymDense (Laplaciano e simetrico por construcao)
	sym := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			val := laplacianData[i*n+j]
			sym.SetSym(i, j, val)
		}
	}

	return sym
}

// eigenDecompose calcula autovalores e autovetores do Laplaciano simetrico
func (sce *SpectralCommunityEngine) eigenDecompose(laplacian *mat.SymDense, n int) ([]float64, *mat.Dense, error) {
	var eigSym mat.EigenSym
	ok := eigSym.Factorize(laplacian, true)
	if !ok {
		return nil, nil, fmt.Errorf("factorizacao eigenSym falhou")
	}

	eigenvalues := eigSym.Values(nil)
	eigenvectors := mat.NewDense(n, n, nil)
	eigSym.VectorsTo(eigenvectors)

	// Ordenar por autovalor crescente (menores primeiro)
	// gonum ja retorna ordenado, mas garantir
	type eigenPair struct {
		value  float64
		vecIdx int
	}
	pairs := make([]eigenPair, n)
	for i := 0; i < n; i++ {
		pairs[i] = eigenPair{value: eigenvalues[i], vecIdx: i}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})

	sortedVals := make([]float64, n)
	sortedVecs := mat.NewDense(n, n, nil)
	for newIdx, pair := range pairs {
		sortedVals[newIdx] = pair.value
		for row := 0; row < n; row++ {
			sortedVecs.Set(row, newIdx, eigenvectors.At(row, pair.vecIdx))
		}
	}

	return sortedVals, sortedVecs, nil
}

// findOptimalK determina numero otimo de comunidades via spectral gap
// O maior gap entre autovalores consecutivos indica a "fronteira natural" entre comunidades
func (sce *SpectralCommunityEngine) findOptimalK(eigenvalues []float64, n int) int {
	if n <= 3 {
		return 1
	}

	maxK := n / 2
	if maxK > 15 {
		maxK = 15 // Maximo razoavel de comunidades
	}
	if maxK < 2 {
		maxK = 2
	}

	// Encontrar maior spectral gap (ignorar lambda_0 que e sempre 0)
	bestGap := 0.0
	bestK := 2

	for k := 2; k < maxK && k < len(eigenvalues); k++ {
		gap := eigenvalues[k] - eigenvalues[k-1]
		// Penalizar K muito alto (parcimonia)
		adjustedGap := gap * math.Exp(-0.1*float64(k-2))
		if adjustedGap > bestGap {
			bestGap = adjustedGap
			bestK = k
		}
	}

	return bestK
}

// spectralKMeans executa k-means nos primeiros k autovetores (excluindo o 1o)
// Usa os autovetores do Fiedler como coordenadas para clustering
func (sce *SpectralCommunityEngine) spectralKMeans(eigenvectors *mat.Dense, n, k int) []int {
	if k <= 1 || n <= k {
		assignments := make([]int, n)
		return assignments
	}

	// Extrair colunas 1..k (pular coluna 0 = autovetor constante)
	features := mat.NewDense(n, k, nil)
	for i := 0; i < n; i++ {
		for j := 0; j < k; j++ {
			colIdx := j + 1 // Pular lambda_0
			if colIdx < n {
				features.Set(i, j, eigenvectors.At(i, colIdx))
			}
		}
		// Normalizar linha (cada no como ponto na esfera unitaria)
		rowNorm := 0.0
		for j := 0; j < k; j++ {
			v := features.At(i, j)
			rowNorm += v * v
		}
		if rowNorm > 1e-10 {
			rowNorm = math.Sqrt(rowNorm)
			for j := 0; j < k; j++ {
				features.Set(i, j, features.At(i, j)/rowNorm)
			}
		}
	}

	// k-means++ initialization
	centroids := make([][]float64, k)
	assignments := make([]int, n)

	// Primeiro centroide: no com maior norma no espaco espectral
	maxNorm := 0.0
	firstIdx := 0
	for i := 0; i < n; i++ {
		norm := 0.0
		for j := 0; j < k; j++ {
			v := features.At(i, j)
			norm += v * v
		}
		if norm > maxNorm {
			maxNorm = norm
			firstIdx = i
		}
	}
	centroids[0] = rowSlice(features, firstIdx, k)

	// Restantes: k-means++ (proporcional a distancia)
	for c := 1; c < k; c++ {
		distances := make([]float64, n)
		totalDist := 0.0
		for i := 0; i < n; i++ {
			minDist := math.MaxFloat64
			row := rowSlice(features, i, k)
			for cc := 0; cc < c; cc++ {
				d := euclideanDist(row, centroids[cc])
				if d < minDist {
					minDist = d
				}
			}
			distances[i] = minDist * minDist
			totalDist += distances[i]
		}
		// Escolher proximo centroide proporcional a distancia^2
		if totalDist < 1e-10 {
			centroids[c] = rowSlice(features, c, k)
			continue
		}
		cumulative := 0.0
		threshold := totalDist * 0.5 // Deterministico: mediana
		chosen := 0
		for i := 0; i < n; i++ {
			cumulative += distances[i]
			if cumulative >= threshold {
				chosen = i
				break
			}
		}
		centroids[c] = rowSlice(features, chosen, k)
	}

	// Iteracoes de Lloyd (max 50)
	for iter := 0; iter < 50; iter++ {
		changed := false

		// Assignment step
		for i := 0; i < n; i++ {
			row := rowSlice(features, i, k)
			bestC := 0
			bestDist := math.MaxFloat64
			for c := 0; c < k; c++ {
				d := euclideanDist(row, centroids[c])
				if d < bestDist {
					bestDist = d
					bestC = c
				}
			}
			if assignments[i] != bestC {
				assignments[i] = bestC
				changed = true
			}
		}

		if !changed {
			break
		}

		// Update step: recalcular centroides
		counts := make([]int, k)
		newCentroids := make([][]float64, k)
		for c := 0; c < k; c++ {
			newCentroids[c] = make([]float64, k)
		}
		for i := 0; i < n; i++ {
			c := assignments[i]
			counts[c]++
			row := rowSlice(features, i, k)
			for j := 0; j < k; j++ {
				newCentroids[c][j] += row[j]
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for j := 0; j < k; j++ {
					newCentroids[c][j] /= float64(counts[c])
				}
				centroids[c] = newCentroids[c]
			}
		}
	}

	return assignments
}

// buildCommunities monta as comunidades a partir dos assignments
func (sce *SpectralCommunityEngine) buildCommunities(nodes []GraphNode, edges []GraphEdge, assignments []int, k int) []MemoryCommunity {
	communities := make([]MemoryCommunity, k)

	// Agrupar nos por comunidade
	for i := 0; i < k; i++ {
		communities[i] = MemoryCommunity{ID: i}
	}

	for i, node := range nodes {
		c := assignments[i]
		if c < 0 || c >= k {
			continue
		}
		communities[c].NodeIDs = append(communities[c].NodeIDs, node.ID)
		communities[c].NodeLabels = append(communities[c].NodeLabels, node.Label)
	}

	// Calcular coerencia interna de cada comunidade
	// Coerencia = arestas internas / (arestas internas + arestas externas)
	for ci := range communities {
		internalWeight := 0.0
		externalWeight := 0.0
		memberSet := make(map[int]bool)
		for idx, a := range assignments {
			if a == ci {
				memberSet[idx] = true
			}
		}

		for _, e := range edges {
			srcIn := memberSet[e.SourceIdx]
			dstIn := memberSet[e.TargetIdx]
			if srcIn && dstIn {
				internalWeight += e.Weight
			} else if srcIn || dstIn {
				externalWeight += e.Weight
			}
		}

		total := internalWeight + externalWeight
		if total > 0 {
			communities[ci].Coherence = internalWeight / total
		}

		// Centralidade = fracao dos nos totais nesta comunidade
		communities[ci].Centrality = float64(len(communities[ci].NodeIDs)) / float64(len(nodes))

		// Label inferido: label mais frequente dos nos
		communities[ci].Label = sce.inferCommunityLabel(communities[ci])
	}

	// Remover comunidades vazias
	var result []MemoryCommunity
	for _, c := range communities {
		if len(c.NodeIDs) > 0 {
			result = append(result, c)
		}
	}

	// Ordenar por centralidade decrescente
	sort.Slice(result, func(i, j int) bool {
		return result[i].Centrality > result[j].Centrality
	})

	return result
}

// inferCommunityLabel infere um nome para a comunidade baseado nos tipos de nos
func (sce *SpectralCommunityEngine) inferCommunityLabel(c MemoryCommunity) string {
	labelCounts := make(map[string]int)
	for _, l := range c.NodeLabels {
		labelCounts[l]++
	}

	bestLabel := "mixed"
	bestCount := 0
	for l, count := range labelCounts {
		if count > bestCount {
			bestCount = count
			bestLabel = l
		}
	}

	switch bestLabel {
	case "Significante":
		return "emocional"
	case "Topic":
		return "tematica"
	case "Event":
		return "episodica"
	case "Emotion":
		return "afetiva"
	case "Condition", "Medication", "Symptom":
		return "clinica"
	default:
		return bestLabel
	}
}

// computeSpectralFractalDimension calcula a dimensao fractal do espectro de autovalores
// Se o espectro segue lei de potencia N(lambda) ~ lambda^{d/2}, entao d = dimensao fractal
// Metodo: Box-counting no espectro de autovalores
func (sce *SpectralCommunityEngine) computeSpectralFractalDimension(eigenvalues []float64) float64 {
	n := len(eigenvalues)
	if n < 10 {
		return 0
	}

	// Filtrar autovalores positivos (ignorar zeros numericos)
	var positiveEigs []float64
	for _, ev := range eigenvalues {
		if ev > 1e-8 {
			positiveEigs = append(positiveEigs, ev)
		}
	}

	if len(positiveEigs) < 5 {
		return 0
	}

	// Integrated Density of States (IDOS): N(lambda) = #{eigenvalues <= lambda}
	// Se fractal: log N(lambda) ~ (d/2) * log(lambda)
	// Regressao linear em log-log para encontrar d

	sort.Float64s(positiveEigs)

	// Amostrar pontos para regressao
	numPoints := len(positiveEigs)
	if numPoints > 50 {
		numPoints = 50
	}

	var logLambda []float64
	var logN []float64

	step := len(positiveEigs) / numPoints
	if step < 1 {
		step = 1
	}

	for i := 0; i < len(positiveEigs); i += step {
		lambda := positiveEigs[i]
		count := float64(i + 1) // N(lambda) = quantos autovalores <= lambda
		logLambda = append(logLambda, math.Log(lambda))
		logN = append(logN, math.Log(count))
	}

	if len(logLambda) < 3 {
		return 0
	}

	// Regressao linear: logN = slope * logLambda + intercept
	// slope = d/2, logo d = 2 * slope
	slope := linearRegressionSlope(logLambda, logN)

	fractalDim := 2.0 * slope
	if fractalDim < 0 {
		fractalDim = 0
	}
	if fractalDim > float64(n) {
		fractalDim = float64(n)
	}

	return fractalDim
}

// WriteCommunities persiste as comunidades de volta no grafo
func (sce *SpectralCommunityEngine) WriteCommunities(ctx context.Context, idosoID int64, analysis *SpectralAnalysis) error {
	if sce.graphAdapter == nil || analysis == nil {
		return nil
	}

	for _, comm := range analysis.Communities {
		for _, nodeID := range comm.NodeIDs {
			// Update node content with community metadata using MergeNode
			node, err := sce.graphAdapter.GetNode(ctx, nodeID, "")
			if err != nil {
				log.Printf("[SPECTRAL] Aviso: falha ao obter no %s: %v", nodeID, err)
				continue
			}

			nodeType := "Unknown"
			if node.NodeType != "" {
				nodeType = node.NodeType
			}

			_, err = sce.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
				NodeType: nodeType,
				MatchKeys: map[string]interface{}{
					"id": nodeID,
				},
				OnMatchSet: map[string]interface{}{
					"community_id":         comm.ID,
					"community_label":      comm.Label,
					"community_coherence":  comm.Coherence,
					"spectral_updated_at":  nietzscheInfra.NowUnix(),
				},
			})
			if err != nil {
				log.Printf("[SPECTRAL] Aviso: falha ao atualizar no %s: %v", nodeID, err)
			}
		}
	}

	// Salvar metadados da analise no no Person
	_, err := sce.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": idosoID,
		},
		OnMatchSet: map[string]interface{}{
			"spectral_communities":  analysis.OptimalK,
			"spectral_fractal_dim":  analysis.FractalDimension,
			"spectral_fiedler":      analysis.FiedlerValue,
			"spectral_analyzed_at":  nietzscheInfra.NowUnix(),
		},
	})

	return err
}

// --- Utilitarios ---

func rowSlice(m *mat.Dense, row, cols int) []float64 {
	s := make([]float64, cols)
	for j := 0; j < cols; j++ {
		s[j] = m.At(row, j)
	}
	return s
}

func euclideanDist(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

func linearRegressionSlope(x, y []float64) float64 {
	n := float64(len(x))
	if n < 2 {
		return 0
	}

	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denom
}

func nodeIDs(nodes []GraphNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

func nodeLabels(nodes []GraphNode) []string {
	labels := make([]string, len(nodes))
	for i, n := range nodes {
		labels[i] = n.Label
	}
	return labels
}
