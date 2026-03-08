// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// EmbeddingService rastreia cadeias de significantes via embeddings semânticos
// Implementa "L'instance de la lettre" - a letra que circula e se repete
type EmbeddingService struct {
	cfg            *config.Config
	vectorAdapter  *nietzscheInfra.VectorAdapter
	httpClient     *http.Client
	collectionName string

	// PERFORMANCE FIX: Caches para reduzir chamadas de API
	embeddingCache  *EmbeddingCache  // Cache de embeddings (90% reducao API calls)
	signifierCache  *SignifierCache  // Cache de signifiers (5min TTL)
}

// SignifierChain representa uma cadeia de significantes detectada
type SignifierChain struct {
	CoreSignifier   string    `json:"core_signifier"`   // Significante nuclear
	RelatedWords    []string  `json:"related_words"`    // Palavras semanticamente próximas
	EmotionalCharge float64   `json:"emotional_charge"` // Carga afetiva (0.0-1.0)
	Frequency       int       `json:"frequency"`        // Vezes que apareceu
	LastOccurrence  time.Time `json:"last_occurrence"`
	Contexts        []string  `json:"contexts"` // Frases onde apareceu
}

// NewEmbeddingService cria serviço de embeddings
func NewEmbeddingService(cfg *config.Config, vectorAdapter *nietzscheInfra.VectorAdapter) (*EmbeddingService, error) {
	svc := &EmbeddingService{
		cfg:            cfg,
		vectorAdapter:  vectorAdapter,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		collectionName: "signifier_chains",
		// Caches initialized with nil adapter (local LRU only).
		// Call SetCacheAdapter() to enable persistent NietzscheDB caching.
		embeddingCache: NewEmbeddingCache(nil),
		signifierCache: NewSignifierCache(nil),
	}

	log.Printf("[EMBEDDING] Service initialized with local LRU caching")
	return svc, nil
}

// SetCacheAdapter wires a NietzscheDB CacheAdapter into the embedding and signifier caches.
// This enables the persistent L2 cache layer (NietzscheDB eva_cache collection).
// Must be called after NewEmbeddingService if distributed caching is desired.
func (e *EmbeddingService) SetCacheAdapter(adapter *nietzscheInfra.CacheAdapter) {
	if adapter == nil {
		return
	}
	e.embeddingCache = NewEmbeddingCache(adapter)
	e.signifierCache = NewSignifierCache(adapter)
	log.Printf("[EMBEDDING] NietzscheDB CacheAdapter enabled (collection: %s)", adapter.Collection())
}

// SetNietzscheDBClient is deprecated. Use SetCacheAdapter instead.
// Kept for backward compatibility; this is a no-op (NietzscheDB replaced NietzscheDB).
func (e *EmbeddingService) SetNietzscheDBClient(NietzscheDBClient interface{}) {
	// DEPRECATED: NietzscheDB has been replaced by NietzscheDB CacheAdapter.
	// Use SetCacheAdapter() instead.
	log.Printf("[EMBEDDING] SetNietzscheDBClient called (deprecated no-op) — use SetCacheAdapter()")
}

// ensureCollection is no longer needed - NietzscheDB handles collection management.
// Kept as no-op for backward compatibility of any callers.
func (e *EmbeddingService) ensureCollection(ctx context.Context) error {
	return nil
}

// GenerateEmbedding gera embedding usando Gemini API
// PERFORMANCE FIX: Usa cache para evitar chamadas repetidas (90% reducao)
func (e *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// 0. Validar texto não vazio (evita erro 400 da API)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text content is empty")
	}

	// 1. Verificar cache primeiro
	if e.embeddingCache != nil {
		if cached, ok := e.embeddingCache.Get(ctx, text); ok {
			return cached, nil
		}
	}

	// 2. Gerar via API (cache miss)
	embedding, err := e.generateEmbeddingFromAPI(ctx, text)
	if err != nil {
		return nil, err
	}

	// 3. Salvar no cache para proximas chamadas
	if e.embeddingCache != nil {
		e.embeddingCache.Set(ctx, text, embedding)
	}

	return embedding, nil
}

// generateEmbeddingFromAPI faz chamada real para Gemini API
func (e *EmbeddingService) generateEmbeddingFromAPI(ctx context.Context, text string) ([]float32, error) {
	// gemini-embedding-001 é o novo modelo recomendado (substitui text-embedding-004)
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=%s",
		e.cfg.GoogleAPIKey,
	)

	payload := map[string]interface{}{
		"model": "models/gemini-embedding-001",
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
		"outputDimensionality": 3072, // Máxima qualidade - NietzscheDB collections com 3072-dim
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding.Values, nil
}

