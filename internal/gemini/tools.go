// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

// ToolDefinition representa o schema de uma função que o Gemini pode chamar
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GetDefaultTools retorna as funções básicas disponíveis para a EVA
func GetDefaultTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"function_declarations": []ToolDefinition{
				{
					Name:        "alert_family",
					Description: "FUNÇÃO DE EMERGÊNCIA: Envia alerta imediato para familiares em situações críticas.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"motivo": map[string]interface{}{
								"type":        "string",
								"description": "Descrição clara do motivo do alerta",
							},
							"urgencia": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"baixa", "média", "alta", "crítica"},
								"description": "Nível de urgência",
							},
						},
						"required": []string{"motivo", "urgencia"},
					},
				},
				{
					Name:        "confirm_medication",
					Description: "Confirma se o idoso tomou o medicamento prescrito.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"medicamento": map[string]interface{}{
								"type":        "string",
								"description": "Nome do medicamento",
							},
							"tomou": map[string]interface{}{
								"type":        "boolean",
								"description": "true se tomou, false se não",
							},
						},
						"required": []string{"medicamento", "tomou"},
					},
				},
			},
		},
	}
}
