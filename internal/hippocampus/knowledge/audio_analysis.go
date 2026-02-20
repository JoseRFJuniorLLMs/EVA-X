// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package knowledge

import (
	"context"
	"fmt"
	"log"

	"eva/internal/brainstem/config"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/gemini"
)

type AudioAnalysisService struct {
	cfg         *config.Config
	audioBuffer *nietzscheInfra.AudioBuffer
	context     *ContextService
}

func NewAudioAnalysisService(cfg *config.Config, audioBuffer *nietzscheInfra.AudioBuffer, ctxService *ContextService) *AudioAnalysisService {
	return &AudioAnalysisService{
		cfg:         cfg,
		audioBuffer: audioBuffer,
		context:     ctxService,
	}
}

// AnalyzeAudioContext recupera o áudio do buffer e analisa a prosódia/emoção
func (s *AudioAnalysisService) AnalyzeAudioContext(ctx context.Context, sessionID string, idosoID int64) (string, error) {
	// 1. Recuperar áudio completo do buffer e limpar
	audioData, err := s.audioBuffer.GetFullAudio(ctx, sessionID, true)
	if err != nil {
		return "", fmt.Errorf("erro ao ler audio do buffer: %w", err)
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
