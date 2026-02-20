package main

import (
	"encoding/json"
	"testing"
)

func TestMCPToEVAMapping_AllToolsMapped(t *testing.T) {
	// Verificar que todas as 42 tools tem mapeamento
	expectedCount := 44
	if len(mcpToEVA) != expectedCount {
		t.Errorf("expected %d tools in mcpToEVA, got %d", expectedCount, len(mcpToEVA))
	}
}

func TestMCPToEVAMapping_NoDuplicateInternalNames(t *testing.T) {
	// manage_calendar_event e usado por 2 tools (create/list) - excecao aceita
	seen := map[string][]string{}
	for mcp, eva := range mcpToEVA {
		seen[eva] = append(seen[eva], mcp)
	}

	for eva, mcps := range seen {
		if len(mcps) > 1 && eva != "manage_calendar_event" {
			t.Errorf("internal tool %q is mapped by multiple MCP tools: %v", eva, mcps)
		}
	}
}

func TestMCPToEVAMapping_AllPrefixed(t *testing.T) {
	for mcp := range mcpToEVA {
		if len(mcp) < 4 || mcp[:4] != "eva_" {
			t.Errorf("MCP tool %q should have 'eva_' prefix", mcp)
		}
	}
}

func TestMCPToEVAMapping_NoEmptyInternalNames(t *testing.T) {
	for mcp, eva := range mcpToEVA {
		if eva == "" {
			t.Errorf("MCP tool %q maps to empty internal name", mcp)
		}
	}
}

func TestHandleInitialize(t *testing.T) {
	server := &EVAMCPServer{apiURL: "http://localhost:8080", apiKey: "test"}
	result := server.handleInitialize()

	data, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("handleInitialize should return map")
	}

	if data["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", data["protocolVersion"])
	}

	info, ok := data["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("serverInfo should be a map")
	}
	if info["name"] != "eva-mind" {
		t.Errorf("expected server name 'eva-mind', got %v", info["name"])
	}
}

func TestHandleToolsList(t *testing.T) {
	server := &EVAMCPServer{apiURL: "http://localhost:8080", apiKey: "test"}
	result := server.handleToolsList()

	data, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("handleToolsList should return map")
	}

	tools, ok := data["tools"].([]MCPTool)
	if !ok {
		t.Fatal("tools should be []MCPTool")
	}

	if len(tools) != 44 {
		t.Errorf("expected 44 tools, got %d", len(tools))
	}

	// Verificar que todas tools tem nome e descricao
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %q schema type should be 'object', got %q", tool.Name, tool.InputSchema.Type)
		}
	}
}

func TestHandleToolsList_AllToolsInMapping(t *testing.T) {
	server := &EVAMCPServer{apiURL: "http://localhost:8080", apiKey: "test"}
	result := server.handleToolsList()
	data := result.(map[string]interface{})
	tools := data["tools"].([]MCPTool)

	for _, tool := range tools {
		if _, ok := mcpToEVA[tool.Name]; !ok {
			t.Errorf("tool %q is listed but not in mcpToEVA mapping", tool.Name)
		}
	}
}

func TestHandleToolsCall_UnknownTool(t *testing.T) {
	server := &EVAMCPServer{apiURL: "http://localhost:8080", apiKey: "test"}

	params, _ := json.Marshal(ToolCallParams{
		Name:      "nonexistent_tool",
		Arguments: map[string]interface{}{},
	})

	_, err := server.handleToolsCall(params)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if err.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", err.Code)
	}
}

func TestHandleToolsCall_InvalidParams(t *testing.T) {
	server := &EVAMCPServer{apiURL: "http://localhost:8080", apiKey: "test"}

	_, err := server.handleToolsCall(json.RawMessage(`{invalid json`))
	if err == nil {
		t.Error("expected error for invalid params")
	}
	if err.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", err.Code)
	}
}

func TestMCPToEVA_CriticalMappings(t *testing.T) {
	// Verificar que os mapeamentos criticos nao apontam para nomes errados
	critical := map[string]string{
		"eva_remember":         "mcp_remember",
		"eva_recall":           "mcp_recall",
		"eva_teach":            "mcp_teach_eva",
		"eva_identity":         "mcp_get_identity",
		"eva_learn_topic":      "mcp_learn_topic",
		"eva_query_postgres":   "query_postgresql",
		"eva_query_neo4j_core": "mcp_query_neo4j_core",
		"eva_read_source":      "mcp_read_source",
		"eva_edit_source":      "mcp_edit_source",
	}

	for mcp, expected := range critical {
		got, ok := mcpToEVA[mcp]
		if !ok {
			t.Errorf("critical mapping missing: %q", mcp)
			continue
		}
		if got != expected {
			t.Errorf("critical mapping %q: expected %q, got %q", mcp, expected, got)
		}
	}
}
