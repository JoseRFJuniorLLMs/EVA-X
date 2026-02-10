package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/tools"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AudioCallback é chamado quando áudio PCM é recebido do Gemini
type AudioCallback func(audioBytes []byte)

// ToolCallCallback é chamado quando uma ferramenta precisa ser executada
type ToolCallCallback func(name string, args map[string]interface{}) map[string]interface{}

// TranscriptCallback é chamado quando há transcrição de áudio (Input ou Output)
type TranscriptCallback func(role, text string)

// Client gerencia a conexão WebSocket com Gemini Live API
type Client struct {
	conn         *websocket.Conn
	mu           sync.Mutex
	cfg          *config.Config
	onAudio      AudioCallback
	onToolCall   ToolCallCallback
	onTranscript TranscriptCallback
}

// NewClient cria um novo cliente Gemini usando WebSocket direto
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	// ✅ VALIDAÇÃO: Verificar se API key existe
	if cfg.GoogleAPIKey == "" {
		return nil, fmt.Errorf("ERRO: GOOGLE_API_KEY está vazia")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// ✅ FIX DEFINITIVO: Usar v1beta (não v1alpha!) conforme documentação oficial
	// https://ai.google.dev/api/live
	wsURL := fmt.Sprintf(
		"wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent?key=%s",
		cfg.GoogleAPIKey,
	)

	// 🔍 DEBUG
	maskedKey := "N/A"
	if len(cfg.GoogleAPIKey) > 8 {
		maskedKey = cfg.GoogleAPIKey[:4] + "..." + cfg.GoogleAPIKey[len(cfg.GoogleAPIKey)-4:]
	}

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🔌 Conectando ao Gemini WebSocket")
	log.Printf("🔑 API Key: %s (len=%d)", maskedKey, len(cfg.GoogleAPIKey))
	log.Printf("🤖 Model: %s", cfg.ModelID)
	log.Printf("📡 API Version: v1beta")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if resp != nil {
			log.Printf("❌ Falha na conexão WebSocket")
			log.Printf("   Status: %d - %s", resp.StatusCode, resp.Status)
			if resp.Body != nil {
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				if n > 0 {
					log.Printf("   Body: %s", string(body[:n]))
				}
			}
		}
		return nil, fmt.Errorf("erro ao conectar: %w", err)
	}

	log.Printf("✅ WebSocket conectado!")
	return &Client{conn: conn, cfg: cfg}, nil
}

// SetCallbacks configura callbacks
func (c *Client) SetCallbacks(onAudio AudioCallback, onToolCall ToolCallCallback, onTranscript TranscriptCallback) {
	c.onAudio = onAudio
	c.onToolCall = onToolCall
	c.onTranscript = onTranscript
}

// SendSetup envia configuração inicial
func (c *Client) SendSetup(instructions string, voiceSettings map[string]interface{}, memories []string, initialAudio string, toolsDef []tools.FunctionDeclaration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if voiceSettings == nil {
		voiceSettings = map[string]interface{}{"voiceName": "Aoede"}
	}

	setup := map[string]interface{}{
		"setup": map[string]interface{}{
			"model": fmt.Sprintf("models/%s", c.cfg.ModelID),
			"generation_config": map[string]interface{}{
				"response_modalities": []string{"AUDIO"},
				"speech_config": map[string]interface{}{
					"voice_config": map[string]interface{}{
						"prebuilt_voice_config": map[string]interface{}{
							"voice_name": voiceSettings["voiceName"],
						},
					},
				},

				"temperature": 0.6,
			},
			"system_instruction": map[string]interface{}{
				"parts": []interface{}{
					map[string]interface{}{
						"text": instructions,
					},
				},
			},
		},
	}

	// ⚠️ CRITICAL ARCHITECTURE FIX:
	// O modelo 'gemini-2.5-flash-native-audio-preview' NÃO suporta Tools via WebSocket.
	// Ele é estritamente para Audio Streaming (Input/Output).
	// Tools devem ser processadas por um client separado (REST/gRPC) usando outro modelo.
	// Portanto, enviamos NIL para tools aqui, igual ao EVA-Mind original.

	/*
		// Tools Logic - DISABLED FOR AUDIO WEBSOCKET
		var toolsPayload []interface{}
		if len(toolsDef) > 0 {
			toolsList := []interface{}{}
			for _, t := range toolsDef {
				toolsList = append(toolsList, t)
			}
			toolsPayload = append(toolsPayload, map[string]interface{}{
				"functionDeclarations": toolsList,
			})
			log.Printf("⚠️ [SETUP] Tools ignoradas para Audio WebSocket (Architectural Fix)")
		}

		if len(toolsPayload) > 0 {
			setup["setup"].(map[string]interface{})["tools"] = toolsPayload
		}
	*/

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🔧 CONFIGURANDO GEMINI")
	log.Printf("🎙️ Input: 16kHz PCM16 Mono")
	log.Printf("🔊 Output: 24kHz PCM16 Mono")
	if len(memories) > 0 {
		log.Printf("🧠 Memórias: %d", len(memories))
	}

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return c.conn.WriteJSON(setup)
}

// StartSession é alias depreciado
func (c *Client) StartSession(instructions string, tools []interface{}, memories []string, voiceName string) error {
	return c.SendSetup(instructions, map[string]interface{}{"voiceName": voiceName}, memories, "", nil)
}

