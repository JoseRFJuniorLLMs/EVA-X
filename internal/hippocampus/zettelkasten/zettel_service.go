// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package zettelkasten

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// ============================================================================
// ZETTELKASTEN SERVICE - Memoria Externa Viva para Idosos
// ============================================================================
// Baseado em Niklas Luhmann's Zettelkasten + Obsidian/Roam concepts
// Adaptado para contexto de idosos: memorias, pessoas, lugares, historias

// ZettelService gerencia o sistema de notas interconectadas
type ZettelService struct {
	db           *database.DB
	graphAdapter *nietzscheInfra.GraphAdapter
	extractor    *EntityExtractor
}

// Zettel representa uma nota/atomo de conhecimento
type Zettel struct {
	ID            string            `json:"id"`             // Hash unico baseado no conteudo
	IdosoID       int64             `json:"idoso_id"`
	Title         string            `json:"title"`          // Titulo curto
	Content       string            `json:"content"`        // Conteudo principal
	ZettelType    ZettelType        `json:"zettel_type"`    // Tipo do zettel
	Source        ZettelSource      `json:"source"`         // De onde veio
	Entities      []Entity          `json:"entities"`       // Pessoas, lugares, datas extraidas
	Tags          []string          `json:"tags"`           // Tags manuais ou automaticas
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
	ZETTEL_MEMORY      ZettelType = "memory"      // Memoria/lembranca
	ZETTEL_PERSON      ZettelType = "person"       // Pessoa importante
	ZETTEL_PLACE       ZettelType = "place"        // Lugar significativo
	ZETTEL_EVENT       ZettelType = "event"        // Evento/acontecimento
	ZETTEL_RECIPE      ZettelType = "recipe"       // Receita de familia
	ZETTEL_WISDOM      ZettelType = "wisdom"       // Sabedoria/conselho
	ZETTEL_STORY       ZettelType = "story"        // Historia completa
	ZETTEL_HEALTH      ZettelType = "health"       // Info de saude
	ZETTEL_PREFERENCE  ZettelType = "preference"   // Preferencia pessoal
	ZETTEL_DAILY       ZettelType = "daily"        // Nota diaria (automatica)
	ZETTEL_FAMILY_NOTE ZettelType = "family_note"  // Nota da familia
)

// ZettelSource origem do zettel
type ZettelSource struct {
	Type      string    `json:"type"`       // "conversation", "family_input", "import", "system"
	SessionID string    `json:"session_id"` // ID da sessao de conversa
	Author    string    `json:"author"`     // Quem criou (idoso, familiar, sistema)
	Timestamp time.Time `json:"timestamp"`
}

// Entity entidade extraida (pessoa, lugar, data)
type Entity struct {
	Type  string `json:"type"`  // "person", "place", "date", "event"
	Value string `json:"value"` // Nome/valor
	Role  string `json:"role"`  // Papel na historia (ex: "avo", "cidade natal")
}

// ZettelLink representa uma conexao entre zettels
type ZettelLink struct {
	FromID        string    `json:"from_id"`
	ToID          string    `json:"to_id"`
	LinkType      string    `json:"link_type"`     // "mentions", "related_to", "part_of", "sequel_to"
	Strength      float64   `json:"strength"`      // 0-1, forca da conexao
	Context       string    `json:"context"`       // Contexto da conexao
	CreatedAt     time.Time `json:"created_at"`
	Bidirectional bool      `json:"bidirectional"` // Se e link bidirecional
}

// NewZettelService cria novo servico
func NewZettelService(db *database.DB, graphAdapter *nietzscheInfra.GraphAdapter) *ZettelService {
	return &ZettelService{
		db:           db,
		graphAdapter: graphAdapter,
		extractor:    NewEntityExtractor(),
	}
}

// ============================================================================
// CRIACAO DE ZETTELS
// ============================================================================

