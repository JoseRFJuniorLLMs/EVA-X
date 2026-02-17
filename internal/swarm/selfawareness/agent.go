// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfawareness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	svc "eva-mind/internal/cortex/selfawareness"
	"eva-mind/internal/swarm"

	"github.com/rs/zerolog/log"
)

// Agent implements the Self-Awareness Swarm — EVA's introspection capabilities.
type Agent struct {
	*swarm.BaseAgent
	svc *svc.SelfAwarenessService
}

// New creates the Self-Awareness swarm agent.
func New() *Agent {
	a := &Agent{
		BaseAgent: swarm.NewBaseAgent(
			"selfawareness",
			"Autoconhecimento — EVA consulta seu codigo, bancos e memorias",
			swarm.PriorityMedium,
		),
	}
	a.registerTools()
	return a
}

// SetService injects the SelfAwarenessService (called from main.go).
func (a *Agent) SetService(s *svc.SelfAwarenessService) {
	a.svc = s
}

func (a *Agent) registerTools() {
	// 1. search_my_code — Semantic search on EVA's source code
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_my_code",
		Description: "Busca semantica no codigo-fonte da EVA (arquivos .go indexados)",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "O que buscar no codigo (ex: 'sistema de memoria', 'handler de voz')",
			},
		},
		Required: []string{"query"},
	}, a.handleSearchCode)

	// 2. query_my_database — Read-only PostgreSQL queries
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "query_my_database",
		Description: "Consulta read-only nas tabelas internas da EVA (SELECT only)",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Query SQL SELECT (ex: 'SELECT * FROM eva_curriculum LIMIT 5')",
			},
		},
		Required: []string{"query"},
	}, a.handleQueryDatabase)

	// 3. list_my_collections — List Qdrant collections
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "list_my_collections",
		Description: "Lista todas as colecoes de memoria vetorial da EVA com contagem de pontos",
		Parameters:  map[string]interface{}{},
	}, a.handleListCollections)

	// 4. system_stats — System statistics
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "system_stats",
		Description: "Estatisticas dos sistemas da EVA: bancos, memorias, runtime",
		Parameters:  map[string]interface{}{},
	}, a.handleSystemStats)

	// 5. update_self_knowledge — Update EVA's self-knowledge
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "update_self_knowledge",
		Description: "Atualiza o conhecimento da EVA sobre si mesma",
		Parameters: map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Chave unica do conhecimento (ex: 'module:brainstem', 'concept:lacan')",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Titulo do conhecimento",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Resumo curto",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Conteudo detalhado",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Tipo: module, concept, database, api, architecture",
				"enum":        []string{"module", "concept", "database", "api", "architecture", "memory_system", "tool", "agent"},
			},
		},
		Required: []string{"key", "title", "content"},
	}, a.handleUpdateSelfKnowledge)

	// 6. search_self_knowledge — Search EVA's self-knowledge
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_self_knowledge",
		Description: "Busca no conhecimento interno da EVA sobre si mesma",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "O que buscar (ex: 'memoria', 'lacan', 'banco de dados')",
			},
		},
		Required: []string{"query"},
	}, a.handleSearchSelfKnowledge)

	// 7. introspect — Full self-report
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "introspect",
		Description: "Retorna estado completo da EVA: personalidade, memorias, stats, colecoes",
		Parameters:  map[string]interface{}{},
	}, a.handleIntrospect)

	// 8. search_my_docs — Semantic search on EVA's architecture documentation (.md files)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "search_my_docs",
		Description: "Busca semantica na documentacao de arquitetura da EVA (arquivos .md)",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "O que buscar na documentacao (ex: 'fase de implementacao', 'arquitetura gemini', 'voice recognition')",
			},
		},
		Required: []string{"query"},
	}, a.handleSearchDocs)
}

// --- Tool Handlers ---

func (a *Agent) handleSearchCode(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "Query nao informada"}, nil
	}
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	log.Info().Str("query", query).Msg("[SELF-AWARE] Searching codebase")

	results, err := a.svc.SearchCode(ctx, query, 5)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro na busca: %v", err)}, nil
	}

	if len(results) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Nao encontrei nada no meu codigo sobre '%s'. Talvez o codebase ainda nao foi indexado.", query),
		}, nil
	}

	var lines []string
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("- %s (pkg: %s, score: %.2f): %s", r.FilePath, r.Package, r.Score, truncate(r.Summary, 200)))
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Encontrei %d resultados no meu codigo sobre '%s':\n%s", len(results), query, strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"results": results, "count": len(results)},
	}, nil
}

func (a *Agent) handleQueryDatabase(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "Query nao informada"}, nil
	}
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	log.Info().Str("query", query).Msg("[SELF-AWARE] Querying PostgreSQL")

	rows, err := a.svc.QueryPostgres(ctx, query)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro: %v", err)}, nil
	}

	if len(rows) == 0 {
		return &swarm.ToolResult{Success: true, Message: "Query retornou 0 resultados."}, nil
	}

	// Format results
	jsonBytes, _ := json.MarshalIndent(rows, "", "  ")
	msg := fmt.Sprintf("Query retornou %d resultados:\n%s", len(rows), string(jsonBytes))
	if len(msg) > 2000 {
		msg = msg[:2000] + "...(truncado)"
	}

	return &swarm.ToolResult{
		Success: true,
		Message: msg,
		Data:    map[string]interface{}{"rows": rows, "count": len(rows)},
	}, nil
}

