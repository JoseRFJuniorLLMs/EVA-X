package memory

import (
	"context"
	"encoding/json"
	"eva-mind/internal/brainstem/infrastructure/cache"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type FDPNEngine struct {
	neo4j         *graph.Neo4jClient
	redis         *cache.RedisClient
	qdrant        *vector.QdrantClient // NEW: Vector search
	localCache    *sync.Map
	activeThreads chan struct{} // Limiter for concurrency
	maxDepth      int
	threshold     float64
}

type SubgraphActivation struct {
	RootNode  string          `json:"root_node"`
	Nodes     []ActivatedNode `json:"nodes"`
	Timestamp time.Time       `json:"timestamp"`
	Energy    float64         `json:"energy"`
	Depth     int             `json:"depth"`
}

type ActivatedNode struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Activation float64                `json:"activation"`
	Level      int                    `json:"level"`
	Properties map[string]interface{} `json:"properties"`
}

func NewFDPNEngine(neo4j *graph.Neo4jClient, redis *cache.RedisClient, qdrant *vector.QdrantClient) *FDPNEngine {
	return &FDPNEngine{
		neo4j:         neo4j,
		redis:         redis,
		qdrant:        qdrant,
		localCache:    &sync.Map{},
		activeThreads: make(chan struct{}, 10), // Max 10 parallel threads
		maxDepth:      3,
		threshold:     0.3,
	}
}

// StreamingPrime activates subgraphs during transcription
func (e *FDPNEngine) StreamingPrime(ctx context.Context, userID string, partialText string) error {
	startTime := time.Now()

	keywords := e.extractKeywords(partialText)
	if len(keywords) == 0 {
		return nil
	}

	// Double check cache to avoid redundant work
	var uncachedKeywords []string
	for _, kw := range keywords {
		cacheKey := fmt.Sprintf("%s:%s", userID, kw)
		if _, cached := e.localCache.Load(cacheKey); !cached {
			uncachedKeywords = append(uncachedKeywords, kw)
		}
	}

	if len(uncachedKeywords) == 0 {
		return nil
	}

	var wg sync.WaitGroup

	for _, kw := range uncachedKeywords {
		wg.Add(1)
		e.activeThreads <- struct{}{} // Acquire token

		go func(keyword string) {
			defer wg.Done()
			defer func() { <-e.activeThreads }() // Release token

			if err := e.primeKeyword(ctx, userID, keyword); err != nil {
				log.Printf("[PRIMING_ERROR] %s: %v", keyword, err)
			}
		}(kw)
	}

	wg.Wait()

	elapsed := time.Since(startTime)
	if elapsed.Milliseconds() > 5 {
		log.Printf("[PRIMING_STATS] Processed %d keywords in %dms", len(uncachedKeywords), elapsed.Milliseconds())
	}

	return nil
}

func (e *FDPNEngine) primeKeyword(ctx context.Context, userID string, keyword string) error {
	// 1. Spreading Activation Query (Fractal Depth 3)
	query := `
		MATCH (raiz:Eneatipo|Topic|Event) // Broad match for now, assuming generalized labels
		WHERE toLower(raiz.nome) CONTAINS toLower($keyword) 
		   OR toLower(raiz.content) CONTAINS toLower($keyword)
		WITH raiz LIMIT 1

		MATCH path = (raiz)-[r*1..3]-(vizinho)
		// Assuming we have weights, otherwise default to 1.0 logic
		// WHERE ALL(rel IN relationships(path) WHERE rel.weight > 0.3)

		WITH raiz, vizinho, relationships(path) as rels
		WITH raiz, vizinho, rels,
			 reduce(energy = 1.0, rel IN rels | energy * 0.85) as activation // Simple decay 15% per hop

		WHERE activation >= $threshold

		RETURN 
			raiz.id as raiz_id,
			coalesce(raiz.nome, raiz.content) as raiz_nome,
			collect({
				id: elementId(vizinho),
				nome: coalesce(vizinho.nome, vizinho.content, 'Unamed'),
				tipo: labels(vizinho)[0],
				activation: activation,
				level: size(rels),
				properties: properties(vizinho)
			}) as nodes
	`

	results, err := e.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"keyword":   keyword,
		"threshold": e.threshold,
	})

	if err != nil {
		return err
	}

	if len(results) == 0 {
		return nil
	}

	// Parse first result (Assuming one root per keyword for simplicity)
	record := results[0]

	// Safe casting handling
	raizNome, _ := record.Values[1].(string)

	nodesRaw, _ := record.Values[2].([]interface{})
	var activatedNodes []ActivatedNode
	var totalEnergy float64

	for _, nr := range nodesRaw {
		nMap, ok := nr.(map[string]interface{})
		if !ok {
			continue
		}

		activation, _ := nMap["activation"].(float64)
		level, _ := nMap["level"].(int64)

		node := ActivatedNode{
			ID:         fmt.Sprintf("%v", nMap["id"]),
			Name:       fmt.Sprintf("%v", nMap["nome"]),
			Type:       fmt.Sprintf("%v", nMap["tipo"]),
			Activation: activation,
			Level:      int(level),
			Properties: nMap["properties"].(map[string]interface{}),
		}
		activatedNodes = append(activatedNodes, node)
		totalEnergy += activation
	}

	// Apply Absolute Zero Entropy Filter
	filteredNodes := e.filterEntropy(activatedNodes)

	subgraph := &SubgraphActivation{
		RootNode:  raizNome,
		Nodes:     filteredNodes,
		Timestamp: time.Now(),
		Energy:    totalEnergy, // Energy might need recalculation if we were strict, but total potential energy remains useful
		Depth:     3,
	}

	// Cache logic
	cacheKey := fmt.Sprintf("%s:%s", userID, keyword)
	e.localCache.Store(cacheKey, subgraph)

	// Redis write (synchronous for reliability)
	if e.redis != nil {
		data, _ := json.Marshal(subgraph)
		// 5 minutes TTL
		if err := e.redis.Set(context.Background(), cacheKey, data, 5*time.Minute); err != nil {
			log.Printf("[REDIS_ERROR] Failed to cache %s: %v", cacheKey, err)
		}
	}

	return nil
}