// CreateFromConversation cria zettel(s) a partir de uma conversa
func (zs *ZettelService) CreateFromConversation(ctx context.Context, idosoID int64, text string, sessionID string) ([]*Zettel, error) {
	log.Printf("[ZETTEL] Processando conversa para idoso %d", idosoID)

	// 1. Extrair entidades do texto
	entities := zs.extractor.Extract(text)

	// 2. Classificar o tipo de conteudo
	zettelType := zs.classifyContent(text)

	// 3. Gerar titulo automatico
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

	// 5. Salvar no NietzscheDB
	if err := zs.saveZettel(ctx, zettel); err != nil {
		return nil, fmt.Errorf("erro ao salvar zettel: %w", err)
	}

	// 6. Salvar no grafo
	if err := zs.saveToGraph(ctx, zettel); err != nil {
		log.Printf("[ZETTEL] Erro ao salvar no grafo: %v", err)
	}

	// 7. Encontrar e criar links com zettels existentes
	links := zs.findAndCreateLinks(ctx, zettel)
	log.Printf("[ZETTEL] %d links criados para zettel %s", len(links), zettel.ID)

	// 8. Criar zettels secundarios para entidades importantes
	secondaryZettels := zs.createEntityZettels(ctx, idosoID, entities, zettel.ID)

	result := []*Zettel{zettel}
	result = append(result, secondaryZettels...)

	log.Printf("[ZETTEL] Criados %d zettels a partir da conversa", len(result))
	return result, nil
}

// CreateManual cria zettel manualmente (familia pode adicionar)
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

// CreateDailyNote cria nota diaria automatica (resumo do dia)
func (zs *ZettelService) CreateDailyNote(ctx context.Context, idosoID int64, date time.Time) (*Zettel, error) {
	// Buscar todas as conversas do dia
	conversations, err := zs.getDayConversations(ctx, idosoID, date)
	if err != nil || len(conversations) == 0 {
		return nil, fmt.Errorf("sem conversas para resumir")
	}

	// Gerar resumo
	summary := zs.generateDailySummary(conversations)

	title := fmt.Sprintf("Diario - %s", date.Format("02/01/2006"))

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
		Tags:      []string{"diario", date.Format("2006-01"), date.Weekday().String()},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	zs.saveZettel(ctx, zettel)
	zs.saveToGraph(ctx, zettel)

	return zettel, nil
}

// ============================================================================
// BUSCA E RECUPERACAO
// ============================================================================

// Search busca zettels por texto, tags ou entidades
func (zs *ZettelService) Search(ctx context.Context, idosoID int64, query string, limit int) ([]*Zettel, error) {
	if limit == 0 {
		limit = 10
	}

	// Query all zettels for this idoso, then filter in Go (NQL doesn't support ILIKE)
	rows, err := zs.db.QueryByLabel(ctx, "zettels",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var zettels []*Zettel

	for _, m := range rows {
		title := database.GetString(m, "title")
		content := database.GetString(m, "content")
		entitiesStr := database.GetString(m, "entities")

		titleMatch := strings.Contains(strings.ToLower(title), queryLower)
		contentMatch := strings.Contains(strings.ToLower(content), queryLower)
		entityMatch := strings.Contains(strings.ToLower(entitiesStr), queryLower)

		// Check tags
		tagMatch := false
		tags := getStringSlice(m, "tags")
		for _, t := range tags {
			if strings.EqualFold(t, query) {
				tagMatch = true
				break
			}
		}

		if titleMatch || contentMatch || tagMatch || entityMatch {
			z := contentToZettel(m)
			zettels = append(zettels, z)
		}
	}

	// Sort: title matches first, then by access_count DESC, then created_at DESC
	sort.Slice(zettels, func(i, j int) bool {
		iTitle := strings.Contains(strings.ToLower(zettels[i].Title), queryLower)
		jTitle := strings.Contains(strings.ToLower(zettels[j].Title), queryLower)
		if iTitle != jTitle {
			return iTitle
		}
		if zettels[i].AccessCount != zettels[j].AccessCount {
			return zettels[i].AccessCount > zettels[j].AccessCount
		}
		return zettels[i].CreatedAt.After(zettels[j].CreatedAt)
	})

	if len(zettels) > limit {
		zettels = zettels[:limit]
	}

	return zettels, nil
}

// GetRelated busca zettels relacionados a um zettel especifico
func (zs *ZettelService) GetRelated(ctx context.Context, zettelID string, depth int) ([]*Zettel, error) {
	if depth == 0 {
		depth = 2
	}

	// Usar NietzscheDB para busca em grafo via BFS
	if zs.graphAdapter != nil {
		return zs.getRelatedFromGraph(ctx, zettelID, depth)
	}

	// Fallback: query links from NietzscheDB
	return zs.getRelatedFromLinks(ctx, zettelID)
}

// GetByPerson busca zettels que mencionam uma pessoa
func (zs *ZettelService) GetByPerson(ctx context.Context, idosoID int64, personName string) ([]*Zettel, error) {
	rows, err := zs.db.QueryByLabel(ctx, "zettels",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	personLower := strings.ToLower(personName)
	var zettels []*Zettel

	for _, m := range rows {
		// Check entities for person match
		entityMatch := false
		entities := getEntitiesFromContent(m)
		for _, e := range entities {
			if e.Type == "person" && strings.EqualFold(e.Value, personName) {
				entityMatch = true
				break
			}
		}

		// Check content for person name mention
		content := database.GetString(m, "content")
		contentMatch := strings.Contains(strings.ToLower(content), personLower)

		if entityMatch || contentMatch {
			z := contentToZettel(m)
			zettels = append(zettels, z)
		}
	}

	// Sort by created_at DESC
	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].CreatedAt.After(zettels[j].CreatedAt)
	})

	return zettels, nil
}

