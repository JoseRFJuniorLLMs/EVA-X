// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"eva/internal/cortex/gemini"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// geminiSemMemoria — Chat REST Stateless para Malaria-Angolar
// ============================================================================
// Consumer:  geminiSemMemoria
// Rota:      POST /api/chat
// Client:    internal/cortex/gemini → AnalyzeText() (REST v1beta, nao WebSocket)
// Frontend:  Malaria-Angolar (qualquer componente)
// Protocolo: REST HTTP — request/response simples, sem sessao, sem streaming
// Ver:       GEMINI_ARCHITECTURE.md para documentacao completa

// chatRequest representa o body do POST /api/chat
type chatRequest struct {
	CPF     string `json:"cpf"`
	Message string `json:"message"`
	Context string `json:"context,omitempty"` // contexto do sistema (vem do frontend)
}

// chatResponse representa a resposta do POST /api/chat
type chatResponse struct {
	Response string `json:"response"`
	CPF      string `json:"cpf,omitempty"`
}

// handleChat processa mensagens de texto via REST usando Gemini
func (s *SignalingServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"JSON invalido"}`, http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, `{"error":"campo message obrigatorio"}`, http.StatusBadRequest)
		return
	}

	// Busca dados do paciente se CPF fornecido
	patientContext := ""
	if req.CPF != "" {
		idoso, err := s.db.GetIdosoByCPF(req.CPF)
		if err == nil && idoso != nil {
			patientContext = "\n\n[Contexto do paciente: " + idoso.Nome + "]"
		}
	}

	// Contexto vem do frontend. Se nao enviou, usa generico minimo.
	systemPrompt := req.Context
	if systemPrompt == "" {
		systemPrompt = "Voce e a EVA, assistente virtual inteligente. Responda em portugues de forma clara e profissional."
	}
	systemPrompt += patientContext

	fullPrompt := systemPrompt + "\n\nUsuario: " + req.Message

	// Chama Gemini via REST client do EVA-Mind
	response, err := gemini.AnalyzeText(s.cfg, fullPrompt)
	if err != nil {
		log.Error().Err(err).Msg("Erro ao chamar Gemini para chat")
		http.Error(w, `{"error":"erro ao processar mensagem"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{
		Response: response,
		CPF:      req.CPF,
	})
}
