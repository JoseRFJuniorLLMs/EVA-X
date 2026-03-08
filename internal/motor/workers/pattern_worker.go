// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// PatternWorker detecta padrões comportamentais
type PatternWorker struct {
	db *database.DB
}

// NewPatternWorker cria um novo worker de padrões
func NewPatternWorker(db *database.DB) *PatternWorker {
	return &PatternWorker{db: db}
}

// Name retorna o nome do worker
func (pw *PatternWorker) Name() string {
	return "Pattern Detector"
}

// Interval retorna o intervalo de execução (6 horas)
func (pw *PatternWorker) Interval() time.Duration {
	return 6 * time.Hour
}

// Run executa a detecção de padrões
func (pw *PatternWorker) Run(ctx context.Context) error {
	log.Println("Iniciando deteccao de padroes comportamentais...")

	// Buscar todos os idosos ativos
	idosos, err := pw.getActiveIdosos(ctx)
	if err != nil {
		return fmt.Errorf("erro ao buscar idosos: %w", err)
	}

	log.Printf("Analisando padroes para %d idoso(s)...", len(idosos))

	totalPadroes := 0
	for _, idosoID := range idosos {
		// Detectar padrões de sono
		if pattern, err := pw.detectSleepPattern(ctx, idosoID); err == nil && pattern != nil {
			if err := pw.savePattern(ctx, pattern); err == nil {
				totalPadroes++
			}
		}

		// Detectar padrões de humor
		if pattern, err := pw.detectMoodPattern(ctx, idosoID); err == nil && pattern != nil {
			if err := pw.savePattern(ctx, pattern); err == nil {
				totalPadroes++
			}
		}

		// Detectar padrões de medicação
		if pattern, err := pw.detectMedicationPattern(ctx, idosoID); err == nil && pattern != nil {
			if err := pw.savePattern(ctx, pattern); err == nil {
				totalPadroes++
			}
		}
	}

	log.Printf("Deteccao concluida: %d padrao(oes) detectado(s)", totalPadroes)
	return nil
}

// getActiveIdosos retorna lista de idosos ativos
func (pw *PatternWorker) getActiveIdosos(ctx context.Context) ([]int, error) {
	rows, err := pw.db.QueryByLabel(ctx, "idosos",
		" AND n.ativo = $ativo",
		map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return nil, err
	}

	var idosos []int
	for _, m := range rows {
		id := int(database.GetInt64(m, "id"))
		if id > 0 {
			idosos = append(idosos, id)
		}
	}

	return idosos, nil
}

// BehaviorPattern representa um padrão comportamental
type BehaviorPattern struct {
	IdosoID           int
	TipoPadrao        string
	Descricao         string
	Frequencia        string
	Confianca         float64
	DadosEstatisticos map[string]interface{}
}

// detectSleepPattern detecta padrões de sono
func (pw *PatternWorker) detectSleepPattern(ctx context.Context, idosoID int) (*BehaviorPattern, error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)

	rows, err := pw.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso AND n.inicio_chamada > $since AND n.tarefa_concluida = $concluida",
		map[string]interface{}{"idoso": idosoID, "since": thirtyDaysAgo, "concluida": true}, 0)
	if err != nil {
		return nil, err
	}

	horariosAtivos := make(map[int]int)
	totalLigacoes := 0

	for _, m := range rows {
		t := database.GetTime(m, "inicio_chamada")
		if !t.IsZero() {
			hora := t.Hour()
			horariosAtivos[hora]++
			totalLigacoes++
		}
	}

	if totalLigacoes < 10 {
		return nil, nil // Dados insuficientes
	}

	// Detectar horários de baixa atividade (sono)
	horasSono := []int{}
	mediaAtividade := float64(totalLigacoes) / 24.0

	for hora := 0; hora < 24; hora++ {
		if float64(horariosAtivos[hora]) < mediaAtividade*0.3 {
			horasSono = append(horasSono, hora)
		}
	}

	if len(horasSono) >= 6 {
		// Encontrar intervalo contínuo de sono
		inicio, fim := pw.findContinuousInterval(horasSono)

		pattern := &BehaviorPattern{
			IdosoID:    idosoID,
			TipoPadrao: "horario_sono",
			Descricao:  fmt.Sprintf("Padrao de sono detectado: %02d:00 - %02d:00", inicio, fim),
			Frequencia: "diario",
			Confianca:  0.85,
			DadosEstatisticos: map[string]interface{}{
				"hora_inicio":           inicio,
				"hora_fim":              fim,
				"horas_sono":            len(horasSono),
				"total_dias_analisados": 30,
				"total_ligacoes":        totalLigacoes,
			},
		}

		return pattern, nil
	}

	return nil, nil
}

// detectMoodPattern detecta padrões de humor
func (pw *PatternWorker) detectMoodPattern(ctx context.Context, idosoID int) (*BehaviorPattern, error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)

	rows, err := pw.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso AND n.inicio_chamada > $since",
		map[string]interface{}{"idoso": idosoID, "since": thirtyDaysAgo}, 0)
	if err != nil {
		return nil, err
	}

	// Count sentimentos
	sentimentoCounts := make(map[string]int)
	sentimentoIntensidade := make(map[string]float64)

	for _, m := range rows {
		sentimento := database.GetString(m, "sentimento_geral")
		if sentimento != "" {
			sentimentoCounts[sentimento]++
			sentimentoIntensidade[sentimento] += database.GetFloat64(m, "sentimento_intensidade")
		}
	}

	// Find dominant sentiment
	var bestSentimento string
	var bestTotal int
	for s, count := range sentimentoCounts {
		if count > bestTotal {
			bestTotal = count
			bestSentimento = s
		}
	}

	if bestTotal >= 10 {
		intensidadeMedia := sentimentoIntensidade[bestSentimento] / float64(bestTotal)
		confianca := float64(bestTotal) / 30.0
		if confianca > 1.0 {
			confianca = 1.0
		}

		pattern := &BehaviorPattern{
			IdosoID:    idosoID,
			TipoPadrao: "humor_recorrente",
			Descricao:  fmt.Sprintf("Humor predominante: %s (%d ocorrencias em 30 dias)", bestSentimento, bestTotal),
			Frequencia: "semanal",
			Confianca:  confianca,
			DadosEstatisticos: map[string]interface{}{
				"sentimento_predominante": bestSentimento,
				"ocorrencias":             bestTotal,
				"intensidade_media":       intensidadeMedia,
				"dias_analisados":         30,
			},
		}

		return pattern, nil
	}

	return nil, nil
}

