// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"eva/internal/brainstem/database"
)

// ToolRecord representa uma ferramenta armazenada no banco de dados
type ToolRecord struct {
	ID                    int64           `json:"id"`
	Name                  string          `json:"name"`
	DisplayName           string          `json:"display_name"`
	Description           string          `json:"description"`
	Category              string          `json:"category"`
	Subcategory           string          `json:"subcategory"`
	Tags                  []string        `json:"tags"`
	Parameters            json.RawMessage `json:"parameters"`
	Enabled               bool            `json:"enabled"`
	RequiresPermission    bool            `json:"requires_permission"`
	PermissionLevel       string          `json:"permission_level"`
	HandlerType           string          `json:"handler_type"`
	HandlerConfig         json.RawMessage `json:"handler_config"`
	RateLimitPerMinute    int             `json:"rate_limit_per_minute"`
	RateLimitPerHour      int             `json:"rate_limit_per_hour"`
	TimeoutSeconds        int             `json:"timeout_seconds"`
	UsageHint             string          `json:"usage_hint"`
	ExamplePrompts        []string        `json:"example_prompts"`
	DependsOn             []string        `json:"depends_on"`
	ConflictsWith         []string        `json:"conflicts_with"`
	Priority              int             `json:"priority"`
	IsCritical            bool            `json:"is_critical"`
	Version               string          `json:"version"`
	Deprecated            bool            `json:"deprecated"`
	DeprecatedMessage     string          `json:"deprecated_message"`
	ReplacementTool       string          `json:"replacement_tool"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	TotalInvocations      int64           `json:"total_invocations"`
	SuccessfulInvocations int64           `json:"successful_invocations"`
	FailedInvocations     int64           `json:"failed_invocations"`
	LastInvokedAt         *time.Time      `json:"last_invoked_at"`
	AvgExecutionTimeMs    int             `json:"avg_execution_time_ms"`
}

// CapabilityRecord representa uma capacidade da EVA
type CapabilityRecord struct {
	ID               int64    `json:"id"`
	CapabilityName   string   `json:"capability_name"`
	CapabilityType   string   `json:"capability_type"`
	Description      string   `json:"description"`
	ShortDescription string   `json:"short_description"`
	RelatedTools     []string `json:"related_tools"`
	WhenToUse        string   `json:"when_to_use"`
	WhenNotToUse     string   `json:"when_not_to_use"`
	ExampleQueries   []string `json:"example_queries"`
	Enabled          bool     `json:"enabled"`
	PromptPriority   int      `json:"prompt_priority"`
}

// ToolRepository gerencia operações CRUD de ferramentas
type ToolRepository struct {
	db *database.DB
}

// NewToolRepository cria uma nova instância do repository
func NewToolRepository(db *database.DB) *ToolRepository {
	return &ToolRepository{db: db}
}

// mapToToolRecord converts a NietzscheDB content map to a ToolRecord
func mapToToolRecord(m map[string]interface{}) *ToolRecord {
	tool := &ToolRecord{
		ID:                    database.GetInt64(m, "id"),
		Name:                  database.GetString(m, "name"),
		DisplayName:           database.GetString(m, "display_name"),
		Description:           database.GetString(m, "description"),
		Category:              database.GetString(m, "category"),
		Subcategory:           database.GetString(m, "subcategory"),
		Enabled:               database.GetBool(m, "enabled"),
		RequiresPermission:    database.GetBool(m, "requires_permission"),
		PermissionLevel:       database.GetString(m, "permission_level"),
		HandlerType:           database.GetString(m, "handler_type"),
		RateLimitPerMinute:    int(database.GetInt64(m, "rate_limit_per_minute")),
		RateLimitPerHour:      int(database.GetInt64(m, "rate_limit_per_hour")),
		TimeoutSeconds:        int(database.GetInt64(m, "timeout_seconds")),
		UsageHint:             database.GetString(m, "usage_hint"),
		Priority:              int(database.GetInt64(m, "priority")),
		IsCritical:            database.GetBool(m, "is_critical"),
		Version:               database.GetString(m, "version"),
		Deprecated:            database.GetBool(m, "deprecated"),
		DeprecatedMessage:     database.GetString(m, "deprecated_message"),
		ReplacementTool:       database.GetString(m, "replacement_tool"),
		CreatedAt:             database.GetTime(m, "created_at"),
		UpdatedAt:             database.GetTime(m, "updated_at"),
		TotalInvocations:      database.GetInt64(m, "total_invocations"),
		SuccessfulInvocations: database.GetInt64(m, "successful_invocations"),
		FailedInvocations:     database.GetInt64(m, "failed_invocations"),
		LastInvokedAt:         database.GetTimePtr(m, "last_invoked_at"),
		AvgExecutionTimeMs:    int(database.GetInt64(m, "avg_execution_time_ms")),
	}

	// Parse JSON fields
	if raw := database.GetString(m, "tags"); raw != "" {
		json.Unmarshal([]byte(raw), &tool.Tags)
	}
	if raw := database.GetString(m, "parameters"); raw != "" {
		tool.Parameters = json.RawMessage(raw)
	}
	if raw := database.GetString(m, "handler_config"); raw != "" {
		tool.HandlerConfig = json.RawMessage(raw)
	}
	if raw := database.GetString(m, "example_prompts"); raw != "" {
		json.Unmarshal([]byte(raw), &tool.ExamplePrompts)
	}
	if raw := database.GetString(m, "depends_on"); raw != "" {
		json.Unmarshal([]byte(raw), &tool.DependsOn)
	}
	if raw := database.GetString(m, "conflicts_with"); raw != "" {
		json.Unmarshal([]byte(raw), &tool.ConflictsWith)
	}

	return tool
}

// GetAll retorna todas as ferramentas registradas
func (r *ToolRepository) GetAll(ctx context.Context) ([]*ToolRecord, error) {
	rows, err := r.db.QueryByLabel(ctx, "available_tools", "", nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools: %w", err)
	}

	var tools []*ToolRecord
	for _, m := range rows {
		tools = append(tools, mapToToolRecord(m))
	}

	// Sort by priority DESC, category, name
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Priority != tools[j].Priority {
			return tools[i].Priority > tools[j].Priority
		}
		if tools[i].Category != tools[j].Category {
			return tools[i].Category < tools[j].Category
		}
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

// GetEnabled retorna apenas ferramentas habilitadas e não depreciadas
func (r *ToolRepository) GetEnabled(ctx context.Context) ([]*ToolRecord, error) {
	rows, err := r.db.QueryByLabel(ctx, "available_tools",
		" AND n.enabled = $enabled AND n.deprecated = $deprecated",
		map[string]interface{}{"enabled": true, "deprecated": false}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled tools: %w", err)
	}

	var tools []*ToolRecord
	for _, m := range rows {
		tools = append(tools, mapToToolRecord(m))
	}

	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Priority != tools[j].Priority {
			return tools[i].Priority > tools[j].Priority
		}
		if tools[i].Category != tools[j].Category {
			return tools[i].Category < tools[j].Category
		}
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

// GetByCategory retorna ferramentas de uma categoria específica
func (r *ToolRepository) GetByCategory(ctx context.Context, category string) ([]*ToolRecord, error) {
	rows, err := r.db.QueryByLabel(ctx, "available_tools",
		" AND n.category = $category AND n.enabled = $enabled AND n.deprecated = $deprecated",
		map[string]interface{}{"category": category, "enabled": true, "deprecated": false}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools by category: %w", err)
	}

	var tools []*ToolRecord
	for _, m := range rows {
		tools = append(tools, mapToToolRecord(m))
	}

	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Priority != tools[j].Priority {
			return tools[i].Priority > tools[j].Priority
		}
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

// GetByName retorna uma ferramenta pelo nome
func (r *ToolRepository) GetByName(ctx context.Context, name string) (*ToolRecord, error) {
	rows, err := r.db.QueryByLabel(ctx, "available_tools",
		" AND n.name = $name",
		map[string]interface{}{"name": name}, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to query tool by name: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return mapToToolRecord(rows[0]), nil
}

// Register registra uma nova ferramenta
func (r *ToolRepository) Register(ctx context.Context, tool *ToolRecord) error {
	tagsJSON, _ := json.Marshal(tool.Tags)
	examplesJSON, _ := json.Marshal(tool.ExamplePrompts)
	dependsJSON, _ := json.Marshal(tool.DependsOn)
	conflictsJSON, _ := json.Marshal(tool.ConflictsWith)

	now := time.Now().UTC().Format(time.RFC3339)

	// Try to find existing tool by name (upsert)
	existing, _ := r.db.QueryByLabel(ctx, "available_tools",
		" AND n.name = $name",
		map[string]interface{}{"name": tool.Name}, 1)

	if len(existing) > 0 {
		// Update existing
		err := r.db.Update(ctx, "available_tools",
			map[string]interface{}{"name": tool.Name},
			map[string]interface{}{
				"display_name": tool.DisplayName,
				"description":  tool.Description,
				"parameters":   string(tool.Parameters),
				"updated_at":   now,
			})
		if err != nil {
			return fmt.Errorf("failed to register tool: %w", err)
		}
		tool.ID = database.GetInt64(existing[0], "id")
	} else {
		id, err := r.db.Insert(ctx, "available_tools", map[string]interface{}{
			"name":                 tool.Name,
			"display_name":         tool.DisplayName,
			"description":          tool.Description,
			"category":             tool.Category,
			"subcategory":          tool.Subcategory,
			"tags":                 string(tagsJSON),
			"parameters":           string(tool.Parameters),
			"enabled":              tool.Enabled,
			"requires_permission":  tool.RequiresPermission,
			"permission_level":     tool.PermissionLevel,
			"handler_type":         tool.HandlerType,
			"handler_config":       string(tool.HandlerConfig),
			"rate_limit_per_minute": tool.RateLimitPerMinute,
			"rate_limit_per_hour":  tool.RateLimitPerHour,
			"timeout_seconds":      tool.TimeoutSeconds,
			"usage_hint":           tool.UsageHint,
			"example_prompts":      string(examplesJSON),
			"depends_on":           string(dependsJSON),
			"conflicts_with":       string(conflictsJSON),
			"priority":             tool.Priority,
			"is_critical":          tool.IsCritical,
			"version":              tool.Version,
			"deprecated":           tool.Deprecated,
			"deprecated_message":   tool.DeprecatedMessage,
			"replacement_tool":     tool.ReplacementTool,
			"created_at":           now,
			"updated_at":           now,
			"total_invocations":    int64(0),
			"successful_invocations": int64(0),
			"failed_invocations":   int64(0),
			"avg_execution_time_ms": 0,
		})
		if err != nil {
			return fmt.Errorf("failed to register tool: %w", err)
		}
		tool.ID = id
	}

	log.Printf("[TOOLS] Ferramenta registrada: %s (ID: %d)", tool.Name, tool.ID)
	return nil
}

// Update atualiza uma ferramenta existente
func (r *ToolRepository) Update(ctx context.Context, tool *ToolRecord) error {
	tagsJSON, _ := json.Marshal(tool.Tags)
	examplesJSON, _ := json.Marshal(tool.ExamplePrompts)
	now := time.Now().UTC().Format(time.RFC3339)

	err := r.db.Update(ctx, "available_tools",
		map[string]interface{}{"name": tool.Name},
		map[string]interface{}{
			"display_name":       tool.DisplayName,
			"description":        tool.Description,
			"category":           tool.Category,
			"tags":               string(tagsJSON),
			"parameters":         string(tool.Parameters),
			"enabled":            tool.Enabled,
			"usage_hint":         tool.UsageHint,
			"example_prompts":    string(examplesJSON),
			"priority":           tool.Priority,
			"is_critical":        tool.IsCritical,
			"version":            tool.Version,
			"deprecated":         tool.Deprecated,
			"deprecated_message": tool.DeprecatedMessage,
			"replacement_tool":   tool.ReplacementTool,
			"updated_at":         now,
		})

	if err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	log.Printf("[TOOLS] Ferramenta atualizada: %s", tool.Name)
	return nil
}

// Delete remove uma ferramenta (soft delete via deprecated)
func (r *ToolRepository) Delete(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	err := r.db.Update(ctx, "available_tools",
		map[string]interface{}{"name": name},
		map[string]interface{}{
			"enabled":            false,
			"deprecated":         true,
			"deprecated_message": "Removida pelo sistema",
			"updated_at":         now,
		})
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	log.Printf("[TOOLS] Ferramenta removida: %s", name)
	return nil
}

// Enable habilita uma ferramenta
func (r *ToolRepository) Enable(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Update(ctx, "available_tools",
		map[string]interface{}{"name": name},
		map[string]interface{}{"enabled": true, "updated_at": now})
}

// Disable desabilita uma ferramenta
func (r *ToolRepository) Disable(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Update(ctx, "available_tools",
		map[string]interface{}{"name": name},
		map[string]interface{}{"enabled": false, "updated_at": now})
}

// LogInvocation registra uma invocação de ferramenta
func (r *ToolRepository) LogInvocation(ctx context.Context, toolName string, idosoID int64, sessionID string,
	invokedBy string, inputParams map[string]interface{}, triggerPhrase string) (int64, error) {

	inputJSON, _ := json.Marshal(inputParams)

	logID, err := r.db.Insert(ctx, "tool_invocation_log", map[string]interface{}{
		"tool_name":        toolName,
		"idoso_id":         idosoID,
		"session_id":       sessionID,
		"invoked_by":       invokedBy,
		"input_parameters": string(inputJSON),
		"trigger_phrase":   triggerPhrase,
		"status":           "running",
		"created_at":       time.Now().UTC().Format(time.RFC3339),
	})

	return logID, err
}

// CompleteInvocation marca uma invocação como concluída
func (r *ToolRepository) CompleteInvocation(ctx context.Context, logID int64, success bool,
	result map[string]interface{}, errorMsg string, executionTimeMs int) error {

	resultJSON, _ := json.Marshal(result)
	status := "success"
	if !success {
		status = "failed"
	}

	err := r.db.Update(ctx, "tool_invocation_log",
		map[string]interface{}{"id": logID},
		map[string]interface{}{
			"status":            status,
			"output_result":     string(resultJSON),
			"error_message":     errorMsg,
			"execution_time_ms": executionTimeMs,
		})
	if err != nil {
		return err
	}

	// Buscar tool_name do log para atualizar estatísticas
	rows, _ := r.db.QueryByLabel(ctx, "tool_invocation_log",
		" AND n.id = $logid",
		map[string]interface{}{"logid": logID}, 1)

	if len(rows) > 0 {
		toolName := database.GetString(rows[0], "tool_name")
		if toolName != "" {
			// Atualizar estatísticas da ferramenta
			toolRows, _ := r.db.QueryByLabel(ctx, "available_tools",
				" AND n.name = $name",
				map[string]interface{}{"name": toolName}, 1)
			if len(toolRows) > 0 {
				t := toolRows[0]
				updates := map[string]interface{}{
					"total_invocations": database.GetInt64(t, "total_invocations") + 1,
					"last_invoked_at":   time.Now().UTC().Format(time.RFC3339),
				}
				if success {
					updates["successful_invocations"] = database.GetInt64(t, "successful_invocations") + 1
				} else {
					updates["failed_invocations"] = database.GetInt64(t, "failed_invocations") + 1
				}
				// Update avg execution time
				total := database.GetInt64(t, "total_invocations") + 1
				oldAvg := int(database.GetInt64(t, "avg_execution_time_ms"))
				newAvg := ((oldAvg * int(total-1)) + executionTimeMs) / int(total)
				updates["avg_execution_time_ms"] = newAvg

				r.db.Update(ctx, "available_tools",
					map[string]interface{}{"name": toolName},
					updates)
			}
		}
	}

	return nil
}

// GetCapabilities retorna todas as capacidades da EVA
func (r *ToolRepository) GetCapabilities(ctx context.Context) ([]*CapabilityRecord, error) {
	rows, err := r.db.QueryByLabel(ctx, "eva_capabilities",
		" AND n.enabled = $enabled",
		map[string]interface{}{"enabled": true}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query capabilities: %w", err)
	}

	var capabilities []*CapabilityRecord
	for _, m := range rows {
		cap := &CapabilityRecord{
			ID:               database.GetInt64(m, "id"),
			CapabilityName:   database.GetString(m, "capability_name"),
			CapabilityType:   database.GetString(m, "capability_type"),
			Description:      database.GetString(m, "description"),
			ShortDescription: database.GetString(m, "short_description"),
			WhenToUse:        database.GetString(m, "when_to_use"),
			WhenNotToUse:     database.GetString(m, "when_not_to_use"),
			Enabled:          database.GetBool(m, "enabled"),
			PromptPriority:   int(database.GetInt64(m, "prompt_priority")),
		}

		if raw := database.GetString(m, "related_tools"); raw != "" {
			json.Unmarshal([]byte(raw), &cap.RelatedTools)
		}
		if raw := database.GetString(m, "example_queries"); raw != "" {
			json.Unmarshal([]byte(raw), &cap.ExampleQueries)
		}

		capabilities = append(capabilities, cap)
	}

	// Sort by prompt_priority DESC
	sort.Slice(capabilities, func(i, j int) bool {
		return capabilities[i].PromptPriority > capabilities[j].PromptPriority
	})

	return capabilities, nil
}

// GetToolsForGemini converte ferramentas do banco para formato Gemini
func (r *ToolRepository) GetToolsForGemini(ctx context.Context) ([]FunctionDeclaration, error) {
	tools, err := r.GetEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var declarations []FunctionDeclaration
	for _, tool := range tools {
		decl := FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
		}

		// Parse parameters JSON para FunctionParameters
		if len(tool.Parameters) > 0 {
			var params FunctionParameters
			if err := json.Unmarshal(tool.Parameters, &params); err == nil {
				decl.Parameters = &params
			}
		}

		declarations = append(declarations, decl)
	}

	return declarations, nil
}
