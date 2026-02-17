// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"fmt"
	"math"
	"os"

	ort "github.com/yalue/onnxruntime_go"
)

// Embedder usa o modelo TitaNet exportado para ONNX para gerar
// D-Vectors (embeddings de speaker) diretamente em Go.
//
// Como exportar o TitaNet para ONNX (rodar UMA VEZ em qualquer máquina com Python):
//
//   pip install nemo_toolkit[asr]
//   python3 -c "
//   import nemo.collections.asr as nemo_asr
//   model = nemo_asr.models.EncDecSpeakerLabelModel.from_pretrained('nvidia/speakerverification_en_titanet_large')
//   model.export('titanet_large.onnx')
//   "
//
// Depois copie o .onnx para o servidor e configure EmbedderConfig.ModelPath.

const (
	embeddingDim   = 512  // TitaNet-Large produz vetores de 512 dimensões
	melBins        = 80   // Mel filterbanks
	hopLength      = 160  // Hop de 10ms a 16kHz
	winLength      = 400  // Janela de 25ms a 16kHz
	maxInputFrames = 400  // ~4s de áudio em mel-frames
)

// EmbedderConfig configura o extrator de D-Vectors.
type EmbedderConfig struct {
	// Caminho para o arquivo titanet_large.onnx
	ModelPath string

	// Número de threads ONNX (0 = automático)
	Threads int
}

// Embedder mantém a sessão ONNX carregada em memória.
type Embedder struct {
	session *ort.DynamicAdvancedSession
	cfg     EmbedderConfig
}

// NewEmbedder inicializa o ONNX Runtime e carrega o modelo TitaNet.
// Deve ser chamado uma única vez na inicialização da aplicação.
func NewEmbedder(cfg EmbedderConfig) (*Embedder, error) {
	if _, err := os.Stat(cfg.ModelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("modelo ONNX não encontrado em %q — veja instruções de exportação no topo deste arquivo", cfg.ModelPath)
	}

	// Inicializa ONNX Runtime
	ort.SetSharedLibraryPath(onnxRuntimeLibPath())
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("ort.InitializeEnvironment: %w", err)
	}

	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("NewSessionOptions: %w", err)
	}
	defer opts.Destroy()

	threads := cfg.Threads
	if threads <= 0 {
		threads = 2
	}
	opts.SetIntraOpNumThreads(threads)
	opts.SetInterOpNumThreads(1)

	// Nomes de input/output do TitaNet exportado
	inputNames  := []string{"audio_signal", "length"}
	outputNames := []string{"logits", "embs"}

	session, err := ort.NewDynamicAdvancedSessionWithOptions(
		cfg.ModelPath, inputNames, outputNames, opts,
	)
	if err != nil {
		return nil, fmt.Errorf("NewDynamicSession: %w", err)
	}

	return &Embedder{session: session, cfg: cfg}, nil
}

// Extract recebe amostras PCM float32 a 16kHz e retorna o D-Vector
// L2-normalizado de 512 dimensões.
func (e *Embedder) Extract(samples []float32) ([]float64, error) {
	if len(samples) < 16000 {
		return nil, fmt.Errorf("áudio muito curto: %d amostras (mínimo 16000 = 1s)", len(samples))
	}

	// 1. Pré-enfatização (realça altas frequências, melhora speaker ID)
	preemphasized := preemphasis(samples, 0.97)

	// 2. Extrai Mel Filterbank (feature frontend do TitaNet)
	melFeatures := extractMelFilterbank(preemphasized)

	// 3. Normaliza CMN (Cepstral Mean Normalization)
	cmn(melFeatures)

	// 4. Prepara tensores ONNX
	flatLen := len(melFeatures) * melBins
	audioFlat := make([]float32, flatLen)
	for i, frame := range melFeatures {
		for j, v := range frame {
			audioFlat[i*melBins+j] = float32(v)
		}
	}

	numFrames := int64(len(melFeatures))
	shape := ort.NewShape(1, int64(len(melFeatures)), melBins)

	audioTensor, err := ort.NewTensor(shape, audioFlat)
	if err != nil {
		return nil, fmt.Errorf("NewTensor audio: %w", err)
	}
	defer audioTensor.Destroy()

	lenShape := ort.NewShape(1)
	lenTensor, err := ort.NewTensor(lenShape, []int64{numFrames})
	if err != nil {
		return nil, fmt.Errorf("NewTensor length: %w", err)
	}
	defer lenTensor.Destroy()

	// 5. Inferência
	outLogits  := ort.NewEmptyTensor[float32]()
	outEmbedds := ort.NewEmptyTensor[float32]()
	defer outLogits.Destroy()
	defer outEmbedds.Destroy()

	err = e.session.Run(
		[]ort.ArbitraryTensor{audioTensor, lenTensor},
		[]ort.ArbitraryTensor{outLogits, outEmbedds},
	)
	if err != nil {
		return nil, fmt.Errorf("session.Run: %w", err)
	}

	// 6. Extrai embedding e converte para float64
	embData := outEmbedds.GetData()
	if len(embData) < embeddingDim {
		return nil, fmt.Errorf("embedding inesperado: got %d, want %d dims", len(embData), embeddingDim)
	}

	embedding := make([]float64, embeddingDim)
	for i := 0; i < embeddingDim; i++ {
		embedding[i] = float64(embData[i])
	}

	return l2Normalize(embedding), nil
}

