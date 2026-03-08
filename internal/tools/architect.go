// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tools

import (
	"context"
	"fmt"
)

// UpdateUserDirective atualiza campos na tabela 'idosos' em tempo real
func (h *ToolsHandler) UpdateUserDirective(idosoID int64, directiveType string, newValue string) error {
	ctx := context.Background()

	switch directiveType {
	case "language":
		return h.db.Update(ctx, "idosos",
			map[string]interface{}{"id": idosoID},
			map[string]interface{}{"idioma": newValue})
	case "voice":
		return h.db.Update(ctx, "idosos",
			map[string]interface{}{"id": idosoID},
			map[string]interface{}{"preferred_voice": newValue})
	case "legacy_mode":
		boolValue := newValue == "true"
		return h.db.Update(ctx, "idosos",
			map[string]interface{}{"id": idosoID},
			map[string]interface{}{"legacy_mode": boolValue})
	default:
		return fmt.Errorf("diretiva desconhecida: %s", directiveType)
	}
}
