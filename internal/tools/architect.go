// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"fmt"
)

// UpdateUserDirective atualiza campos na tabela 'idosos' em tempo real
func (h *ToolsHandler) UpdateUserDirective(idosoID int64, directiveType string, newValue string) error {
	var query string
	var args []interface{}

	switch directiveType {
	case "language":
		query = "UPDATE idosos SET idioma = $1 WHERE id = $2"
		args = []interface{}{newValue, idosoID}
	case "voice":
		query = "UPDATE idosos SET preferred_voice = $1 WHERE id = $2"
		args = []interface{}{newValue, idosoID}
	case "legacy_mode":
		boolValue := newValue == "true"
		query = "UPDATE idosos SET legacy_mode = $1 WHERE id = $2"
		args = []interface{}{boolValue, idosoID}
	default:
		return fmt.Errorf("diretiva desconhecida: %s", directiveType)
	}

	_, err := h.db.Conn.Exec(query, args...)
	return err
}
