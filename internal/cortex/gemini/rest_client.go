package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"eva-mind/internal/brainstem/config"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ============================================================================
// PERFORMANCE FIX: HTTP Client Singleton com Timeout
// Issue: http.Post() sem timeout pode travar indefinidamente
// Fix: Cliente reutilizavel com timeout de 30s e connection pooling
// ============================================================================

var httpClientWithTimeout = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	},
}

// AnalyzeText envia um prompt para o Gemini via REST API (não-stream)
// Útil para raciocínio (Thinking) e análise de contexto
func AnalyzeText(cfg *config.Config, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.GeminiAnalysisModel, cfg.GoogleAPIKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.2, // Baixa temperatura para raciocínio lógico
			"maxOutputTokens": 1024,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("erro ao criar JSON: %w", err)
	}

	// PERFORMANCE FIX: Usar HTTP client com timeout (30s)
	resp, err := httpClientWithTimeout.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("erro da API Gemini (%d): %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	// Extrair texto da resposta
	if candidates, ok := response["candidates"].([]interface{}); ok && len(candidates) > 0 {
		candidate := candidates[0].(map[string]interface{})
		if content, ok := candidate["content"].(map[string]interface{}); ok {
			if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
				part := parts[0].(map[string]interface{})
				if text, ok := part["text"].(string); ok {
					return text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("nenhum texto retornado na resposta")
}

// AnalyzeAudio envia áudio (PCM) + prompt para o Gemini via REST API
func AnalyzeAudio(cfg *config.Config, audioData []byte, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.GeminiAnalysisModel, cfg.GoogleAPIKey)

	// Encode PCM to Base64 (Gemini REST expects base64 in inlineData)
	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
					{
						"inlineData": map[string]string{
							"mimeType": "audio/pcm;rate=24000", // Assuming 24kHz match
							"data":     encodedAudio,
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.4, // Um pouco mais criativo para detectar emoção
			"maxOutputTokens": 1024,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("erro ao criar JSON: %w", err)
	}

	// PERFORMANCE FIX: Usar HTTP client com timeout (30s)
	resp, err := httpClientWithTimeout.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("erro da API Gemini Audio (%d): %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	// Extrair texto da resposta
	if candidates, ok := response["candidates"].([]interface{}); ok && len(candidates) > 0 {
		candidate := candidates[0].(map[string]interface{})
		if content, ok := candidate["content"].(map[string]interface{}); ok {
			if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
				part := parts[0].(map[string]interface{})
				if text, ok := part["text"].(string); ok {
					return text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("nenhum texto retornado na análise de áudio")
}
