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

// PredictionWorker prediz emergências
type PredictionWorker struct {
	db *database.DB
}

// NewPredictionWorker cria um novo worker de predições
func NewPredictionWorker(db *database.DB) *PredictionWorker {
	return &PredictionWorker{db: db}
}

// Name retorna o nome do worker
func (pw *PredictionWorker) Name() string {
	return "Emergency Predictor"
}

// Interval retorna o intervalo de execução (12 horas)
func (pw *PredictionWorker) Interval() time.Duration {
	return 12 * time.Hour
}

// Run executa a predição de emergências
func (pw *PredictionWorker) Run(ctx context.Context) error {
	log.Println("Iniciando predicao de emergencias...")

	// Buscar todos os idosos ativos
	idosos, err := pw.getActiveIdosos(ctx)
	if err != nil {
		return fmt.Errorf("erro ao buscar idosos: %w", err)
	}

	log.Printf("Analisando riscos para %d idoso(s)...", len(idosos))

	totalPredicoes := 0
	for _, idosoID := range idosos {
		// Predizer depressão
		if pred, err := pw.predictDepression(ctx, idosoID); err == nil && pred != nil {
			if err := pw.savePrediction(ctx, pred); err == nil {
				totalPredicoes++
			}
		}

		// Predizer confusão mental
		if pred, err := pw.predictConfusion(ctx, idosoID); err == nil && pred != nil {
			if err := pw.savePrediction(ctx, pred); err == nil {
				totalPredicoes++
			}
		}

		// Predizer risco de queda (baseado em padrões)
		if pred, err := pw.predictFallRisk(ctx, idosoID); err == nil && pred != nil {
			if err := pw.savePrediction(ctx, pred); err == nil {
				totalPredicoes++
			}
		}
	}

	log.Printf("Predicao concluida: %d predicao(oes) gerada(s)", totalPredicoes)
	return nil
}

// getActiveIdosos retorna lista de idosos ativos
func (pw *PredictionWorker) getActiveIdosos(ctx context.Context) ([]int, error) {
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

// EmergencyPrediction representa uma predição de emergência
type EmergencyPrediction struct {
	IdosoID              int
	TipoEmergencia       string
	Probabilidade        float64
	NivelRisco           string
	FatoresContribuintes []string
	SinaisDetectados     map[string]interface{}
	Recomendacoes        []string
}

// predictDepression prediz risco de depressão
func (pw *PredictionWorker) predictDepression(ctx context.Context, idosoID int) (*EmergencyPrediction, error) {
	fourteenDaysAgo := time.Now().AddDate(0, 0, -14).UTC().Format(time.RFC3339)

	rows, err := pw.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso AND n.inicio_chamada > $since",
		map[string]interface{}{"idoso": idosoID, "since": fourteenDaysAgo}, 0)
	if err != nil {
		return nil, err
	}

	total := len(rows)
	if total < 5 {
		return nil, nil // Dados insuficientes
	}

	negativos := 0
	var somaIntensidade float64
	for _, m := range rows {
		sentimento := database.GetString(m, "sentimento_geral")
		if sentimento == "triste" || sentimento == "apatico" {
			negativos++
			somaIntensidade += database.GetFloat64(m, "sentimento_intensidade")
		}
	}

	var intensidade float64
	if negativos > 0 {
		intensidade = somaIntensidade / float64(negativos)
	}

	// Calcular probabilidade
	percentualNegativo := float64(negativos) / float64(total)
	probabilidade := (percentualNegativo * 0.6) + (intensidade / 10.0 * 0.4)

	// Determinar nível de risco
	var nivelRisco string
	switch {
	case probabilidade >= 0.75:
		nivelRisco = "critico"
	case probabilidade >= 0.50:
		nivelRisco = "alto"
	case probabilidade >= 0.30:
		nivelRisco = "medio"
	default:
		nivelRisco = "baixo"
	}

	// Só salvar se risco for médio ou superior
	if probabilidade >= 0.30 {
		prediction := &EmergencyPrediction{
			IdosoID:        idosoID,
			TipoEmergencia: "depressao_severa",
			Probabilidade:  probabilidade,
			NivelRisco:     nivelRisco,
			FatoresContribuintes: []string{
				fmt.Sprintf("%.0f%% de sentimentos negativos nos ultimos 14 dias", percentualNegativo*100),
				fmt.Sprintf("Intensidade media de tristeza: %.1f/10", intensidade),
				fmt.Sprintf("Total de %d ligacoes analisadas", total),
			},
			SinaisDetectados: map[string]interface{}{
				"sentimentos_negativos": negativos,
				"total_ligacoes":        total,
				"percentual_negativo":   percentualNegativo,
				"intensidade_media":     intensidade,
			},
			Recomendacoes: []string{
				"Agendar consulta com psicologo ou psiquiatra",
				"Aumentar frequencia de ligacoes e monitoramento",
				"Notificar familiares sobre mudanca de humor",
				"Avaliar necessidade de suporte emocional adicional",
			},
		}

		return prediction, nil
	}

	return nil, nil
}

