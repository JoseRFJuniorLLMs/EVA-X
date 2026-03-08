// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"eva/internal/brainstem/database"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// changeVoice troca a voz da EVA em tempo real
func (s *SignalingServer) changeVoice(client *PCMClient, newVoice string) map[string]interface{} {
	log.Printf("[VOICE] Solicitacao de troca de voz: %s -> %s", client.CPF, newVoice)
	ctx := context.Background()

	// Validar se a voz existe via NietzscheDB
	voiceRows, err := s.db.QueryByLabel(ctx, "eva_voices",
		" AND n.voice_name = $vname AND n.is_active = $active",
		map[string]interface{}{
			"vname":  newVoice,
			"active": true,
		}, 1)
	if err != nil || len(voiceRows) == 0 {
		log.Printf("[VOICE] Voz invalida: %s", newVoice)
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Voz '%s' nao encontrada", newVoice),
		}
	}

	// Obter voz atual do idoso
	oldVoice := "Aoede" // Padrao
	idosoNode, err := s.db.GetNodeByID(ctx, "idosos", client.IdosoID)
	if err == nil && idosoNode != nil {
		if pv := database.GetString(idosoNode, "preferred_voice"); pv != "" {
			oldVoice = pv
		}
	}

	// Atualizar preferencia no NietzscheDB
	err = s.db.Update(ctx, "idosos",
		map[string]interface{}{"id": float64(client.IdosoID)},
		map[string]interface{}{
			"preferred_voice": newVoice,
		})
	if err != nil {
		log.Printf("[VOICE] Erro ao atualizar banco: %v", err)
		return map[string]interface{}{
			"success": false,
			"error":   "Erro ao salvar preferencia",
		}
	}

	// Registrar historico via NietzscheDB
	if _, err := s.db.Insert(ctx, "voice_change_history", map[string]interface{}{
		"idoso_id":      client.IdosoID,
		"old_voice":     oldVoice,
		"new_voice":     newVoice,
		"change_method": "voice_command",
		"criado_em":     time.Now().Format(time.RFC3339),
	}); err != nil {
		log.Printf("[VOICE] Erro ao registrar historico de troca: %v", err)
	}

	log.Printf("[VOICE] Voz alterada: %s -> %s (Idoso %d)", oldVoice, newVoice, client.IdosoID)

	// Obter nome amigavel da voz
	displayName := newVoice // fallback
	if len(voiceRows) > 0 {
		if dn := database.GetString(voiceRows[0], "display_name"); dn != "" {
			displayName = dn
		}
	}

	// A mudanca so afeta proxima sessao
	// Gemini nao suporta mudanca de voz mid-session
	return map[string]interface{}{
		"success":      true,
		"old_voice":    oldVoice,
		"new_voice":    newVoice,
		"display_name": displayName,
		"message":      fmt.Sprintf("Voz alterada para %s! A mudanca sera aplicada na proxima conversa.", displayName),
		"takes_effect": "next_session",
	}
}

// getAvailableVoices retorna lista de vozes disponiveis
func (s *SignalingServer) getAvailableVoices() map[string]interface{} {
	ctx := context.Background()

	rows, err := s.db.QueryByLabel(ctx, "eva_voices",
		" AND n.is_active = $active",
		map[string]interface{}{
			"active": true,
		}, 0)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Erro ao buscar vozes",
		}
	}

	var voices []map[string]string
	for _, m := range rows {
		voices = append(voices, map[string]string{
			"voice_name":   database.GetString(m, "voice_name"),
			"display_name": database.GetString(m, "display_name"),
			"gender":       database.GetString(m, "gender"),
			"tone":         database.GetString(m, "tone"),
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

	// Padroes de comando
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
			// Detectar voz especifica mencionada
			voiceNames := []string{
				// Classicas
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
					return true, cases.Title(language.English).String(voiceName)
				}
			}

			// Sem voz especifica: retornar voz aleatoria
			return true, "" // Vazio = escolher aleatoria
		}
	}

	return false, ""
}
