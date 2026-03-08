// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cron

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskExecutor função que executa uma tool
type TaskExecutor func(toolName string, args map[string]interface{}, idosoID int64) (map[string]interface{}, error)

// Task tarefa agendada
type Task struct {
	ID          string                 `json:"id"`
	IdosoID     int64                  `json:"idoso_id"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule"` // "every 5m", "every 1h", "daily 08:00", "weekly mon 09:00"
	ToolName    string                 `json:"tool_name"`
	ToolArgs    map[string]interface{} `json:"tool_args"`
	NextRun     time.Time              `json:"next_run"`
	LastRun     time.Time              `json:"last_run,omitempty"`
	RunCount    int                    `json:"run_count"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Service scheduler de tarefas com cron-like scheduling
type Service struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	executor TaskExecutor
	stop     chan struct{}
	stopOnce sync.Once
	running  bool
}

// NewService cria cron service
func NewService() *Service {
	return &Service{
		tasks: make(map[string]*Task),
		stop:  make(chan struct{}),
	}
}

// SetExecutor configura o executor de tools
func (s *Service) SetExecutor(executor TaskExecutor) {
	s.executor = executor
}

// Start inicia o loop do scheduler
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		log.Println("⏰ [CRON] Scheduler iniciado")

		for {
			select {
			case <-ticker.C:
				s.checkAndExecute()
			case <-s.stop:
				log.Println("⏰ [CRON] Scheduler parado")
				return
			}
		}
	}()
}

// Stop para o scheduler (safe to call multiple times)
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		s.running = false
	})
}

// CreateTask cria nova tarefa agendada
func (s *Service) CreateTask(idosoID int64, description, schedule, toolName string, toolArgs map[string]interface{}) (*Task, error) {
	nextRun, err := parseSchedule(schedule, time.Now())
	if err != nil {
		return nil, fmt.Errorf("schedule inválido: %v", err)
	}

	task := &Task{
		ID:          uuid.New().String()[:8],
		IdosoID:     idosoID,
		Description: description,
		Schedule:    schedule,
		ToolName:    toolName,
		ToolArgs:    toolArgs,
		NextRun:     nextRun,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	log.Printf("⏰ [CRON] Task criada: %s — %s (próximo: %s)", task.ID, description, nextRun.Format("15:04"))
	return task, nil
}

// ListTasks lista tarefas de um idoso
func (s *Service) ListTasks(idosoID int64) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.IdosoID == idosoID && t.Enabled {
			result = append(result, t)
		}
	}
	return result
}

// CancelTask cancela uma tarefa
func (s *Service) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task não encontrada: %s", taskID)
	}

	task.Enabled = false
	delete(s.tasks, taskID)
	log.Printf("⏰ [CRON] Task cancelada: %s", taskID)
	return nil
}

// checkAndExecute verifica e executa tasks prontas
func (s *Service) checkAndExecute() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, task := range s.tasks {
		if !task.Enabled || now.Before(task.NextRun) {
			continue
		}

		if s.executor == nil {
			log.Printf("⚠️ [CRON] Executor não configurado, pulando task %s", task.ID)
			continue
		}

		// Executar em goroutine
		go func(t *Task) {
			log.Printf("⏰ [CRON] Executando task %s: %s → %s", t.ID, t.Description, t.ToolName)
			result, err := s.executor(t.ToolName, t.ToolArgs, t.IdosoID)
			if err != nil {
				log.Printf("❌ [CRON] Task %s falhou: %v", t.ID, err)
			} else {
				log.Printf("✅ [CRON] Task %s concluída: %v", t.ID, result)
			}
		}(task)

		// Atualizar próxima execução
		task.LastRun = now
		task.RunCount++
		nextRun, err := parseSchedule(task.Schedule, now)
		if err != nil {
			task.Enabled = false
			log.Printf("⚠️ [CRON] Task %s desabilitada (schedule inválido)", task.ID)
		} else {
			task.NextRun = nextRun
		}
	}
}

// parseSchedule interpreta schedule string e retorna próxima execução
// Formatos: "every 5m", "every 1h", "every 30s", "daily 08:00", "weekly mon 09:00", "hourly"
func parseSchedule(schedule string, after time.Time) (time.Time, error) {
	schedule = strings.TrimSpace(strings.ToLower(schedule))

	if schedule == "hourly" {
		return after.Add(1 * time.Hour), nil
	}

	if strings.HasPrefix(schedule, "every ") {
		durStr := strings.TrimPrefix(schedule, "every ")
		dur, err := parseSimpleDuration(durStr)
		if err != nil {
			return time.Time{}, err
		}
		if dur < 10*time.Second {
			return time.Time{}, fmt.Errorf("intervalo mínimo: 10s")
		}
		return after.Add(dur), nil
	}

	if strings.HasPrefix(schedule, "daily ") {
		timeStr := strings.TrimPrefix(schedule, "daily ")
		parts := strings.Split(timeStr, ":")
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("formato: daily HH:MM")
		}
		hour, err1 := strconv.Atoi(parts[0])
		min, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return time.Time{}, fmt.Errorf("hora inválida: %s", timeStr)
		}

		next := time.Date(after.Year(), after.Month(), after.Day(), hour, min, 0, 0, after.Location())
		if next.Before(after) || next.Equal(after) {
			next = next.AddDate(0, 0, 1)
		}
		return next, nil
	}

	if strings.HasPrefix(schedule, "weekly ") {
		parts := strings.Fields(strings.TrimPrefix(schedule, "weekly "))
		if len(parts) < 2 {
			return time.Time{}, fmt.Errorf("formato: weekly day HH:MM")
		}
		dayMap := map[string]time.Weekday{
			"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
			"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
		}
		targetDay, ok := dayMap[parts[0]]
		if !ok {
			return time.Time{}, fmt.Errorf("dia inválido: %s", parts[0])
		}

		timeParts := strings.Split(parts[1], ":")
		if len(timeParts) != 2 {
			return time.Time{}, fmt.Errorf("formato hora: HH:MM")
		}
		hour, _ := strconv.Atoi(timeParts[0])
		min, _ := strconv.Atoi(timeParts[1])

		next := time.Date(after.Year(), after.Month(), after.Day(), hour, min, 0, 0, after.Location())
		for next.Weekday() != targetDay || next.Before(after) || next.Equal(after) {
			next = next.AddDate(0, 0, 1)
		}
		return next, nil
	}

	return time.Time{}, fmt.Errorf("schedule não reconhecido: '%s' (use 'every 5m', 'daily 08:00', 'weekly mon 09:00', 'hourly')", schedule)
}

func parseSimpleDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("duração vazia")
	}

	// Tentar parse padrão Go ("5m", "1h30m", "30s")
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Tentar formatos simples: "5 minutes", "1 hour"
	parts := strings.Fields(s)
	if len(parts) == 2 {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("número inválido: %s", parts[0])
		}
		switch strings.ToLower(parts[1]) {
		case "s", "sec", "second", "seconds", "segundo", "segundos":
			return time.Duration(n) * time.Second, nil
		case "m", "min", "minute", "minutes", "minuto", "minutos":
			return time.Duration(n) * time.Minute, nil
		case "h", "hour", "hours", "hora", "horas":
			return time.Duration(n) * time.Hour, nil
		}
	}

	return 0, fmt.Errorf("duração não reconhecida: %s", s)
}
