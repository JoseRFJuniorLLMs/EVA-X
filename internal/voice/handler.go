package voice

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/pkg/models"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/zaf/g711"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	db     *database.DB
	cfg    *config.Config
	logger zerolog.Logger
}

func NewHandler(db *database.DB, cfg *config.Config, logger zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		cfg:    cfg,
		logger: logger,
	}
}

func (h *Handler) HandleMediaStream(w http.ResponseWriter, r *http.Request) {
	// ✅ Validação melhorada do agendamento_id
	agIDStr := r.URL.Query().Get("agendamento_id")
	if agIDStr == "" {
		h.logger.Error().Msg("agendamento_id obrigatório")
		http.Error(w, "agendamento_id obrigatório", http.StatusBadRequest)
		return
	}

	agID, err := strconv.Atoi(agIDStr)
	if err != nil || agID <= 0 {
		h.logger.Error().Str("ag_id_str", agIDStr).Msg("agendamento_id inválido")
		http.Error(w, "agendamento_id inválido", http.StatusBadRequest)
		return
	}

	// ✅ Busca sessão Gemini pré-criada
	geminiClient := GetSession(agIDStr)
	if geminiClient == nil {
		h.logger.Error().Int("ag_id", agID).Msg("Sessão não encontrada")
		http.Error(w, "Sessão não encontrada", http.StatusNotFound)
		return
	}

	// ✅ Busca contexto do agendamento no DB
	ctx := r.Context()
	callCtx, err := h.db.GetCallContext(ctx, agID)
	if err != nil {
		h.logger.Error().Err(err).Int("ag_id", agID).Msg("Erro ao buscar contexto")
		http.Error(w, "Erro ao buscar contexto", http.StatusInternalServerError)
		return
	}

	// ✅ Upgrade WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	l := h.logger.With().
		Int("ag_id", agID).
		Str("idoso_nome", callCtx.IdosoNome).
		Str("telefone", callCtx.Telefone).
		Logger()

	l.Info().Msg("Twilio Media Stream connected")

	// ✅ Atualiza status para em_andamento
	if err := h.db.UpdateCallStatus(ctx, agID, "em_andamento", 0); err != nil {
		l.Error().Err(err).Msg("Erro ao atualizar status")
	}

	// ✅ Variável para armazenar call_sid
	var callSID string

	// ✅ Cria registro de histórico
	hist := &models.Historico{
		AgendamentoID: agID,
		IdosoID:       callCtx.IdosoID,
		Status:        "iniciada",
		Inicio:        time.Now(),
	}
	histID, err := h.db.CreateHistorico(ctx, hist)
	if err != nil {
		l.Error().Err(err).Msg("Erro ao criar histórico")
	}

	errors := make(chan error, 2)
	startTime := time.Now()

	// ✅ Goroutine: Gemini -> Twilio (com timeout)
	go func() {
		timeout := time.NewTimer(10 * time.Minute)
		defer timeout.Stop()

		for {
			select {
			case <-timeout.C:
				l.Warn().Msg("Timeout na conversa (Gemini->Twilio)")
				errors <- fmt.Errorf("timeout gemini->twilio")
				return

			default:
				resp, err := geminiClient.ReadResponse()
				if err != nil {
					l.Error().Err(err).Msg("Erro ao ler resposta Gemini")
					errors <- err
					return
				}

				audioData, ok := extractAudio(resp)
				if !ok {
					continue
				}

				l.Debug().
					Int("audio_bytes", len(audioData)).
					Str("direction", "gemini->twilio").
					Msg("Áudio recebido do Gemini")

				// ✅ Converte PCM 24kHz para mu-law 8kHz
				mulawData, err := convertPCMToMulaw(audioData)
				if err != nil {
					l.Error().Err(err).Msg("Erro na conversão PCM->mulaw")
					continue
				}

				msg := map[string]interface{}{
					"event": "media",
					"media": map[string]string{
						"payload": base64.StdEncoding.EncodeToString(mulawData),
					},
				}

				if err := conn.WriteJSON(msg); err != nil {
					l.Error().Err(err).Msg("Erro ao enviar para Twilio")
					errors <- err
					return
				}
			}
		}
	}()

	// ✅ Goroutine: Twilio -> Gemini (com timeout)
	go func() {
		timeout := time.NewTimer(10 * time.Minute)
		defer timeout.Stop()

		for {
			select {
			case <-timeout.C:
				l.Warn().Msg("Timeout na conversa (Twilio->Gemini)")
				errors <- fmt.Errorf("timeout twilio->gemini")
				return

			default:
				var twilioMsg map[string]interface{}
				if err := conn.ReadJSON(&twilioMsg); err != nil {
					l.Error().Err(err).Msg("Erro ao ler do Twilio")
					errors <- err
					return
				}

				event, _ := twilioMsg["event"].(string)

				// ✅ Captura call_sid do evento "start"
				if event == "start" {
					if start, ok := twilioMsg["start"].(map[string]interface{}); ok {
						if sid, ok := start["callSid"].(string); ok {
							callSID = sid
							l.Info().Str("call_sid", callSID).Msg("Call SID capturado")

							// Atualiza no DB (não mudamos o status, apenas garantimos que está em_andamento)
							h.db.UpdateCallStatus(ctx, agID, "em_andamento", 0)

							// Atualiza histórico com call_sid
							h.db.UpdateHistorico(ctx, histID, map[string]interface{}{
								"call_sid": callSID,
							})
						}
					}
					continue
				}

				// ✅ Processa áudio
				if event == "media" {
					media, ok := twilioMsg["media"].(map[string]interface{})
					if !ok {
						continue
					}

					payloadBase64, ok := media["payload"].(string)
					if !ok {
						continue
					}

					mulawData, err := base64.StdEncoding.DecodeString(payloadBase64)
					if err != nil {
						l.Error().Err(err).Msg("Erro ao decodificar base64")
						continue
					}

					// ✅ Converte mu-law 8kHz para PCM 16kHz
					pcmData, err := convertMulawToPCM(mulawData)
					if err != nil {
						l.Error().Err(err).Msg("Erro na conversão mulaw->PCM")
						continue
					}

					l.Debug().
						Int("audio_bytes", len(pcmData)).
						Str("direction", "twilio->gemini").
						Msg("Áudio enviado para Gemini")

					if err := geminiClient.SendAudio(pcmData); err != nil {
						l.Error().Err(err).Msg("Erro ao enviar áudio para Gemini")
						errors <- err
						return
					}
				}

				// ✅ Evento stop
				if event == "stop" {
					l.Info().Msg("Twilio stream stopped")
					errors <- nil
					return
				}
			}
		}
	}()

	// ✅ Aguarda finalização
	err = <-errors
	duration := time.Since(startTime)

	l.Info().
		Dur("duration", duration).
		Msg("Cleaning up session")

	// ✅ Atualiza status final
	finalStatus := "concluido"
	if err != nil {
		finalStatus = "aguardando_retry"
		l.Error().Err(err).Msg("Chamada falhou")
		h.db.UpdateCallStatus(ctx, agID, finalStatus, callCtx.RetryInterval)
	} else {
		h.db.UpdateCallStatus(ctx, agID, finalStatus, 0)
	}

	// ✅ Atualiza histórico final
	h.db.UpdateHistorico(ctx, histID, map[string]interface{}{
		"fim":    time.Now(),
		"status": finalStatus,
	})

	RemoveSession(agIDStr)
}

