// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

// MemoryMetadata stores transient metadata for memory processing
type MemoryMetadata struct {
	Emotion    string
	Importance float64
	Topics     []string
}
