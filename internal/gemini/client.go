package gemini

import (
	"context"
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

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendAudio envia áudio PCM 16kHz para o Gemini
func (c *Client) SendAudio(data []byte) error {
	msg := map[string]interface{}{
		"realtime_input": map[string]interface{}{
			"media_chunks": []map[string]interface{}{
				{
					"data":      data,
					"mime_type": "audio/pcm;rate=16000", // ✅ CORRIGIDO!
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
			"generation_config": map[string]interface{}{
				"response_modalities": []string{"AUDIO"},
			},
			"system_instruction": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": context},
				},
			},
			"tools": tools,
		},
	}
	return c.conn.WriteJSON(setup)
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
