// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"eva/internal/brainstem/infrastructure/vector"

	"github.com/qdrant/go-client/qdrant"
)

// WisdomService busca sabedoria relevante em múltiplas coleções Qdrant
// Implementa busca semântica para histórias, fábulas, ensinamentos, técnicas
type WisdomService struct {
	qdrant   *vector.QdrantClient
	embedder *EmbeddingService
}

// WisdomResult representa um resultado de busca de sabedoria
type WisdomResult struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Source     string   `json:"source"`     // nasrudin, esopo, gurdjieff, osho, zen...
	Tradition  string   `json:"tradition"`  // sufi, grega, quarto_caminho, budista...
	Type       string   `json:"type"`       // story, fable, teaching, exercise, koan
	Moral      string   `json:"moral"`      // Para fábulas
	Tags       []string `json:"tags"`
	Score      float32  `json:"score"`      // Similaridade semântica
	Collection string   `json:"collection"` // Coleção Qdrant de origem
}

// WisdomSearchOptions define opções de busca
type WisdomSearchOptions struct {
	Collections   []string // Coleções específicas para buscar (vazio = todas)
	MinScore      float32  // Score mínimo de similaridade (default: 0.7)
	Limit         int      // Máximo de resultados (default: 5)
	ExcludeSeen   []string // IDs já vistos/usados para evitar repetição
	PreferTypes   []string // Tipos preferidos (story, exercise, teaching)
	EmotionalTone string   // Tom emocional desejado (provocativo, acolhedor, paradoxal)
}

// Coleções disponíveis de sabedoria
var WisdomCollections = []string{
	"nasrudin_stories",    // Histórias Sufi de Nasrudin
	"aesop_fables",        // Fábulas de Esopo
	"gurdjieff_teachings", // Ensinamentos do Quarto Caminho
	"osho_insights",       // Insights e provocações de Osho
	"zen_koans",           // Koans Zen
	"nietzsche_aphorisms", // Aforismos de Nietzsche
	"stoic_meditations",   // Meditações estoicas
	"rumi_poems",          // Poemas de Rumi
	"breathing_scripts",   // Scripts de respiração guiada
	"hypnosis_scripts",    // Scripts de auto-hipnose
	"somatic_exercises",   // Exercícios somáticos
}

// NewWisdomService cria um novo serviço de busca de sabedoria
func NewWisdomService(qdrant *vector.QdrantClient, embedder *EmbeddingService) *WisdomService {
	return &WisdomService{
		qdrant:   qdrant,
		embedder: embedder,
	}
}

// SearchWisdom busca sabedoria relevante baseada no texto do usuário
func (w *WisdomService) SearchWisdom(ctx context.Context, query string, opts *WisdomSearchOptions) ([]*WisdomResult, error) {
	if w.qdrant == nil || w.embedder == nil {
		return nil, fmt.Errorf("wisdom service not properly initialized")
	}

	// Defaults
	if opts == nil {
		opts = &WisdomSearchOptions{}
	}
	if opts.MinScore == 0 {
		opts.MinScore = 0.7
	}
	if opts.Limit == 0 {
		opts.Limit = 5
	}

	// Gerar embedding da query
	embedding, err := w.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Determinar coleções para buscar
	collections := opts.Collections
	if len(collections) == 0 {
		collections = w.getAvailableCollections(ctx)
	}

	log.Printf("🔍 [WISDOM] Buscando em %d coleções: %v", len(collections), collections)

	// Buscar em cada coleção
	var allResults []*WisdomResult
	excludeMap := make(map[string]bool)
	for _, id := range opts.ExcludeSeen {
		excludeMap[id] = true
	}

	for _, collection := range collections {
		results, err := w.searchCollection(ctx, collection, embedding, opts.Limit*2) // Buscar mais para filtrar depois
		if err != nil {
			log.Printf("⚠️ [WISDOM] Erro ao buscar em %s: %v", collection, err)
			continue
		}

		for _, result := range results {
			// Filtrar por score mínimo
			if result.Score < opts.MinScore {
				continue
			}

			// Filtrar já vistos
			if excludeMap[result.ID] {
				continue
			}

			result.Collection = collection
			allResults = append(allResults, result)
		}
	}

	// Ordenar por score (maior primeiro)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// Aplicar preferência de tipos se especificado
	if len(opts.PreferTypes) > 0 {
		allResults = w.boostPreferredTypes(allResults, opts.PreferTypes)
	}

	// Limitar resultados
	if len(allResults) > opts.Limit {
		allResults = allResults[:opts.Limit]
	}

	log.Printf("✅ [WISDOM] Encontrados %d resultados relevantes", len(allResults))

	return allResults, nil
}

