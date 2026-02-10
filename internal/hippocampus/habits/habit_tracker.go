package habits

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ============================================================================
// HABIT TRACKER - Monitoramento de Comportamentos para Idosos
// ============================================================================
// Registra sucesso/falha de hábitos e identifica padrões para ajustar notificações

// HabitTracker gerencia o rastreamento de hábitos
type HabitTracker struct {
	db         *sql.DB
	notifyFunc func(idosoID int64, msgType string, payload interface{})
}

// Habit representa um hábito sendo rastreado
type Habit struct {
	ID            int64     `json:"id"`
	IdosoID       int64     `json:"idoso_id"`
	Name          string    `json:"name"`           // "tomar_agua", "tomar_remedio", "exercicio", etc
	Description   string    `json:"description"`    // Descrição amigável
	Category      string    `json:"category"`       // health, medication, activity, social
	TargetPerDay  int       `json:"target_per_day"` // Meta diária (ex: 8 copos de água)
	ReminderTimes []string  `json:"reminder_times"` // Horários de lembrete ["08:00", "12:00", "18:00"]
	CreatedAt     time.Time `json:"created_at"`
	Active        bool      `json:"active"`
}

// HabitLog representa um registro de hábito
type HabitLog struct {
	ID          int64      `json:"id"`
	HabitID     int64      `json:"habit_id"`
	IdosoID     int64      `json:"idoso_id"`
	Success     bool       `json:"success"`       // true = completou, false = falhou/pulou
	Timestamp   time.Time  `json:"timestamp"`
	DayOfWeek   int        `json:"day_of_week"`   // 0=Domingo, 6=Sábado
	TimeOfDay   string     `json:"time_of_day"`   // "morning", "afternoon", "evening", "night"
	Source      string     `json:"source"`        // "voice", "app", "auto"
	Notes       string     `json:"notes"`         // Observações
	Metadata    string     `json:"metadata"`      // JSON com dados extras
}

// HabitPattern padrão identificado
type HabitPattern struct {
	HabitID          int64              `json:"habit_id"`
	HabitName        string             `json:"habit_name"`
	SuccessRate      float64            `json:"success_rate"`       // Taxa geral de sucesso
	ByDayOfWeek      map[int]float64    `json:"by_day_of_week"`     // Taxa por dia da semana
	ByTimeOfDay      map[string]float64 `json:"by_time_of_day"`     // Taxa por período
	WeakDays         []int              `json:"weak_days"`          // Dias com menor sucesso
	BestDays         []int              `json:"best_days"`          // Dias com maior sucesso
	Streak           int                `json:"streak"`             // Sequência atual
	LongestStreak    int                `json:"longest_streak"`     // Maior sequência
	RecommendedLevel string             `json:"recommended_level"`  // Nível de notificação recomendado
}

// NotificationLevel níveis de agressividade de notificação
type NotificationLevel string

const (
	NOTIFY_GENTLE    NotificationLevel = "gentle"    // 1 lembrete suave
	NOTIFY_NORMAL    NotificationLevel = "normal"    // 2 lembretes
	NOTIFY_ASSERTIVE NotificationLevel = "assertive" // 3 lembretes + família
	NOTIFY_CRITICAL  NotificationLevel = "critical"  // Alertas contínuos + escalation
)

// Dias da semana em português
var dayNames = map[int]string{
	0: "domingo", 1: "segunda", 2: "terça", 3: "quarta",
	4: "quinta", 5: "sexta", 6: "sábado",
}

// NewHabitTracker cria novo rastreador
func NewHabitTracker(db *sql.DB) *HabitTracker {
	tracker := &HabitTracker{db: db}

	if err := tracker.createTables(); err != nil {
		log.Printf("⚠️ [HABITS] Erro ao criar tabelas: %v", err)
	}

	// Criar hábitos padrão se não existirem
	go tracker.ensureDefaultHabits()

	return tracker
}

// SetNotifyFunc configura função de notificação
func (h *HabitTracker) SetNotifyFunc(fn func(idosoID int64, msgType string, payload interface{})) {
	h.notifyFunc = fn
}

// ============================================================================
// REGISTRO DE HÁBITOS
// ============================================================================

