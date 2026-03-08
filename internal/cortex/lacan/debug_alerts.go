// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// AlertSystem gerencia alertas proativos para o Arquiteto
type AlertSystem struct {
	db                 *database.DB
	memoryInvestigator *MemoryInvestigator
	lastCheck          time.Time
	alerts             []Alert
}

// Alert representa um alerta do sistema
type Alert struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"`     // "info", "warning", "critical"
	Category  string    `json:"category"`  // "memoria", "sistema", "paciente", "medicamento"
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// AlertSummary resumo de alertas
type AlertSummary struct {
	TotalAlerts    int      `json:"total_alerts"`
	Critical       int      `json:"critical"`
	Warning        int      `json:"warning"`
	Info           int      `json:"info"`
	Categories     []string `json:"categories"`
	LastCheck      string   `json:"last_check"`
	SystemHealthy  bool     `json:"system_healthy"`
	Alerts         []Alert  `json:"alerts"`
}

// NewAlertSystem cria novo sistema de alertas
func NewAlertSystem(db *database.DB, memInvestigator *MemoryInvestigator) *AlertSystem {
	return &AlertSystem{
		db:                 db,
		memoryInvestigator: memInvestigator,
		alerts:             []Alert{},
	}
}

// ═══════════════════════════════════════════════════════════
// 🔔 VERIFICAÇÃO DE ALERTAS
// ═══════════════════════════════════════════════════════════

// CheckAllAlerts verifica todos os tipos de alertas
func (a *AlertSystem) CheckAllAlerts(ctx context.Context) *AlertSummary {
	a.alerts = []Alert{} // Limpa alertas anteriores
	a.lastCheck = time.Now()

	// Verificar cada categoria
	a.checkMemoryAlerts(ctx)
	a.checkSystemAlerts(ctx)
	a.checkPatientAlerts(ctx)
	a.checkMedicationAlerts(ctx)

	// Contar por nível
	summary := &AlertSummary{
		TotalAlerts:   len(a.alerts),
		LastCheck:     a.lastCheck.Format("02/01/2006 15:04:05"),
		SystemHealthy: true,
		Alerts:        a.alerts,
	}

	categories := make(map[string]bool)
	for _, alert := range a.alerts {
		categories[alert.Category] = true
		switch alert.Level {
		case "critical":
			summary.Critical++
			summary.SystemHealthy = false
		case "warning":
			summary.Warning++
		case "info":
			summary.Info++
		}
	}

	for cat := range categories {
		summary.Categories = append(summary.Categories, cat)
	}

	log.Printf("🔔 [ALERTAS] Verificação completa: %d alertas (%d críticos, %d avisos, %d info)",
		summary.TotalAlerts, summary.Critical, summary.Warning, summary.Info)

	return summary
}

// ═══════════════════════════════════════════════════════════
// 🧠 ALERTAS DE MEMÓRIA
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkMemoryAlerts(ctx context.Context) {
	if a.memoryInvestigator == nil {
		return
	}

	// Verificar integridade
	integrity, err := a.memoryInvestigator.CheckMemoryIntegrity(ctx)
	if err != nil {
		a.addAlert("critical", "memoria", "Erro ao verificar memórias",
			fmt.Sprintf("Falha na verificação de integridade: %v", err), nil)
		return
	}

	// Alertar sobre memórias órfãs
	if integrity.MemoriesOrfas > 0 {
		a.addAlert("warning", "memoria", "Memórias órfãs detectadas",
			fmt.Sprintf("%d memórias sem paciente válido associado", integrity.MemoriesOrfas),
			map[string]interface{}{"count": integrity.MemoriesOrfas})
	}

	// Alertar sobre memórias sem embedding
	if integrity.MemoriasSemEmbedding > 10 {
		a.addAlert("warning", "memoria", "Memórias sem embedding",
			fmt.Sprintf("%d memórias não têm embedding vetorial (busca semântica prejudicada)", integrity.MemoriasSemEmbedding),
			map[string]interface{}{"count": integrity.MemoriasSemEmbedding})
	}

	// Alertar sobre duplicatas
	if integrity.MemoriasDuplicadas > 5 {
		a.addAlert("info", "memoria", "Memórias duplicadas",
			fmt.Sprintf("%d possíveis duplicatas encontradas", integrity.MemoriasDuplicadas),
			map[string]interface{}{"count": integrity.MemoriasDuplicadas})
	}

	// Verificar se há memórias recentes (today)
	today := time.Now()
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	allMemories, err := a.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err == nil {
		recentCount := int64(0)
		for _, row := range allMemories {
			ts := database.GetTime(row, "timestamp")
			if !ts.Before(todayStart) {
				recentCount++
			}
		}
		if recentCount == 0 {
			a.addAlert("info", "memoria", "Sem memórias hoje",
				"Nenhuma memória foi criada hoje. Sistema pode estar ocioso.", nil)
		}
	}
}

