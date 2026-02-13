package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
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
	db *sql.DB
}

// NewToolRepository cria uma nova instância do repository
func NewToolRepository(db *sql.DB) *ToolRepository {
	return &ToolRepository{db: db}
}

// GetAll retorna todas as ferramentas registradas
func (r *ToolRepository) GetAll(ctx context.Context) ([]*ToolRecord, error) {
	query := `
		SELECT
			id, name, COALESCE(display_name, ''), description, category,
			COALESCE(subcategory, ''), COALESCE(tags, '[]'), parameters,
			enabled, requires_permission, permission_level, handler_type,
			COALESCE(handler_config, '{}'), rate_limit_per_minute, rate_limit_per_hour,
			timeout_seconds, COALESCE(usage_hint, ''), COALESCE(example_prompts, '[]'),
			COALESCE(depends_on, '[]'), COALESCE(conflicts_with, '[]'),
			priority, is_critical, version, deprecated,
			COALESCE(deprecated_message, ''), COALESCE(replacement_tool, ''),
			created_at, updated_at, total_invocations, successful_invocations,
			failed_invocations, last_invoked_at, avg_execution_time_ms
		FROM available_tools
		ORDER BY priority DESC, category, name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools: %w", err)
	}
	defer rows.Close()

	return r.scanTools(rows)
}

// GetEnabled retorna apenas ferramentas habilitadas e não depreciadas
func (r *ToolRepository) GetEnabled(ctx context.Context) ([]*ToolRecord, error) {
	query := `
		SELECT
			id, name, COALESCE(display_name, ''), description, category,
			COALESCE(subcategory, ''), COALESCE(tags, '[]'), parameters,
			enabled, requires_permission, permission_level, handler_type,
			COALESCE(handler_config, '{}'), rate_limit_per_minute, rate_limit_per_hour,
			timeout_seconds, COALESCE(usage_hint, ''), COALESCE(example_prompts, '[]'),
			COALESCE(depends_on, '[]'), COALESCE(conflicts_with, '[]'),
			priority, is_critical, version, deprecated,
			COALESCE(deprecated_message, ''), COALESCE(replacement_tool, ''),
			created_at, updated_at, total_invocations, successful_invocations,
			failed_invocations, last_invoked_at, avg_execution_time_ms
		FROM available_tools
		WHERE enabled = true AND deprecated = false
		ORDER BY priority DESC, category, name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled tools: %w", err)
	}
	defer rows.Close()

	return r.scanTools(rows)
}

// GetByCategory retorna ferramentas de uma categoria específica
func (r *ToolRepository) GetByCategory(ctx context.Context, category string) ([]*ToolRecord, error) {
	query := `
		SELECT
			id, name, COALESCE(display_name, ''), description, category,
			COALESCE(subcategory, ''), COALESCE(tags, '[]'), parameters,
			enabled, requires_permission, permission_level, handler_type,
			COALESCE(handler_config, '{}'), rate_limit_per_minute, rate_limit_per_hour,
			timeout_seconds, COALESCE(usage_hint, ''), COALESCE(example_prompts, '[]'),
			COALESCE(depends_on, '[]'), COALESCE(conflicts_with, '[]'),
			priority, is_critical, version, deprecated,
			COALESCE(deprecated_message, ''), COALESCE(replacement_tool, ''),
			created_at, updated_at, total_invocations, successful_invocations,
			failed_invocations, last_invoked_at, avg_execution_time_ms
		FROM available_tools
		WHERE category = $1 AND enabled = true AND deprecated = false
		ORDER BY priority DESC, name
	`

	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools by category: %w", err)
	}
	defer rows.Close()

	return r.scanTools(rows)
}

// GetByName retorna uma ferramenta pelo nome
func (r *ToolRepository) GetByName(ctx context.Context, name string) (*ToolRecord, error) {
	query := `
		SELECT
			id, name, COALESCE(display_name, ''), description, category,
			COALESCE(subcategory, ''), COALESCE(tags, '[]'), parameters,
			enabled, requires_permission, permission_level, handler_type,
			COALESCE(handler_config, '{}'), rate_limit_per_minute, rate_limit_per_hour,
			timeout_seconds, COALESCE(usage_hint, ''), COALESCE(example_prompts, '[]'),
			COALESCE(depends_on, '[]'), COALESCE(conflicts_with, '[]'),
			priority, is_critical, version, deprecated,
			COALESCE(deprecated_message, ''), COALESCE(replacement_tool, ''),
			created_at, updated_at, total_invocations, successful_invocations,
			failed_invocations, last_invoked_at, avg_execution_time_ms
		FROM available_tools
		WHERE name = $1
	`

	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanTool(row)
}