// SendAudio envia áudio PCM
func (c *Client) SendAudio(audioData []byte) error {
	encoded := base64.StdEncoding.EncodeToString(audioData)

	msg := map[string]interface{}{
		"realtime_input": map[string]interface{}{
			"media_chunks": []map[string]string{
				{
					"mime_type": "audio/pcm;rate=16000",
					"data":      encoded,
				},
			},
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// SendText envia mensagem de texto
func (c *Client) SendText(text string) error {
	msg := map[string]interface{}{
		"client_content": map[string]interface{}{
			"turn_complete": true,
			"turns": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]interface{}{
						{
							"text": text,
						},
					},
				},
			},
		},
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// SendImage envia imagem JPEG
func (c *Client) SendImage(imageData []byte) error {
	encoded := base64.StdEncoding.EncodeToString(imageData)

	msg := map[string]interface{}{
		"realtime_input": map[string]interface{}{
			"media_chunks": []map[string]string{
				{
					"mime_type": "image/jpeg",
					"data":      encoded,
				},
			},
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// SendMessage envia mensagem genérica
func (c *Client) SendMessage(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// ReadResponse lê resposta
func (c *Client) ReadResponse() (map[string]interface{}, error) {
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	// PERFORMANCE: Logs de debug removidos (causavam overhead de I/O)
	// Para debug, descomentar linha abaixo:
	// log.Printf("🔍 [GEMINI] Response: %d bytes", len(message))

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return nil, fmt.Errorf("unmarshal error: %v", err)
	}
	return response, nil
}

// HandleResponses processa loop de mensagens
func (c *Client) HandleResponses(ctx context.Context) error {
	log.Printf("👂 HandleResponses: loop iniciado")

	// PERFORMANCE FIX: Constante para ReadDeadline
	const readTimeout = 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 HandleResponses: contexto cancelado")
			return ctx.Err()
		default:
			// PERFORMANCE FIX: Definir deadline antes de cada leitura
			// Evita que o WebSocket fique bloqueado indefinidamente
			if err := c.conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				log.Printf("⚠️ Erro ao definir ReadDeadline: %v", err)
			}

			resp, err := c.ReadResponse()
			if err != nil {
				select {
				case <-ctx.Done():
					log.Printf("🛑 HandleResponses: Contexto finalizado (%v)", err)
				default:
					log.Printf("❌ Erro ao ler resposta Gemini: %v", err)
				}
				return err
			}

			// Verificar setupComplete
			if setupComplete, ok := resp["setupComplete"].(bool); ok && setupComplete {
				log.Printf("✅ Gemini Setup Complete - Pronto!")
				continue
			}

			// Erros
			if errMsg, ok := resp["error"]; ok {
				log.Printf("❌ Gemini Error: %v", errMsg)
				continue
			}

			// Áudio e transcrição
			if serverContent, ok := resp["serverContent"].(map[string]interface{}); ok {

				// Transcrição do usuário
				if inputTrans, ok := serverContent["inputAudioTranscription"].(map[string]interface{}); ok {
					if userText, ok := inputTrans["text"].(string); ok && userText != "" {
						if c.onTranscript != nil {
							c.onTranscript("user", userText)
						}
					}
				}

				// Transcrição da IA
				if audioTrans, ok := serverContent["audioTranscription"].(map[string]interface{}); ok {
					if aiText, ok := audioTrans["text"].(string); ok && aiText != "" {
						if c.onTranscript != nil {
							c.onTranscript("assistant", aiText)
						}
					}
				}

				if modelTurn, ok := serverContent["modelTurn"].(map[string]interface{}); ok {
					if parts, ok := modelTurn["parts"].([]interface{}); ok {
						for _, p := range parts {
							part, ok := p.(map[string]interface{})
							if !ok {
								continue
							}

							// Áudio
							if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
								if audioB64, ok := inlineData["data"].(string); ok {
									audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
									if err != nil {
										log.Printf("❌ Erro decode base64: %v", err)
										continue
									}
									if c.onAudio != nil {
										c.onAudio(audioBytes)
									}
								}
							}
						}
					}
				}
			}

			// Tool calls
			if toolCall, ok := resp["toolCall"].(map[string]interface{}); ok {
				log.Printf("🔧 Tool call detectado")
				c.handleToolCalls(toolCall)
			}
		}
	}
}

func (c *Client) handleToolCalls(toolCall map[string]interface{}) {
	if fcList, ok := toolCall["functionCalls"].([]interface{}); ok {
		for _, f := range fcList {
			fc := f.(map[string]interface{})
			name := fc["name"].(string)
			args := fc["args"].(map[string]interface{})

			if c.onToolCall != nil {
				go func(n string, a map[string]interface{}) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("🚨 PANIC Tool %s: %v", n, r)
							c.SendToolResponse(n, map[string]interface{}{"error": "Internal error"})
						}
					}()

					result := c.onToolCall(n, a)
					c.SendToolResponse(n, result)
				}(name, args)
			}
		}
	}
}

// SendToolResponse envia resultado de ferramenta
func (c *Client) SendToolResponse(name string, result map[string]interface{}) error {
	msg := map[string]interface{}{
		"tool_response": map[string]interface{}{
			"function_responses": []map[string]interface{}{
				{
					"name":     name,
					"response": result,
				},
			},
		},
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(msg)
}

// Close fecha conexão
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
