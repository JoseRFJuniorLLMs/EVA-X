// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package utils

import "time"

// Time utility functions
func ParseTimestamp(ts string) (time.Time, error) {
	return time.Parse(time.RFC3339, ts)
}
