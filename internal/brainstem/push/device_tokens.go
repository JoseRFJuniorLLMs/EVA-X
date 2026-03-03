// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/v4/messaging"
)

// errNoDB is returned when a DeviceTokenManager method is called without a PostgreSQL connection.
var errNoDB = fmt.Errorf("DeviceTokenManager: database not configured (push notifications disabled)")

// DeviceTokenManager gerencia tokens de dispositivos para push notifications
type DeviceTokenManager struct {
	db          *sql.DB
	pushService *FirebaseService
}

// NewDeviceTokenManager cria um novo gerenciador de tokens
func NewDeviceTokenManager(db *sql.DB, pushService *FirebaseService) *DeviceTokenManager {
	return &DeviceTokenManager{
		db:          db,
		pushService: pushService,
	}
}

// RegisterDeviceTokenRequest request para registro de token
type RegisterDeviceTokenRequest struct {
	CPF         string `json:"cpf"`
	DeviceToken string `json:"device_token"`
	Platform    string `json:"platform"` // "ios" ou "android"
	AppVersion  string `json:"app_version,omitempty"`
	DeviceModel string `json:"device_model,omitempty"`
}

// RegisterDeviceTokenResponse resposta do registro
type RegisterDeviceTokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TokenID int64  `json:"token_id,omitempty"`
}

// HandleRegisterDeviceToken endpoint HTTP para registro de token
func (dtm *DeviceTokenManager) HandleRegisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	if dtm.db == nil {
		http.Error(w, "Push notifications not available (database not configured)", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterDeviceTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Erro ao decodificar request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validar campos obrigatórios
	if req.CPF == "" || req.DeviceToken == "" || req.Platform == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "CPF, device_token, and platform are required",
		})
		return
	}

	// Validar plataforma
	if req.Platform != "ios" && req.Platform != "android" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Platform must be 'ios' or 'android'",
		})
		return
	}

	// Verificar se o token é válido com Firebase
	if !dtm.ValidateFirebaseToken(req.DeviceToken) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Invalid Firebase token",
		})
		return
	}

	// Buscar idoso por CPF
	var idosoID int64
	err := dtm.db.QueryRow(`
		SELECT id FROM idosos WHERE cpf = $1
	`, req.CPF).Scan(&idosoID)

	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
				Success: false,
				Message: "CPF not found",
			})
			return
		}

		log.Printf("❌ Erro ao buscar idoso: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Internal server error",
		})
		return
	}

	// Salvar token no banco
	tokenID, err := dtm.SaveDeviceToken(idosoID, req.DeviceToken, req.Platform, req.AppVersion, req.DeviceModel)
	if err != nil {
		log.Printf("❌ Erro ao salvar token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Failed to save device token",
		})
		return
	}

	log.Printf("✅ Device token registrado: ID=%d, IdosoID=%d, Platform=%s", tokenID, idosoID, req.Platform)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
		Success: true,
		Message: "Device token registered successfully",
		TokenID: tokenID,
	})
}

// SaveDeviceToken salva ou atualiza um token no banco de dados
func (dtm *DeviceTokenManager) SaveDeviceToken(
	idosoID int64,
	token string,
	platform string,
	appVersion string,
	deviceModel string,
) (int64, error) {
	if dtm.db == nil {
		return 0, errNoDB
	}
	// Verificar se token já existe
	var existingID int64
	err := dtm.db.QueryRow(`
		SELECT id FROM device_tokens
		WHERE idoso_id = $1 AND token = $2
	`, idosoID, token).Scan(&existingID)

	if err == nil {
		// Token já existe, atualizar
		_, err := dtm.db.Exec(`
			UPDATE device_tokens
			SET last_used_at = NOW(),
			    app_version = $1,
			    device_model = $2,
			    is_active = true
			WHERE id = $3
		`, appVersion, deviceModel, existingID)

		if err != nil {
			return 0, fmt.Errorf("failed to update token: %w", err)
		}

		log.Printf("🔄 Token atualizado: ID=%d", existingID)
		return existingID, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to check existing token: %w", err)
	}

	// Token não existe, criar novo
	var newID int64
	err = dtm.db.QueryRow(`
		INSERT INTO device_tokens (idoso_id, token, platform, app_version, device_model, is_active, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())
		RETURNING id
	`, idosoID, token, platform, appVersion, deviceModel).Scan(&newID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert token: %w", err)
	}

	log.Printf("➕ Novo token criado: ID=%d", newID)
	return newID, nil
}

