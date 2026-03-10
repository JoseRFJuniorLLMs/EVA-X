// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"eva/internal/cortex/brain"
	gemini "eva/internal/cortex/gemini"
	evaSelf "eva/internal/cortex/self"
	"eva/internal/cortex/voice/speaker"
	"eva/internal/tools"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// geminiApp — Voz e Video via WebSocket para App Mobile (EVA-Mobile)
// ============================================================================
// Consumer:  geminiApp
// Rota:      /ws/browser
// Client:    internal/gemini (v1beta, producao)
// Frontend:  App mobile EVA-Mobile / Malaria frontend
// Protocolo: WebSocket — audio PCM (16kHz in, 24kHz out) + video JPEG + texto
// Memoria:   Meta-cognitiva via NietzscheDB (carrega no inicio, salva transcricoes)
// CRITICO:   Protocolo WebSocket NAO pode mudar — app mobile depende
// Ver:       GEMINI_ARCHITECTURE.md para documentacao completa
//
// RECONEXAO: Quando o Gemini faz timeout (~10 min), o handler reconecta
// automaticamente sem fechar o WebSocket do browser. O browser recebe
// {"type":"status","text":"reconnecting"} e depois {"type":"status","text":"ready"}.

// browserWSUpgrader permite conexoes de browsers com verificacao de origem
var browserWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Permitir conexões sem Origin (ex: apps mobile, curl)
		}
		allowedOrigins := []string{
			"https://eva-ia.org",
			"https://www.eva-ia.org",
			"https://app.eva-ia.org",
			"https://136.111.0.47",
			"http://136.111.0.47",
			"http://localhost:3000",
			"http://localhost:8080",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// browserMessage formato de mensagem browser <-> server
type browserMessage struct {
	Type     string      `json:"type"`                // "audio", "text", "config", "status", "tool_event"
	Data     string      `json:"data,omitempty"`      // base64 PCM para audio / "user" para transcricao
	Text     string      `json:"text,omitempty"`      // texto para subtitles/chat
	Tool     string      `json:"tool,omitempty"`      // nome da tool executada
	ToolData interface{} `json:"tool_data,omitempty"` // payload estruturado da tool
	Status   string      `json:"status,omitempty"`    // "executing", "success", "error"
}

// browserSignalKind classifica o tipo de sinal do loop de reconexao
type browserSignalKind int

const (
	bsigFatal     browserSignalKind = iota // browser desconectou ou erro irrecuperavel
	bsigReconnect                          // Gemini fez timeout — reconectar sem fechar o browser WS
)

// browserSignal carrega o tipo de sinal, a geracao do client e o erro
type browserSignal struct {
	kind browserSignalKind
	gen  int64 // geracao do client Gemini que gerou o sinal (0 = writer goroutine)
	err  error
}

// isGeminiTimeout detecta erros de timeout/cancelamento da API Gemini Live.
// Esses erros sao esperados apos ~10 minutos de sessao e sao recuperaveis.
func isGeminiTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "CANCELLED") ||
		strings.Contains(msg, "Thread was cancelled") ||
		strings.Contains(msg, "websocket: close 1011") ||
		strings.Contains(msg, "context deadline exceeded")
}

