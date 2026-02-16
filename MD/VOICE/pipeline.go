package voice

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ─── Pipeline Principal ────────────────────────────────────────────────────
//
// Fluxo completo para cada chunk de áudio recebido pelo WebSocket:
//
//   AudioChunk (PCM float32, 16kHz)
//       │
//       ▼
//   VAD (Remove silêncio/ruído) ──► Rejeita se <1.5s de fala real
//       │
//       ▼
//   Embedder (TitaNet ONNX) ──────► D-Vector [512] L2-normalizado
//       │
//       ▼
//   profileCache.Get() ───────────► []VoiceProfile (Neo4j, TTL 5min)
//       │
//       ▼
//   FilterCandidates (cosseno) ───► Top-K candidatos (filtra irrelevantes)
//       │
//       ▼
//   RunOMP ───────────────────────► OMPResult {winner, coeff, residual}
//       │
//       ▼
//   CalibrateConfidence ──────────► confidence ∈ [0.0, 1.0]
//       │
//       ├── confidence ≥ threshold ──► IdentificationResult (falante conhecido)
//       │                                   │
//       │                                   ▼
//       │                             HebbianUpdate (LTP) async
//       │                                   │
//       │                                   ▼
//       │                             BuildPriming → Gemini
//       │
//       └── confidence < threshold ──► VozDesconhecida
//                                           │
//                                           ▼
//                                     HebbianUpdate (LTD) async
//                                           │
//                                           ▼
//                                     EnrollProtocol (opcional)

// ─── Tipos de resultado ────────────────────────────────────────────────────

// IdentificationResult é o output final do pipeline para cada chunk.
type IdentificationResult struct {
	// Identificação
	SpeakerID  string
	Name       string
	Confidence float64

	// Métricas brutas (para debug e Hebbian)
	CosineSim   float64
	OMPCoeff    float64
	ResidualErr float64
	Iterations  int

	// Qualidade do áudio de entrada
	AudioQuality float64 // RMS dB
	SpeechRatio  float64

	// Estado
	Unknown bool          // true = abaixo do limiar de confiança
	Latency time.Duration // Tempo total do pipeline

	// Priming gerado para o Gemini (se identificado)
	GeminiPriming string
}

// EnrollSample é uma amostra de voz bruta para o processo de cadastro.
type EnrollSample struct {
	SpeakerID string
	Name      string
	Samples   []float32 // PCM float32 a 16kHz
}

// EnrollResult é o resultado do processo de cadastro de um falante.
type EnrollResult struct {
	SpeakerID     string
	Name          string
	SamplesUsed   int
	IntraVariance float64
	Quality       string // "excellent" / "good" / "poor"
}

// ─── Pipeline ──────────────────────────────────────────────────────────────

// Pipeline orquestra todos os componentes de biometria de voz.
type Pipeline struct {
	cfg      SRCConfig
	embedder *Embedder
	store    *Neo4jStore
	cache    *profileCache
	log      *zap.Logger
}

// NewPipeline cria e inicializa o pipeline completo.
func NewPipeline(
	cfg SRCConfig,
	embedCfg EmbedderConfig,
	store *Neo4jStore,
	log *zap.Logger,
) (*Pipeline, error) {
	embedder, err := NewEmbedder(embedCfg)
	if err != nil {
		return nil, fmt.Errorf("NewEmbedder: %w", err)
	}

	cache := newProfileCache(store, 5*time.Minute, log)

	return &Pipeline{
		cfg:      cfg,
		embedder: embedder,
		store:    store,
		cache:    cache,
		log:      log,
	}, nil
}

