package zettelkasten

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// ============================================================================
// ZETTELKASTEN SERVICE - Memória Externa Viva para Idosos
// ============================================================================
// Baseado em Niklas Luhmann's Zettelkasten + Obsidian/Roam concepts
// Adaptado para contexto de idosos: memórias, pessoas, lugares, histórias

// ZettelService gerencia o sistema de notas interconectadas
type ZettelService struct {
	db        *sql.DB
	neo4j     neo4j.DriverWithContext
	extractor *EntityExtractor
}

// Zettel representa uma nota/átomo de conhecimento
type Zettel struct {
	ID            string            `json:"id"`             // Hash único baseado no conteúdo
	IdosoID       int64             `json:"idoso_id"`
	Title         string            `json:"title"`          // Título curto
	Content       string            `json:"content"`        // Conteúdo principal
	ZettelType    ZettelType        `json:"zettel_type"`    // Tipo do zettel
	Source        ZettelSource      `json:"source"`         // De onde veio
	Entities      []Entity          `json:"entities"`       // Pessoas, lugares, datas extraídas
	Tags          []string          `json:"tags"`           // Tags manuais ou automáticas
	LinkedZettels []string          `json:"linked_zettels"` // IDs de zettels relacionados
	Metadata      map[string]string `json:"metadata"`       // Metadados extras
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	AccessCount   int               `json:"access_count"`   // Quantas vezes foi acessado
	LastAccessed  *time.Time        `json:"last_accessed"`
}

// ZettelType tipos de zettel
type ZettelType string

const (
	ZETTEL_MEMORY       ZettelType = "memory"       // Memória/lembrança
	ZETTEL_PERSON       ZettelType = "person"       // Pessoa importante
	ZETTEL_PLACE        ZettelType = "place"        // Lugar significativo
	ZETTEL_EVENT        ZettelType = "event"        // Evento/acontecimento
	ZETTEL_RECIPE       ZettelType = "recipe"       // Receita de família
	ZETTEL_WISDOM       ZettelType = "wisdom"       // Sabedoria/conselho
	ZETTEL_STORY        ZettelType = "story"        // História completa
	ZETTEL_HEALTH       ZettelType = "health"       // Info de saúde
	ZETTEL_PREFERENCE   ZettelType = "preference"   // Preferência pessoal
	ZETTEL_DAILY        ZettelType = "daily"        // Nota diária (automática)
	ZETTEL_FAMILY_NOTE  ZettelType = "family_note"  // Nota da família
)

// ZettelSource origem do zettel
type ZettelSource struct {
	Type      string    `json:"type"`       // "conversation", "family_input", "import", "system"
	SessionID string    `json:"session_id"` // ID da sessão de conversa
	Author    string    `json:"author"`     // Quem criou (idoso, familiar, sistema)
	Timestamp time.Time `json:"timestamp"`
}

// Entity entidade extraída (pessoa, lugar, data)
type Entity struct {
	Type  string `json:"type"`  // "person", "place", "date", "event"
	Value string `json:"value"` // Nome/valor
	Role  string `json:"role"`  // Papel na história (ex: "avô", "cidade natal")
}

// ZettelLink representa uma conexão entre zettels
type ZettelLink struct {
	FromID       string    `json:"from_id"`
	ToID         string    `json:"to_id"`
	LinkType     string    `json:"link_type"`     // "mentions", "related_to", "part_of", "sequel_to"
	Strength     float64   `json:"strength"`      // 0-1, força da conexão
	Context      string    `json:"context"`       // Contexto da conexão
	CreatedAt    time.Time `json:"created_at"`
	Bidirectional bool     `json:"bidirectional"` // Se é link bidirecional
}

// NewZettelService cria novo serviço
func NewZettelService(db *sql.DB, neo4jDriver neo4j.DriverWithContext) *ZettelService {
	svc := &ZettelService{
		db:        db,
		neo4j:     neo4jDriver,
		extractor: NewEntityExtractor(),
	}

	// Criar tabelas se não existirem
	svc.ensureTables()

	return svc
}

// ============================================================================
// CRIAÇÃO DE ZETTELS
// ============================================================================

