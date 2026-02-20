// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package stories

import (
	"context"
	"fmt"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/memory"
	"eva/pkg/types"
)

type Repository struct {
	vectorAdapter *nietzscheInfra.VectorAdapter
	embedder      *memory.EmbeddingService
}

func NewRepository(vectorAdapter *nietzscheInfra.VectorAdapter, embedder *memory.EmbeddingService) *Repository {
	return &Repository{
		vectorAdapter: vectorAdapter,
		embedder:      embedder,
	}
}

// FindRelatedStories busca histórias baseadas na emoção ou contexto atual
func (r *Repository) FindRelatedStories(ctx context.Context, query string, limit int) ([]*types.TherapeuticStory, error) {
	// 1. Gerar embedding da query (emoção/contexto)
	vec, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for story search: %w", err)
	}

	// 2. Buscar no NietzscheDB
	results, err := r.vectorAdapter.Search(ctx, "stories", vec, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to search stories: %w", err)
	}

	var stories []*types.TherapeuticStory

	// 3. Mapear resultados
	for _, res := range results {
		story := &types.TherapeuticStory{}

		// Extrair campos do Payload
		if title, ok := res.Payload["title"]; ok {
			if s, ok := title.(string); ok {
				story.Title = s
			}
		}
		if content, ok := res.Payload["content"]; ok {
			if s, ok := content.(string); ok {
				story.Content = s
			}
		}
		if archetype, ok := res.Payload["archetype"]; ok {
			if s, ok := archetype.(string); ok {
				story.Archetype = s
			}
		}
		if moral, ok := res.Payload["moral"]; ok {
			if s, ok := moral.(string); ok {
				story.Moral = s
			}
		}

		// Tags (lista)
		if tags, ok := res.Payload["tags"]; ok {
			if tagList, ok := tags.([]interface{}); ok {
				for _, v := range tagList {
					if s, ok := v.(string); ok {
						story.Tags = append(story.Tags, s)
					}
				}
			}
		}

		stories = append(stories, story)
	}

	return stories, nil
}