// Identify processa um chunk de áudio e retorna a identidade do falante.
func (p *Pipeline) Identify(ctx context.Context, rawPCM []float32) (IdentificationResult, error) {
	start := time.Now()

	// ── 1. VAD ────────────────────────────────────────────────────────────
	vadResult := ApplyVAD(rawPCM)

	if !HasEnoughSpeech(vadResult) {
		return IdentificationResult{
			Unknown:      true,
			SpeechRatio:  vadResult.SpeechRatio,
			AudioQuality: vadResult.RMSdB,
			Latency:      time.Since(start),
		}, nil
	}

	// ── 2. Embedding (TitaNet ONNX) ───────────────────────────────────────
	embedding, err := p.embedder.Extract(vadResult.Speech)
	if err != nil {
		return IdentificationResult{Unknown: true}, fmt.Errorf("Extract: %w", err)
	}

	// ── 3. Carrega perfis (cache) ─────────────────────────────────────────
	profiles, err := p.cache.Get(ctx)
	if err != nil {
		p.log.Warn("cache fallback", zap.Error(err))
	}
	if len(profiles) == 0 {
		return IdentificationResult{
			Unknown:     true,
			Latency:     time.Since(start),
			SpeechRatio: vadResult.SpeechRatio,
		}, nil
	}

	// ── 4. Pré-filtragem por cosseno ──────────────────────────────────────
	candidates := FilterCandidates(embedding, profiles, p.cfg)
	if len(candidates) == 0 {
		p.log.Info("SRC: nenhum candidato acima do limiar mínimo de cosseno",
			zap.Float64("min_cosine", p.cfg.MinCosineSim),
		)
		result := IdentificationResult{
			Unknown:      true,
			AudioQuality: vadResult.RMSdB,
			SpeechRatio:  vadResult.SpeechRatio,
			Latency:      time.Since(start),
		}
		p.triggerHebbianAsync(candidates, result, vadResult)
		return result, nil
	}

	// ── 5. OMP ────────────────────────────────────────────────────────────
	dict := make([][]float64, len(candidates))
	for i, c := range candidates {
		dict[i] = c.NormCenter
	}

	ompResult := RunOMP(embedding, dict, p.cfg)

	if ompResult.WinnerIndex < 0 {
		return IdentificationResult{Unknown: true, Latency: time.Since(start)}, nil
	}

	winner := candidates[ompResult.WinnerIndex]

	// ── 6. Calibra confiança ──────────────────────────────────────────────
	confidence := CalibrateConfidence(CalibrationInput{
		CosineSim:     winner.CosineSim,
		OMPCoeff:      ompResult.WinnerCoeff,
		ResidualNorm:  ompResult.ResidualNorm,
		IntraVariance: winner.Profile.IntraVariance,
		Cfg:           p.cfg,
	})

	unknown := confidence < p.cfg.ConfidenceThreshold

	p.log.Info("SRC identification",
		zap.String("speaker", winner.Profile.Name),
		zap.Float64("cosine", winner.CosineSim),
		zap.Float64("omp_coeff", ompResult.WinnerCoeff),
		zap.Float64("residual", ompResult.ResidualNorm),
		zap.Float64("confidence", confidence),
		zap.Bool("unknown", unknown),
		zap.Int("omp_iters", ompResult.Iterations),
		zap.Duration("latency", time.Since(start)),
	)

	result := IdentificationResult{
		SpeakerID:    winner.Profile.SpeakerID,
		Name:         winner.Profile.Name,
		Confidence:   confidence,
		CosineSim:    winner.CosineSim,
		OMPCoeff:     ompResult.WinnerCoeff,
		ResidualErr:  ompResult.ResidualNorm,
		Iterations:   ompResult.Iterations,
		AudioQuality: vadResult.RMSdB,
		SpeechRatio:  vadResult.SpeechRatio,
		Unknown:      unknown,
		Latency:      time.Since(start),
	}

	// ── 7. Gera priming para o Gemini ─────────────────────────────────────
	if !unknown {
		result.GeminiPriming = BuildGeminiPriming(result)
	}

	// ── 8. Hebbian Update (assíncrono — não bloqueia a resposta) ──────────
	p.triggerHebbianAsync(candidates, result, vadResult)

	return result, nil
}

// triggerHebbianAsync dispara o update de Hebb sem bloquear o pipeline.
// Em caso de falha (Neo4j down), loga o erro mas não propaga.
func (p *Pipeline) triggerHebbianAsync(
	candidates []CandidateProfile,
	result IdentificationResult,
	vad VADResult,
) {
	if result.Unknown || len(candidates) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		event := VoiceEvent{
			SpeakerID:    result.SpeakerID,
			CosineSim:    result.CosineSim,
			ResidualErr:  result.ResidualErr,
			Confidence:   result.Confidence,
			Confirmed:    !result.Unknown,
			AudioQuality: vad.RMSdB,
			Timestamp:    time.Now(),
		}

		if _, err := p.store.HebbianUpdate(ctx, event); err != nil {
			p.log.Warn("hebbian update failed (async)", zap.Error(err))
		}
	}()
}

// ─── Enroll ───────────────────────────────────────────────────────────────

