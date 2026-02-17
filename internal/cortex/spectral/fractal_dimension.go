// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package spectral

import (
	"math"
	"sort"
)

// FractalHierarchy resultado da analise fractal hierarquica do grafo
// Responde: "As comunidades de memoria tem sub-comunidades auto-similares?"
type FractalHierarchy struct {
	// Dimensao fractal do espectro (d): quanto maior, mais complexa a estrutura
	// d ~ 1.0: grafo quase-linear (cadeia de memorias)
	// d ~ 1.5: estrutura moderadamente hierarquica
	// d ~ 2.0+: estrutura rica com comunidades dentro de comunidades
	SpectralDimension float64 `json:"spectral_dimension"`

	// Expoente de Weyl: N(lambda) ~ lambda^{d_w/2}
	// Mede como a densidade de estados escala com energia
	WeylExponent float64 `json:"weyl_exponent"`

	// Lacunaridade: variabilidade na estrutura fractal
	// Alta lacunaridade = comunidades de tamanhos muito diferentes
	// Baixa lacunaridade = comunidades uniformes
	Lacunarity float64 `json:"lacunarity"`

	// Hierarquia detectada em niveis
	// Nivel 0: grafo inteiro
	// Nivel 1: comunidades principais
	// Nivel 2: sub-comunidades dentro das principais
	HierarchyDepth int               `json:"hierarchy_depth"`
	LevelSizes     []int             `json:"level_sizes"`     // Quantas comunidades em cada nivel
	LevelGaps      []float64         `json:"level_gaps"`      // Spectral gap em cada nivel

	// Classificacao
	Classification string `json:"classification"`
	// "random"      = sem estrutura (grafo aleatorio)
	// "modular"     = comunidades claras mas nao hierarquicas
	// "hierarchical"= comunidades dentro de comunidades (fractal!)
	// "scale-free"  = distribuicao de grau lei de potencia

	IsFractal bool `json:"is_fractal"` // d > 1.2 E hierarchy_depth >= 2
}

// AnalyzeFractalHierarchy analise completa da estrutura fractal do grafo
// usando o espectro de autovalores do Laplaciano
func AnalyzeFractalHierarchy(eigenvalues []float64) *FractalHierarchy {
	result := &FractalHierarchy{}

	n := len(eigenvalues)
	if n < 5 {
		result.Classification = "trivial"
		return result
	}

	// 1. Dimensao espectral via IDOS (Integrated Density of States)
	result.SpectralDimension = computeSpectralDim(eigenvalues)
	result.WeylExponent = result.SpectralDimension / 2.0

	// 2. Lacunaridade do espectro
	result.Lacunarity = computeSpectralLacunarity(eigenvalues)

	// 3. Detectar niveis hierarquicos via spectral gaps
	gaps := findSignificantGaps(eigenvalues)
	result.HierarchyDepth = len(gaps)
	result.LevelGaps = gaps

	// Tamanhos dos niveis: quantos autovalores entre gaps consecutivos
	result.LevelSizes = computeLevelSizes(eigenvalues, gaps)

	// 4. Classificacao
	result.Classification = classifyStructure(result)
	result.IsFractal = result.SpectralDimension > 1.2 && result.HierarchyDepth >= 2

	return result
}

// computeSpectralDim calcula dimensao espectral via regressao log-log do IDOS
func computeSpectralDim(eigenvalues []float64) float64 {
	// Filtrar positivos
	var pos []float64
	for _, ev := range eigenvalues {
		if ev > 1e-8 {
			pos = append(pos, ev)
		}
	}
	if len(pos) < 5 {
		return 0
	}

	sort.Float64s(pos)

	// IDOS: N(lambda) = #{eigenvalues <= lambda}
	var logL, logN []float64
	for i, lambda := range pos {
		logL = append(logL, math.Log(lambda))
		logN = append(logN, math.Log(float64(i+1)))
	}

	slope := linearRegressionSlope(logL, logN)
	dim := 2.0 * slope

	if dim < 0 {
		dim = 0
	}
	return dim
}

