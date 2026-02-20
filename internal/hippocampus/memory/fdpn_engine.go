// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"encoding/json"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

type FDPNEngine struct {
	graphAdapter  *nietzscheInfra.GraphAdapter
	cacheStore    *nietzscheInfra.CacheStore
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

func NewFDPNEngine(graphAdapter *nietzscheInfra.GraphAdapter, cacheStore *nietzscheInfra.CacheStore) *FDPNEngine {
	return &FDPNEngine{
		graphAdapter:  graphAdapter,
		cacheStore:    cacheStore,
		localCache:    &sync.Map{},
		activeThreads: make(chan struct{}, 10), // Max 10 parallel threads
		maxDepth:      3,
		threshold:     0.3,
	}
}

// SetModulation permite que o ExecutiveController ajuste parametros dinamicamente
func (e *FDPNEngine) SetModulation(depth int, threshold float64) {
	if depth >= 0 {
		e.maxDepth = depth
	}
	if threshold > 0 {
		e.threshold = threshold
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
	// 1. Find root node matching keyword via NQL
	nql := `MATCH (n) WHERE n.nome CONTAINS $keyword OR n.content CONTAINS $keyword RETURN n LIMIT 1`
	queryResult, err := e.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"keyword": strings.ToLower(keyword),
	}, "")
	if err != nil {
		return err
	}

	if len(queryResult.Nodes) == 0 {
		return nil
	}

	rootNode := queryResult.Nodes[0]
	rootID := rootNode.ID
	raizNome := ""
	if nome, ok := rootNode.Content["nome"]; ok {
		raizNome = fmt.Sprintf("%v", nome)
	} else if content, ok := rootNode.Content["content"]; ok {
		raizNome = fmt.Sprintf("%v", content)
	}

	// 2. BFS spreading activation (depth 3)
	neighborIDs, err := e.graphAdapter.Bfs(ctx, rootID, uint32(e.maxDepth), "")
	if err != nil {
		return err
	}

	var activatedNodes []ActivatedNode
	var totalEnergy float64

	for _, nid := range neighborIDs {
		if nid == rootID {
			continue
		}

		// Get each neighbor node
		nodeResult, err := e.graphAdapter.GetNode(ctx, nid, "")
		if err != nil {
			continue
		}

		// Estimate level/depth (approximate: we don't have exact hop count from BFS,
		// so we use a simple heuristic based on position in the result list)
		// In practice, BFS returns nodes in order of distance
		level := 1
		if len(activatedNodes) > 5 {
			level = 2
		}
		if len(activatedNodes) > 15 {
			level = 3
		}

		// Simple decay 15% per hop: activation = 1.0 * 0.85^level
		activation := math.Pow(0.85, float64(level))

		if activation < e.threshold {
			continue
		}

		nome := "Unnamed"
		if n, ok := nodeResult.Content["nome"]; ok {
			nome = fmt.Sprintf("%v", n)
		} else if c, ok := nodeResult.Content["content"]; ok {
			nome = fmt.Sprintf("%v", c)
		}

		nodeType := nodeResult.NodeType
		if nodeType == "" {
			nodeType = "Unknown"
		}

		node := ActivatedNode{
			ID:         nid,
			Name:       nome,
			Type:       nodeType,
			Activation: activation,
			Level:      level,
			Properties: nodeResult.Content,
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
		Energy:    totalEnergy,
		Depth:     e.maxDepth,
	}

	// Cache logic
	cacheKey := fmt.Sprintf("%s:%s", userID, keyword)
	e.localCache.Store(cacheKey, subgraph)

	// CacheStore write (L2 cache, 5 minutes TTL)
	if e.cacheStore != nil {
		data, _ := json.Marshal(subgraph)
		if err := e.cacheStore.Set(context.Background(), cacheKey, string(data), 5*time.Minute); err != nil {
			log.Printf("[CACHE_ERROR] Failed to cache %s: %v", cacheKey, err)
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

		// 2. L2 Cache (CacheStore)
		valStr, err := e.cacheStore.Get(ctx, cacheKey)
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
		"um": true, "para": true, "com": true, "nao": true, "uma": true, "os": true, "no": true,
		"se": true, "na": true, "por": true, "mais": true, "as": true, "dos": true, "como": true,
		"mas": true, "foi": true, "ao": true, "ele": true, "das": true, "tem": true,
		"seu": true, "sua": true, "ou": true, "ser": true, "quando": true, "muito": true,
		"estou": true, "esta": true, "estava": true,
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
