// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfawareness

import (
	"context"
	"encoding/json"
		"sync"
	"fmt"
	"sort"
	"strings"
	"time"

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

	// 13. recall_memory — Automatic memory recall (RAG per turn)
	a.RegisterTool(swarm.ToolDefinition{
		Name:        "recall_memory",
		Description: "Busca nas minhas memorias e no perfil do utilizador por informacoes relevantes ao que foi dito. IMPORTANTE: Chame esta ferramenta AUTOMATICAMENTE sempre que o utilizador mencionar algo do passado, perguntar se voce lembra de algo, ou quando contexto historico ajudaria na resposta. Nao espere o utilizador pedir explicitamente.",
		Parameters: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Palavras-chave ou frase para buscar nas memorias (ex: 'medicamento ontem', 'nome da filha', 'ultima consulta')",
			},
		},
		Required: []string{"query"},
	}, a.handleRecallMemory)
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
	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB client nao inicializado"}, nil
	}

	collections, err := a.nietzscheClient.ListCollections(ctx)
	if err != nil {
		return &swarm.ToolResult{Success: false, Message: fmt.Sprintf("Erro ListCollections: %v", err)}, nil
	}

	// Sort by NodeCount desc for top-5
	type colEntry struct {
		name  string
		nodes uint64
		edges uint64
	}
	entries := make([]colEntry, 0, len(collections))
	totalNodes := uint64(0)
	totalEdges := uint64(0)
	for _, c := range collections {
		entries = append(entries, colEntry{name: c.Name, nodes: c.NodeCount, edges: c.EdgeCount})
		totalNodes += c.NodeCount
		totalEdges += c.EdgeCount
	}
	// Sort descending by nodes
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].nodes > entries[i].nodes {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Top 5 + summary
	var top []string
	limit := 5
	if len(entries) < limit {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		top = append(top, fmt.Sprintf("%s: %d nos, %d edges", entries[i].name, entries[i].nodes, entries[i].edges))
	}

	msg := fmt.Sprintf("Tenho %d colecoes com %d nos e %d edges no total. As maiores: %s", len(collections), totalNodes, totalEdges, strings.Join(top, "; "))
	return &swarm.ToolResult{
		Success: true,
		Message: msg,
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

	// --- Self-Observation: persist energy stats summary ---
	var topNames []string
	var relatedIDs []string
	for _, e := range entries {
		label := e.ID
		if e.NodeType != "" {
			label = fmt.Sprintf("%s(%s)", e.NodeType, e.ID[:8])
		}
		topNames = append(topNames, label)
		relatedIDs = append(relatedIDs, e.ID)
	}
	topSummary := strings.Join(topNames, ", ")
	if len(topSummary) > 300 {
		topSummary = topSummary[:300] + "..."
	}
	var avgEnergy float32
	for _, e := range entries {
		avgEnergy += e.Energy
	}
	if len(entries) > 0 {
		avgEnergy /= float32(len(entries))
	}
	observationSummary := fmt.Sprintf("Self-observation: top %d concepts are %s. PageRank ran in %dms (%d iterations, %d total nodes). Avg top energy: %.3f",
		limit, topSummary, pr.DurationMs, pr.Iterations, len(scores), avgEnergy)
	go a.storeSelfObservation(ctx, "energy_stats", observationSummary, map[string]interface{}{
		"top_count":      limit,
		"total_nodes":    len(scores),
		"pagerank_ms":    pr.DurationMs,
		"iterations":     pr.Iterations,
		"avg_top_energy": avgEnergy,
		"collection":     collection,
	}, relatedIDs)

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

	// --- Self-Observation: persist evolution report ---
	observationSummary := fmt.Sprintf("Evolution report: energy changed from %.3f to %.3f (delta: %.3f). %d nodes updated, %d echoes created, %d elite nodes (threshold %.3f). %d cycles in %dms.",
		result.MeanEnergyBefore, result.MeanEnergyAfter, result.TotalEnergyDelta,
		result.NodesUpdated, result.EchoesCreated, result.EliteCount, result.EliteThreshold,
		result.CyclesRun, result.DurationMs)
	go a.storeSelfObservation(ctx, "evolution", observationSummary, map[string]interface{}{
		"cycles_run":         result.CyclesRun,
		"duration_ms":        result.DurationMs,
		"nodes_updated":      result.NodesUpdated,
		"mean_energy_before": result.MeanEnergyBefore,
		"mean_energy_after":  result.MeanEnergyAfter,
		"total_energy_delta": result.TotalEnergyDelta,
		"echoes_created":     result.EchoesCreated,
		"echoes_evicted":     result.EchoesEvicted,
		"elite_count":        result.EliteCount,
		"elite_threshold":    result.EliteThreshold,
		"collection":         collection,
	}, result.EliteNodeIDs)

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

	// --- Self-Observation: persist topology summary ---
	observationSummary := fmt.Sprintf("Self-observation: %d communities detected (modularity: %.4f, largest: %d nodes). Mean depth: %.3f (%d nodes measured). Depth distribution: shallow=%d, medium=%d, deep=%d, abyss=%d. Graph: %v nodes, %v edges.",
		louvain.CommunityCount, louvain.Modularity, louvain.LargestSize,
		meanDepth, depthCount,
		depthBuckets["shallow (0-0.3)"], depthBuckets["medium (0.3-0.6)"],
		depthBuckets["deep (0.6-0.9)"], depthBuckets["abyss (0.9+)"],
		stats["node_count"], stats["edge_count"])
	go a.storeSelfObservation(ctx, "topology", observationSummary, map[string]interface{}{
		"community_count": louvain.CommunityCount,
		"modularity":      louvain.Modularity,
		"largest_size":    louvain.LargestSize,
		"mean_depth":      meanDepth,
		"depth_buckets":   depthBuckets,
		"node_count":      stats["node_count"],
		"edge_count":      stats["edge_count"],
		"louvain_ms":      louvain.DurationMs,
		"collection":      collection,
	}, nil)

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

// storeSelfObservation persists a SelfObservation node into eva_core and
// optionally links it to related concept node IDs via SELF_OBSERVED edges.
// This is fire-and-forget: errors are logged but do not block the caller.
func (a *Agent) storeSelfObservation(ctx context.Context, observationType, summary string, metrics map[string]interface{}, relatedNodeIDs []string) {
	if a.nietzscheClient == nil {
		return
	}

	collection := "eva_core"
	ts := time.Now().UTC().Format(time.RFC3339)

	content := map[string]interface{}{
		"node_label":       "SelfObservation",
		"observation_type": observationType,
		"summary":          summary,
		"timestamp":        ts,
	}
	for k, v := range metrics {
		content[k] = v
	}

	// 128-dim zero coords for relational data (eva_core is 128D poincare)
	coords := make([]float64, 128)
	// Shallow depth for self-observations (magnitude ~0.15)
	coords[0] = 0.15

	result, err := a.nietzscheClient.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Coords:     coords,
		Content:    content,
		NodeType:   "Semantic",
		Energy:     0.6,
		Collection: collection,
	})
	if err != nil {
		log.Warn().Err(err).Str("type", observationType).Msg("[SELF-AWARE] Failed to store self-observation node")
		return
	}

	observationID := result.ID
	log.Info().Str("id", observationID).Str("type", observationType).Msg("[SELF-AWARE] Stored self-observation")

	// Create SELF_OBSERVED edges to related concept nodes (best-effort, limit to 5)
	limit := 5
	if len(relatedNodeIDs) < limit {
		limit = len(relatedNodeIDs)
	}
	for _, nodeID := range relatedNodeIDs[:limit] {
		_, edgeErr := a.nietzscheClient.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:       observationID,
			To:         nodeID,
			EdgeType:   "Association",
			Weight:     0.7,
			Collection: collection,
		})
		if edgeErr != nil {
			log.Warn().Err(edgeErr).Str("from", observationID).Str("to", nodeID).Msg("[SELF-AWARE] Failed to create SELF_OBSERVED edge")
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}


