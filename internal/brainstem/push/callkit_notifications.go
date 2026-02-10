package push

import (
	"context"
	"fmt"
	"log"
	"time"

	"firebase.google.com/go/v4/messaging"
)

// CallKitNotification representa uma notificação de chamada VoIP para iOS
type CallKitNotification struct {
	CallerName   string
	CallType     string // "audio" ou "video"
	SessionID    string
	IdosoID      int64
	CuidadorName string
	Priority     string // "normal", "urgent", "emergency"
	Timestamp    time.Time
}

// SendCallKitNotification envia notificação CallKit para iOS via Firebase
func (fs *FirebaseService) SendCallKitNotification(
	ctx context.Context,
	token string,
	notification *CallKitNotification,
) error {
	if fs.client == nil {
		return fmt.Errorf("Firebase client not initialized")
	}

	// Para iOS, usar APNs com payload específico para CallKit
	// Referência: https://developer.apple.com/documentation/pushkit/supporting_pushkit_notifications_in_your_app

	message := &messaging.Message{
		Token: token,
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority":    "10", // Alta prioridade
				"apns-push-type":   "voip", // Push tipo VoIP
				"apns-expiration":  fmt.Sprintf("%d", time.Now().Add(60*time.Second).Unix()),
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
					Sound:            "default",
					MutableContent:   true,
				},
				// Dados customizados para CallKit
				CustomData: map[string]interface{}{
					"type":           "voip_call",
					"call_type":      notification.CallType,
					"caller_name":    notification.CallerName,
					"session_id":     notification.SessionID,
					"idoso_id":       notification.IdosoID,
					"cuidador_name":  notification.CuidadorName,
					"priority":       notification.Priority,
					"timestamp":      notification.Timestamp.Unix(),

					// Dados específicos CallKit
					"callkit": map[string]interface{}{
						"handle":      notification.CallerName,
						"handle_type": "generic",
						"has_video":   notification.CallType == "video",
						"supportsGrouping": false,
						"supportsUngrouping": false,
						"supportsHolding": true,
						"supportsDTMF": false,
					},
				},
			},
		},
		// Fallback para Android (não usa CallKit)
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Title:     fmt.Sprintf("📞 Chamada de %s", notification.CallerName),
				Body:      "Toque para atender",
				ChannelID: "voip_calls",
				Priority:  messaging.PriorityHigh,
				Sound:     "ringtone",
				Tag:       notification.SessionID, // Para substituir notificações antigas
			},
			Data: map[string]string{
				"type":           "voip_call",
				"call_type":      notification.CallType,
				"caller_name":    notification.CallerName,
				"session_id":     notification.SessionID,
				"idoso_id":       fmt.Sprintf("%d", notification.IdosoID),
				"cuidador_name":  notification.CuidadorName,
				"priority":       notification.Priority,
			},
		},
		// Dados genéricos (acessíveis em ambas plataformas)
		Data: map[string]string{
			"action":      "incoming_call",
			"call_type":   notification.CallType,
			"session_id":  notification.SessionID,
			"caller_name": notification.CallerName,
			"priority":    notification.Priority,
		},
	}

	// Enviar notificação
	messageID, err := fs.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send CallKit notification: %w", err)
	}

	log.Printf("✅ CallKit notification sent: %s (Session: %s, Type: %s)",
		messageID, notification.SessionID, notification.CallType)

	return nil
}

// SendCallEndedNotification envia notificação que a chamada terminou
func (fs *FirebaseService) SendCallEndedNotification(
	ctx context.Context,
	token string,
	sessionID string,
	reason string,
) error {
	if fs.client == nil {
		return fmt.Errorf("Firebase client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				CustomData: map[string]interface{}{
					"type":       "call_ended",
					"session_id": sessionID,
					"reason":     reason,
				},
			},
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Data: map[string]string{
				"type":       "call_ended",
				"session_id": sessionID,
				"reason":     reason,
			},
		},
		Data: map[string]string{
			"action":     "call_ended",
			"session_id": sessionID,
			"reason":     reason,
		},
	}

	messageID, err := fs.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send call ended notification: %w", err)
	}

	log.Printf("✅ Call ended notification sent: %s (Session: %s)", messageID, sessionID)
	return nil
}

// SendCallAnsweredNotification notifica que chamada foi atendida
func (fs *FirebaseService) SendCallAnsweredNotification(
	ctx context.Context,
	token string,
	sessionID string,
	answeredBy string,
) error {
	if fs.client == nil {
		return fmt.Errorf("Firebase client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"action":      "call_answered",
			"session_id":  sessionID,
			"answered_by": answeredBy,
		},
	}

	messageID, err := fs.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send call answered notification: %w", err)
	}

	log.Printf("✅ Call answered notification sent: %s (Session: %s, By: %s)",
		messageID, sessionID, answeredBy)

	return nil
}

// SendCallKitToMultipleDevices envia CallKit para múltiplos dispositivos
func (fs *FirebaseService) SendCallKitToMultipleDevices(
	ctx context.Context,
	tokens []string,
	notification *CallKitNotification,
) error {
	if len(tokens) == 0 {
		return fmt.Errorf("no tokens provided")
	}

	log.Printf("📱 Enviando CallKit para %d dispositivos (Session: %s)",
		len(tokens), notification.SessionID)

	var lastError error
	successCount := 0

	for _, token := range tokens {
		err := fs.SendCallKitNotification(ctx, token, notification)
		if err != nil {
			log.Printf("⚠️ Falha ao enviar para token %s: %v", token[:10]+"...", err)
			lastError = err
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("failed to send to any device: %w", lastError)
	}

	log.Printf("✅ CallKit enviado com sucesso para %d/%d dispositivos",
		successCount, len(tokens))

	return nil
}

// ValidatePushKitToken valida um token PushKit (iOS VoIP)
func (fs *FirebaseService) ValidatePushKitToken(ctx context.Context, token string) (bool, error) {
	// Criar mensagem de teste seca
	message := &messaging.Message{
		Token: token,
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-push-type": "voip",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
				CustomData: map[string]interface{}{
					"test": true,
				},
			},
		},
	}

	// Usar dry-run para validar
	_, err := fs.client.SendDryRun(ctx, message)
	if err != nil {
		log.Printf("❌ Token PushKit inválido: %v", err)
		return false, err
	}

	return true, nil
}