// Enroll cadastra um novo falante a partir de múltiplas amostras de áudio.
// Requer no mínimo 5 amostras para um perfil robusto.
// Recomendado: 7–10 amostras em condições diferentes (normal, sussurro, ambiente barulhento).
func (p *Pipeline) Enroll(ctx context.Context, samples []EnrollSample) (EnrollResult, error) {
	if len(samples) < 3 {
		return EnrollResult{}, fmt.Errorf("mínimo de 3 amostras necessário, recebido: %d", len(samples))
	}
	if len(samples[0].SpeakerID) == 0 {
		return EnrollResult{}, fmt.Errorf("speaker_id obrigatório")
	}

	speakerID := samples[0].SpeakerID
	name := samples[0].Name

	// Extrai embeddings de cada amostra
	var embeddings [][]float64
	for i, s := range samples {
		vad := ApplyVAD(s.Samples)
		if !HasEnoughSpeech(vad) {
			p.log.Warn("enroll: amostra rejeitada por falta de fala",
				zap.Int("sample_index", i),
				zap.Float64("speech_ratio", vad.SpeechRatio),
			)
			continue
		}
		emb, err := p.embedder.Extract(vad.Speech)
		if err != nil {
			p.log.Warn("enroll: embedding falhou", zap.Int("index", i), zap.Error(err))
			continue
		}
		embeddings = append(embeddings, emb)
	}

	if len(embeddings) < 3 {
		return EnrollResult{}, fmt.Errorf("apenas %d amostras válidas após VAD (mínimo 3)", len(embeddings))
	}

	// Calcula centróide (mediana componente a componente — mais robusta que média)
	centroid := medianCentroid(embeddings)
	centroid = l2Normalize(centroid)

	// Calcula variância intra-speaker (diagnóstico de qualidade do enroll)
	variance := intraVariance(centroid, embeddings)

	// Classifica qualidade
	quality := enrollQuality(variance)

	profile := VoiceProfile{
		SpeakerID:     speakerID,
		Name:          name,
		Centroid:      centroid,
		IntraVariance: variance,
		SampleCount:   len(embeddings),
		EnrolledAt:    time.Now(),
		Active:        true,
	}

	if err := p.store.UpsertProfile(ctx, profile); err != nil {
		return EnrollResult{}, fmt.Errorf("UpsertProfile: %w", err)
	}

	// Invalida cache para o próximo Identify() buscar o novo perfil
	p.cache.Invalidate()

	p.log.Info("enroll concluído",
		zap.String("speaker_id", speakerID),
		zap.String("name", name),
		zap.Int("samples", len(embeddings)),
		zap.Float64("variance", variance),
		zap.String("quality", quality),
	)

	return EnrollResult{
		SpeakerID:     speakerID,
		Name:          name,
		SamplesUsed:   len(embeddings),
		IntraVariance: variance,
		Quality:       quality,
	}, nil
}

// Close libera os recursos do pipeline.
func (p *Pipeline) Close() {
	p.embedder.Close()
}

// ─── Matemática de Enroll ─────────────────────────────────────────────────

// medianCentroid calcula a mediana componente a componente de N embeddings.
// Mais robusta que a média: um embedding ruidoso não puxa o centróide.
func medianCentroid(embeddings [][]float64) []float64 {
	if len(embeddings) == 0 {
		return nil
	}
	dim := len(embeddings[0])
	centroid := make([]float64, dim)
	column := make([]float64, len(embeddings))

	for d := 0; d < dim; d++ {
		for i, emb := range embeddings {
			column[i] = emb[d]
		}
		centroid[d] = medianFloat(column)
	}
	return centroid
}

func medianFloat(data []float64) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	// Insertion sort (N pequeno: ≤ 20 amostras)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// intraVariance calcula a variância da similaridade entre o centróide e cada amostra.
// Baixa variância = perfil estável = reconhecimento mais confiável.
func intraVariance(centroid []float64, embeddings [][]float64) float64 {
	if len(embeddings) == 0 {
		return 0
	}
	var sims []float64
	for _, emb := range embeddings {
		sims = append(sims, dot(centroid, l2Normalize(emb)))
	}
	var mean float64
	for _, s := range sims {
		mean += s
	}
	mean /= float64(len(sims))
	var variance float64
	for _, s := range sims {
		d := s - mean
		variance += d * d
	}
	return variance / float64(len(sims))
}

// enrollQuality classifica a qualidade do perfil baseado na variância intra-speaker.
func enrollQuality(variance float64) string {
	switch {
	case variance < 0.001:
		return "excellent"
	case variance < 0.005:
		return "good"
	default:
		return "poor" // Recomenda re-enroll em melhores condições
	}
}
