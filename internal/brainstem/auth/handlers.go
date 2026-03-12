// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"encoding/json"
	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	"net/http"
)

type Handler struct {
	DB     *database.DB
	Config *config.Config
}

func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{DB: db, Config: cfg}
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Nome     string `json:"nome"`     // Compatibilidade com frontend
	Email    string `json:"email"`
	Password string `json:"password"`
	Senha    string `json:"senha"`    // Compatibilidade com frontend
	CPF      string `json:"cpf"`      // AUDITORIA FIX 2026-03-12: CPF para criar Idoso
	Role     string `json:"role"`
	Tipo     string `json:"tipo"`     // Compatibilidade com frontend
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Compatibilidade: frontend envia "nome" e "senha" além de "name" e "password"
	if req.Name == "" && req.Nome != "" {
		req.Name = req.Nome
	}
	if req.Password == "" && req.Senha != "" {
		req.Password = req.Senha
	}
	if req.Role == "" && req.Tipo != "" {
		req.Role = req.Tipo
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPwd, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Default role if empty
	if req.Role == "" {
		req.Role = "operator"
	}

	err = h.DB.CreateUser(req.Name, req.Email, hashedPwd, req.Role)
	if err != nil {
		http.Error(w, "Failed to create user (email might be taken)", http.StatusConflict)
		return
	}

	// AUDITORIA FIX 2026-03-12: Se CPF fornecido, criar também o registo de Idoso.
	// Antes, o Register só criava User (tabela usuarios) e ignorava o CPF.
	// Sem Idoso, o EVA não consegue buscar nome, medicamentos, persona, etc.
	var idosoID int64
	if req.CPF != "" {
		// Verificar se já existe
		existing, _ := h.DB.GetIdosoByCPF(req.CPF)
		if existing == nil {
			idoso, err := h.DB.CreateIdoso(req.Name, req.CPF, "")
			if err != nil {
				// Log but don't fail registration — User was already created
				// log.Printf("⚠️ [Register] Failed to create Idoso for CPF: %v", err)
			} else {
				idosoID = idoso.ID
			}
		} else {
			idosoID = existing.ID
		}
	}

	resp := map[string]interface{}{
		"message": "User registered successfully",
	}
	if idosoID > 0 {
		resp["idoso_id"] = idosoID
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.DB.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := GenerateToken(user.ID, user.Role, h.Config.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := GenerateRefreshToken(user.ID, h.Config.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	// Update last login
	h.DB.UpdateLastLogin(user.ID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":         token,
		"refresh_token": refreshToken,
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (set by middleware)
	claims, ok := r.Context().Value(UserContextKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.DB.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenHandler renova o access token usando o refresh token
func (h *Handler) RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		http.Error(w, "Refresh token is required", http.StatusBadRequest)
		return
	}

	// Validar refresh token
	claims, err := ValidateRefreshToken(req.RefreshToken, h.Config.JWTSecret)
	if err != nil {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Buscar usuário
	user, err := h.DB.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Gerar novo access token
	newToken, err := GenerateToken(user.ID, user.Role, h.Config.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Gerar novo refresh token (optional - rotation)
	newRefreshToken, err := GenerateRefreshToken(user.ID, h.Config.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":         newToken,
		"refresh_token": newRefreshToken,
	})
}
