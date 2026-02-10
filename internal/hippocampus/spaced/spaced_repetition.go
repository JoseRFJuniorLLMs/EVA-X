package spaced

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// ============================================================================
// SPACED REPETITION SERVICE - Consolidação de Memória para Idosos
// ============================================================================
// Baseado no algoritmo SM-2 (SuperMemo) adaptado para contexto de saúde
// Intervalos: 1h -> 4h -> 1 dia -> 3 dias -> 1 semana -> 2 semanas -> 1 mês

// SpacedRepetitionService gerencia reforços de memória
type SpacedRepetitionService struct {
	db         *sql.DB
	notifyFunc func(idosoID int64, msgType string, payload interface{})
}

// MemoryItem representa um item a ser lembrado
type MemoryItem struct {
	ID              int64      `json:"id"`
	IdosoID         int64      `json:"idoso_id"`
	Content         string     `json:"content"`          // "Documento está na gaveta do escritório"
	Category        string     `json:"category"`         // location, medication, person, event, routine
	Trigger         string     `json:"trigger"`          // O que disparou (ex: "onde guardei o documento")
	Importance      int        `json:"importance"`       // 1-5 (5 = crítico)
	RepetitionCount int        `json:"repetition_count"` // Quantas vezes foi reforçado
	EaseFactor      float64    `json:"ease_factor"`      // Fator de facilidade (SM-2)
	IntervalDays    float64    `json:"interval_days"`    // Intervalo atual em dias
	NextReview      time.Time  `json:"next_review"`      // Próximo reforço
	LastReview      *time.Time `json:"last_review"`      // Último reforço
	SuccessCount    int        `json:"success_count"`    // Vezes que lembrou corretamente
	FailCount       int        `json:"fail_count"`       // Vezes que esqueceu
	Status          string     `json:"status"`           // active, paused, mastered, archived
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ReviewResult resultado de uma revisão
type ReviewResult struct {
	ItemID       int64   `json:"item_id"`
	Quality      int     `json:"quality"`       // 0-5 (0=esqueceu, 5=fácil)
	ResponseTime float64 `json:"response_time"` // Tempo para responder (segundos)
	Remembered   bool    `json:"remembered"`    // Se lembrou ou não
}

// Intervalos iniciais em horas (adaptados para idosos)
var initialIntervals = []float64{
	1,      // 1 hora
	4,      // 4 horas
	24,     // 1 dia
	72,     // 3 dias
	168,    // 1 semana
	336,    // 2 semanas
	720,    // 1 mês
}

// NewSpacedRepetitionService cria novo serviço
func NewSpacedRepetitionService(db *sql.DB) *SpacedRepetitionService {
	svc := &SpacedRepetitionService{
		db: db,
	}

	// Criar tabela se não existir
	if err := svc.createTable(); err != nil {
		log.Printf("⚠️ [SPACED] Erro ao criar tabela: %v", err)
	}

	// Iniciar goroutine para processar reforços pendentes
	go svc.processRemindersLoop()

	return svc
}

// SetNotifyFunc configura função de notificação
func (s *SpacedRepetitionService) SetNotifyFunc(fn func(idosoID int64, msgType string, payload interface{})) {
	s.notifyFunc = fn
}

// ============================================================================
// MÉTODOS PÚBLICOS
// ============================================================================

// CaptureMemory captura um novo item para reforço de memória
func (s *SpacedRepetitionService) CaptureMemory(ctx context.Context, idosoID int64, content, category, trigger string, importance int) (*MemoryItem, error) {
	if content == "" {
		return nil, fmt.Errorf("conteúdo não pode ser vazio")
	}

	// Normalizar categoria
	if category == "" {
		category = "general"
	}
	category = strings.ToLower(category)

	// Validar importância
	if importance < 1 || importance > 5 {
		importance = 3 // média
	}

	// Calcular primeiro intervalo baseado na importância
	// Items mais importantes = intervalos iniciais menores
	firstInterval := initialIntervals[0]
	if importance >= 4 {
		firstInterval = 0.5 // 30 minutos para itens críticos
	}

	nextReview := time.Now().Add(time.Duration(firstInterval * float64(time.Hour)))

	query := `
		INSERT INTO spaced_memory_items
		(idoso_id, content, category, trigger_phrase, importance, ease_factor, interval_hours, next_review, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 2.5, $6, $7, 'active', NOW(), NOW())
		RETURNING id, created_at
	`

	var item MemoryItem
	err := s.db.QueryRowContext(ctx, query, idosoID, content, category, trigger, importance, firstInterval, nextReview).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao capturar memória: %w", err)
	}

	item.IdosoID = idosoID
	item.Content = content
	item.Category = category
	item.Trigger = trigger
	item.Importance = importance
	item.EaseFactor = 2.5
	item.IntervalDays = firstInterval / 24
	item.NextReview = nextReview
	item.Status = "active"

	log.Printf("🧠 [SPACED] Nova memória capturada ID=%d: '%s' (próximo reforço em %.1fh)", item.ID, content, firstInterval)

	return &item, nil
}