// ═══════════════════════════════════════════════════════════
// 🖥️ ALERTAS DE SISTEMA
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkSystemAlerts(ctx context.Context) {
	// Verificar uso de memória
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryMB := m.Alloc / 1024 / 1024
	if memoryMB > 500 {
		a.addAlert("critical", "sistema", "Uso de memória alto",
			fmt.Sprintf("Sistema usando %dMB de RAM (limite: 500MB)", memoryMB),
			map[string]interface{}{"memory_mb": memoryMB})
	} else if memoryMB > 300 {
		a.addAlert("warning", "sistema", "Uso de memória elevado",
			fmt.Sprintf("Sistema usando %dMB de RAM", memoryMB),
			map[string]interface{}{"memory_mb": memoryMB})
	}

	// Verificar goroutines
	goroutines := runtime.NumGoroutine()
	if goroutines > 500 {
		a.addAlert("critical", "sistema", "Muitas goroutines",
			fmt.Sprintf("%d goroutines ativas (possível vazamento)", goroutines),
			map[string]interface{}{"goroutines": goroutines})
	} else if goroutines > 200 {
		a.addAlert("warning", "sistema", "Goroutines elevadas",
			fmt.Sprintf("%d goroutines ativas", goroutines),
			map[string]interface{}{"goroutines": goroutines})
	}

	// NietzscheDB doesn't have a Ping method, so we check by attempting a count query
	_, err := a.db.Count(ctx, "Idoso", "", nil)
	if err != nil {
		a.addAlert("critical", "sistema", "Banco de dados indisponível",
			fmt.Sprintf("Erro de conexão: %v", err), nil)
	}

	// Verificar erros recentes in analysis records
	analyses, err := a.db.QueryByLabel(ctx, "AnaliseGemini", "", nil, 0)
	if err == nil {
		oneHourAgo := time.Now().Add(-1 * time.Hour)
		errorCount := int64(0)
		for _, row := range analyses {
			createdAt := database.GetTime(row, "created_at")
			if createdAt.Before(oneHourAgo) {
				continue
			}
			conteudo := strings.ToLower(database.GetString(row, "conteudo"))
			if strings.Contains(conteudo, "error") {
				errorCount++
			}
		}
		if errorCount > 10 {
			a.addAlert("warning", "sistema", "Muitos erros recentes",
				fmt.Sprintf("%d erros na última hora", errorCount),
				map[string]interface{}{"error_count": errorCount})
		}
	}
}

