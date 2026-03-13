// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package habits

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// HABIT TRACKER - Monitoramento de Comportamentos para Idosos
// ============================================================================
// Registra sucesso/falha de hábitos e identifica padrões para ajustar notificações

// HabitTracker gerencia o rastreamento de hábitos
type HabitTracker struct {
	db         *database.DB
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
	ID        int64     `json:"id"`
	HabitID   int64     `json:"habit_id"`
	IdosoID   int64     `json:"idoso_id"`
	Success   bool      `json:"success"`     // true = completou, false = falhou/pulou
	Timestamp time.Time `json:"timestamp"`
	DayOfWeek int       `json:"day_of_week"` // 0=Domingo, 6=Sábado
	TimeOfDay string    `json:"time_of_day"` // "morning", "afternoon", "evening", "night"
	Source    string    `json:"source"`      // "voice", "app", "auto"
	Notes     string    `json:"notes"`       // Observações
	Metadata  string    `json:"metadata"`    // JSON com dados extras
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
func NewHabitTracker(db *database.DB) *HabitTracker {
	tracker := &HabitTracker{db: db}

	if db == nil {
		log.Printf("⚠️ [HABITS] NietzscheDB unavailable — HabitTracker running in degraded mode")
		return tracker
	}

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
	dateStr := now.Format("2006-01-02")

	metadataJSON := "{}"
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(b)
		}
	}

	content := map[string]interface{}{
		"habit_id":    habit.ID,
		"idoso_id":    idosoID,
		"success":     success,
		"logged_at":   now.Format(time.RFC3339),
		"day_of_week": int(now.Weekday()),
		"time_of_day": timeOfDay,
		"source":      source,
		"notes":       notes,
		"metadata":    metadataJSON,
	}

	logID, err := h.db.Insert(ctx, "habit_logs", content)
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

	// ── Graph edges: connect user → habit → completion event ──
	go h.ensureHabitEdges(idosoID, habit, logID, habitName, dateStr, success)

	// Atualizar padrões em background
	go h.updatePatterns(idosoID, habit.ID)

	return logEntry, nil
}

