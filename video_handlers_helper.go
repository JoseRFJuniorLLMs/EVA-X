// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// ==========================================
// 🎥 VIDEO SIGNALING HANDLERS (WebRTC via DB)
// ==========================================

func (s *SignalingServer) handleCreateVideoSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		IdosoID   int64  `json:"idoso_id"` // Opcional, se não vier pegamos do contexto ou fixo por enquanto
		SdpOffer  string `json:"sdp_offer"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// ⚠️ Como o fluxo REST não tem auth ainda, vamos fixar ou pegar de algum header.
	// Para o MVP, vamos assumir IdosoID = 1 se não vier, ou confiar no Mobile enviando.
	if req.IdosoID == 0 {
		req.IdosoID = 1 // Default fallback
	}

	log.Printf("🎥 Criando Sessão de Vídeo: %s (Idoso: %d)", req.SessionID, req.IdosoID)

	// Salvar no banco
	err := s.db.CreateVideoSession(req.SessionID, req.IdosoID, req.SdpOffer)
	if err != nil {
		log.Printf("❌ Erro ao criar sessão de vídeo: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// 🔔 Notificar Operador (Via Firebase ou Log)
	// Aqui poderíamos enviar um Push para o Painel Administrativo
	go func() {
		// Mock notification
		log.Printf("🔔 NOTIFICANDO OPERADOR SOBRE CHAMADA DE VÍDEO: %s", req.SessionID)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "created",
		"session_id": req.SessionID,
	})
}

func (s *SignalingServer) handleCreateVideoCandidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string                 `json:"session_id"`
		Sender    string                 `json:"sender"`
		Type      string                 `json:"type"`
		Payload   map[string]interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payloadJSON, _ := json.Marshal(req.Payload)
	log.Printf("❄️ ICE Candidate (%s): %s", req.Sender, req.SessionID)

	err := s.db.CreateSignalingMessage(req.SessionID, req.Sender, req.Type, string(payloadJSON))
	if err != nil {
		log.Printf("❌ Erro ao salvar candidato ICE: %v", err)
		http.Error(w, "Failed to save candidate", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *SignalingServer) handleGetVideoAnswer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	answer, err := s.db.GetVideoSessionAnswer(sessionID)
	if err != nil {
		log.Printf("❌ Erro ao buscar answer: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if answer == "" {
		// Ainda não tem resposta (Polling continue)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sdp_answer": nil,
			"status":     "waiting",
		})
		return
	}

	// Operador atendeu!
	json.NewEncoder(w).Encode(map[string]string{
		"sdp_answer": answer,
		"status":     "answered",
	})
}

// Retorna a sessão (com o Offer) para o Operador
func (s *SignalingServer) handleGetVideoSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	session, err := s.db.GetVideoSession(sessionID)
	if err != nil {
		log.Printf("❌ Session not found: %s (%v)", sessionID, err)
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// Salva o Answer do Operador (POST /video/session/{id}/answer)
func (s *SignalingServer) handleSaveVideoAnswer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	var req struct {
		SdpAnswer string `json:"sdp_answer"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("✅ Salvando Answer para sessão %s", sessionID)
	err := s.db.UpdateVideoSessionAnswer(sessionID, req.SdpAnswer)
	if err != nil {
		log.Printf("❌ Erro ao salvar Answer: %v", err)
		http.Error(w, "Failed to update session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Operador verifica candidatos vindos do Mobile
func (s *SignalingServer) handleGetMobileCandidates(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Pega query param ?since=0
	sinceID := int64(0)
	// (Simplificado para MVP, pegando tudo > 0)

	candidates, err := s.db.GetMobileCandidates(sessionID, sinceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(candidates)
}

// Retorna lista de sessões pendentes (Poll do Dashboard)
func (s *SignalingServer) handleGetPendingSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.db.GetPendingVideoSessions()
	if err != nil {
		log.Printf("❌ Erro ao buscar sessões pendentes: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if sessions == nil {
		json.NewEncoder(w).Encode([]interface{}{})
	} else {
		json.NewEncoder(w).Encode(sessions)
	}
}
