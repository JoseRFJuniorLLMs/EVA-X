// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package google

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o GoogleSwarm - Calendar, Gmail, Drive, Sheets, Docs, Maps, YouTube, Fit
type Agent struct {
	*swarm.BaseAgent
}

// New cria o GoogleSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"google",
			"Integrações Google (Calendar, Gmail, Drive, Sheets, Docs, Maps, YouTube, Fit)",
			swarm.PriorityMedium,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// Calendar
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "manage_calendar_event",
		Description: "Gerencia eventos no Google Calendar (cria ou lista)",
		Parameters: map[string]interface{}{
			"action":      map[string]interface{}{"type": "string", "description": "Ação: create ou list", "enum": []string{"create", "list"}},
			"summary":     map[string]interface{}{"type": "string", "description": "Título do evento"},
			"description": map[string]interface{}{"type": "string", "description": "Descrição"},
			"start_time":  map[string]interface{}{"type": "string", "description": "Início (ISO 8601)"},
			"end_time":    map[string]interface{}{"type": "string", "description": "Término (ISO 8601)"},
		},
		Required: []string{"action"},
	}, a.handleGoogle("calendar"))

	// Gmail
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "send_email",
		Description: "Envia email usando Gmail do usuário",
		Parameters: map[string]interface{}{
			"to":      map[string]interface{}{"type": "string", "description": "Email destinatário"},
			"subject": map[string]interface{}{"type": "string", "description": "Assunto"},
			"body":    map[string]interface{}{"type": "string", "description": "Corpo do email"},
		},
		Required: []string{"to", "subject", "body"},
	}, a.handleGoogle("gmail"))

	// Drive
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "save_to_drive",
		Description: "Salva documento no Google Drive",
		Parameters: map[string]interface{}{
			"filename": map[string]interface{}{"type": "string", "description": "Nome do arquivo"},
			"content":  map[string]interface{}{"type": "string", "description": "Conteúdo"},
			"folder":   map[string]interface{}{"type": "string", "description": "Pasta (opcional)"},
		},
		Required: []string{"filename", "content"},
	}, a.handleGoogle("drive"))

	// Sheets
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "manage_health_sheet",
		Description: "Gerencia planilha de saúde no Google Sheets",
		Parameters: map[string]interface{}{
			"action": map[string]interface{}{"type": "string", "description": "create ou append", "enum": []string{"create", "append"}},
			"title":  map[string]interface{}{"type": "string", "description": "Título da planilha"},
			"data":   map[string]interface{}{"type": "object", "description": "Dados de saúde"},
		},
		Required: []string{"action"},
	}, a.handleGoogle("sheets"))

	// Docs
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "create_health_doc",
		Description: "Cria documento de saúde no Google Docs",
		Parameters: map[string]interface{}{
			"title":   map[string]interface{}{"type": "string", "description": "Título"},
			"content": map[string]interface{}{"type": "string", "description": "Conteúdo"},
		},
		Required: []string{"title", "content"},
	}, a.handleGoogle("docs"))

	// Maps
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "find_nearby_places",
		Description: "Busca lugares próximos (farmácias, hospitais, etc)",
		Parameters: map[string]interface{}{
			"place_type": map[string]interface{}{"type": "string", "description": "Tipo de lugar"},
			"location":   map[string]interface{}{"type": "string", "description": "Localização (lat,lng)"},
			"radius":     map[string]interface{}{"type": "integer", "description": "Raio em metros"},
		},
		Required: []string{"place_type", "location"},
	}, a.handleGoogle("maps"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_places",
		Description: "Pesquisar endereços, restaurantes, farmácias",
		Parameters: map[string]interface{}{
			"query":  map[string]interface{}{"type": "string", "description": "O que buscar"},
			"type":   map[string]interface{}{"type": "string", "description": "restaurant, pharmacy, hospital..."},
			"radius": map[string]interface{}{"type": "integer", "description": "Distância em metros"},
		},
	}, a.handleGoogle("maps"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "get_directions",
		Description: "Obter rota para um local",
		Parameters: map[string]interface{}{
			"destination": map[string]interface{}{"type": "string", "description": "Endereço/local"},
			"mode":        map[string]interface{}{"type": "string", "description": "walking, driving, transit"},
		},
		Required: []string{"destination"},
	}, a.handleGoogle("maps"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "nearby_transport",
		Description: "Ver transporte público próximo",
		Parameters: map[string]interface{}{
			"type": map[string]interface{}{"type": "string", "description": "bus, metro, all"},
		},
	}, a.handleGoogle("maps"))

	// YouTube
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_videos",
		Description: "Busca vídeos no YouTube",
		Parameters: map[string]interface{}{
			"query":       map[string]interface{}{"type": "string", "description": "Termo de busca"},
			"max_results": map[string]interface{}{"type": "integer", "description": "Máximo de resultados"},
		},
		Required: []string{"query"},
	}, a.handleGoogle("youtube"))

	// Fit
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "get_health_data",
		Description: "Recupera dados de saúde do Google Fit",
		Parameters:  map[string]interface{}{},
	}, a.handleGoogle("fit"))

	// Search
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "google_search_retrieval",
		Description: "Pesquisa informações na internet",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "description": "Termo de pesquisa"},
		},
	}, a.handleGoogle("search"))
}

func (a *Agent) handleGoogle(service string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🔷 [GOOGLE:%s] %s userID=%d args=%v", service, call.Name, call.UserID, call.Args)

		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Executando %s via Google %s", call.Name, service),
			Data: map[string]interface{}{
				"action":  call.Name,
				"service": service,
				"args":    call.Args,
				"user_id": call.UserID,
			},
		}, nil
	}
}
