package voice

import (
	"math"
	"sort"
)

// ─── Tipos ────────────────────────────────────────────────────────────────

// SRCConfig controla os limiares e hiperparâmetros do motor SRC/OMP.
type SRCConfig struct {
	// Similaridade de cosseno mínima para um perfil entrar no dicionário OMP.
	// Filtra candidatos irrelevantes antes do OMP — economiza CPU.
	MinCosineSim float64 // Recomendado: 0.72

	// Confiança mínima para declarar identidade conhecida.
	// Abaixo disto: VozDesconhecida.
	ConfidenceThreshold float64 // Recomendado: 0.82

	// Máximo de candidatos passados para o OMP (top-K por cosseno).
	DictionaryTopK int // Recomendado: 6

	// Máximo de iterações OMP.
	MaxOMPIterations int // Recomendado: 12

	// Erro residual mínimo para parar OMP cedo (convergência).
	ResidualTolerance float64 // Recomendado: 0.04

	// Peso da variância intra-speaker na calibração de confiança.
	// Perfis com maior variância (voz instável) recebem penalidade.
	VariancePenaltyScale float64 // Recomendado: 80.0
}

// DefaultSRCConfig retorna a configuração recomendada para o projeto EVA.
func DefaultSRCConfig() SRCConfig {
	return SRCConfig{
		MinCosineSim:         0.72,
		ConfidenceThreshold:  0.82,
		DictionaryTopK:       6,
		MaxOMPIterations:     12,
		ResidualTolerance:    0.04,
		VariancePenaltyScale: 80.0,
	}
}

// OMPResult é o resultado bruto de uma execução do algoritmo OMP.
type OMPResult struct {
	// Índice do átomo (perfil) dominante no dicionário
	WinnerIndex int
	// Coeficiente de reconstrução do átomo dominante
	WinnerCoeff float64
	// Norma L2 do vetor residual final
	ResidualNorm float64
	// Coeficientes de todos os átomos (para debug/análise)
	AllCoeffs []float64
	// Número de iterações executadas
	Iterations int
}

// ─── Álgebra Linear (sem dependências externas) ───────────────────────────

