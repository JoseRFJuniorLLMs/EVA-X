// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package thinking

// Integration helper for EVA-Mind websocket handler
// This file provides the integration point between Thinking Mode and the main websocket handler

import (
	"context"
	"database/sql"
	"log"

	"eva/internal/brainstem/push"
)

// HealthTriageService orquestra análise de saúde com Thinking Mode
type HealthTriageService struct {
	thinkingClient      *ThinkingClient
	auditLogger         *AuditLogger
	notificationService *NotificationService
}

// NewHealthTriageService cria um novo serviço de triagem de saúde
func NewHealthTriageService(apiKey string, db *sql.DB, pushService *push.FirebaseService) (*HealthTriageService, error) {
	thinkingClient, err := NewThinkingClient(apiKey)
	if err != nil {
		return nil, err
	}

	return &HealthTriageService{
		thinkingClient:      thinkingClient,
		auditLogger:         NewAuditLogger(db),
		notificationService: NewNotificationService(db, pushService),
	}, nil
}

// ProcessHealthConcern processa uma preocupação de saúde end-to-end
func (hts *HealthTriageService) ProcessHealthConcern(ctx context.Context, idosoID int64, message string, patientContext string) (string, error) {
	// 1. Verificar se é preocupação de saúde
	if !IsHealthConcern(message) {
		return "", nil // Não é preocupação de saúde, retornar vazio
	}

	log.Printf("🏥 Preocupação de saúde detectada para idoso %d", idosoID)

	// 2. Analisar com Thinking Mode
	response, err := hts.thinkingClient.AnalyzeHealthConcern(ctx, message, patientContext)
	if err != nil {
		log.Printf("❌ Erro ao analisar preocupação: %v", err)
		return "", err
	}

	log.Printf("✅ Análise completa - Risco: %s, Urgência: %s", response.RiskLevel.String(), response.UrgencyLevel)

	// 3. Salvar auditoria
	auditID, err := hts.auditLogger.LogHealthAnalysis(ctx, idosoID, message, response)
	if err != nil {
		log.Printf("⚠️ Erro ao salvar auditoria: %v", err)
		// Continuar mesmo com erro de auditoria
	}

	// 4. Notificar cuidador se risco alto ou crítico
	if response.RiskLevel >= RiskHigh {
		go func() {
			err := hts.notificationService.NotifyCaregiver(context.Background(), idosoID, auditID, message, response.RiskLevel)
			if err != nil {
				log.Printf("⚠️ Erro ao notificar cuidador: %v", err)
			}
		}()
	}

	// 5. Adicionar disclaimer médico à resposta
	finalAnswer := addMedicalDisclaimer(response.FinalAnswer, response.RiskLevel)

	return finalAnswer, nil
}

// ShouldUseThinkingMode determina se deve usar Thinking Mode para uma mensagem
func (hts *HealthTriageService) ShouldUseThinkingMode(message string) bool {
	return IsHealthConcern(message)
}

// addMedicalDisclaimer adiciona disclaimer apropriado à resposta
func addMedicalDisclaimer(answer string, riskLevel RiskLevel) string {
	var disclaimer string

	switch riskLevel {
	case RiskCritical:
		disclaimer = "\n\n🚨 **ATENÇÃO**: Esta é uma situação que pode requerer atendimento médico IMEDIATO. Estou notificando seu cuidador agora. Se os sintomas piorarem, procure o pronto-socorro sem demora."
	case RiskHigh:
		disclaimer = "\n\n⚠️ **IMPORTANTE**: Recomendo fortemente que você consulte um médico nas próximas 24 horas. Estou notificando seu cuidador sobre esta preocupação."
	case RiskMedium:
		disclaimer = "\n\n💡 **LEMBRETE**: Sou uma assistente virtual e não substituo um profissional de saúde. Se os sintomas persistirem ou piorarem, consulte seu médico."
	default:
		disclaimer = "\n\n📋 **NOTA**: Esta é apenas uma orientação geral. Para diagnóstico e tratamento adequados, sempre consulte um profissional de saúde."
	}

	return answer + disclaimer
}

// Close fecha os recursos do serviço
func (hts *HealthTriageService) Close() error {
	return hts.thinkingClient.Close()
}
