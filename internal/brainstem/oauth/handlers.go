package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"eva-mind/internal/brainstem/database"
	"log"
	"net/http"
)

type Handler struct {
	service *Service
	db      *database.DB
}

func NewHandler(service *Service, db *database.DB) *Handler {
	return &Handler{
		service: service,
		db:      db,
	}
}

// HandleAuthorize redirects user to Google OAuth consent screen
func (h *Handler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	// Generate random state for CSRF protection
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Store state in session/cookie (simplified here)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.service.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback processes OAuth callback from Google
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
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
		log.Printf("Failed to exchange code: %v", err)
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Get user info
	email, err := h.service.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
	}

	log.Printf("✅ OAuth successful for %s", email)

	// Return tokens to client (they will send to mobile which will send to backend)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expiry":        token.Expiry,
		"email":         email,
	})
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
		log.Printf("Failed to exchange code: %v", err)
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Save tokens to database
	err = h.db.SaveGoogleTokens(req.IdosoID, token.RefreshToken, token.AccessToken, token.Expiry)
	if err != nil {
		log.Printf("Failed to save tokens: %v", err)
		http.Error(w, "Failed to save tokens", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Tokens saved for idoso %d", req.IdosoID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Google account linked successfully",
	})
}
