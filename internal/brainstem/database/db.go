// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	nietzsche "nietzsche-sdk"
)

const evaMindCollection = "eva_mind"

// idCounter generates unique int64 IDs for new records (no collisions with migrated data).
var idCounter = time.Now().Unix() * 1000

func nextID() int64 {
	return atomic.AddInt64(&idCounter, 1)
}

// DB wraps NietzscheDB for the EVA data layer.
type DB struct {
	nz *nietzsche.NietzscheClient // NietzscheDB gRPC client
}

// NewNietzscheDB creates a DB backed by NietzscheDB gRPC.
func NewNietzscheDB(nzClient *nietzsche.NietzscheClient, _ *sql.DB) *DB {
	return &DB{
		nz: nzClient,
	}
}

func (db *DB) Close() error {
	return nil
}

// ── NietzscheDB internal helpers ──────────────────────────────────────

// nodeID computes the deterministic UUID v5 for a migrated NietzscheDB row.
// Key format matches the pg-to-nietzsche migration tool: "eva_mind:table:pgID".
func nodeID(table string, pgID interface{}) string {
	key := fmt.Sprintf("%s:%s:%v", evaMindCollection, table, pgID)
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(key)).String()
}

// nqlQuery executes an NQL query against the eva_mind collection.
func (db *DB) nqlQuery(ctx context.Context, nql string, params map[string]interface{}) (*nietzsche.QueryResult, error) {
	if db.nz == nil {
		return nil, fmt.Errorf("NietzscheDB not initialized")
	}
	return db.nz.Query(ctx, nql, params, evaMindCollection)
}

// getNode retrieves a single node by its original NietzscheDB table + ID.
func (db *DB) getNode(ctx context.Context, table string, pgID interface{}) (map[string]interface{}, error) {
	if db.nz == nil {
		return nil, fmt.Errorf("NietzscheDB not initialized")
	}
	result, err := db.nz.GetNode(ctx, nodeID(table, pgID), evaMindCollection)
	if err != nil {
		return nil, err
	}
	if !result.Found {
		return nil, nil
	}
	return result.Content, nil
}

// insertRow inserts a new row as a NietzscheDB node with auto-generated int64 ID.
func (db *DB) insertRow(ctx context.Context, table string, content map[string]interface{}) (int64, error) {
	if db.nz == nil {
		return 0, fmt.Errorf("NietzscheDB not initialized")
	}
	id := nextID()
	content["node_label"] = table
	content["id"] = id
	_, err := db.nz.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         nodeID(table, id),
		Content:    content,
		NodeType:   "Semantic",
		Collection: evaMindCollection,
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// insertRowWithID inserts a row with a specific ID.
func (db *DB) insertRowWithID(ctx context.Context, table string, pgID interface{}, content map[string]interface{}) error {
	if db.nz == nil {
		return fmt.Errorf("NietzscheDB not initialized")
	}
	content["node_label"] = table
	content["id"] = pgID
	_, err := db.nz.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         nodeID(table, pgID),
		Content:    content,
		NodeType:   "Semantic",
		Collection: evaMindCollection,
	})
	return err
}

// updateFields updates specific content fields on an existing node via MergeNode.
func (db *DB) updateFields(ctx context.Context, table string, matchKeys map[string]interface{}, updates map[string]interface{}) error {
	if db.nz == nil {
		return fmt.Errorf("NietzscheDB not initialized")
	}
	matchKeys["node_label"] = table
	_, err := db.nz.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection: evaMindCollection,
		NodeType:   "Semantic",
		MatchKeys:  matchKeys,
		OnMatchSet: updates,
	})
	return err
}

// queryNodesByLabel executes NQL to find nodes with a specific node_label.
// Additional WHERE clauses can be appended via extraWhere (e.g., "AND n.status = $status").
func (db *DB) queryNodesByLabel(ctx context.Context, label string, extraWhere string, params map[string]interface{}, limit int) ([]map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["nlabel"] = label

	nql := fmt.Sprintf(`MATCH (n) WHERE n.node_label = $nlabel%s RETURN n`, extraWhere)
	if limit > 0 {
		nql += fmt.Sprintf(" LIMIT %d", limit)
	}

	result, err := db.nqlQuery(ctx, nql, params)
	if err != nil {
		return nil, err
	}

	rows := make([]map[string]interface{}, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		if node.Content != nil {
			rows = append(rows, node.Content)
		}
	}
	return rows, nil
}

// ── Public API for external packages ────────────────────────────────────
// These methods expose NietzscheDB operations to packages outside database/.

// QueryByLabel finds all nodes with a specific node_label.
// extraWhere adds NQL conditions (e.g., " AND n.status = $status").
func (db *DB) QueryByLabel(ctx context.Context, label string, extraWhere string, params map[string]interface{}, limit int) ([]map[string]interface{}, error) {
	return db.queryNodesByLabel(ctx, label, extraWhere, params, limit)
}

// Insert creates a new node with auto-generated int64 ID. Returns the new ID.
func (db *DB) Insert(ctx context.Context, table string, content map[string]interface{}) (int64, error) {
	return db.insertRow(ctx, table, content)
}