// TrackSignifierChain rastreia cadeia de significantes
func (e *EmbeddingService) TrackSignifierChain(ctx context.Context, idosoID int64, text string, emotionalCharge float64) error {
	if e.vectorAdapter == nil {
		return nil
	}

	// Gerar embedding do texto
	embedding, err := e.GenerateEmbedding(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Extrair palavras-chave (simples split por enquanto, ideal seria NLP)
	keywords := strings.Fields(text)

	pointID := fmt.Sprintf("%d", time.Now().UnixNano())

	payload := map[string]interface{}{
		"idoso_id":         idosoID,
		"text":             text,
		"keywords":         keywords,
		"emotional_charge": emotionalCharge,
		"timestamp":        time.Now().Unix(),
	}

	err = e.vectorAdapter.Upsert(ctx, e.collectionName, pointID, embedding, payload)
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	log.Printf("🔗 [VECTOR] Signifier chain tracked: idoso=%d", idosoID)
	return nil
}

// FindRelatedSignifiers busca significantes semanticamente relacionados
// PERFORMANCE FIX: Cache de signifiers (TTL 5min)
func (e *EmbeddingService) FindRelatedSignifiers(ctx context.Context, idosoID int64, text string, limit int) ([]SignifierChain, error) {
	if e.vectorAdapter == nil {
		return nil, nil
	}

	// 1. Verificar cache primeiro
	if e.signifierCache != nil {
		if cached, ok := e.signifierCache.GetSignifiers(ctx, idosoID, text); ok {
			return cached, nil
		}
	}

	// 2. Gerar embedding da consulta (tambem usa cache interno)
	embedding, err := e.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	// Buscar pontos similares com filtro de usuario via VectorAdapter
	searchResult, err := e.vectorAdapter.Search(ctx, e.collectionName, embedding, limit, idosoID)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var chains []SignifierChain
	for _, point := range searchResult {
		chain := SignifierChain{}

		if kw, ok := point.Payload["keywords"]; ok {
			if kwList, ok := kw.([]interface{}); ok {
				for _, v := range kwList {
					if s, ok := v.(string); ok {
						chain.RelatedWords = append(chain.RelatedWords, s)
					}
				}
			}
		}

		if len(chain.RelatedWords) > 0 {
			chain.CoreSignifier = chain.RelatedWords[0]
		}

		if charge, ok := point.Payload["emotional_charge"]; ok {
			if dbl, ok := charge.(float64); ok {
				chain.EmotionalCharge = dbl
			}
		}

		if txt, ok := point.Payload["text"]; ok {
			if str, ok := txt.(string); ok {
				chain.Contexts = append(chain.Contexts, str)
			}
		}

		if ts, ok := point.Payload["timestamp"]; ok {
			switch v := ts.(type) {
			case int64:
				chain.LastOccurrence = time.Unix(v, 0)
			case float64:
				chain.LastOccurrence = time.Unix(int64(v), 0)
			}
		}

		chains = append(chains, chain)
	}

	// 3. Salvar no cache para proximas chamadas
	if e.signifierCache != nil && len(chains) > 0 {
		e.signifierCache.SetSignifiers(ctx, idosoID, text, chains)
	}

	return chains, nil
}

// GetSemanticContext monta contexto para o prompt usando similaridade semântica
func (e *EmbeddingService) GetSemanticContext(ctx context.Context, idosoID int64, currentText string) string {
	// Validar texto não vazio
	currentText = strings.TrimSpace(currentText)
	if currentText == "" {
		return ""
	}

	chains, err := e.FindRelatedSignifiers(ctx, idosoID, currentText, 5)
	if err != nil {
		log.Printf("⚠️ Error finding related signifiers: %v", err)
		return ""
	}

	if len(chains) == 0 {
		return ""
	}

	context := "\n🔗 CADEIA DE SIGNIFICANTES (Análise Semântica):\n\n"
	context += "O sistema detectou que palavras/temas similares já apareceram antes:\n"

	for i, chain := range chains {
		context += fmt.Sprintf("%d. '%s' (carga emocional: %.2f)\n",
			i+1, chain.CoreSignifier, chain.EmotionalCharge)

		if len(chain.Contexts) > 0 {
			context += fmt.Sprintf("   Contexto anterior: \"%s\"\n",
				truncateText(chain.Contexts[0], 100))
		}

		context += fmt.Sprintf("   Última vez: %s\n\n",
			chain.LastOccurrence.Format("02/01/2006 15:04"))
	}

	context += "→ Use essas informações para fazer conexões entre o que o paciente disse antes e agora.\n"
	context += "→ Se houver repetição de temas, isso pode indicar um nó sintomático.\n"

	return context
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
