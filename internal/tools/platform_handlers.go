// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"fmt"
	"log"
)

// ============================================================================
// 💬 MESSAGING CHANNELS (Slack, Discord, Teams, Signal)
// ============================================================================

func (h *ToolsHandler) handleSendSlack(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	channel, _ := args["channel"].(string)
	message, _ := args["message"].(string)

	if message == "" {
		return map[string]interface{}{"error": "Informe a mensagem"}, nil
	}
	if channel == "" {
		channel = "#general"
	}

	if h.slackService == nil {
		return map[string]interface{}{"error": "Slack não configurado — defina SLACK_BOT_TOKEN"}, nil
	}

	go func() {
		err := h.slackService.SendMessage(channel, message)
		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [SLACK] Erro: %v", err)
				h.NotifyFunc(idosoID, "slack_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "slack_sent", map[string]interface{}{
				"channel": channel,
				"message": message,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "enviando",
		"channel": channel,
		"message": fmt.Sprintf("Enviando mensagem para %s no Slack...", channel),
	}, nil
}

func (h *ToolsHandler) handleSendDiscord(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	channelID, _ := args["channel_id"].(string)
	message, _ := args["message"].(string)

	if message == "" {
		return map[string]interface{}{"error": "Informe a mensagem"}, nil
	}
	if channelID == "" {
		return map[string]interface{}{"error": "Informe o channel_id do Discord"}, nil
	}

	if h.discordService == nil {
		return map[string]interface{}{"error": "Discord não configurado — defina DISCORD_BOT_TOKEN"}, nil
	}

	go func() {
		err := h.discordService.SendMessage(channelID, message)
		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [DISCORD] Erro: %v", err)
				h.NotifyFunc(idosoID, "discord_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "discord_sent", map[string]interface{}{
				"channel_id": channelID,
				"message":    message,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "enviando",
		"message": "Enviando mensagem no Discord...",
	}, nil
}

func (h *ToolsHandler) handleSendTeams(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	message, _ := args["message"].(string)

	if message == "" {
		return map[string]interface{}{"error": "Informe a mensagem"}, nil
	}

	if h.teamsService == nil {
		return map[string]interface{}{"error": "Teams não configurado — defina TEAMS_WEBHOOK_URL"}, nil
	}

	go func() {
		err := h.teamsService.SendMessage("", message)
		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [TEAMS] Erro: %v", err)
				h.NotifyFunc(idosoID, "teams_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "teams_sent", map[string]interface{}{"message": message})
		}
	}()

	return map[string]interface{}{
		"status":  "enviando",
		"message": "Enviando mensagem no Microsoft Teams...",
	}, nil
}

func (h *ToolsHandler) handleSendSignal(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	recipient, _ := args["recipient"].(string)
	message, _ := args["message"].(string)

	if message == "" {
		return map[string]interface{}{"error": "Informe a mensagem"}, nil
	}
	if recipient == "" {
		return map[string]interface{}{"error": "Informe o destinatário (número de telefone)"}, nil
	}

	if h.signalService == nil {
		return map[string]interface{}{"error": "Signal não configurado — instale signal-cli e defina SIGNAL_CLI_PATH"}, nil
	}

	go func() {
		err := h.signalService.SendMessage(recipient, message)
		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [SIGNAL] Erro: %v", err)
				h.NotifyFunc(idosoID, "signal_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "signal_sent", map[string]interface{}{
				"recipient": recipient,
				"message":   message,
			})
		}
	}()

	return map[string]interface{}{
		"status":    "enviando",
		"recipient": recipient,
		"message":   fmt.Sprintf("Enviando mensagem Signal para %s...", recipient),
	}, nil
}

// ============================================================================
// 🏠 SMART HOME (Home Assistant IoT)
// ============================================================================

