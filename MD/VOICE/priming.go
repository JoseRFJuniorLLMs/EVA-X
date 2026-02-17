// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package voice

import (
	"fmt"
	"strings"
)

// BuildGeminiPriming constrói o bloco de contexto que é injetado no
// System Instruction do Gemini 2.5 Flash antes de cada resposta.
//
// O priming contém:
//   1. Identidade confirmada via biometria
//   2. Nível de confiança e qualidade do áudio
//   3. Instrução para usar o histórico personalizado
//   4. Aviso contextual se a qualidade do áudio estiver baixa
//
// Exemplo de saída:
//   [BIOMETRIA] Falante identificado: Junior (confiança: 94%)
//   Use o histórico, preferências e contexto emocional do Junior para responder.
//   O áudio está limpo. Responda normalmente.

func BuildGeminiPriming(result IdentificationResult) string {
	if result.Unknown {
		return buildUnknownPriming(result)
	}

	var sb strings.Builder

	// ── Bloco de identidade ───────────────────────────────────────────────
	confPct := int(result.Confidence * 100)
	sb.WriteString(fmt.Sprintf(
		"[BIOMETRIA_EVA] Falante identificado via SRC/OMP: **%s** (confiança: %d%%)\n",
		result.Name, confPct,
	))

	// ── Instrução de personalização ───────────────────────────────────────
	sb.WriteString(fmt.Sprintf(
		"Recupere e use o contexto, histórico de conversas, preferências e estado emocional de **%s**.\n",
		result.Name,
	))

	// ── Aviso de qualidade de áudio ───────────────────────────────────────
	audioNote := audioQualityNote(result.AudioQuality, result.SpeechRatio)
	if audioNote != "" {
		sb.WriteString(audioNote + "\n")
	}

	// ── Calibração de confiança ───────────────────────────────────────────
	if result.Confidence < 0.90 {
		sb.WriteString(fmt.Sprintf(
			"⚠️ Confiança moderada (%d%%). Se houver ambiguidade no contexto, confirme sutilmente a identidade.\n",
			confPct,
		))
	}

	return sb.String()
}

// buildUnknownPriming gera o priming para voz não reconhecida.
// A EVA inicia o protocolo de apresentação/aprendizado.
func buildUnknownPriming(result IdentificationResult) string {
	var sb strings.Builder
	sb.WriteString("[BIOMETRIA_EVA] Falante NÃO identificado no dicionário de vozes.\n")
	sb.WriteString("Inicie o protocolo de apresentação: pergunte o nome do usuário de forma natural e amigável.\n")
	sb.WriteString("Se o usuário concordar, informe que você pode aprender a reconhecer a voz dele para futuras conversas.\n")

	if result.AudioQuality < 5.0 {
		sb.WriteString("⚠️ Qualidade de áudio muito baixa — pode ser ruído de fundo. Confirme se está ouvindo bem.\n")
	}

	return sb.String()
}

// BuildVoiceDeviationAlert gera o alerta enviado pela EVA quando detecta
// desvio de voz significativo (ex: usuário gripado, emocionalmente alterado).
//
// Isso é o RAM (Realistic Accuracy Model) em ação: a EVA percebe a mudança
// e demonstra empatia proativamente.
func BuildVoiceDeviationAlert(result IdentificationResult, baselineVariance float64) string {
	deviation := result.ResidualErr

	switch {
	case deviation > 0.30:
		// Desvio severo — pode ser outro falante tentando se passar
		return fmt.Sprintf(
			"[BIOMETRIA_EVA] ⚠️ Desvio severo de voz detectado para %s (residual: %.2f). "+
				"Verifique se é mesmo o usuário esperado antes de revelar informações sensíveis.",
			result.Name, deviation,
		)
	case deviation > 0.18:
		// Desvio moderado — provavelmente condição física (gripe, cansaço, estresse)
		return fmt.Sprintf(
			"[BIOMETRIA_EVA] A voz de %s apresenta desvio do padrão biométrico (residual: %.2f). "+
				"É natural perguntar sutilmente se está tudo bem ou se algo aconteceu.",
			result.Name, deviation,
		)
	default:
		return "" // Sem desvio significativo
	}
}

// InjectIntoPriming injeta o priming de voz no system prompt existente do Gemini.
// Posiciona o bloco BIOMETRIA logo após o system prompt base para máxima atenção.
func InjectIntoPriming(existingSystemPrompt, voicePriming string) string {
	if voicePriming == "" {
		return existingSystemPrompt
	}
	return voicePriming + "\n---\n" + existingSystemPrompt
}

// audioQualityNote retorna uma nota sobre qualidade de áudio para o Gemini.
func audioQualityNote(rmsDB, speechRatio float64) string {
	switch {
	case rmsDB < 5.0:
		return "⚠️ Áudio com muito ruído de fundo. Considere pedir ao usuário para se aproximar do microfone."
	case speechRatio < 0.40:
		return "ℹ️ Grande parte do áudio foi silêncio. O usuário pode estar em ambiente barulhento."
	default:
		return "" // Áudio ok — não sobrecarrega o contexto com nota desnecessária
	}
}
