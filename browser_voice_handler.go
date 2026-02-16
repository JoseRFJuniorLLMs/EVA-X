package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"eva-mind/internal/gemini"
	"eva-mind/internal/voice"
	"net/http"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

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

	// Cria sessao Gemini para este browser client
	geminiClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao criar Gemini client para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: " + err.Error()})
		return
	}

	// System prompt padrao para malaria
	systemPrompt := `Voce e a EVA Assistente Malaria Angola, assistente por voz do Sistema de Gestao de Malaria.
Ajude profissionais de saude com diagnostico, tratamento e uso do sistema.
Responda em portugues, de forma breve e conversacional.
Voce esta em uma conversa por voz em tempo real - seja concisa (2-3 frases).

Especies de Plasmodium em Angola:
- P. falciparum (95%) - mais grave
- P. vivax (~2%), P. malariae (~2%), P. ovale (~1%)

Fluxo: Tecnico coleta amostra -> IA analisa imagem -> Medico revisa -> Tratamento prescrito.`

	// Configura sessao com voz
	err = geminiClient.SendSetup(systemPrompt, nil)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao configurar Gemini para browser")
		conn.WriteJSON(browserMessage{Type: "status", Text: "error: setup failed"})
		return
	}

	// Armazena sessao para limpeza
	sessionID := "browser-" + time.Now().Format("20060102150405")
	voice.StoreSession(sessionID, geminiClient)
	defer voice.RemoveSession(sessionID)

	log.Info().Str("session", sessionID).Msg("Browser voice session started")

	// Notifica browser que esta pronto
	conn.WriteJSON(browserMessage{Type: "status", Text: "ready"})

	var writeMu sync.Mutex
	errChan := make(chan error, 2)

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
					continue
				}

				// Turn complete
				if turnComplete, ok := serverContent["turnComplete"].(bool); ok && turnComplete {
					writeMu.Lock()
					conn.WriteJSON(browserMessage{Type: "status", Text: "turn_complete"})
					writeMu.Unlock()
					continue
				}

				// Transcricao do input do usuario
				if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
					if text, ok := inputTrans["text"].(string); ok && text != "" {
						writeMu.Lock()
						conn.WriteJSON(browserMessage{Type: "text", Text: text, Data: "user"})
						writeMu.Unlock()
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
	log.Info().Str("session", sessionID).Err(sessionErr).Msg("Browser voice session ended")
}
