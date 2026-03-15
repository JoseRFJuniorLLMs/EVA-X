// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
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
	nz        *nietzsche.NietzscheClient // NietzscheDB gRPC client
	aqlClient *nietzsche.NietzscheClient // AQL executor uses same client (alias for clarity)
}

// AqlClient returns the underlying NietzscheDB client for AQL operations.
// The AQL executor (cortex/aql) uses this to dispatch cognitive verbs.
func (db *DB) AqlClient() *nietzsche.NietzscheClient {
	return db.nz
}

// NewNietzscheDB creates a DB backed by NietzscheDB gRPC.
func NewNietzscheDB(nzClient *nietzsche.NietzscheClient) *DB {
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

// contentToCoords generates a 128-dimensional Poincaré-ball coordinate vector
// from the semantic content of the node. This is a deterministic, lightweight
// fallback used when a full Gemini embedding is not available (e.g. low-level
// inserts from the database package).
//
// Algorithm:
//   - Compute FNV-64 hash of the combined content string.
//   - Expand into 128 float64 values via sequential hashing.
//   - Normalise to unit sphere then scale to magnitude ~0.5 (mid-Poincaré ball).
//
// Layers doing a full Gemini embedding (hippocampus, cortex) should call
// InsertNode / MergeNode directly with real coords to override this.
func contentToCoords(content map[string]interface{}) []float64 {
	// Build a deterministic string from the content keys that carry semantics.
	var sb strings.Builder
	for _, key := range []string{"content", "text", "summary", "description", "title", "trigger_phrase"} {
		if v, ok := content[key]; ok && v != nil {
			if s, ok := v.(string); ok && s != "" {
				sb.WriteString(key)
				sb.WriteByte(':')
				sb.WriteString(s)
				sb.WriteByte('|')
			}
		}
	}
	// Fallback: use the node label + id so we never produce a zero vector.
	if sb.Len() == 0 {
		for _, key := range []string{"node_label", "id", "category", "tipo"} {
			if v, ok := content[key]; ok && v != nil {
				sb.WriteString(fmt.Sprintf("%v|", v))
			}
		}
	}

	const dim = 128
	coords := make([]float64, dim)

	seed := sb.String()
	if seed == "" {
		seed = fmt.Sprintf("empty-%d", time.Now().UnixNano())
	}

	// Expand hash into dim floats by hashing successive chunks.
	h := fnv.New64a()
	for i := 0; i < dim; i++ {
		h.Reset()
		_, _ = h.Write([]byte(seed))
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(i))
		_, _ = h.Write(buf[:])
		hv := h.Sum64()
		// Map to [-1, 1]
		coords[i] = float64(int64(hv)) / float64(math.MaxInt64)
	}

	// Normalise to unit length.
	var norm float64
	for _, v := range coords {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	// Scale to magnitude ~0.5 (mid Poincaré ball — neither origin nor boundary).
	const targetMag = 0.5
	for i := range coords {
		coords[i] = coords[i] / norm * targetMag
	}
	return coords
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
// P0-1 FIX: now generates 128D Poincaré coordinates from content hash so KNN
// searches are non-degenerate even without a full Gemini embedding.
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
		Coords:     contentToCoords(content), // P0-1: non-zero coordinates
		Energy:     0.5,                       // P0-2: non-zero energy (avoids NiilistaGc)
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
// All data goes to the eva_mind collection (primary relational store).
func (db *DB) Insert(ctx context.Context, table string, content map[string]interface{}) (int64, error) {
	return db.insertRow(ctx, table, content)
}

// InsertTo creates a new node in a SPECIFIC collection (not eva_mind).
// Use this when you need to write to eva_learnings, eva_curriculum, stories, etc.
// FASE 2 FIX: Allows routing data to the correct collection.
func (db *DB) InsertTo(ctx context.Context, collection string, table string, content map[string]interface{}) (int64, error) {
	if db.nz == nil {
		return 0, fmt.Errorf("NietzscheDB not initialized")
	}
	if collection == "" {
		collection = evaMindCollection
	}
	id := nextID()
	content["node_label"] = table
	content["id"] = id
	_, err := db.nz.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         fmt.Sprintf("%s:%s:%d", collection, table, id),
		Content:    content,
		Coords:     contentToCoords(content), // P0-1: non-zero coordinates
		Energy:     0.5,                       // P0-2: non-zero energy (avoids NiilistaGc)
		NodeType:   "Semantic",
		Collection: collection,
	})
	if err != nil {
		return 0, err
	}
	return id, nil
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

// NQLIn executes a raw NQL query against a SPECIFIC collection.
// FASE 2 FIX: Allows querying eva_learnings, eva_curriculum, stories, etc.
func (db *DB) NQLIn(ctx context.Context, collection string, nql string, params map[string]interface{}) (*nietzsche.QueryResult, error) {
	if db.nz == nil {
		return nil, fmt.Errorf("NietzscheDB not initialized")
	}
	if collection == "" {
		collection = evaMindCollection
	}
	return db.nz.Query(ctx, nql, params, collection)
}

// QueryByLabelIn finds all nodes with a specific node_label in a SPECIFIC collection.
// FASE 2 FIX: Allows querying collections other than eva_mind.
func (db *DB) QueryByLabelIn(ctx context.Context, collection string, label string, extraWhere string, params map[string]interface{}, limit int) ([]map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["nlabel"] = label

	nql := fmt.Sprintf(`MATCH (n) WHERE n.node_label = $nlabel%s RETURN n`, extraWhere)
	if limit > 0 {
		nql += fmt.Sprintf(" LIMIT %d", limit)
	}

	result, err := db.NQLIn(ctx, collection, nql, params)
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

// MergeNode performs a MERGE (upsert) on a node in eva_mind.
// Returns the node ID (created or matched).
func (db *DB) MergeNode(ctx context.Context, table string, matchKeys map[string]interface{},
	onCreateSet map[string]interface{}, onMatchSet map[string]interface{}) (string, bool, error) {
	if db.nz == nil {
		return "", false, fmt.Errorf("NietzscheDB not initialized")
	}
	mk := make(map[string]interface{}, len(matchKeys)+1)
	for k, v := range matchKeys {
		mk[k] = v
	}
	mk["node_label"] = table

	ocs := make(map[string]interface{}, len(onCreateSet)+1)
	for k, v := range onCreateSet {
		ocs[k] = v
	}
	ocs["node_label"] = table

	result, err := db.nz.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection:  evaMindCollection,
		NodeType:    "Semantic",
		MatchKeys:   mk,
		OnCreateSet: ocs,
		OnMatchSet:  onMatchSet,
	})
	if err != nil {
		return "", false, err
	}
	return result.NodeID, result.Created, nil
}

