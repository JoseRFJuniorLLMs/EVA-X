// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"eva/internal/brainstem/database"

	"firebase.google.com/go/v4/messaging"
)

// errNoDB is returned when a DeviceTokenManager method is called without a NietzscheDB connection.
var errNoDB = fmt.Errorf("DeviceTokenManager: database not configured (push notifications disabled)")

// DeviceTokenManager gerencia tokens de dispositivos para push notifications
type DeviceTokenManager struct {
	db          *database.DB
	pushService *FirebaseService
}

// NewDeviceTokenManager cria um novo gerenciador de tokens
func NewDeviceTokenManager(db *database.DB, pushService *FirebaseService) *DeviceTokenManager {
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
		log.Printf("Erro ao decodificar request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validar campos obrigatorios
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

	// Verificar se o token e valido com Firebase
	if !dtm.ValidateFirebaseToken(req.DeviceToken) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Invalid Firebase token",
		})
		return
	}

	// Buscar idoso por CPF via NietzscheDB
	idoso, err := dtm.db.GetIdosoByCPF(req.CPF)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "CPF not found",
		})
		return
	}

	idosoID := idoso.ID

	// Salvar token no banco
	tokenID, err := dtm.SaveDeviceToken(idosoID, req.DeviceToken, req.Platform, req.AppVersion, req.DeviceModel)
	if err != nil {
		log.Printf("Erro ao salvar token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(RegisterDeviceTokenResponse{
			Success: false,
			Message: "Failed to save device token",
		})
		return
	}

	log.Printf("Device token registrado: ID=%d, IdosoID=%d, Platform=%s", tokenID, idosoID, req.Platform)

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
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)

	// Verificar se token ja existe via NietzscheDB
	rows, err := dtm.db.QueryByLabel(ctx, "device_tokens",
		` AND n.idoso_id = $iid AND n.token = $tok`, map[string]interface{}{
			"iid": idosoID,
			"tok": token,
		}, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to check existing token: %w", err)
	}

	if len(rows) > 0 {
		// Token ja existe, atualizar
		existingID := database.GetInt64(rows[0], "id")
		err := dtm.db.Update(ctx, "device_tokens",
			map[string]interface{}{"id": float64(existingID)},
			map[string]interface{}{
				"last_used_at": now,
				"app_version":  appVersion,
				"device_model": deviceModel,
				"is_active":    true,
			})
		if err != nil {
			return 0, fmt.Errorf("failed to update token: %w", err)
		}

		log.Printf("Token atualizado: ID=%d", existingID)
		return existingID, nil
	}

	// Token nao existe, criar novo
	newID, err := dtm.db.Insert(ctx, "device_tokens", map[string]interface{}{
		"idoso_id":     idosoID,
		"token":        token,
		"platform":     platform,
		"app_version":  appVersion,
		"device_model": deviceModel,
		"is_active":    true,
		"created_at":   now,
		"last_used_at": now,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert token: %w", err)
	}

	log.Printf("Novo token criado: ID=%d", newID)
	return newID, nil
}