// predictConfusion prediz risco de confusão mental
func (pw *PredictionWorker) predictConfusion(ctx context.Context, idosoID int) (*EmergencyPrediction, error) {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)

	rows, err := pw.db.QueryByLabel(ctx, "historico_ligacoes",
		" AND n.idoso_id = $idoso AND n.inicio_chamada > $since",
		map[string]interface{}{"idoso": idosoID, "since": sevenDaysAgo}, 0)
	if err != nil {
		return nil, err
	}

	total := len(rows)
	if total < 3 {
		return nil, nil
	}

	confusao := 0
	var somaIntensidade float64
	for _, m := range rows {
		if database.GetString(m, "sentimento_geral") == "confuso" {
			confusao++
			somaIntensidade += database.GetFloat64(m, "sentimento_intensidade")
		}
	}

	var intensidade float64
	if confusao > 0 {
		intensidade = somaIntensidade / float64(confusao)
	}

	percentualConfusao := float64(confusao) / float64(total)
	probabilidade := (percentualConfusao * 0.7) + (intensidade / 10.0 * 0.3)

	var nivelRisco string
	switch {
	case probabilidade >= 0.60:
		nivelRisco = "critico"
	case probabilidade >= 0.40:
		nivelRisco = "alto"
	case probabilidade >= 0.20:
		nivelRisco = "medio"
	default:
		nivelRisco = "baixo"
	}

	if probabilidade >= 0.20 {
		prediction := &EmergencyPrediction{
			IdosoID:        idosoID,
			TipoEmergencia: "confusao_mental",
			Probabilidade:  probabilidade,
			NivelRisco:     nivelRisco,
			FatoresContribuintes: []string{
				fmt.Sprintf("%d episodio(s) de confusao em 7 dias", confusao),
				fmt.Sprintf("%.0f%% das ligacoes com sinais de confusao", percentualConfusao*100),
			},
			SinaisDetectados: map[string]interface{}{
				"episodios_confusao": confusao,
				"total_ligacoes":     total,
				"percentual":         percentualConfusao,
			},
			Recomendacoes: []string{
				"Avaliacao medica urgente para descartar causas reversiveis",
				"Verificar medicacoes que podem causar confusao",
				"Aumentar supervisao e monitoramento",
				"Considerar avaliacao neurologica",
			},
		}

		return prediction, nil
	}

	return nil, nil
}

