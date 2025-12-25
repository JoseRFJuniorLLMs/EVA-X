package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	"eva-mind/internal/config"

	"github.com/gorilla/websocket"
)

type Client struct {
	cfg  *config.Config
	conn *websocket.Conn
}

func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	geminiURL := url.URL{
		Scheme:   "wss",
		Host:     "generativelanguage.googleapis.com",
		Path:     "/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContent",
		RawQuery: "key=" + cfg.GoogleAPIKey,
	}

	conn, _, err := websocket.DefaultDialer.Dial(geminiURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Gemini: %w", err)
	}

	return &Client{
		cfg:  cfg,
		conn: conn,
	}, nil
}

// NewLiveClient cria um cliente Gemini configurado de forma livre e natural
func NewLiveClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// ✅ Prompt mínimo - deixa o modelo TOTALMENTE livre
	minimalPrompt := `Você é Eva, uma assistente de voz natural e conversacional.
Responda qualquer pergunta de forma útil, criativa e amigável.`

	err = client.SendSetup(minimalPrompt, nil) // ✅ SEM TOOLS, SEM CONTEXTO DO BANCO
	if err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendAudio envia áudio PCM 16kHz para o Gemini
func (c *Client) SendAudio(data []byte) error {
	encodedData := base64.StdEncoding.EncodeToString(data)

	msg := map[string]interface{}{
		"realtime_input": map[string]interface{}{
			"media_chunks": []map[string]interface{}{
				{
					"data":      encodedData,
					"mime_type": "audio/pcm;rate=16000", // ✅ CRÍTICO: Especificar 16kHz!
				},
			},
		},
	}
	return c.conn.WriteJSON(msg)
}

func (c *Client) ReadResponse() (map[string]interface{}, error) {
	var resp map[string]interface{}
	err := c.conn.ReadJSON(&resp)
	return resp, err
}

func (c *Client) SendSetup(context string, tools []map[string]interface{}) error {
	setup := map[string]interface{}{
		"setup": map[string]interface{}{
			"model": "models/" + c.cfg.ModelID,
			"generationConfig": map[string]interface{}{
				"responseModalities": []string{"AUDIO"},
				"speechConfig": map[string]interface{}{
					"voiceConfig": map[string]interface{}{
						"prebuiltVoiceConfig": map[string]interface{}{
							"voiceName": "Aoede",
						},
					},
				},
			},
		},
	}

	// ✅ Só adiciona systemInstruction se houver contexto
	if context != "" {
		setup["setup"].(map[string]interface{})["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": context},
			},
		}
	}

	// ✅ Só adiciona tools se houver
	if len(tools) > 0 {
		setup["setup"].(map[string]interface{})["tools"] = tools
	}

	return c.conn.WriteJSON(setup)
}

// SendText envia uma mensagem de texto (user turn) para o Gemini
func (c *Client) SendText(text string) error {
	msg := map[string]interface{}{
		"client_content": map[string]interface{}{
			"turns": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]interface{}{
						{"text": text},
					},
				},
			},
			"turn_complete": true,
		},
	}
	return c.conn.WriteJSON(msg)
}

// WriteJSON envia uma mensagem arbitrária via WebSocket
func (c *Client) WriteJSON(v interface{}) error {
	return c.conn.WriteJSON(v)
}

// Ping verifica se a conexão ainda está ativa
func (c *Client) Ping() error {
	if c.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}
