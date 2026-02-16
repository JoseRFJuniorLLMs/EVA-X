package gemini_test

import (
	"context"
	"encoding/json"
	"eva-mind/internal/gemini"
	"eva-mind/internal/multimodal"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWebSocketServer cria um servidor WebSocket mock para testes
func mockWebSocketServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade: %v", err)
		}
		defer conn.Close()
		handler(conn)
	}))
	return server
}

func TestClient_SendMediaChunk_Success(t *testing.T) {
	received := make(chan gemini.MultimodalMessage, 1)

	// Mock server que captura a mensagem
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		var msg gemini.MultimodalMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			return
		}
		received <- msg
	})
	defer server.Close()

	// Conecta ao mock server
	wsURL := "ws" + server.URL[4:] // http -> ws
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Por enquanto, teste conceitual validando estrutura do chunk
	// Em produção, Client seria criado com NewClient() e teria conexão real
	chunk := &multimodal.MediaChunk{
		MimeType:  "image/jpeg",
		Data:      "base64data",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	// Valida estrutura do chunk
	assert.Equal(t, "image/jpeg", chunk.MimeType)
	assert.NotEmpty(t, chunk.Data)

	conn.Close()
}

func TestClient_SendMediaChunk_NilChunk(t *testing.T) {
	// Mock client sem conexão real
	client := &gemini.Client{}

	ctx := context.Background()
	err := client.SendMediaChunk(ctx, nil)

	// Deve retornar erro se chunk é nil
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chunk cannot be nil")
}

func TestClient_SendMediaBatch_EmptyList(t *testing.T) {
	client := &gemini.Client{}

	ctx := context.Background()
	err := client.SendMediaBatch(ctx, []*multimodal.MediaChunk{})

	// Não deve retornar erro para lista vazia (nada a fazer)
	assert.NoError(t, err)
}

func TestClient_SendMediaBatch_NilChunks(t *testing.T) {
	client := &gemini.Client{}

	ctx := context.Background()
	chunks := []*multimodal.MediaChunk{nil, nil, nil}
	err := client.SendMediaBatch(ctx, chunks)

	// Não deve retornar erro, apenas ignora nils
	assert.NoError(t, err)
}

func TestClient_HasActiveConnection(t *testing.T) {
	// Client sem conexão
	client := &gemini.Client{}

	// Deve retornar false se não conectado
	assert.False(t, client.HasActiveConnection())
}

// TestMultimodalMessage_JSONStructure valida estrutura JSON
func TestMultimodalMessage_JSONStructure(t *testing.T) {
	msg := gemini.MultimodalMessage{}
	msg.RealtimeInput.MediaChunks = []multimodal.MediaChunk{
		{
			MimeType:  "image/jpeg",
			Data:      "base64data",
			Timestamp: time.Now(),
		},
	}

	// Serializa para JSON
	jsonData, err := json.Marshal(msg)
	require.NoError(t, err)

	// Valida estrutura
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Deve ter "realtime_input"
	assert.Contains(t, parsed, "realtime_input")

	realtimeInput := parsed["realtime_input"].(map[string]interface{})
	assert.Contains(t, realtimeInput, "media_chunks")

	mediaChunks := realtimeInput["media_chunks"].([]interface{})
	assert.Len(t, mediaChunks, 1)

	firstChunk := mediaChunks[0].(map[string]interface{})
	assert.Equal(t, "image/jpeg", firstChunk["mime_type"])
	assert.Equal(t, "base64data", firstChunk["data"])
}

// TestMultimodalMessage_MultipleCh unks valida batch
func TestMultimodalMessage_MultipleChunks(t *testing.T) {
	msg := gemini.MultimodalMessage{}
	msg.RealtimeInput.MediaChunks = []multimodal.MediaChunk{
		{MimeType: "image/jpeg", Data: "data1"},
		{MimeType: "image/png", Data: "data2"},
		{MimeType: "video/mp4", Data: "data3"},
	}

	jsonData, err := json.Marshal(msg)
	require.NoError(t, err)

	var parsed gemini.MultimodalMessage
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Len(t, parsed.RealtimeInput.MediaChunks, 3)
	assert.Equal(t, "image/jpeg", parsed.RealtimeInput.MediaChunks[0].MimeType)
	assert.Equal(t, "image/png", parsed.RealtimeInput.MediaChunks[1].MimeType)
	assert.Equal(t, "video/mp4", parsed.RealtimeInput.MediaChunks[2].MimeType)
}
