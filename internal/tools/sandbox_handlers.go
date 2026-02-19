// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// 🖥️ SANDBOX — Execução de Código (Bash, Python, Node)
// ============================================================================

func (h *ToolsHandler) handleExecuteCode(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	language, _ := args["language"].(string)
	code, _ := args["code"].(string)
	timeoutSec, _ := args["timeout"].(float64)

	if code == "" {
		return map[string]interface{}{"error": "Informe o código a executar"}, nil
	}
	if language == "" {
		language = "bash"
	}

	if h.sandboxService == nil {
		return map[string]interface{}{"error": "Serviço de sandbox não configurado"}, nil
	}

	timeout := 30 * time.Second
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}

	// Non-blocking
	go func() {
		ctx := context.Background()
		result, err := h.sandboxService.Execute(ctx, language, code, timeout)

		if h.NotifyFunc != nil {
			if err != nil {
				h.NotifyFunc(idosoID, "code_error", map[string]interface{}{
					"language": language,
					"error":    err.Error(),
				})
				return
			}
			h.NotifyFunc(idosoID, "code_result", map[string]interface{}{
				"language":  result.Language,
				"output":    result.Output,
				"exit_code": result.ExitCode,
				"duration":  result.Duration.String(),
			})
		}
	}()

	return map[string]interface{}{
		"status":   "executando",
		"language": language,
		"message":  fmt.Sprintf("Executando código %s...", language),
	}, nil
}

// ============================================================================
// 🌐 BROWSER AUTOMATION
// ============================================================================

