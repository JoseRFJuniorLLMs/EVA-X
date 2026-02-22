// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// EVA-Mind MCP Server (stdio)
// Bridges Claude Code ↔ EVA's 150+ tools via JSON-RPC over stdin/stdout.
// Usage: claude mcp add eva-mind -- go run ./cmd/mcp-server/main.go

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// JSON-RPC 2.0 Types
// ═══════════════════════════════════════════════════════════════════

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ═══════════════════════════════════════════════════════════════════
// MCP Protocol Types
// ═══════════════════════════════════════════════════════════════════

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ═══════════════════════════════════════════════════════════════════
// EVA MCP Server
// ═══════════════════════════════════════════════════════════════════

type EVAMCPServer struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewEVAMCPServer() *EVAMCPServer {
	apiURL := os.Getenv("EVA_API_URL")
	if apiURL == "" {
		apiURL = "http://34.35.36.178:8080"
	}
	apiURL = strings.TrimRight(apiURL, "/")

	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "FATAL: MCP_API_KEY not set — refusing to start with no authentication\n")
		os.Exit(1)
	}

	return &EVAMCPServer{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ═══════════════════════════════════════════════════════════════════
// Main Loop — stdin/stdout JSON-RPC
// ═══════════════════════════════════════════════════════════════════

func main() {
	server := NewEVAMCPServer()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "MCP server shutting down (signal: %v)\n", sig)
		os.Exit(0)
	}()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintf(os.Stderr, "MCP: invalid JSON-RPC: %v\n", err)
			continue
		}

		var resp JSONRPCResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = server.handleInitialize()
		case "notifications/initialized":
			continue // no response needed
		case "tools/list":
			resp.Result = server.handleToolsList()
		case "tools/call":
			resp.Result, resp.Error = server.handleToolsCall(req.Params)
		case "resources/list":
			resp.Result = map[string]interface{}{"resources": []interface{}{}}
		case "prompts/list":
			resp.Result = map[string]interface{}{"prompts": []interface{}{}}
		default:
			resp.Error = &RPCError{Code: -32601, Message: "method not found: " + req.Method}
		}

		out, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", out)
	}

	// Log scanner error if stdin closed unexpectedly
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP: stdin read error: %v\n", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
// Initialize
// ═══════════════════════════════════════════════════════════════════

func (s *EVAMCPServer) handleInitialize() interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "eva-mind",
			"version": "2.0.0",
		},
	}
}

// ═══════════════════════════════════════════════════════════════════
// Tools List — ALL 44 tools
// ═══════════════════════════════════════════════════════════════════