// predictFallRisk prediz risco de queda baseado em mobilidade
func (pw *PredictionWorker) predictFallRisk(ctx context.Context, idosoID int) (*EmergencyPrediction, error) {
	// Buscar informações do idoso
	m, err := pw.db.GetNodeByID(ctx, "idosos", idosoID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}

	mobilidade := database.GetString(m, "mobilidade")
	limitacoesVisuais := database.GetBool(m, "limitacoes_visuais")
	limitacoesAuditivas := database.GetBool(m, "limitacoes_auditivas")

	// Contar alertas de queda nos últimos 90 dias
	ninetyDaysAgo := time.Now().AddDate(0, 0, -90).UTC().Format(time.RFC3339)
	alertRows, _ := pw.db.QueryByLabel(ctx, "alertas",
		" AND n.idoso_id = $idoso AND n.tipo = $tipo AND n.criado_em > $since",
		map[string]interface{}{"idoso": idosoID, "tipo": "queda", "since": ninetyDaysAgo}, 0)
	totalAlertasQueda := len(alertRows)

	// Calcular probabilidade baseada em fatores de risco
	probabilidade := 0.0
	fatores := []string{}

	// Mobilidade
	switch mobilidade {
	case "acamado":
		probabilidade += 0.10
		fatores = append(fatores, "Mobilidade: acamado (risco ao transferir)")
	case "cadeira_rodas":
		probabilidade += 0.20
		fatores = append(fatores, "Mobilidade: cadeira de rodas")
	case "auxiliado":
		probabilidade += 0.35
		fatores = append(fatores, "Mobilidade: necessita auxilio")
	case "independente":
		probabilidade += 0.05
	}

	// Limitações sensoriais
	if limitacoesVisuais {
		probabilidade += 0.25
		fatores = append(fatores, "Limitacoes visuais")
	}
	if limitacoesAuditivas {
		probabilidade += 0.10
		fatores = append(fatores, "Limitacoes auditivas")
	}

	// Histórico de quedas
	if totalAlertasQueda > 0 {
		probabilidade += float64(totalAlertasQueda) * 0.15
		fatores = append(fatores, fmt.Sprintf("%d queda(s) nos ultimos 90 dias", totalAlertasQueda))
	}

	// Limitar probabilidade a 1.0
	if probabilidade > 1.0 {
		probabilidade = 1.0
	}

	var nivelRisco string
	switch {
	case probabilidade >= 0.70:
		nivelRisco = "critico"
	case probabilidade >= 0.50:
		nivelRisco = "alto"
	case probabilidade >= 0.30:
		nivelRisco = "medio"
	default:
		nivelRisco = "baixo"
	}

	if probabilidade >= 0.30 {
		prediction := &EmergencyPrediction{
			IdosoID:              idosoID,
			TipoEmergencia:       "queda",
			Probabilidade:        probabilidade,
			NivelRisco:           nivelRisco,
			FatoresContribuintes: fatores,
			SinaisDetectados: map[string]interface{}{
				"mobilidade":           mobilidade,
				"limitacoes_visuais":   limitacoesVisuais,
				"limitacoes_auditivas": limitacoesAuditivas,
				"historico_quedas":     totalAlertasQueda,
			},
			Recomendacoes: []string{
				"Avaliar ambiente domestico para riscos de queda",
				"Considerar uso de dispositivos de auxilio (andador, bengala)",
				"Fisioterapia para fortalecimento e equilibrio",
				"Instalar barras de apoio em banheiro e corredores",
			},
		}

		return prediction, nil
	}

	return nil, nil
}

// savePrediction salva predição no banco
func (pw *PredictionWorker) savePrediction(ctx context.Context, pred *EmergencyPrediction) error {
	fatoresJSON, _ := json.Marshal(pred.FatoresContribuintes)
	sinaisJSON, _ := json.Marshal(pred.SinaisDetectados)
	recomendacoesJSON, _ := json.Marshal(pred.Recomendacoes)

	now := time.Now().UTC().Format(time.RFC3339)
	validadeAte := time.Now().AddDate(0, 0, 7).UTC().Format(time.RFC3339)

	_, err := pw.db.Insert(ctx, "predicoes_emergencia", map[string]interface{}{
		"idoso_id":              pred.IdosoID,
		"tipo_emergencia":       pred.TipoEmergencia,
		"probabilidade":         pred.Probabilidade,
		"nivel_risco":           pred.NivelRisco,
		"fatores_contribuintes": string(fatoresJSON),
		"sinais_detectados":     string(sinaisJSON),
		"recomendacoes":         string(recomendacoesJSON),
		"validade_ate":          validadeAte,
		"criado_em":             now,
	})

	if err == nil {
		log.Printf("Predicao '%s' salva para idoso %d (risco: %s, prob: %.0f%%)",
			pred.TipoEmergencia, pred.IdosoID, pred.NivelRisco, pred.Probabilidade*100)
	}

	return err
}
