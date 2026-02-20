// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package selfawareness

import (
	"context"
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/infrastructure/vector"
	"eva/internal/hippocampus/knowledge"

	"github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog/log"
)

const (
	codebaseCollection = "eva_codebase"
	docsCollection     = "eva_docs"
	vectorDimension    = 3072
)

// SelfAwarenessService provides EVA with introspection capabilities:
// search own codebase, query own databases, update self-knowledge.
type SelfAwarenessService struct {
	db       *sql.DB
	qdrant   *vector.QdrantClient
	embedSvc *knowledge.EmbeddingService
	cfg      *config.Config
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

// ======================== AST-Based Types ========================

// StructField represents a single field in a struct.
type StructField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

// StructDef represents a struct definition with all fields.
type StructDef struct {
	Name   string        `json:"name"`
	Fields []StructField `json:"fields"`
}

// InterfaceMethod represents a method in an interface.
type InterfaceMethod struct {
	Name    string `json:"name"`
	Params  string `json:"params"`
	Returns string `json:"returns"`
}

// InterfaceDef represents an interface definition.
type InterfaceDef struct {
	Name    string            `json:"name"`
	Methods []InterfaceMethod `json:"methods"`
}

// FuncDef represents a function or method definition.
type FuncDef struct {
	Name     string `json:"name"`
	Receiver string `json:"receiver,omitempty"`
	Params   string `json:"params"`
	Returns  string `json:"returns"`
}

// ConstDef represents a constant definition.
type ConstDef struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// VarDef represents a variable definition.
type VarDef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FileInfo represents comprehensive parsed information about a Go source file.
type FileInfo struct {
	Path       string         `json:"path"`
	Package    string         `json:"package"`
	Imports    []string       `json:"imports"`
	Structs    []StructDef    `json:"structs"`
	Interfaces []InterfaceDef `json:"interfaces"`
	Functions  []FuncDef      `json:"functions"`
	Constants  []ConstDef     `json:"constants"`
	Variables  []VarDef       `json:"variables"`
	LineCount  int            `json:"line_count"`
}

// ======================== AST Parser ========================

// exprToString converts an ast.Expr to its string representation.
func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprToString(t.Elt)
		}
		return "[...]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + exprToString(t.Value)
	case *ast.FuncType:
		return "func(" + fieldListToString(t.Params) + ")" + fieldListReturns(t.Results)
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.StructType:
		return "struct{}"
	default:
		return "?"
	}
}

// fieldListToString converts a field list to a comma-separated string.
func fieldListToString(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		typeName := exprToString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typeName)
		} else {
			for _, n := range f.Names {
				parts = append(parts, n.Name+" "+typeName)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// fieldListReturns formats return types.
func fieldListReturns(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	s := fieldListToString(fl)
	if len(fl.List) == 1 && len(fl.List[0].Names) == 0 {
		return " " + s
	}
	return " (" + s + ")"
}

// ParseGoFile extracts comprehensive metadata from a Go source file using AST.
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

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		// Fallback: at least get line count and package via simple scan
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "package ") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					info.Package = parts[1]
				}
				break
			}
		}
		return info, nil
	}

	info.Package = file.Name.Name

	// Imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		info.Imports = append(info.Imports, importPath)
	}

	// Walk declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					switch st := s.Type.(type) {
					case *ast.StructType:
						sd := StructDef{Name: s.Name.Name}
						if st.Fields != nil {
							for _, f := range st.Fields.List {
								typeName := exprToString(f.Type)
								tag := ""
								if f.Tag != nil {
									tag = f.Tag.Value
								}
								if len(f.Names) == 0 {
									// Embedded field
									sd.Fields = append(sd.Fields, StructField{
										Name: typeName, Type: "(embedded)", Tag: tag,
									})
								} else {
									for _, n := range f.Names {
										sd.Fields = append(sd.Fields, StructField{
											Name: n.Name, Type: typeName, Tag: tag,
										})
									}
								}
							}
						}
						info.Structs = append(info.Structs, sd)
					case *ast.InterfaceType:
						id := InterfaceDef{Name: s.Name.Name}
						if st.Methods != nil {
							for _, m := range st.Methods.List {
								if ft, ok := m.Type.(*ast.FuncType); ok && len(m.Names) > 0 {
									id.Methods = append(id.Methods, InterfaceMethod{
										Name:    m.Names[0].Name,
										Params:  fieldListToString(ft.Params),
										Returns: strings.TrimSpace(fieldListReturns(ft.Results)),
									})
								}
							}
						}
						info.Interfaces = append(info.Interfaces, id)
					}
				case *ast.ValueSpec:
					switch d.Tok {
					case token.CONST:
						for i, n := range s.Names {
							c := ConstDef{Name: n.Name}
							if s.Type != nil {
								c.Type = exprToString(s.Type)
							}
							if i < len(s.Values) {
								if bl, ok := s.Values[i].(*ast.BasicLit); ok {
									c.Value = bl.Value
								}
							}
							info.Constants = append(info.Constants, c)
						}
					case token.VAR:
						for _, n := range s.Names {
							v := VarDef{Name: n.Name}
							if s.Type != nil {
								v.Type = exprToString(s.Type)
							}
							info.Variables = append(info.Variables, v)
						}
					}
				}
			}
		case *ast.FuncDecl:
			fd := FuncDef{Name: d.Name.Name}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recvType := exprToString(d.Recv.List[0].Type)
				if len(d.Recv.List[0].Names) > 0 {
					fd.Receiver = d.Recv.List[0].Names[0].Name + " " + recvType
				} else {
					fd.Receiver = recvType
				}
			}
			if d.Type != nil {
				fd.Params = fieldListToString(d.Type.Params)
				fd.Returns = strings.TrimSpace(fieldListReturns(d.Type.Results))
			}
			info.Functions = append(info.Functions, fd)
		}
	}

	return info, nil
}