// InsertEdge creates an edge between two nodes in eva_mind.
// edgeType examples: "TRACKS_HABIT", "COMPLETED_ON", "Association".
func (db *DB) InsertEdge(ctx context.Context, fromID, toID, edgeType string, weight float64) (string, error) {
	if db.nz == nil {
		return "", fmt.Errorf("NietzscheDB not initialized")
	}
	return db.nz.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       fromID,
		To:         toID,
		EdgeType:   edgeType,
		Weight:     weight,
		Collection: evaMindCollection,
	})
}

// MergeEdge finds or creates an edge between two nodes in eva_mind (MERGE semantics).
func (db *DB) MergeEdge(ctx context.Context, fromID, toID, edgeType string) (string, error) {
	if db.nz == nil {
		return "", fmt.Errorf("NietzscheDB not initialized")
	}
	result, err := db.nz.MergeEdge(ctx, nietzsche.MergeEdgeOpts{
		Collection: evaMindCollection,
		FromNodeID: fromID,
		ToNodeID:   toID,
		EdgeType:   edgeType,
	})
	if err != nil {
		return "", err
	}
	return result.EdgeID, nil
}

// NzClient returns the underlying NietzscheDB client (for advanced operations).
func (db *DB) NzClient() *nietzsche.NietzscheClient {
	return db.nz
}

// NodeUUID returns the deterministic UUID for a (table, id) pair in eva_mind.
// This is the same UUID used when storing/retrieving nodes.
func (db *DB) NodeUUID(table string, pgID interface{}) string {
	return nodeID(table, pgID)
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