// Close libera a sessão ONNX.
func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
	}
}

// ─── Mel Filterbank (Go nativo) ───────────────────────────────────────────

// preemphasis aplica filtro de pré-enfatização H(z) = 1 - coeff*z⁻¹
func preemphasis(samples []float32, coeff float32) []float32 {
	out := make([]float32, len(samples))
	out[0] = samples[0]
	for i := 1; i < len(samples); i++ {
		out[i] = samples[i] - coeff*samples[i-1]
	}
	return out
}

// extractMelFilterbank extrai a representação mel-spectrogram.
// Retorna slice de frames × melBins.
func extractMelFilterbank(samples []float32) [][]float64 {
	numFrames := (len(samples)-winLength)/hopLength + 1
	if numFrames <= 0 {
		numFrames = 1
	}
	if numFrames > maxInputFrames {
		numFrames = maxInputFrames
	}

	// Banco de filtros mel (calculado estaticamente)
	filterbank := buildMelFilterbank(melBins, winLength/2+1, 16000, 0, 8000)

	result := make([][]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		start := i * hopLength
		end := start + winLength
		if end > len(samples) {
			end = len(samples)
		}

		frame := make([]float64, winLength)
		for j := 0; j < winLength && start+j < len(samples); j++ {
			// Hann window
			w := 0.5 * (1 - math.Cos(2*math.Pi*float64(j)/float64(winLength-1)))
			frame[j] = float64(samples[start+j]) * w
		}

		// FFT via Goertzel/DFT simplificado para o banco de filtros
		powerSpec := powerSpectrum(frame)

		// Aplica banco de filtros mel e log
		melFrame := make([]float64, melBins)
		for m := 0; m < melBins; m++ {
			var energy float64
			for k, fv := range filterbank[m] {
				if k < len(powerSpec) {
					energy += fv * powerSpec[k]
				}
			}
			melFrame[m] = math.Log(energy + 1e-9)
		}

		result[i] = melFrame
	}
	return result
}

// powerSpectrum calcula |FFT|² de um frame via DFT.
func powerSpectrum(frame []float64) []float64 {
	n := len(frame)
	nfft := winLength/2 + 1
	power := make([]float64, nfft)

	for k := 0; k < nfft; k++ {
		var re, im float64
		for t := 0; t < n; t++ {
			angle := 2 * math.Pi * float64(k) * float64(t) / float64(n)
			re += frame[t] * math.Cos(angle)
			im -= frame[t] * math.Sin(angle)
		}
		power[k] = re*re + im*im
	}
	return power
}

// buildMelFilterbank constrói o banco de filtros triangulares na escala mel.
func buildMelFilterbank(nMels, nFFT, sr, fMin, fMax int) [][]float64 {
	hzToMel := func(hz float64) float64 { return 2595 * math.Log10(1+hz/700) }
	melToHz := func(mel float64) float64 { return 700 * (math.Pow(10, mel/2595) - 1) }

	melMin := hzToMel(float64(fMin))
	melMax := hzToMel(float64(fMax))

	// nMels+2 pontos igualmente espaçados na escala mel
	melPoints := make([]float64, nMels+2)
	for i := range melPoints {
		melPoints[i] = melToHz(melMin + float64(i)*(melMax-melMin)/float64(nMels+1))
	}

	// Converte para índices FFT
	fftFreqs := make([]float64, nFFT)
	for i := range fftFreqs {
		fftFreqs[i] = float64(i) * float64(sr) / float64(2*(nFFT-1))
	}

	filterbank := make([][]float64, nMels)
	for m := 0; m < nMels; m++ {
		filterbank[m] = make([]float64, nFFT)
		lower := melPoints[m]
		center := melPoints[m+1]
		upper := melPoints[m+2]

		for k, f := range fftFreqs {
			if f >= lower && f <= center {
				filterbank[m][k] = (f - lower) / (center - lower + 1e-9)
			} else if f > center && f <= upper {
				filterbank[m][k] = (upper - f) / (upper - center + 1e-9)
			}
		}
	}
	return filterbank
}

// cmn aplica Cepstral Mean Normalization em place.
func cmn(frames [][]float64) {
	if len(frames) == 0 {
		return
	}
	means := make([]float64, len(frames[0]))
	for _, f := range frames {
		for i, v := range f {
			means[i] += v
		}
	}
	n := float64(len(frames))
	for i := range means {
		means[i] /= n
	}
	for _, f := range frames {
		for i := range f {
			f[i] -= means[i]
		}
	}
}

// onnxRuntimeLibPath retorna o caminho da libonnxruntime.so no sistema.
func onnxRuntimeLibPath() string {
	candidates := []string{
		"/usr/local/lib/libonnxruntime.so",
		"/usr/lib/libonnxruntime.so",
		"./libonnxruntime.so",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/local/lib/libonnxruntime.so" // fallback
}