// buildRichSummary builds a comprehensive text summary for embedding.
func buildRichSummary(info *FileInfo, relPath string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("File: %s | Package: %s | Lines: %d\n\n",
		relPath, info.Package, info.LineCount))

	// Imports (only eva-mind internal)
	var internalImports []string
	for _, imp := range info.Imports {
		if strings.HasPrefix(imp, "eva/") {
			internalImports = append(internalImports, imp)
		}
	}
	if len(internalImports) > 0 {
		b.WriteString("Internal imports: " + strings.Join(internalImports, ", ") + "\n\n")
	}

	// Structs with fields
	for _, s := range info.Structs {
		b.WriteString(fmt.Sprintf("struct %s {\n", s.Name))
		for _, f := range s.Fields {
			if f.Tag != "" {
				b.WriteString(fmt.Sprintf("  %s %s %s\n", f.Name, f.Type, f.Tag))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", f.Name, f.Type))
			}
		}
		b.WriteString("}\n\n")
	}

	// Interfaces with methods
	for _, iface := range info.Interfaces {
		b.WriteString(fmt.Sprintf("interface %s {\n", iface.Name))
		for _, m := range iface.Methods {
			b.WriteString(fmt.Sprintf("  %s(%s)%s\n", m.Name, m.Params, m.Returns))
		}
		b.WriteString("}\n\n")
	}

	// Functions and methods
	for _, f := range info.Functions {
		if f.Receiver != "" {
			b.WriteString(fmt.Sprintf("func (%s) %s(%s)%s\n", f.Receiver, f.Name, f.Params, f.Returns))
		} else {
			b.WriteString(fmt.Sprintf("func %s(%s)%s\n", f.Name, f.Params, f.Returns))
		}
	}
	if len(info.Functions) > 0 {
		b.WriteString("\n")
	}

	// Constants
	if len(info.Constants) > 0 {
		b.WriteString("Constants: ")
		var cs []string
		for _, c := range info.Constants {
			if c.Value != "" {
				cs = append(cs, fmt.Sprintf("%s = %s", c.Name, c.Value))
			} else {
				cs = append(cs, c.Name)
			}
		}
		b.WriteString(strings.Join(cs, ", ") + "\n\n")
	}

	// Variables
	if len(info.Variables) > 0 {
		b.WriteString("Variables: ")
		var vs []string
		for _, v := range info.Variables {
			if v.Type != "" {
				vs = append(vs, v.Name+" "+v.Type)
			} else {
				vs = append(vs, v.Name)
			}
		}
		b.WriteString(strings.Join(vs, ", ") + "\n")
	}

	return b.String()
}

// ======================== Code Search ========================

// CodeResult represents a codebase search result.
type CodeResult struct {
	FilePath  string  `json:"file_path"`
	Package   string  `json:"package"`
	Summary   string  `json:"summary"`
	Functions string  `json:"functions"`
	Structs   string  `json:"structs"`
	Score     float32 `json:"score"`
}

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

// ======================== Docs Search ========================

