package voice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// Handler expõe os endpoints HTTP do módulo de biometria de voz.
//
// Endpoints:
//   POST /voice/enroll   — Cadastra novo falante (multipart, N arquivos WAV)
//   POST /voice/identify — Identifica falante a partir de um WAV (para testes manuais)
//   DELETE /voice/:id    — Desativa perfil de voz
//   GET  /voice/profiles — Lista perfis ativos (admin)

type Handler struct {
	pipeline *Pipeline
	log      *zap.Logger
}

func NewHandler(pipeline *Pipeline, log *zap.Logger) *Handler {
	return &Handler{pipeline: pipeline, log: log}
}

// RegisterRoutes registra os endpoints no mux fornecido.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /voice/enroll",    h.handleEnroll)
	mux.HandleFunc("POST /voice/identify",  h.handleIdentify)
}

// ─── POST /voice/enroll ────────────────────────────────────────────────────
//
// Recebe N arquivos WAV (multipart/form-data) + campos speaker_id e name.
// Recomendado: 7–10 amostras, cada uma com ~3–5s de fala limpa.
//
// Form fields:
//   speaker_id  string  (ex: "person_junior")
//   name        string  (ex: "Junior")
//   audio[]     file[]  (WAV PCM float32 ou int16, 16kHz mono)

func (h *Handler) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB limit
		jsonError(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	speakerID := r.FormValue("speaker_id")
	name := r.FormValue("name")
	if speakerID == "" || name == "" {
		jsonError(w, "speaker_id e name são obrigatórios", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["audio[]"]
	if len(files) < 3 {
		jsonError(w, fmt.Sprintf("mínimo 3 arquivos de áudio, recebido %d", len(files)), http.StatusBadRequest)
		return
	}

	var samples []EnrollSample
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(f)
		f.Close()

		pcm, err := decodeWAVtoPCM(raw)
		if err != nil {
			h.log.Warn("enroll: arquivo WAV inválido", zap.String("file", fh.Filename), zap.Error(err))
			continue
		}
		samples = append(samples, EnrollSample{
			SpeakerID: speakerID,
			Name:      name,
			Samples:   pcm,
		})
	}

	result, err := h.pipeline.Enroll(r.Context(), samples)
	if err != nil {
		jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	jsonOK(w, map[string]any{
		"speaker_id":     result.SpeakerID,
		"name":           result.Name,
		"samples_used":   result.SamplesUsed,
		"intra_variance": result.IntraVariance,
		"quality":        result.Quality,
	})
}

// ─── POST /voice/identify ──────────────────────────────────────────────────
//
// Endpoint de teste manual: recebe um único WAV e retorna a identificação.
// Em produção, a identificação é feita pelo WebSocket handler no eva-mind.

func (h *Handler) handleIdentify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	pcm, err := decodeWAVtoPCM(raw)
	if err != nil {
		jsonError(w, "decode wav: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	result, err := h.pipeline.Identify(r.Context(), pcm)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"unknown":       result.Unknown,
		"latency_ms":    result.Latency.Milliseconds(),
		"speech_ratio":  result.SpeechRatio,
		"audio_quality": result.AudioQuality,
	}

	if !result.Unknown {
		resp["speaker_id"]  = result.SpeakerID
		resp["name"]        = result.Name
		resp["confidence"]  = result.Confidence
		resp["cosine_sim"]  = result.CosineSim
		resp["omp_coeff"]   = result.OMPCoeff
		resp["residual"]    = result.ResidualErr
		resp["omp_iters"]   = result.Iterations
		resp["priming"]     = result.GeminiPriming
	}

	jsonOK(w, resp)
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// decodeWAVtoPCM decodifica um arquivo WAV para PCM float32 a 16kHz.
// Suporta: PCM int16 (formato mais comum), float32, e mono/stereo (downmix).
func decodeWAVtoPCM(data []byte) ([]float32, error) {
	if len(data) < 44 {
		return nil, fmt.Errorf("arquivo muito pequeno para ser WAV (%d bytes)", len(data))
	}

	// Lê o header WAV
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("não é um arquivo WAV válido")
	}

	// Localiza o chunk "fmt "
	var audioFormat uint16
	var numChannels uint16
	var sampleRate  uint32
	var bitsPerSample uint16
	var dataOffset int

	i := 12
	for i < len(data)-8 {
		chunkID   := string(data[i : i+4])
		chunkSize := int(uint32(data[i+4]) | uint32(data[i+5])<<8 | uint32(data[i+6])<<16 | uint32(data[i+7])<<24)

		if chunkID == "fmt " {
			audioFormat  = uint16(data[i+8])  | uint16(data[i+9])<<8
			numChannels  = uint16(data[i+10]) | uint16(data[i+11])<<8
			sampleRate   = uint32(data[i+12]) | uint32(data[i+13])<<8 | uint32(data[i+14])<<16 | uint32(data[i+15])<<24
			bitsPerSample = uint16(data[i+22]) | uint16(data[i+23])<<8
		}
		if chunkID == "data" {
			dataOffset = i + 8
			break
		}
		i += 8 + chunkSize
	}

	if dataOffset == 0 {
		return nil, fmt.Errorf("chunk 'data' não encontrado no WAV")
	}
	if sampleRate != 16000 {
		return nil, fmt.Errorf("sample rate %dHz não suportado — converta para 16kHz antes de enviar", sampleRate)
	}

	rawData := data[dataOffset:]

	var samples []float32

	switch {
	case audioFormat == 1 && bitsPerSample == 16: // PCM int16
		if len(rawData)%2 != 0 {
			rawData = rawData[:len(rawData)-1]
		}
		step := int(numChannels)
		for i := 0; i+1 < len(rawData); i += 2 * step {
			// Canal 0 apenas (downmix simples)
			s := int16(uint16(rawData[i]) | uint16(rawData[i+1])<<8)
			samples = append(samples, float32(s)/32768.0)
		}

	case audioFormat == 3 && bitsPerSample == 32: // IEEE float32
		if len(rawData)%4 != 0 {
			rawData = rawData[:len(rawData)-(len(rawData)%4)]
		}
		step := int(numChannels) * 4
		for i := 0; i+3 < len(rawData); i += step {
			bits := uint32(rawData[i]) | uint32(rawData[i+1])<<8 | uint32(rawData[i+2])<<16 | uint32(rawData[i+3])<<24
			samples = append(samples, float32FromBits(bits))
		}

	default:
		return nil, fmt.Errorf("formato WAV não suportado: audioFormat=%d, bits=%d", audioFormat, bitsPerSample)
	}

	return samples, nil
}

// float32FromBits converte uint32 para float32 sem unsafe.
func float32FromBits(b uint32) float32 {
	// Usa encoding/binary semantics manualmente
	// Equivalente a math.Float32frombits
	sign := float32(1)
	if b>>31 != 0 {
		sign = -1
	}
	exp := int((b>>23)&0xFF) - 127
	mantissa := float64(b&0x7FFFFF)/float64(1<<23) + 1.0
	if exp == -127 {
		mantissa = float64(b&0x7FFFFF) / float64(1<<23)
		exp = -126
	}
	result := sign * float32(mantissa)
	for exp > 0 {
		result *= 2
		exp--
	}
	for exp < 0 {
		result /= 2
		exp++
	}
	return result
}
