package voice

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/telemetry"
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

	// ✅ Busca contexto do agendamento no DB com timeout de 10 minutos
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

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

	// ✅ Telemetria: Registra métrica ao final
	defer func() {
		duration := time.Since(startTime)
		telemetry.CallDuration.Observe(duration.Seconds())
		l.Info().Dur("duration", duration).Msg("Sessão finalizada")
	}()

	// ✅ Goroutine: Gemini -> Twilio (com timeout)
	go func() {
		defer RemoveSession(agIDStr)
		for {
			select {
			case <-ctx.Done():
				l.Warn().Msg("Session context done (Gemini->Twilio)")
				return
			default:
				resp, err := geminiClient.ReadResponse()
				if err != nil {
					l.Warn().Err(err).Msg("Sessão Gemini encerrada ou erro de leitura")
					// errors <- err // Não envia aqui para deixar a outra goroutine limpar se necessário ou aguardar stop
					return
				}

				audioData, ok := extractAudio(resp)
				if !ok {
					// ✅ Se não for áudio, verifica se é um Tool Call
					h.handleToolCalls(resp, geminiClient, l)
					continue
				}

				// ✅ Converte PCM (Gemini) para mu-law (Twilio)
				mulawData, err := h.convertPCMToMulaw(audioData)
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
		for {
			select {
			case <-ctx.Done():
				l.Warn().Msg("Session context done (Twilio->Gemini)")
				return
			default:
				var twilioMsg map[string]interface{}
				if err := conn.ReadJSON(&twilioMsg); err != nil {
					l.Error().Err(err).Msg("Erro ao ler do Twilio")
					errors <- err
					return
				}

				event, _ := twilioMsg["event"].(string)

				if event == "start" {
					if start, ok := twilioMsg["start"].(map[string]interface{}); ok {
						if sid, ok := start["callSid"].(string); ok {
							callSID = sid
							l.Info().Str("call_sid", callSID).Msg("Call SID capturado")
							h.db.UpdateCallStatus(ctx, agID, "em_andamento", 0)
							h.db.UpdateHistorico(ctx, histID, map[string]interface{}{"call_sid": callSID})
						}
					}
					continue
				}

				if event == "media" {
					media, _ := twilioMsg["media"].(map[string]interface{})
					payloadBase64, _ := media["payload"].(string)
					mulawData, _ := base64.StdEncoding.DecodeString(payloadBase64)

					pcmData, err := h.convertMulawToPCM(mulawData)
					if err != nil {
						l.Error().Err(err).Msg("Erro na conversão mulaw->PCM")
						continue
					}

					if err := geminiClient.SendAudio(pcmData); err != nil {
						l.Warn().Err(err).Msg("Sessão Gemini encerrada ou erro ao enviar áudio")
						errors <- err
						return
					}
				}

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

	// ✅ Atualiza status final
	finalStatus := "concluido"
	if err != nil {
		finalStatus = "aguardando_retry"
		l.Error().Err(err).Msg("Chamada falhou")
		h.db.UpdateCallStatus(ctx, agID, finalStatus, callCtx.RetryInterval)
	} else {
		h.db.UpdateCallStatus(ctx, agID, finalStatus, 0)
	}

	// ✅ Registra métrica de status
	telemetry.CallsTotal.WithLabelValues(finalStatus).Inc()

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
func (h *Handler) convertMulawToPCM(mulaw []byte) ([]byte, error) {
	if len(mulaw) == 0 {
		return nil, fmt.Errorf("empty mulaw data")
	}

	// 1. Decodifica mu-law para PCM 16-bit bytes (ainda 8kHz)
	pcm8kBytes := g711.DecodeUlaw(mulaw)
	pcm8k := h.bytesToInt16(pcm8kBytes)

	// 2. Resample de 8kHz para 16kHz (dobra as amostras)
	pcm16k := h.resample8to16kHz(pcm8k)

	// 3. Converte int16 para bytes (little-endian)
	return h.int16ToBytes(pcm16k), nil
}

// ✅ CONVERSÃO COMPLETA: PCM 24kHz -> mu-law 8kHz
func (h *Handler) convertPCMToMulaw(pcm []byte) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, fmt.Errorf("empty pcm data")
	}

	// 1. Converte bytes para int16
	samples := h.bytesToInt16(pcm)

	// 2. Resample de 24kHz para 8kHz (reduz por fator de 3)
	samples8k := h.resample24to8kHz(samples)

	// 3. Codifica para mu-law
	samples8kBytes := h.int16ToBytes(samples8k)
	mulaw := g711.EncodeUlaw(samples8kBytes)

	return mulaw, nil
}

// ✅ Resample 8kHz -> 16kHz (linear interpolation)
func (h *Handler) resample8to16kHz(samples []int16) []int16 {
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
func (h *Handler) resample24to8kHz(samples []int16) []int16 {
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
func (h *Handler) int16ToBytes(samples []int16) []byte {
	bytes := make([]byte, len(samples)*2)
	for i, sample := range samples {
		bytes[i*2] = byte(sample)
		bytes[i*2+1] = byte(sample >> 8)
	}
	return bytes
}

// ✅ Converte bytes para int16 (little-endian) com validação
func (h *Handler) bytesToInt16(bytes []byte) []int16 {
	if len(bytes)%2 != 0 {
		h.logger.Warn().Int("len", len(bytes)).Msg("PCM data has odd number of bytes, truncating last byte")
		bytes = bytes[:len(bytes)-1]
	}
	samples := make([]int16, len(bytes)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(bytes[i*2]) | int16(bytes[i*2+1])<<8
	}
	return samples
}

// ✅ Processamento de Tool Calls (Function Calling)
func (h *Handler) handleToolCalls(resp map[string]interface{}, geminiClient *SafeSession, l zerolog.Logger) {
	serverContent, ok := resp["serverContent"].(map[string]interface{})
	if !ok {
		return
	}
	modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
	if !ok {
		return
	}
	parts, ok := modelTurn["parts"].([]interface{})
	if !ok {
		return
	}

	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		toolCall, ok := part["toolCall"].(map[string]interface{})
		if !ok {
			continue
		}

		calls, ok := toolCall["functionCalls"].([]interface{})
		if !ok {
			continue
		}

		for _, c := range calls {
			call, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := call["name"].(string)
			args, _ := call["args"].(map[string]interface{})
			callID, _ := call["id"].(string)

			l.Info().Str("function", name).Interface("args", args).Msg("Gemini solicitou chamada de função")

			// Executa a função localmente
			result := h.dispatchFunction(name, args)

			// Envia a resposta de volta para o Gemini
			toolResponse := map[string]interface{}{
				"tool_response": map[string]interface{}{
					"function_responses": []map[string]interface{}{
						{
							"name":     name,
							"id":       callID,
							"response": result,
						},
					},
				},
			}

			if err := geminiClient.SendToolResponse(toolResponse); err != nil {
				l.Error().Err(err).Msg("Erro ao enviar resposta de ferramenta para Gemini")
			}
		}
	}
}

// ✅ Despacha a execução para a função correta
func (h *Handler) dispatchFunction(name string, args map[string]interface{}) map[string]interface{} {
	switch name {
	case "alert_family":
		motivo, _ := args["motivo"].(string)
		urgencia, _ := args["urgencia"].(string)
		h.logger.Warn().Str("motivo", motivo).Str("urgencia", urgencia).Msg("🚨 ALERTA FAMÍLIA DISPARADO")
		// TODO: Implementar envio real de alerta (Sprint 3 P1)
		return map[string]interface{}{"status": "alerta_enviado", "message": "Família foi notificada"}

	case "confirm_medication":
		med, _ := args["medicamento"].(string)
		tomou, _ := args["tomou"].(bool)
		h.logger.Info().Str("medicamento", med).Bool("tomou", tomou).Msg("💊 CONFIRMAÇÃO DE MEDICAMENTO")
		return map[string]interface{}{"status": "success", "confirmed": tomou}

	default:
		return map[string]interface{}{"error": "função não encontrada"}
	}
}
