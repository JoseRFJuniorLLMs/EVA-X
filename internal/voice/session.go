package voice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"eva-mind/internal/config"
	"eva-mind/internal/database"
	"eva-mind/internal/gemini"
	"eva-mind/pkg/models"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type GeminiSession struct {
	conn    *websocket.Conn
	callCtx *models.CallContext
	db      *database.DB
	cfg     *config.Config
	logger  zerolog.Logger
}

func NewGeminiSession(conn *websocket.Conn, callCtx *models.CallContext, db *database.DB, cfg *config.Config, logger zerolog.Logger) *GeminiSession {
	return &GeminiSession{
		conn:    conn,
		callCtx: callCtx,
		db:      db,
		cfg:     cfg,
		logger:  logger,
	}
}

func (s *GeminiSession) Start(ctx context.Context) error {
	s.logger.Info().Msg("Gemini session started")

	// 1. Initialize Gemini client
	gClient, err := gemini.NewClient(ctx, s.cfg)
	if err != nil {
		return fmt.Errorf("failed to start Gemini client: %w", err)
	}
	defer gClient.Close()

	// 2. Send Setup
	// Use elderly context for system prompt
	systemPrompt := fmt.Sprintf("Você é a EVA, uma assistente virtual carinhosa para idosos. Você está ligando para %s para lembrá-lo(a) de seus remédios: %s. O nível cognitivo dele(a) é %s. Seja paciente e fale de forma clara.",
		s.callCtx.IdosoNome, s.callCtx.Medicamento, s.callCtx.NivelCognitivo)

	if err := gClient.SendSetup(systemPrompt); err != nil {
		return fmt.Errorf("failed to send setup to Gemini: %w", err)
	}

	// 3. Start bidirectional loop
	errors := make(chan error, 2)

	// Twilio -> Gemini
	go func() {
		for {
			_, message, err := s.conn.ReadMessage()
			if err != nil {
				errors <- err
				return
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}

			if msg["event"] == "media" {
				media := msg["media"].(map[string]interface{})
				payload := media["payload"].(string)
				audioData, _ := base64.StdEncoding.DecodeString(payload)

				// Here we should ideally convert PCMU to PCM, but Gemini might handle it or we deal with it later.
				// For now, let's send it.
				if err := gClient.SendAudio(audioData); err != nil {
					errors <- err
					return
				}
			} else if msg["event"] == "stop" {
				errors <- nil
				return
			}
		}
	}()

	// Gemini -> Twilio
	go func() {
		for {
			resp, err := gClient.ReadResponse()
			if err != nil {
				errors <- err
				return
			}

			if serverContent, ok := resp["server_content"].(map[string]interface{}); ok {
				if modelTurn, ok := serverContent["model_turn"].(map[string]interface{}); ok {
					if parts, ok := modelTurn["parts"].([]interface{}); ok {
						for _, p := range parts {
							part := p.(map[string]interface{})
							if inlineData, ok := part["inline_data"].(map[string]interface{}); ok {
								audioBase64 := inlineData["data"].(string)

								// Send back to Twilio
								responseMsg := map[string]interface{}{
									"event": "media",
									"media": map[string]interface{}{
										"payload": audioBase64,
									},
								}
								if err := s.conn.WriteJSON(responseMsg); err != nil {
									errors <- err
									return
								}
							}
						}
					}
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errors:
		return err
	}
}