// GetByPlace busca zettels sobre um lugar
func (zs *ZettelService) GetByPlace(ctx context.Context, idosoID int64, placeName string) ([]*Zettel, error) {
	rows, err := zs.db.QueryByLabel(ctx, "zettels",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	placeLower := strings.ToLower(placeName)
	var zettels []*Zettel

	for _, m := range rows {
		zettelType := database.GetString(m, "zettel_type")
		typeMatch := zettelType == "place"

		// Check entities for place match
		entityMatch := false
		entities := getEntitiesFromContent(m)
		for _, e := range entities {
			if e.Type == "place" && strings.EqualFold(e.Value, placeName) {
				entityMatch = true
				break
			}
		}

		// Check content for place name mention
		content := database.GetString(m, "content")
		contentMatch := strings.Contains(strings.ToLower(content), placeLower)

		if typeMatch || entityMatch || contentMatch {
			z := contentToZettel(m)
			zettels = append(zettels, z)
		}
	}

	// Sort by created_at DESC
	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].CreatedAt.After(zettels[j].CreatedAt)
	})

	return zettels, nil
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
// GRAPH MAP (Visualizacao tipo Obsidian)
// ============================================================================

// GraphNode no do grafo para visualizacao
type GraphNode struct {
	ID    string     `json:"id"`
	Label string     `json:"label"`
	Type  ZettelType `json:"type"`
	Size  int        `json:"size"` // Baseado em access_count
	Color string     `json:"color"`
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

// GetGraphMap retorna dados para visualizacao do grafo de conhecimento
func (zs *ZettelService) GetGraphMap(ctx context.Context, idosoID int64, centerZettelID string, depth int) (*GraphData, error) {
	if depth == 0 {
		depth = 3
	}

	graphData := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Se tiver graphAdapter, usar NietzscheDB
	if zs.graphAdapter != nil {
		return zs.getGraphFromNietzsche(ctx, idosoID, centerZettelID, depth)
	}

	// Fallback: construir do NietzscheDB
	zettels, err := zs.getAllZettels(ctx, idosoID, 100)
	if err != nil {
		return nil, err
	}

	// Criar nos
	for _, z := range zettels {
		node := GraphNode{
			ID:    z.ID,
			Label: z.Title,
			Type:  z.ZettelType,
			Size:  z.AccessCount + 1,
			Color: zs.getColorForType(z.ZettelType),
		}
		graphData.Nodes = append(graphData.Nodes, node)
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
		graphData.Edges = append(graphData.Edges, edge)
	}

	return graphData, nil
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

	// Detectar tipo baseado em padroes
	if strings.Contains(textLower, "receita") || strings.Contains(textLower, "ingrediente") {
		return ZETTEL_RECIPE
	}
	if strings.Contains(textLower, "conselho") || strings.Contains(textLower, "aprendi que") {
		return ZETTEL_WISDOM
	}
	if strings.Contains(textLower, "quando eu era") || strings.Contains(textLower, "lembro quando") {
		return ZETTEL_MEMORY
	}
	if strings.Contains(textLower, "meu pai") || strings.Contains(textLower, "minha mae") ||
		strings.Contains(textLower, "meu filho") || strings.Contains(textLower, "minha esposa") {
		return ZETTEL_PERSON
	}
	if strings.Contains(textLower, "cidade") || strings.Contains(textLower, "casa") ||
		strings.Contains(textLower, "fazenda") || strings.Contains(textLower, "sitio") {
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

	// Se nao tem pontuacao, pegar primeiras palavras
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

	// Tags baseadas em padroes
	textLower := strings.ToLower(text)

	tagPatterns := map[string][]string{
		"familia":   {"familia", "filho", "filha", "neto", "esposa", "marido"},
		"infancia":  {"crianca", "infancia", "escola", "brincadeira"},
		"trabalho":  {"trabalho", "emprego", "profissao", "carreira"},
		"saude":     {"medico", "remedio", "hospital", "doenca"},
		"religiao":  {"deus", "igreja", "oracao", "fe"},
		"culinaria": {"receita", "comida", "cozinha", "ingrediente"},
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
		"que": true, "e": true, "para": true,
		"com": true, "foi": true, "era": true,
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
		ZETTEL_FAMILY_NOTE: "#3F51B5", // Indigo
	}

	if color, ok := colors[t]; ok {
		return color
	}
	return "#9E9E9E" // Cinza padrao
}

// ============================================================================
// PERSISTENCIA - NietzscheDB
// ============================================================================

func (zs *ZettelService) saveZettel(ctx context.Context, z *Zettel) error {
	sourceJSON, _ := json.Marshal(z.Source)
	entitiesJSON, _ := json.Marshal(z.Entities)
	metadataJSON, _ := json.Marshal(z.Metadata)
	tagsJSON, _ := json.Marshal(z.Tags)
	linkedJSON, _ := json.Marshal(z.LinkedZettels)

	content := map[string]interface{}{
		"id":              z.ID,
		"idoso_id":        z.IdosoID,
		"title":           z.Title,
		"content":         z.Content,
		"zettel_type":     string(z.ZettelType),
		"source":          string(sourceJSON),
		"entities":        string(entitiesJSON),
		"tags":            string(tagsJSON),
		"linked_zettels":  string(linkedJSON),
		"metadata":        string(metadataJSON),
		"created_at":      z.CreatedAt.Format(time.RFC3339),
		"updated_at":      z.UpdatedAt.Format(time.RFC3339),
		"access_count":    z.AccessCount,
	}

	// Try to get existing node first (upsert logic)
	existing, err := zs.db.GetNodeByID(ctx, "zettels", z.ID)
	if err == nil && existing != nil {
		// Update existing zettel
		return zs.db.Update(ctx, "zettels",
			map[string]interface{}{"id": z.ID},
			map[string]interface{}{
				"title":      z.Title,
				"content":    z.Content,
				"entities":   string(entitiesJSON),
				"tags":       string(tagsJSON),
				"metadata":   string(metadataJSON),
				"updated_at": z.UpdatedAt.Format(time.RFC3339),
			})
	}

	// Insert new zettel
	return zs.db.InsertWithID(ctx, "zettels", z.ID, content)
}

// saveToGraph saves a zettel and its entities to NietzscheDB graph
// Uses NietzscheDB MergeNode/MergeEdge for graph persistence
func (zs *ZettelService) saveToGraph(ctx context.Context, z *Zettel) error {
	if zs.graphAdapter == nil {
		return nil
	}

	// 1. MERGE Zettel node
	zettelResult, err := zs.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Zettel",
		MatchKeys: map[string]interface{}{
			"id": z.ID,
		},
		OnCreateSet: map[string]interface{}{
			"idoso_id":   z.IdosoID,
			"title":      z.Title,
			"type":       string(z.ZettelType),
			"created_at": nietzscheInfra.NowUnix(),
		},
		OnMatchSet: map[string]interface{}{
			"idoso_id":   z.IdosoID,
			"title":      z.Title,
			"type":       string(z.ZettelType),
			"created_at": nietzscheInfra.NowUnix(),
		},
	})
	if err != nil {
		return err
	}

	// 2. Create entity nodes and MENTIONS edges
	for _, e := range z.Entities {
		// MERGE Entity node
		entityResult, err := zs.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType: "Entity",
			MatchKeys: map[string]interface{}{
				"type":     e.Type,
				"value":    e.Value,
				"idoso_id": z.IdosoID,
			},
		})
		if err != nil {
			continue
		}

		// MERGE MENTIONS edge from Zettel to Entity
		_, err = zs.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
			FromNodeID: zettelResult.NodeID,
			ToNodeID:   entityResult.NodeID,
			EdgeType:   "MENTIONS",
			OnCreateSet: map[string]interface{}{
				"role": e.Role,
			},
		})
		if err != nil {
			log.Printf("[ZETTEL] Aviso: falha ao criar edge MENTIONS: %v", err)
		}
	}

	return nil
}