// searchCollection busca em uma coleção específica
func (w *WisdomService) searchCollection(ctx context.Context, collection string, embedding []float32, limit int) ([]*WisdomResult, error) {
	results, err := w.qdrant.Search(ctx, collection, embedding, uint64(limit), nil)
	if err != nil {
		return nil, err
	}

	var wisdom []*WisdomResult
	for _, point := range results {
		result := &WisdomResult{
			Score: point.Score,
		}

		// Extrair payload
		if id, ok := extractString(point.Payload, "id"); ok {
			result.ID = id
		}
		if title, ok := extractString(point.Payload, "title"); ok {
			result.Title = title
		}
		if content, ok := extractString(point.Payload, "content"); ok {
			result.Content = content
		}
		if source, ok := extractString(point.Payload, "source"); ok {
			result.Source = source
		}
		if tradition, ok := extractString(point.Payload, "tradition"); ok {
			result.Tradition = tradition
		}
		if typeStr, ok := extractString(point.Payload, "type"); ok {
			result.Type = typeStr
		}
		if moral, ok := extractString(point.Payload, "moral"); ok {
			result.Moral = moral
		}
		if tags, ok := extractStringList(point.Payload, "tags"); ok {
			result.Tags = tags
		}

		wisdom = append(wisdom, result)
	}

	return wisdom, nil
}

// getAvailableCollections retorna coleções que existem no Qdrant
func (w *WisdomService) getAvailableCollections(ctx context.Context) []string {
	var available []string

	for _, collection := range WisdomCollections {
		_, err := w.qdrant.GetCollectionInfo(ctx, collection)
		if err == nil {
			available = append(available, collection)
		}
	}

	return available
}