// CreateFromConversation cria zettel(s) a partir de uma conversa
func (zs *ZettelService) CreateFromConversation(ctx context.Context, idosoID int64, text string, sessionID string) ([]*Zettel, error) {
	log.Printf("📝 [ZETTEL] Processando conversa para idoso %d", idosoID)

	// 1. Extrair entidades do texto
	entities := zs.extractor.Extract(text)

	// 2. Classificar o tipo de conteúdo
	zettelType := zs.classifyContent(text)

	// 3. Gerar título automático
	title := zs.generateTitle(text, zettelType)

	// 4. Criar zettel principal
	zettel := &Zettel{
		ID:         zs.generateID(idosoID, text),
		IdosoID:    idosoID,
		Title:      title,
		Content:    text,
		ZettelType: zettelType,
		Source: ZettelSource{
			Type:      "conversation",
			SessionID: sessionID,
			Author:    "idoso",
			Timestamp: time.Now(),
		},
		Entities:  entities,
		Tags:      zs.extractTags(text, entities),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 5. Salvar no PostgreSQL
	if err := zs.saveZettel(ctx, zettel); err != nil {
		return nil, fmt.Errorf("erro ao salvar zettel: %w", err)
	}

	// 6. Salvar no Neo4j (grafo)
	if err := zs.saveToGraph(ctx, zettel); err != nil {
		log.Printf("⚠️ [ZETTEL] Erro ao salvar no grafo: %v", err)
	}

	// 7. Encontrar e criar links com zettels existentes
	links := zs.findAndCreateLinks(ctx, zettel)
	log.Printf("🔗 [ZETTEL] %d links criados para zettel %s", len(links), zettel.ID)

	// 8. Criar zettels secundários para entidades importantes
	secondaryZettels := zs.createEntityZettels(ctx, idosoID, entities, zettel.ID)

	result := []*Zettel{zettel}
	result = append(result, secondaryZettels...)

	log.Printf("✅ [ZETTEL] Criados %d zettels a partir da conversa", len(result))
	return result, nil
}

// CreateManual cria zettel manualmente (família pode adicionar)
func (zs *ZettelService) CreateManual(ctx context.Context, idosoID int64, title, content string, zettelType ZettelType, author string, tags []string) (*Zettel, error) {
	entities := zs.extractor.Extract(content)

	zettel := &Zettel{
		ID:         zs.generateID(idosoID, content),
		IdosoID:    idosoID,
		Title:      title,
		Content:    content,
		ZettelType: zettelType,
		Source: ZettelSource{
			Type:      "family_input",
			Author:    author,
			Timestamp: time.Now(),
		},
		Entities:  entities,
		Tags:      tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := zs.saveZettel(ctx, zettel); err != nil {
		return nil, err
	}

	zs.saveToGraph(ctx, zettel)
	zs.findAndCreateLinks(ctx, zettel)

	return zettel, nil
}

// CreateDailyNote cria nota diária automática (resumo do dia)
func (zs *ZettelService) CreateDailyNote(ctx context.Context, idosoID int64, date time.Time) (*Zettel, error) {
	// Buscar todas as conversas do dia
	conversations, err := zs.getDayConversations(ctx, idosoID, date)
	if err != nil || len(conversations) == 0 {
		return nil, fmt.Errorf("sem conversas para resumir")
	}

	// Gerar resumo
	summary := zs.generateDailySummary(conversations)

	title := fmt.Sprintf("Diário - %s", date.Format("02/01/2006"))

	zettel := &Zettel{
		ID:         zs.generateID(idosoID, title+date.String()),
		IdosoID:    idosoID,
		Title:      title,
		Content:    summary,
		ZettelType: ZETTEL_DAILY,
		Source: ZettelSource{
			Type:      "system",
			Author:    "eva",
			Timestamp: time.Now(),
		},
		Tags:      []string{"diário", date.Format("2006-01"), date.Weekday().String()},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	zs.saveZettel(ctx, zettel)
	zs.saveToGraph(ctx, zettel)

	return zettel, nil
}

// ============================================================================
// BUSCA E RECUPERAÇÃO
// ============================================================================

// Search busca zettels por texto, tags ou entidades
func (zs *ZettelService) Search(ctx context.Context, idosoID int64, query string, limit int) ([]*Zettel, error) {
	if limit == 0 {
		limit = 10
	}

	sqlQuery := `
		SELECT id, title, content, zettel_type, source, entities, tags,
		       linked_zettels, metadata, created_at, updated_at, access_count
		FROM zettels
		WHERE idoso_id = $1
		  AND (
		    title ILIKE '%' || $2 || '%'
		    OR content ILIKE '%' || $2 || '%'
		    OR $2 = ANY(tags)
		    OR entities::text ILIKE '%' || $2 || '%'
		  )
		ORDER BY
		  CASE WHEN title ILIKE '%' || $2 || '%' THEN 0 ELSE 1 END,
		  access_count DESC,
		  created_at DESC
		LIMIT $3
	`

	rows, err := zs.db.QueryContext(ctx, sqlQuery, idosoID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

// GetRelated busca zettels relacionados a um zettel específico
func (zs *ZettelService) GetRelated(ctx context.Context, zettelID string, depth int) ([]*Zettel, error) {
	if depth == 0 {
		depth = 2
	}

	// Usar Neo4j para busca em grafo
	if zs.neo4j != nil {
		return zs.getRelatedFromGraph(ctx, zettelID, depth)
	}

	// Fallback para PostgreSQL
	return zs.getRelatedFromSQL(ctx, zettelID)
}

// GetByPerson busca zettels que mencionam uma pessoa
func (zs *ZettelService) GetByPerson(ctx context.Context, idosoID int64, personName string) ([]*Zettel, error) {
	query := `
		SELECT id, title, content, zettel_type, source, entities, tags,
		       linked_zettels, metadata, created_at, updated_at, access_count
		FROM zettels
		WHERE idoso_id = $1
		  AND (
		    entities @> $2::jsonb
		    OR content ILIKE '%' || $3 || '%'
		  )
		ORDER BY created_at DESC
	`

	entityFilter := fmt.Sprintf(`[{"type":"person","value":"%s"}]`, personName)

	rows, err := zs.db.QueryContext(ctx, query, idosoID, entityFilter, personName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

// GetByPlace busca zettels sobre um lugar
func (zs *ZettelService) GetByPlace(ctx context.Context, idosoID int64, placeName string) ([]*Zettel, error) {
	query := `
		SELECT id, title, content, zettel_type, source, entities, tags,
		       linked_zettels, metadata, created_at, updated_at, access_count
		FROM zettels
		WHERE idoso_id = $1
		  AND (
		    zettel_type = 'place'
		    OR entities @> $2::jsonb
		    OR content ILIKE '%' || $3 || '%'
		  )
		ORDER BY created_at DESC
	`

	entityFilter := fmt.Sprintf(`[{"type":"place","value":"%s"}]`, placeName)

	rows, err := zs.db.QueryContext(ctx, query, idosoID, entityFilter, placeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

// GetContextForConversation busca zettels relevantes para o contexto atual
func (zs *ZettelService) GetContextForConversation(ctx context.Context, idosoID int64, currentText string, limit int) ([]*Zettel, error) {
	if limit == 0 {
		limit = 5
	}

	// Extrair entidades do texto atual
	entities := zs.extractor.Extract(currentText)

	var allZettels []*Zettel
	seen := make(map[string]bool)

	// Buscar por cada entidade
	for _, entity := range entities {
		var zettels []*Zettel
		var err error

		switch entity.Type {
		case "person":
			zettels, err = zs.GetByPerson(ctx, idosoID, entity.Value)
		case "place":
			zettels, err = zs.GetByPlace(ctx, idosoID, entity.Value)
		default:
			zettels, err = zs.Search(ctx, idosoID, entity.Value, 3)
		}

		if err == nil {
			for _, z := range zettels {
				if !seen[z.ID] {
					seen[z.ID] = true
					allZettels = append(allZettels, z)
				}
			}
		}
	}

	// Buscar por palavras-chave do texto
	keywords := zs.extractKeywords(currentText)
	for _, kw := range keywords {
		zettels, err := zs.Search(ctx, idosoID, kw, 2)
		if err == nil {
			for _, z := range zettels {
				if !seen[z.ID] {
					seen[z.ID] = true
					allZettels = append(allZettels, z)
				}
			}
		}
	}

	// Limitar resultado
	if len(allZettels) > limit {
		allZettels = allZettels[:limit]
	}

	// Incrementar contador de acesso
	for _, z := range allZettels {
		zs.incrementAccessCount(ctx, z.ID)
	}

	return allZettels, nil
}

// ============================================================================
// GRAPH MAP (Visualização tipo Obsidian)
// ============================================================================

// GraphNode nó do grafo para visualização
type GraphNode struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Type     ZettelType `json:"type"`
	Size     int        `json:"size"` // Baseado em access_count
	Color    string     `json:"color"`
}

// GraphEdge aresta do grafo
type GraphEdge struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Strength float64 `json:"strength"`
	Label    string  `json:"label"`
}

// GraphData dados completos do grafo
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetGraphMap retorna dados para visualização do grafo de conhecimento
func (zs *ZettelService) GetGraphMap(ctx context.Context, idosoID int64, centerZettelID string, depth int) (*GraphData, error) {
	if depth == 0 {
		depth = 3
	}

	graph := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Se tiver Neo4j, usar
	if zs.neo4j != nil {
		return zs.getGraphFromNeo4j(ctx, idosoID, centerZettelID, depth)
	}

	// Fallback: construir do PostgreSQL
	zettels, err := zs.getAllZettels(ctx, idosoID, 100)
	if err != nil {
		return nil, err
	}

	// Criar nós
	for _, z := range zettels {
		node := GraphNode{
			ID:    z.ID,
			Label: z.Title,
			Type:  z.ZettelType,
			Size:  z.AccessCount + 1,
			Color: zs.getColorForType(z.ZettelType),
		}
		graph.Nodes = append(graph.Nodes, node)
	}

	// Criar arestas baseado em links
	links, _ := zs.getAllLinks(ctx, idosoID)
	for _, link := range links {
		edge := GraphEdge{
			From:     link.FromID,
			To:       link.ToID,
			Strength: link.Strength,
			Label:    link.LinkType,
		}
		graph.Edges = append(graph.Edges, edge)
	}

	return graph, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func (zs *ZettelService) generateID(idosoID int64, content string) string {
	data := fmt.Sprintf("%d:%s:%d", idosoID, content, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

func (zs *ZettelService) classifyContent(text string) ZettelType {
	textLower := strings.ToLower(text)

	// Detectar tipo baseado em padrões
	if strings.Contains(textLower, "receita") || strings.Contains(textLower, "ingrediente") {
		return ZETTEL_RECIPE
	}
	if strings.Contains(textLower, "conselho") || strings.Contains(textLower, "aprendi que") {
		return ZETTEL_WISDOM
	}
	if strings.Contains(textLower, "quando eu era") || strings.Contains(textLower, "lembro quando") {
		return ZETTEL_MEMORY
	}
	if strings.Contains(textLower, "meu pai") || strings.Contains(textLower, "minha mãe") ||
		strings.Contains(textLower, "meu filho") || strings.Contains(textLower, "minha esposa") {
		return ZETTEL_PERSON
	}
	if strings.Contains(textLower, "cidade") || strings.Contains(textLower, "casa") ||
		strings.Contains(textLower, "fazenda") || strings.Contains(textLower, "sítio") {
		return ZETTEL_PLACE
	}

	return ZETTEL_MEMORY // Default
}

func (zs *ZettelService) generateTitle(text string, zettelType ZettelType) string {
	// Pegar primeira frase ou primeiras palavras
	text = strings.TrimSpace(text)

	// Encontrar primeira frase
	endIdx := strings.IndexAny(text, ".!?")
	if endIdx > 0 && endIdx < 100 {
		return text[:endIdx]
	}

	// Se não tem pontuação, pegar primeiras palavras
	words := strings.Fields(text)
	if len(words) > 8 {
		return strings.Join(words[:8], " ") + "..."
	}

	return text
}

func (zs *ZettelService) extractTags(text string, entities []Entity) []string {
	tags := []string{}

	// Tags baseadas em entidades
	for _, e := range entities {
		if e.Type == "person" {
			tags = append(tags, "pessoa:"+strings.ToLower(e.Value))
		} else if e.Type == "place" {
			tags = append(tags, "lugar:"+strings.ToLower(e.Value))
		}
	}

	// Tags baseadas em padrões
	textLower := strings.ToLower(text)

	tagPatterns := map[string][]string{
		"família":   {"família", "filho", "filha", "neto", "esposa", "marido"},
		"infância":  {"criança", "infância", "escola", "brincadeira"},
		"trabalho":  {"trabalho", "emprego", "profissão", "carreira"},
		"saúde":     {"médico", "remédio", "hospital", "doença"},
		"religião":  {"deus", "igreja", "oração", "fé"},
		"culinária": {"receita", "comida", "cozinha", "ingrediente"},
	}

	for tag, patterns := range tagPatterns {
		for _, p := range patterns {
			if strings.Contains(textLower, p) {
				tags = append(tags, tag)
				break
			}
		}
	}

	return tags
}

func (zs *ZettelService) extractKeywords(text string) []string {
	// Remover stopwords e extrair palavras significativas
	stopwords := map[string]bool{
		"o": true, "a": true, "os": true, "as": true,
		"um": true, "uma": true, "de": true, "da": true,
		"do": true, "em": true, "no": true, "na": true,
		"que": true, "e": true, "é": true, "para": true,
		"com": true, "não": true, "foi": true, "era": true,
		"eu": true, "ele": true, "ela": true, "isso": true,
		"esse": true, "essa": true, "muito": true, "mais": true,
	}

	words := strings.Fields(strings.ToLower(text))
	keywords := []string{}

	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'")
		if len(w) > 3 && !stopwords[w] {
			keywords = append(keywords, w)
		}
	}

	// Limitar a 5 keywords
	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	return keywords
}

func (zs *ZettelService) getColorForType(t ZettelType) string {
	colors := map[ZettelType]string{
		ZETTEL_MEMORY:      "#4CAF50", // Verde
		ZETTEL_PERSON:      "#2196F3", // Azul
		ZETTEL_PLACE:       "#FF9800", // Laranja
		ZETTEL_EVENT:       "#9C27B0", // Roxo
		ZETTEL_RECIPE:      "#F44336", // Vermelho
		ZETTEL_WISDOM:      "#FFD700", // Dourado
		ZETTEL_STORY:       "#00BCD4", // Ciano
		ZETTEL_HEALTH:      "#E91E63", // Rosa
		ZETTEL_PREFERENCE:  "#607D8B", // Cinza
		ZETTEL_DAILY:       "#8BC34A", // Verde claro
		ZETTEL_FAMILY_NOTE: "#3F51B5", // Índigo
	}

	if color, ok := colors[t]; ok {
		return color
	}
	return "#9E9E9E" // Cinza padrão
}

// ============================================================================
// PERSISTÊNCIA
// ============================================================================

func (zs *ZettelService) ensureTables() {
	query := `
		CREATE TABLE IF NOT EXISTS zettels (
			id VARCHAR(32) PRIMARY KEY,
			idoso_id BIGINT NOT NULL REFERENCES idosos(id),
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			zettel_type VARCHAR(50) NOT NULL,
			source JSONB NOT NULL,
			entities JSONB DEFAULT '[]',
			tags TEXT[] DEFAULT '{}',
			linked_zettels TEXT[] DEFAULT '{}',
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			access_count INT DEFAULT 0,
			last_accessed TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_zettels_idoso ON zettels(idoso_id);
		CREATE INDEX IF NOT EXISTS idx_zettels_type ON zettels(zettel_type);
		CREATE INDEX IF NOT EXISTS idx_zettels_tags ON zettels USING GIN(tags);
		CREATE INDEX IF NOT EXISTS idx_zettels_entities ON zettels USING GIN(entities);
		CREATE INDEX IF NOT EXISTS idx_zettels_content ON zettels USING GIN(to_tsvector('portuguese', content));

		CREATE TABLE IF NOT EXISTS zettel_links (
			id SERIAL PRIMARY KEY,
			from_id VARCHAR(32) REFERENCES zettels(id),
			to_id VARCHAR(32) REFERENCES zettels(id),
			link_type VARCHAR(50) NOT NULL,
			strength DECIMAL(3,2) DEFAULT 0.5,
			context TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			bidirectional BOOLEAN DEFAULT true,
			UNIQUE(from_id, to_id, link_type)
		);

		CREATE INDEX IF NOT EXISTS idx_links_from ON zettel_links(from_id);
		CREATE INDEX IF NOT EXISTS idx_links_to ON zettel_links(to_id);
	`

	_, err := zs.db.Exec(query)
	if err != nil {
		log.Printf("⚠️ [ZETTEL] Erro ao criar tabelas: %v", err)
	}
}

func (zs *ZettelService) saveZettel(ctx context.Context, z *Zettel) error {
	sourceJSON, _ := json.Marshal(z.Source)
	entitiesJSON, _ := json.Marshal(z.Entities)
	metadataJSON, _ := json.Marshal(z.Metadata)

	query := `
		INSERT INTO zettels (id, idoso_id, title, content, zettel_type, source, entities, tags, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			title = $3,
			content = $4,
			entities = $7,
			tags = $8,
			metadata = $9,
			updated_at = $11
	`

	_, err := zs.db.ExecContext(ctx, query,
		z.ID, z.IdosoID, z.Title, z.Content, z.ZettelType,
		sourceJSON, entitiesJSON, z.Tags, metadataJSON,
		z.CreatedAt, z.UpdatedAt,
	)

	return err
}

func (zs *ZettelService) saveToGraph(ctx context.Context, z *Zettel) error {
	if zs.neo4j == nil {
		return nil
	}

	session := zs.neo4j.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	// Criar nó do zettel
	_, err := session.Run(ctx, `
		MERGE (z:Zettel {id: $id})
		SET z.idoso_id = $idoso_id,
		    z.title = $title,
		    z.type = $type,
		    z.created_at = datetime($created_at)
	`, map[string]interface{}{
		"id":         z.ID,
		"idoso_id":   z.IdosoID,
		"title":      z.Title,
		"type":       string(z.ZettelType),
		"created_at": z.CreatedAt.Format(time.RFC3339),
	})

	if err != nil {
		return err
	}

	// Criar nós e relações para entidades
	for _, e := range z.Entities {
		_, err = session.Run(ctx, `
			MERGE (e:Entity {type: $type, value: $value, idoso_id: $idoso_id})
			WITH e
			MATCH (z:Zettel {id: $zettel_id})
			MERGE (z)-[:MENTIONS {role: $role}]->(e)
		`, map[string]interface{}{
			"type":      e.Type,
			"value":     e.Value,
			"idoso_id":  z.IdosoID,
			"zettel_id": z.ID,
			"role":      e.Role,
		})
	}

	return err
}

func (zs *ZettelService) findAndCreateLinks(ctx context.Context, z *Zettel) []ZettelLink {
	var links []ZettelLink

	// Buscar zettels com entidades em comum
	for _, entity := range z.Entities {
		query := `
			SELECT id, entities
			FROM zettels
			WHERE idoso_id = $1 AND id != $2
			  AND entities @> $3::jsonb
		`
		entityFilter := fmt.Sprintf(`[{"type":"%s","value":"%s"}]`, entity.Type, entity.Value)

		rows, err := zs.db.QueryContext(ctx, query, z.IdosoID, z.ID, entityFilter)
		if err != nil {
			continue
		}

		for rows.Next() {
			var relatedID string
			var entitiesJSON []byte
			if rows.Scan(&relatedID, &entitiesJSON) == nil {
				link := ZettelLink{
					FromID:       z.ID,
					ToID:         relatedID,
					LinkType:     "shares_entity",
					Strength:     0.7,
					Context:      fmt.Sprintf("Compartilham: %s (%s)", entity.Value, entity.Type),
					CreatedAt:    time.Now(),
					Bidirectional: true,
				}
				zs.saveLink(ctx, link)
				links = append(links, link)
			}
		}
		rows.Close()
	}

	// Buscar zettels com tags em comum
	if len(z.Tags) > 0 {
		query := `
			SELECT id
			FROM zettels
			WHERE idoso_id = $1 AND id != $2
			  AND tags && $3
		`

		rows, err := zs.db.QueryContext(ctx, query, z.IdosoID, z.ID, z.Tags)
		if err == nil {
			for rows.Next() {
				var relatedID string
				if rows.Scan(&relatedID) == nil {
					link := ZettelLink{
						FromID:       z.ID,
						ToID:         relatedID,
						LinkType:     "related_topic",
						Strength:     0.5,
						Context:      "Tags em comum",
						CreatedAt:    time.Now(),
						Bidirectional: true,
					}
					zs.saveLink(ctx, link)
					links = append(links, link)
				}
			}
			rows.Close()
		}
	}

	return links
}

func (zs *ZettelService) saveLink(ctx context.Context, link ZettelLink) error {
	query := `
		INSERT INTO zettel_links (from_id, to_id, link_type, strength, context, bidirectional)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (from_id, to_id, link_type) DO UPDATE SET
			strength = GREATEST(zettel_links.strength, $4)
	`

	_, err := zs.db.ExecContext(ctx, query,
		link.FromID, link.ToID, link.LinkType, link.Strength, link.Context, link.Bidirectional,
	)

	// Se bidirecional, criar link reverso também
	if link.Bidirectional && err == nil {
		zs.db.ExecContext(ctx, query,
			link.ToID, link.FromID, link.LinkType, link.Strength, link.Context, link.Bidirectional,
		)
	}

	return err
}

func (zs *ZettelService) createEntityZettels(ctx context.Context, idosoID int64, entities []Entity, parentID string) []*Zettel {
	var created []*Zettel

	for _, e := range entities {
		if e.Type != "person" && e.Type != "place" {
			continue
		}

		// Verificar se já existe zettel para esta entidade
		var exists bool
		checkQuery := `
			SELECT EXISTS(
				SELECT 1 FROM zettels
				WHERE idoso_id = $1 AND zettel_type = $2
				  AND title ILIKE $3
			)
		`
		zettelType := ZETTEL_PERSON
		if e.Type == "place" {
			zettelType = ZETTEL_PLACE
		}

		zs.db.QueryRowContext(ctx, checkQuery, idosoID, zettelType, "%"+e.Value+"%").Scan(&exists)

		if !exists {
			// Criar zettel para a entidade
			zettel := &Zettel{
				ID:         zs.generateID(idosoID, e.Value+e.Type),
				IdosoID:    idosoID,
				Title:      e.Value,
				Content:    fmt.Sprintf("%s mencionado em conversa", e.Value),
				ZettelType: zettelType,
				Source: ZettelSource{
					Type:      "system",
					Author:    "eva",
					Timestamp: time.Now(),
				},
				Entities: []Entity{e},
				Tags:     []string{e.Type},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := zs.saveZettel(ctx, zettel); err == nil {
				created = append(created, zettel)

				// Criar link com zettel pai
				link := ZettelLink{
					FromID:       parentID,
					ToID:         zettel.ID,
					LinkType:     "mentions",
					Strength:     0.8,
					Context:      fmt.Sprintf("Menciona %s", e.Value),
					CreatedAt:    time.Now(),
					Bidirectional: true,
				}
				zs.saveLink(ctx, link)
			}
		}
	}

	return created
}

func (zs *ZettelService) incrementAccessCount(ctx context.Context, zettelID string) {
	query := `
		UPDATE zettels
		SET access_count = access_count + 1,
		    last_accessed = NOW()
		WHERE id = $1
	`
	zs.db.ExecContext(ctx, query, zettelID)
}

func (zs *ZettelService) scanZettels(rows *sql.Rows) ([]*Zettel, error) {
	var zettels []*Zettel

	for rows.Next() {
		z := &Zettel{}
		var sourceJSON, entitiesJSON, metadataJSON []byte
		var linkedZettels, tags []string

		err := rows.Scan(
			&z.ID, &z.Title, &z.Content, &z.ZettelType,
			&sourceJSON, &entitiesJSON, &tags,
			&linkedZettels, &metadataJSON,
			&z.CreatedAt, &z.UpdatedAt, &z.AccessCount,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(sourceJSON, &z.Source)
		json.Unmarshal(entitiesJSON, &z.Entities)
		json.Unmarshal(metadataJSON, &z.Metadata)
		z.Tags = tags
		z.LinkedZettels = linkedZettels

		zettels = append(zettels, z)
	}

	return zettels, nil
}

func (zs *ZettelService) getDayConversations(ctx context.Context, idosoID int64, date time.Time) ([]string, error) {
	query := `
		SELECT transcricao_completa
		FROM historico_ligacoes
		WHERE idoso_id = $1
		  AND DATE(inicio_chamada) = $2
		  AND transcricao_completa IS NOT NULL
		ORDER BY inicio_chamada
	`

	rows, err := zs.db.QueryContext(ctx, query, idosoID, date.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []string
	for rows.Next() {
		var text string
		if rows.Scan(&text) == nil {
			conversations = append(conversations, text)
		}
	}

	return conversations, nil
}

func (zs *ZettelService) generateDailySummary(conversations []string) string {
	if len(conversations) == 0 {
		return ""
	}

	// Simples: concatenar conversas
	// TODO: usar LLM para gerar resumo
	return strings.Join(conversations, "\n\n---\n\n")
}

func (zs *ZettelService) getAllZettels(ctx context.Context, idosoID int64, limit int) ([]*Zettel, error) {
	query := `
		SELECT id, title, content, zettel_type, source, entities, tags,
		       linked_zettels, metadata, created_at, updated_at, access_count
		FROM zettels
		WHERE idoso_id = $1
		ORDER BY access_count DESC, created_at DESC
		LIMIT $2
	`

	rows, err := zs.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

func (zs *ZettelService) getAllLinks(ctx context.Context, idosoID int64) ([]ZettelLink, error) {
	query := `
		SELECT zl.from_id, zl.to_id, zl.link_type, zl.strength, zl.context, zl.created_at, zl.bidirectional
		FROM zettel_links zl
		JOIN zettels z ON zl.from_id = z.id
		WHERE z.idoso_id = $1
	`

	rows, err := zs.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []ZettelLink
	for rows.Next() {
		var l ZettelLink
		if rows.Scan(&l.FromID, &l.ToID, &l.LinkType, &l.Strength, &l.Context, &l.CreatedAt, &l.Bidirectional) == nil {
			links = append(links, l)
		}
	}

	return links, nil
}

func (zs *ZettelService) getRelatedFromSQL(ctx context.Context, zettelID string) ([]*Zettel, error) {
	query := `
		SELECT z.id, z.title, z.content, z.zettel_type, z.source, z.entities, z.tags,
		       z.linked_zettels, z.metadata, z.created_at, z.updated_at, z.access_count
		FROM zettels z
		JOIN zettel_links zl ON z.id = zl.to_id
		WHERE zl.from_id = $1
		ORDER BY zl.strength DESC
	`

	rows, err := zs.db.QueryContext(ctx, query, zettelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

func (zs *ZettelService) getRelatedFromGraph(ctx context.Context, zettelID string, depth int) ([]*Zettel, error) {
	session := zs.neo4j.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (z:Zettel {id: $id})-[*1..`+fmt.Sprintf("%d", depth)+`]-(related:Zettel)
		WHERE related.id <> $id
		RETURN DISTINCT related.id as id
		LIMIT 20
	`, map[string]interface{}{"id": zettelID})

	if err != nil {
		return nil, err
	}

	var ids []string
	for result.Next(ctx) {
		if id, ok := result.Record().Get("id"); ok {
			ids = append(ids, id.(string))
		}
	}

	if len(ids) == 0 {
		return []*Zettel{}, nil
	}

	// Buscar zettels do PostgreSQL pelos IDs
	query := `
		SELECT id, title, content, zettel_type, source, entities, tags,
		       linked_zettels, metadata, created_at, updated_at, access_count
		FROM zettels
		WHERE id = ANY($1)
	`

	rows, err := zs.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return zs.scanZettels(rows)
}

func (zs *ZettelService) getGraphFromNeo4j(ctx context.Context, idosoID int64, centerID string, depth int) (*GraphData, error) {
	session := zs.neo4j.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	graph := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Query para nós
	var nodeQuery string
	params := map[string]interface{}{"idoso_id": idosoID}

	if centerID != "" {
		nodeQuery = `
			MATCH (center:Zettel {id: $center_id})
			MATCH (z:Zettel)-[*0..` + fmt.Sprintf("%d", depth) + `]-(center)
			WHERE z.idoso_id = $idoso_id
			RETURN DISTINCT z.id as id, z.title as title, z.type as type
		`
		params["center_id"] = centerID
	} else {
		nodeQuery = `
			MATCH (z:Zettel {idoso_id: $idoso_id})
			RETURN z.id as id, z.title as title, z.type as type
			LIMIT 100
		`
	}

	nodeResult, err := session.Run(ctx, nodeQuery, params)
	if err != nil {
		return nil, err
	}

	nodeIDs := make(map[string]bool)
	for nodeResult.Next(ctx) {
		record := nodeResult.Record()
		id, _ := record.Get("id")
		title, _ := record.Get("title")
		nodeType, _ := record.Get("type")

		idStr := id.(string)
		nodeIDs[idStr] = true

		node := GraphNode{
			ID:    idStr,
			Label: title.(string),
			Type:  ZettelType(nodeType.(string)),
			Size:  5,
			Color: zs.getColorForType(ZettelType(nodeType.(string))),
		}
		graph.Nodes = append(graph.Nodes, node)
	}

	// Query para arestas
	edgeQuery := `
		MATCH (z1:Zettel {idoso_id: $idoso_id})-[r]->(z2:Zettel)
		RETURN z1.id as from, z2.id as to, type(r) as rel_type
	`

	edgeResult, err := session.Run(ctx, edgeQuery, params)
	if err == nil {
		for edgeResult.Next(ctx) {
			record := edgeResult.Record()
			from, _ := record.Get("from")
			to, _ := record.Get("to")
			relType, _ := record.Get("rel_type")

			fromStr := from.(string)
			toStr := to.(string)

			// Só incluir edges entre nós que existem
			if nodeIDs[fromStr] && nodeIDs[toStr] {
				edge := GraphEdge{
					From:     fromStr,
					To:       toStr,
					Strength: 0.5,
					Label:    relType.(string),
				}
				graph.Edges = append(graph.Edges, edge)
			}
		}
	}

	return graph, nil
}
