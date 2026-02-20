// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package oauth

import (
	"context"
	"encoding/json"
	"eva/internal/brainstem/database"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	service         *Service
	db              *database.DB
	frontendBaseURL string
}

func NewHandler(service *Service, db *database.DB, frontendBaseURL string) *Handler {
	return &Handler{
		service:         service,
		db:              db,
		frontendBaseURL: frontendBaseURL,
	}
}

// HandleAuthorize redirects user to Google OAuth consent screen with HMAC-signed state
func (h *Handler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	cpf := r.URL.Query().Get("cpf")
	if cpf == "" {
		http.Error(w, "CPF required", http.StatusBadRequest)
		return
	}

	// Validate CPF exists in database
	if _, err := h.db.GetIdosoByCPF(cpf); err != nil {
		log.Warn().Str("cpf", cpf).Err(err).Msg("[OAUTH] CPF nao encontrado no banco")
		http.Error(w, "CPF not found", http.StatusNotFound)
		return
	}

	// Generate HMAC-signed state with CPF embedded
	state := h.service.SignState(cpf)

	// Store state in cookie for CSRF verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.service.GetAuthURL(state)
	log.Info().Str("cpf", cpf[:3]+"***").Msg("[OAUTH] Redirecionando para Google OAuth")
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback processes OAuth callback from Google
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")

	// Verify HMAC state and extract CPF
	cpf, err := h.service.VerifyState(state)
	if err != nil {
		log.Warn().Err(err).Msg("[OAUTH] State invalido")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in request", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := h.service.ExchangeCode(ctx, code)
	if err != nil {
		log.Error().Err(err).Msg("[OAUTH] Falha ao trocar code")
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Get user info (email)
	email, err := h.service.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		log.Warn().Err(err).Msg("[OAUTH] Falha ao obter email do usuario")
	}

	// Lookup idoso by CPF to save tokens
	idoso, err := h.db.GetIdosoByCPF(cpf)
	if err != nil {
		log.Error().Err(err).Str("cpf", cpf[:3]+"***").Msg("[OAUTH] Idoso nao encontrado")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Save tokens to database
	if err := h.db.SaveGoogleTokens(idoso.ID, token.RefreshToken, token.AccessToken, token.Expiry); err != nil {
		log.Error().Err(err).Msg("[OAUTH] Falha ao salvar tokens")
		http.Error(w, "Failed to save tokens", http.StatusInternalServerError)
		return
	}

	// Save Google email
	if email != "" {
		if err := h.db.SaveGoogleEmail(idoso.ID, email); err != nil {
			log.Warn().Err(err).Msg("[OAUTH] Falha ao salvar email Google")
		}
	}

	log.Info().Str("email", email).Int64("idoso_id", idoso.ID).Msg("[OAUTH] Google account linked")

	// Redirect to frontend
	frontendURL := h.frontendBaseURL + "?google=success"
	http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
}

// HandleTokenExchange receives auth code from mobile and returns tokens
func (h *Handler) HandleTokenExchange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code    string `json:"code"`
		IdosoID int64  `json:"idoso_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	token, err := h.service.ExchangeCode(ctx, req.Code)
	if err != nil {
		log.Error().Err(err).Msg("[OAUTH] Falha ao trocar code (mobile)")
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Save tokens to database
	if err := h.db.SaveGoogleTokens(req.IdosoID, token.RefreshToken, token.AccessToken, token.Expiry); err != nil {
		log.Error().Err(err).Msg("[OAUTH] Falha ao salvar tokens (mobile)")
		http.Error(w, "Failed to save tokens", http.StatusInternalServerError)
		return
	}

	// Save Google email
	email, err := h.service.GetUserInfo(ctx, token.AccessToken)
	if err == nil && email != "" {
		h.db.SaveGoogleEmail(req.IdosoID, email)
	}

	log.Info().Int64("idoso_id", req.IdosoID).Msg("[OAUTH] Tokens salvos (mobile)")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Google account linked successfully",
	})
}

// HandleGoogleStatus returns Google account connection status for a CPF
func (h *Handler) HandleGoogleStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cpf := vars["cpf"]

	status, err := h.db.GetGoogleStatusByCPF(cpf)
	if err != nil {
		log.Warn().Err(err).Str("cpf", cpf[:3]+"***").Msg("[OAUTH] Status nao encontrado")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"connected": false,
			"email":     "",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleGoogleDisconnect clears Google tokens for a CPF
func (h *Handler) HandleGoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cpf := vars["cpf"]

	idoso, err := h.db.GetIdosoByCPF(cpf)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := h.db.ClearGoogleTokens(idoso.ID); err != nil {
		log.Error().Err(err).Msg("[OAUTH] Falha ao desconectar Google")
		http.Error(w, "Failed to disconnect", http.StatusInternalServerError)
		return
	}

	log.Info().Str("cpf", cpf[:3]+"***").Msg("[OAUTH] Google desconectado")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Google account disconnected",
	})
}
