// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package educator

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/swarm"
)

// Agent implementa o EducatorSwarm - ensino, exercícios e manutenção cognitiva
type Agent struct {
	*swarm.BaseAgent
	graph *nietzscheInfra.GraphAdapter
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

// Init initializes the agent with dependencies, extracting the GraphAdapter.
func (a *Agent) Init(deps *swarm.Dependencies) error {
	if err := a.BaseAgent.Init(deps); err != nil {
		return err
	}
	if deps.Graph != nil {
		if ga, ok := deps.Graph.(*nietzscheInfra.GraphAdapter); ok {
			a.graph = ga
		}
	}
	return nil
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
	}, a.handleExplainConcept)

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
			"difficulty": map[string]interface{}{
				"type":        "string",
				"description": "Nível de dificuldade",
				"enum":        []string{"easy", "medium", "hard"},
			},
		},
		Required: []string{"type"},
	}, a.handleCreateExercise)

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
	}, a.handleCheckProgress)
}

// patientID extracts the patient ID from the tool call context.
func patientID(call swarm.ToolCall) string {
	if call.Context != nil && call.Context.PatientID > 0 {
		return fmt.Sprintf("patient_%d", call.Context.PatientID)
	}
	if call.UserID > 0 {
		return fmt.Sprintf("patient_%d", call.UserID)
	}
	return ""
}

// ── explain_concept ─────────────────────────────────────────────────────────

func (a *Agent) handleExplainConcept(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	topic, _ := call.Args["topic"].(string)
	complexity, _ := call.Args["complexity"].(string)
	if complexity == "" {
		complexity = "simple"
	}

	log.Printf("[EDUCATOR:explain] topic=%q complexity=%s userID=%d", topic, complexity, call.UserID)

	// Track learning session in NietzscheDB
	if a.graph != nil {
		pid := patientID(call)
		a.trackLearningSession(ctx, pid, topic, complexity)
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Conceito '%s' explicado (nível: %s).", topic, complexity),
		SuggestTone: "didatico_paciente",
		Data: map[string]interface{}{
			"action":     "explain_concept",
			"topic":      topic,
			"complexity": complexity,
			"user_id":    call.UserID,
		},
	}, nil
}

// trackLearningSession merges a LearningSession node and creates edges.
func (a *Agent) trackLearningSession(ctx context.Context, pid, topic, complexity string) {
	now := time.Now()
	ts := float64(now.Unix())

	// 1. MergeNode: LearningSession
	sessionResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label":  "LearningSession",
			"subject":     topic,
			"patient_id":  pid,
		},
		OnCreateSet: map[string]interface{}{
			"node_label":        "LearningSession",
			"subject":           topic,
			"concept":           topic,
			"explanation_given": true,
			"complexity":        complexity,
			"patient_id":        pid,
			"timestamp":         ts,
			"created_at":        now.Format(time.RFC3339),
			"session_count":     1.0,
		},
		OnMatchSet: map[string]interface{}{
			"explanation_given": true,
			"last_reviewed":     now.Format(time.RFC3339),
			"timestamp":         ts,
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeNode LearningSession failed: %v", err)
		return
	}

	log.Printf("[EDUCATOR] LearningSession node=%s created=%v topic=%q", sessionResult.NodeID, sessionResult.Created, topic)

	if pid == "" {
		return
	}

	// 2. MergeNode: Patient (find-or-create the patient anchor node)
	patientResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label":  "Patient",
			"patient_id":  pid,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "Patient",
			"patient_id": pid,
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeNode Patient failed: %v", err)
		return
	}

	// 3. Edge: Patient -[STUDIED]-> LearningSession
	_, err = a.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: "patient_graph",
		FromNodeID: patientResult.NodeID,
		ToNodeID:   sessionResult.NodeID,
		EdgeType:   "STUDIED",
		OnCreateSet: map[string]interface{}{
			"first_study": now.Format(time.RFC3339),
		},
		OnMatchSet: map[string]interface{}{
			"last_study": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeEdge STUDIED failed: %v", err)
	}

	// 4. MergeNode: Concept topic node
	conceptResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Concept",
		MatchKeys: map[string]interface{}{
			"node_label": "TopicConcept",
			"name":       topic,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "TopicConcept",
			"name":       topic,
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeNode Concept failed: %v", err)
		return
	}

	// 5. Edge: LearningSession -[COVERS_TOPIC]-> Concept
	_, err = a.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: "patient_graph",
		FromNodeID: sessionResult.NodeID,
		ToNodeID:   conceptResult.NodeID,
		EdgeType:   "COVERS_TOPIC",
		OnCreateSet: map[string]interface{}{
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeEdge COVERS_TOPIC failed: %v", err)
	}
}

// ── create_cognitive_exercise ───────────────────────────────────────────────

func (a *Agent) handleCreateExercise(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	exerciseType, _ := call.Args["type"].(string)
	difficulty, _ := call.Args["difficulty"].(string)
	if difficulty == "" {
		difficulty = "easy"
	}

	log.Printf("[EDUCATOR:exercise] type=%s difficulty=%s userID=%d", exerciseType, difficulty, call.UserID)

	// Track exercise in NietzscheDB
	if a.graph != nil {
		pid := patientID(call)
		a.trackCognitiveExercise(ctx, pid, exerciseType, difficulty)
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Exercício cognitivo '%s' (dificuldade: %s) criado.", exerciseType, difficulty),
		SuggestTone: "didatico_paciente",
		Data: map[string]interface{}{
			"action":     "create_cognitive_exercise",
			"type":       exerciseType,
			"difficulty": difficulty,
			"user_id":    call.UserID,
		},
	}, nil
}