// RecordReview registra resultado de uma revisão
func (s *SpacedRepetitionService) RecordReview(ctx context.Context, itemID int64, quality int, remembered bool) (*MemoryItem, error) {
	// Buscar item atual
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	// Aplicar algoritmo SM-2 adaptado
	item = s.calculateNextInterval(item, quality, remembered)

	// Atualizar no banco
	query := `
		UPDATE spaced_memory_items
		SET ease_factor = $1, interval_hours = $2, next_review = $3,
		    last_review = NOW(), repetition_count = repetition_count + 1,
		    success_count = success_count + $4, fail_count = fail_count + $5,
		    status = $6, updated_at = NOW()
		WHERE id = $7
	`

	successInc := 0
	failInc := 0
	if remembered {
		successInc = 1
	} else {
		failInc = 1
	}

	_, err = s.db.ExecContext(ctx, query,
		item.EaseFactor, item.IntervalDays*24, item.NextReview,
		successInc, failInc, item.Status, itemID)
	if err != nil {
		return nil, fmt.Errorf("erro ao atualizar revisão: %w", err)
	}

	log.Printf("🧠 [SPACED] Revisão registrada ID=%d: quality=%d, remembered=%v, próximo=%.1f dias",
		itemID, quality, remembered, item.IntervalDays)

	return item, nil
}