func (s *EVAMCPServer) handleToolsList() interface{} {
	tools := []MCPTool{
		// ═══════════════════════════════════════════════════════════
		// 🧠 MEMORY & KNOWLEDGE
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_remember",
			Description: "Armazena uma memoria na EVA. Use para ensinar algo, registrar decisoes, salvar contexto.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"content": {Type: "string", Description: "Conteudo da memoria a armazenar"},
				},
				Required: []string{"content"},
			},
		},
		{
			Name:        "eva_recall",
			Description: "Busca memorias da EVA por query. Retorna memorias relevantes armazenadas.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {Type: "string", Description: "Busca por texto nas memorias"},
					"limit": {Type: "string", Description: "Limite de resultados (default: 10)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_teach",
			Description: "Ensina algo novo a EVA. Grava como CoreMemory no NietzscheDB (eva_core). Ela lembra pra sempre.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"teaching":   {Type: "string", Description: "O que ensinar a EVA"},
					"importance": {Type: "string", Description: "Importancia de 0.0 a 1.0 (default: 0.8)"},
				},
				Required: []string{"teaching"},
			},
		},
		{
			Name:        "eva_identity",
			Description: "Retorna a identidade atual da EVA: personalidade, memorias, capacidades, como ela se ve.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "eva_learn_topic",
			Description: "EVA estuda um topico autonomamente: pesquisa web, resume com Gemini, armazena no Qdrant.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"topic": {Type: "string", Description: "Topico para EVA estudar"},
				},
				Required: []string{"topic"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 📧 COMMUNICATION
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_send_email",
			Description: "Envia email via Gmail. EVA e sua secretaria — manda email por voce.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"to":      {Type: "string", Description: "Email do destinatario"},
					"subject": {Type: "string", Description: "Assunto do email"},
					"body":    {Type: "string", Description: "Corpo do email"},
				},
				Required: []string{"to", "subject", "body"},
			},
		},
		{
			Name:        "eva_send_whatsapp",
			Description: "Envia mensagem no WhatsApp via Meta Graph API.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"to":      {Type: "string", Description: "Numero de telefone (com DDI, ex: +5511999999999)"},
					"message": {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"to", "message"},
			},
		},
		{
			Name:        "eva_send_telegram",
			Description: "Envia mensagem no Telegram via Bot API.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"chat_id": {Type: "string", Description: "Chat ID do Telegram"},
					"message": {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"chat_id", "message"},
			},
		},
		{
			Name:        "eva_send_slack",
			Description: "Envia mensagem no Slack.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"channel": {Type: "string", Description: "Canal do Slack (ex: #general)"},
					"message": {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"message"},
			},
		},
		{
			Name:        "eva_send_discord",
			Description: "Envia mensagem no Discord.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"channel_id": {Type: "string", Description: "ID do canal Discord"},
					"message":    {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"channel_id", "message"},
			},
		},
		{
			Name:        "eva_send_teams",
			Description: "Envia mensagem no Microsoft Teams via Incoming Webhook.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"message": {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"message"},
			},
		},
		{
			Name:        "eva_send_signal",
			Description: "Envia mensagem no Signal via signal-cli.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"recipient": {Type: "string", Description: "Numero de telefone do destinatario"},
					"message":   {Type: "string", Description: "Mensagem a enviar"},
				},
				Required: []string{"recipient", "message"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 📅 PRODUCTIVITY
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_calendar_create",
			Description: "Cria evento no Google Calendar.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"title":       {Type: "string", Description: "Titulo do evento"},
					"description": {Type: "string", Description: "Descricao do evento"},
					"start_time":  {Type: "string", Description: "Data/hora inicio (ISO 8601)"},
					"end_time":    {Type: "string", Description: "Data/hora fim (ISO 8601)"},
					"location":    {Type: "string", Description: "Local do evento"},
				},
				Required: []string{"title", "start_time"},
			},
		},
		{
			Name:        "eva_calendar_list",
			Description: "Lista proximos eventos do Google Calendar.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"max_results": {Type: "string", Description: "Maximo de eventos (default: 10)"},
				},
			},
		},
		{
			Name:        "eva_drive_save",
			Description: "Salva arquivo no Google Drive.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_name": {Type: "string", Description: "Nome do arquivo"},
					"content":   {Type: "string", Description: "Conteudo do arquivo"},
					"mime_type": {Type: "string", Description: "MIME type (default: text/plain)"},
				},
				Required: []string{"file_name", "content"},
			},
		},
		{
			Name:        "eva_create_reminder",
			Description: "Cria tarefa agendada (cron). EVA executa automaticamente no horario.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"description": {Type: "string", Description: "Descricao da tarefa"},
					"schedule":    {Type: "string", Description: "Agendamento (ex: 'every 5m', 'daily 08:00', 'hourly')"},
					"tool_name":   {Type: "string", Description: "Tool a executar (ex: send_email)"},
					"tool_args":   {Type: "string", Description: "Args da tool em JSON"},
				},
				Required: []string{"description", "schedule"},
			},
		},
		{
			Name:        "eva_list_reminders",
			Description: "Lista tarefas agendadas ativas.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "eva_cancel_reminder",
			Description: "Cancela uma tarefa agendada.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"task_id": {Type: "string", Description: "ID da tarefa a cancelar"},
				},
				Required: []string{"task_id"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 📹 MEDIA
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_youtube_search",
			Description: "Busca videos no YouTube. Retorna titulo, URL e thumbnail.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":       {Type: "string", Description: "Busca no YouTube"},
					"max_results": {Type: "string", Description: "Maximo de resultados (default: 5)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_spotify_search",
			Description: "Busca musicas no Spotify. Retorna nome, artista e URI.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":       {Type: "string", Description: "Busca no Spotify"},
					"max_results": {Type: "string", Description: "Maximo de resultados (default: 5)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_web_browse",
			Description: "Navega uma pagina web e extrai conteudo (titulo, texto, links).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {Type: "string", Description: "URL da pagina a acessar"},
				},
				Required: []string{"url"},
			},
		},
		{
			Name:        "eva_web_search",
			Description: "Pesquisa na internet e retorna resultados resumidos.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {Type: "string", Description: "Busca na internet"},
				},
				Required: []string{"query"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 🗄️ DATABASES
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_query_postgres",
			Description: "Executa query SQL no PostgreSQL da EVA (130+ tabelas, dados dos pacientes, agendamentos, medicamentos).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":  {Type: "string", Description: "Query SQL (SELECT apenas por seguranca)"},
					"params": {Type: "string", Description: "Parametros em JSON array (opcional)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_query_nietzsche_graph",
			Description: "Executa query NQL no NietzscheDB (grafo geral). Grafo de conhecimento: Person, Condition, Medication, Symptom, FDPNNode.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":  {Type: "string", Description: "Query NQL"},
					"params": {Type: "string", Description: "Parametros em JSON object (opcional)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_query_nietzsche_core",
			Description: "Executa query NQL no NietzscheDB eva_core. Memoria pessoal da EVA: EvaSelf, CoreMemory, MetaInsight.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":  {Type: "string", Description: "Query NQL"},
					"params": {Type: "string", Description: "Parametros em JSON object (opcional)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "eva_query_nietzsche_vector",
			Description: "Busca vetorial no NietzscheDB (KNN semantico). 20+ colecoes com embeddings 3072-dim.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"collection": {Type: "string", Description: "Nome da colecao NietzscheDB"},
					"query":      {Type: "string", Description: "Texto para busca semantica"},
					"limit":      {Type: "string", Description: "Limite de resultados (default: 5)"},
				},
				Required: []string{"collection", "query"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 🖥️ CODE & SKILLS
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_execute_code",
			Description: "Executa codigo em sandbox seguro (bash, Python, Node.js). Timeout max 2min.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"language": {Type: "string", Description: "Linguagem: bash, python, node"},
					"code":     {Type: "string", Description: "Codigo a executar"},
					"timeout":  {Type: "string", Description: "Timeout em segundos (default: 30, max: 120)"},
				},
				Required: []string{"code"},
			},
		},
		{
			Name:        "eva_create_skill",
			Description: "Cria nova skill (capacidade) na EVA. Persiste em disco como JSON.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":        {Type: "string", Description: "Nome da skill"},
					"description": {Type: "string", Description: "Descricao do que a skill faz"},
					"language":    {Type: "string", Description: "Linguagem: bash, python, node"},
					"code":        {Type: "string", Description: "Codigo da skill"},
				},
				Required: []string{"name", "code"},
			},
		},
		{
			Name:        "eva_list_skills",
			Description: "Lista todas as skills disponiveis na EVA.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "eva_run_skill",
			Description: "Executa uma skill existente da EVA.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"skill_name": {Type: "string", Description: "Nome da skill a executar"},
					"args":       {Type: "string", Description: "Argumentos em JSON object (opcional)"},
				},
				Required: []string{"skill_name"},
			},
		},
		{
			Name:        "eva_delete_skill",
			Description: "Remove uma skill da EVA.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"skill_name": {Type: "string", Description: "Nome da skill a remover"},
				},
				Required: []string{"skill_name"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 📂 FILESYSTEM
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_read_file",
			Description: "Le conteudo de um arquivo no workspace da EVA.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string", Description: "Caminho do arquivo (relativo ao workspace)"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "eva_write_file",
			Description: "Escreve conteudo em um arquivo no workspace da EVA.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":    {Type: "string", Description: "Caminho do arquivo"},
					"content": {Type: "string", Description: "Conteudo a escrever"},
				},
				Required: []string{"path", "content"},
			},
		},
		{
			Name:        "eva_list_files",
			Description: "Lista arquivos em um diretorio do workspace da EVA.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"directory": {Type: "string", Description: "Diretorio a listar (default: /)"},
				},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 🏠 SMART HOME
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_smart_home_control",
			Description: "Controla dispositivo IoT via Home Assistant (ligar, desligar, toggle).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"device_id":  {Type: "string", Description: "ID do dispositivo (ex: light.sala, switch.ventilador)"},
					"action":     {Type: "string", Description: "Acao: on, off, toggle, ligar, desligar"},
					"brightness": {Type: "string", Description: "Brilho 0-255 (apenas para luzes)"},
				},
				Required: []string{"device_id", "action"},
			},
		},
		{
			Name:        "eva_smart_home_status",
			Description: "Lista dispositivos IoT e seus estados atuais.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"device_id": {Type: "string", Description: "ID do dispositivo (vazio = listar todos)"},
				},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 🔗 WEBHOOKS
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_create_webhook",
			Description: "Cria um webhook para notificacoes automaticas.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":   {Type: "string", Description: "Nome do webhook"},
					"url":    {Type: "string", Description: "URL de destino"},
					"events": {Type: "string", Description: "Eventos a escutar (JSON array, default: ['*'])"},
				},
				Required: []string{"name", "url"},
			},
		},
		{
			Name:        "eva_list_webhooks",
			Description: "Lista todos os webhooks configurados.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "eva_trigger_webhook",
			Description: "Dispara um webhook manualmente.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":    {Type: "string", Description: "Nome do webhook a disparar"},
					"payload": {Type: "string", Description: "Payload em JSON (opcional)"},
				},
				Required: []string{"name"},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 💻 SELF-CODING
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_read_source",
			Description: "Le um arquivo do codigo-fonte da EVA-Mind.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {Type: "string", Description: "Caminho relativo ao projeto (ex: internal/tools/handlers.go)"},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "eva_edit_source",
			Description: "Edita um arquivo do codigo-fonte da EVA (APENAS em branches eva/*).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {Type: "string", Description: "Caminho do arquivo"},
					"content":   {Type: "string", Description: "Novo conteudo do arquivo"},
				},
				Required: []string{"file_path", "content"},
			},
		},
		{
			Name:        "eva_run_tests",
			Description: "Executa testes do projeto EVA-Mind (go test ./...).",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "eva_get_diff",
			Description: "Retorna git diff do projeto EVA-Mind.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},

		// ═══════════════════════════════════════════════════════════
		// 🤖 MULTI-LLM
		// ═══════════════════════════════════════════════════════════
		{
			Name:        "eva_ask_llm",
			Description: "Consulta outro LLM (Claude, GPT, DeepSeek) para segunda opiniao.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"provider": {Type: "string", Description: "Provedor: claude, gpt, deepseek (default: claude)"},
					"prompt":   {Type: "string", Description: "Prompt para o LLM"},
				},
				Required: []string{"prompt"},
			},
		},
	}

	return map[string]interface{}{"tools": tools}
}

