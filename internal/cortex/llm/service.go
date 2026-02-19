// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Response resposta de um LLM
type Response struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Text     string `json:"text"`
	Tokens   int    `json:"tokens,omitempty"`
	Duration int64  `json:"duration_ms"`
}

// Provider configuração de um provider LLM
type Provider struct {
	Name    string
	APIKey  string
	BaseURL string
	Model   string
}

// Service multi-LLM provider
type Service struct {
	providers map[string]*Provider
	client    *http.Client
}

// NewService cria multi-LLM service
func NewService() *Service {
	return &Service{
		providers: make(map[string]*Provider),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// AddProvider adiciona um provider LLM
func (s *Service) AddProvider(name, apiKey, baseURL, model string) {
	s.providers[strings.ToLower(name)] = &Provider{
		Name:    name,
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}
}

// ListProviders lista providers configurados
func (s *Service) ListProviders() []string {
	var names []string
	for name, p := range s.providers {
		if p.APIKey != "" {
			names = append(names, name)
		}
	}
	return names
}

// Ask envia prompt para um provider específico
func (s *Service) Ask(providerName, prompt string) (*Response, error) {
	providerName = strings.ToLower(providerName)

	provider, ok := s.providers[providerName]
	if !ok {
		available := s.ListProviders()
		return nil, fmt.Errorf("provider '%s' não configurado (disponíveis: %v)", providerName, available)
	}
	if provider.APIKey == "" {
		return nil, fmt.Errorf("API key não configurada para %s", providerName)
	}

	switch providerName {
	case "claude", "anthropic":
		return s.askClaude(provider, prompt)
	case "gpt", "openai", "chatgpt":
		return s.askOpenAI(provider, prompt)
	case "deepseek":
		return s.askDeepSeek(provider, prompt)
	default:
		// Tentar formato OpenAI-compatible (funciona com muitos providers)
		return s.askOpenAICompatible(provider, prompt)
	}
}

// askClaude chama Anthropic Messages API
func (s *Service) askClaude(p *Provider, prompt string) (*Response, error) {
	body := map[string]interface{}{
		"model":      p.Model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", p.BaseURL+"/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude request failed: %v", err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("claude error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &result)

	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}

	return &Response{
		Provider: "claude",
		Model:    p.Model,
		Text:     text,
		Tokens:   result.Usage.OutputTokens,
		Duration: duration.Milliseconds(),
	}, nil
}

// askOpenAI chama OpenAI Chat Completions API
func (s *Service) askOpenAI(p *Provider, prompt string) (*Response, error) {
	return s.askOpenAICompatible(p, prompt)
}

// askDeepSeek chama DeepSeek API (OpenAI-compatible)
func (s *Service) askDeepSeek(p *Provider, prompt string) (*Response, error) {
	return s.askOpenAICompatible(p, prompt)
}

// askOpenAICompatible formato genérico OpenAI Chat Completions
func (s *Service) askOpenAICompatible(p *Provider, prompt string) (*Response, error) {
	body := map[string]interface{}{
		"model":      p.Model,
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(body)

	apiURL := p.BaseURL
	if !strings.HasSuffix(apiURL, "/chat/completions") {
		apiURL = strings.TrimSuffix(apiURL, "/") + "/v1/chat/completions"
	}

	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %v", p.Name, err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s error (%d): %s", p.Name, resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &result)

	text := ""
	if len(result.Choices) > 0 {
		text = result.Choices[0].Message.Content
	}

	return &Response{
		Provider: p.Name,
		Model:    p.Model,
		Text:     text,
		Tokens:   result.Usage.TotalTokens,
		Duration: duration.Milliseconds(),
	}, nil
}
