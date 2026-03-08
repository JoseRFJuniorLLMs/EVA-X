// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfawareness

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	svc "eva/internal/cortex/selfawareness"
	"eva/internal/swarm"

	nietzsche "nietzsche-sdk"

	"github.com/rs/zerolog/log"
)

// Agent implements the Self-Awareness Swarm — EVA's introspection capabilities.
type Agent struct {
	*swarm.BaseAgent
	svc             *svc.SelfAwarenessService
	nietzscheClient *nietzscheInfra.Client
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

// SetNietzscheClient injects the NietzscheDB gRPC client for NQL tools (called from main.go).
func (a *Agent) SetNietzscheClient(client *nietzscheInfra.Client) {
	a.nietzscheClient = client
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

	// 2. query_my_database — Read-only NietzscheDB queries
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

	// 3. list_my_collections — List NietzscheDB vector collections
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

	// 9. query_my_graph — NQL native read-only queries against NietzscheDB
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "query_my_graph",
		Description: "Consulta NQL (Nietzsche Query Language) read-only no grafo de memorias da EVA. Ex: 'MATCH (n:Condition) WHERE n.energy > 0.5 RETURN n LIMIT 20'",
		Parameters: map[string]interface{}{
			"nql": map[string]interface{}{
				"type":        "string",
				"description": "Query NQL read-only (MATCH, RETURN, WHERE — sem DELETE/DROP/CREATE)",
			},
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Colecao alvo (ex: 'eva_core', 'patient_graph', 'memories'). Padrao: 'eva_core'",
			},
		},
		Required: []string{"nql"},
	}, a.handleQueryMyGraph)

	// 10. my_energy_stats — Energy map (PageRank + top-20 by energy)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "my_energy_stats",
		Description: "Mapa energetico: PageRank + top-20 nos mais energeticos da EVA",
		Parameters: map[string]interface{}{
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Colecao alvo (padrao: 'eva_core')",
			},
		},
	}, a.handleMyEnergyStats)

	// 11. invoke_zaratustra — Autonomous evolution engine
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "invoke_zaratustra",
		Description: "Invoca o motor de evolucao autonoma Zaratustra (Will to Power + Eternal Recurrence + Ubermensch). CUIDADO: modifica o grafo",
		Parameters: map[string]interface{}{
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Colecao alvo (padrao: 'eva_core')",
			},
			"cycles": map[string]interface{}{
				"type":        "number",
				"description": "Numero de ciclos completos (padrao: 1)",
			},
		},
	}, a.handleInvokeZaratustra)

	// 12. my_topology — Topological map (Louvain communities + depth distribution + stats)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "my_topology",
		Description: "Mapa topologico: comunidades Louvain, distribuicao de profundidade e estatisticas globais do grafo",
		Parameters: map[string]interface{}{
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Colecao alvo (padrao: 'eva_core')",
			},
		},
	}, a.handleMyTopology)
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

	log.Info().Str("query", query).Msg("[SELF-AWARE] Querying NietzscheDB")

	rows, err := a.svc.QueryNietzsche(ctx, query)
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
- NietzscheDB: %d labels, %d memorias episodicas
- NietzscheDB: %d colecoes, %d nos vetoriais
- Curriculum: %d pendentes, %d completados
- Runtime: %d goroutines, %dMB RAM, uptime %s`,
		stats.NietzscheLabels, stats.TotalMemories,
		stats.NietzscheCollections, stats.NietzscheTotalNodes,
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
		msg.WriteString(fmt.Sprintf("\nSistemas: %d labels NietzscheDB, %d colecoes NietzscheDB (%d nos)\n",
			report.Stats.NietzscheLabels, report.Stats.NietzscheCollections, report.Stats.NietzscheTotalNodes))
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

// --- NQL Tools (9-12) ---

// isDestructiveNQL checks if an NQL string contains destructive operations.
// Returns the offending keyword if found, or empty string if safe.
func isDestructiveNQL(nql string) string {
	upper := strings.ToUpper(nql)
	// Tokenise on whitespace + punctuation to avoid false positives inside identifiers.
	// A simple word-boundary check: ensure the keyword is preceded/followed by a
	// non-alpha character (or start/end of string).
	for _, kw := range []string{"DELETE", "DROP", "CREATE", "DETACH", "REMOVE", "SET ", "MERGE"} {
		trimmed := strings.TrimSpace(kw)
		idx := strings.Index(upper, trimmed)
		if idx < 0 {
			continue
		}
		// Check left boundary
		if idx > 0 {
			ch := upper[idx-1]
			if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				continue
			}
		}
		// Check right boundary
		end := idx + len(trimmed)
		if end < len(upper) {
			ch := upper[end]
			if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				continue
			}
		}
		return trimmed
	}
	return ""
}

func (a *Agent) handleQueryMyGraph(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	nql, _ := call.Args["nql"].(string)
	if nql == "" {
		return &swarm.ToolResult{Success: false, Message: "NQL query nao informada"}, nil
	}
	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB client nao inicializado"}, nil
	}

	// Safety: reject destructive NQL
	if kw := isDestructiveNQL(nql); kw != "" {
		return &swarm.ToolResult{
			Success: false,
			Message: fmt.Sprintf("Query rejeitada: operacao '%s' nao permitida (read-only apenas)", kw),
		}, nil
	}

	collection, _ := call.Args["collection"].(string)
	if collection == "" {
		collection = "eva_core"
	}

	log.Info().Str("nql", nql).Str("collection", collection).Msg("[SELF-AWARE] NQL query_my_graph")

	result, err := a.nietzscheClient.Query(ctx, nql, nil, collection)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro NQL: %v", err)}, nil
	}
	if result.Error != "" {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("NQL error: %s", result.Error)}, nil
	}

	// Format nodes
	var lines []string
	for _, n := range result.Nodes {
		lines = append(lines, fmt.Sprintf("- [%s] %s (energy: %.2f, depth: %.2f)", n.NodeType, n.ID, n.Energy, n.Depth))
	}
	for _, p := range result.NodePairs {
		lines = append(lines, fmt.Sprintf("- %s -> %s", p.From.ID, p.To.ID))
	}
	for _, row := range result.ScalarRows {
		rowJSON, _ := json.Marshal(row)
		lines = append(lines, fmt.Sprintf("- %s", string(rowJSON)))
	}

	if len(lines) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: fmt.Sprintf("NQL retornou 0 resultados em '%s'.", collection),
		}, nil
	}

	msg := fmt.Sprintf("NQL retornou %d resultados em '%s':\n%s",
		len(result.Nodes)+len(result.NodePairs)+len(result.ScalarRows),
		collection, strings.Join(lines, "\n"))
	if len(msg) > 3000 {
		msg = msg[:3000] + "...(truncado)"
	}

	return &swarm.ToolResult{
		Success: true,
		Message: msg,
		Data: map[string]interface{}{
			"nodes":       len(result.Nodes),
			"node_pairs":  len(result.NodePairs),
			"scalar_rows": len(result.ScalarRows),
			"collection":  collection,
		},
	}, nil
}

func (a *Agent) handleMyEnergyStats(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB client nao inicializado"}, nil
	}

	collection, _ := call.Args["collection"].(string)
	if collection == "" {
		collection = "eva_core"
	}

	log.Info().Str("collection", collection).Msg("[SELF-AWARE] my_energy_stats")

	// Run PageRank (damping=0.85, maxIter=20)
	pr, err := a.nietzscheClient.RunPageRank(ctx, collection, 0.85, 20)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro PageRank: %v", err)}, nil
	}

	// Sort by score descending and take top 20
	scores := pr.Scores
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	limit := 20
	if len(scores) < limit {
		limit = len(scores)
	}
	top := scores[:limit]

	// For each top node, fetch full data (type, energy, depth)
	type energyEntry struct {
		ID       string  `json:"id"`
		NodeType string  `json:"node_type"`
		Energy   float32 `json:"energy"`
		Depth    float32 `json:"depth"`
		PageRank float64 `json:"pagerank"`
	}
	var entries []energyEntry
	var lines []string

	for _, ns := range top {
		entry := energyEntry{ID: ns.NodeID, PageRank: ns.Score}
		// Best-effort: fetch node details
		nr, nerr := a.nietzscheClient.GetNode(ctx, ns.NodeID, collection)
		if nerr == nil && nr.Found {
			entry.NodeType = nr.NodeType
			entry.Energy = nr.Energy
			entry.Depth = nr.Depth
		}
		entries = append(entries, entry)
		lines = append(lines, fmt.Sprintf("- %s [%s] energy=%.2f depth=%.2f pagerank=%.4f",
			entry.ID, entry.NodeType, entry.Energy, entry.Depth, entry.PageRank))
	}

	msg := fmt.Sprintf("Meus %d nos mais energeticos (%d total, PageRank em %dms, %d iteracoes):\n%s",
		limit, len(scores), pr.DurationMs, pr.Iterations, strings.Join(lines, "\n"))

	return &swarm.ToolResult{
		Success: true,
		Message: msg,
		Data: map[string]interface{}{
			"top":        entries,
			"total":      len(scores),
			"duration_ms": pr.DurationMs,
			"iterations": pr.Iterations,
			"collection": collection,
		},
	}, nil
}

func (a *Agent) handleInvokeZaratustra(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB client nao inicializado"}, nil
	}

	collection, _ := call.Args["collection"].(string)
	if collection == "" {
		collection = "eva_core"
	}

	var cycles uint32 = 1
	if c, ok := call.Args["cycles"].(float64); ok && c > 0 {
		cycles = uint32(c)
	}

	log.Info().Str("collection", collection).Uint32("cycles", cycles).Msg("[SELF-AWARE] invoke_zaratustra")

	result, err := a.nietzscheClient.InvokeZaratustra(ctx, nietzsche.ZaratustraOpts{
		Collection: collection,
		Cycles:     cycles,
	})
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro Zaratustra: %v", err)}, nil
	}

	msg := fmt.Sprintf(`Evolucao Zaratustra concluida (%d ciclos, %dms):