// computeSpectralLacunarity mede a "textura" do espectro
// Lacunaridade = Var(gaps) / Mean(gaps)^2
// Alta lacunaridade = gaps irregulares = comunidades de tamanhos diferentes
func computeSpectralLacunarity(eigenvalues []float64) float64 {
	if len(eigenvalues) < 3 {
		return 0
	}

	// Calcular spacings entre autovalores consecutivos
	spacings := make([]float64, len(eigenvalues)-1)
	for i := 1; i < len(eigenvalues); i++ {
		spacings[i-1] = eigenvalues[i] - eigenvalues[i-1]
	}

	// Filtrar spacings positivos
	var posSpacings []float64
	for _, s := range spacings {
		if s > 1e-10 {
			posSpacings = append(posSpacings, s)
		}
	}

	if len(posSpacings) < 2 {
		return 0
	}

	// Media e variancia
	mean := 0.0
	for _, s := range posSpacings {
		mean += s
	}
	mean /= float64(len(posSpacings))

	variance := 0.0
	for _, s := range posSpacings {
		d := s - mean
		variance += d * d
	}
	variance /= float64(len(posSpacings))

	if mean < 1e-10 {
		return 0
	}

	// Lacunaridade = 1 + Var/Mean^2
	return 1.0 + variance/(mean*mean)
}

// findSignificantGaps encontra gaps significativos no espectro
// Um gap significativo indica fronteira entre niveis hierarquicos
func findSignificantGaps(eigenvalues []float64) []float64 {
	if len(eigenvalues) < 3 {
		return nil
	}

	// Calcular todos os gaps
	type gapInfo struct {
		value float64
		index int
	}
	var allGaps []gapInfo
	for i := 1; i < len(eigenvalues); i++ {
		gap := eigenvalues[i] - eigenvalues[i-1]
		if gap > 1e-8 {
			allGaps = append(allGaps, gapInfo{value: gap, index: i})
		}
	}

	if len(allGaps) < 2 {
		return nil
	}

	// Calcular gap medio
	meanGap := 0.0
	for _, g := range allGaps {
		meanGap += g.value
	}
	meanGap /= float64(len(allGaps))

	// Gaps significativos: > 2x a media (outliers)
	var significant []float64
	for _, g := range allGaps {
		if g.value > 2.0*meanGap {
			significant = append(significant, g.value)
		}
	}

	// Ordenar por tamanho decrescente
	sort.Sort(sort.Reverse(sort.Float64Slice(significant)))

	// Limitar a 5 niveis
	if len(significant) > 5 {
		significant = significant[:5]
	}

	return significant
}

// computeLevelSizes calcula quantos nos em cada nivel hierarquico
func computeLevelSizes(eigenvalues []float64, gaps []float64) []int {
	if len(gaps) == 0 {
		return []int{len(eigenvalues)}
	}

	// Para cada gap significativo, contar quantos autovalores estao abaixo
	gapThresholds := make([]float64, len(gaps))
	copy(gapThresholds, gaps)

	// Encontrar indices dos gaps no espectro
	var levelSizes []int
	prevIdx := 0

	for i := 1; i < len(eigenvalues); i++ {
		gap := eigenvalues[i] - eigenvalues[i-1]
		for _, sigGap := range gaps {
			if math.Abs(gap-sigGap) < 1e-8 {
				levelSizes = append(levelSizes, i-prevIdx)
				prevIdx = i
				break
			}
		}
	}

	// Ultimo nivel
	if prevIdx < len(eigenvalues) {
		levelSizes = append(levelSizes, len(eigenvalues)-prevIdx)
	}

	return levelSizes
}