func (zs *ZettelService) findAndCreateLinks(ctx context.Context, z *Zettel) []ZettelLink {
	var links []ZettelLink

	// Buscar zettels com entidades em comum
	for _, entity := range z.Entities {
		rows, err := zs.db.QueryByLabel(ctx, "zettels",
			` AND n.idoso_id = $idoso_id`, map[string]interface{}{
				"idoso_id": z.IdosoID,
			}, 0)
		if err != nil {
			continue
		}

		for _, m := range rows {
			relatedID := database.GetString(m, "id")
			if relatedID == z.ID {
				continue
			}

			// Check if this zettel contains the same entity
			relatedEntities := getEntitiesFromContent(m)
			found := false
			for _, re := range relatedEntities {
				if re.Type == entity.Type && strings.EqualFold(re.Value, entity.Value) {
					found = true
					break
				}
			}
			if !found {
				continue
			}

			link := ZettelLink{
				FromID:        z.ID,
				ToID:          relatedID,
				LinkType:      "shares_entity",
				Strength:      0.7,
				Context:       fmt.Sprintf("Compartilham: %s (%s)", entity.Value, entity.Type),
				CreatedAt:     time.Now(),
				Bidirectional: true,
			}
			zs.saveLink(ctx, link)
			links = append(links, link)
		}
	}

	// Buscar zettels com tags em comum
	if len(z.Tags) > 0 {
		rows, err := zs.db.QueryByLabel(ctx, "zettels",
			` AND n.idoso_id = $idoso_id`, map[string]interface{}{
				"idoso_id": z.IdosoID,
			}, 0)
		if err == nil {
			zTagSet := make(map[string]bool)
			for _, t := range z.Tags {
				zTagSet[t] = true
			}

			for _, m := range rows {
				relatedID := database.GetString(m, "id")
				if relatedID == z.ID {
					continue
				}

				// Check for overlapping tags
				relatedTags := getStringSlice(m, "tags")
				hasCommon := false
				for _, rt := range relatedTags {
					if zTagSet[rt] {
						hasCommon = true
						break
					}
				}
				if !hasCommon {
					continue
				}

				link := ZettelLink{
					FromID:        z.ID,
					ToID:          relatedID,
					LinkType:      "related_topic",
					Strength:      0.5,
					Context:       "Tags em comum",
					CreatedAt:     time.Now(),
					Bidirectional: true,
				}
				zs.saveLink(ctx, link)
				links = append(links, link)
			}
		}
	}

	return links
}