// boostPreferredTypes reordena resultados dando preferência a certos tipos
func (w *WisdomService) boostPreferredTypes(results []*WisdomResult, preferTypes []string) []*WisdomResult {
	preferMap := make(map[string]float32)
	for i, t := range preferTypes {
		// Boost decrescente: primeiro tipo +0.1, segundo +0.05, etc
		preferMap[t] = 0.1 - float32(i)*0.025
	}

	for _, result := range results {
		if boost, ok := preferMap[result.Type]; ok {
			result.Score += boost
		}
	}

	// Re-ordenar
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// GetWisdomContext monta contexto de sabedoria para o prompt
func (w *WisdomService) GetWisdomContext(ctx context.Context, query string, opts *WisdomSearchOptions) string {
	// Validar query não vazia (evita erro 400 na API de embedding)
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	results, err := w.SearchWisdom(ctx, query, opts)
	if err != nil {
		log.Printf("⚠️ [WISDOM] Erro na busca: %v", err)
		return ""
	}

	if len(results) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n📚 SABEDORIA RELEVANTE (Busca Semântica):\n\n")

	for i, r := range results {
		builder.WriteString(fmt.Sprintf("──────────────────────────────────────────\n"))
		builder.WriteString(fmt.Sprintf("📖 %d. %s (%s | %s)\n", i+1, r.Title, r.Source, r.Tradition))
		builder.WriteString(fmt.Sprintf("   Tipo: %s | Score: %.2f\n\n", r.Type, r.Score))

		// Truncar conteúdo se muito longo
		content := r.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		builder.WriteString(fmt.Sprintf("   %s\n", content))

		if r.Moral != "" {
			builder.WriteString(fmt.Sprintf("\n   💡 Moral: %s\n", r.Moral))
		}

		if len(r.Tags) > 0 {
			builder.WriteString(fmt.Sprintf("   🏷️ Tags: %s\n", strings.Join(r.Tags, ", ")))
		}

		builder.WriteString("\n")
	}

	builder.WriteString("──────────────────────────────────────────\n")
	builder.WriteString("→ Use estas histórias/ensinamentos se forem relevantes para a situação.\n")
	builder.WriteString("→ Adapte a linguagem ao contexto do paciente.\n")
	builder.WriteString("→ Não force o uso - só apresente se fizer sentido terapêutico.\n\n")

	return builder.String()
}

// SearchByEmotion busca sabedoria para um estado emocional específico
func (w *WisdomService) SearchByEmotion(ctx context.Context, emotion string, limit int) ([]*WisdomResult, error) {
	// Mapear emoção para query semântica enriquecida
	emotionQueries := map[string]string{
		"ansiedade":   "ansiedade preocupação mente inquieta não consegue parar de pensar",
		"tristeza":    "tristeza melancolia perda saudade vazio",
		"raiva":       "raiva frustração injustiça irritação",
		"medo":        "medo insegurança perigo ameaça",
		"solidão":     "solidão isolamento abandono sozinho",
		"culpa":       "culpa arrependimento erro falha",
		"vergonha":    "vergonha inadequação exposição julgamento",
		"desespero":   "desespero sem saída desesperança",
		"confusão":    "confusão perdido não sabe o que fazer",
		"impaciência": "impaciência pressa querer resultado rápido",
	}

	query, ok := emotionQueries[strings.ToLower(emotion)]
	if !ok {
		query = emotion
	}

	return w.SearchWisdom(ctx, query, &WisdomSearchOptions{
		Limit:    limit,
		MinScore: 0.65, // Score um pouco mais baixo para emoções
	})
}

// SearchByPattern busca sabedoria para um padrão psicológico específico
func (w *WisdomService) SearchByPattern(ctx context.Context, pattern string, limit int) ([]*WisdomResult, error) {
	// Mapear padrão Lacaniano para query semântica
	patternQueries := map[string]string{
		"projection":       "projeção culpar outros ver no outro o que é seu",
		"denial":           "negação não aceitar realidade evitar verdade",
		"rationalization":  "racionalização justificar desculpas explicar",
		"displacement":     "deslocamento transferir emoção outro alvo",
		"regression":       "regressão voltar comportamento anterior infantil",
		"repression":       "repressão esconder sentimentos enterrar emoções",
		"sublimation":      "sublimação transformar energia criatividade",
		"identification":   "identificação querer ser como admirar",
		"introjection":     "introjeção aceitar valores externos como seus",
		"splitting":        "cisão tudo ou nada preto branco",
		"obsessive":        "obsessão repetição controle perfeccionismo",
		"hysteric":         "histeria drama atenção sedução",
		"demand_for_love":  "demanda amor reconhecimento validação",
		"fear_of_loss":     "medo perda abandono separação",
		"guilt_punishment": "culpa punição merecer castigo",
	}

	query, ok := patternQueries[strings.ToLower(pattern)]
	if !ok {
		query = pattern
	}

	return w.SearchWisdom(ctx, query, &WisdomSearchOptions{
		Limit:    limit,
		MinScore: 0.65,
	})
}

// GetRandomWisdom retorna sabedoria aleatória de uma tradição específica
func (w *WisdomService) GetRandomWisdom(ctx context.Context, tradition string) (*WisdomResult, error) {
	// Mapear tradição para coleção
	traditionCollections := map[string]string{
		"sufi":           "nasrudin_stories",
		"grega":          "aesop_fables",
		"quarto_caminho": "gurdjieff_teachings",
		"osho":           "osho_insights",
		"zen":            "zen_koans",
		"nietzsche":      "nietzsche_aphorisms",
		"estoica":        "stoic_meditations",
		"rumi":           "rumi_poems",
	}

	collection, ok := traditionCollections[strings.ToLower(tradition)]
	if !ok {
		return nil, fmt.Errorf("tradição desconhecida: %s", tradition)
	}

	// Usar uma query genérica para pegar algo "aleatório" semanticamente
	results, err := w.SearchWisdom(ctx, "sabedoria vida ensinamento", &WisdomSearchOptions{
		Collections: []string{collection},
		Limit:       10,
		MinScore:    0.5,
	})

	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("nenhuma sabedoria encontrada para %s", tradition)
	}

	// Retornar um "aleatório" (na prática, o mais genérico semanticamente)
	return results[len(results)/2], nil
}

// Helpers para extrair valores do payload Qdrant
func extractString(payload map[string]*qdrant.Value, key string) (string, bool) {
	if val, ok := payload[key]; ok {
		if str, ok := val.GetKind().(*qdrant.Value_StringValue); ok {
			return str.StringValue, true
		}
	}
	return "", false
}

func extractStringList(payload map[string]*qdrant.Value, key string) ([]string, bool) {
	if val, ok := payload[key]; ok {
		if list, ok := val.GetKind().(*qdrant.Value_ListValue); ok {
			var result []string
			for _, v := range list.ListValue.Values {
				if s, ok := v.GetKind().(*qdrant.Value_StringValue); ok {
					result = append(result, s.StringValue)
				}
			}
			return result, true
		}
	}
	return nil, false
}