// GetDeviceTokens recupera todos os tokens ativos de um idoso
func (dtm *DeviceTokenManager) GetDeviceTokens(idosoID int64) ([]string, error) {
	if dtm.db == nil {
		return nil, errNoDB
	}
	ctx := context.Background()

	rows, err := dtm.db.QueryByLabel(ctx, "device_tokens",
		` AND n.idoso_id = $iid`, map[string]interface{}{
			"iid": idosoID,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}

	var tokens []string
	for _, m := range rows {
		if database.GetBool(m, "is_active") {
			tok := database.GetString(m, "token")
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
	}

	return tokens, nil
}

// GetDeviceTokensByCPF recupera tokens por CPF
func (dtm *DeviceTokenManager) GetDeviceTokensByCPF(cpf string) ([]string, error) {
	if dtm.db == nil {
		return nil, errNoDB
	}

	// First get idoso by CPF
	idoso, err := dtm.db.GetIdosoByCPF(cpf)
	if err != nil {
		return nil, fmt.Errorf("failed to find idoso by CPF: %w", err)
	}

	// Then query device_tokens by idoso_id
	return dtm.GetDeviceTokens(idoso.ID)
}

// ValidateFirebaseToken valida se o token e valido no Firebase
func (dtm *DeviceTokenManager) ValidateFirebaseToken(token string) bool {
	if dtm.pushService == nil || dtm.pushService.client == nil {
		log.Printf("Firebase client nao inicializado - rejeitando token (fail-closed, M9)")
		return false // M9: fail-closed -- reject tokens when Firebase is not configured
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
		log.Printf("Token invalido: %v", err)
		return false
	}

	return true
}

// DeactivateToken desativa um token (usuario fez logout, etc.)
func (dtm *DeviceTokenManager) DeactivateToken(token string) error {
	if dtm.db == nil {
		return errNoDB
	}
	ctx := context.Background()

	// Check that the token exists first
	rows, err := dtm.db.QueryByLabel(ctx, "device_tokens",
		` AND n.token = $tok`, map[string]interface{}{
			"tok": token,
		}, 1)
	if err != nil {
		return fmt.Errorf("failed to query token: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("token not found")
	}

	err = dtm.db.Update(ctx, "device_tokens",
		map[string]interface{}{"token": token},
		map[string]interface{}{"is_active": false})
	if err != nil {
		return fmt.Errorf("failed to deactivate token: %w", err)
	}

	log.Printf("Token desativado: %s", token)
	return nil
}

// CleanupExpiredTokens remove tokens que nao foram usados ha muito tempo
func (dtm *DeviceTokenManager) CleanupExpiredTokens(ctx context.Context) {
	if dtm.db == nil {
		log.Printf("Token cleanup disabled (database not configured)")
		return
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Token cleanup scheduler stopped")
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
	ctx := context.Background()
	now := time.Now()
	cutoff90 := now.AddDate(0, 0, -90)
	cutoff180 := now.AddDate(0, 0, -180)

	// Fetch all device_tokens
	rows, err := dtm.db.QueryByLabel(ctx, "device_tokens", "", nil, 0)
	if err != nil {
		log.Printf("Erro ao buscar tokens para cleanup: %v", err)
		return
	}

	var deactivated, deleted int

	for _, m := range rows {
		lastUsed := database.GetTime(m, "last_used_at")
		isActive := database.GetBool(m, "is_active")
		tokenID := database.GetInt64(m, "id")

		// Desativar tokens nao usados ha mais de 90 dias
		if isActive && !lastUsed.IsZero() && lastUsed.Before(cutoff90) {
			err := dtm.db.Update(ctx, "device_tokens",
				map[string]interface{}{"id": float64(tokenID)},
				map[string]interface{}{"is_active": false})
			if err != nil {
				log.Printf("Erro ao desativar token ID=%d: %v", tokenID, err)
				continue
			}
			deactivated++
		}

		// Soft-delete tokens desativados ha mais de 180 dias
		if !isActive && !lastUsed.IsZero() && lastUsed.Before(cutoff180) {
			err := dtm.db.SoftDelete(ctx, "device_tokens",
				map[string]interface{}{"id": float64(tokenID)})
			if err != nil {
				log.Printf("Erro ao deletar token ID=%d: %v", tokenID, err)
				continue
			}
			deleted++
		}
	}

	if deactivated > 0 {
		log.Printf("Tokens expirados desativados: %d", deactivated)
	}
	if deleted > 0 {
		log.Printf("Tokens antigos deletados (soft): %d", deleted)
	}
}

// SendTestNotification envia uma notificacao de teste
func (dtm *DeviceTokenManager) SendTestNotification(token string) error {
	if dtm.pushService == nil || dtm.pushService.client == nil {
		return fmt.Errorf("Firebase client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: "EVA - Teste de Notificacao",
			Body:  "Seu dispositivo esta configurado corretamente!",
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

	log.Printf("Notificacao de teste enviada: %s", messageID)
	return nil
}
