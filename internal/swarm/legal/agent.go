// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package legal

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o LegalSwarm - auxílio jurídico, direitos e burocracia
type Agent struct {
	*swarm.BaseAgent
}

// New cria o LegalSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"legal",
			"Auxílio jurídico, direitos do idoso e gestão de documentos",
			swarm.PriorityMedium,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// Ferramenta para listar direitos do idoso
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "get_elderly_rights",
		Description: "Busca informações sobre direitos específicos do idoso (transporte, saúde, lazer)",
		Parameters: map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "Tópico de interesse",
				"enum":        []string{"transporte", "saude", "prioridade", "isencao_impostos", "lazer"},
			},
		},
		Required: []string{"topic"},
	}, a.handleLegal("get_rights"))

	// Ferramenta para gestão de documentos (simulação)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "document_status",
		Description: "Verifica o status de renovação de documentos (RG, CNH, Carteira do Idoso)",
		Parameters: map[string]interface{}{
			"document_type": map[string]interface{}{
				"type":        "string",
				"description": "Tipo do documento",
			},
		},
		Required: []string{"document_type"},
	}, a.handleLegal("doc_status"))

	// Ferramenta para explicar termos jurídicos
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "explain_legal_term",
		Description: "Explica um termo jurídico de forma simples e acessível",
		Parameters: map[string]interface{}{
			"term": map[string]interface{}{
				"type":        "string",
				"description": "O termo jurídico a ser explicado",
			},
		},
		Required: []string{"term"},
	}, a.handleLegal("explain_term"))
}

func (a *Agent) handleLegal(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("⚖️ [LEGAL:%s] %s userID=%d", action, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Ação legal '%s' processada com sucesso.", action),
			SuggestTone: "formal_seguro",
			Data: map[string]interface{}{
				"action":  call.Name,
				"args":    call.Args,
				"user_id": call.UserID,
			},
		}, nil
	}
}
