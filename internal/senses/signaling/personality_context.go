// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package signaling

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// getPersonalityContext busca estado de personalidade e monta contexto para o prompt
func getPersonalityContext(idosoID int64, db *sql.DB) string {
	// Buscar estado de personalidade
	var (
		relationshipLevel int
		conversationCount int
		lastInteraction   time.Time
		dominantEmotion   string
		favoriteTopics    string
		firstMeetingDate  time.Time
	)

	query := `
		SELECT 
			relationship_level,
			conversation_count,
			last_interaction,
			dominant_emotion,
			favorite_topics,
			first_meeting_date
		FROM eva_personality_state
		WHERE idoso_id = $1
	`

	err := db.QueryRow(query, idosoID).Scan(
		&relationshipLevel,
		&conversationCount,
		&lastInteraction,
		&dominantEmotion,
		&favoriteTopics,
		&firstMeetingDate,
	)

	if err != nil {
		// Se não existir registro, retornar vazio (primeira conversa)
		return ""
	}

	// Calcular dias desde primeira conversa
	daysSinceMeeting := int(time.Since(firstMeetingDate).Hours() / 24)
	daysSinceLastCall := int(time.Since(lastInteraction).Hours() / 24)

	// Determinar estilo de tratamento
	style := getRelationshipStyle(relationshipLevel)
	label := getRelationshipLabel(relationshipLevel)

	// Parse tópicos favoritos
	topics := parseTopics(favoriteTopics)

	// Montar contexto
	context := fmt.Sprintf(`

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧠 CONTEXTO DE RELACIONAMENTO AFETIVO (PERSONALIDADE DA EVA)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 VÍNCULO EMOCIONAL:
- Nível de Intimidade: %d/10 (%s)
- Vocês conversam há: %d dias
- Total de conversas: %d
- Última conversa foi há: %d dias

💬 ESTILO DE COMUNICAÇÃO:
- Tratamento recomendado: %s
- Emoção predominante do paciente: %s`,
		relationshipLevel,
		label,
		daysSinceMeeting,
		conversationCount,
		daysSinceLastCall,
		style,
		dominantEmotion,
	)

	if len(topics) > 0 {
		context += fmt.Sprintf("\n- Tópicos favoritos: %s", strings.Join(topics, ", "))
	}

	// Instruções específicas baseadas no nível
	context += "\n\n🎭 INSTRUÇÕES DE COMPORTAMENTO AFETIVO:\n"

	switch relationshipLevel {
	case 1, 2:
		context += "- Você está CONHECENDO esta pessoa. Seja respeitosa e profissional.\n"
		context += "- Use tratamento formal (Senhora/Senhor).\n"
		context += "- Faça perguntas para conhecê-la melhor.\n"
	case 3, 4, 5:
		context += "- Vocês já são AMIGAS. Seja calorosa e atenciosa.\n"
		context += "- Pode usar 'Dona/Seu' no nome.\n"
		context += "- Demonstre que lembra de conversas anteriores.\n"
	case 6, 7, 8:
		context += "- Vocês são MUITO PRÓXIMAS (confidentes). Seja afetuosa.\n"
		context += "- Pode usar o primeiro nome ou um apelido carinhoso.\n"
		context += "- Demonstre preocupação genuína e carinho.\n"
	default: // 9, 10
		context += "- Vocês são COMO FAMÍLIA. Seja extremamente carinhosa e íntima.\n"
		context += "- Use apelidos carinhosos (ex: 'minha querida', 'meu amor').\n"
		context += "- Demonstre saudades e alegria genuína em conversar.\n"
	}

	// Contexto especial se não conversam há dias
	if daysSinceLastCall >= 3 {
		context += fmt.Sprintf("\n⚠️ IMPORTANTE: Faz %d dias que vocês não conversam!\n", daysSinceLastCall)
		context += "- Demonstre que sentiu falta e estava preocupada.\n"
		context += "- Pergunte se está tudo bem.\n"
	}

	context += "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

	return context
}

func getRelationshipStyle(level int) string {
	switch {
	case level <= 2:
		return "Formal (Senhora Maria, Senhor João)"
	case level <= 5:
		return "Amigável (Dona Maria, Seu João)"
	case level <= 8:
		return "Íntimo (Maria, João ou apelido)"
	default:
		return "Familiar (Mariazinha, Joãozinho, minha querida, meu amor)"
	}
}

func getRelationshipLabel(level int) string {
	labels := map[int]string{
		1:  "Nos conhecendo",
		2:  "Conhecidas",
		3:  "Amigas",
		4:  "Boas amigas",
		5:  "Amigas próximas",
		6:  "Confidentes",
		7:  "Muito próximas",
		8:  "Inseparáveis",
		9:  "Como família",
		10: "Família do coração",
	}

	if label, ok := labels[level]; ok {
		return label
	}
	return "Conhecidas"
}

func parseTopics(topicsStr string) []string {
	if topicsStr == "{}" || topicsStr == "" {
		return []string{}
	}

	// Remove {} e split
	topicsStr = strings.Trim(topicsStr, "{}")
	if topicsStr == "" {
		return []string{}
	}

	parts := strings.Split(topicsStr, ",")
	var result []string
	for _, p := range parts {
		cleaned := strings.Trim(strings.Trim(p, "\""), " ")
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}

	return result
}
