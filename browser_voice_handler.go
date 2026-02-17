// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	gemini "eva-mind/internal/cortex/gemini"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// ============================================================================
// geminiApp — Voz e Video via WebSocket para App Mobile (EVA-Mobile)
// ============================================================================
// Consumer:  geminiApp
// Rota:      /ws/browser
// Client:    internal/gemini (v1alpha, simples)
// Frontend:  App mobile EVA-Mobile
// Protocolo: WebSocket — audio PCM (16kHz in, 24kHz out) + video JPEG + texto
// Memoria:   Meta-cognitiva via Neo4j (carrega no inicio, salva transcricoes)
// CRITICO:   Protocolo WebSocket NAO pode mudar — app mobile depende
// Ver:       GEMINI_ARCHITECTURE.md para documentacao completa

// browserWSUpgrader permite conexoes de browsers (CORS flexivel)
var browserWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Browser clients de qualquer origem
	},
}

// browserMessage formato de mensagem browser <-> server
type browserMessage struct {
	Type string `json:"type"`           // "audio", "text", "config", "status"
	Data string `json:"data,omitempty"` // base64 PCM para audio
	Text string `json:"text,omitempty"` // texto para subtitles/chat
}

// handleBrowserVoice lida com WebSocket de voz vindo do browser
// Protocolo simples:
//   Browser envia: {"type":"audio","data":"base64_pcm_16khz"}
//   Browser envia: {"type":"config","text":"system_prompt"} (opcional, no inicio)
//   Server envia:  {"type":"audio","data":"base64_pcm_24khz"}
//   Server envia:  {"type":"text","text":"transcricao"}
//   Server envia:  {"type":"status","text":"ready|speaking|listening"}
func (s *SignalingServer) handleBrowserVoice(w http.ResponseWriter, r *http.Request) {
	conn, err := browserWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Browser WS upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Cria sessao Gemini usando cortex/gemini (v1beta, producao)
	geminiClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao criar cortex/gemini client para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: " + err.Error()})
		return
	}
	defer geminiClient.Close()

	sessionID := "browser-" + time.Now().Format("20060102150405")

	// Espera primeira mensagem do cliente (deve ser tipo "config" com contexto + CPF)
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

	// Se cliente nao enviou contexto, usa generico minimo
	if clientContext == "" {
		clientContext = "Voce e a EVA, assistente virtual inteligente. Responda em portugues de forma clara e profissional."
	}

	log.Info().Str("session", sessionID).Str("cpf", clientCPF).Bool("hasContext", configMsg.Text != "").Msg("[BROWSER] Config recebida do cliente")

	// Carregar dados da pessoa pelo CPF (PostgreSQL)
	if clientCPF != "" && s.db != nil {
		idoso, err := s.db.GetIdosoByCPF(clientCPF)
		if err != nil {
			log.Warn().Err(err).Str("cpf", clientCPF).Msg("[BROWSER] Pessoa nao encontrada")
		} else {
			fullIdoso, err := s.db.GetIdoso(idoso.ID)
			if err == nil && fullIdoso != nil {
				clientContext += fmt.Sprintf("\n\nVoce esta conversando com %s (CPF: %s, nascido em %s). Use o nome dele/dela na conversa.",
					fullIdoso.Nome, clientCPF, fullIdoso.DataNascimento.Format("02/01/2006"))
				log.Info().Str("session", sessionID).Str("nome", fullIdoso.Nome).Int64("id", fullIdoso.ID).Msg("[BROWSER] Pessoa carregada")
			}

			// Buscar agendamentos/medicamentos da pessoa via query direta
			rows, err := s.db.Conn.Query(`
				SELECT tipo, dados_tarefa, status, data_hora_agendada
				FROM agendamentos
				WHERE idoso_id = $1 AND status IN ('agendado','ativo','pendente')
				ORDER BY data_hora_agendada ASC LIMIT 20`, idoso.ID)
			if err == nil {
				defer rows.Close()
				var medsInfo strings.Builder
				count := 0
				for rows.Next() {
					var tipo, dados, status string
					var dataHora time.Time
					if err := rows.Scan(&tipo, &dados, &status, &dataHora); err == nil {
						if count == 0 {
							medsInfo.WriteString("\n\n[MEDICAMENTOS E AGENDAMENTOS DO PACIENTE]")
						}
						medsInfo.WriteString(fmt.Sprintf("\n- %s: %s (Status: %s, Hora: %s)",
							tipo, dados, status, dataHora.Format("02/01 15:04")))
						count++
					}
				}
				if count > 0 {
					clientContext += medsInfo.String()
					log.Info().Str("session", sessionID).Int("count", count).Msg("[BROWSER] Agendamentos carregados")
				}
			}
		}
	}

	// Carregar memoria meta-cognitiva do Neo4j
	var memories []string
	if s.evaMemory != nil {
		if err := s.evaMemory.StartSession(ctx, sessionID); err != nil {
			log.Warn().Err(err).Msg("[BROWSER] Falha ao registrar sessao no Neo4j")
		}
		metaCognition, err := s.evaMemory.LoadMetaCognition(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("[BROWSER] Falha ao carregar memoria meta-cognitiva")
		} else if metaCognition != "" {
			memories = []string{metaCognition}
			log.Info().Str("session", sessionID).Msg("[BROWSER] Memoria meta-cognitiva injetada")
		}
	}

	// Setup com cortex/gemini — contexto vem do frontend, memoria do Neo4j
	err = geminiClient.SendSetup(
		clientContext,
		map[string]interface{}{
			"voiceName":    "Aoede",
			"languageCode": "pt-BR",
		},
		memories,
		"",
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao configurar Gemini para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: setup failed"})
		return
	}

	log.Info().Str("session", sessionID).Int("memories", len(memories)).Msg("Browser voice session started (cortex/gemini)")

	// Notifica browser que esta pronto
	conn.WriteJSON(browserMessage{Type: "status", Text: "ready"})

	var writeMu sync.Mutex
	errChan := make(chan error, 2)

	// Buffer para acumular transcricao da resposta da EVA (para salvar no Neo4j)
	var responseAccum strings.Builder

	// Goroutine: Gemini -> Browser (audio responses)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := geminiClient.ReadResponse()
				if err != nil {
					errChan <- err
					return
				}

				// setupComplete
				if _, ok := resp["setupComplete"]; ok {
					continue
				}

				serverContent, ok := resp["serverContent"].(map[string]interface{})
				if !ok {
					continue
				}

				// Interrupcao
				if interrupted, ok := serverContent["interrupted"].(bool); ok && interrupted {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "interrupted"})
					writeMu.Unlock()
					responseAccum.Reset()
					continue
				}

				// Turn complete
				if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "turn_complete"})
					writeMu.Unlock()

					// Salvar resposta acumulada no Neo4j
					if s.evaMemory != nil && responseAccum.Len() > 0 {
						go s.evaMemory.StoreTurn(ctx, sessionID, "assistant", responseAccum.String())
					}
					responseAccum.Reset()
					continue
				}

				// Transcricao do input do usuario
				if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := inputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text, Data: "user"})
						writeMu.Unlock()
						// Salvar transcricao do usuario no Neo4j
						if s.evaMemory != nil {
							go s.evaMemory.StoreTurn(ctx, sessionID, "user", text)
						}
					}
				}

				// Transcricao do output do modelo (audio -> texto)
				if outputTrans, ok := serverContent["outputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := outputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text})
						writeMu.Unlock()
						responseAccum.WriteString(text)
					}
				}

				// Audio e texto do modelo
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

					// Texto (subtitles)
					if text, ok := part["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text})
						writeMu.Unlock()
					}

					// Audio inline
					if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
						if audioB64, ok := inlineData["data"].(string); ok {
							writeMu.Lock()
							conn.WriteJSON(browserMessage{Type: "audio", Data: audioB64})
							writeMu.Unlock()
						}
					}
				}
			}
		}
	}()

	// Goroutine: Browser -> Gemini (audio input)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msgBytes, err := conn.ReadMessage()
				if err != nil {
					errChan <- err
					return
				}

				var msg browserMessage
				if err := json.Unmarshal(msgBytes, &msg); err != nil {
					continue
				}

				switch msg.Type {
				case "audio":
					// Decode base64 PCM do browser (16kHz, 16-bit)
					pcmData, err := base64.StdEncoding.DecodeString(msg.Data)
					if err != nil {
						continue
					}
					geminiClient.SendAudio(pcmData)

				case "video":
					// Frame JPEG da camera do browser (1 FPS)
					jpegData, err := base64.StdEncoding.DecodeString(msg.Data)
					if err != nil {
						continue
					}
					geminiClient.SendImage(jpegData)

				case "text":
					// Mensagem de texto direta
					if msg.Text != "" {
						if s.evaMemory != nil {
							go s.evaMemory.StoreTurn(ctx, sessionID, "user", msg.Text)
						}
						geminiClient.SendText(msg.Text)
					}

				case "config":
					// Permite reconfigurar system prompt mid-session
					log.Info().Str("session", sessionID).Msg("Browser sent config update")
				}
			}
		}
	}()

	// Espera erro de qualquer goroutine
	sessionErr := <-errChan

	// Finalizar sessao no Neo4j
	if s.evaMemory != nil {
		s.evaMemory.EndSession(ctx, sessionID)
		go s.evaMemory.DetectPatterns(context.Background())
	}

	log.Info().Str("session", sessionID).Err(sessionErr).Msg("Browser voice session ended")
}