// handleRecallMemory searches across EVA's memory collections for relevant context.
// This is the RAG mechanism for voice sessions - Gemini calls it automatically.
// OPTIMIZED: 1.5s timeout, 1 result per collection, parallel GetNode.
func (a *Agent) handleRecallMemory(ctx context.Context, call swarm.ToolCall) (*swarm.ToolResult, error) {
	query, _ := call.Args["query"].(string)
	if query == "" {
		return &swarm.ToolResult{Success: false, Message: "query is required"}, nil
	}

	if a.nietzscheClient == nil {
		return &swarm.ToolResult{Success: false, Message: "NietzscheDB not available"}, nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	type memHit struct {
		text string
	}

	hitCh := make(chan memHit, 6) // max 2 per collection x 3
	var wg sync.WaitGroup

	collections := []string{"eva_mind"}  // Single collection for speed (<200ms)
	for _, col := range collections {
		wg.Add(1)
		go func(collection string) {
			defer wg.Done()
			// FullTextSearchRich: FTS + parallel GetNode in single SDK call
			richResults, err := a.nietzscheClient.FullTextSearchRich(searchCtx, query, collection, 2)
			if err != nil || len(richResults) == 0 {
				return
			}
			for _, r := range richResults {
				if !r.Found {
					continue
				}
				text := extractRecallContent(r.Content, collection)
				if text != "" {
					hitCh <- memHit{text: text}
				}
			}
		}(col)
	}

	// Close channel when all goroutines finish
	go func() {
		wg.Wait()
		close(hitCh)
	}()

	// Collect results with dedup
	seen := make(map[string]bool)
	var unique []string
	for hit := range hitCh {
		key := hit.text
		if len(key) > 80 {
			key = key[:80]
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, hit.text)
		}
		if len(unique) >= 3 {
			break
		}
	}

	if len(unique) == 0 {
		return &swarm.ToolResult{
			Success: true,
			Message: "Nao encontrei memorias relevantes sobre isso.",
		}, nil
	}

	msg := fmt.Sprintf("Memorias relevantes:\n%s", strings.Join(unique, "\n"))
	return &swarm.ToolResult{
		Success: true,
		Message: msg,
	}, nil
}

// extractRecallContent extracts readable text from FTS result content.
func extractRecallContent(content map[string]interface{}, collection string) string {
	for _, key := range []string{"content", "text", "summary", "description", "self_description"} {
		if v, ok := content[key]; ok {
			if s, ok := v.(string); ok && len(s) > 10 {
				if len(s) > 200 {
					s = s[:197] + "..."
				}
				return s
			}
		}
	}
	label, _ := content["node_label"].(string)
	name, _ := content["name"].(string)
	if label != "" && name != "" {
		return fmt.Sprintf("[%s] %s: %s", collection, label, name)
	}
	return ""
}
