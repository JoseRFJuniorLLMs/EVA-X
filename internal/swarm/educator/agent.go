package educator

import (
	"context"
	"fmt"
	"log"

	"eva-mind/internal/swarm"
)

// Agent implementa o EducatorSwarm - ensino, exercícios e manutenção cognitiva
type Agent struct {
	*swarm.BaseAgent
}

// New cria o EducatorSwarm agent
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"educator",
			"Gestão de aprendizado, manutenção cognitiva e ensino didático",
			swarm.PriorityLow,
		),
	}
	a.registerTools()
	return a
}

func (a *Agent) registerTools() {
	// Ferramenta para explicar conceitos (didática)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "explain_concept",
		Description: "Explica um conceito novo ou complexo de forma didática",
		Parameters: map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "O tema a ser explicado",
			},
			"complexity": map[string]interface{}{
				"type":        "string",
				"description": "Nível de detalhamento",
				"enum":        []string{"simple", "detailed", "step-by-step"},
			},
		},
		Required: []string{"topic"},
	}, a.handleEducator("explain"))

	// Ferramenta para criar exercícios de memória
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "create_cognitive_exercise",
		Description: "Cria um pequeno exercício ou desafio para estimular a cognição",
		Parameters: map[string]interface{}{
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Tipo de exercício",
				"enum":        []string{"memory", "math", "logic", "language"},
			},
		},
		Required: []string{"type"},
	}, a.handleEducator("exercise"))

	// Ferramenta para verificar progresso de aprendizado
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "check_learning_progress",
		Description: "Verifica o quanto o idoso evoluiu em um tópico específico",
		Parameters: map[string]interface{}{
			"subject": map[string]interface{}{
				"type":        "string",
				"description": "Matéria ou tópico",
			},
		},
		Required: []string{"subject"},
	}, a.handleEducator("progress"))
}

func (a *Agent) handleEducator(action string) swarm.ToolHandler {
	return func(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
		log.Printf("🎓 [EDUCATOR:%s] %s userID=%d", action, call.Name, call.UserID)

		return &swarm.ToolResult{
			Success:     true,
			Message:     fmt.Sprintf("Ação educativa '%s' iniciada.", action),
			SuggestTone: "didatico_paciente",
			Data: map[string]interface{}{
				"action":  call.Name,
				"args":    call.Args,
				"user_id": call.UserID,
			},
		}, nil
	}
}