// InsertWithID creates a node with a specific ID (for migrations or deterministic IDs).
func (db *DB) InsertWithID(ctx context.Context, table string, pgID interface{}, content map[string]interface{}) error {
	return db.insertRowWithID(ctx, table, pgID, content)
}

// Update modifies fields on nodes matching the given keys.
func (db *DB) Update(ctx context.Context, table string, matchKeys map[string]interface{}, updates map[string]interface{}) error {
	return db.updateFields(ctx, table, matchKeys, updates)
}

// GetNodeByID retrieves a single node by table name + ID.
func (db *DB) GetNodeByID(ctx context.Context, table string, pgID interface{}) (map[string]interface{}, error) {
	return db.getNode(ctx, table, pgID)
}

// NQL executes a raw NQL query against the eva_mind collection.
func (db *DB) NQL(ctx context.Context, nql string, params map[string]interface{}) (*nietzsche.QueryResult, error) {
	return db.nqlQuery(ctx, nql, params)
}

// SoftDelete marks matching nodes as deleted (sets _deleted=true, ativo=false).
func (db *DB) SoftDelete(ctx context.Context, table string, matchKeys map[string]interface{}) error {
	return db.updateFields(ctx, table, matchKeys, map[string]interface{}{
		"_deleted": true,
		"ativo":    false,
	})
}

// Count returns the number of nodes matching label + optional WHERE clause.
func (db *DB) Count(ctx context.Context, label string, extraWhere string, params map[string]interface{}) (int, error) {
	rows, err := db.queryNodesByLabel(ctx, label, extraWhere, params, 0)
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// NzClient returns the underlying NietzscheDB client (for advanced operations).
func (db *DB) NzClient() *nietzsche.NietzscheClient {
	return db.nz
}

// ── Exported type conversion helpers ────────────────────────────────────

// GetString extracts a string from a NietzscheDB content map.
func GetString(m map[string]interface{}, key string) string { return getString(m, key) }

// GetInt64 extracts an int64 from a NietzscheDB content map.
func GetInt64(m map[string]interface{}, key string) int64 { return getInt64(m, key) }

// GetBool extracts a bool from a NietzscheDB content map.
func GetBool(m map[string]interface{}, key string) bool { return getBool(m, key) }

// GetNullBool extracts a sql.NullBool from a NietzscheDB content map.
func GetNullBool(m map[string]interface{}, key string) sql.NullBool { return getNullBool(m, key) }

// GetNullString extracts a sql.NullString from a NietzscheDB content map.
func GetNullString(m map[string]interface{}, key string) sql.NullString { return getNullString(m, key) }

// GetTime extracts a time.Time from a NietzscheDB content map.
func GetTime(m map[string]interface{}, key string) time.Time { return getTime(m, key) }

// GetTimePtr extracts a *time.Time from a NietzscheDB content map (nil if missing).
func GetTimePtr(m map[string]interface{}, key string) *time.Time { return getTimePtr(m, key) }

// GetFloat64 extracts a float64 from a NietzscheDB content map.
func GetFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return f
		}
		if i, ok := v.(int64); ok {
			return float64(i)
		}
		if i, ok := v.(int); ok {
			return float64(i)
		}
	}
	return 0
}

// StripNonDigits removes all non-digit characters from a string (exported wrapper).
func StripNonDigits(s string) string { return stripNonDigits(s) }

// EnsureIndexes creates needed indexes for the eva_mind collection.
func (db *DB) EnsureIndexes(ctx context.Context) error {
	if db.nz == nil {
		return nil
	}
	for _, field := range []string{
		"node_label", "idoso_id", "status", "cpf_hash", "email",
		"session_id", "medication_id", "ativo", "tipo", "sender",
	} {
		_ = db.nz.CreateIndex(ctx, evaMindCollection, field)
	}
	return nil
}

// ── Content map → Go type conversion helpers ──────────────────────────

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
		if f, ok := v.(float64); ok {
			return f != 0
		}
	}
	return false
}

func getNullBool(m map[string]interface{}, key string) sql.NullBool {
	v, ok := m[key]
	if !ok || v == nil {
		return sql.NullBool{}
	}
	if b, ok := v.(bool); ok {
		return sql.NullBool{Bool: b, Valid: true}
	}
	if f, ok := v.(float64); ok {
		return sql.NullBool{Bool: f != 0, Valid: true}
	}
	return sql.NullBool{}
}

func getNullString(m map[string]interface{}, key string) sql.NullString {
	v, ok := m[key]
	if !ok || v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: getString(m, key), Valid: true}
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999-07",
	"2006-01-02 15:04:05-07",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func getTime(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			for _, layout := range timeLayouts {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		}
		if f, ok := v.(float64); ok {
			return time.Unix(int64(f), 0)
		}
	}
	return time.Time{}
}

func getTimePtr(m map[string]interface{}, key string) *time.Time {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	t := getTime(m, key)
	if t.IsZero() {
		return nil
	}
	return &t
}

// stripNonDigits removes all non-digit characters from a string.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