// handleBrowserVoice lida com WebSocket de voz vindo do browser.
// Reconecta automaticamente ao Gemini quando a sessao expira (~10 min),
// sem interromper o WebSocket do browser (max 5 reconexoes por sessao).
func (s *SignalingServer) handleBrowserVoice(w http.ResponseWriter, r *http.Request) {
	conn, err := browserWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Browser WS upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sessionID := "browser-" + time.Now().Format("20060102150405")
	sessionStart := time.Now()

	// --- Config inicial do cliente ---
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, configBytes, err := conn.ReadMessage()
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		log.Error().Err(err).Msg("[BROWSER] Timeout esperando config do cliente")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: config timeout"})
		return
	}

	var configMsg browserMessage
	var clientContext string
	var clientCPF string
	if err := json.Unmarshal(configBytes, &configMsg); err == nil && configMsg.Type == "config" {
		clientContext = configMsg.Text
		clientCPF = configMsg.Data
	}

	if clientContext == "" {
		clientContext = "Voce e a EVA, assistente virtual inteligente. Responda em portugues de forma clara e profissional."
	}

	log.Info().Str("session", sessionID).Str("cpf", clientCPF).Bool("hasContext", configMsg.Text != "").Msg("[BROWSER] Config recebida do cliente")

	// === CARREGAR CONTEXTO E MEMORIAS ===
	var memories []string

	// 1. Iniciar sessao e carregar meta-cognicao (sessoes recentes, topicos, insights)
	if s.evaMemory != nil {
		s.evaMemory.StartSession(ctx, sessionID)
		metaCognition, err := s.evaMemory.LoadMetaCognition(ctx)
		if err == nil && metaCognition != "" {
			memories = []string{metaCognition}
			log.Info().Str("session", sessionID).Int("len", len(metaCognition)).Msg("[BROWSER] Meta-cognicao carregada")
		} else if err != nil {
			log.Warn().Err(err).Str("session", sessionID).Msg("[BROWSER] Erro ao carregar meta-cognicao")
		}
	}

	// 2. Carregar identidade (personalidade, memorias core, capacidades)
	if s.coreMemory != nil {
		identityCtx, err := s.coreMemory.GetIdentityContext(ctx)
		if err == nil && identityCtx != "" {
			clientContext = identityCtx + "\n\n---\n\n" + clientContext
			log.Info().Str("session", sessionID).Int("len", len(identityCtx)).Msg("[BROWSER] Identidade carregada")
		} else if err != nil {
			log.Warn().Err(err).Str("session", sessionID).Msg("[BROWSER] Erro ao carregar identidade")
		}
	}

	log.Info().Str("session", sessionID).Int("memories", len(memories)).Bool("hasIdentity", s.coreMemory != nil).Msg("[BROWSER] Contexto carregado")

	// --- setupGemini: cria e configura um novo client Gemini ---
	// Captura clientContext e memories do escopo externo — sao imutaveis apos esta linha.
	// Tool definitions para Gemini Function Calling (Swarm tools + built-in)
	toolDefs := tools.GetToolDefinitions()

	setupGemini := func() (*gemini.Client, error) {
		client, err := gemini.NewClient(ctx, s.cfg)
		if err != nil {
			return nil, err
		}
		if err := client.SendSetup(
			clientContext,
			map[string]interface{}{"voiceName": "Aoede", "languageCode": "pt-BR"},
			memories, "", toolDefs,
		); err != nil {
			client.Close()
			return nil, err
		}
		return client, nil
	}

	initialClient, err := setupGemini()
	if err != nil {
		log.Error().Err(err).Msg("Erro ao configurar Gemini para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: setup failed"})
		return
	}

	log.Info().Str("session", sessionID).Int("memories", len(memories)).Msg("Browser voice session started (cortex/gemini)")

	conn.WriteJSON(browserMessage{Type: "status", Text: "ready"})

	// --- Estado compartilhado entre goroutines ---
	var writeMu sync.Mutex      // protege escritas no conn do browser

	var geminiMu sync.RWMutex   // protege geminiRef
	geminiRef := initialClient  // client Gemini ativo
	var currentGen int64 = 1    // geracao atual (incrementada a cada reconexao)
	var reconnecting atomic.Bool // true enquanto reconexao em progresso
	var responseAccum strings.Builder
	var responseAccumMu sync.Mutex // protege responseAccum contra acessos concorrentes

	// Transcript accumulation — necessario para CoreMemory.ProcessSessionEnd
	var transcriptMu sync.Mutex
	var transcriptAccum strings.Builder // transcript completo da sessao
	var evaResponses []string           // respostas da EVA para CoreMemory

	// sigChan recebe sinais das goroutines para o loop principal
	// Buffer 4: captura sinais de goroutines mortas sem bloquear
	sigChan := make(chan browserSignal, 4)

	const maxReconnects = 5

	// --- startReader: lanca goroutine que le do client Gemini e encaminha ao browser ---
	// gen identifica a geracao deste client.
	// Sinais de geracoes antigas (goroutines mortas pelo Close do client anterior)
	// sao filtrados no loop principal usando a comparacao de gen.
	startReader := func(client *gemini.Client, gen int64) {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				resp, err := client.ReadResponse()
				if err != nil {
					if isGeminiTimeout(err) {
						sigChan <- browserSignal{kind: bsigReconnect, gen: gen, err: err}
					} else {
						sigChan <- browserSignal{kind: bsigFatal, gen: gen, err: err}
					}
					return
				}

				if _, ok := resp["setupComplete"]; ok {
					continue
				}

				// Tool calls do Gemini Function Calling
				if toolCall, ok := resp["toolCall"].(map[string]interface{}); ok {
					if fcList, ok := toolCall["functionCalls"].([]interface{}); ok {
						for _, f := range fcList {
							fc, ok := f.(map[string]interface{})
							if !ok {
								continue
							}
							name, _ := fc["name"].(string)
							args, _ := fc["args"].(map[string]interface{})
							if name == "" {
								continue
							}
							log.Info().Str("session", sessionID).Str("tool", name).Msg("[BROWSER] Tool call recebido do Gemini")

							writeMu.Lock()
							conn.WriteJSON(browserMessage{Type: "tool_event", Tool: name, Status: "executing"})
							writeMu.Unlock()

							go func(n string, a map[string]interface{}) {
								defer func() {
									if r := recover(); r != nil {
										log.Error().Str("tool", n).Interface("panic", r).Msg("[BROWSER] Tool panic")
										geminiMu.RLock()
										c := geminiRef
										geminiMu.RUnlock()
										if c != nil {
											c.SendToolResponse(n, map[string]interface{}{"error": "Internal error"})
										}
										writeMu.Lock()
										conn.WriteJSON(browserMessage{Type: "tool_event", Tool: n, Status: "error", Text: "Internal error"})
										writeMu.Unlock()
									}
								}()

								result, execErr := s.toolsHandler.ExecuteTool(n, a, 0)
								if execErr != nil {
									log.Warn().Err(execErr).Str("tool", n).Msg("[BROWSER] Tool execution failed")
									result = map[string]interface{}{"error": execErr.Error()}
								}

								geminiMu.RLock()
								c := geminiRef
								geminiMu.RUnlock()
								if c != nil {
									c.SendToolResponse(n, result)
								}

								status := "success"
								if execErr != nil {
									status = "error"
								}
								writeMu.Lock()
								conn.WriteJSON(browserMessage{Type: "tool_event", Tool: n, ToolData: result, Status: status})
								writeMu.Unlock()

								log.Info().Str("tool", n).Str("status", status).Msg("[BROWSER] Tool call concluido")
							}(name, args)
						}
					}
					continue
				}

				serverContent, ok := resp["serverContent"].(map[string]interface{})
				if !ok {
					continue
				}

				if interrupted, ok := serverContent["interrupted"].(bool); ok && interrupted {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "interrupted"})
					writeMu.Unlock()
					responseAccumMu.Lock()
					responseAccum.Reset()
					responseAccumMu.Unlock()
					continue
				}

				if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "turn_complete"})
					writeMu.Unlock()
					responseAccumMu.Lock()
					if responseAccum.Len() > 0 {
						turn := responseAccum.String()
						if s.evaMemory != nil {
							go func(t string) {
								storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer storeCancel()
								s.evaMemory.StoreTurn(storeCtx, sessionID, "assistant", t)
							}(turn)
						}
						// FASE 1: Save to vector memory with embeddings (async, non-blocking)
						if s.brainService != nil {
							go func(t string) {
								memCtx := brain.MemoryContext{
									Emotion:    "neutral",
									Urgency:    "low",
									Importance: 0.3,
								}
								if err := s.brainService.SaveEpisodicMemoryWithContext(0, "assistant", t, time.Now(), false, memCtx); err != nil {
									log.Warn().Err(err).Msg("[BRAIN] Falha ao salvar resposta EVA em memória vetorial")
								}
							}(turn)
						}
						// Acumular transcript para ProcessSessionEnd (evolucao de personalidade)
						transcriptMu.Lock()
						transcriptAccum.WriteString("EVA: " + turn + "\n")
						evaResponses = append(evaResponses, turn)
						transcriptMu.Unlock()
					}
					responseAccum.Reset()
					responseAccumMu.Unlock()
					continue
				}

				// inputAudioTranscription — envia transcricao do usuario ao browser + salva memoria
				if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := inputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Data: "user", Text: text})
						writeMu.Unlock()
						if s.evaMemory != nil {
							go func(t string) {
								storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer storeCancel()
								s.evaMemory.StoreTurn(storeCtx, sessionID, "user", t)
							}(text)
						}
						// FASE 1: Save user input to vector memory with embeddings
						if s.brainService != nil {
							go func(t string) {
								memCtx := brain.MemoryContext{
									Emotion:    "neutral",
									Urgency:    "low",
									Importance: 0.5,
								}
								if err := s.brainService.SaveEpisodicMemoryWithContext(0, "user", t, time.Now(), false, memCtx); err != nil {
									log.Warn().Err(err).Msg("[BRAIN] Falha ao salvar input do utilizador em memória vetorial")
								}
							}(text)
						}
						// Acumular transcript do usuario
						transcriptMu.Lock()
						transcriptAccum.WriteString("Usuario: " + text + "\n")
						transcriptMu.Unlock()
					}
				}

				// outputAudioTranscription — envia legenda (subtitle) ao browser + acumula para memoria
				if outputTrans, ok := serverContent["outputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := outputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text})
						writeMu.Unlock()
						responseAccumMu.Lock()
						responseAccum.WriteString(text)
						responseAccumMu.Unlock()
					}
				}

				modelTurn, ok := serverContent["modelTurn"].(map[string]interface{})
				if !ok {
					continue
				}

				parts, ok := modelTurn["parts"].([]interface{})
				if !ok {
					continue
				}

				for _, p := range parts {
					part, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					// AUDIO ONLY: inlineData (audio PCM) is the primary output.
					// Text parts in modelTurn.parts during native-audio mode are
					// tool-call artifacts ([[TOOL:...]]) or model fallback text —
					// NOT the real transcription. The actual EVA transcription comes
					// from outputAudioTranscription (handled above, line ~535).
					// Sending text here was causing voice "cuts" because the model
					// alternates between generating audio and text, creating gaps.
					if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
						if audioB64, ok := inlineData["data"].(string); ok {
							writeMu.Lock()
							conn.WriteJSON(browserMessage{Type: "audio", Data: audioB64})
							writeMu.Unlock()
						}
					} else if text, ok := part["text"].(string); ok && text != "" {
						// Log para debug — NAO enviar ao browser como subtitle.
						// A transcricao real vem de outputAudioTranscription.
						log.Debug().Str("session", sessionID).Str("text", text).Msg("[BROWSER] modelTurn text ignorado (audio-only mode)")
					}
				}
			}
		}()
	}

	startReader(initialClient, 1)

	// --- Speaker Recognition ---
	if s.speakerSvc != nil {
		s.speakerSvc.SetCallback(sessionID, func(sid string, msg speaker.SpeakerMessage) {
			writeMu.Lock()
			conn.WriteJSON(msg)
			writeMu.Unlock()
		})
		defer s.speakerSvc.RemoveSession(sessionID)
	}

	// --- Goroutine: Browser -> Gemini ---
	// Usa gen=0 no sinal para que o loop principal sempre o processe (e nunca o filtre).
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				sigChan <- browserSignal{kind: bsigFatal, gen: 0, err: err}
				return
			}

			// Dropa mensagens enquanto reconexao ao Gemini esta em progresso
			if reconnecting.Load() {
				continue
			}

			var msg browserMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			geminiMu.RLock()
			client := geminiRef
			geminiMu.RUnlock()

			if client == nil {
				continue
			}

			switch msg.Type {
			case "audio":
				pcmData, err := base64.StdEncoding.DecodeString(msg.Data)
				if err == nil {
					client.SendAudio(pcmData)
					if s.speakerSvc != nil {
						go s.speakerSvc.ProcessAudioChunk(sessionID, clientCPF, pcmData)
					}
				}
			case "video":
				jpegData, err := base64.StdEncoding.DecodeString(msg.Data)
				if err == nil {
					client.SendImage(jpegData)
				}
			case "text":
				if msg.Text != "" {
					if s.evaMemory != nil {
						go func(text string) {
							storeCtx, storeCancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer storeCancel()
							s.evaMemory.StoreTurn(storeCtx, sessionID, "user", text)
						}(msg.Text)
					}
					// NAO chamar SendText — gemini native-audio rejeita client_content
				// com close 1008 "policy violation". Texto salvo em memoria apenas.
				log.Info().Str("session", sessionID).Str("text", msg.Text).Msg("[BROWSER] Texto recebido (nao enviado ao Gemini native-audio)")
				}
			case "config":
				log.Info().Str("session", sessionID).Msg("Browser sent config update")
			}
		}
	}()

	// --- Loop principal: processa sinais e reconecta quando necessario ---
	reconnectCount := 0
	var finalErr error

	for {
		sig := <-sigChan

		// Filtra sinais de geracoes antigas (goroutines mortas pelo Close do client anterior).
		// gen=0 e reservado para o writer goroutine e nunca filtrado.
		if sig.gen != 0 && sig.gen != atomic.LoadInt64(&currentGen) {
			continue
		}

		if sig.kind == bsigFatal {
			finalErr = sig.err
			break
		}

		// bsigReconnect: Gemini expirou — reconectar sem fechar o WebSocket do browser
		reconnectCount++
		if reconnectCount > maxReconnects {
			log.Error().
				Str("session", sessionID).
				Int("attempts", reconnectCount).
				Msg("[BROWSER] Limite de reconexoes atingido — encerrando sessao")
			writeMu.Lock()
			conn.WriteJSON(browserMessage{Type: "status", Text: "error: max reconnects exceeded"})
			writeMu.Unlock()
			break
		}

		log.Warn().
			Str("session", sessionID).
			Int("attempt", reconnectCount).
			Err(sig.err).
			Msg("[BROWSER] Gemini timeout — reconectando...")

		reconnecting.Store(true)
		writeMu.Lock()
		conn.WriteJSON(browserMessage{Type: "status", Text: "reconnecting"})
		writeMu.Unlock()

		// Fecha client antigo (faz a goroutine reader antiga retornar)
		geminiMu.Lock()
		old := geminiRef
		geminiRef = nil
		geminiMu.Unlock()
		if old != nil {
			old.Close()
		}

		// Backoff antes de reconectar (evita hammering na API)
		time.Sleep(1500 * time.Millisecond)

		newClient, err := setupGemini()
		if err != nil {
			log.Error().Err(err).Str("session", sessionID).Msg("[BROWSER] Falha ao reconectar ao Gemini")
			writeMu.Lock()
			conn.WriteJSON(browserMessage{Type: "status", Text: "error: reconnect failed"})
			writeMu.Unlock()
			break
		}

		// Incrementa geracao ANTES de atualizar geminiRef para que o loop principal
		// ignore eventuais sinais tardios da goroutine antiga
		newGen := atomic.AddInt64(&currentGen, 1)
		geminiMu.Lock()
		geminiRef = newClient
		geminiMu.Unlock()

		reconnecting.Store(false)
		startReader(newClient, newGen)

		writeMu.Lock()
		conn.WriteJSON(browserMessage{Type: "status", Text: "ready"})
		writeMu.Unlock()

		log.Info().
			Str("session", sessionID).
			Int("attempt", reconnectCount).
			Int64("gen", newGen).
			Msg("[BROWSER] Reconexao ao Gemini bem-sucedida")
	}

	// --- Finalizar sessao no NietzscheDB ---
	if s.evaMemory != nil {
		s.evaMemory.EndSession(ctx, sessionID)
		go s.evaMemory.DetectPatterns(context.Background())
	}

	// Processar fim de sessao no CoreMemoryEngine (reflexao + memorias pessoais + evolucao Big Five)
	if s.coreMemory != nil {
		transcriptMu.Lock()
		transcript := transcriptAccum.String()
		evaResponsesCopy := make([]string, len(evaResponses))
		copy(evaResponsesCopy, evaResponses)
		transcriptMu.Unlock()
		if transcript != "" {
			duration := time.Since(sessionStart).Minutes()
			go func() {
				data := evaSelf.SessionData{
					SessionID:       sessionID,
					Transcript:      transcript,
					DurationMinutes: duration,
					EVAResponses:    evaResponsesCopy,
					Timestamp:       sessionStart,
				}
				bgCtx := context.Background()
				if err := s.coreMemory.ProcessSessionEnd(bgCtx, data); err != nil {
					log.Warn().Err(err).Str("session", sessionID).Msg("[CoreMemory] Falha ao processar fim de sessao de voz")
				} else {
					log.Info().Str("session", sessionID).Msg("[CoreMemory] Sessao de voz processada — memorias pessoais atualizadas")
				}

				// 7.12.1 Simbiose AGI: Feed energy based on situation to trigger reflexes
				if s.situationMod != nil && s.energyFeeder != nil {
					sit, _ := s.situationMod.Infer(bgCtx, clientCPF, transcript, nil)
					if err := s.energyFeeder.FeedReflexes(bgCtx, clientCPF, sit, "default"); err != nil {
						log.Warn().Err(err).Msg("[EnergyFeeder] Falha ao alimentar reflexos (voz)")
					}
				}
			}()
		}
	}

	log.Info().
		Str("session", sessionID).
		Int("reconnects", reconnectCount).
		Err(finalErr).
		Msg("Browser voice session ended")
}