// DocResult represents a documentation search result.
type DocResult struct {
	FilePath string  `json:"file_path"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	Score    float32 `json:"score"`
}

// SearchDocs performs semantic search on the indexed documentation.
func (s *SelfAwarenessService) SearchDocs(ctx context.Context, query string, limit int) ([]DocResult, error) {
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

	points, err := s.qdrant.Search(ctx, docsCollection, embedding, uint64(limit), nil)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]DocResult, 0, len(points))
	for _, p := range points {
		r := DocResult{Score: p.Score}
		if v, ok := p.Payload["file_path"]; ok {
			r.FilePath = v.GetStringValue()
		}
		if v, ok := p.Payload["title"]; ok {
			r.Title = v.GetStringValue()
		}
		if v, ok := p.Payload["content"]; ok {
			r.Content = v.GetStringValue()
		}
		results = append(results, r)
	}
	return results, nil
}

// ======================== Database Queries (read-only) ========================

// QueryPostgres executes a read-only SELECT query on PostgreSQL.
func (s *SelfAwarenessService) QueryPostgres(ctx context.Context, query string) ([]map[string]interface{}, error) {
	normalized := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(normalized, "SELECT") {
		return nil, fmt.Errorf("apenas queries SELECT sao permitidas")
	}
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
		if len(results) >= 50 {
			break
		}
	}
	return results, rows.Err()
}

// ======================== Qdrant Collections ========================

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

	knownCollections := []string{
		"eva_learnings", "eva_codebase", "eva_self_knowledge", "eva_docs",
		"signifier_chains", "speaker_embeddings",
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
			continue
		}
		count := int64(0)
		if info != nil && info.PointsCount != nil {
			count = int64(*info.PointsCount)
		}
		infos = append(infos, CollectionInfo{Name: name, PointCount: count})
	}
	return infos, nil
}

// ======================== System Stats ========================

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

	collections, err := s.ListCollections(ctx)
	if err == nil {
		stats.QdrantCollections = len(collections)
		for _, c := range collections {
			stats.QdrantTotalPoints += c.PointCount
		}
	}

	return stats, nil
}

// ======================== Self-Knowledge CRUD ========================

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
	if s.db == nil {
		return nil, fmt.Errorf("nenhum backend de busca disponivel")
	}

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

// ======================== Introspect ========================

// IntrospectionReport is a full self-report of EVA's state.
type IntrospectionReport struct {
	Stats           *SystemStats           `json:"stats"`
	Collections     []CollectionInfo       `json:"collections"`
	RecentLearnings []string               `json:"recent_learnings"`
	Personality     map[string]interface{} `json:"personality,omitempty"`
}

// Introspect returns a full self-report of EVA's state.
func (s *SelfAwarenessService) Introspect(ctx context.Context) (*IntrospectionReport, error) {
	report := &IntrospectionReport{}

	stats, err := s.GetSystemStats(ctx)
	if err == nil {
		report.Stats = stats
	}

	collections, err := s.ListCollections(ctx)
	if err == nil {
		report.Collections = collections
	}

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

// ======================== Code Indexing (AST) ========================

// IndexCodebase indexes all .go files in the given base path into Qdrant
// using full Go AST parsing (structs with fields, method signatures, interfaces, constants).
func (s *SelfAwarenessService) IndexCodebase(ctx context.Context, basePath string) (int, error) {
	if s.qdrant == nil || s.embedSvc == nil {
		return 0, fmt.Errorf("qdrant ou embedding service indisponivel")
	}

	s.qdrant.CreateCollection(ctx, codebaseCollection, uint64(vectorDimension))

	var files []string
	err := filepath.Walk(basePath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
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

		relPath := fpath
		if strings.HasPrefix(fpath, basePath) {
			relPath = fpath[len(basePath):]
			relPath = strings.TrimPrefix(relPath, "/")
			relPath = strings.TrimPrefix(relPath, "\\")
		}

		summary := buildRichSummary(info, relPath)

		// Truncate for embedding API limit (max ~10k tokens)
		embedText := summary
		if len(embedText) > 8000 {
			embedText = embedText[:8000]
		}

		embedding, err := s.embedSvc.GenerateEmbedding(ctx, embedText)
		if err != nil {
			log.Warn().Str("file", relPath).Err(err).Msg("[INDEX] Embedding failed")
			continue
		}

		// Build function and struct name lists for payload filtering
		var funcNames, structNames []string
		for _, f := range info.Functions {
			sig := f.Name
			if f.Receiver != "" {
				sig = "(" + f.Receiver + ")." + f.Name
			}
			funcNames = append(funcNames, sig)
		}
		for _, s := range info.Structs {
			structNames = append(structNames, s.Name)
		}

		pointID := uint64(time.Now().UnixNano()/1000000 + int64(i))
		point := vector.CreatePoint(pointID, embedding, map[string]interface{}{
			"file_path":    relPath,
			"package_name": info.Package,
			"summary":      summary,
			"functions":    strings.Join(funcNames, ", "),
			"structs":      strings.Join(structNames, ", "),
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
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(batch) > 0 {
		if err := s.qdrant.Upsert(ctx, codebaseCollection, batch); err != nil {
			log.Error().Err(err).Msg("[INDEX] Upsert final batch failed")
		} else {
			indexed += len(batch)
		}
	}

	log.Info().Int("indexed", indexed).Int("total_files", len(files)).Msg("[INDEX] Codebase indexing complete (AST)")
	return indexed, nil
}

// ======================== Docs Indexing (.md) ========================

// IndexDocs indexes all .md files in the given base path into Qdrant.
// Large files are split into chunks for better semantic search.
func (s *SelfAwarenessService) IndexDocs(ctx context.Context, basePath string) (int, error) {
	if s.qdrant == nil || s.embedSvc == nil {
		return 0, fmt.Errorf("qdrant ou embedding service indisponivel")
	}

	s.qdrant.CreateCollection(ctx, docsCollection, uint64(vectorDimension))

	var files []string
	err := filepath.Walk(basePath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk failed: %w", err)
	}

	indexed := 0
	var batch []*qdrant.PointStruct
	batchSize := 3
	chunkID := 0

	for _, fpath := range files {
		data, err := os.ReadFile(fpath)
		if err != nil {
			log.Warn().Str("file", fpath).Err(err).Msg("[INDEX-DOCS] Skip file")
			continue
		}

		relPath := fpath
		if strings.HasPrefix(fpath, basePath) {
			relPath = fpath[len(basePath):]
			relPath = strings.TrimPrefix(relPath, "/")
			relPath = strings.TrimPrefix(relPath, "\\")
		}

		content := string(data)
		title := extractMDTitle(content, relPath)

		// Split large docs into chunks (~3000 chars each, split on headings)
		chunks := splitMDIntoChunks(content, 3000)

		for ci, chunk := range chunks {
			chunkTitle := title
			if len(chunks) > 1 {
				chunkTitle = fmt.Sprintf("%s (part %d/%d)", title, ci+1, len(chunks))
			}

			embedText := fmt.Sprintf("Document: %s\nFile: %s\n\n%s", chunkTitle, relPath, chunk)
			if len(embedText) > 8000 {
				embedText = embedText[:8000]
			}

			embedding, err := s.embedSvc.GenerateEmbedding(ctx, embedText)
			if err != nil {
				log.Warn().Str("file", relPath).Int("chunk", ci).Err(err).Msg("[INDEX-DOCS] Embedding failed")
				continue
			}

			pointID := uint64(time.Now().UnixNano()/1000000 + int64(chunkID))
			chunkID++

			// Truncate content for payload (Qdrant payload limit)
			payloadContent := chunk
			if len(payloadContent) > 4000 {
				payloadContent = payloadContent[:4000]
			}

			point := vector.CreatePoint(pointID, embedding, map[string]interface{}{
				"file_path":  relPath,
				"title":      chunkTitle,
				"content":    payloadContent,
				"chunk":      int64(ci),
				"total":      int64(len(chunks)),
				"indexed_at": time.Now().Format(time.RFC3339),
			})

			batch = append(batch, point)

			if len(batch) >= batchSize {
				if err := s.qdrant.Upsert(ctx, docsCollection, batch); err != nil {
					log.Error().Err(err).Msg("[INDEX-DOCS] Upsert batch failed")
				} else {
					indexed += len(batch)
				}
				batch = batch[:0]
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	if len(batch) > 0 {
		if err := s.qdrant.Upsert(ctx, docsCollection, batch); err != nil {
			log.Error().Err(err).Msg("[INDEX-DOCS] Upsert final batch failed")
		} else {
			indexed += len(batch)
		}
	}

	log.Info().Int("indexed", indexed).Int("total_files", len(files)).Msg("[INDEX-DOCS] Documentation indexing complete")
	return indexed, nil
}

// extractMDTitle extracts the first heading or uses the filename.
func extractMDTitle(content, fallback string) string {
	lines := strings.SplitN(content, "\n", 20)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return filepath.Base(fallback)
}

// splitMDIntoChunks splits markdown content into chunks, preferring to split at headings.
func splitMDIntoChunks(content string, maxChars int) []string {
	if len(content) <= maxChars {
		return []string{content}
	}

	lines := strings.Split(content, "\n")
	var chunks []string
	var current strings.Builder

	for _, line := range lines {
		// If adding this line exceeds the limit and current has content
		if current.Len()+len(line)+1 > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		// If a heading is a natural break point
		if strings.HasPrefix(strings.TrimSpace(line), "## ") && current.Len() > 500 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}