// ensureHabitEdges creates graph edges linking user → habit → completion event.
// 1. MergeNode for user profile (node_label="UserProfile")
// 2. MergeNode for habit definition (node_label="Habit")
// 3. MergeNode for habit completion event (node_label="HabitLog", match on habit_name+date)
// 4. Edge: user → habit (TRACKS_HABIT)
// 5. Edge: habit → completion event (COMPLETED_ON)
func (h *HabitTracker) ensureHabitEdges(idosoID int64, habit *Habit, logID int64, habitName, dateStr string, success bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. MergeNode: user profile
	userNodeID, _, err := h.db.MergeNode(ctx, "UserProfile",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"idoso_id":   idosoID,
			"created_at": time.Now().Format(time.RFC3339),
		},
		nil, // no update on match
	)
	if err != nil {
		log.Printf("⚠️ [HABITS] MergeNode UserProfile failed: %v", err)
		return
	}

	// 2. MergeNode: habit definition
	habitNodeID, _, err := h.db.MergeNode(ctx, "Habit",
		map[string]interface{}{
			"habit_name": habitName,
		},
		map[string]interface{}{
			"habit_name":  habitName,
			"description": habit.Description,
			"category":    habit.Category,
			"created_at":  time.Now().Format(time.RFC3339),
		},
		nil,
	)
	if err != nil {
		log.Printf("⚠️ [HABITS] MergeNode Habit failed: %v", err)
		return
	}

	// 3. MergeNode: habit completion event (unique per habit_name + date)
	completedStr := "false"
	if success {
		completedStr = "true"
	}
	habitLogNodeID, _, err := h.db.MergeNode(ctx, "HabitLog",
		map[string]interface{}{
			"habit_name": habitName,
			"date":       dateStr,
			"idoso_id":   idosoID,
		},
		map[string]interface{}{
			"habit_name": habitName,
			"date":       dateStr,
			"idoso_id":   idosoID,
			"completed":  completedStr,
			"timestamp":  time.Now().Format(time.RFC3339),
			"log_id":     logID,
		},
		map[string]interface{}{
			"completed": completedStr,
			"timestamp": time.Now().Format(time.RFC3339),
			"log_id":    logID,
		},
	)
	if err != nil {
		log.Printf("⚠️ [HABITS] MergeNode HabitLog failed: %v", err)
		return
	}

	// 4. Edge: user → habit (TRACKS_HABIT) — idempotent via MergeEdge
	if _, err := h.db.MergeEdge(ctx, userNodeID, habitNodeID, "TRACKS_HABIT"); err != nil {
		log.Printf("⚠️ [HABITS] MergeEdge TRACKS_HABIT failed: %v", err)
	}

	// 5. Edge: habit → completion event (COMPLETED_ON)
	if _, err := h.db.MergeEdge(ctx, habitNodeID, habitLogNodeID, "COMPLETED_ON"); err != nil {
		log.Printf("⚠️ [HABITS] MergeEdge COMPLETED_ON failed: %v", err)
	}

	log.Printf("✅ [HABITS] Graph edges created: user(%s) -TRACKS_HABIT-> habit(%s) -COMPLETED_ON-> log(%s)",
		userNodeID[:8], habitNodeID[:8], habitLogNodeID[:8])
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

	// Fetch all habit_logs for this habit + idoso from the last 30 days
	cutoff := time.Now().AddDate(0, 0, -30)
	logs, err := h.db.QueryByLabel(ctx, "habit_logs",
		" AND n.habit_id = $habit_id AND n.idoso_id = $idoso_id",
		map[string]interface{}{
			"habit_id": habit.ID,
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar logs de hábito: %w", err)
	}

	// Filter to last 30 days in Go (NQL doesn't support date arithmetic)
	var recentLogs []map[string]interface{}
	for _, m := range logs {
		loggedAt := database.GetTime(m, "logged_at")
		if !loggedAt.IsZero() && loggedAt.After(cutoff) {
			recentLogs = append(recentLogs, m)
		}
	}

	// Taxa geral de sucesso
	if len(recentLogs) > 0 {
		successCount := 0
		for _, m := range recentLogs {
			if database.GetBool(m, "success") {
				successCount++
			}
		}
		pattern.SuccessRate = float64(successCount) / float64(len(recentLogs))
	}

	// Taxa por dia da semana
	dayTotal := make(map[int]int)
	daySuccess := make(map[int]int)
	for _, m := range recentLogs {
		day := int(database.GetInt64(m, "day_of_week"))
		dayTotal[day]++
		if database.GetBool(m, "success") {
			daySuccess[day]++
		}
	}
	for day, total := range dayTotal {
		if total > 0 {
			pattern.ByDayOfWeek[day] = float64(daySuccess[day]) / float64(total)
		}
	}

	// Taxa por período do dia
	timeTotal := make(map[string]int)
	timeSuccess := make(map[string]int)
	for _, m := range recentLogs {
		period := database.GetString(m, "time_of_day")
		if period == "" {
			continue
		}
		timeTotal[period]++
		if database.GetBool(m, "success") {
			timeSuccess[period]++
		}
	}
	for period, total := range timeTotal {
		if total > 0 {
			pattern.ByTimeOfDay[period] = float64(timeSuccess[period]) / float64(total)
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
	// Get all active habits
	habits, err := h.db.QueryByLabel(ctx, "habits",
		" AND n.active = $active",
		map[string]interface{}{
			"active": true,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Get all habit_logs for this idoso to find which habits have logs
	logs, err := h.db.QueryByLabel(ctx, "habit_logs",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Build set of habit IDs that have logs for this idoso
	loggedHabitIDs := make(map[int64]bool)
	for _, m := range logs {
		loggedHabitIDs[database.GetInt64(m, "habit_id")] = true
	}

	// Build patterns only for habits that have log entries
	var patterns []HabitPattern
	for _, m := range habits {
		habitID := database.GetInt64(m, "id")
		habitName := database.GetString(m, "name")
		if !loggedHabitIDs[habitID] {
			continue
		}
		if pattern, err := h.GetPattern(ctx, idosoID, habitName); err == nil {
			patterns = append(patterns, *pattern)
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
	// Get all active habits
	allHabits, err := h.db.QueryByLabel(ctx, "habits",
		" AND n.active = $active",
		map[string]interface{}{
			"active": true,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Get today's logs for this idoso
	todayLogs, err := h.db.QueryByLabel(ctx, "habit_logs",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Filter logs to today only
	today := time.Now().Truncate(24 * time.Hour)
	todayEnd := today.Add(24 * time.Hour)
	var todaysLogs []map[string]interface{}
	for _, m := range todayLogs {
		loggedAt := database.GetTime(m, "logged_at")
		if !loggedAt.IsZero() && !loggedAt.Before(today) && loggedAt.Before(todayEnd) {
			todaysLogs = append(todaysLogs, m)
		}
	}

	// Group today's logs by habit_id
	logsByHabit := make(map[int64][]map[string]interface{})
	for _, m := range todaysLogs {
		hid := database.GetInt64(m, "habit_id")
		logsByHabit[hid] = append(logsByHabit[hid], m)
	}

	habits := make([]map[string]interface{}, 0)
	totalCompleted := 0
	totalHabits := 0

	for _, m := range allHabits {
		habitID := database.GetInt64(m, "id")
		name := database.GetString(m, "name")
		description := database.GetString(m, "description")

		habitLogs := logsByHabit[habitID]
		completed := 0
		total := len(habitLogs)
		for _, lg := range habitLogs {
			if database.GetBool(lg, "success") {
				completed++
			}
		}

		habits = append(habits, map[string]interface{}{
			"name":        name,
			"description": description,
			"completed":   completed,
			"total":       total,
		})
		totalCompleted += completed
		totalHabits++
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
	now := time.Now()

	content := map[string]interface{}{
		"name":           name,
		"description":    description,
		"category":       category,
		"target_per_day": 1,
		"active":         true,
		"created_at":     now.Format(time.RFC3339),
	}

	id, err := h.db.Insert(ctx, "habits", content)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar hábito: %w", err)
	}

	habit = &Habit{
		ID:           id,
		Name:         name,
		Description:  description,
		Category:     category,
		TargetPerDay: 1,
		CreatedAt:    now,
		Active:       true,
	}

	return habit, nil
}

func (h *HabitTracker) getHabitByName(ctx context.Context, idosoID int64, name string) (*Habit, error) {
	rows, err := h.db.QueryByLabel(ctx, "habits",
		" AND n.name = $name AND n.active = $active",
		map[string]interface{}{
			"name":   name,
			"active": true,
		}, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("hábito não encontrado: %s", name)
	}

	m := rows[0]
	return &Habit{
		ID:           database.GetInt64(m, "id"),
		Name:         database.GetString(m, "name"),
		Description:  database.GetString(m, "description"),
		Category:     database.GetString(m, "category"),
		TargetPerDay: int(database.GetInt64(m, "target_per_day")),
		CreatedAt:    database.GetTime(m, "created_at"),
		Active:       database.GetBool(m, "active"),
	}, nil
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
	// Fetch recent logs ordered by time (we sort in Go)
	logs, err := h.db.QueryByLabel(ctx, "habit_logs",
		" AND n.habit_id = $habit_id AND n.idoso_id = $idoso_id",
		map[string]interface{}{
			"habit_id": habitID,
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		return 0, 0
	}

	type logEntry struct {
		date    time.Time
		success bool
	}

	var entries []logEntry
	for _, m := range logs {
		loggedAt := database.GetTime(m, "logged_at")
		if loggedAt.IsZero() {
			continue
		}
		entries = append(entries, logEntry{
			date:    loggedAt,
			success: database.GetBool(m, "success"),
		})
	}

	// Sort descending by date (most recent first), limit to 90
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date.After(entries[j].date)
	})
	if len(entries) > 90 {
		entries = entries[:90]
	}

	// Calcular streak atual
	for _, d := range entries {
		if d.success {
			current++
		} else {
			break
		}
	}

	// Calcular maior streak
	streak := 0
	for _, d := range entries {
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

	// Buscar nome do hábito pelo ID
	habitNode, err := h.db.GetNodeByID(ctx, "habits", habitID)
	if err != nil || habitNode == nil {
		return
	}
	habitName := database.GetString(habitNode, "name")

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
		// Check if habit already exists
		existing, err := h.db.QueryByLabel(ctx, "habits",
			" AND n.name = $name",
			map[string]interface{}{
				"name": d.name,
			}, 1)
		if err != nil {
			log.Printf("⚠️ [HABITS] Erro ao verificar hábito %s: %v", d.name, err)
			continue
		}

		if len(existing) > 0 {
			// Already exists, ensure it is active
			if !database.GetBool(existing[0], "active") {
				_ = h.db.Update(ctx, "habits",
					map[string]interface{}{"name": d.name},
					map[string]interface{}{"active": true},
				)
			}
			continue
		}

		// Insert new default habit
		_, err = h.db.Insert(ctx, "habits", map[string]interface{}{
			"name":           d.name,
			"description":    d.description,
			"category":       d.category,
			"target_per_day": d.target,
			"active":         true,
			"created_at":     time.Now().Format(time.RFC3339),
		})
		if err != nil {
			log.Printf("⚠️ [HABITS] Erro ao criar hábito padrão %s: %v", d.name, err)
		}
	}

	log.Println("✅ [HABITS] Hábitos padrão verificados")
}

// ============================================================================
// CRIAÇÃO DE TABELAS (NO-OP para NietzscheDB)
// ============================================================================

func (h *HabitTracker) createTables() error {
	// NietzscheDB is schemaless — no CREATE TABLE needed.
	log.Println("✅ [HABITS] Tabelas de hábitos verificadas/criadas (NietzscheDB: schemaless)")
	return nil
}