// filterEntropy reduces context noise (Absolute Zero concept)
func (e *FDPNEngine) filterEntropy(nodes []ActivatedNode) []ActivatedNode {
	if len(nodes) < 3 {
		return nodes
	}

	var totalActivation float64
	for _, n := range nodes {
		totalActivation += n.Activation
	}

	if totalActivation == 0 {
		return nodes
	}

	// Simplified "Top-K" strategy for entropy filtering
	// In production, this would calculate Shannon Entropy
	// For now, we use dynamic thresholding (practical equivalent)

	// Sort by activation desc
	// (Simulating sort)

	// For now, let's just return top 50% if count > 10 to reduce noise
	// Ideally we'd do the full math, but let's stick to the practical effect:
	// "Focus on the signal"

	// Simple thresholding relative to max
	var maxAct float64
	for _, n := range nodes {
		if n.Activation > maxAct {
			maxAct = n.Activation
		}
	}

	var filtered []ActivatedNode
	for _, n := range nodes {
		// Keep if at least 20% of the max activation (Dynamic Threshold)
		if n.Activation >= maxAct*0.2 {
			filtered = append(filtered, n)
		}
	}

	return filtered
}

// GetContext retrieves the primed context for a set of keywords
func (e *FDPNEngine) GetContext(ctx context.Context, userID string, keywords []string) map[string]*SubgraphActivation {
	result := make(map[string]*SubgraphActivation)

	for _, kw := range keywords {
		cacheKey := fmt.Sprintf("%s:%s", userID, kw)

		// 1. L1 Cache (Memory)
		if val, ok := e.localCache.Load(cacheKey); ok {
			result[kw] = val.(*SubgraphActivation)
			continue
		}

		// 2. L2 Cache (Redis)
		valStr, err := e.redis.Get(ctx, cacheKey)
		if err == nil && valStr != "" {
			var subgraph SubgraphActivation
			if err := json.Unmarshal([]byte(valStr), &subgraph); err == nil {
				result[kw] = &subgraph
				// Promote to L1
				e.localCache.Store(cacheKey, &subgraph)
			}
		}
	}
	return result
}

func (e *FDPNEngine) extractKeywords(text string) []string {
	// Simple STOPWORD filter for Portuguese
	stopwords := map[string]bool{
		"o": true, "a": true, "de": true, "que": true, "e": true, "do": true, "da": true, "em": true,
		"um": true, "para": true, "com": true, "não": true, "uma": true, "os": true, "no": true,
		"se": true, "na": true, "por": true, "mais": true, "as": true, "dos": true, "como": true,
		"mas": true, "foi": true, "ao": true, "ele": true, "das": true, "tem": true, "à": true,
		"seu": true, "sua": true, "ou": true, "ser": true, "quando": true, "muito": true,
		"estou": true, "está": true, "estava": true,
	}

	words := strings.Fields(strings.ToLower(text))
	var clean []string
	seen := make(map[string]bool)

	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"")
		if len(w) < 3 {
			continue
		}
		if stopwords[w] {
			continue
		}

		if !seen[w] {
			clean = append(clean, w)
			seen[w] = true
		}
	}
	return clean
}