func (zs *ZettelService) saveLink(ctx context.Context, link ZettelLink) error {
	linkID := fmt.Sprintf("%s_%s_%s", link.FromID, link.ToID, link.LinkType)

	content := map[string]interface{}{
		"link_id":       linkID,
		"from_id":       link.FromID,
		"to_id":         link.ToID,
		"link_type":     link.LinkType,
		"strength":      link.Strength,
		"context":       link.Context,
		"created_at":    link.CreatedAt.Format(time.RFC3339),
		"bidirectional": link.Bidirectional,
	}

	// Try to get existing link (upsert logic)
	existing, err := zs.db.GetNodeByID(ctx, "zettel_links", linkID)
	if err == nil && existing != nil {
		// Update: keep the highest strength
		existingStrength := database.GetFloat64(existing, "strength")
		if link.Strength > existingStrength {
			zs.db.Update(ctx, "zettel_links",
				map[string]interface{}{"link_id": linkID},
				map[string]interface{}{"strength": link.Strength})
		}
	} else {
		// Insert new link
		zs.db.InsertWithID(ctx, "zettel_links", linkID, content)
	}

	// Se bidirecional, criar link reverso tambem
	if link.Bidirectional {
		reverseLinkID := fmt.Sprintf("%s_%s_%s", link.ToID, link.FromID, link.LinkType)
		reverseContent := map[string]interface{}{
			"link_id":       reverseLinkID,
			"from_id":       link.ToID,
			"to_id":         link.FromID,
			"link_type":     link.LinkType,
			"strength":      link.Strength,
			"context":       link.Context,
			"created_at":    link.CreatedAt.Format(time.RFC3339),
			"bidirectional": link.Bidirectional,
		}

		existingReverse, err := zs.db.GetNodeByID(ctx, "zettel_links", reverseLinkID)
		if err == nil && existingReverse != nil {
			existingStrength := database.GetFloat64(existingReverse, "strength")
			if link.Strength > existingStrength {
				zs.db.Update(ctx, "zettel_links",
					map[string]interface{}{"link_id": reverseLinkID},
					map[string]interface{}{"strength": link.Strength})
			}
		} else {
			zs.db.InsertWithID(ctx, "zettel_links", reverseLinkID, reverseContent)
		}
	}

	return nil
}