- Will to Power: %d nos atualizados, energia media %.3f -> %.3f (delta: %.3f)
- Eternal Recurrence: %d echoes criados, %d evicted, %d total
- Ubermensch: %d elite (threshold %.3f), energia elite %.3f vs base %.3f`,
		result.CyclesRun, result.DurationMs,
		result.NodesUpdated, result.MeanEnergyBefore, result.MeanEnergyAfter, result.TotalEnergyDelta,
		result.EchoesCreated, result.EchoesEvicted, result.TotalEchoes,
		result.EliteCount, result.EliteThreshold, result.MeanEliteEnergy, result.MeanBaseEnergy)

	return &swarm.ToolResult{
		Success:     true,
		Message:     msg,
		SuggestTone: "cautious",
		Data: map[string]interface{}{
			"cycles_run":        result.CyclesRun,
			"duration_ms":       result.DurationMs,
			"nodes_updated":     result.NodesUpdated,
			"mean_energy_before": result.MeanEnergyBefore,
			"mean_energy_after":  result.MeanEnergyAfter,
			"elite_count":       result.EliteCount,
			"elite_node_ids":    result.EliteNodeIDs,
			"echoes_created":    result.EchoesCreated,
			"total_echoes":      result.TotalEchoes,
			"collection":        collection,
		},
	}, nil
}

func (a *Agent) handleMyTopology(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB client nao inicializado"}, nil
	}

	collection, _ := call.Args["collection"].(string)
	if collection == "" {
		collection = "eva_core"
	}

	log.Info().Str("collection", collection).Msg("[SELF-AWARE] my_topology")

	// Run Louvain community detection (maxIter=50, resolution=1.0)
	louvain, err := a.nietzscheClient.RunLouvain(ctx, collection, 50, 1.0)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro Louvain: %v", err)}, nil
	}

	// Compute depth distribution from community assignments (fetch node depth)
	depthBuckets := map[string]int{
		"shallow (0-0.3)": 0,
		"medium (0.3-0.6)": 0,
		"deep (0.6-0.9)":   0,
		"abyss (0.9+)":     0,
	}
	var totalDepth float64
	var depthCount int

	for _, nc := range louvain.Assignments {
		nr, nerr := a.nietzscheClient.GetNode(ctx, nc.NodeID, collection)
		if nerr != nil || !nr.Found {
			continue
		}
		d := float64(nr.Depth)
		totalDepth += d
		depthCount++
		switch {
		case d < 0.3:
			depthBuckets["shallow (0-0.3)"]++
		case d < 0.6:
			depthBuckets["medium (0.3-0.6)"]++
		case d < 0.9:
			depthBuckets["deep (0.6-0.9)"]++
		default:
			depthBuckets["abyss (0.9+)"]++
		}
	}

	meanDepth := float64(0)
	if depthCount > 0 {
		meanDepth = totalDepth / float64(depthCount)
	}

	// Get global stats (node_count, edge_count, hausdorff)
	stats, err := a.nietzscheClient.GetStats(ctx)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro GetStats: %v", err)}, nil
	}

	var depthLines []string
	for bucket, count := range depthBuckets {
		if count > 0 {
			depthLines = append(depthLines, fmt.Sprintf("  %s: %d nos", bucket, count))
		}
	}
	sort.Strings(depthLines)

	msg := fmt.Sprintf(`Topologia de '%s':
- Comunidades Louvain: %d (modularidade: %.4f, maior: %d nos)
- Profundidade media: %.3f (%d nos medidos)
- Distribuicao de profundidade:
%s
- Grafo global: %v nos, %v arestas (versao: %v)
- Louvain: %d iteracoes, %dms`,
		collection,
		louvain.CommunityCount, louvain.Modularity, louvain.LargestSize,
		meanDepth, depthCount,
		strings.Join(depthLines, "\n"),
		stats["node_count"], stats["edge_count"], stats["version"],
		louvain.Iterations, louvain.DurationMs)

	return &swarm.ToolResult{
		Success: true,
		Message: msg,
		Data: map[string]interface{}{
			"community_count": louvain.CommunityCount,
			"modularity":      louvain.Modularity,
			"largest_size":    louvain.LargestSize,
			"mean_depth":      meanDepth,
			"depth_buckets":   depthBuckets,
			"stats":           stats,
			"iterations":      louvain.Iterations,
			"duration_ms":     louvain.DurationMs,
			"collection":      collection,
		},
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
