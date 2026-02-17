// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

// IdosoProfile encapsula dados ricos sobre o usuário para personalização
type IdosoProfile struct {
	ID        int64
	Name      string
	NeuroType []string // Ex: ["tdah", "ansioso"]
	BaseType  int      // Eneatipo (1-9)
}