// l2Norm retorna a norma L2 de um vetor.
func l2Norm(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

// l2Normalize retorna v / ||v||₂. Se ||v|| ≈ 0, retorna zeros.
func l2Normalize(v []float64) []float64 {
	norm := l2Norm(v)
	if norm < 1e-9 {
		out := make([]float64, len(v))
		return out
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// dot retorna o produto interno de dois vetores de mesmo tamanho.
// Para vetores L2-normalizados, equivale à similaridade de cosseno.
func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// vectorSub subtrai b de a em place (a = a - scale*b).
func vectorSubScaled(a []float64, b []float64, scale float64) {
	for i := range a {
		a[i] -= scale * b[i]
	}
}

// ─── OMP: Orthogonal Matching Pursuit ─────────────────────────────────────
//
// Dado:
//   q ∈ ℝ^512  (D-Vector da voz atual, L2-normalizado)
//   D = [d₁, d₂, ..., dₖ]  (Dicionário: centroides dos perfis cadastrados)
//
// Objetivo: encontrar uma recombinação esparsa x tal que D·x ≈ q
//
// O índice com o maior coeficiente x_i aponta para o falante mais provável.
// O erro residual ||q - D·x||₂ mede a certeza da identificação.
//
// Complexidade: O(K * D * I) onde K=top-k, D=512, I=iterações → ~40μs
//
func RunOMP(query []float64, dictionary [][]float64, cfg SRCConfig) OMPResult {
	K := len(dictionary)
	if K == 0 {
		return OMPResult{WinnerIndex: -1}
	}

	// Resíduo inicial = query
	residual := make([]float64, len(query))
	copy(residual, query)

	coefficients := make([]float64, K)
	// Conjunto de suporte (índices já selecionados)
	selected := make([]bool, K)

	var iterations int

	for iter := 0; iter < cfg.MaxOMPIterations; iter++ {
		// ── Passo 1: Seleciona átomo mais correlacionado com o resíduo ──
		bestCorr := -1.0
		bestIdx := -1
		for j := 0; j < K; j++ {
			if selected[j] {
				continue
			}
			corr := math.Abs(dot(residual, dictionary[j]))
			if corr > bestCorr {
				bestCorr = corr
				bestIdx = j
			}
		}
		if bestIdx < 0 {
			break
		}
		selected[bestIdx] = true

		// ── Passo 2: Projeção — coeficiente = <resíduo, átomo> ──────────
		// Como os átomos são L2-normalizados, a projeção ortogonal é direta.
		coeff := dot(residual, dictionary[bestIdx])
		coefficients[bestIdx] += coeff

		// ── Passo 3: Atualiza resíduo ─────────────────────────────────────
		vectorSubScaled(residual, dictionary[bestIdx], coeff)

		iterations++

		// ── Passo 4: Critério de parada por convergência ─────────────────
		if l2Norm(residual) < cfg.ResidualTolerance {
			break
		}
	}

	// Encontra o átomo com maior coeficiente absoluto (speaker dominante)
	winnerIdx := 0
	winnerCoeff := math.Abs(coefficients[0])
	for j := 1; j < K; j++ {
		if c := math.Abs(coefficients[j]); c > winnerCoeff {
			winnerCoeff = c
			winnerIdx = j
		}
	}

	return OMPResult{
		WinnerIndex:  winnerIdx,
		WinnerCoeff:  winnerCoeff,
		ResidualNorm: l2Norm(residual),
		AllCoeffs:    coefficients,
		Iterations:   iterations,
	}
}

// ─── Calibração de Confiança ──────────────────────────────────────────────

// CalibrationInput agrupa os valores usados para calcular a confiança final.
type CalibrationInput struct {
	CosineSim     float64 // Similaridade de cosseno direta (0.0–1.0)
	OMPCoeff      float64 // Coeficiente OMP do vencedor (0.0–1.0)
	ResidualNorm  float64 // Erro residual OMP (quanto menor, mais certeza)
	IntraVariance float64 // Variância intra-speaker do perfil cadastrado
	Cfg           SRCConfig
}

// CalibrateConfidence combina as métricas em uma única pontuação de confiança.
//
// Fórmula:
//   confidence = (α·cosine + β·coeff) · residualFactor / variancePenalty
//
//   α = 0.6  (cosseno tem peso maior — mais direto)
//   β = 0.4  (coeff OMP complementa)
//   residualFactor = exp(-2·residual)  (penaliza erros altos suavemente)
//   variancePenalty = 1 + scale·variance  (perfis instáveis são penalizados)
//
func CalibrateConfidence(in CalibrationInput) float64 {
	alpha := 0.6
	beta := 0.4

	base := alpha*in.CosineSim + beta*in.OMPCoeff
	residualFactor := math.Exp(-2.0 * in.ResidualNorm)
	variancePenalty := 1.0 + in.Cfg.VariancePenaltyScale*in.IntraVariance

	confidence := (base * residualFactor) / variancePenalty
	return math.Min(math.Max(confidence, 0.0), 1.0)
}

// ─── Pré-filtragem por Cosseno ────────────────────────────────────────────

// CandidateProfile é um perfil de voz com sua similaridade de cosseno pré-calculada.
type CandidateProfile struct {
	Profile    VoiceProfile
	CosineSim  float64
	NormCenter []float64 // Centróide L2-normalizado (cacheado)
}

// FilterCandidates pré-filtra os perfis por cosseno e retorna os top-K.
// Isso evita rodar OMP contra todo o dicionário quando há muitos cadastrados.
func FilterCandidates(query []float64, profiles []VoiceProfile, cfg SRCConfig) []CandidateProfile {
	var candidates []CandidateProfile

	for _, p := range profiles {
		norm := l2Normalize(p.Centroid)
		sim := dot(query, norm)
		if sim >= cfg.MinCosineSim {
			candidates = append(candidates, CandidateProfile{
				Profile:    p,
				CosineSim:  sim,
				NormCenter: norm,
			})
		}
	}

	// Ordena por similaridade decrescente
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CosineSim > candidates[j].CosineSim
	})

	// Top-K
	if len(candidates) > cfg.DictionaryTopK {
		candidates = candidates[:cfg.DictionaryTopK]
	}

	return candidates
}
