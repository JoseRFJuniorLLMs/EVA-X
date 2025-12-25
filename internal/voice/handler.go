package voice

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/gemini"
	"eva-mind/internal/telemetry"
	"eva-mind/pkg/models"
	"eva-mind/pkg/utils"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/zaf/g711"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	db           *database.DB
	cfg          *config.Config
	logger       zerolog.Logger
	alertService *AlertService
}

// AudioBuffer acumula áudio antes de enviar ao Gemini
type AudioBuffer struct {
	data         []byte
	threshold    int // bytes mínimos antes de enviar (padrão: 3200 = ~200ms)
	chunkCounter int // contador para logs periódicos
}

func NewHandler(db *database.DB, cfg *config.Config, logger zerolog.Logger, alertService *AlertService) *Handler {
	return &Handler{
		db:           db,
		cfg:          cfg,
		logger:       logger,
		alertService: alertService,
	}
}

func (h *Handler) HandleMediaStream(w http.ResponseWriter, r *http.Request) {
	// ✅ Validação melhorada do agendamento_id (Suporta Query ? ou Path /)
	agIDStr := r.URL.Query().Get("agendamento_id")
	if agIDStr == "" {
		// Tenta extrair da Path (ex: /calls/stream/123 -> 123)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) > 0 {
			agIDStr = parts[len(parts)-1]
		}
	}

	if agIDStr == "" || agIDStr == "stream" { // Evita pegar o prefixo da rota como ID
		h.logger.Error().Str("path", r.URL.Path).Msg("agendamento_id obrigatório")
		http.Error(w, "agendamento_id obrigatório", http.StatusBadRequest)
		return
	}

	agID, err := strconv.Atoi(agIDStr)
	if err != nil {
		h.logger.Error().Str("ag_id_str", agIDStr).Msg("agendamento_id inválido")
		http.Error(w, "agendamento_id inválido", http.StatusBadRequest)
		return
	}

	// ✅ Tratamento de Chamadas Especiais (Alertas e Escalonamento)
	isSpecialCall := false
	if agID < -2000000 {
		// Escalonamento: session_id = escalation_<ag_id>
		realAgID := -agID - 2000000
		agIDStr = fmt.Sprintf("escalation_%d", realAgID)
		isSpecialCall = true
		agID = realAgID // Para logs e DB lookup se necessário
	} else if agID < -1000000 {
		// Alerta Família: session_id = alert_<idoso_id>
		idosoID := -agID - 1000000
		agIDStr = fmt.Sprintf("alert_%d", idosoID)
		isSpecialCall = true
		// Como não temos um ag_id real para o alerta, vamos buscar o idoso diretamente depois
	}

	// ✅ Busca sessão Gemini
	geminiClient := GetSession(agIDStr)
	if geminiClient == nil {
		h.logger.Error().Str("session_id", agIDStr).Msg("Sessão não encontrada")
		http.Error(w, "Sessão não encontrada", http.StatusNotFound)
		return
	}

	h.logger.Info().Str("ag_id_str", agIDStr).Int("ag_id", agID).Bool("is_special", isSpecialCall).Msg("Processando Media Stream")

	// ✅ Contexto com timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	var callCtx *models.CallContext
	if agID > 0 {
		callCtx, err = h.db.GetCallContext(ctx, agID)
	} else {
		// Para alertas puros onde agID ainda pode ser negativo ou 0 se não mapeado
		// Por enquanto, vamos carregar um contexto básico se for special
		callCtx = &models.CallContext{AgendamentoID: agID, IdosoNome: "Familiar"}
	}

	if err != nil && !isSpecialCall {
		h.logger.Error().Err(err).Int("ag_id", agID).Msg("Erro ao buscar contexto")
		http.Error(w, "Erro ao buscar contexto", http.StatusInternalServerError)
		return
	}
	if callCtx == nil {
		callCtx = &models.CallContext{IdosoNome: "Desconhecido"}
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
		Inicio:        time.Now(),
	}
	histID, err := h.db.CreateHistorico(ctx, hist)
	if err != nil {
		l.Error().Err(err).Msg("Erro ao criar histórico")
	}

	errors := make(chan error, 2)
	startTime := time.Now()
	var transcript strings.Builder
	transcript.WriteString(fmt.Sprintf("--- Início da Sessão (%s) ---\n", startTime.Format("15:04:05")))

	// ✅ Telemetria e Persistência de Transcrição
	defer func() {
		duration := time.Since(startTime)
		telemetry.CallDuration.Observe(duration.Seconds())
		l.Info().Dur("duration", duration).Msg("Sessão finalizada")

		// ❌ ANÁLISE DESATIVADA TEMPORARIAMENTE PARA DEBUG
		/*
			// Salva a transcrição final e realiza análise de sentimento
			finalTranscript := transcript.String()
			if finalTranscript != "" {
				l.Info().Msg("Realizando análise de sentimento e resumo...")
				// Usamos um novo contexto para a análise não ser cancelada pelo fim da chamada
				analysisCtx, cancelAnalysis := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancelAnalysis()

				summary, sentiment, score := h.analyzeConversation(analysisCtx, finalTranscript)

				updates := map[string]interface{}{
					"fim":                    time.Now(),
					"status":                 "concluido",
					"transcricao_completa":   finalTranscript,
					"transcricao_resumo":     summary,
					"sentimento_geral":       sentiment,
					"sentimento_intensidade": score,
					"duracao_segundos":       int(duration.Seconds()),
				}
				if err := h.db.UpdateHistorico(context.Background(), histID, updates); err != nil {
					l.Error().Err(err).Msg("Erro ao salvar transcrição e análise no histórico")
				}
			}
		*/
	}()

	// Canal para sinalizar que o streamSid foi capturado
	streamSidChan := make(chan string, 1)

	// ✅ Goroutine: Gemini -> Twilio (com timeout)
	go func() {
		defer RemoveSession(agIDStr)

		// Aguarda streamSid antes de começar a enviar áudio
		var streamSid string
		select {
		case sid := <-streamSidChan:
			streamSid = sid
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			l.Warn().Msg("Timeout aguardando streamSid")
			return
		}

		for {
			select {
			case <-ctx.Done():
				l.Warn().Msg("Session context done (Gemini->Twilio)")
				return
			default:
				resp, err := geminiClient.ReadResponse()
				if err != nil {
					l.Warn().Err(err).Msg("Sessão Gemini encerrada ou erro de leitura")
					return
				}

				// Log das chaves da resposta apenas em nível trace (reduzido)
				// ... (logs de debug removidos para limpeza) ...

				audioData, ok := extractAudio(resp)
				if !ok {
					// ✅ Se não for áudio, verifica se é um Tool Call ou Transcrição de Texto
					h.handleToolCalls(ctx, agID, callCtx.IdosoID, resp, geminiClient, l)
					if txt, autor, exists := extractText(resp); exists {
						l.Info().Str("autor", autor).Str("texto", txt).Msg("Transcrição capturada")
						transcript.WriteString(fmt.Sprintf("%s: %s\n", autor, txt))
					}
					continue
				}

				// ✅ Converte PCM (Gemini) para mu-law (Twilio)
				mulawData, err := h.convertPCMToMulaw(audioData)
				if err != nil {
					l.Error().Err(err).Msg("Erro na conversão PCM->mulaw")
					continue
				}

				msg := map[string]interface{}{
					"event":     "media",
					"streamSid": streamSid, // ✅ OBRIGATÓRIO!
					"media": map[string]string{
						"payload": base64.StdEncoding.EncodeToString(mulawData),
					},
				}

				if err := conn.WriteJSON(msg); err != nil {
					l.Error().Err(err).Msg("Erro ao enviar para Twilio")
					errors <- err
					return
				}

				// Atualiza estado para Speaking
				geminiClient.SetState(StateSpeaking)
			}
		}
	}()

	// ✅ Goroutine: Twilio -> Gemini (com buffer e VAD)
	go func() {
		// Inicializa buffer com threshold menor
		buffer := &AudioBuffer{
			data:      make([]byte, 0, 3200), // ~200ms de buffer
			threshold: 1280,                  // ~80ms mínimo (mais agressivo)
		}
		vad := utils.NewVAD(300.0) // Threshold mais sensível

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

				switch event {
				case "start":
					if start, ok := twilioMsg["start"].(map[string]interface{}); ok {
						if sid, ok := start["callSid"].(string); ok {
							callSID = sid
							l.Info().Str("call_sid", callSID).Msg("Call SID capturado")
							h.db.UpdateCallStatus(ctx, agID, "em_andamento", 0)
							h.db.UpdateHistorico(ctx, histID, map[string]interface{}{"call_sid": callSID})
						}
						// Captura streamSid
						if sid, ok := start["streamSid"].(string); ok {
							l.Info().Str("stream_sid", sid).Msg("Stream SID capturado")
							streamSidChan <- sid // Desbloqueia goroutine de envio
						}

						// ✅ GATILHO INICIAL: Força EVA a falar primeiro!
						l.Info().Msg("🔔 Enviando gatilho para EVA iniciar a conversa")
						if err := geminiClient.SendText("O usuário atendeu. Diga 'Olá' e inicie a conversa."); err != nil {
							l.Error().Err(err).Msg("Erro ao enviar gatilho inicial")
						}
					}
				case "media":
					media, _ := twilioMsg["media"].(map[string]interface{})
					payloadBase64, _ := media["payload"].(string)
					mulawData, _ := base64.StdEncoding.DecodeString(payloadBase64)

					// ✅ Log reduzido: apenas a cada 50 chunks (~1 segundo)
					buffer.chunkCounter++
					if buffer.chunkCounter%50 == 0 {
						l.Debug().Int("chunks_received", buffer.chunkCounter).Msg("Processando áudio do Twilio")
					}

					// ✅ Converte mu-law (Twilio) para PCM (Gemini)
					pcmData, err := h.convertMulawToPCM(mulawData)
					if err != nil {
						l.Error().Err(err).Msg("Erro na conversão mulaw->PCM")
						continue
					}

					// ✅ Detecta atividade de voz
					hasActivity := vad.DetectActivity(pcmData)

					// Sempre adiciona ao buffer (mesmo sem atividade forte)
					buffer.data = append(buffer.data, pcmData...)

					// Envia quando:
					// 1. Atingir threshold (1280 bytes = ~80ms)
					// 2. A cada 5 chunks com atividade detectada
					// 3. A cada 3 chunks independente de atividade (fallback)
					shouldSend := len(buffer.data) >= buffer.threshold ||
						(hasActivity && buffer.chunkCounter%5 == 0) ||
						buffer.chunkCounter%3 == 0

					if shouldSend && len(buffer.data) > 0 {
						l.Debug().Int("buffer_size", len(buffer.data)).Bool("has_activity", hasActivity).Msg("Enviando buffer para Gemini")

						if err := geminiClient.SendAudio(buffer.data); err != nil {
							l.Error().Err(err).Msg("Erro ao enviar áudio para Gemini")
							errors <- err
							return
						}

						// Limpa buffer após envio
						buffer.data = buffer.data[:0]
						geminiClient.SetState(StateProcessing)
					}

				case "stop":
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
		"fim": time.Now(),
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

// ✅ Extração de transcrição de texto
func extractText(resp map[string]interface{}) (string, string, bool) {
	serverContent, ok := resp["serverContent"].(map[string]interface{})
	if !ok {
		return "", "", false
	}

	var turn map[string]interface{}
	autor := ""

	if t, ok := serverContent["modelTurn"].(map[string]interface{}); ok {
		turn = t
		autor = "EVA"
	} else if t, ok := serverContent["userTurn"].(map[string]interface{}); ok {
		turn = t
		autor = "IDOSO"
	}

	if turn == nil {
		return "", "", false
	}

	parts, ok := turn["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", "", false
	}

	var fullText strings.Builder
	for _, p := range parts {
		part, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		if txt, ok := part["text"].(string); ok {
			fullText.WriteString(txt)
		}
	}

	if fullText.Len() > 0 {
		return fullText.String(), autor, true
	}

	return "", "", false
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

// ✅ Realiza a análise da conversa usando Gemini REST API
func (h *Handler) analyzeConversation(ctx context.Context, transcript string) (string, string, int) {
	res, err := gemini.AnalyzeTranscript(ctx, h.cfg, transcript)
	if err != nil {
		h.logger.Error().Err(err).Msg("Falha na análise de sentimento da conversa")
		return "Resumo indisponível", "neutro", 5
	}
	return res.Summary, res.Sentiment, res.Score
}

// ✅ Processamento de Tool Calls (Function Calling)
func (h *Handler) handleToolCalls(ctx context.Context, agendamentoID int, idosoID int, resp map[string]interface{}, geminiClient *SafeSession, l zerolog.Logger) {
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
			result := h.dispatchFunction(ctx, agendamentoID, idosoID, name, args)

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
func (h *Handler) dispatchFunction(ctx context.Context, agendamentoID int, idosoID int, name string, args map[string]interface{}) map[string]interface{} {
	switch name {
	case "alert_family":
		motivo, _ := args["motivo"].(string)
		urgencia, _ := args["urgencia"].(string)
		h.logger.Warn().Str("motivo", motivo).Str("urgencia", urgencia).Msg("🚨 ALERTA FAMÍLIA DISPARADO")

		// Dispara a ligação real para a família
		if err := h.alertService.TriggerFamilyAlertCall(ctx, idosoID, motivo, urgencia); err != nil {
			h.logger.Error().Err(err).Msg("Falha ao disparar alerta real para família")
			return map[string]interface{}{"status": "error", "message": "Falha ao enviar alerta"}
		}

		return map[string]interface{}{"status": "alerta_enviado", "message": "Família foi notificada via ligação de voz"}

	case "confirm_medication":
		med, _ := args["medicamento"].(string)
		tomou, _ := args["tomou"].(bool)

		h.logger.Info().Str("medicamento", med).Bool("tomou", tomou).Msg("💊 CONFIRMAÇÃO DE MEDICAMENTO")

		// ✅ PERSISTÊNCIA NO BANCO
		if err := h.db.ConfirmMedication(ctx, agendamentoID, tomou); err != nil {
			h.logger.Error().Err(err).Msg("Erro ao salvar confirmação de medicamento no DB")
			return map[string]interface{}{"status": "error", "message": "Erro ao salvar no banco"}
		}

		return map[string]interface{}{"status": "success", "confirmed": tomou}

	default:
		return map[string]interface{}{"error": "função não encontrada"}
	}
}
