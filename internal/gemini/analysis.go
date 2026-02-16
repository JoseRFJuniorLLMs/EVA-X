package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"eva-mind/internal/brainstem/config"
)

type AnalysisResult struct {
	Summary   string `json:"summary"`
	Sentiment string `json:"sentiment"`
	Score     int    `json:"score"`
}

// AnalyzeTranscript envia a transcrição para o Gemini via REST API para análise
func AnalyzeTranscript(ctx context.Context, cfg *config.Config, transcript string) (*AnalysisResult, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		cfg.ModelID, cfg.GoogleAPIKey)

	prompt := fmt.Sprintf(`Analise a seguinte transcrição de uma conversa entre a assistente virtual EVA e um idoso.
Extraia:
1. Um resumo curto da conversa (máximo 200 caracteres).
2. O sentimento geral do idoso (feliz, neutro, triste, ansioso, irritado, confuso, apatico).
3. Uma intensidade desse sentimento de 1 a 10.

Responda APENAS em formato JSON puro, sem markdown, com os campos: "summary", "sentiment", "score".

Transcrição:
%s`, transcript)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"response_mime_type": "application/json",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api error: %s", resp.Status)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	var result AnalysisResult
	err = json.Unmarshal([]byte(geminiResp.Candidates[0].Content.Parts[0].Text), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis json: %w", err)
	}

	return &result, nil
}