// ═══════════════════════════════════════════════════════════
// 👤 ALERTAS DE PACIENTES
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkPatientAlerts(ctx context.Context) {
	// Buscar pacientes ativos
	patients, err := a.db.QueryByLabel(ctx, "Idoso", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return
	}

	// Buscar todas as análises
	analyses, _ := a.db.QueryByLabel(ctx, "AnaliseGemini", "", nil, 0)

	// Build map of last interaction per patient
	lastInteraction := make(map[int64]time.Time)
	for _, an := range analyses {
		idosoID := database.GetInt64(an, "idoso_id")
		createdAt := database.GetTime(an, "created_at")
		if last, exists := lastInteraction[idosoID]; !exists || createdAt.After(last) {
			lastInteraction[idosoID] = createdAt
		}
	}

	// Pacientes sem interação recente (mais de 7 dias)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	inactiveCount := int64(0)
	for _, patient := range patients {
		patientID := database.GetInt64(patient, "pg_id")
		if patientID == 0 {
			patientID = database.GetInt64(patient, "id")
		}
		last, exists := lastInteraction[patientID]
		if !exists || last.Before(sevenDaysAgo) {
			inactiveCount++
		}
	}

	if inactiveCount > 0 {
		a.addAlert("warning", "paciente", "Pacientes inativos",
			fmt.Sprintf("%d pacientes ativos sem interação há mais de 7 dias", inactiveCount),
			map[string]interface{}{"inactive_count": inactiveCount})
	}

	// Pacientes com muitas emoções negativas recentes
	allMemories, err := a.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return
	}

	negativeEmotions := map[string]bool{
		"triste": true, "ansioso": true, "irritado": true,
		"preocupado": true, "frustrado": true, "deprimido": true,
	}
	threeDaysAgo := time.Now().AddDate(0, 0, -3)

	// Count negative emotions per patient in last 3 days
	negativeByPatient := make(map[int64]int64)
	for _, mem := range allMemories {
		ts := database.GetTime(mem, "timestamp")
		if ts.Before(threeDaysAgo) {
			continue
		}
		emotion := database.GetString(mem, "emotion")
		if negativeEmotions[emotion] {
			idosoID := database.GetInt64(mem, "idoso_id")
			negativeByPatient[idosoID]++
		}
	}

	// Build patient name lookup
	patientNames := make(map[int64]string)
	for _, p := range patients {
		pid := database.GetInt64(p, "pg_id")
		if pid == 0 {
			pid = database.GetInt64(p, "id")
		}
		patientNames[pid] = database.GetString(p, "nome")
	}

	for idosoID, count := range negativeByPatient {
		if count >= 5 {
			nome := patientNames[idosoID]
			if nome == "" {
				nome = fmt.Sprintf("ID %d", idosoID)
			}
			a.addAlert("warning", "paciente", "Paciente com emoções negativas",
				fmt.Sprintf("%s teve %d interações com emoções negativas nos últimos 3 dias", nome, count),
				map[string]interface{}{"idoso_id": idosoID, "nome": nome, "count": count})
		}
	}
}

// ═══════════════════════════════════════════════════════════
// 💊 ALERTAS DE MEDICAMENTOS
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkMedicationAlerts(ctx context.Context) {
	// Buscar agendamentos de medicamentos
	agendamentos, err := a.db.QueryByLabel(ctx, "Agendamento",
		" AND n.tipo = $tipo",
		map[string]interface{}{"tipo": "medicamento"}, 0)
	if err != nil {
		return
	}

	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	twoHoursLater := now.Add(2 * time.Hour)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	missedCount := int64(0)
	upcomingCount := int64(0)

	for _, ag := range agendamentos {
		status := database.GetString(ag, "status")
		dataHora := database.GetTime(ag, "data_hora_agendada")

		// Medicamentos não confirmados (agendados, atrasados 2+ horas, today)
		if status == "agendado" && dataHora.Before(twoHoursAgo) && !dataHora.Before(today) {
			missedCount++
		}

		// Medicamentos para as próximas 2 horas
		if (status == "agendado" || status == "ativo") &&
			!dataHora.Before(now) && dataHora.Before(twoHoursLater) {
			upcomingCount++
		}
	}

	if missedCount > 0 {
		a.addAlert("critical", "medicamento", "Medicamentos não confirmados",
			fmt.Sprintf("%d medicamentos agendados não foram confirmados (atrasados 2+ horas)", missedCount),
			map[string]interface{}{"missed_count": missedCount})
	}

	if upcomingCount > 0 {
		a.addAlert("info", "medicamento", "Medicamentos próximos",
			fmt.Sprintf("%d medicamentos agendados para as próximas 2 horas", upcomingCount),
			map[string]interface{}{"upcoming_count": upcomingCount})
	}

	// Pacientes sem medicamentos cadastrados (mas ativos)
	patients, err := a.db.QueryByLabel(ctx, "Idoso", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return
	}

	// Build set of patient IDs that have medication agendamentos
	patientsWithMeds := make(map[int64]bool)
	for _, ag := range agendamentos {
		idosoID := database.GetInt64(ag, "idoso_id")
		patientsWithMeds[idosoID] = true
	}

	noMedCount := int64(0)
	for _, patient := range patients {
		patientID := database.GetInt64(patient, "pg_id")
		if patientID == 0 {
			patientID = database.GetInt64(patient, "id")
		}
		if !patientsWithMeds[patientID] {
			noMedCount++
		}
	}

	if noMedCount > 0 {
		a.addAlert("info", "medicamento", "Pacientes sem medicamentos",
			fmt.Sprintf("%d pacientes ativos não têm medicamentos cadastrados", noMedCount),
			map[string]interface{}{"count": noMedCount})
	}
}