// LogHabit registra sucesso ou falha de um hábito
func (h *HabitTracker) LogHabit(ctx context.Context, idosoID int64, habitName string, success bool, source, notes string, metadata map[string]interface{}) (*HabitLog, error) {
	// Buscar ou criar hábito
	habit, err := h.getOrCreateHabit(ctx, idosoID, habitName)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	timeOfDay := h.getTimeOfDay(now)

	metadataJSON := "{}"
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(b)
		}
	}

	query := `
		INSERT INTO habit_logs (habit_id, idoso_id, success, logged_at, day_of_week, time_of_day, source, notes, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var logID int64
	err = h.db.QueryRowContext(ctx, query,
		habit.ID, idosoID, success, now, int(now.Weekday()),
		timeOfDay, source, notes, metadataJSON,
	).Scan(&logID)
	if err != nil {
		return nil, fmt.Errorf("erro ao registrar hábito: %w", err)
	}

	logEntry := &HabitLog{
		ID:        logID,
		HabitID:   habit.ID,
		IdosoID:   idosoID,
		Success:   success,
		Timestamp: now,
		DayOfWeek: int(now.Weekday()),
		TimeOfDay: timeOfDay,
		Source:    source,
		Notes:     notes,
	}

	status := "✅"
	if !success {
		status = "❌"
	}
	log.Printf("%s [HABITS] %s registrado para idoso %d: %s", status, habitName, idosoID, source)

	// Atualizar padrões em background
	go h.updatePatterns(idosoID, habit.ID)

	return logEntry, nil
}

// LogMedication atalho para registrar medicamento
func (h *HabitTracker) LogMedication(ctx context.Context, idosoID int64, medicationName string, taken bool, source string) (*HabitLog, error) {
	habitName := "tomar_remedio"
	notes := medicationName
	metadata := map[string]interface{}{
		"medication": medicationName,
		"taken":      taken,
	}
	return h.LogHabit(ctx, idosoID, habitName, taken, source, notes, metadata)
}

// LogWater atalho para registrar água
func (h *HabitTracker) LogWater(ctx context.Context, idosoID int64, glasses int, source string) (*HabitLog, error) {
	habitName := "tomar_agua"
	notes := fmt.Sprintf("%d copo(s)", glasses)
	metadata := map[string]interface{}{
		"glasses": glasses,
	}
	return h.LogHabit(ctx, idosoID, habitName, true, source, notes, metadata)
}

// ============================================================================
// ANÁLISE DE PADRÕES
// ============================================================================

// GetPattern retorna padrões de um hábito específico
func (h *HabitTracker) GetPattern(ctx context.Context, idosoID int64, habitName string) (*HabitPattern, error) {
	habit, err := h.getHabitByName(ctx, idosoID, habitName)
	if err != nil {
		return nil, err
	}

	pattern := &HabitPattern{
		HabitID:     habit.ID,
		HabitName:   habit.Name,
		ByDayOfWeek: make(map[int]float64),
		ByTimeOfDay: make(map[string]float64),
	}

	// Taxa geral de sucesso (últimos 30 dias)
	generalQuery := `
		SELECT
			COALESCE(AVG(CASE WHEN success THEN 1.0 ELSE 0.0 END), 0) as success_rate
		FROM habit_logs
		WHERE habit_id = $1 AND idoso_id = $2 AND logged_at > NOW() - INTERVAL '30 days'
	`
	h.db.QueryRowContext(ctx, generalQuery, habit.ID, idosoID).Scan(&pattern.SuccessRate)

	// Taxa por dia da semana
	dayQuery := `
		SELECT day_of_week,
		       COALESCE(AVG(CASE WHEN success THEN 1.0 ELSE 0.0 END), 0) as success_rate
		FROM habit_logs
		WHERE habit_id = $1 AND idoso_id = $2 AND logged_at > NOW() - INTERVAL '30 days'
		GROUP BY day_of_week
	`
	rows, err := h.db.QueryContext(ctx, dayQuery, habit.ID, idosoID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var day int
			var rate float64
			if rows.Scan(&day, &rate) == nil {
				pattern.ByDayOfWeek[day] = rate
			}
		}
	}

	// Taxa por período do dia
	timeQuery := `
		SELECT time_of_day,
		       COALESCE(AVG(CASE WHEN success THEN 1.0 ELSE 0.0 END), 0) as success_rate
		FROM habit_logs
		WHERE habit_id = $1 AND idoso_id = $2 AND logged_at > NOW() - INTERVAL '30 days'
		GROUP BY time_of_day
	`
	rows2, err := h.db.QueryContext(ctx, timeQuery, habit.ID, idosoID)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var period string
			var rate float64
			if rows2.Scan(&period, &rate) == nil {
				pattern.ByTimeOfDay[period] = rate
			}
		}
	}

	// Identificar dias fracos e fortes
	for day, rate := range pattern.ByDayOfWeek {
		if rate < 0.5 {
			pattern.WeakDays = append(pattern.WeakDays, day)
		} else if rate > 0.8 {
			pattern.BestDays = append(pattern.BestDays, day)
		}
	}

	// Calcular streak
	pattern.Streak, pattern.LongestStreak = h.calculateStreak(ctx, habit.ID, idosoID)

	// Recomendar nível de notificação
	pattern.RecommendedLevel = h.recommendNotificationLevel(pattern)

	return pattern, nil
}

// GetAllPatterns retorna padrões de todos os hábitos do idoso
func (h *HabitTracker) GetAllPatterns(ctx context.Context, idosoID int64) ([]HabitPattern, error) {
	query := `
		SELECT DISTINCT h.id, h.name
		FROM habits h
		JOIN habit_logs hl ON h.id = hl.habit_id
		WHERE hl.idoso_id = $1 AND h.active = true
	`

	rows, err := h.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []HabitPattern
	for rows.Next() {
		var habitID int64
		var habitName string
		if rows.Scan(&habitID, &habitName) == nil {
			if pattern, err := h.GetPattern(ctx, idosoID, habitName); err == nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	return patterns, nil
}

// GetNotificationLevel retorna o nível de notificação recomendado para um hábito
func (h *HabitTracker) GetNotificationLevel(ctx context.Context, idosoID int64, habitName string) (NotificationLevel, string) {
	pattern, err := h.GetPattern(ctx, idosoID, habitName)
	if err != nil {
		return NOTIFY_NORMAL, "padrão (sem histórico)"
	}

	level := NotificationLevel(pattern.RecommendedLevel)

	// Verificar se hoje é um dia fraco
	today := int(time.Now().Weekday())
	for _, weakDay := range pattern.WeakDays {
		if weakDay == today {
			// Aumentar agressividade em dias fracos
			switch level {
			case NOTIFY_GENTLE:
				level = NOTIFY_NORMAL
			case NOTIFY_NORMAL:
				level = NOTIFY_ASSERTIVE
			}
			return level, fmt.Sprintf("aumentado para %s (hoje é %s, dia de dificuldade)", level, dayNames[today])
		}
	}

	return level, pattern.RecommendedLevel
}

// ============================================================================
// RELATÓRIOS
// ============================================================================

// GetDailySummary retorna resumo do dia
func (h *HabitTracker) GetDailySummary(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	query := `
		SELECT h.name, h.description,
		       COUNT(*) FILTER (WHERE hl.success = true) as completed,
		       COUNT(*) as total
		FROM habits h
		LEFT JOIN habit_logs hl ON h.id = hl.habit_id
		    AND hl.idoso_id = $1
		    AND hl.logged_at::date = CURRENT_DATE
		WHERE h.active = true
		GROUP BY h.id, h.name, h.description
	`

	rows, err := h.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	habits := make([]map[string]interface{}, 0)
	totalCompleted := 0
	totalHabits := 0

	for rows.Next() {
		var name, description string
		var completed, total int
		if rows.Scan(&name, &description, &completed, &total) == nil {
			habits = append(habits, map[string]interface{}{
				"name":        name,
				"description": description,
				"completed":   completed,
				"total":       total,
			})
			totalCompleted += completed
			totalHabits++
		}
	}

	return map[string]interface{}{
		"date":            time.Now().Format("02/01/2006"),
		"habits":          habits,
		"total_completed": totalCompleted,
		"total_habits":    totalHabits,
	}, nil
}

// GetWeeklyReport retorna relatório semanal
func (h *HabitTracker) GetWeeklyReport(ctx context.Context, idosoID int64) (map[string]interface{}, error) {
	patterns, err := h.GetAllPatterns(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	// Encontrar hábitos problemáticos
	var problematic []string
	var excellent []string

	for _, p := range patterns {
		if p.SuccessRate < 0.5 {
			problematic = append(problematic, p.HabitName)
		} else if p.SuccessRate > 0.9 {
			excellent = append(excellent, p.HabitName)
		}
	}

	// Encontrar dias mais difíceis
	dayProblems := make(map[int]int)
	for _, p := range patterns {
		for _, day := range p.WeakDays {
			dayProblems[day]++
		}
	}

	var difficultDays []string
	for day, count := range dayProblems {
		if count > 0 {
			difficultDays = append(difficultDays, fmt.Sprintf("%s (%d problemas)", dayNames[day], count))
		}
	}

	return map[string]interface{}{
		"patterns":       patterns,
		"problematic":    problematic,
		"excellent":      excellent,
		"difficult_days": difficultDays,
		"week_start":     time.Now().AddDate(0, 0, -7).Format("02/01"),
		"week_end":       time.Now().Format("02/01"),
	}, nil
}

// ============================================================================
// HELPERS INTERNOS
// ============================================================================

func (h *HabitTracker) getOrCreateHabit(ctx context.Context, idosoID int64, name string) (*Habit, error) {
	habit, err := h.getHabitByName(ctx, idosoID, name)
	if err == nil {
		return habit, nil
	}

	// Criar hábito padrão
	description := h.getDefaultDescription(name)
	category := h.getDefaultCategory(name)

	query := `
		INSERT INTO habits (name, description, category, target_per_day, active, created_at)
		VALUES ($1, $2, $3, 1, true, NOW())
		ON CONFLICT (name) DO UPDATE SET active = true
		RETURNING id, name, description, category, target_per_day, created_at, active
	`

	habit = &Habit{}
	err = h.db.QueryRowContext(ctx, query, name, description, category).Scan(
		&habit.ID, &habit.Name, &habit.Description, &habit.Category,
		&habit.TargetPerDay, &habit.CreatedAt, &habit.Active,
	)
	if err != nil {
		return nil, err
	}

	return habit, nil
}

func (h *HabitTracker) getHabitByName(ctx context.Context, idosoID int64, name string) (*Habit, error) {
	query := `
		SELECT id, name, description, category, target_per_day, created_at, active
		FROM habits
		WHERE name = $1 AND active = true
		LIMIT 1
	`

	habit := &Habit{}
	err := h.db.QueryRowContext(ctx, query, name).Scan(
		&habit.ID, &habit.Name, &habit.Description, &habit.Category,
		&habit.TargetPerDay, &habit.CreatedAt, &habit.Active,
	)
	if err != nil {
		return nil, err
	}

	return habit, nil
}

func (h *HabitTracker) getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 18:
		return "afternoon"
	case hour >= 18 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

func (h *HabitTracker) calculateStreak(ctx context.Context, habitID, idosoID int64) (current, longest int) {
	query := `
		SELECT logged_at::date, success
		FROM habit_logs
		WHERE habit_id = $1 AND idoso_id = $2
		ORDER BY logged_at DESC
		LIMIT 90
	`

	rows, err := h.db.QueryContext(ctx, query, habitID, idosoID)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()

	var dates []struct {
		date    time.Time
		success bool
	}

	for rows.Next() {
		var d struct {
			date    time.Time
			success bool
		}
		if rows.Scan(&d.date, &d.success) == nil {
			dates = append(dates, d)
		}
	}

	// Calcular streak atual
	for _, d := range dates {
		if d.success {
			current++
		} else {
			break
		}
	}

	// Calcular maior streak
	streak := 0
	for _, d := range dates {
		if d.success {
			streak++
			if streak > longest {
				longest = streak
			}
		} else {
			streak = 0
		}
	}

	return current, longest
}

func (h *HabitTracker) recommendNotificationLevel(pattern *HabitPattern) string {
	rate := pattern.SuccessRate

	switch {
	case rate >= 0.9:
		return string(NOTIFY_GENTLE)
	case rate >= 0.7:
		return string(NOTIFY_NORMAL)
	case rate >= 0.5:
		return string(NOTIFY_ASSERTIVE)
	default:
		return string(NOTIFY_CRITICAL)
	}
}

func (h *HabitTracker) updatePatterns(idosoID, habitID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Buscar padrão atualizado
	var habitName string
	h.db.QueryRowContext(ctx, "SELECT name FROM habits WHERE id = $1", habitID).Scan(&habitName)

	pattern, err := h.GetPattern(ctx, idosoID, habitName)
	if err != nil {
		return
	}

	// Notificar se há dias problemáticos detectados
	if len(pattern.WeakDays) > 0 && h.notifyFunc != nil {
		var dayList []string
		for _, d := range pattern.WeakDays {
			dayList = append(dayList, dayNames[d])
		}

		h.notifyFunc(idosoID, "habit_pattern_detected", map[string]interface{}{
			"habit":      habitName,
			"weak_days":  dayList,
			"suggestion": fmt.Sprintf("Notamos que %s é mais difícil %s. Vamos reforçar os lembretes nesses dias.", habitName, strings.Join(dayList, ", ")),
		})
	}
}

func (h *HabitTracker) getDefaultDescription(name string) string {
	descriptions := map[string]string{
		"tomar_agua":    "Beber água regularmente",
		"tomar_remedio": "Tomar medicamentos no horário",
		"exercicio":     "Fazer exercício físico",
		"caminhar":      "Caminhar ao ar livre",
		"dormir":        "Dormir bem e no horário",
		"comer":         "Fazer refeições regulares",
		"socializar":    "Interagir com família ou amigos",
	}

	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return name
}

func (h *HabitTracker) getDefaultCategory(name string) string {
	categories := map[string]string{
		"tomar_agua":    "health",
		"tomar_remedio": "medication",
		"exercicio":     "activity",
		"caminhar":      "activity",
		"dormir":        "health",
		"comer":         "health",
		"socializar":    "social",
	}

	if cat, ok := categories[name]; ok {
		return cat
	}
	return "general"
}

func (h *HabitTracker) ensureDefaultHabits() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defaults := []struct {
		name        string
		description string
		category    string
		target      int
	}{
		{"tomar_agua", "Beber água regularmente", "health", 8},
		{"tomar_remedio", "Tomar medicamentos no horário", "medication", 1},
		{"exercicio", "Fazer exercício físico", "activity", 1},
		{"comer", "Fazer refeições regulares", "health", 3},
	}

	for _, d := range defaults {
		query := `
			INSERT INTO habits (name, description, category, target_per_day, active, created_at)
			VALUES ($1, $2, $3, $4, true, NOW())
			ON CONFLICT (name) DO NOTHING
		`
		h.db.ExecContext(ctx, query, d.name, d.description, d.category, d.target)
	}

	log.Println("✅ [HABITS] Hábitos padrão verificados")
}

// ============================================================================
// CRIAÇÃO DE TABELAS
// ============================================================================

func (h *HabitTracker) createTables() error {
	query := `
		CREATE TABLE IF NOT EXISTS habits (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) UNIQUE NOT NULL,
			description TEXT,
			category VARCHAR(50) DEFAULT 'general',
			target_per_day INT DEFAULT 1,
			reminder_times JSONB DEFAULT '[]',
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS habit_logs (
			id SERIAL PRIMARY KEY,
			habit_id INT REFERENCES habits(id),
			idoso_id BIGINT NOT NULL REFERENCES idosos(id),
			success BOOLEAN NOT NULL,
			logged_at TIMESTAMP DEFAULT NOW(),
			day_of_week INT NOT NULL,
			time_of_day VARCHAR(20),
			source VARCHAR(50) DEFAULT 'voice',
			notes TEXT,
			metadata JSONB DEFAULT '{}'
		);

		CREATE INDEX IF NOT EXISTS idx_habit_logs_idoso ON habit_logs(idoso_id);
		CREATE INDEX IF NOT EXISTS idx_habit_logs_habit ON habit_logs(habit_id);
		CREATE INDEX IF NOT EXISTS idx_habit_logs_date ON habit_logs(logged_at);
		CREATE INDEX IF NOT EXISTS idx_habit_logs_day ON habit_logs(day_of_week);
	`

	_, err := h.db.Exec(query)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}

	log.Println("✅ [HABITS] Tabelas de hábitos verificadas/criadas")
	return nil
}
