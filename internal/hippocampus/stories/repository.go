package stories

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/vector" // Correct package for EmbeddingService
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/pkg/types"
	"fmt"
)

type Repository struct {
	qdrant   *vector.QdrantClient
	embedder *memory.EmbeddingService
}

func NewRepository(qdrant *vector.QdrantClient, embedder *memory.EmbeddingService) *Repository {
	return &Repository{
		qdrant:   qdrant,
		embedder: embedder,
	}
}

// FindRelatedStories busca histórias baseadas na emoção ou contexto atual
func (r *Repository) FindRelatedStories(ctx context.Context, query string, limit int) ([]*types.TherapeuticStory, error) {
	// 1. Gerar embedding da query (emoção/contexto)
	vector, err := r.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for story search: %w", err)
	}

	// 2. Buscar no Qdrant
	results, err := r.qdrant.Search(ctx, "stories", vector, uint64(limit), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search stories: %w", err)
	}

	var stories []*types.TherapeuticStory

	// 3. Mapear resultados
	for _, res := range results {
		story := &types.TherapeuticStory{}

		// Extrair campos do Payload
		if title, ok := res.Payload["title"]; ok {
			story.Title = title.GetStringValue()
		}
		if content, ok := res.Payload["content"]; ok {
			story.Content = content.GetStringValue()
		}
		if archetype, ok := res.Payload["archetype"]; ok {
			story.Archetype = archetype.GetStringValue()
		}
		if moral, ok := res.Payload["moral"]; ok {
			story.Moral = moral.GetStringValue()
		}

		// Tags e TargetEmotions (Listas)
		if tags, ok := res.Payload["tags"]; ok {
			if list := tags.GetListValue(); list != nil {
				for _, v := range list.Values {
					story.Tags = append(story.Tags, v.GetStringValue())
				}
			}
		}

		stories = append(stories, story)
	}

	return stories, nil
}