func (zs *ZettelService) createEntityZettels(ctx context.Context, idosoID int64, entities []Entity, parentID string) []*Zettel {
	var created []*Zettel

	for _, e := range entities {
		if e.Type != "person" && e.Type != "place" {
			continue
		}

		zettelType := ZETTEL_PERSON
		if e.Type == "place" {
			zettelType = ZETTEL_PLACE
		}

		// Verificar se ja existe zettel para esta entidade
		rows, err := zs.db.QueryByLabel(ctx, "zettels",
			` AND n.idoso_id = $idoso_id AND n.zettel_type = $zettel_type`,
			map[string]interface{}{
				"idoso_id":    idosoID,
				"zettel_type": string(zettelType),
			}, 0)
		if err != nil {
			continue
		}

		// Filter: check if title contains the entity value (case insensitive)
		exists := false
		entityLower := strings.ToLower(e.Value)
		for _, m := range rows {
			title := database.GetString(m, "title")
			if strings.Contains(strings.ToLower(title), entityLower) {
				exists = true
				break
			}
		}

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
				Entities:  []Entity{e},
				Tags:      []string{e.Type},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := zs.saveZettel(ctx, zettel); err == nil {
				created = append(created, zettel)

				// Criar link com zettel pai
				link := ZettelLink{
					FromID:        parentID,
					ToID:          zettel.ID,
					LinkType:      "mentions",
					Strength:      0.8,
					Context:       fmt.Sprintf("Menciona %s", e.Value),
					CreatedAt:     time.Now(),
					Bidirectional: true,
				}
				zs.saveLink(ctx, link)
			}
		}
	}

	return created
}

func (zs *ZettelService) incrementAccessCount(ctx context.Context, zettelID string) {
	// Get current access_count, then update with incremented value
	existing, err := zs.db.GetNodeByID(ctx, "zettels", zettelID)
	if err != nil || existing == nil {
		return
	}

	currentCount := int(database.GetInt64(existing, "access_count"))
	zs.db.Update(ctx, "zettels",
		map[string]interface{}{"id": zettelID},
		map[string]interface{}{
			"access_count":  currentCount + 1,
			"last_accessed": time.Now().Format(time.RFC3339),
		})
}