// detectMedicationPattern detecta padrões de adesão à medicação
func (pw *PatternWorker) detectMedicationPattern(ctx context.Context, idosoID int) (*BehaviorPattern, error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)

	rows, err := pw.db.QueryByLabel(ctx, "agendamentos",
		" AND n.idoso_id = $idoso AND n.tipo = $tipo AND n.data_hora_agendada > $since",
		map[string]interface{}{"idoso": idosoID, "tipo": "lembrete_medicamento", "since": thirtyDaysAgo}, 0)
	if err != nil {
		return nil, err
	}

	totalAgendamentos := len(rows)
	if totalAgendamentos < 10 {
		return nil, nil
	}

	medicamentosTomados := 0
	for _, m := range rows {
		if database.GetBool(m, "medicamento_tomado") {
			medicamentosTomados++
		}
	}

	taxaAdesao := float64(medicamentosTomados) / float64(totalAgendamentos)

	var descricao string
	if taxaAdesao >= 0.9 {
		descricao = fmt.Sprintf("Excelente adesao a medicacao: %.0f%%", taxaAdesao*100)
	} else if taxaAdesao >= 0.7 {
		descricao = fmt.Sprintf("Boa adesao a medicacao: %.0f%%", taxaAdesao*100)
	} else if taxaAdesao >= 0.5 {
		descricao = fmt.Sprintf("Adesao moderada a medicacao: %.0f%%", taxaAdesao*100)
	} else {
		descricao = fmt.Sprintf("Baixa adesao a medicacao: %.0f%% - ATENCAO", taxaAdesao*100)
	}

	pattern := &BehaviorPattern{
		IdosoID:    idosoID,
		TipoPadrao: "medicacao_adesao",
		Descricao:  descricao,
		Frequencia: "diario",
		Confianca:  0.90,
		DadosEstatisticos: map[string]interface{}{
			"total_agendamentos":   totalAgendamentos,
			"medicamentos_tomados": medicamentosTomados,
			"taxa_adesao":          taxaAdesao,
			"dias_analisados":      30,
		},
	}

	return pattern, nil
}

// savePattern salva padrão no banco
func (pw *PatternWorker) savePattern(ctx context.Context, pattern *BehaviorPattern) error {
	dadosJSON, _ := json.Marshal(pattern.DadosEstatisticos)
	now := time.Now().UTC().Format(time.RFC3339)

	// Try to find existing pattern (upsert on idoso_id + tipo_padrao)
	existing, _ := pw.db.QueryByLabel(ctx, "padroes_comportamento",
		" AND n.idoso_id = $idoso AND n.tipo_padrao = $tipo",
		map[string]interface{}{"idoso": pattern.IdosoID, "tipo": pattern.TipoPadrao}, 1)

	if len(existing) > 0 {
		// Update existing
		ocorrencias := database.GetInt64(existing[0], "ocorrencias") + 1
		err := pw.db.Update(ctx, "padroes_comportamento",
			map[string]interface{}{"idoso_id": pattern.IdosoID, "tipo_padrao": pattern.TipoPadrao},
			map[string]interface{}{
				"descricao":           pattern.Descricao,
				"confianca":           pattern.Confianca,
				"dados_estatisticos":  string(dadosJSON),
				"ultima_confirmacao":  now,
				"ocorrencias":         ocorrencias,
				"atualizado_em":       now,
			})
		if err == nil {
			log.Printf("Padrao '%s' atualizado para idoso %d", pattern.TipoPadrao, pattern.IdosoID)
		}
		return err
	}

	// Insert new
	_, err := pw.db.Insert(ctx, "padroes_comportamento", map[string]interface{}{
		"idoso_id":           pattern.IdosoID,
		"tipo_padrao":        pattern.TipoPadrao,
		"descricao":          pattern.Descricao,
		"frequencia":         pattern.Frequencia,
		"confianca":          pattern.Confianca,
		"dados_estatisticos": string(dadosJSON),
		"ocorrencias":        int64(1),
		"ultima_confirmacao": now,
		"criado_em":          now,
		"atualizado_em":      now,
	})

	if err == nil {
		log.Printf("Padrao '%s' salvo para idoso %d", pattern.TipoPadrao, pattern.IdosoID)
	}

	return err
}

// findContinuousInterval encontra o maior intervalo contínuo em uma lista de horas
func (pw *PatternWorker) findContinuousInterval(horas []int) (int, int) {
	if len(horas) == 0 {
		return 0, 0
	}

	inicio := horas[0]
	fim := horas[0]

	for i := 1; i < len(horas); i++ {
		if horas[i] == horas[i-1]+1 || (horas[i-1] == 23 && horas[i] == 0) {
			fim = horas[i]
		}
	}

	return inicio, fim
}