func (h *ToolsHandler) handleBrowserNavigate(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return map[string]interface{}{"error": "Informe a URL"}, nil
	}

	if h.browserService == nil {
		return map[string]interface{}{"error": "Serviço de browser não configurado"}, nil
	}

	// Non-blocking
	go func() {
		result, err := h.browserService.Navigate(url)

		if h.NotifyFunc != nil {
			if err != nil {
				h.NotifyFunc(idosoID, "browser_error", map[string]interface{}{
					"url":   url,
					"error": err.Error(),
				})
				return
			}

			// Converter links para interface
			var links []interface{}
			for _, link := range result.Links {
				links = append(links, map[string]interface{}{
					"text": link.Text,
					"href": link.Href,
				})
			}

			h.NotifyFunc(idosoID, "browser_result", map[string]interface{}{
				"url":         result.URL,
				"title":       result.Title,
				"text":        result.Text,
				"status_code": result.StatusCode,
				"links_count": len(result.Links),
				"links":       links,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "navegando",
		"url":     url,
		"message": fmt.Sprintf("Acessando %s...", url),
	}, nil
}

func (h *ToolsHandler) handleBrowserFillForm(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return map[string]interface{}{"error": "Informe a URL do formulário"}, nil
	}

	if h.browserService == nil {
		return map[string]interface{}{"error": "Serviço de browser não configurado"}, nil
	}

	// Extrair campos do formulário
	fields := make(map[string]string)
	if fieldsMap, ok := args["fields"].(map[string]interface{}); ok {
		for k, v := range fieldsMap {
			fields[k] = fmt.Sprintf("%v", v)
		}
	}

	if len(fields) == 0 {
		return map[string]interface{}{"error": "Informe os campos do formulário"}, nil
	}

	go func() {
		result, err := h.browserService.FillForm(url, fields)

		if h.NotifyFunc != nil {
			if err != nil {
				h.NotifyFunc(idosoID, "browser_error", map[string]interface{}{
					"url":   url,
					"error": err.Error(),
				})
				return
			}
			h.NotifyFunc(idosoID, "form_submitted", map[string]interface{}{
				"url":         result.URL,
				"status_code": result.StatusCode,
				"response":    result.Body,
			})
		}
	}()

	return map[string]interface{}{
		"status":  "submetendo",
		"url":     url,
		"fields":  len(fields),
		"message": fmt.Sprintf("Submetendo formulário em %s com %d campos...", url, len(fields)),
	}, nil
}

func (h *ToolsHandler) handleBrowserExtract(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	url, _ := args["url"].(string)
	selector, _ := args["selector"].(string)

	if url == "" {
		return map[string]interface{}{"error": "Informe a URL"}, nil
	}
	if selector == "" {
		selector = "text"
	}

	if h.browserService == nil {
		return map[string]interface{}{"error": "Serviço de browser não configurado"}, nil
	}

	go func() {
		results, err := h.browserService.ExtractData(url, selector)

		if h.NotifyFunc != nil {
			if err != nil {
				h.NotifyFunc(idosoID, "browser_error", map[string]interface{}{
					"url":   url,
					"error": err.Error(),
				})
				return
			}
			h.NotifyFunc(idosoID, "extract_result", map[string]interface{}{
				"url":      url,
				"selector": selector,
				"results":  results,
				"count":    len(results),
			})
		}
	}()

	return map[string]interface{}{
		"status":   "extraindo",
		"url":      url,
		"selector": selector,
		"message":  fmt.Sprintf("Extraindo dados de %s...", url),
	}, nil
}

// ============================================================================
// ⏰ CRON / SCHEDULED TASKS
// ============================================================================

func (h *ToolsHandler) handleCreateScheduledTask(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	description, _ := args["description"].(string)
	schedule, _ := args["schedule"].(string)
	toolName, _ := args["tool_name"].(string)

	if schedule == "" {
		return map[string]interface{}{"error": "Informe o schedule (ex: 'every 5m', 'daily 08:00', 'hourly')"}, nil
	}
	if description == "" {
		description = "Tarefa agendada"
	}

	if h.cronService == nil {
		return map[string]interface{}{"error": "Serviço de cron não configurado"}, nil
	}

	// Extrair args da tool a ser executada
	toolArgs := make(map[string]interface{})
	if ta, ok := args["tool_args"].(map[string]interface{}); ok {
		toolArgs = ta
	}

	task, err := h.cronService.CreateTask(idosoID, description, schedule, toolName, toolArgs)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	return map[string]interface{}{
		"status":      "sucesso",
		"task_id":     task.ID,
		"description": description,
		"schedule":    schedule,
		"next_run":    task.NextRun.Format("2006-01-02 15:04:05"),
		"message":     fmt.Sprintf("Tarefa '%s' agendada: %s (próximo: %s)", description, schedule, task.NextRun.Format("15:04")),
	}, nil
}

func (h *ToolsHandler) handleListScheduledTasks(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	if h.cronService == nil {
		return map[string]interface{}{"error": "Serviço de cron não configurado"}, nil
	}

	tasks := h.cronService.ListTasks(idosoID)

	var taskList []map[string]interface{}
	for _, t := range tasks {
		taskList = append(taskList, map[string]interface{}{
			"id":          t.ID,
			"description": t.Description,
			"schedule":    t.Schedule,
			"tool_name":   t.ToolName,
			"next_run":    t.NextRun.Format("2006-01-02 15:04:05"),
			"run_count":   t.RunCount,
		})
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"tasks":   taskList,
		"count":   len(taskList),
		"message": fmt.Sprintf("%d tarefas agendadas", len(taskList)),
	}, nil
}

func (h *ToolsHandler) handleCancelScheduledTask(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return map[string]interface{}{"error": "Informe o task_id"}, nil
	}

	if h.cronService == nil {
		return map[string]interface{}{"error": "Serviço de cron não configurado"}, nil
	}

	if err := h.cronService.CancelTask(taskID); err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}

	return map[string]interface{}{
		"status":  "sucesso",
		"message": fmt.Sprintf("Tarefa %s cancelada", taskID),
	}, nil
}

// ============================================================================
// 🤖 MULTI-LLM (Claude, GPT, DeepSeek)
// ============================================================================

func (h *ToolsHandler) handleAskLLM(idosoID int64, args map[string]interface{}) (map[string]interface{}, error) {
	provider, _ := args["provider"].(string)
	prompt, _ := args["prompt"].(string)

	if prompt == "" {
		return map[string]interface{}{"error": "Informe o prompt"}, nil
	}
	if provider == "" {
		provider = "claude"
	}

	if h.llmService == nil {
		return map[string]interface{}{"error": "Serviço multi-LLM não configurado"}, nil
	}

	// Non-blocking
	go func() {
		result, err := h.llmService.Ask(provider, prompt)

		if h.NotifyFunc != nil {
			if err != nil {
				log.Printf("❌ [LLM] %s erro: %v", provider, err)
				h.NotifyFunc(idosoID, "llm_error", map[string]interface{}{
					"provider": provider,
					"error":    err.Error(),
				})
				return
			}
			h.NotifyFunc(idosoID, "llm_response", map[string]interface{}{
				"provider": result.Provider,
				"model":    result.Model,
				"text":     result.Text,
				"tokens":   result.Tokens,
				"duration": result.Duration,
			})
		}
	}()

	return map[string]interface{}{
		"status":   "consultando",
		"provider": provider,
		"message":  fmt.Sprintf("Consultando %s...", provider),
	}, nil
}