func (h *ToolsHandler) handleSmartHomeControl(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	deviceID, _ := args["device_id"].(string)
	action, _ := args["action"].(string)

	if deviceID == "" {
		return map[string]interface{}{"error": "Informe o device_id (ex: light.sala, switch.ventilador)"}, nil
	}
	if action == "" {
		return map[string]interface{}{"error": "Informe a ação (on, off, toggle)"}, nil
	}

	if h.smartHomeService == nil {
		return map[string]interface{}{"error": "Smart Home não configurado — defina HOME_ASSISTANT_URL e HOME_ASSISTANT_TOKEN"}, nil
	}

	// Extrair dados extras (brightness, temperature, etc)
	data := make(map[string]interface{})
	if brightness, ok := args["brightness"].(float64); ok {
		data["brightness"] = int(brightness)
	}
	if temperature, ok := args["temperature"].(float64); ok {
		data["temperature"] = temperature
	}

	go func() {
		err := h.smartHomeService.ControlDevice(deviceID, action, data)
		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [SMART HOME] Erro: %v", err)
				h.NotifyFunc(idosoID, "smart_home_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "smart_home_controlled", map[string]interface{}{
				"device_id": deviceID,
				"action":    action,
			})
		}
	}()

	return map[string]interface{}{
		"status":    "executando",
		"device_id": deviceID,
		"action":    action,
		"message":   fmt.Sprintf("Executando '%s' em %s...", action, deviceID),
	}, nil
}

func (h *ToolsHandler) handleSmartHomeStatus(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	deviceID, _ := args["device_id"].(string)

	if h.smartHomeService == nil {
		return map[string]interface{}{"error": "Smart Home não configurado"}, nil
	}

	if deviceID != "" {
		device, err := h.smartHomeService.GetDeviceState(deviceID)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}, nil
		}
		return map[string]interface{}{
			"status":     "sucesso",
			"device_id":  device.ID,
			"name":       device.Name,
			"state":      device.State,
			"attributes": device.Attributes,
		}, nil
	}

	// Listar todos
	devices, err := h.smartHomeService.ListDevices()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	var deviceList []map[string]interface{}
	for _, d := range devices {
		deviceList = append(deviceList, map[string]interface{}{
			"id":     d.ID,
			"name":   d.Name,
			"domain": d.Domain,
			"state":  d.State,
		})
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"devices": deviceList,
		"count":   len(deviceList),
		"message": fmt.Sprintf("%d dispositivos encontrados", len(deviceList)),
	}, nil
}

// ============================================================================
// 🔗 WEBHOOKS
// ============================================================================

func (h *ToolsHandler) handleCreateWebhook(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["name"].(string)
	url, _ := args["url"].(string)

	if name == "" || url == "" {
		return map[string]interface{}{"error": "Informe name e url"}, nil
	}

	if h.webhookService == nil {
		return map[string]interface{}{"error": "Serviço de webhooks não configurado"}, nil
	}

	var events []string
	if evts, ok := args["events"].([]interface{}); ok {
		for _, e := range evts {
			if s, ok := e.(string); ok {
				events = append(events, s)
			}
		}
	}
	if len(events) == 0 {
		events = []string{"*"}
	}

	wh, err := h.webhookService.Create(name, url, events)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	return map[string]interface{}{
		"status":     "sucesso",
		"webhook_id": wh.ID,
		"name":       wh.Name,
		"url":        wh.URL,
		"events":     wh.Events,
		"secret":     wh.Secret,
		"message":    fmt.Sprintf("Webhook '%s' criado", name),
	}, nil
}

func (h *ToolsHandler) handleListWebhooks(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.webhookService == nil {
		return map[string]interface{}{"error": "Serviço de webhooks não configurado"}, nil
	}

	webhooks := h.webhookService.List()

	var whList []map[string]interface{}
	for _, wh := range webhooks {
		whList = append(whList, map[string]interface{}{
			"id":         wh.ID,
			"name":       wh.Name,
			"url":        wh.URL,
			"events":     wh.Events,
			"active":     wh.Active,
			"fire_count": wh.FireCount,
		})
	}

	return map[string]interface{}{
		"status":   "sucesso",
		"webhooks": whList,
		"count":    len(whList),
	}, nil
}