// ✅ Extração de áudio com validações robustas
func extractAudio(resp map[string]interface{}) ([]byte, bool) {
	serverContent, ok := resp["serverContent"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
	if !ok {
		return nil, false
	}

	parts, ok := modelTurn["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return nil, false
	}

	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		inlineData, ok := part["inlineData"].(map[string]interface{})
		if !ok {
			continue
		}

		data, ok := inlineData["data"].(string)
		if !ok {
			continue
		}

		audio, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			continue
		}

		return audio, true
	}

	return nil, false
}

// ✅ CONVERSÃO COMPLETA: mu-law 8kHz -> PCM 16kHz
func convertMulawToPCM(mulaw []byte) ([]byte, error) {
	if len(mulaw) == 0 {
		return nil, fmt.Errorf("empty mulaw data")
	}

	// 1. Decodifica mu-law para PCM 16-bit bytes (ainda 8kHz)
	pcm8kBytes := g711.DecodeUlaw(mulaw)
	pcm8k := bytesToInt16(pcm8kBytes)

	// 2. Resample de 8kHz para 16kHz (dobra as amostras)
	pcm16k := resample8to16kHz(pcm8k)

	// 3. Converte int16 para bytes (little-endian)
	return int16ToBytes(pcm16k), nil
}

// ✅ CONVERSÃO COMPLETA: PCM 24kHz -> mu-law 8kHz
func convertPCMToMulaw(pcm []byte) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, fmt.Errorf("empty pcm data")
	}

	// 1. Converte bytes para int16
	samples := bytesToInt16(pcm)

	// 2. Resample de 24kHz para 8kHz (reduz por fator de 3)
	samples8k := resample24to8kHz(samples)

	// 3. Codifica para mu-law
	samples8kBytes := int16ToBytes(samples8k)
	mulaw := g711.EncodeUlaw(samples8kBytes)

	return mulaw, nil
}

// ✅ Resample 8kHz -> 16kHz (linear interpolation)
func resample8to16kHz(samples []int16) []int16 {
	outLen := len(samples) * 2
	output := make([]int16, outLen)

	for i := 0; i < len(samples); i++ {
		output[i*2] = samples[i]

		// Interpolação linear para preencher amostras intermediárias
		if i < len(samples)-1 {
			output[i*2+1] = int16((int32(samples[i]) + int32(samples[i+1])) / 2)
		} else {
			output[i*2+1] = samples[i]
		}
	}

	return output
}

// ✅ Resample 24kHz -> 8kHz (decimate by 3)
func resample24to8kHz(samples []int16) []int16 {
	outLen := len(samples) / 3
	output := make([]int16, outLen)

	for i := 0; i < outLen; i++ {
		// Média simples de 3 amostras para evitar aliasing
		idx := i * 3
		if idx+2 < len(samples) {
			sum := int32(samples[idx]) + int32(samples[idx+1]) + int32(samples[idx+2])
			output[i] = int16(sum / 3)
		} else {
			output[i] = samples[idx]
		}
	}

	return output
}

// ✅ Converte int16 para bytes (little-endian)
func int16ToBytes(samples []int16) []byte {
	bytes := make([]byte, len(samples)*2)
	for i, sample := range samples {
		bytes[i*2] = byte(sample)
		bytes[i*2+1] = byte(sample >> 8)
	}
	return bytes
}

// ✅ Converte bytes para int16 (little-endian)
func bytesToInt16(bytes []byte) []int16 {
	samples := make([]int16, len(bytes)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(bytes[i*2]) | int16(bytes[i*2+1])<<8
	}
	return samples
}
