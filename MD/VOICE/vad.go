package voice

import (
	"math"
)

// VAD implementa Voice Activity Detection por energia adaptativa.
// Sem dependências externas — puramente Go.
//
// Princípio: janelas de 20ms são classificadas como fala/silêncio
// comparando a energia RMS contra um limiar adaptativo baseado
// no histórico recente do sinal.

const (
	vadSampleRate    = 16000
	vadFrameMS       = 20                              // ms por frame
	vadFrameSamples  = vadSampleRate * vadFrameMS / 1000 // 320 amostras
	vadHangoverFrames = 8                              // frames de "cauda" após fala
	minSpeechFrames  = 15                              // mínimo para considerar fala real (~300ms)
)

// VADResult contém o áudio filtrado e métricas de qualidade.
type VADResult struct {
	Speech       []float32 // Apenas os frames de fala
	SpeechRatio  float64   // Proporção de fala no áudio original (0.0–1.0)
	RMSdB        float64   // RMS médio da fala em dB
	Frames       int       // Total de frames analisados
	SpeechFrames int       // Frames classificados como fala
}

// ApplyVAD remove silêncio e ruído de um áudio PCM float32 a 16kHz.
func ApplyVAD(samples []float32) VADResult {
	if len(samples) == 0 {
		return VADResult{}
	}

	// 1. Divide em frames de 20ms
	numFrames := len(samples) / vadFrameSamples
	frameEnergies := make([]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		frame := samples[i*vadFrameSamples : (i+1)*vadFrameSamples]
		frameEnergies[i] = rmsEnergy(frame)
	}

	// 2. Estima ruído de fundo: percentil 10 das energias
	noiseFloor := percentile(frameEnergies, 10)

	// 3. Limiar adaptativo: 6dB acima do ruído de fundo
	threshold := noiseFloor * 2.0 // x2 em amplitude linear ≈ +6dB

	// 4. Classifica frames com hangover
	isSpeech := make([]bool, numFrames)
	hangover := 0
	for i, e := range frameEnergies {
		if e >= threshold {
			isSpeech[i] = true
			hangover = vadHangoverFrames
		} else if hangover > 0 {
			isSpeech[i] = true
			hangover--
		}
	}

	// 5. Extrai frames de fala
	var speechSamples []float32
	speechFrames := 0
	var sumSquares float64

	for i, active := range isSpeech {
		if active {
			chunk := samples[i*vadFrameSamples : (i+1)*vadFrameSamples]
			speechSamples = append(speechSamples, chunk...)
			speechFrames++
			for _, s := range chunk {
				sumSquares += float64(s) * float64(s)
			}
		}
	}

	if len(speechSamples) == 0 {
		return VADResult{Frames: numFrames}
	}

	rmsLinear := math.Sqrt(sumSquares / float64(len(speechSamples)))
	rmsDB := 20 * math.Log10(rmsLinear+1e-9)

	return VADResult{
		Speech:       speechSamples,
		SpeechRatio:  float64(speechFrames) / float64(numFrames),
		RMSdB:        rmsDB,
		Frames:       numFrames,
		SpeechFrames: speechFrames,
	}
}

// HasEnoughSpeech verifica se o resultado do VAD tem fala suficiente
// para gerar um embedding confiável (mínimo ~1.5s).
func HasEnoughSpeech(r VADResult) bool {
	return r.SpeechFrames >= minSpeechFrames
}

// rmsEnergy calcula a energia RMS de um frame.
func rmsEnergy(frame []float32) float64 {
	var sum float64
	for _, s := range frame {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(frame)))
}

// percentile retorna o valor no percentil p de uma slice (0–100).
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	// Insertion sort (N é pequeno: tipicamente < 500 frames)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	idx := int(p / 100.0 * float64(len(sorted)-1))
	return sorted[idx]
}
