package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// changeVoice troca a voz da EVA em tempo real
func (s *SignalingServer) changeVoice(client *PCMClient, newVoice string) map[string]interface{} {
	log.Printf("🎙️ [VOICE] Solicitação de troca de voz: %s → %s", client.CPF, newVoice)

	// Validar se a voz existe
	var exists bool
	err := s.db.GetConnection().QueryRow(`
		SELECT EXISTS(SELECT 1 FROM eva_voices WHERE voice_name = $1 AND is_active = true)
	`, newVoice).Scan(&exists)

	if err != nil || !exists {
		log.Printf("❌ [VOICE] Voz inválida: %s", newVoice)
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Voz '%s' não encontrada", newVoice),
		}
	}

	// Obter voz atual
	var currentVoice sql.NullString
	s.db.GetConnection().QueryRow(`
		SELECT preferred_voice FROM idosos WHERE id = $1
	`, client.IdosoID).Scan(&currentVoice)

	oldVoice := "Aoede" // Padrão
	if currentVoice.Valid {
		oldVoice = currentVoice.String
	}

	// Atualizar preferência no banco
	_, err = s.db.GetConnection().Exec(`
		UPDATE idosos SET preferred_voice = $1 WHERE id = $2
	`, newVoice, client.IdosoID)

	if err != nil {
		log.Printf("❌ [VOICE] Erro ao atualizar banco: %v", err)
		return map[string]interface{}{
			"success": false,
			"error":   "Erro ao salvar preferência",
		}
	}

	// Registrar histórico
	s.db.GetConnection().Exec(`
		INSERT INTO voice_change_history (idoso_id, old_voice, new_voice, change_method)
		VALUES ($1, $2, $3, 'voice_command')
	`, client.IdosoID, oldVoice, newVoice)

	log.Printf("✅ [VOICE] Voz alterada: %s → %s (Idoso %d)", oldVoice, newVoice, client.IdosoID)

	// Obter nome amigável da voz
	var displayName string
	s.db.GetConnection().QueryRow(`
		SELECT display_name FROM eva_voices WHERE voice_name = $1
	`, newVoice).Scan(&displayName)

	// **CRÍTICO:** A mudança só afeta próxima sessão
	// Para mudar EM TEMPO REAL, precisamos reconfigurar Gemini
	// Infelizmente, Gemini não suporta mudança de voz mid-session
	// Então vamos avisar que mudará na próxima chamada

	return map[string]interface{}{
		"success":      true,
		"old_voice":    oldVoice,
		"new_voice":    newVoice,
		"display_name": displayName,
		"message":      fmt.Sprintf("Voz alterada para %s! A mudança será aplicada na próxima conversa.", displayName),
		"takes_effect": "next_session",
	}
}

// getAvailableVoices retorna lista de vozes disponíveis
func (s *SignalingServer) getAvailableVoices() map[string]interface{} {
	rows, err := s.db.GetConnection().Query(`
		SELECT voice_name, display_name, gender, tone
		FROM eva_voices
		WHERE is_active = true
		ORDER BY display_name
	`)

	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Erro ao buscar vozes",
		}
	}
	defer rows.Close()

	var voices []map[string]string
	for rows.Next() {
		var voiceName, displayName, gender, tone string
		rows.Scan(&voiceName, &displayName, &gender, &tone)

		voices = append(voices, map[string]string{
			"voice_name":   voiceName,
			"display_name": displayName,
			"gender":       gender,
			"tone":         tone,
		})
	}

	return map[string]interface{}{
		"success": true,
		"voices":  voices,
		"count":   len(voices),
	}
}

// detectVoiceChangeCommand detecta comando de troca de voz na fala
func detectVoiceChangeCommand(text string) (bool, string) {
	textLower := strings.ToLower(text)

	// Padrões de comando
	patterns := []string{
		"troque sua voz",
		"mude sua voz",
		"troca de voz",
		"mudar voz",
		"outra voz",
		"voz diferente",
	}

	for _, pattern := range patterns {
		if strings.Contains(textLower, pattern) {
			// Detectar voz específica mencionada
			voiceNames := []string{
				// Clássicas
				"aoede", "charon", "fenrir", "kore", "puck",
				// Novas Femininas
				"zephyr", "leda", "callirrhoe", "autonoe", "despina",
				"erinome", "laomedeia",
				// Novas Masculinas
				"orus", "enceladus", "iapetus", "umbriel", "algieba",
				"algenib", "rasalgethi", "alnilam",
			}

			for _, voiceName := range voiceNames {
				if strings.Contains(textLower, voiceName) {
					// Capitalize primeira letra
					return true, strings.Title(voiceName)
				}
			}

			// Sem voz específica: retornar voz aleatória
			return true, "" // Vazio = escolher aleatória
		}
	}

	return false, ""
}