// ═══════════════════════════════════════════════════════════════════
// Tool Call Handler — routes to EVA API
// ═══════════════════════════════════════════════════════════════════

// mcpToEVA maps MCP tool names to EVA internal tool names
var mcpToEVA = map[string]string{
	// Memory & Knowledge
	"eva_remember":    "mcp_remember",
	"eva_recall":      "mcp_recall",
	"eva_teach":       "mcp_teach_eva",
	"eva_identity":    "mcp_get_identity",
	"eva_learn_topic": "mcp_learn_topic",
	// Communication
	"eva_send_email":    "send_email",
	"eva_send_whatsapp": "send_whatsapp",
	"eva_send_telegram": "send_telegram",
	"eva_send_slack":    "send_slack",
	"eva_send_discord":  "send_discord",
	"eva_send_teams":    "send_teams",
	"eva_send_signal":   "send_signal",
	// Productivity
	"eva_calendar_create": "manage_calendar_event",
	"eva_calendar_list":   "manage_calendar_event",
	"eva_drive_save":      "save_to_drive",
	"eva_create_reminder": "create_scheduled_task",
	"eva_list_reminders":  "list_scheduled_tasks",
	"eva_cancel_reminder": "cancel_scheduled_task",
	// Media
	"eva_youtube_search": "search_videos",
	"eva_spotify_search": "play_music",
	"eva_web_browse":     "browser_navigate",
	"eva_web_search":     "web_search",
	// Databases
	"eva_query_postgres":   "query_postgresql",
	"eva_query_nietzsche_graph":  "query_nietzsche_graph",
	"eva_query_nietzsche_core":   "mcp_query_nietzsche_core",
	"eva_query_nietzsche_vector": "query_nietzsche_vector",
	// Code & Skills
	"eva_execute_code": "execute_code",
	"eva_create_skill": "create_skill",
	"eva_list_skills":  "list_skills",
	"eva_run_skill":    "execute_skill",
	"eva_delete_skill": "delete_skill",
	// Filesystem
	"eva_read_file":  "read_file",
	"eva_write_file": "write_file",
	"eva_list_files": "list_files",
	// Smart Home
	"eva_smart_home_control": "smart_home_control",
	"eva_smart_home_status":  "smart_home_status",
	// Webhooks
	"eva_create_webhook":  "create_webhook",
	"eva_list_webhooks":   "list_webhooks",
	"eva_trigger_webhook": "trigger_webhook",
	// Self-Coding
	"eva_read_source": "mcp_read_source",
	"eva_edit_source": "mcp_edit_source",
	"eva_run_tests":   "run_tests",
	"eva_get_diff":    "get_code_diff",
	// Multi-LLM
	"eva_ask_llm": "ask_llm",
}

func (s *EVAMCPServer) handleToolsCall(params json.RawMessage) (interface{}, *RPCError) {
	var call ToolCallParams
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	// Map MCP tool name to EVA internal name
	evaToolName, ok := mcpToEVA[call.Name]
	if !ok {
		return nil, &RPCError{Code: -32602, Message: "unknown tool: " + call.Name}
	}

	// Call EVA API
	result, err := s.callEVA(evaToolName, call.Arguments)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: "EVA API error: " + err.Error()}
	}

	// Format as MCP content
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return map[string]interface{}{
		"content": []ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

// ═══════════════════════════════════════════════════════════════════
// EVA API Client
// ═══════════════════════════════════════════════════════════════════

func (s *EVAMCPServer) callEVA(toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"tool_name": toolName,
		"args":      args,
		"idoso_id":  1, // Creator's ID
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.apiURL+"/api/v1/tools/execute", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MCP-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call EVA API at %s: %w", s.apiURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EVA API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