func (a *Agent) handleListCollections(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	collections, err := a.svc.ListCollections(ctx)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro: %v", err)}, nil
	}

	var lines []string
	totalPoints := int64(0)
	for _, c := range collections {
		lines = append(lines, fmt.Sprintf("- %s: %d pontos", c.Name, c.PointCount))
		totalPoints += c.PointCount
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Minhas %d colecoes vetoriais (%d pontos total):\n%s", len(collections), totalPoints, strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"collections": collections, "total_points": totalPoints},
	}, nil
}

func (a *Agent) handleSystemStats(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	stats, err := a.svc.GetSystemStats(ctx)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro: %v", err)}, nil
	}

	msg := fmt.Sprintf(`Meus sistemas:
- PostgreSQL: %d tabelas, %d memorias episodicas
- Qdrant: %d colecoes, %d pontos vetoriais
- Curriculum: %d pendentes, %d completados
- Runtime: %d goroutines, %dMB RAM, uptime %s`,
		stats.PostgresTables, stats.TotalMemories,
		stats.QdrantCollections, stats.QdrantTotalPoints,
		stats.CurriculumPending, stats.CurriculumDone,
		stats.GoRoutines, stats.MemAllocMB, stats.Uptime)

	return &swarm.ToolResult{
		Success: true,
		Message: msg,
		Data:    map[string]interface{}{"stats": stats},
	}, nil
}

func (a *Agent) handleUpdateSelfKnowledge(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	key, _ := call.Args["key"].(string)
	title, _ := call.Args["title"].(string)
	content, _ := call.Args["content"].(string)
	summary, _ := call.Args["summary"].(string)
	knowledgeType, _ := call.Args["type"].(string)

	if key == "" || title == "" || content == "" {
		return &swarm.ToolResult{Success: false, Message: "key, title e content sao obrigatorios"}, nil
	}
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}
	if knowledgeType == "" {
		knowledgeType = "concept"
	}
	if summary == "" {
		summary = truncate(content, 200)
	}

	log.Info().Str("key", key).Str("title", title).Msg("[SELF-AWARE] Updating self-knowledge")

	err := a.svc.UpdateSelfKnowledge(ctx, knowledgeType, key, title, summary, content, 5)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro ao atualizar: %v", err)}, nil
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Conhecimento atualizado: '%s' (%s)", title, key),
	}, nil
}

func (a *Agent) handleSearchSelfKnowledge(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "Query nao informada"}, nil
	}
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	items, err := a.svc.SearchSelfKnowledge(ctx, query, 5)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro: %v", err)}, nil
	}

	if len(items) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Nao encontrei nada sobre '%s' no meu conhecimento interno.", query),
		}, nil
	}

	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", item.Type, item.Title, truncate(item.Summary, 150)))
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Encontrei %d resultados sobre '%s':\n%s", len(items), query, strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"items": items, "count": len(items)},
	}, nil
}

func (a *Agent) handleIntrospect(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	report, err := a.svc.Introspect(ctx)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro: %v", err)}, nil
	}

	var msg strings.Builder
	msg.WriteString("Meu estado atual:\n")

	if report.Stats != nil {
		msg.WriteString(fmt.Sprintf("\nSistemas: %d tabelas PostgreSQL, %d colecoes Qdrant (%d pontos)\n",
			report.Stats.PostgresTables, report.Stats.QdrantCollections, report.Stats.QdrantTotalPoints))
		msg.WriteString(fmt.Sprintf("Memorias: %d episodicas | Curriculum: %d pendentes, %d completados\n",
			report.Stats.TotalMemories, report.Stats.CurriculumPending, report.Stats.CurriculumDone))
		msg.WriteString(fmt.Sprintf("Runtime: %d goroutines, %dMB RAM, uptime %s\n",
			report.Stats.GoRoutines, report.Stats.MemAllocMB, report.Stats.Uptime))
	}

	if len(report.RecentLearnings) > 0 {
		msg.WriteString("\nAprendizados recentes: " + strings.Join(report.RecentLearnings, ", "))
	}

	return &swarm.ToolResult{
		Success:     true,
		Message:     msg.String(),
		SuggestTone: "introspectivo_consciente",
		Data:        map[string]interface{}{"report": report},
	}, nil
}

func (a *Agent) handleSearchDocs(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "Query nao informada"}, nil
	}
	if a.svc == nil {
		return &swarm.ToolResult{Success: false, Message: "Service nao inicializado"}, nil
	}

	log.Info().Str("query", query).Msg("[SELF-AWARE] Searching docs")

	results, err := a.svc.SearchDocs(ctx, query, 5)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro na busca: %v", err)}, nil
	}

	if len(results) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("Nao encontrei nada na minha documentacao sobre '%s'.", query),
		}, nil
	}

	var lines []string
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("- %s [%s] (score: %.2f): %s", r.Title, r.FilePath, r.Score, truncate(r.Content, 300)))
	}

	return &swarm.ToolResult{
		Success: true,
		Message: fmt.Sprintf("Encontrei %d resultados na minha documentacao sobre '%s':\n%s", len(results), query, strings.Join(lines, "\n")),
		Data:    map[string]interface{}{"results": results, "count": len(results)},
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
