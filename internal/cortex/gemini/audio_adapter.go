// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Limites duros para sessões de áudio Gemini
const (
	AudioPayloadMaxBytes = 1200 // ~300 tokens Gemini
	AudioSummarizeAbove  = 800  // acima disto → summarize
)

// AdaptForAudio adapta tool responses para o Gemini Native Audio.
// Este client é SEMPRE audio (Gemini Live API), então sempre adapta.
// Fluxo: _voice_summary → small payload → deterministic → fallback
func AdaptForAudio(toolName string, result map[string]interface{}) map[string]interface{} {
	// 1. A tool declarou um voice summary? Usa directo.
	if v, ok := result["_voice_summary"]; ok {
		if summary, ok := v.(string); ok && summary != "" {
			return map[string]interface{}{"result": enforceLimit(summary)}
		}
	}

	// 2. Serializa para medir tamanho
	raw, err := json.Marshal(result)
	if err != nil {
		return map[string]interface{}{"result": fmt.Sprintf("Tool %s executada com sucesso.", toolName)}
	}

	// 3. Pequeno o suficiente? Passa directo (but strip internal _voice_summary field).
	if len(raw) <= AudioSummarizeAbove {
		delete(result, "_voice_summary") // M18 fix: don't leak internal field to Gemini
		return result
	}

	// 4. Summarizer determinístico por tool name
	if summary := deterministicSummary(toolName, result); summary != "" {
		return map[string]interface{}{"result": enforceLimit(summary)}
	}

	// 5. Fallback genérico — extrai campos-chave
	return map[string]interface{}{"result": enforceLimit(genericSummary(toolName, result))}
}

// enforceLimit garante que nunca passa do limite,
// cortando na última frase completa.
func enforceLimit(s string) string {
	if len(s) <= AudioPayloadMaxBytes {
		return s
	}
	// H9 fix: truncate on rune boundary to avoid invalid UTF-8
	runes := []rune(s)
	truncated := string(runes)
	for len(truncated) > AudioPayloadMaxBytes {
		runes = runes[:len(runes)-1]
		truncated = string(runes)
	}
	if idx := strings.LastIndexAny(truncated, ".!?"); idx > 0 {
		return truncated[:idx+1]
	}
	return truncated
}

