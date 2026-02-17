// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// AlertSystem gerencia alertas proativos para o Arquiteto
type AlertSystem struct {
	db                 *sql.DB
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
func NewAlertSystem(db *sql.DB, memInvestigator *MemoryInvestigator) *AlertSystem {
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

	// Verificar se há memórias recentes
	var recentCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE timestamp >= CURRENT_DATE
	`).Scan(&recentCount)

	if recentCount == 0 {
		a.addAlert("info", "memoria", "Sem memórias hoje",
			"Nenhuma memória foi criada hoje. Sistema pode estar ocioso.", nil)
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

	// Verificar conexão com banco
	if err := a.db.PingContext(ctx); err != nil {
		a.addAlert("critical", "sistema", "Banco de dados indisponível",
			fmt.Sprintf("Erro de conexão: %v", err), nil)
	}

	// Verificar erros recentes
	var errorCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM analise_gemini
		WHERE conteudo::text ILIKE '%error%'
		AND created_at >= CURRENT_TIMESTAMP - INTERVAL '1 hour'
	`).Scan(&errorCount)

	if errorCount > 10 {
		a.addAlert("warning", "sistema", "Muitos erros recentes",
			fmt.Sprintf("%d erros na última hora", errorCount),
			map[string]interface{}{"error_count": errorCount})
	}
}

// ═══════════════════════════════════════════════════════════
// 👤 ALERTAS DE PACIENTES
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkPatientAlerts(ctx context.Context) {
	// Pacientes sem interação recente (mais de 7 dias)
	var inactiveCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.id)
		FROM idosos i
		LEFT JOIN (
			SELECT idoso_id, MAX(created_at) as last_interaction
			FROM analise_gemini
			GROUP BY idoso_id
		) ag ON i.id = ag.idoso_id
		WHERE i.ativo = true
		AND (ag.last_interaction IS NULL OR ag.last_interaction < CURRENT_DATE - INTERVAL '7 days')
	`).Scan(&inactiveCount)

	if inactiveCount > 0 {
		a.addAlert("warning", "paciente", "Pacientes inativos",
			fmt.Sprintf("%d pacientes ativos sem interação há mais de 7 dias", inactiveCount),
			map[string]interface{}{"inactive_count": inactiveCount})
	}

	// Pacientes com muitas emoções negativas recentes
	rows, err := a.db.QueryContext(ctx, `
		SELECT i.id, i.nome, COUNT(*) as negative_count
		FROM episodic_memories em
		JOIN idosos i ON em.idoso_id = i.id
		WHERE em.emotion IN ('triste', 'ansioso', 'irritado', 'preocupado', 'frustrado', 'deprimido')
		AND em.timestamp >= CURRENT_DATE - INTERVAL '3 days'
		GROUP BY i.id, i.nome
		HAVING COUNT(*) >= 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var nome string
			var count int64
			if rows.Scan(&id, &nome, &count) == nil {
				a.addAlert("warning", "paciente", "Paciente com emoções negativas",
					fmt.Sprintf("%s teve %d interações com emoções negativas nos últimos 3 dias", nome, count),
					map[string]interface{}{"idoso_id": id, "nome": nome, "count": count})
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════
// 💊 ALERTAS DE MEDICAMENTOS
// ═══════════════════════════════════════════════════════════

func (a *AlertSystem) checkMedicationAlerts(ctx context.Context) {
	// Medicamentos não confirmados nas últimas 24h
	var missedCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agendamentos
		WHERE tipo = 'medicamento'
		AND status = 'agendado'
		AND data_hora_agendada < CURRENT_TIMESTAMP - INTERVAL '2 hours'
		AND data_hora_agendada >= CURRENT_DATE
	`).Scan(&missedCount)

	if missedCount > 0 {
		a.addAlert("critical", "medicamento", "Medicamentos não confirmados",
			fmt.Sprintf("%d medicamentos agendados não foram confirmados (atrasados 2+ horas)", missedCount),
			map[string]interface{}{"missed_count": missedCount})
	}

	// Medicamentos para as próximas 2 horas
	var upcomingCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agendamentos
		WHERE tipo = 'medicamento'
		AND status IN ('agendado', 'ativo')
		AND data_hora_agendada BETWEEN CURRENT_TIMESTAMP AND CURRENT_TIMESTAMP + INTERVAL '2 hours'
	`).Scan(&upcomingCount)

	if upcomingCount > 0 {
		a.addAlert("info", "medicamento", "Medicamentos próximos",
			fmt.Sprintf("%d medicamentos agendados para as próximas 2 horas", upcomingCount),
			map[string]interface{}{"upcoming_count": upcomingCount})
	}

	// Pacientes sem medicamentos cadastrados (mas ativos)
	var noMedCount int64
	a.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.id)
		FROM idosos i
		LEFT JOIN agendamentos ag ON i.id = ag.idoso_id AND ag.tipo = 'medicamento'
		WHERE i.ativo = true AND ag.id IS NULL
	`).Scan(&noMedCount)

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
		builder.WriteString("Arquiteto, está tudo tranquilo! Não encontrei nenhum alerta no sistema.\n")
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("Arquiteto, encontrei %d alertas no sistema.\n\n", summary.TotalAlerts))

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
