// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	gemini "eva-mind/internal/cortex/gemini"
	"net/http"
	"strings"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// evaWSUpgrader permite conexoes do browser para /ws/eva
var evaWSUpgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// evaMessage formato de mensagem browser <-> EVA
type evaMessage struct {
	Type string `json:"type"`           // "text", "status"
	Data string `json:"data,omitempty"` // metadado opcional
	Text string `json:"text,omitempty"` // conteudo de texto
}

// ============================================================================
// geminiWeb — Chat de Texto via WebSocket para pagina /eva (Malaria-Angolar)
// ============================================================================
// Consumer:  geminiWeb
// Rota:      /ws/eva
// Client:    internal/cortex/gemini (v1beta, producao)
// Frontend:  Malaria-Angolar/frontend/src/pages/EvaPage.tsx
// Memoria:   Meta-cognitiva via Neo4j (internal/cortex/eva_memory)
// Protocolo:
//
//	Browser envia: {"type":"text","text":"pergunta do usuario"}
//	Server envia:  {"type":"text","text":"chunk da resposta"}  (streaming)
//	Server envia:  {"type":"status","text":"ready|turn_complete|interrupted|error"}
//
// Ver:       GEMINI_ARCHITECTURE.md para documentacao completa
func (s *SignalingServer) handleEvaChat(w http.ResponseWriter, r *http.Request) {
	conn, err := evaWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("EVA WS upgrade failed")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Cria sessao Gemini usando cortex/gemini (producao)
	geminiClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao criar cortex/gemini client para EVA")
		conn.WriteJSON(evaMessage{Type: "status", Text: "error: " + err.Error()})
		return
	}
	defer geminiClient.Close()

	sessionID := "eva-" + time.Now().Format("20060102150405")

	// Espera primeira mensagem do cliente (deve ser tipo "config" com CPF)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, configBytes, err := conn.ReadMessage()
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		log.Error().Err(err).Msg("[EVA] Timeout esperando config do cliente")
		conn.WriteJSON(evaMessage{Type: "status", Text: "error: config timeout"})
		return
	}

	var configMsg evaMessage
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

	log.Info().Str("session", sessionID).Str("cpf", clientCPF).Bool("hasContext", configMsg.Text != "").Msg("[EVA] Config recebida do cliente")

	// Carregar memoria meta-cognitiva do Neo4j
	var memories []string
	if s.evaMemory != nil {
		if err := s.evaMemory.StartSession(ctx, sessionID); err != nil {
			log.Warn().Err(err).Msg("[EVA] Falha ao registrar sessao no Neo4j")
		}

		metaCognition, err := s.evaMemory.LoadMetaCognition(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("[EVA] Falha ao carregar memoria meta-cognitiva")
		} else if metaCognition != "" {
			memories = []string{metaCognition}
			log.Info().Str("session", sessionID).Msg("[EVA] Memoria meta-cognitiva injetada")
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
		log.Error().Err(err).Msg("Erro ao configurar cortex/gemini para EVA")
		conn.WriteJSON(evaMessage{Type: "status", Text: "error: setup failed"})
		return
	}

	log.Info().Str("session", sessionID).Int("memories", len(memories)).Msg("EVA chat session started (cortex/gemini)")

	// Notifica browser que esta pronto
	conn.WriteJSON(evaMessage{Type: "status", Text: "ready"})

	var writeMu sync.Mutex
	errChan := make(chan error, 2)

	// Buffer para acumular resposta da EVA (para salvar no Neo4j)
	var responseAccum strings.Builder

	// Goroutine: Gemini -> Browser (text responses)
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

				if _, ok := resp["setupComplete"]; ok {
					continue
				}

				serverContent, ok := resp["serverContent"].(map[string]interface{})
				if !ok {
					continue
				}

				if interrupted, ok := serverContent["interrupted"].(bool); ok && interrupted {
					writeMu.Lock()
					conn.WriteJSON(evaMessage{Type: "status", Text: "interrupted"})
					writeMu.Unlock()
					responseAccum.Reset()
					continue
				}

				if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
					writeMu.Lock()
					conn.WriteJSON(evaMessage{Type: "status", Text: "turn_complete"})
					writeMu.Unlock()

					// Salvar resposta acumulada no Neo4j
					if s.evaMemory != nil && responseAccum.Len() > 0 {
						go s.evaMemory.StoreTurn(ctx, sessionID, "assistant", responseAccum.String())
					}
					responseAccum.Reset()
					continue
				}

				// Transcricao do output do modelo
				if outputTrans, ok := serverContent["outputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := outputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(evaMessage{Type: "text", Text: text})
						writeMu.Unlock()
						responseAccum.WriteString(text)
					}
				}

				// Texto direto do modelo
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
					if text, ok := part["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(evaMessage{Type: "text", Text: text})
						writeMu.Unlock()
						responseAccum.WriteString(text)
					}
				}
			}
		}
	}()

	// Goroutine: Browser -> Gemini (text input)
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

				var msg evaMessage
				if err := json.Unmarshal(msgBytes, &msg); err != nil {
					continue
				}

				if msg.Type == "text" && msg.Text != "" {
					// Salvar mensagem do usuario no Neo4j
					if s.evaMemory != nil {
						go s.evaMemory.StoreTurn(ctx, sessionID, "user", msg.Text)
					}
					geminiClient.SendText(msg.Text)
				}
			}
		}
	}()

	sessionErr := <-errChan

	// Finalizar sessao no Neo4j
	if s.evaMemory != nil {
		s.evaMemory.EndSession(ctx, sessionID)
		// Detectar padroes apos cada sessao
		go s.evaMemory.DetectPatterns(context.Background())
	}

	log.Info().Str("session", sessionID).Err(sessionErr).Msg("EVA chat session ended")
}