func (h *ToolsHandler) handleTriggerWebhook(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return map[string]interface{}{"error": "Informe o nome do webhook"}, nil
	}

	if h.webhookService == nil {
		return map[string]interface{}{"error": "Serviço de webhooks não configurado"}, nil
	}

	payload := make(map[string]interface{})
	if p, ok := args["payload"].(map[string]interface{}); ok {
		payload = p
	}

	go func() {
		result, err := h.webhookService.TriggerByName(name, payload)
		if h.NotifyFunc != nil {
			if err != nil {
				h.NotifyFunc(idosoID, "webhook_error", map[string]interface{}{"error": err.Error()})
				return
			}
			h.NotifyFunc(idosoID, "webhook_triggered", map[string]interface{}{
				"webhook_id":  result.WebhookID,
				"status_code": result.StatusCode,
				"duration":    result.Duration,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "disparando",
		"name":    name,
		"message": fmt.Sprintf("Disparando webhook '%s'...", name),
	}, nil
}

// ============================================================================
// 🧩 SKILLS (Self-Improving Runtime)
// ============================================================================

func (h *ToolsHandler) handleCreateSkill(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	language, _ := args["language"].(string)
	code, _ := args["code"].(string)

	if name == "" || code == "" {
		return map[string]interface{}{"error": "Informe name e code"}, nil
	}
	if language == "" {
		language = "bash"
	}

	if h.skillsService == nil {
		return map[string]interface{}{"error": "Serviço de skills não configurado"}, nil
	}

	skill, err := h.skillsService.Create(name, description, language, code, "eva")
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	return map[string]interface{}{
		"status":      "sucesso",
		"name":        skill.Name,
		"language":    skill.Language,
		"version":     skill.Version,
		"description": skill.Description,
		"message":     fmt.Sprintf("Skill '%s' criada (v%d, %s)", skill.Name, skill.Version, skill.Language),
	}, nil
}

func (h *ToolsHandler) handleListSkills(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.skillsService == nil {
		return map[string]interface{}{"error": "Serviço de skills não configurado"}, nil
	}

	skills := h.skillsService.List()

	var skillList []map[string]interface{}
	for _, s := range skills {
		skillList = append(skillList, map[string]interface{}{
			"name":        s.Name,
			"description": s.Description,
			"language":    s.Language,
			"version":     s.Version,
			"author":      s.Author,
			"run_count":   s.RunCount,
		})
	}

	return map[string]interface{}{
		"status": "sucesso",
		"skills": skillList,
		"count":  len(skillList),
	}, nil
}

func (h *ToolsHandler) handleExecuteSkill(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["skill_name"].(string)
	if name == "" {
		return map[string]interface{}{"error": "Informe o skill_name"}, nil
	}

	if h.skillsService == nil {
		return map[string]interface{}{"error": "Serviço de skills não configurado"}, nil
	}

	// Extrair args para a skill
	skillArgs := make(map[string]interface{})
	if sa, ok := args["args"].(map[string]interface{}); ok {
		skillArgs = sa
	}

	go func() {
		ctx := context.Background()
		result, err := h.skillsService.Execute(ctx, name, skillArgs)

		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [SKILLS] Erro ao executar %s: %v", name, err)
				h.NotifyFunc(idosoID, "skill_error", map[string]interface{}{
					"skill_name": name,
					"error":      err.Error(),
				})
				return
			}
			h.NotifyFunc(idosoID, "skill_result", map[string]interface{}{
				"skill_name": result.SkillName,
				"output":     result.Output,
				"exit_code":  result.ExitCode,
				"duration":   result.Duration,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "executando",
		"skill":   name,
		"message": fmt.Sprintf("Executando skill '%s'...", name),
	}, nil
}

func (h *ToolsHandler) handleDeleteSkill(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	name, _ := args["skill_name"].(string)
	if name == "" {
		return map[string]interface{}{"error": "Informe o skill_name"}, nil
	}

	if h.skillsService == nil {
		return map[string]interface{}{"error": "Serviço de skills não configurado"}, nil
	}

	if err := h.skillsService.Delete(name); err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"message": fmt.Sprintf("Skill '%s' removida", name),
	}, nil
}
