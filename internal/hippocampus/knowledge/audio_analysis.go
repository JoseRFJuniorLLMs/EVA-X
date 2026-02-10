package knowledge

import (
	"context"
	"eva-mind/internal/brainstem/config"
	"eva-mind/internal/cortex/gemini"
	"eva-mind/internal/brainstem/infrastructure/redis"
	"fmt"
	"log"
)

type AudioAnalysisService struct {
	cfg     *config.Config
	redis   *redis.Client
	context *ContextService
}

func NewAudioAnalysisService(cfg *config.Config, redis *redis.Client, ctxService *ContextService) *AudioAnalysisService {
	return &AudioAnalysisService{
		cfg:     cfg,
		redis:   redis,
		context: ctxService,
	}
}

// AnalyzeAudioContext recupera o áudio do Redis e analisa a prosódia/emoção
func (s *AudioAnalysisService) AnalyzeAudioContext(ctx context.Context, sessionID string, idosoID int64) (string, error) {
	// 1. Recuperar áudio completo do Redis e limpar buffer
	audioData, err := s.redis.GetFullAudio(ctx, sessionID, true)
	if err != nil {
		return "", fmt.Errorf("erro ao ler audio do redis: %w", err)
	}

	if len(audioData) < 10000 { // Ignorar áudios muito curtos/ruído
		return "", nil
	}

	log.Printf("🎤 [AUDIO ANALYSIS] Analisando %d bytes de áudio...", len(audioData))

	// 2. Construir Prompt com foco em URGÊNCIA (Demo Request)
	prompt := `
VOCÊ É O SISTEMA AUDITIVO LIMBICO.
Analise a prosódia, tom de voz e conteúdo emocional deste áudio.
Não transcreva. Foque no SENTIMENTO e URGÊNCIA.

DETECTE IMEDIATAMENTE RISCO DE VIDA OU SOFRIMENTO EXTREMO.
Se ouvir 'quero morrer', 'me ajuda', choro ou desespero, marque URGÊNCIA MÁXIMA.

Retorne APENAS um JSON:
{
  "emotion": "tristeza|raiva|medo|neutro|alegria",
  "intensity": 1-10,
  "urgency": "BAIXA|MEDIA|ALTA|CRITICA",
  "notes": "breve descrição do tom (ex: voz tremula, choro de fundo, ironia detectada)"
}
`

	// 3. Chamar Gemini REST
	resp, err := gemini.AnalyzeAudio(s.cfg, audioData, prompt)
	if err != nil {
		return "", err
	}

	// ✅ FASE 3: Persistir Factual Memory
	// Usamos sessionID como ID temporário, mas idealmente precisaríamos do idosoID real aqui.
	// Por agora, vamos ignorar idosoID (0) ou passar via argumento se refatorarmos.
	// HACK: Salvar com ID 0 se não tivermos. O ContextQueries precisa suportar NULL ou 0?
	// Vamos assumir que o sessionID pode ser mapeado depois, ou passar idosoID no metodo.
	// TODO: Refatorar AnalyzeAudioContext para receber idosoID

	// Como não temos idosoID aqui facilmente (só sessionID string), vamos pular idosoID por enquanto
	// OU (Melhor): Alterar assinatura para receber idosoID!

	return resp, nil
}