// classifyStructure classifica a estrutura do grafo baseada na analise fractal
func classifyStructure(fh *FractalHierarchy) string {
	dim := fh.SpectralDimension
	lac := fh.Lacunarity
	depth := fh.HierarchyDepth

	// Grafo aleatorio (Erdos-Renyi): d ~ 1.0, lacunaridade baixa, sem hierarquia
	if dim < 1.1 && depth <= 1 {
		return "random"
	}

	// Modular (comunidades claras mas planas): d moderado, 1 nivel de gaps
	if depth == 1 && dim >= 1.1 && dim < 1.5 {
		return "modular"
	}

	// Hierarquico/Fractal: multiplos niveis de comunidades aninhadas
	if depth >= 2 && dim >= 1.2 {
		return "hierarchical"
	}

	// Scale-free: lacunaridade muito alta (comunidades com tamanhos lei de potencia)
	if lac > 5.0 && dim > 1.0 {
		return "scale-free"
	}

	// Default
	if dim >= 1.5 {
		return "hierarchical"
	}
	if dim >= 1.1 {
		return "modular"
	}
	return "random"
}

// ComputeHurstFromSpectrum calcula Hurst exponent do espectro (nao do vetor!)
// H > 0.5: persistencia no espectro = memoria de longo alcance no grafo
// H = 0.5: random walk spectral = grafo sem memoria estrutural
// H < 0.5: anti-persistencia = grafo com repulsao estrutural
func ComputeHurstFromSpectrum(eigenvalues []float64) float64 {
	// Filtrar positivos
	var pos []float64
	for _, ev := range eigenvalues {
		if ev > 1e-8 {
			pos = append(pos, ev)
		}
	}

	if len(pos) < 10 {
		return 0.5 // Indeterminado
	}

	// Tratar espacamentos entre autovalores como serie temporal
	spacings := make([]float64, len(pos)-1)
	for i := 1; i < len(pos); i++ {
		spacings[i-1] = pos[i] - pos[i-1]
	}

	return computeHurstRS(spacings)
}

// computeHurstRS Hurst via Rescaled Range (R/S)
// Mesma matematica do hurst_analyzer, mas aplicada ao espectro do grafo (nao a vetores)
func computeHurstRS(data []float64) float64 {
	n := len(data)
	if n < 8 {
		return 0.5
	}

	// Media
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(n)

	// Multiplas escalas
	var logTau []float64
	var logRS []float64

	for tau := 8; tau <= n/2; tau *= 2 {
		// Para cada escala, calcular R/S medio
		numBlocks := n / tau
		if numBlocks < 1 {
			continue
		}

		totalRS := 0.0
		validBlocks := 0

		for b := 0; b < numBlocks; b++ {
			start := b * tau
			end := start + tau
			block := data[start:end]

			// Media do bloco
			blockMean := 0.0
			for _, v := range block {
				blockMean += v
			}
			blockMean /= float64(tau)

			// Soma cumulativa dos desvios
			cumSum := make([]float64, tau)
			sum := 0.0
			for i, v := range block {
				sum += v - blockMean
				cumSum[i] = sum
			}

			// Range
			maxCS, minCS := cumSum[0], cumSum[0]
			for _, v := range cumSum {
				if v > maxCS {
					maxCS = v
				}
				if v < minCS {
					minCS = v
				}
			}
			R := maxCS - minCS

			// Standard deviation
			variance := 0.0
			for _, v := range block {
				d := v - blockMean
				variance += d * d
			}
			S := math.Sqrt(variance / float64(tau))

			if S > 1e-10 {
				totalRS += R / S
				validBlocks++
			}
		}

		if validBlocks > 0 {
			avgRS := totalRS / float64(validBlocks)
			logTau = append(logTau, math.Log(float64(tau)))
			logRS = append(logRS, math.Log(avgRS))
		}
	}

	if len(logTau) < 2 {
		return 0.5
	}

	// Hurst = slope da regressao log(R/S) vs log(tau)
	return linearRegressionSlope(logTau, logRS)
}
