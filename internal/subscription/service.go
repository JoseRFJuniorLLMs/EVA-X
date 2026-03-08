// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package subscription

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// PlanFeatures define as features disponíveis por plano
var PlanFeatures = map[string]map[string]bool{
	"livre": {
		"interface_acessivel":      true,
		"cadastro_idoso":           true,
		"historico_chamadas":       true,
		"ligar_agora_manual":       true,
		"lembretes_automaticos":    false,
		"confirmacao_medicacao":    false,
		"personalizacao_audio":     false,
		"alertas_nao_atendeu":      false,
		"deteccao_emergencias":     false,
		"monitoramento_tempo_real": false,
		"relatorios_detalhados":    false,
		"ia_avancada":              false,
		"api_integracao":           false,
		"suporte_prioritario":      false,
	},
	"essencial": {
		"interface_acessivel":      true,
		"cadastro_idoso":           true,
		"historico_chamadas":       true,
		"ligar_agora_manual":       true,
		"lembretes_automaticos":    true,
		"confirmacao_medicacao":    true,
		"personalizacao_audio":     true,
		"alertas_nao_atendeu":      true,
		"deteccao_emergencias":     false,
		"monitoramento_tempo_real": false,
		"relatorios_detalhados":    false,
		"ia_avancada":              false,
		"api_integracao":           false,
		"suporte_prioritario":      false,
	},
	"familia_plus": {
		"interface_acessivel":      true,
		"cadastro_idoso":           true,
		"historico_chamadas":       true,
		"ligar_agora_manual":       true,
		"lembretes_automaticos":    true,
		"confirmacao_medicacao":    true,
		"personalizacao_audio":     true,
		"alertas_nao_atendeu":      true,
		"deteccao_emergencias":     true,
		"monitoramento_tempo_real": true,
		"relatorios_detalhados":    true,
		"ia_avancada":              true,
		"api_integracao":           false,
		"suporte_prioritario":      false,
	},
	"profissional": {
		"interface_acessivel":      true,
		"cadastro_idoso":           true,
		"historico_chamadas":       true,
		"ligar_agora_manual":       true,
		"lembretes_automaticos":    true,
		"confirmacao_medicacao":    true,
		"personalizacao_audio":     true,
		"alertas_nao_atendeu":      true,
		"deteccao_emergencias":     true,
		"monitoramento_tempo_real": true,
		"relatorios_detalhados":    true,
		"ia_avancada":              true,
		"idosos_ilimitados":        true,
		"integracao_sensores":      true,
		"lembretes_consultas":      true,
		"hipaa_ready":              true,
		"api_integracao":           true,
		"suporte_prioritario":      true,
	},
}

// Subscription representa uma assinatura na tabela assinaturas_entidade
type Subscription struct {
	ID              int
	EntityName      string
	Status          string
	PlanID          string
	NextBillingDate *time.Time
	MinutesLimit    int
	MinutesConsumed int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SubscriptionService gerencia operações de assinaturas
type SubscriptionService struct {
	db *database.DB
}

// NewSubscriptionService cria uma nova instância do serviço
func NewSubscriptionService(db *database.DB) *SubscriptionService {
	return &SubscriptionService{db: db}
}

// GetActiveSubscription busca a assinatura ativa de uma entidade
func (s *SubscriptionService) GetActiveSubscription(entityName string) (*Subscription, error) {
	ctx := context.Background()
	rows, err := s.db.QueryByLabel(ctx, "assinaturas_entidade",
		" AND n.entidade_nome = $entity AND n.status = $status",
		map[string]interface{}{"entity": entityName, "status": "ativo"}, 1)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar assinatura: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("assinatura nao encontrada para entidade: %s", entityName)
	}

	m := rows[0]
	sub := &Subscription{
		ID:              int(database.GetInt64(m, "id")),
		EntityName:      database.GetString(m, "entidade_nome"),
		Status:          database.GetString(m, "status"),
		PlanID:          database.GetString(m, "plano_id"),
		NextBillingDate: database.GetTimePtr(m, "data_proxima_cobranca"),
		MinutesLimit:    int(database.GetInt64(m, "limite_minutos")),
		MinutesConsumed: int(database.GetInt64(m, "minutos_consumidos")),
		CreatedAt:       database.GetTime(m, "criado_em"),
		UpdatedAt:       database.GetTime(m, "atualizado_em"),
	}

	return sub, nil
}

// CheckFeature verifica se uma entidade tem acesso a uma feature específica
func (s *SubscriptionService) CheckFeature(entityName, feature string) (bool, error) {
	sub, err := s.GetActiveSubscription(entityName)
	if err != nil {
		return false, err
	}

	features, exists := PlanFeatures[sub.PlanID]
	if !exists {
		log.Printf("Plano desconhecido: %s", sub.PlanID)
		return false, fmt.Errorf("plano desconhecido: %s", sub.PlanID)
	}

	hasFeature, exists := features[feature]
	if !exists {
		log.Printf("Feature desconhecida: %s", feature)
		return false, nil
	}

	return hasFeature, nil
}

// GetPlanFeatures retorna todas as features de um plano
func (s *SubscriptionService) GetPlanFeatures(planID string) (map[string]bool, error) {
	features, exists := PlanFeatures[planID]
	if !exists {
		return nil, fmt.Errorf("plano desconhecido: %s", planID)
	}

	return features, nil
}

// ConsumeMinutes registra o consumo de minutos
func (s *SubscriptionService) ConsumeMinutes(entityName string, minutes int) error {
	ctx := context.Background()
	sub, err := s.GetActiveSubscription(entityName)
	if err != nil {
		return err
	}

	newConsumed := sub.MinutesConsumed + minutes
	if newConsumed > sub.MinutesLimit {
		return fmt.Errorf("limite de minutos excedido: %d/%d", newConsumed, sub.MinutesLimit)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err = s.db.Update(ctx, "assinaturas_entidade",
		map[string]interface{}{"id": int64(sub.ID)},
		map[string]interface{}{
			"minutos_consumidos": newConsumed,
			"atualizado_em":      now,
		})
	if err != nil {
		return fmt.Errorf("erro ao atualizar consumo: %w", err)
	}

	log.Printf("Consumo registrado: %s - %d minutos (total: %d/%d)",
		entityName, minutes, newConsumed, sub.MinutesLimit)

	return nil
}

// GetUsageStats retorna estatísticas de uso
func (s *SubscriptionService) GetUsageStats(entityName string) (consumed, limit int, err error) {
	sub, err := s.GetActiveSubscription(entityName)
	if err != nil {
		return 0, 0, err
	}

	return sub.MinutesConsumed, sub.MinutesLimit, nil
}

// GetSubscriptionByID busca assinatura por ID
func (s *SubscriptionService) GetSubscriptionByID(id int) (*Subscription, error) {
	ctx := context.Background()
	m, err := s.db.GetNodeByID(ctx, "assinaturas_entidade", id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar assinatura: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("assinatura nao encontrada: ID %d", id)
	}

	sub := &Subscription{
		ID:              int(database.GetInt64(m, "id")),
		EntityName:      database.GetString(m, "entidade_nome"),
		Status:          database.GetString(m, "status"),
		PlanID:          database.GetString(m, "plano_id"),
		NextBillingDate: database.GetTimePtr(m, "data_proxima_cobranca"),
		MinutesLimit:    int(database.GetInt64(m, "limite_minutos")),
		MinutesConsumed: int(database.GetInt64(m, "minutos_consumidos")),
		CreatedAt:       database.GetTime(m, "criado_em"),
		UpdatedAt:       database.GetTime(m, "atualizado_em"),
	}

	return sub, nil
}
