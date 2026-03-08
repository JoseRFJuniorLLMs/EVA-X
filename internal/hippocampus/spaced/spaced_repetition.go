// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package spaced

import (
	"context"
	"eva/internal/brainstem/database"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// SPACED REPETITION SERVICE - Consolidacao de Memoria para Idosos
// ============================================================================
// Baseado no algoritmo SM-2 (SuperMemo) adaptado para contexto de saude
// Intervalos: 1h -> 4h -> 1 dia -> 3 dias -> 1 semana -> 2 semanas -> 1 mes

const spacedLabel = "spaced_memory_items"

// SpacedRepetitionService gerencia reforcos de memoria
type SpacedRepetitionService struct {
	db         *database.DB
	notifyFunc func(idosoID int64, msgType string, payload interface{})
}

// MemoryItem representa um item a ser lembrado
type MemoryItem struct {
	ID              int64      `json:"id"`
	IdosoID         int64      `json:"idoso_id"`
	Content         string     `json:"content"`          // "Documento esta na gaveta do escritorio"
	Category        string     `json:"category"`         // location, medication, person, event, routine
	Trigger         string     `json:"trigger"`          // O que disparou (ex: "onde guardei o documento")
	Importance      int        `json:"importance"`       // 1-5 (5 = critico)
	RepetitionCount int        `json:"repetition_count"` // Quantas vezes foi reforcado
	EaseFactor      float64    `json:"ease_factor"`      // Fator de facilidade (SM-2)
	IntervalDays    float64    `json:"interval_days"`    // Intervalo atual em dias
	NextReview      time.Time  `json:"next_review"`      // Proximo reforco
	LastReview      *time.Time `json:"last_review"`      // Ultimo reforco
	SuccessCount    int        `json:"success_count"`    // Vezes que lembrou corretamente
	FailCount       int        `json:"fail_count"`       // Vezes que esqueceu
	Status          string     `json:"status"`           // active, paused, mastered, archived
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ReviewResult resultado de uma revisao
type ReviewResult struct {
	ItemID       int64   `json:"item_id"`
	Quality      int     `json:"quality"`       // 0-5 (0=esqueceu, 5=facil)
	ResponseTime float64 `json:"response_time"` // Tempo para responder (segundos)
	Remembered   bool    `json:"remembered"`    // Se lembrou ou nao
}

// Intervalos iniciais em horas (adaptados para idosos)
var initialIntervals = []float64{
	1,   // 1 hora
	4,   // 4 horas
	24,  // 1 dia
	72,  // 3 dias
	168, // 1 semana
	336, // 2 semanas
	720, // 1 mes
}

// NewSpacedRepetitionService cria novo servico
func NewSpacedRepetitionService(db *database.DB) *SpacedRepetitionService {
	if db == nil {
		log.Printf("[SPACED] NietzscheDB unavailable - running in degraded mode")
		return &SpacedRepetitionService{}
	}

	svc := &SpacedRepetitionService{
		db: db,
	}

	// NietzscheDB nao precisa de CREATE TABLE - apenas loga sucesso
	svc.createTable()

	// Iniciar goroutine para processar reforcos pendentes
	go svc.processRemindersLoop()

	return svc
}

// SetNotifyFunc configura funcao de notificacao
func (s *SpacedRepetitionService) SetNotifyFunc(fn func(idosoID int64, msgType string, payload interface{})) {
	s.notifyFunc = fn
}

// ============================================================================
// METODOS PUBLICOS
// ============================================================================

// CaptureMemory captura um novo item para reforco de memoria
func (s *SpacedRepetitionService) CaptureMemory(ctx context.Context, idosoID int64, content, category, trigger string, importance int) (*MemoryItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	if content == "" {
		return nil, fmt.Errorf("conteudo nao pode ser vazio")
	}

	// Normalizar categoria
	if category == "" {
		category = "general"
	}
	category = strings.ToLower(category)

	// Validar importancia
	if importance < 1 || importance > 5 {
		importance = 3 // media
	}

	// Calcular primeiro intervalo baseado na importancia
	// Items mais importantes = intervalos iniciais menores
	firstInterval := initialIntervals[0]
	if importance >= 4 {
		firstInterval = 0.5 // 30 minutos para itens criticos
	}

	now := time.Now()
	nextReview := now.Add(time.Duration(firstInterval * float64(time.Hour)))

	contentMap := map[string]interface{}{
		"idoso_id":         idosoID,
		"content":          content,
		"category":         category,
		"trigger_phrase":   trigger,
		"importance":       importance,
		"ease_factor":      2.5,
		"interval_hours":   firstInterval,
		"next_review":      nextReview.Format(time.RFC3339),
		"last_review":      nil,
		"repetition_count": 0,
		"success_count":    0,
		"fail_count":       0,
		"status":           "active",
		"created_at":       now.Format(time.RFC3339),
		"updated_at":       now.Format(time.RFC3339),
	}

	id, err := s.db.Insert(ctx, spacedLabel, contentMap)
	if err != nil {
		return nil, fmt.Errorf("erro ao capturar memoria: %w", err)
	}

	item := &MemoryItem{
		ID:           id,
		IdosoID:      idosoID,
		Content:      content,
		Category:     category,
		Trigger:      trigger,
		Importance:   importance,
		EaseFactor:   2.5,
		IntervalDays: firstInterval / 24,
		NextReview:   nextReview,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	log.Printf("[SPACED] Nova memoria capturada ID=%d: '%s' (proximo reforco em %.1fh)", item.ID, content, firstInterval)

	return item, nil
}

// RecordReview registra resultado de uma revisao
func (s *SpacedRepetitionService) RecordReview(ctx context.Context, itemID int64, quality int, remembered bool) (*MemoryItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	// Buscar item atual
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	// Aplicar algoritmo SM-2 adaptado
	item = s.calculateNextInterval(item, quality, remembered)

	// Atualizar no banco
	now := time.Now()
	successInc := 0
	failInc := 0
	if remembered {
		successInc = 1
	} else {
		failInc = 1
	}

	updates := map[string]interface{}{
		"ease_factor":      item.EaseFactor,
		"interval_hours":   item.IntervalDays * 24,
		"next_review":      item.NextReview.Format(time.RFC3339),
		"last_review":      now.Format(time.RFC3339),
		"repetition_count": item.RepetitionCount + 1,
		"success_count":    item.SuccessCount + successInc,
		"fail_count":       item.FailCount + failInc,
		"status":           item.Status,
		"updated_at":       now.Format(time.RFC3339),
	}

	err = s.db.Update(ctx, spacedLabel, map[string]interface{}{"id": float64(itemID)}, updates)
	if err != nil {
		return nil, fmt.Errorf("erro ao atualizar revisao: %w", err)
	}

	log.Printf("[SPACED] Revisao registrada ID=%d: quality=%d, remembered=%v, proximo=%.1f dias",
		itemID, quality, remembered, item.IntervalDays)

	return item, nil
}

// GetPendingReviews retorna itens pendentes de revisao
func (s *SpacedRepetitionService) GetPendingReviews(ctx context.Context, idosoID int64, limit int) ([]MemoryItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	if limit <= 0 {
		limit = 5
	}

	rows, err := s.db.QueryByLabel(ctx, spacedLabel,
		" AND n.idoso_id = $idoso_id AND n.status = $status",
		map[string]interface{}{
			"idoso_id": idosoID,
			"status":   "active",
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar revisoes: %w", err)
	}

	now := time.Now()
	var items []MemoryItem
	for _, m := range rows {
		item := contentToMemoryItem(m)
		// Filter: next_review <= now
		if !item.NextReview.After(now) {
			items = append(items, item)
		}
	}

	// Sort by importance DESC, next_review ASC
	sort.Slice(items, func(i, j int) bool {
		if items[i].Importance != items[j].Importance {
			return items[i].Importance > items[j].Importance
		}
		return items[i].NextReview.Before(items[j].NextReview)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// GetItem busca um item especifico
func (s *SpacedRepetitionService) GetItem(ctx context.Context, itemID int64) (*MemoryItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}

	m, err := s.db.GetNodeByID(ctx, spacedLabel, itemID)
	if err != nil {
		return nil, fmt.Errorf("item nao encontrado: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("item nao encontrado: id=%d", itemID)
	}

	item := contentToMemoryItem(m)
	return &item, nil
}

// GetStats retorna estatisticas de memoria do idoso
func (s *SpacedRepetitionService) GetStats(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	if s.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}

	rows, err := s.db.QueryByLabel(ctx, spacedLabel,
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	total := len(rows)
	active := 0
	mastered := 0
	pending := 0
	easeSum := 0.0
	successRateSum := 0.0
	successRateCount := 0

	for _, m := range rows {
		status := database.GetString(m, "status")
		switch status {
		case "active":
			active++
			nextReview := database.GetTime(m, "next_review")
			if !nextReview.After(now) {
				pending++
			}
		case "mastered":
			mastered++
		}

		easeSum += database.GetFloat64(m, "ease_factor")

		repCount := database.GetInt64(m, "repetition_count")
		if repCount > 0 {
			succCount := database.GetFloat64(m, "success_count")
			successRateSum += succCount / float64(repCount)
			successRateCount++
		}
	}

	avgEase := 2.5
	if total > 0 {
		avgEase = easeSum / float64(total)
	}

	avgSuccessRate := 0.0
	if successRateCount > 0 {
		avgSuccessRate = successRateSum / float64(successRateCount)
	}

	return map[string]interface{}{
		"total_items":      total,
		"active_items":     active,
		"mastered_items":   mastered,
		"pending_reviews":  pending,
		"avg_success_rate": avgSuccessRate,
		"avg_ease_factor":  avgEase,
	}, nil
}

// PauseItem pausa reforcos de um item
func (s *SpacedRepetitionService) PauseItem(ctx context.Context, itemID int64) error {
	if s.db == nil {
		return fmt.Errorf("NietzscheDB unavailable")
	}
	return s.db.Update(ctx, spacedLabel,
		map[string]interface{}{"id": float64(itemID)},
		map[string]interface{}{
			"status":     "paused",
			"updated_at": time.Now().Format(time.RFC3339),
		})
}

// ResumeItem retoma reforcos de um item
func (s *SpacedRepetitionService) ResumeItem(ctx context.Context, itemID int64) error {
	if s.db == nil {
		return fmt.Errorf("NietzscheDB unavailable")
	}
	now := time.Now()
	return s.db.Update(ctx, spacedLabel,
		map[string]interface{}{"id": float64(itemID)},
		map[string]interface{}{
			"status":      "active",
			"next_review": now.Format(time.RFC3339),
			"updated_at":  now.Format(time.RFC3339),
		})
}

// ============================================================================
// ALGORITMO SM-2 ADAPTADO
// ============================================================================

func (s *SpacedRepetitionService) calculateNextInterval(item *MemoryItem, quality int, remembered bool) *MemoryItem {
	// Quality: 0 = esqueceu completamente, 5 = lembrou facilmente
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	// Se esqueceu (quality < 3), resetar intervalo
	if quality < 3 || !remembered {
		// Voltar para intervalo inicial, mas nao menor que 30 min
		item.IntervalDays = initialIntervals[0] / 24 // 1 hora
		if item.Importance >= 4 {
			item.IntervalDays = 0.5 / 24 // 30 minutos para itens criticos
		}
		item.RepetitionCount = 0 // Reset repetition count

		// Diminuir ease factor
		item.EaseFactor = math.Max(1.3, item.EaseFactor-0.2)
	} else {
		// Lembrou corretamente
		if item.RepetitionCount == 0 {
			// Primeira repeticao bem-sucedida
			item.IntervalDays = initialIntervals[1] / 24 // 4 horas
		} else if item.RepetitionCount == 1 {
			// Segunda repeticao
			item.IntervalDays = 1.0 // 1 dia
		} else {
			// Aplicar SM-2
			item.IntervalDays = item.IntervalDays * item.EaseFactor
		}

		// Atualizar ease factor baseado na qualidade
		item.EaseFactor = item.EaseFactor + (0.1 - (5-float64(quality))*(0.08+(5-float64(quality))*0.02))
		if item.EaseFactor < 1.3 {
			item.EaseFactor = 1.3
		}

		item.RepetitionCount++
	}

	// Limitar intervalo maximo baseado na importancia
	maxInterval := 30.0 // 30 dias para itens normais
	if item.Importance >= 4 {
		maxInterval = 14.0 // 2 semanas para itens criticos
	}
	if item.IntervalDays > maxInterval {
		item.IntervalDays = maxInterval
	}

	// Calcular proxima revisao
	item.NextReview = time.Now().Add(time.Duration(item.IntervalDays * 24 * float64(time.Hour)))

	// Verificar se foi "dominado" (10+ revisoes com sucesso e intervalo > 2 semanas)
	if item.RepetitionCount >= 10 && item.IntervalDays >= 14 && item.SuccessCount > item.FailCount*3 {
		item.Status = "mastered"
	}

	return item
}

// ============================================================================
// PROCESSAMENTO AUTOMATICO DE LEMBRETES
// ============================================================================

func (s *SpacedRepetitionService) processRemindersLoop() {
	ticker := time.NewTicker(5 * time.Minute) // Verificar a cada 5 minutos
	defer ticker.Stop()

	for range ticker.C {
		s.sendPendingReminders()
	}
}

func (s *SpacedRepetitionService) sendPendingReminders() {
	if s.db == nil || s.notifyFunc == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Buscar todos os itens ativos
	rows, err := s.db.QueryByLabel(ctx, spacedLabel,
		" AND n.status = $status",
		map[string]interface{}{
			"status": "active",
		}, 0)
	if err != nil {
		log.Printf("[SPACED] Erro ao buscar itens pendentes: %v", err)
		return
	}

	// Filtrar por next_review <= now e coletar idoso_ids distintos
	now := time.Now()
	idosoSet := make(map[int64]bool)
	for _, m := range rows {
		nextReview := database.GetTime(m, "next_review")
		if !nextReview.After(now) {
			idosoID := database.GetInt64(m, "idoso_id")
			idosoSet[idosoID] = true
		}
	}

	for idosoID := range idosoSet {
		// Buscar itens pendentes para este idoso
		items, err := s.GetPendingReviews(ctx, idosoID, 3)
		if err != nil || len(items) == 0 {
			continue
		}

		// Enviar notificacao
		for _, item := range items {
			s.notifyFunc(idosoID, "memory_reinforcement", map[string]interface{}{
				"item_id":    item.ID,
				"content":    item.Content,
				"category":   item.Category,
				"importance": item.Importance,
				"message":    s.buildReminderMessage(item),
			})

			log.Printf("[SPACED] Reforco enviado para idoso %d: '%s'", idosoID, item.Content)
		}
	}
}

func (s *SpacedRepetitionService) buildReminderMessage(item MemoryItem) string {
	// Mensagens contextuais baseadas na categoria
	switch item.Category {
	case "location":
		return fmt.Sprintf("Lembra onde voce guardou? %s", item.Content)
	case "medication":
		return fmt.Sprintf("Importante lembrar: %s", item.Content)
	case "person":
		return fmt.Sprintf("Voce lembra? %s", item.Content)
	case "event":
		return fmt.Sprintf("Nao esqueca: %s", item.Content)
	case "routine":
		return fmt.Sprintf("Sua rotina: %s", item.Content)
	default:
		return fmt.Sprintf("Reforco de memoria: %s", item.Content)
	}
}

// ============================================================================
// CRIACAO DE TABELA (NO-OP para NietzscheDB)
// ============================================================================

func (s *SpacedRepetitionService) createTable() {
	log.Println("[SPACED] NietzscheDB label 'spaced_memory_items' ready (no CREATE TABLE needed)")
}

// ============================================================================
// HELPER: content map -> MemoryItem
// ============================================================================

func contentToMemoryItem(m map[string]interface{}) MemoryItem {
	intervalHours := database.GetFloat64(m, "interval_hours")
	return MemoryItem{
		ID:              database.GetInt64(m, "id"),
		IdosoID:         database.GetInt64(m, "idoso_id"),
		Content:         database.GetString(m, "content"),
		Category:        database.GetString(m, "category"),
		Trigger:         database.GetString(m, "trigger_phrase"),
		Importance:      int(database.GetInt64(m, "importance")),
		RepetitionCount: int(database.GetInt64(m, "repetition_count")),
		EaseFactor:      database.GetFloat64(m, "ease_factor"),
		IntervalDays:    intervalHours / 24,
		NextReview:      database.GetTime(m, "next_review"),
		LastReview:      database.GetTimePtr(m, "last_review"),
		SuccessCount:    int(database.GetInt64(m, "success_count")),
		FailCount:       int(database.GetInt64(m, "fail_count")),
		Status:          database.GetString(m, "status"),
		CreatedAt:       database.GetTime(m, "created_at"),
		UpdatedAt:       database.GetTime(m, "updated_at"),
	}
}
