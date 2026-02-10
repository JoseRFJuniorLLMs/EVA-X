package external

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o ExternalSwarm - Spotify, Uber, WhatsApp, SQL, Voice, Apps
type Agent struct {
	*swarm.BaseAgent
}

// New cria o ExternalSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"external",
			"Spotify, Uber, WhatsApp, SQL, mudança de voz, apps",
			swarm.PriorityLow,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "play_music",
		Description: "Toca música no Spotify",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "description": "Música ou artista"},
		},
		Required: []string{"query"},
	}, a.handleExternal("spotify"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "request_ride",
		Description: "Solicita corrida Uber",
		Parameters: map[string]interface{}{
			"startLat": map[string]interface{}{"type": "number", "description": "Latitude origem"},
			"startLng": map[string]interface{}{"type": "number", "description": "Longitude origem"},
			"endLat":   map[string]interface{}{"type": "number", "description": "Latitude destino"},
			"endLng":   map[string]interface{}{"type": "number", "description": "Longitude destino"},
		},
		Required: []string{"startLat", "startLng", "endLat", "endLng"},
	}, a.handleExternal("uber"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "send_whatsapp",
		Description: "Envia mensagem WhatsApp",
		Parameters: map[string]interface{}{
			"to":      map[string]interface{}{"type": "string", "description": "Número destino"},
			"message": map[string]interface{}{"type": "string", "description": "Mensagem"},
		},
		Required: []string{"to", "message"},
	}, a.handleExternal("whatsapp"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "run_sql_select",
		Description: "Executa consulta SQL SELECT (apenas leitura)",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "description": "Query SQL SELECT"},
		},
		Required: []string{"query"},
	}, a.handleExternal("sql"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "change_voice",
		Description: "Altera a voz da EVA em tempo real",
		Parameters: map[string]interface{}{
			"voice_name": map[string]interface{}{
				"type": "string", "description": "Nome da voz",
				"enum": []string{"Puck", "Charon", "Kore", "Fenrir", "Aoede"},
			},
		},
		Required: []string{"voice_name"},
	}, a.handleExternal("voice"))

	a.RegisterTool(swarm.ToolDefinition{
		Name:        "open_app",
		Description: "Abrir aplicativo no celular",
		Parameters: map[string]interface{}{
			"app_name": map[string]interface{}{"type": "string", "description": "Nome do app"},
		},
		Required: []string{"app_name"},
	}, a.handleExternal("app"))
}

func (a *Agent) handleExternal(service string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🔌 [EXTERNAL:%s] %s userID=%d", service, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Executando %s via %s", call.Name, service),
			Data: map[string]interface{}{
				"action":  call.Name,
				"service": service,
				"args":    call.Args,
				"user_id": call.UserID,
			},
		}, nil
	}
}
