// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scholar

import (
	"context"
	"fmt"
	"strings"

	"eva/internal/cortex/learning"
	"eva/internal/swarm"

	"github.com/rs/zerolog/log"
)

// Agent implementa o ScholarSwarm - aprendizagem autonoma e pesquisa de conhecimento
type Agent struct {
	*swarm.BaseAgent
	learner *learning.AutonomousLearner
}

// New cria o ScholarSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"scholar",
			"Aprendizagem autonoma — pesquisa, estuda e memoriza conhecimento",
			swarm.PriorityLow,
		),
	}
	a.registerTools()
	return a
}

// SetLearner injeta o AutonomousLearner (chamado apos inicializacao em main.go)
func (a *Agent) SetLearner(learner *learning.AutonomousLearner) {
	a.learner = learner
}

func (a *Agent) registerTools() {
	// 1. study_topic — Pesquisar e aprender sobre um topico imediatamente
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "study_topic",
		Description: "Pesquisa um topico na internet, resume e armazena o conhecimento",
		Parameters: map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "O topico a ser estudado (ex: filosofia estoica, neurociencia do sono)",
			},
		},
		Required: []string{"topic"},
	}, a.handleStudyTopic)

	// 2. add_to_curriculum — Adicionar topico na fila de estudo
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "add_to_curriculum",
		Description: "Adiciona um topico na fila de estudo da EVA para aprender depois",
		Parameters: map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "O topico a ser adicionado ao curriculum",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Categoria do topico",
				"enum":        []string{"filosofia", "ciencia", "psicologia", "saude", "tecnologia", "historia", "arte", "educacao", "religiao", "cultura", "geral"},
			},
			"priority": map[string]interface{}{
				"type":        "integer",
				"description": "Prioridade de 1 (baixa) a 5 (alta)",
			},
		},
		Required: []string{"topic"},
	}, a.handleAddToCurriculum)

	// 3. list_curriculum — Listar topicos do curriculum
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "list_curriculum",
		Description: "Lista os topicos pendentes e completados do curriculum da EVA",
		Parameters: map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Filtrar por status",
				"enum":        []string{"pending", "studying", "completed", "failed", "all"},
			},
		},
	}, a.handleListCurriculum)

	// 4. search_knowledge — Busca semantica no que ja aprendeu
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_knowledge",
		Description: "Busca semantica no conhecimento que a EVA ja aprendeu",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "O que buscar no conhecimento aprendido",
			},
		},
		Required: []string{"query"},
	}, a.handleSearchKnowledge)
}

func (a *Agent) handleStudyTopic(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	topic, _ := call.Args["topic"].(string)
	if topic == "" {
		return &swarm.ToolResult{Success: false, Message: "Topico nao informado"}, nil
	}

	if a.learner == nil {
		return &swarm.ToolResult{Success: false, Message: "Learner nao inicializado"}, nil
	}

	log.Info().Str("topic", topic).Msg("[SCHOLAR] Studying topic on demand...")

	insights, err := a.learner.StudyTopic(ctx, topic)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro ao estudar '%s': %v", topic, err),
		}, nil
	}

	// Montar resposta com os insights
	var titles []string
	for _, i := range insights {
		titles = append(titles, i.Title)
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Aprendi %d insights sobre %s: %s", len(insights), topic, strings.Join(titles, "; ")),
		SuggestTone: "didatico_entusiasmado",
		Data: map[string]interface{}{
			"topic":          topic,
			"insights_count": len(insights),
			"insights":       insights,
		},
	}, nil
}

func (a *Agent) handleAddToCurriculum(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	topic, _ := call.Args["topic"].(string)
	if topic == "" {
		return &swarm.ToolResult{Success: false, Message: "Topico nao informado"}, nil
	}

	if a.learner == nil {
		return &swarm.ToolResult{Success: false, Message: "Learner nao inicializado"}, nil
	}

	category, _ := call.Args["category"].(string)
	priority := 3
	if p, ok := call.Args["priority"].(float64); ok {
		priority = int(p)
	}

	err := a.learner.AddToCurriculum(ctx, topic, category, "user", priority)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro ao adicionar '%s' ao curriculum: %v", topic, err),
		}, nil
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Topico '%s' adicionado ao meu curriculum de estudos (prioridade %d). Vou estudar em breve!", topic, priority),
	}, nil
}

func (a *Agent) handleListCurriculum(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.learner == nil {
		return &swarm.ToolResult{Success: false, Message: "Learner nao inicializado"}, nil
	}

	status, _ := call.Args["status"].(string)
	if status == "all" {
		status = ""
	}

	items, err := a.learner.ListCurriculum(ctx, status, 20)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro ao listar curriculum: %v", err),
		}, nil
	}

	if len(items) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: "Meu curriculum esta vazio. Adicione topicos para eu estudar!",
		}, nil
	}

	var lines []string
	for _, item := range items {
		statusIcon := "⏳"
		switch item.Status {
		case "completed":
			statusIcon = "✅"
		case "studying":
			statusIcon = "📖"
		case "failed":
			statusIcon = "❌"
		}
		lines = append(lines, fmt.Sprintf("%s %s [%s] (P%d, %d insights)", statusIcon, item.Topic, item.Category, item.Priority, item.InsightsCount))
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Meu curriculum (%d topicos):\n%s", len(items), strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"items": items, "count": len(items)},
	}, nil
}

func (a *Agent) handleSearchKnowledge(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "Query nao informada"}, nil
	}

	if a.learner == nil {
		return &swarm.ToolResult{Success: false, Message: "Learner nao inicializado"}, nil
	}

	insights, err := a.learner.SearchLearnings(ctx, query, 5)
	if err != nil {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Erro na busca: %v", err),
		}, nil
	}

	if len(insights) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Nao encontrei nada sobre '%s' no meu conhecimento. Quer que eu estude sobre isso?", query),
		}, nil
	}

	var lines []string
	for _, i := range insights {
		lines = append(lines, fmt.Sprintf("- %s (%s): %s", i.Title, i.Category, truncate(i.Summary, 150)))
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Encontrei %d resultados sobre '%s':\n%s", len(insights), query, strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"insights": insights, "count": len(insights)},
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