func (zs *ZettelService) getDayConversations(ctx context.Context, idosoID int64, date time.Time) ([]string, error) {
	rows, err := zs.db.QueryByLabel(ctx, "historico_ligacoes",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	dateStr := date.Format("2006-01-02")
	var conversations []string

	for _, m := range rows {
		// Filter by date: check if inicio_chamada starts with the target date
		inicioChamada := database.GetString(m, "inicio_chamada")
		if !strings.HasPrefix(inicioChamada, dateStr) {
			// Also try parsing as time
			t := database.GetTime(m, "inicio_chamada")
			if t.Format("2006-01-02") != dateStr {
				continue
			}
		}

		text := database.GetString(m, "transcricao_completa")
		if text != "" {
			conversations = append(conversations, text)
		}
	}

	// Sort by inicio_chamada (already filtered by date, order preserved from NQL)
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
	rows, err := zs.db.QueryByLabel(ctx, "zettels",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	var zettels []*Zettel
	for _, m := range rows {
		zettels = append(zettels, contentToZettel(m))
	}

	// Sort by access_count DESC, then created_at DESC
	sort.Slice(zettels, func(i, j int) bool {
		if zettels[i].AccessCount != zettels[j].AccessCount {
			return zettels[i].AccessCount > zettels[j].AccessCount
		}
		return zettels[i].CreatedAt.After(zettels[j].CreatedAt)
	})

	if limit > 0 && len(zettels) > limit {
		zettels = zettels[:limit]
	}

	return zettels, nil
}

func (zs *ZettelService) getAllLinks(ctx context.Context, idosoID int64) ([]ZettelLink, error) {
	// Get all links, then filter to only those belonging to zettels of this idoso
	linkRows, err := zs.db.QueryByLabel(ctx, "zettel_links", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Build set of zettel IDs for this idoso (for filtering links)
	zettelRows, err := zs.db.QueryByLabel(ctx, "zettels",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	idosoZettelIDs := make(map[string]bool)
	for _, m := range zettelRows {
		idosoZettelIDs[database.GetString(m, "id")] = true
	}

	var links []ZettelLink
	for _, m := range linkRows {
		fromID := database.GetString(m, "from_id")
		if !idosoZettelIDs[fromID] {
			continue
		}

		links = append(links, contentToZettelLink(m))
	}

	return links, nil
}

func (zs *ZettelService) getRelatedFromLinks(ctx context.Context, zettelID string) ([]*Zettel, error) {
	// Get all links from this zettel
	linkRows, err := zs.db.QueryByLabel(ctx, "zettel_links", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Collect target zettel IDs
	var targetIDs []string
	for _, m := range linkRows {
		fromID := database.GetString(m, "from_id")
		if fromID == zettelID {
			toID := database.GetString(m, "to_id")
			targetIDs = append(targetIDs, toID)
		}
	}

	if len(targetIDs) == 0 {
		return []*Zettel{}, nil
	}

	// Fetch each related zettel
	targetSet := make(map[string]bool)
	for _, id := range targetIDs {
		targetSet[id] = true
	}

	allZettels, err := zs.db.QueryByLabel(ctx, "zettels", "", nil, 0)
	if err != nil {
		return nil, err
	}

	var zettels []*Zettel
	for _, m := range allZettels {
		id := database.GetString(m, "id")
		if targetSet[id] {
			zettels = append(zettels, contentToZettel(m))
		}
	}

	return zettels, nil
}

// getRelatedFromGraph uses BFS from NietzscheDB for variable-length path traversal
func (zs *ZettelService) getRelatedFromGraph(ctx context.Context, zettelID string, depth int) ([]*Zettel, error) {
	// Find the zettel node by its zettel ID
	zettelResult, err := zs.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Zettel",
		MatchKeys: map[string]interface{}{
			"id": zettelID,
		},
	})
	if err != nil {
		return nil, err
	}

	// BFS from zettel node to the specified depth
	neighborIDs, err := zs.graphAdapter.Bfs(ctx, zettelResult.NodeID, uint32(depth), "")
	if err != nil {
		return nil, err
	}

	// Collect unique zettel IDs (filter to only Zettel nodes)
	var ids []string
	for _, nID := range neighborIDs {
		if nID == zettelResult.NodeID {
			continue
		}
		node, err := zs.graphAdapter.GetNode(ctx, nID, "")
		if err != nil {
			continue
		}
		// Only include Zettel nodes
		if node.NodeType == "Zettel" {
			if id, ok := node.Content["id"].(string); ok && id != zettelID {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return []*Zettel{}, nil
	}

	// Limit to 20
	if len(ids) > 20 {
		ids = ids[:20]
	}

	// Fetch zettels from NietzscheDB by IDs
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	allRows, err := zs.db.QueryByLabel(ctx, "zettels", "", nil, 0)
	if err != nil {
		return nil, err
	}

	var zettels []*Zettel
	for _, m := range allRows {
		id := database.GetString(m, "id")
		if idSet[id] {
			zettels = append(zettels, contentToZettel(m))
		}
	}

	return zettels, nil
}

// getGraphFromNietzsche builds the graph data from NietzscheDB
// Builds graph data from NietzscheDB
func (zs *ZettelService) getGraphFromNietzsche(ctx context.Context, idosoID int64, centerID string, depth int) (*GraphData, error) {
	graphData := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	if centerID != "" {
		// Find center zettel node
		centerResult, err := zs.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType: "Zettel",
			MatchKeys: map[string]interface{}{
				"id": centerID,
			},
		})
		if err != nil {
			return nil, err
		}

		// BFS from center to depth
		neighborIDs, err := zs.graphAdapter.Bfs(ctx, centerResult.NodeID, uint32(depth), "")
		if err != nil {
			return nil, err
		}

		// Include center node
		allIDs := append([]string{centerResult.NodeID}, neighborIDs...)

		for _, nID := range allIDs {
			node, err := zs.graphAdapter.GetNode(ctx, nID, "")
			if err != nil {
				continue
			}

			// Only include Zettel nodes belonging to this idoso
			if node.NodeType != "Zettel" {
				continue
			}
			nodeIdosoID := toFloat64Zettel(node.Content["idoso_id"])
			if int64(nodeIdosoID) != idosoID {
				continue
			}

			zettelID, _ := node.Content["id"].(string)
			title, _ := node.Content["title"].(string)
			zettelType, _ := node.Content["type"].(string)

			graphData.Nodes = append(graphData.Nodes, GraphNode{
				ID:    zettelID,
				Label: title,
				Type:  ZettelType(zettelType),
				Size:  5,
				Color: zs.getColorForType(ZettelType(zettelType)),
			})
		}
	} else {
		// No center: query all zettels for this idoso via NQL
		nql := `MATCH (z:Zettel) RETURN z LIMIT 100`
		result, err := zs.graphAdapter.ExecuteNQL(ctx, nql, nil, "")
		if err != nil {
			return nil, err
		}

		for _, node := range result.Nodes {
			nodeIdosoID := toFloat64Zettel(node.Content["idoso_id"])
			if int64(nodeIdosoID) != idosoID {
				continue
			}

			zettelID, _ := node.Content["id"].(string)
			title, _ := node.Content["title"].(string)
			zettelType, _ := node.Content["type"].(string)

			graphData.Nodes = append(graphData.Nodes, GraphNode{
				ID:    zettelID,
				Label: title,
				Type:  ZettelType(zettelType),
				Size:  5,
				Color: zs.getColorForType(ZettelType(zettelType)),
			})
		}
	}

	// Get edges from NietzscheDB zettel_links (more reliable for known links)
	links, _ := zs.getAllLinks(ctx, idosoID)
	nodeSet := make(map[string]bool)
	for _, n := range graphData.Nodes {
		nodeSet[n.ID] = true
	}

	for _, link := range links {
		if nodeSet[link.FromID] && nodeSet[link.ToID] {
			graphData.Edges = append(graphData.Edges, GraphEdge{
				From:     link.FromID,
				To:       link.ToID,
				Strength: link.Strength,
				Label:    link.LinkType,
			})
		}
	}

	return graphData, nil
}

// ============================================================================
// NietzscheDB content map -> struct conversion helpers
// ============================================================================

// contentToZettel converts a NietzscheDB content map to a Zettel struct
func contentToZettel(m map[string]interface{}) *Zettel {
	z := &Zettel{
		ID:          database.GetString(m, "id"),
		IdosoID:     database.GetInt64(m, "idoso_id"),
		Title:       database.GetString(m, "title"),
		Content:     database.GetString(m, "content"),
		ZettelType:  ZettelType(database.GetString(m, "zettel_type")),
		CreatedAt:   database.GetTime(m, "created_at"),
		UpdatedAt:   database.GetTime(m, "updated_at"),
		AccessCount: int(database.GetInt64(m, "access_count")),
		LastAccessed: database.GetTimePtr(m, "last_accessed"),
	}

	// Parse source JSON
	sourceStr := database.GetString(m, "source")
	if sourceStr != "" {
		json.Unmarshal([]byte(sourceStr), &z.Source)
	}

	// Parse entities JSON
	z.Entities = getEntitiesFromContent(m)

	// Parse tags JSON
	z.Tags = getStringSlice(m, "tags")

	// Parse linked_zettels JSON
	z.LinkedZettels = getStringSlice(m, "linked_zettels")

	// Parse metadata JSON
	metadataStr := database.GetString(m, "metadata")
	if metadataStr != "" {
		z.Metadata = make(map[string]string)
		json.Unmarshal([]byte(metadataStr), &z.Metadata)
	}

	return z
}

// contentToZettelLink converts a NietzscheDB content map to a ZettelLink struct
func contentToZettelLink(m map[string]interface{}) ZettelLink {
	return ZettelLink{
		FromID:        database.GetString(m, "from_id"),
		ToID:          database.GetString(m, "to_id"),
		LinkType:      database.GetString(m, "link_type"),
		Strength:      database.GetFloat64(m, "strength"),
		Context:       database.GetString(m, "context"),
		CreatedAt:     database.GetTime(m, "created_at"),
		Bidirectional: database.GetBool(m, "bidirectional"),
	}
}

// getEntitiesFromContent parses the entities field from a content map.
// Handles both JSON string and native slice representations.
func getEntitiesFromContent(m map[string]interface{}) []Entity {
	var entities []Entity

	raw, ok := m["entities"]
	if !ok || raw == nil {
		return entities
	}

	switch v := raw.(type) {
	case string:
		json.Unmarshal([]byte(v), &entities)
	default:
		// Could be []interface{} from JSON deserialization
		if b, err := json.Marshal(v); err == nil {
			json.Unmarshal(b, &entities)
		}
	}

	return entities
}

// getStringSlice parses a string slice from a content map.
// Handles both JSON string and native slice representations.
func getStringSlice(m map[string]interface{}, key string) []string {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case string:
		var result []string
		json.Unmarshal([]byte(v), &result)
		return result
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	}

	return nil
}

// Helper type conversion
func toFloat64Zettel(v interface{}) float64 {
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
	default:
		return 0
	}
}