// ═══════════════════════════════════════════════════════════
// 🔧 HELPERS
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) addAlert(level, category, title, message string, data map[string]interface{}) {
	alert := Alert{
		ID:        fmt.Sprintf("%s_%s_%d", category, level, time.Now().UnixNano()),
		Level:     level,
		Category:  category,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
		Resolved:  false,
		Data:      data,
	}
	a.alerts = append(a.alerts, alert)
	log.Printf("🔔 [ALERTA %s] %s: %s", strings.ToUpper(level), title, message)
}

// GetAlerts retorna alertas atuais
func (a *AlertSystem) GetAlerts() []Alert {
	return a.alerts
}

// GetCriticalAlerts retorna apenas alertas críticos
func (a *AlertSystem) GetCriticalAlerts() []Alert {
	var critical []Alert
	for _, alert := range a.alerts {
		if alert.Level == "critical" {
			critical = append(critical, alert)
		}
	}
	return critical
}

// FormatAlertsForSpeech formata alertas para fala da EVA
func (a *AlertSystem) FormatAlertsForSpeech(summary *AlertSummary) string {
	var builder strings.Builder

	if summary.TotalAlerts == 0 {
		builder.WriteString("Está tudo tranquilo! Não encontrei nenhum alerta no sistema.\n")
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("Encontrei %d alertas no sistema.\n\n", summary.TotalAlerts))

	if summary.Critical > 0 {
		builder.WriteString(fmt.Sprintf("⚠️ CRÍTICOS: %d\n", summary.Critical))
		for _, alert := range summary.Alerts {
			if alert.Level == "critical" {
				builder.WriteString(fmt.Sprintf("  🔴 %s: %s\n", alert.Title, alert.Message))
			}
		}
		builder.WriteString("\n")
	}

	if summary.Warning > 0 {
		builder.WriteString(fmt.Sprintf("⚠️ AVISOS: %d\n", summary.Warning))
		for _, alert := range summary.Alerts {
			if alert.Level == "warning" {
				builder.WriteString(fmt.Sprintf("  🟡 %s: %s\n", alert.Title, alert.Message))
			}
		}
		builder.WriteString("\n")
	}

	if summary.Info > 0 {
		builder.WriteString(fmt.Sprintf("ℹ️ INFORMAÇÕES: %d\n", summary.Info))
		for _, alert := range summary.Alerts {
			if alert.Level == "info" {
				builder.WriteString(fmt.Sprintf("  🔵 %s\n", alert.Title))
			}
		}
	}

	return builder.String()
}

// BuildAlertSection constrói seção de alertas para o prompt do criador
func (a *AlertSystem) BuildAlertSection(ctx context.Context) string {
	summary := a.CheckAllAlerts(ctx)

	if summary.TotalAlerts == 0 {
		return "" // Sem alertas, não adiciona seção
	}

	var builder strings.Builder

	builder.WriteString("⚠️ ALERTAS DO SISTEMA:\n")

	if summary.Critical > 0 {
		builder.WriteString(fmt.Sprintf("  🔴 %d alertas CRÍTICOS\n", summary.Critical))
	}
	if summary.Warning > 0 {
		builder.WriteString(fmt.Sprintf("  🟡 %d avisos\n", summary.Warning))
	}
	if summary.Info > 0 {
		builder.WriteString(fmt.Sprintf("  🔵 %d informações\n", summary.Info))
	}

	// Mostrar críticos no prompt
	for _, alert := range summary.Alerts {
		if alert.Level == "critical" {
			builder.WriteString(fmt.Sprintf("  → %s\n", alert.Title))
		}
	}

	builder.WriteString("\n")
	return builder.String()
}
