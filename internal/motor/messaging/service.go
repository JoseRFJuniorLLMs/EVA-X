// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

// ============================================================================
// Interface comum para messaging channels
// ============================================================================

// Channel interface para envio de mensagens
type Channel interface {
	SendMessage(recipient, text string) error
	Name() string
}

// ============================================================================
// SLACK
// ============================================================================

// SlackService cliente para Slack Web API
type SlackService struct {
	botToken string
	client   *http.Client
}

// NewSlackService cria Slack service
func NewSlackService(botToken string) *SlackService {
	return &SlackService{
		botToken: botToken,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SlackService) Name() string { return "slack" }

// SendMessage envia mensagem para canal ou usuário Slack
func (s *SlackService) SendMessage(channel, text string) error {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    text,
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal(body, &result)

	if !result.OK {
		return fmt.Errorf("slack error: %s", result.Error)
	}
	return nil
}

// SendRichMessage envia mensagem com blocos Slack (markdown)
func (s *SlackService) SendRichMessage(channel, text string, blocks []map[string]interface{}) error {
	payload := map[string]interface{}{
		"channel": channel,
		"text":    text,
		"blocks":  blocks,
	}

	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack request failed: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

// ============================================================================
// DISCORD
// ============================================================================

// DiscordService cliente para Discord Bot API
type DiscordService struct {
	botToken string
	client   *http.Client
}

// NewDiscordService cria Discord service
func NewDiscordService(botToken string) *DiscordService {
	return &DiscordService{
		botToken: botToken,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DiscordService) Name() string { return "discord" }

// SendMessage envia mensagem para canal Discord
func (d *DiscordService) SendMessage(channelID, text string) error {
	payload := map[string]interface{}{
		"content": text,
	}

	jsonBody, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.botToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendEmbed envia mensagem com embed formatado
func (d *DiscordService) SendEmbed(channelID, title, description string, color int) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": description,
				"color":       color,
			},
		},
	}

	jsonBody, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.botToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord embed failed: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

// ============================================================================
// MICROSOFT TEAMS (Incoming Webhook)
// ============================================================================

// TeamsService cliente para Microsoft Teams via Incoming Webhook
type TeamsService struct {
	webhookURL string
	client     *http.Client
}

// NewTeamsService cria Teams service
func NewTeamsService(webhookURL string) *TeamsService {
	return &TeamsService{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TeamsService) Name() string { return "teams" }

// SendMessage envia mensagem para canal Teams
func (t *TeamsService) SendMessage(_, text string) error {
	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    "EVA Message",
		"themeColor": "0076D7",
		"title":      "EVA-Mind",
		"sections": []map[string]interface{}{
			{
				"activityTitle": "Mensagem da EVA",
				"text":          text,
			},
		},
	}

	jsonBody, _ := json.Marshal(payload)
	resp, err := t.client.Post(t.webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("teams request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// ============================================================================
// SIGNAL (via signal-cli)
// ============================================================================

// SignalService cliente para Signal via signal-cli
type SignalService struct {
	cliPath    string
	senderNum  string
}

// NewSignalService cria Signal service (usa signal-cli)
func NewSignalService(cliPath, senderNumber string) *SignalService {
	if cliPath == "" {
		cliPath = "signal-cli"
	}
	return &SignalService{
		cliPath:   cliPath,
		senderNum: senderNumber,
	}
}

func (s *SignalService) Name() string { return "signal" }

// SendMessage envia mensagem via Signal
func (s *SignalService) SendMessage(recipient, text string) error {
	cmd := exec.Command(s.cliPath, "-u", s.senderNum, "send", "-m", text, recipient)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal send failed: %v — %s", err, string(output))
	}
	return nil
}