// GetPendingReviews retorna itens pendentes de revisão
func (s *SpacedRepetitionService) GetPendingReviews(ctx context.Context, idosoID int64, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 5
	}

	query := `
		SELECT id, idoso_id, content, category, COALESCE(trigger_phrase, ''), importance,
		       repetition_count, ease_factor, interval_hours, next_review, last_review,
		       success_count, fail_count, status, created_at, updated_at
		FROM spaced_memory_items
		WHERE idoso_id = $1 AND status = 'active' AND next_review <= NOW()
		ORDER BY importance DESC, next_review ASC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar revisões: %w", err)
	}
	defer rows.Close()

	var items []MemoryItem
	for rows.Next() {
		var item MemoryItem
		var intervalHours float64
		var lastReview sql.NullTime

		err := rows.Scan(
			&item.ID, &item.IdosoID, &item.Content, &item.Category, &item.Trigger,
			&item.Importance, &item.RepetitionCount, &item.EaseFactor, &intervalHours,
			&item.NextReview, &lastReview, &item.SuccessCount, &item.FailCount,
			&item.Status, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			continue
		}

		item.IntervalDays = intervalHours / 24
		if lastReview.Valid {
			item.LastReview = &lastReview.Time
		}

		items = append(items, item)
	}

	return items, nil
}

// GetItem busca um item específico
func (s *SpacedRepetitionService) GetItem(ctx context.Context, itemID int64) (*MemoryItem, error) {
	query := `
		SELECT id, idoso_id, content, category, COALESCE(trigger_phrase, ''), importance,
		       repetition_count, ease_factor, interval_hours, next_review, last_review,
		       success_count, fail_count, status, created_at, updated_at
		FROM spaced_memory_items
		WHERE id = $1
	`

	var item MemoryItem
	var intervalHours float64
	var lastReview sql.NullTime

	err := s.db.QueryRowContext(ctx, query, itemID).Scan(
		&item.ID, &item.IdosoID, &item.Content, &item.Category, &item.Trigger,
		&item.Importance, &item.RepetitionCount, &item.EaseFactor, &intervalHours,
		&item.NextReview, &lastReview, &item.SuccessCount, &item.FailCount,
		&item.Status, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("item não encontrado: %w", err)
	}

	item.IntervalDays = intervalHours / 24
	if lastReview.Valid {
		item.LastReview = &lastReview.Time
	}

	return &item, nil
}

// GetStats retorna estatísticas de memória do idoso
func (s *SpacedRepetitionService) GetStats(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'active') as active,
			COUNT(*) FILTER (WHERE status = 'mastered') as mastered,
			COUNT(*) FILTER (WHERE next_review <= NOW() AND status = 'active') as pending,
			COALESCE(AVG(success_count::float / NULLIF(repetition_count, 0)), 0) as avg_success_rate,
			COALESCE(AVG(ease_factor), 2.5) as avg_ease
		FROM spaced_memory_items
		WHERE idoso_id = $1
	`

	var total, active, mastered, pending int
	var avgSuccessRate, avgEase float64

	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(&total, &active, &mastered, &pending, &avgSuccessRate, &avgEase)
	if err != nil {
		return nil, err
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

// PauseItem pausa reforços de um item
func (s *SpacedRepetitionService) PauseItem(ctx context.Context, itemID int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE spaced_memory_items SET status = 'paused', updated_at = NOW() WHERE id = $1", itemID)
	return err
}

// ResumeItem retoma reforços de um item
func (s *SpacedRepetitionService) ResumeItem(ctx context.Context, itemID int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE spaced_memory_items SET status = 'active', next_review = NOW(), updated_at = NOW() WHERE id = $1", itemID)
	return err
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
		// Voltar para intervalo inicial, mas não menor que 30 min
		item.IntervalDays = initialIntervals[0] / 24 // 1 hora
		if item.Importance >= 4 {
			item.IntervalDays = 0.5 / 24 // 30 minutos para itens críticos
		}
		item.RepetitionCount = 0 // Reset repetition count

		// Diminuir ease factor
		item.EaseFactor = math.Max(1.3, item.EaseFactor-0.2)
	} else {
		// Lembrou corretamente
		if item.RepetitionCount == 0 {
			// Primeira repetição bem-sucedida
			item.IntervalDays = initialIntervals[1] / 24 // 4 horas
		} else if item.RepetitionCount == 1 {
			// Segunda repetição
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

	// Limitar intervalo máximo baseado na importância
	maxInterval := 30.0 // 30 dias para itens normais
	if item.Importance >= 4 {
		maxInterval = 14.0 // 2 semanas para itens críticos
	}
	if item.IntervalDays > maxInterval {
		item.IntervalDays = maxInterval
	}

	// Calcular próxima revisão
	item.NextReview = time.Now().Add(time.Duration(item.IntervalDays * 24 * float64(time.Hour)))

	// Verificar se foi "dominado" (10+ revisões com sucesso e intervalo > 2 semanas)
	if item.RepetitionCount >= 10 && item.IntervalDays >= 14 && item.SuccessCount > item.FailCount*3 {
		item.Status = "mastered"
	}

	return item
}

// ============================================================================
// PROCESSAMENTO AUTOMÁTICO DE LEMBRETES
// ============================================================================

func (s *SpacedRepetitionService) processRemindersLoop() {
	ticker := time.NewTicker(5 * time.Minute) // Verificar a cada 5 minutos
	defer ticker.Stop()

	for range ticker.C {
		s.sendPendingReminders()
	}
}

func (s *SpacedRepetitionService) sendPendingReminders() {
	if s.notifyFunc == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Buscar todos os idosos com revisões pendentes
	query := `
		SELECT DISTINCT idoso_id
		FROM spaced_memory_items
		WHERE status = 'active' AND next_review <= NOW()
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("⚠️ [SPACED] Erro ao buscar idosos: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var idosoID int64
		if err := rows.Scan(&idosoID); err != nil {
			continue
		}

		// Buscar itens pendentes para este idoso
		items, err := s.GetPendingReviews(ctx, idosoID, 3)
		if err != nil || len(items) == 0 {
			continue
		}

		// Enviar notificação
		for _, item := range items {
			s.notifyFunc(idosoID, "memory_reinforcement", map[string]interface{}{
				"item_id":    item.ID,
				"content":    item.Content,
				"category":   item.Category,
				"importance": item.Importance,
				"message":    s.buildReminderMessage(item),
			})

			log.Printf("🧠 [SPACED] Reforço enviado para idoso %d: '%s'", idosoID, item.Content)
		}
	}
}

func (s *SpacedRepetitionService) buildReminderMessage(item MemoryItem) string {
	// Mensagens contextuais baseadas na categoria
	switch item.Category {
	case "location":
		return fmt.Sprintf("Lembra onde você guardou? %s", item.Content)
	case "medication":
		return fmt.Sprintf("Importante lembrar: %s", item.Content)
	case "person":
		return fmt.Sprintf("Você lembra? %s", item.Content)
	case "event":
		return fmt.Sprintf("Não esqueça: %s", item.Content)
	case "routine":
		return fmt.Sprintf("Sua rotina: %s", item.Content)
	default:
		return fmt.Sprintf("Reforço de memória: %s", item.Content)
	}
}

// ============================================================================
// CRIAÇÃO DE TABELA
// ============================================================================

func (s *SpacedRepetitionService) createTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS spaced_memory_items (
			id SERIAL PRIMARY KEY,
			idoso_id BIGINT NOT NULL REFERENCES idosos(id),
			content TEXT NOT NULL,
			category VARCHAR(50) DEFAULT 'general',
			trigger_phrase TEXT,
			importance INT DEFAULT 3 CHECK (importance >= 1 AND importance <= 5),
			repetition_count INT DEFAULT 0,
			ease_factor DECIMAL(4,2) DEFAULT 2.5,
			interval_hours DECIMAL(10,2) DEFAULT 1,
			next_review TIMESTAMP NOT NULL,
			last_review TIMESTAMP,
			success_count INT DEFAULT 0,
			fail_count INT DEFAULT 0,
			status VARCHAR(20) DEFAULT 'active',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_spaced_idoso ON spaced_memory_items(idoso_id);
		CREATE INDEX IF NOT EXISTS idx_spaced_next_review ON spaced_memory_items(next_review) WHERE status = 'active';
		CREATE INDEX IF NOT EXISTS idx_spaced_status ON spaced_memory_items(status);
		CREATE INDEX IF NOT EXISTS idx_spaced_category ON spaced_memory_items(category);
	`

	_, err := s.db.Exec(query)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}

	log.Println("✅ [SPACED] Tabela 'spaced_memory_items' verificada/criada")
	return nil
}