// GetDeviceTokens recupera todos os tokens ativos de um idoso
func (dtm *DeviceTokenManager) GetDeviceTokens(idosoID int64) ([]string, error) {
	if dtm.db == nil {
		return nil, errNoDB
	}
	rows, err := dtm.db.Query(`
		SELECT token
		FROM device_tokens
		WHERE idoso_id = $1 AND is_active = true
		ORDER BY last_used_at DESC
	`, idosoID)

	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			log.Printf("⚠️ Erro ao escanear token: %v", err)
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// GetDeviceTokensByCPF recupera tokens por CPF
func (dtm *DeviceTokenManager) GetDeviceTokensByCPF(cpf string) ([]string, error) {
	if dtm.db == nil {
		return nil, errNoDB
	}
	rows, err := dtm.db.Query(`
		SELECT dt.token
		FROM device_tokens dt
		INNER JOIN idosos i ON i.id = dt.idoso_id
		WHERE i.cpf = $1 AND dt.is_active = true
		ORDER BY dt.last_used_at DESC
	`, cpf)

	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			log.Printf("⚠️ Erro ao escanear token: %v", err)
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// ValidateFirebaseToken valida se o token é válido no Firebase
func (dtm *DeviceTokenManager) ValidateFirebaseToken(token string) bool {
	if dtm.pushService == nil || dtm.pushService.client == nil {
		log.Printf("⚠️ Firebase client não inicializado — rejeitando token (fail-closed, M9)")
		return false // M9: fail-closed — reject tokens when Firebase is not configured
	}

	// Criar mensagem de teste seca (dry-run)
	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "Test",
			Body:  "Test notification",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Usar dry-run para validar sem enviar
	_, err := dtm.pushService.client.SendDryRun(ctx, message)

	if err != nil {
		log.Printf("❌ Token inválido: %v", err)
		return false
	}

	return true
}

// DeactivateToken desativa um token (usuário fez logout, etc.)
func (dtm *DeviceTokenManager) DeactivateToken(token string) error {
	if dtm.db == nil {
		return errNoDB
	}
	result, err := dtm.db.Exec(`
		UPDATE device_tokens
		SET is_active = false
		WHERE token = $1
	`, token)

	if err != nil {
		return fmt.Errorf("failed to deactivate token: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("token not found")
	}

	log.Printf("🔕 Token desativado: %s", token)
	return nil
}

// CleanupExpiredTokens remove tokens que não foram usados há muito tempo
func (dtm *DeviceTokenManager) CleanupExpiredTokens(ctx context.Context) {
	if dtm.db == nil {
		log.Printf("⚠️ Token cleanup disabled (database not configured)")
		return
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Token cleanup scheduler stopped")
			return
		case <-ticker.C:
			dtm.performCleanup()
		}
	}
}

func (dtm *DeviceTokenManager) performCleanup() {
	if dtm.db == nil {
		return
	}
	// Desativar tokens não usados há mais de 90 dias
	result, err := dtm.db.Exec(`
		UPDATE device_tokens
		SET is_active = false
		WHERE last_used_at < NOW() - INTERVAL '90 days'
		  AND is_active = true
	`)

	if err != nil {
		log.Printf("❌ Erro ao limpar tokens expirados: %v", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("🧹 Tokens expirados desativados: %d", rowsAffected)
	}

	// Deletar tokens desativados há mais de 180 dias
	result, err = dtm.db.Exec(`
		DELETE FROM device_tokens
		WHERE last_used_at < NOW() - INTERVAL '180 days'
		  AND is_active = false
	`)

	if err != nil {
		log.Printf("❌ Erro ao deletar tokens antigos: %v", err)
		return
	}

	rowsAffected, _ = result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("🗑️ Tokens antigos deletados: %d", rowsAffected)
	}
}

// SendTestNotification envia uma notificação de teste
func (dtm *DeviceTokenManager) SendTestNotification(token string) error {
	if dtm.pushService == nil || dtm.pushService.client == nil {
		return fmt.Errorf("Firebase client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "🔔 EVA - Teste de Notificação",
			Body:  "Seu dispositivo está configurado corretamente!",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "test_notifications",
				Sound:     "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageID, err := dtm.pushService.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send test notification: %w", err)
	}

	log.Printf("✅ Notificação de teste enviada: %s", messageID)
	return nil
}