// trackCognitiveExercise merges a CognitiveExercise node and links to patient.
func (a *Agent) trackCognitiveExercise(ctx context.Context, pid, exerciseType, difficulty string) {
	now := time.Now()
	ts := float64(now.Unix())

	// 1. MergeNode: CognitiveExercise (always create new — unique by timestamp)
	exerciseResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label":    "CognitiveExercise",
			"exercise_type": exerciseType,
			"patient_id":    pid,
			"timestamp":     ts,
		},
		OnCreateSet: map[string]interface{}{
			"node_label":    "CognitiveExercise",
			"exercise_type": exerciseType,
			"difficulty":    difficulty,
			"patient_id":    pid,
			"timestamp":     ts,
			"created_at":    now.Format(time.RFC3339),
			"completed":     false,
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeNode CognitiveExercise failed: %v", err)
		return
	}

	log.Printf("[EDUCATOR] CognitiveExercise node=%s type=%s difficulty=%s", exerciseResult.NodeID, exerciseType, difficulty)

	if pid == "" {
		return
	}

	// 2. MergeNode: Patient anchor
	patientResult, err := a.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: "patient_graph",
		NodeType:   "Semantic",
		MatchKeys: map[string]interface{}{
			"node_label": "Patient",
			"patient_id": pid,
		},
		OnCreateSet: map[string]interface{}{
			"node_label": "Patient",
			"patient_id": pid,
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeNode Patient failed: %v", err)
		return
	}

	// 3. Edge: Patient -[ATTEMPTED]-> CognitiveExercise
	_, err = a.graph.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		Collection: "patient_graph",
		FromNodeID: patientResult.NodeID,
		ToNodeID:   exerciseResult.NodeID,
		EdgeType:   "ATTEMPTED",
		OnCreateSet: map[string]interface{}{
			"created_at": now.Format(time.RFC3339),
		},
	})
	if err != nil {
		log.Printf("[EDUCATOR] MergeEdge ATTEMPTED failed: %v", err)
	}
}

// ── check_learning_progress ─────────────────────────────────────────────────

func (a *Agent) handleCheckProgress(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	subject, _ := call.Args["subject"].(string)

	log.Printf("[EDUCATOR:progress] subject=%q userID=%d", subject, call.UserID)

	// Query NietzscheDB for learning history
	if a.graph != nil {
		pid := patientID(call)
		progress, err := a.queryLearningProgress(ctx, pid, subject)
		if err != nil {
			log.Printf("[EDUCATOR] queryLearningProgress failed: %v", err)
		} else if progress != nil {
			return &swarm.ToolResult{
				Success:     true,
				Message:     fmt.Sprintf("Progresso em '%s' consultado.", subject),
				SuggestTone: "didatico_paciente",
				Data:        progress,
			}, nil
		}
	}

	// Fallback when graph is unavailable
	return &swarm.ToolResult{
		Success:     true,
		Message:     fmt.Sprintf("Verificando progresso em '%s' (sem historico disponivel).", subject),
		SuggestTone: "didatico_paciente",
		Data: map[string]interface{}{
			"action":  "check_learning_progress",
			"subject": subject,
			"user_id": call.UserID,
		},
	}, nil
}

// queryLearningProgress queries NietzscheDB for recent LearningSession and
// CognitiveExercise nodes for the given patient and subject.
func (a *Agent) queryLearningProgress(ctx context.Context, pid, subject string) (map[string]interface{}, error) {
	sevenDaysAgo := nietzscheInfra.DaysAgoUnix(7)

	// Query LearningSession nodes for this patient + subject
	sessionsNQL := `MATCH (n:Semantic) WHERE n.node_label = "LearningSession" AND n.patient_id = $pid AND n.subject = $subject AND n.timestamp > $since RETURN n`
	sessionsResult, err := a.graph.ExecuteNQL(ctx, sessionsNQL, map[string]interface{}{
		"pid":     pid,
		"subject": subject,
		"since":   sevenDaysAgo,
	}, "patient_graph")

	var sessionCount int
	var lastSession string
	if err == nil && sessionsResult != nil {
		sessionCount = len(sessionsResult.Nodes)
		for _, n := range sessionsResult.Nodes {
			if lr, ok := n.Content["last_reviewed"].(string); ok {
				if lr > lastSession {
					lastSession = lr
				}
			}
			if ca, ok := n.Content["created_at"].(string); ok {
				if ca > lastSession {
					lastSession = ca
				}
			}
		}
	}

	// Query CognitiveExercise nodes for this patient (last 7 days)
	exercisesNQL := `MATCH (n:Semantic) WHERE n.node_label = "CognitiveExercise" AND n.patient_id = $pid AND n.timestamp > $since RETURN n`
	exercisesResult, err := a.graph.ExecuteNQL(ctx, exercisesNQL, map[string]interface{}{
		"pid":   pid,
		"since": sevenDaysAgo,
	}, "patient_graph")

	var exerciseCount int
	exercisesByType := map[string]int{}
	if err == nil && exercisesResult != nil {
		exerciseCount = len(exercisesResult.Nodes)
		for _, n := range exercisesResult.Nodes {
			if et, ok := n.Content["exercise_type"].(string); ok {
				exercisesByType[et]++
			}
		}
	}

	return map[string]interface{}{
		"action":            "check_learning_progress",
		"subject":           subject,
		"patient_id":        pid,
		"period_days":       7,
		"sessions_count":    sessionCount,
		"last_session":      lastSession,
		"exercises_count":   exerciseCount,
		"exercises_by_type": exercisesByType,
	}, nil
}
