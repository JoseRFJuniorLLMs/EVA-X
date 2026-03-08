// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package util

import (
	"fmt"
	"strconv"
)

// ToFloat64 converts an interface{} value to float64.
// Handles float64, float32, int, int64, int32, and string types.
// Returns 0 for nil or unrecognized types.
func ToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// ToInt converts an interface{} value to int.
// Handles int, int64, int32, float64, and float32 types.
// Returns 0 for nil or unrecognized types.
func ToInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}

// ToInt64 converts an interface{} value to int64.
// Handles int64, int, int32, float64, and float32 types.
// Returns 0 for nil or unrecognized types.
func ToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	default:
		return 0
	}
}

// ToString converts an interface{} value to string.
// Returns "" for nil, the string itself if already a string,
// or fmt.Sprintf("%v", v) for any other type.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