// Register registra uma nova ferramenta
func (r *ToolRepository) Register(ctx context.Context, tool *ToolRecord) error {
	tagsJSON, _ := json.Marshal(tool.Tags)
	examplesJSON, _ := json.Marshal(tool.ExamplePrompts)
	dependsJSON, _ := json.Marshal(tool.DependsOn)
	conflictsJSON, _ := json.Marshal(tool.ConflictsWith)

	query := `
		INSERT INTO available_tools (
			name, display_name, description, category, subcategory,
			tags, parameters, enabled, requires_permission, permission_level,
			handler_type, handler_config, rate_limit_per_minute, rate_limit_per_hour,
			timeout_seconds, usage_hint, example_prompts, depends_on, conflicts_with,
			priority, is_critical, version, deprecated, deprecated_message, replacement_tool
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25
		)
		ON CONFLICT (name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			parameters = EXCLUDED.parameters,
			updated_at = NOW()
		RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query,
		tool.Name, tool.DisplayName, tool.Description, tool.Category, tool.Subcategory,
		tagsJSON, tool.Parameters, tool.Enabled, tool.RequiresPermission, tool.PermissionLevel,
		tool.HandlerType, tool.HandlerConfig, tool.RateLimitPerMinute, tool.RateLimitPerHour,
		tool.TimeoutSeconds, tool.UsageHint, examplesJSON, dependsJSON, conflictsJSON,
		tool.Priority, tool.IsCritical, tool.Version, tool.Deprecated, tool.DeprecatedMessage, tool.ReplacementTool,
	).Scan(&tool.ID)

	if err != nil {
		return fmt.Errorf("failed to register tool: %w", err)
	}

	log.Printf("✅ [TOOLS] Ferramenta registrada: %s (ID: %d)", tool.Name, tool.ID)
	return nil
}

// Update atualiza uma ferramenta existente
func (r *ToolRepository) Update(ctx context.Context, tool *ToolRecord) error {
	tagsJSON, _ := json.Marshal(tool.Tags)
	examplesJSON, _ := json.Marshal(tool.ExamplePrompts)

	query := `
		UPDATE available_tools SET
			display_name = $2,
			description = $3,
			category = $4,
			tags = $5,
			parameters = $6,
			enabled = $7,
			usage_hint = $8,
			example_prompts = $9,
			priority = $10,
			is_critical = $11,
			version = $12,
			deprecated = $13,
			deprecated_message = $14,
			replacement_tool = $15,
			updated_at = NOW()
		WHERE name = $1
	`

	_, err := r.db.ExecContext(ctx, query,
		tool.Name, tool.DisplayName, tool.Description, tool.Category,
		tagsJSON, tool.Parameters, tool.Enabled, tool.UsageHint,
		examplesJSON, tool.Priority, tool.IsCritical, tool.Version,
		tool.Deprecated, tool.DeprecatedMessage, tool.ReplacementTool,
	)

	if err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	log.Printf("✅ [TOOLS] Ferramenta atualizada: %s", tool.Name)
	return nil
}

// Delete remove uma ferramenta (soft delete via deprecated)
func (r *ToolRepository) Delete(ctx context.Context, name string) error {
	query := `
		UPDATE available_tools SET
			enabled = false,
			deprecated = true,
			deprecated_message = 'Removida pelo sistema',
			updated_at = NOW()
		WHERE name = $1
	`

	_, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	log.Printf("🗑️ [TOOLS] Ferramenta removida: %s", name)
	return nil
}

// Enable habilita uma ferramenta
func (r *ToolRepository) Enable(ctx context.Context, name string) error {
	query := `UPDATE available_tools SET enabled = true, updated_at = NOW() WHERE name = $1`
	_, err := r.db.ExecContext(ctx, query, name)
	return err
}

// Disable desabilita uma ferramenta
func (r *ToolRepository) Disable(ctx context.Context, name string) error {
	query := `UPDATE available_tools SET enabled = false, updated_at = NOW() WHERE name = $1`
	_, err := r.db.ExecContext(ctx, query, name)
	return err
}

// LogInvocation registra uma invocação de ferramenta
func (r *ToolRepository) LogInvocation(ctx context.Context, toolName string, idosoID int64, sessionID string,
	invokedBy string, inputParams map[string]interface{}, triggerPhrase string) (int64, error) {

	inputJSON, _ := json.Marshal(inputParams)

	query := `
		INSERT INTO tool_invocation_log (
			tool_name, idoso_id, session_id, invoked_by,
			input_parameters, trigger_phrase, status
		) VALUES ($1, $2, $3, $4, $5, $6, 'running')
		RETURNING id
	`

	var logID int64
	err := r.db.QueryRowContext(ctx, query,
		toolName, idosoID, sessionID, invokedBy,
		inputJSON, triggerPhrase,
	).Scan(&logID)

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

	query := `
		UPDATE tool_invocation_log SET
			status = $2,
			output_result = $3,
			error_message = $4,
			execution_time_ms = $5
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, logID, status, resultJSON, errorMsg, executionTimeMs)
	if err != nil {
		return err
	}

	// Atualizar estatísticas da ferramenta
	var toolName string
	r.db.QueryRowContext(ctx, "SELECT tool_name FROM tool_invocation_log WHERE id = $1", logID).Scan(&toolName)

	if toolName != "" {
		r.db.ExecContext(ctx, "SELECT increment_tool_usage($1, $2, $3)", toolName, success, executionTimeMs)
	}

	return nil
}

