package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// Client cliente para EVA-Voice API
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient cria novo cliente
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// CloneVoiceResponse resposta de clonagem
type CloneVoiceResponse struct {
	VoiceID     string  `json:"voice_id"`
	UserID      string  `json:"user_id"`
	EmotionType string  `json:"emotion_type"`
	FileSizeKB  float64 `json:"file_size_kb"`
}

// SpeakRequest requisição de fala
type SpeakRequest struct {
	Text        string  `json:"text"`
	VoiceID     string  `json:"voice_id"`
	Language    string  `json:"language"`
	EmotionType string  `json:"emotion_type"`
	Speed       float64 `json:"speed"`
}

// CloneVoice clona uma voz a partir de áudio
func (c *Client) CloneVoice(audioData []byte, userID, emotionType string) (*CloneVoiceResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Adicionar áudio
	part, err := writer.CreateFormFile("audio", "voice.wav")
	if err != nil {
		return nil, err
	}
	part.Write(audioData)

	// Adicionar campos
	writer.WriteField("user_id", userID)
	writer.WriteField("emotion_type", emotionType)
	writer.Close()

	resp, err := c.client.Post(
		c.baseURL+"/clone-voice",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro ao clonar voz: %s", string(bodyBytes))
	}

	var result CloneVoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Speak gera áudio com voz clonada
func (c *Client) Speak(text, voiceID, emotionType string, speed float64) ([]byte, error) {
	req := SpeakRequest{
		Text:        text,
		VoiceID:     voiceID,
		Language:    "pt",
		EmotionType: emotionType,
		Speed:       speed,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(
		c.baseURL+"/speak",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro ao gerar fala: %s", string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// HealthCheck verifica se o serviço está online
func (c *Client) HealthCheck() (bool, error) {
	resp, err := c.client.Get(c.baseURL + "/")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
