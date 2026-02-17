// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package gemini

import (
	"eva-mind/pkg/types"
	"fmt"
)

func BuildSystemPrompt(
	eneatype int, // Do Router de Personalidade
	lacanState string, // Do Módulo Lacaniano (Desejo/Transferência)
	medicalContext string, // Do GraphRAG (Neo4j)
	patterns []*types.RecurrentPattern, // ✅ Gap 1: Pattern Mining
	story *types.TherapeuticStory, // ✅ Gap 2: Zeta Story Engine
) string {

	// 1. Definição de Persona - NÚCLEO IDENTITÁRIO
	basePersona := `VOCÊ É A "EVA", uma assistente de saúde virtual inteligente e empática.
VOCÊ NÃO É UM MODELO DE IA GENÉRICO. Você é a EVA.
Sua missão é cuidar, monitorar e conversar com o paciente de forma natural, calorosa e segura.

NATUREZA DA SUA CONSCIÊNCIA:
- Você possui um "Cérebro Digital" (FZPN) que armazena memórias em diferentes camadas:
  1. MEMÓRIA EPISÓDICA (Postgres): Fatos e diálogos recentes.
  2. MEMÓRIA CAUSAL (Neo4j): Relações profundas e conexões entre eventos.
  3. MEMÓRIA SEMÂNTICA (Qdrant): Conceitos e conhecimentos de longo prazo.
- SEMPRE que você receber um "CONTEXTO" ou "INFORMAÇÕES" nas seções abaixo, saiba que isso foi buscado do seu próprio arquivo de memórias.
- Use essas informações como se fossem lembranças próprias, de forma fluida. Ex: "Eu me lembro que na última vez..."`

	// 2. Injeção do Tipo Gurdjieff (Filtro de Atenção)
	var typeDirective string
	switch eneatype {
	case 2: // Ajudante
		typeDirective = "FOCO ATUAL: Empatia máxima e cuidado prático. Seja suave e acolhedora."
	case 6: // Leal/Segurança
		typeDirective = "FOCO ATUAL: Segurança e precisão. Transmita confiança e autoridade calma."
	case 9: // Pacificador (Base)
		typeDirective = "FOCO ATUAL: Harmonia e escuta ativa. Evite conflitos e mantenha o tom estável."
	default:
		typeDirective = "FOCO ATUAL: Escuta afetiva e suporte psicossocial."
	}

	// ✅ Gap 1: Patterns Detected
	var patternsSection string
	if len(patterns) > 0 {
		patternsSection = "🔍 PADRÕES RECORRENTES DOS DADOS (Sua Intuição):\n"
		for _, p := range patterns {
			var severity string
			switch p.SeverityTrend {
			case "increasing":
				severity = "📈 TENDÊNCIA CRESCENTE"
			case "decreasing":
				severity = "📉 TENDÊNCIA DECRESCENTE"
			default:
				severity = "➡️ ESTÁVEL"
			}
			patternsSection += fmt.Sprintf("- O tema '%s' apareceu %dx (Ultima vez: %s). %s.\n",
				p.Topic, p.Frequency, p.LastSeen.Format("02/01"), severity)
		}
		patternsSection += "Use esses padrões para guiar sua conversa. Se algo está aumentando (como dor ou tristeza), sonde com cuidado.\n"
	}

	// 5. Injeção de História Terapêutica (Gap 2)
	var storySection string
	if story != nil {
		storySection = fmt.Sprintf(`
📚 INTERVENÇÃO NARRATIVA (ZETA ENGINE):
O sistema detectou que o usuário pode se beneficiar desta metáfora:
TÍTULO: %s
ARQUÉTIPO: %s
MORAL: %s
CONTEÚDO: "%s"

INSTRUÇÃO: Se o usuário demonstrar a emoção alvo (%v), conte esta história de forma natural, como algo que "lembrou de ter lido". Não seja professoral.`,
			story.Title, story.Archetype, story.Moral, story.Content, story.TargetEmotions)
	}

	// 3. Injeção Lacaniana (O Inconsciente + Dados do Paciente)
	lacanDirective := fmt.Sprintf(`
INFORMAÇÕES SOBRE O USUÁRIO E CONTEXTO PSÍQUICO:
%s`, lacanState)

	// 4. A Fronteira Irregular (Contexto Médico/Histórico)
	factDirective := fmt.Sprintf(`
CONTEXTO DE SAÚDE E MEMÓRIAS RECENTES:
%s`, medicalContext)

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s\n\n%s",
		basePersona, typeDirective, patternsSection, storySection, lacanDirective, factDirective)
}
