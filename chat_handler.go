// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"eva-mind/internal/cortex/gemini"
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

	// System prompt para o contexto de malaria
	systemPrompt := `Voce e a EVA Assistente Malaria Angola, uma assistente inteligente do Sistema de Gestao de Malaria de Angola.

Seu papel:
- Ajudar profissionais de saude a usar o sistema
- Explicar funcionalidades (pacientes, amostras, deteccao AI, estatisticas, clinicas)
- Responder duvidas sobre malaria, diagnostico e tratamento
- Orientar sobre o fluxo de trabalho clinico
- Responder de forma clara, concisa e profissional em portugues

Funcionalidades do sistema:
- Dashboard: visao geral de estatisticas, amostras recentes, alertas
- Pacientes: cadastro e gestao de pacientes com historico
- Amostras: coleta, analise por IA (YOLOv8 com 99.14% mAP50), revisao medica
- Deteccao: upload de imagens de laminas para deteccao automatica de parasitas
- Clinicas: gestao de unidades de saude em Angola

Especies de Plasmodium em Angola:
- P. falciparum (95% dos casos) - mais grave
- P. vivax (~2%) - moderado
- P. malariae (~2%) - leve
- P. ovale (~1%) - moderado

Mantenha respostas curtas (2-4 frases) a menos que o usuario peca explicacoes detalhadas.` + patientContext

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
