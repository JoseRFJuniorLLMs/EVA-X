// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfawareness

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/hippocampus/knowledge"

	"github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog/log"
)

const (
	codebaseCollection = "eva_codebase"
	vectorDimension    = 3072
)

// SelfAwarenessService provides EVA with introspection capabilities:
// search own codebase, query own databases, update self-knowledge.
type SelfAwarenessService struct {
	db         *sql.DB
	qdrant     *vector.QdrantClient
	embedSvc   *knowledge.EmbeddingService
	cfg        *config.Config
}

// NewSelfAwarenessService creates the self-awareness service.
func NewSelfAwarenessService(db *sql.DB, qdrant *vector.QdrantClient, embedSvc *knowledge.EmbeddingService, cfg *config.Config) *SelfAwarenessService {
	return &SelfAwarenessService{
		db:       db,
		qdrant:   qdrant,
		embedSvc: embedSvc,
		cfg:      cfg,
	}
}

// ---------- Code Search ----------

// SearchCode performs semantic search on the indexed codebase.
func (s *SelfAwarenessService) SearchCode(ctx context.Context, query string, limit int) ([]CodeResult, error) {
	if s.qdrant == nil || s.embedSvc == nil {
		return nil, fmt.Errorf("qdrant ou embedding service indisponivel")
	}
	if limit <= 0 {
		limit = 5
	}

	embedding, err := s.embedSvc.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	points, err := s.qdrant.Search(ctx, codebaseCollection, embedding, uint64(limit), nil)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]CodeResult, 0, len(points))
	for _, p := range points {
		r := CodeResult{Score: p.Score}
		if v, ok := p.Payload["file_path"]; ok {
			r.FilePath = v.GetStringValue()
		}
		if v, ok := p.Payload["package_name"]; ok {
			r.Package = v.GetStringValue()
		}
		if v, ok := p.Payload["summary"]; ok {
			r.Summary = v.GetStringValue()
		}
		if v, ok := p.Payload["functions"]; ok {
			r.Functions = v.GetStringValue()
		}
		if v, ok := p.Payload["structs"]; ok {
			r.Structs = v.GetStringValue()
		}
		results = append(results, r)
	}
	return results, nil
}

// CodeResult represents a codebase search result.
type CodeResult struct {
	FilePath  string  `json:"file_path"`
	Package   string  `json:"package"`
	Summary   string  `json:"summary"`
	Functions string  `json:"functions"`
	Structs   string  `json:"structs"`
	Score     float32 `json:"score"`
}

// ---------- Database Queries (read-only) ----------

// allowedTables is a whitelist of tables EVA can query.
var allowedTables = []string{
	"eva_self_knowledge", "eva_curriculum", "eva_personalidade_criador",
	"eva_memorias_criador", "eva_conhecimento_projeto", "episodic_memories",
	"spaced_repetition_items", "kid_missions", "gtd_tasks", "habits_log",
	"idosos", "agendamentos", "alertas",
}

