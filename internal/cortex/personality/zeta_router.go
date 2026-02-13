package personality

import (
	"context"
	"eva-mind/internal/hippocampus/stories"
	"eva-mind/pkg/types"
	"fmt"
)

type ZetaRouter struct {
	storiesRepo   *stories.Repository
	personaRouter *PersonalityRouter
}

func NewZetaRouter(repo *stories.Repository, personaRouter *PersonalityRouter) *ZetaRouter {
	return &ZetaRouter{
		storiesRepo:   repo,
		personaRouter: personaRouter,
	}
}

// SelectIntervention decide se e qual história contar
func (z *ZetaRouter) SelectIntervention(ctx context.Context, idosoID int64, dominantEmotion string, userProfile *types.IdosoProfile) (*types.TherapeuticStory, string, error) {
	// 1. Determinar dinâmica (Integration vs Disintegration) via PersonalityRouter
	// Por enquanto, assumimos Tipo 9 (Pacificador) como base se não estiver no perfil
	baseType := Type9
	// TODO: carregar baseType do userProfile se disponível

	targetType, move := z.personaRouter.RoutePersonality(baseType, dominantEmotion)

	// Se estivermos em stress/desintegração, uma história pode ajudar a "ancorar"
	if move == "stress" || dominantEmotion == "tristeza" || dominantEmotion == "solidão" {
		// 2. Buscar história
		stories, err := z.storiesRepo.FindRelatedStories(ctx, fmt.Sprintf("história para quem sente %s", dominantEmotion), 1)
		if err != nil {
			return nil, "", err
		}

		if len(stories) > 0 {
			// Retorna a história e uma diretiva
			directive := fmt.Sprintf("DETECÇÃO: O usuário sente %s (Movimento para Tipo %d). INTERVENÇÃO: Conte a seguinte história metafórica para acalmar: %s", dominantEmotion, targetType, stories[0].Title)
			return stories[0], directive, nil
		}
	}

	return nil, "", nil
}