// deterministicSummary cobre as tools que retornam dados grandes.
func deterministicSummary(toolName string, result map[string]interface{}) string {
	switch toolName {

	case "list_my_collections":
		msg := strField(result, "message", "")
		if msg != "" {
			return msg // o swarm handler já formata compacto
		}
		total := intField(result, "total", 0)
		return fmt.Sprintf("Tenho %d colecoes no NietzscheDB. Os detalhes estao na tela.", total)

	case "query_my_database", "query_my_graph", "search_knowledge":
		nodes := sliceField(result, "nodes")
		total := intField(result, "total", len(nodes))
		collection := strField(result, "collection", "minha base")
		top := topContentNames(nodes, 3)
		if top == "" {
			return fmt.Sprintf("Encontrei %d resultados em %s. Detalhes na tela.", total, collection)
		}
		return fmt.Sprintf("Encontrei %d resultados em %s. Destaques: %s. Detalhes na tela.", total, collection, top)

	case "search_my_code", "search_my_docs", "search_self_knowledge":
		results := sliceField(result, "results")
		if results == nil {
			results = sliceField(result, "nodes")
		}
		total := intField(result, "total", len(results))
		return fmt.Sprintf("Encontrei %d resultados. Detalhes na tela.", total)

	case "system_stats":
		nodes := intField(result, "nietzsche_total_nodes", 0)
		cols := intField(result, "nietzsche_collections", 0)
		mem := intField(result, "mem_alloc_mb", 0)
		uptime := strField(result, "uptime", "")
		return fmt.Sprintf("Sistema ativo ha %s, %d MB de memoria, %d colecoes com %d nos no NietzscheDB.", uptime, mem, cols, nodes)

	case "introspect":
		report := strField(result, "report", strField(result, "message", ""))
		if len(report) > AudioPayloadMaxBytes {
			return report[:AudioPayloadMaxBytes-30] + "... Relatorio completo na tela."
		}
		if report != "" {
			return report
		}
		return "Introspeccao concluida. Relatorio na tela."

	case "my_energy_stats":
		msg := strField(result, "message", strField(result, "summary", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Estatisticas de energia calculadas. Detalhes na tela."

	case "my_topology":
		msg := strField(result, "message", strField(result, "summary", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Topologia do grafo analisada. Detalhes na tela."

	case "invoke_zaratustra":
		msg := strField(result, "message", strField(result, "response", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Zaratustra respondeu. Detalhes na tela."

	case "list_memories", "memory_stats":
		total := intField(result, "total", intField(result, "count", 0))
		return fmt.Sprintf("Encontrei %d memorias. Detalhes na tela.", total)

	case "list_tasks":
		tasks := sliceField(result, "tasks")
		total := intField(result, "total", len(tasks))
		return fmt.Sprintf("Tens %d tarefas. Detalhes na tela.", total)

	case "list_curriculum", "check_learning_progress":
		total := intField(result, "total", intField(result, "count", 0))
		pending := intField(result, "pending", 0)
		done := intField(result, "done", intField(result, "completed", 0))
		if pending > 0 || done > 0 {
			return fmt.Sprintf("%d topicos no curriculo, %d pendentes, %d concluidos.", total, pending, done)
		}
		return fmt.Sprintf("%d topicos no curriculo. Detalhes na tela.", total)

	case "list_alarms":
		alarms := sliceField(result, "alarms")
		total := intField(result, "total", len(alarms))
		return fmt.Sprintf("Tens %d alarmes configurados. Detalhes na tela.", total)

	case "pending_schedule":
		events := sliceField(result, "events")
		total := intField(result, "total", len(events))
		if total == 0 {
			return "Nao tens compromissos pendentes."
		}
		return fmt.Sprintf("Tens %d compromissos pendentes. Detalhes na tela.", total)

	case "get_health_data", "manage_health_sheet":
		msg := strField(result, "message", strField(result, "summary", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Dados de saude carregados. Detalhes na tela."

	case "habit_stats", "habit_summary":
		msg := strField(result, "message", strField(result, "summary", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Resumo de habitos pronto. Detalhes na tela."

	case "weekly_review":
		msg := strField(result, "message", strField(result, "summary", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Revisao semanal concluida. Detalhes na tela."

	case "google_search_retrieval":
		msg := strField(result, "message", strField(result, "answer", ""))
		if msg != "" {
			return enforceLimit(msg)
		}
		return "Pesquisa concluida. Resultados na tela."

	case "recall_memory":
		mem := strField(result, "memories", "")
		count := intField(result, "count", 0)
		if mem == "" || mem == "Nenhuma memoria encontrada." {
			return "Nao encontrei memorias relevantes sobre esse assunto."
		}
		if mem != "" {
			return enforceLimit(mem)
		}
		return fmt.Sprintf("Encontrei %d memorias relevantes.", count)
	}

	return "" // não reconhecida → fallback genérico
}

// genericSummary — fallback para tools não mapeadas com payload grande
func genericSummary(toolName string, result map[string]interface{}) string {
	// Tenta extrair campo de mensagem comum
	for _, key := range []string{"message", "summary", "result", "response", "text", "answer"} {
		if msg, ok := result[key]; ok {
			if s, ok := msg.(string); ok && s != "" {
				return enforceLimit(s)
			}
		}
	}

	// Tenta extrair contagem
	for _, key := range []string{"total", "count"} {
		if v, ok := result[key]; ok {
			return fmt.Sprintf("Tool %s retornou %v itens. Detalhes na tela.", toolName, v)
		}
	}

	// Conta campos de nível superior
	if len(result) > 0 {
		// Verifica se tem sucesso/erro
		if success, ok := result["success"]; ok {
			if b, ok := success.(bool); ok {
				if b {
					return fmt.Sprintf("Tool %s executada com sucesso. Detalhes na tela.", toolName)
				}
				errMsg := strField(result, "error", "erro desconhecido")
				return fmt.Sprintf("Tool %s falhou: %s", toolName, enforceLimit(errMsg))
			}
		}
	}

	return fmt.Sprintf("Tool %s concluida. Detalhes na tela.", toolName)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func strField(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func intField(m map[string]interface{}, key string, fallback int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		case uint64:
			return int(n)
		}
	}
	return fallback
}

func sliceField(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if s, ok := v.([]interface{}); ok {
			return s
		}
	}
	return nil
}

func topContentNames(nodes []interface{}, n int) string {
	var names []string
	for i, node := range nodes {
		if i >= n {
			break
		}
		if m, ok := node.(map[string]interface{}); ok {
			name := strField(m, "content", strField(m, "name", strField(m, "title", "")))
			if name != "" {
				// Trunca nomes individuais longos
				if len(name) > 60 {
					name = name[:57] + "..."
				}
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}