// GetCapabilities retorna todas as capacidades da EVA
func (r *ToolRepository) GetCapabilities(ctx context.Context) ([]*CapabilityRecord, error) {
	query := `
		SELECT
			id, capability_name, capability_type, description,
			COALESCE(short_description, ''), COALESCE(related_tools, '[]'),
			COALESCE(when_to_use, ''), COALESCE(when_not_to_use, ''),
			COALESCE(example_queries, '[]'), enabled, prompt_priority
		FROM eva_capabilities
		WHERE enabled = true
		ORDER BY prompt_priority DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query capabilities: %w", err)
	}
	defer rows.Close()

	var capabilities []*CapabilityRecord
	for rows.Next() {
		cap := &CapabilityRecord{}
		var relatedToolsJSON, examplesJSON []byte

		err := rows.Scan(
			&cap.ID, &cap.CapabilityName, &cap.CapabilityType, &cap.Description,
			&cap.ShortDescription, &relatedToolsJSON, &cap.WhenToUse, &cap.WhenNotToUse,
			&examplesJSON, &cap.Enabled, &cap.PromptPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan capability: %w", err)
		}

		json.Unmarshal(relatedToolsJSON, &cap.RelatedTools)
		json.Unmarshal(examplesJSON, &cap.ExampleQueries)

		capabilities = append(capabilities, cap)
	}

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

// Helper: scan múltiplas ferramentas
func (r *ToolRepository) scanTools(rows *sql.Rows) ([]*ToolRecord, error) {
	var tools []*ToolRecord

	for rows.Next() {
		tool, err := r.scanToolFromRow(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

// Helper: scan uma ferramenta de Row
func (r *ToolRepository) scanTool(row *sql.Row) (*ToolRecord, error) {
	tool := &ToolRecord{}
	var tagsJSON, examplesJSON, dependsJSON, conflictsJSON []byte

	err := row.Scan(
		&tool.ID, &tool.Name, &tool.DisplayName, &tool.Description, &tool.Category,
		&tool.Subcategory, &tagsJSON, &tool.Parameters,
		&tool.Enabled, &tool.RequiresPermission, &tool.PermissionLevel, &tool.HandlerType,
		&tool.HandlerConfig, &tool.RateLimitPerMinute, &tool.RateLimitPerHour,
		&tool.TimeoutSeconds, &tool.UsageHint, &examplesJSON,
		&dependsJSON, &conflictsJSON,
		&tool.Priority, &tool.IsCritical, &tool.Version, &tool.Deprecated,
		&tool.DeprecatedMessage, &tool.ReplacementTool,
		&tool.CreatedAt, &tool.UpdatedAt, &tool.TotalInvocations, &tool.SuccessfulInvocations,
		&tool.FailedInvocations, &tool.LastInvokedAt, &tool.AvgExecutionTimeMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan tool: %w", err)
	}

	json.Unmarshal(tagsJSON, &tool.Tags)
	json.Unmarshal(examplesJSON, &tool.ExamplePrompts)
	json.Unmarshal(dependsJSON, &tool.DependsOn)
	json.Unmarshal(conflictsJSON, &tool.ConflictsWith)

	return tool, nil
}

// Helper: scan uma ferramenta de Rows
func (r *ToolRepository) scanToolFromRow(rows *sql.Rows) (*ToolRecord, error) {
	tool := &ToolRecord{}
	var tagsJSON, examplesJSON, dependsJSON, conflictsJSON []byte

	err := rows.Scan(
		&tool.ID, &tool.Name, &tool.DisplayName, &tool.Description, &tool.Category,
		&tool.Subcategory, &tagsJSON, &tool.Parameters,
		&tool.Enabled, &tool.RequiresPermission, &tool.PermissionLevel, &tool.HandlerType,
		&tool.HandlerConfig, &tool.RateLimitPerMinute, &tool.RateLimitPerHour,
		&tool.TimeoutSeconds, &tool.UsageHint, &examplesJSON,
		&dependsJSON, &conflictsJSON,
		&tool.Priority, &tool.IsCritical, &tool.Version, &tool.Deprecated,
		&tool.DeprecatedMessage, &tool.ReplacementTool,
		&tool.CreatedAt, &tool.UpdatedAt, &tool.TotalInvocations, &tool.SuccessfulInvocations,
		&tool.FailedInvocations, &tool.LastInvokedAt, &tool.AvgExecutionTimeMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan tool: %w", err)
	}

	json.Unmarshal(tagsJSON, &tool.Tags)
	json.Unmarshal(examplesJSON, &tool.ExamplePrompts)
	json.Unmarshal(dependsJSON, &tool.DependsOn)
	json.Unmarshal(conflictsJSON, &tool.ConflictsWith)

	return tool, nil
}