// QueryPostgres executes a read-only SELECT query on PostgreSQL.
func (s *SelfAwarenessService) QueryPostgres(ctx context.Context, query string) ([]map[string]interface{}, error) {
	normalized := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(normalized, "SELECT") {
		return nil, fmt.Errorf("apenas queries SELECT sao permitidas")
	}
	// Block dangerous keywords
	for _, kw := range []string{"UPDATE ", "DELETE ", "DROP ", "INSERT ", "ALTER ", "TRUNCATE ", "CREATE ", "GRANT "} {
		if strings.Contains(normalized, kw) {
			return nil, fmt.Errorf("query contem keyword proibida: %s", kw)
		}
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		pointers := make([]interface{}, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		if len(results) >= 50 { // Safety limit
			break
		}
	}
	return results, rows.Err()
}

// ---------- Qdrant Collections ----------

// CollectionInfo represents info about a Qdrant collection.
type CollectionInfo struct {
	Name       string `json:"name"`
	PointCount int64  `json:"point_count"`
}

// ListCollections lists all Qdrant collections with point counts.
func (s *SelfAwarenessService) ListCollections(ctx context.Context) ([]CollectionInfo, error) {
	if s.qdrant == nil {
		return nil, fmt.Errorf("qdrant indisponivel")
	}

	// Known collections in EVA
	knownCollections := []string{
		"eva_learnings", "eva_codebase", "eva_self_knowledge", "signifier_chains",
		"gurdjieff_teachings", "osho_insights", "ouspensky_fragments",
		"nietzsche_aphorisms", "rumi_poems", "hafiz_poems",
		"kabir_songs", "zen_koans", "sufi_stories",
		"jung_concepts", "lacan_concepts", "marcus_aurelius",
		"seneca_letters", "epictetus_discourses", "buddha_suttas",
	}

	var infos []CollectionInfo
	for _, name := range knownCollections {
		info, err := s.qdrant.GetCollectionInfo(ctx, name)
		if err != nil {
			continue // Collection may not exist
		}
		count := int64(0)
		if info != nil && info.PointsCount != nil {
			count = int64(*info.PointsCount)
		}
		infos = append(infos, CollectionInfo{Name: name, PointCount: count})
	}
	return infos, nil
}

// ---------- System Stats ----------

// SystemStats represents EVA's overall system statistics.
type SystemStats struct {
	PostgresTables    int    `json:"postgres_tables"`
	QdrantCollections int    `json:"qdrant_collections"`
	QdrantTotalPoints int64  `json:"qdrant_total_points"`
	CurriculumPending int    `json:"curriculum_pending"`
	CurriculumDone    int    `json:"curriculum_done"`
	TotalMemories     int    `json:"total_memories"`
	GoRoutines        int    `json:"go_routines"`
	MemAllocMB        uint64 `json:"mem_alloc_mb"`
	Uptime            string `json:"uptime"`
}

var startTime = time.Now()

// GetSystemStats returns stats about EVA's systems.
func (s *SelfAwarenessService) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{
		GoRoutines: runtime.NumGoroutine(),
		Uptime:     time.Since(startTime).Round(time.Second).String(),
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.MemAllocMB = m.Alloc / 1024 / 1024

	// PostgreSQL stats
	if s.db != nil {
		row := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'")
		row.Scan(&stats.PostgresTables)

		row = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM eva_curriculum WHERE status = 'pending'")
		row.Scan(&stats.CurriculumPending)

		row = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM eva_curriculum WHERE status = 'completed'")
		row.Scan(&stats.CurriculumDone)

		row = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM episodic_memories")
		row.Scan(&stats.TotalMemories)
	}

	// Qdrant stats
	collections, err := s.ListCollections(ctx)
	if err == nil {
		stats.QdrantCollections = len(collections)
		for _, c := range collections {
			stats.QdrantTotalPoints += c.PointCount
		}
	}

	return stats, nil
}

// ---------- Self-Knowledge CRUD ----------

// SearchSelfKnowledge searches EVA's self-knowledge using semantic search (Qdrant)
// with PostgreSQL ILIKE as fallback.
func (s *SelfAwarenessService) SearchSelfKnowledge(ctx context.Context, query string, limit int) ([]SelfKnowledgeItem, error) {
	if limit <= 0 {
		limit = 5
	}

	// Try semantic search first (Qdrant eva_self_knowledge collection)
	if s.qdrant != nil && s.embedSvc != nil {
		embedding, err := s.embedSvc.GenerateEmbedding(ctx, query)
		if err == nil {
			points, err := s.qdrant.Search(ctx, "eva_self_knowledge", embedding, uint64(limit), nil)
			if err == nil && len(points) > 0 {
				var items []SelfKnowledgeItem
				for _, p := range points {
					item := SelfKnowledgeItem{}
					if v, ok := p.Payload["key"]; ok {
						item.Key = v.GetStringValue()
					}
					if v, ok := p.Payload["type"]; ok {
						item.Type = v.GetStringValue()
					}
					if v, ok := p.Payload["title"]; ok {
						item.Title = v.GetStringValue()
					}
					if v, ok := p.Payload["summary"]; ok {
						item.Summary = v.GetStringValue()
					}
					if v, ok := p.Payload["content"]; ok {
						item.Content = v.GetStringValue()
					}
					if v, ok := p.Payload["location"]; ok {
						item.CodeLocation = v.GetStringValue()
					}
					if v, ok := p.Payload["importance"]; ok {
						item.Importance = int(v.GetIntegerValue())
					}
					items = append(items, item)
				}
				return items, nil
			}
		}
	}

	// Fallback: PostgreSQL ILIKE
	sqlQuery := `
		SELECT id, knowledge_type, knowledge_key, title, summary, detailed_content,
			   COALESCE(code_location, ''), importance
		FROM eva_self_knowledge
		WHERE title ILIKE $1 OR summary ILIKE $1 OR detailed_content ILIKE $1
		ORDER BY importance DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, sqlQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var items []SelfKnowledgeItem
	for rows.Next() {
		var item SelfKnowledgeItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Key, &item.Title, &item.Summary, &item.Content, &item.CodeLocation, &item.Importance); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SelfKnowledgeItem represents an entry in eva_self_knowledge.
type SelfKnowledgeItem struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	Key          string `json:"key"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Content      string `json:"content"`
	CodeLocation string `json:"code_location"`
	Importance   int    `json:"importance"`
}

// UpdateSelfKnowledge upserts an entry in eva_self_knowledge.
func (s *SelfAwarenessService) UpdateSelfKnowledge(ctx context.Context, knowledgeType, key, title, summary, content string, importance int) error {
	query := `
		INSERT INTO eva_self_knowledge (knowledge_type, knowledge_key, title, summary, detailed_content, importance, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (knowledge_key) DO UPDATE SET
			title = EXCLUDED.title,
			summary = EXCLUDED.summary,
			detailed_content = EXCLUDED.detailed_content,
			importance = EXCLUDED.importance,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, knowledgeType, key, title, summary, content, importance)
	return err
}

// ---------- Introspect ----------

// IntrospectionReport is a full self-report of EVA's state.
type IntrospectionReport struct {
	Stats          *SystemStats        `json:"stats"`
	Collections    []CollectionInfo    `json:"collections"`
	RecentLearnings []string           `json:"recent_learnings"`
	Personality    map[string]interface{} `json:"personality,omitempty"`
}

// Introspect returns a full self-report of EVA's state.
func (s *SelfAwarenessService) Introspect(ctx context.Context) (*IntrospectionReport, error) {
	report := &IntrospectionReport{}

	// Stats
	stats, err := s.GetSystemStats(ctx)
	if err == nil {
		report.Stats = stats
	}

	// Collections
	collections, err := s.ListCollections(ctx)
	if err == nil {
		report.Collections = collections
	}

	// Recent learnings from curriculum
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx,
			"SELECT topic FROM eva_curriculum WHERE status = 'completed' ORDER BY completed_at DESC LIMIT 5")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var topic string
				if rows.Scan(&topic) == nil {
					report.RecentLearnings = append(report.RecentLearnings, topic)
				}
			}
		}
	}

	return report, nil
}

// ---------- Code Indexing ----------

// FileInfo represents parsed information about a Go source file.
type FileInfo struct {
	Path      string
	Package   string
	Structs   []string
	Functions []string
	LineCount int
}

var (
	rePackage  = regexp.MustCompile(`^package\s+(\w+)`)
	reFunc     = regexp.MustCompile(`^func\s+(?:\([\w\s*]+\)\s+)?(\w+)\(`)
	reStruct   = regexp.MustCompile(`^type\s+(\w+)\s+struct\s*\{`)
	reType     = regexp.MustCompile(`^type\s+(\w+)\s+`)
)

// ParseGoFile extracts metadata from a Go source file.
func ParseGoFile(path string) (*FileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	info := &FileInfo{
		Path:      path,
		LineCount: len(lines),
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := rePackage.FindStringSubmatch(trimmed); m != nil {
			info.Package = m[1]
		}
		if m := reFunc.FindStringSubmatch(trimmed); m != nil {
			info.Functions = append(info.Functions, m[1])
		}
		if m := reStruct.FindStringSubmatch(trimmed); m != nil {
			info.Structs = append(info.Structs, m[1])
		} else if m := reType.FindStringSubmatch(trimmed); m != nil && !strings.Contains(trimmed, "func") {
			info.Structs = append(info.Structs, m[1])
		}
	}

	return info, nil
}

// IndexCodebase indexes all .go files in the given base path into Qdrant.
func (s *SelfAwarenessService) IndexCodebase(ctx context.Context, basePath string) (int, error) {
	if s.qdrant == nil || s.embedSvc == nil {
		return 0, fmt.Errorf("qdrant ou embedding service indisponivel")
	}

	// Ensure collection exists
	s.qdrant.CreateCollection(ctx, codebaseCollection, uint64(vectorDimension))

	// Walk and collect .go files
	var files []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk failed: %w", err)
	}

	indexed := 0
	batchSize := 5
	var batch []*qdrant.PointStruct

	for i, fpath := range files {
		info, err := ParseGoFile(fpath)
		if err != nil {
			log.Warn().Str("file", fpath).Err(err).Msg("[INDEX] Skip file")
			continue
		}

		// Build summary for embedding
		relPath := fpath
		if strings.HasPrefix(fpath, basePath) {
			relPath = fpath[len(basePath):]
			relPath = strings.TrimPrefix(relPath, "/")
			relPath = strings.TrimPrefix(relPath, "\\")
		}

		summary := fmt.Sprintf("Package %s (%s). %d lines.",
			info.Package, relPath, info.LineCount)
		if len(info.Structs) > 0 {
			summary += fmt.Sprintf(" Structs: %s.", strings.Join(info.Structs, ", "))
		}
		if len(info.Functions) > 0 {
			summary += fmt.Sprintf(" Functions: %s.", strings.Join(info.Functions, ", "))
		}

		embedding, err := s.embedSvc.GenerateEmbedding(ctx, summary)
		if err != nil {
			log.Warn().Str("file", relPath).Err(err).Msg("[INDEX] Embedding failed")
			continue
		}

		pointID := uint64(time.Now().UnixNano()/1000000 + int64(i))
		point := vector.CreatePoint(pointID, embedding, map[string]interface{}{
			"file_path":    relPath,
			"package_name": info.Package,
			"summary":      summary,
			"functions":    strings.Join(info.Functions, ", "),
			"structs":      strings.Join(info.Structs, ", "),
			"line_count":   int64(info.LineCount),
			"indexed_at":   time.Now().Format(time.RFC3339),
		})

		batch = append(batch, point)

		if len(batch) >= batchSize {
			if err := s.qdrant.Upsert(ctx, codebaseCollection, batch); err != nil {
				log.Error().Err(err).Msg("[INDEX] Upsert batch failed")
			} else {
				indexed += len(batch)
			}
			batch = batch[:0]
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		if err := s.qdrant.Upsert(ctx, codebaseCollection, batch); err != nil {
			log.Error().Err(err).Msg("[INDEX] Upsert final batch failed")
		} else {
			indexed += len(batch)
		}
	}

	log.Info().Int("indexed", indexed).Int("total_files", len(files)).Msg("[INDEX] Codebase indexing complete")
	return indexed, nil
}
