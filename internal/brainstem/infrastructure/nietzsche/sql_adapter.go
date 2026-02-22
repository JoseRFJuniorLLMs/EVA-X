// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"eva/internal/brainstem/logger"
)

// SqlAdapter provides a high-level interface for EVA services to use
// NietzscheDB's embedded Swartz SQL engine (GlueSQL on RocksDB).
//
// Each adapter is scoped to a single NietzscheDB collection, so SQL tables
// are isolated per collection. Typical usage:
//
//	adapter := nietzsche.NewSqlAdapter(client, "session_data")
//	adapter.CreateTable(ctx, "CREATE TABLE moods (id INTEGER, label TEXT, score FLOAT)")
//	adapter.Insert(ctx, "moods", []string{"id", "label", "score"}, []interface{}{1, "calm", 0.85})
//	rows, _ := adapter.Query(ctx, "SELECT * FROM moods WHERE score > 0.5")
type SqlAdapter struct {
	client     *Client
	collection string // NietzscheDB collection scope (each collection has its own SQL tables)
}

// NewSqlAdapter creates a SQL adapter scoped to a specific NietzscheDB collection.
func NewSqlAdapter(client *Client, collection string) *SqlAdapter {
	if collection == "" {
		collection = "default"
	}
	return &SqlAdapter{
		client:     client,
		collection: collection,
	}
}

// CreateTable executes a CREATE TABLE DDL statement.
func (s *SqlAdapter) CreateTable(ctx context.Context, ddl string) error {
	log := logger.Nietzsche()

	result, err := s.client.SqlExec(ctx, ddl, s.collection)
	if err != nil {
		return fmt.Errorf("SqlAdapter.CreateTable: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("SqlAdapter.CreateTable failed: %s", result.Message)
	}

	log.Info().Str("ddl", ddl).Str("collection", s.collection).Msg("[Swartz] Table created")
	return nil
}

// DropTable drops a SQL table.
func (s *SqlAdapter) DropTable(ctx context.Context, tableName string) error {
	sql := fmt.Sprintf("DROP TABLE %s", tableName)
	_, err := s.client.SqlExec(ctx, sql, s.collection)
	if err != nil {
		return fmt.Errorf("SqlAdapter.DropTable: %w", err)
	}
	return nil
}

// Insert inserts a row using column names and values.
//
// Example:
//
//	adapter.Insert(ctx, "moods", []string{"id", "label", "score"}, []interface{}{1, "calm", 0.85})
func (s *SqlAdapter) Insert(ctx context.Context, table string, columns []string, values []interface{}) error {
	if len(columns) != len(values) {
		return fmt.Errorf("SqlAdapter.Insert: columns(%d) and values(%d) length mismatch", len(columns), len(values))
	}

	// Build INSERT INTO table (col1, col2) VALUES (val1, val2)
	colsStr := strings.Join(columns, ", ")
	placeholders := make([]string, len(values))
	for i, v := range values {
		placeholders[i] = formatSqlValue(v)
	}
	valsStr := strings.Join(placeholders, ", ")

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, colsStr, valsStr)
	_, err := s.client.SqlExec(ctx, sql, s.collection)
	if err != nil {
		return fmt.Errorf("SqlAdapter.Insert: %w", err)
	}
	return nil
}

// Query executes a SQL SELECT query and returns rows as maps.
func (s *SqlAdapter) Query(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	result, err := s.client.SqlQuery(ctx, sql, s.collection)
	if err != nil {
		return nil, fmt.Errorf("SqlAdapter.Query: %w", err)
	}
	return result.Rows, nil
}

// Exec executes a SQL DML statement (INSERT, UPDATE, DELETE) and returns affected row count.
func (s *SqlAdapter) Exec(ctx context.Context, sql string) (int64, error) {
	result, err := s.client.SqlExec(ctx, sql, s.collection)
	if err != nil {
		return 0, fmt.Errorf("SqlAdapter.Exec: %w", err)
	}
	return int64(result.AffectedRows), nil
}

// QueryOne executes a query expecting exactly one row. Returns nil if no rows found.
func (s *SqlAdapter) QueryOne(ctx context.Context, sql string) (map[string]interface{}, error) {
	rows, err := s.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// Collection returns the collection scope for this adapter.
func (s *SqlAdapter) Collection() string {
	return s.collection
}

// formatSqlValue converts a Go value to a SQL literal string.
func formatSqlValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		// Escape single quotes
		escaped := strings.ReplaceAll(val, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32:
		return fmt.Sprintf("%g", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case nil:
		return "NULL"
	default:
		// Attempt JSON serialization for complex types
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("'%v'", val)
		}
		escaped := strings.ReplaceAll(string(b), "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	}
}
