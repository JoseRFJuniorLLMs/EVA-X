// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package explainability

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PDFGenerator gera relatórios PDF para médicos
type PDFGenerator struct {
	db       *sql.DB
	explainer *ClinicalDecisionExplainer
}

// PDFReport representa um relatório PDF
type PDFReport struct {
	ID            string
	PatientID     int64
	PatientName   string
	GeneratedAt   time.Time
	ReportType    string // clinical_explanation, weekly_summary, crisis_alert
	Content       []byte
	S3URL         string
	ExpiresAt     time.Time
}

// PatientInfo informações do paciente para relatório
type PatientInfo struct {
	ID        int64
	Name      string
	CPF       string // Parcialmente mascarado
	BirthDate string
	Age       int
	Phone     string
	Doctor    string
}

// NewPDFGenerator cria novo gerador de PDF
func NewPDFGenerator(db *sql.DB, explainer *ClinicalDecisionExplainer) *PDFGenerator {
	return &PDFGenerator{
		db:       db,
		explainer: explainer,
	}
}

// GenerateExplanationPDF gera PDF com explicação clínica
func (pg *PDFGenerator) GenerateExplanationPDF(explanation *Explanation) (*PDFReport, error) {
	log.Printf("📄 [PDF] Gerando relatório para explicação %s", explanation.ID)

	// 1. Buscar dados do paciente
	patient, err := pg.getPatientInfo(explanation.PatientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar paciente: %w", err)
	}

	// 2. Gerar conteúdo HTML/Markdown para conversão
	content := pg.generateHTMLContent(explanation, patient)

	// 3. Converter para PDF (usando biblioteca externa ou serviço)
	pdfBytes, err := pg.htmlToPDF(content)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar PDF: %w", err)
	}

	// 4. Fazer upload para S3
	s3URL, err := pg.uploadToS3(pdfBytes, explanation.ID)
	if err != nil {
		log.Printf("⚠️ [PDF] Erro ao fazer upload para S3: %v", err)
		s3URL = "" // Continua mesmo sem S3
	}

	// 5. Salvar referência no banco
	report := &PDFReport{
		PatientID:   explanation.PatientID,
		PatientName: patient.Name,
		GeneratedAt: time.Now(),
		ReportType:  "clinical_explanation",
		Content:     pdfBytes,
		S3URL:       s3URL,
		ExpiresAt:   time.Now().AddDate(0, 0, 90), // 90 dias
	}

	err = pg.saveReport(report, explanation.ID)
	if err != nil {
		return nil, fmt.Errorf("erro ao salvar relatório: %w", err)
	}

	log.Printf("✅ [PDF] Relatório gerado: %s", report.ID)

	return report, nil
}

// GenerateWeeklySummaryPDF gera resumo semanal do paciente
func (pg *PDFGenerator) GenerateWeeklySummaryPDF(patientID int64) (*PDFReport, error) {
	log.Printf("📄 [PDF] Gerando resumo semanal para paciente %d", patientID)

	// 1. Buscar dados do paciente
	patient, err := pg.getPatientInfo(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar paciente: %w", err)
	}

	// 2. Buscar métricas da semana
	metrics := pg.getWeeklyMetrics(patientID)

	// 3. Buscar alertas da semana
	alerts := pg.getWeeklyAlerts(patientID)

	// 4. Gerar conteúdo
	content := pg.generateWeeklySummaryHTML(patient, metrics, alerts)

	// 5. Converter para PDF
	pdfBytes, err := pg.htmlToPDF(content)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar PDF: %w", err)
	}

	// 6. Upload e salvar
	s3URL, _ := pg.uploadToS3(pdfBytes, fmt.Sprintf("weekly-%d-%s", patientID, time.Now().Format("2006-01-02")))

	report := &PDFReport{
		PatientID:   patientID,
		PatientName: patient.Name,
		GeneratedAt: time.Now(),
		ReportType:  "weekly_summary",
		Content:     pdfBytes,
		S3URL:       s3URL,
		ExpiresAt:   time.Now().AddDate(0, 0, 90),
	}

	err = pg.saveWeeklyReport(report)
	if err != nil {
		return nil, err
	}

	return report, nil
}

// generateHTMLContent gera conteúdo HTML para conversão em PDF
func (pg *PDFGenerator) generateHTMLContent(explanation *Explanation, patient *PatientInfo) string {
	var buf bytes.Buffer

	// Header
	buf.WriteString(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; color: #333; }
        .header { border-bottom: 2px solid #0066cc; padding-bottom: 20px; margin-bottom: 30px; }
        .logo { color: #0066cc; font-size: 24px; font-weight: bold; }
        .patient-info { background: #f5f5f5; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .alert-box { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; }
        .critical { background: #f8d7da; border-left-color: #dc3545; }
        .factor { margin: 15px 0; padding: 10px; background: #f8f9fa; }
        .factor-name { font-weight: bold; color: #0066cc; }
        .recommendation { background: #d4edda; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; font-size: 12px; color: #666; }
        .confidential { color: red; font-weight: bold; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background: #0066cc; color: white; }
    </style>
</head>
<body>`)

	// Header com logo e info
	buf.WriteString(`
    <div class="header">
        <div class="logo">EVA Healthcare</div>
        <div style="float: right; text-align: right;">
            <strong>RELATÓRIO CLÍNICO EXPLICATIVO</strong><br>
            <small>Gerado em: ` + time.Now().Format("02/01/2006 15:04") + `</small>
        </div>
        <div style="clear: both;"></div>
    </div>`)

	// Info do paciente
	buf.WriteString(fmt.Sprintf(`
    <div class="patient-info">
        <strong>Paciente:</strong> %s<br>
        <strong>Idade:</strong> %d anos<br>
        <strong>Médico Responsável:</strong> %s<br>
        <strong>ID do Relatório:</strong> %s
    </div>`, patient.Name, patient.Age, patient.Doctor, explanation.ID))

	// Alerta principal
	alertClass := "alert-box"
	if explanation.Severity == "critical" || explanation.Severity == "high" {
		alertClass = "alert-box critical"
	}

	buf.WriteString(fmt.Sprintf(`
    <div class="%s">
        <strong>%s</strong><br>
        Probabilidade: %.0f%% | Severidade: %s | Janela: %s
    </div>`,
		alertClass,
		pg.translateDecisionType(explanation.DecisionType),
		explanation.PredictionScore*100,
		pg.translateSeverity(explanation.Severity),
		explanation.Timeframe))

	// Fatores principais
	buf.WriteString(`<h2>Fatores Principais</h2>`)
	for i, factor := range explanation.PrimaryFactors {
		buf.WriteString(fmt.Sprintf(`
        <div class="factor">
            <span class="factor-name">%d. %s</span> (contribuição: %.0f%%)<br>
            Status: %s<br>
            %s
        </div>`, i+1, factor.Factor, factor.Contribution*100, factor.Status, factor.HumanReadable))
	}

	// Fatores secundários
	if len(explanation.SecondaryFactors) > 0 {
		buf.WriteString(`<h2>Fatores Secundários</h2><ul>`)
		for _, factor := range explanation.SecondaryFactors {
			buf.WriteString(fmt.Sprintf(`<li><strong>%s</strong>: %s</li>`, factor.Factor, factor.HumanReadable))
		}
		buf.WriteString(`</ul>`)
	}

	// Recomendações
	buf.WriteString(`<h2>Recomendações</h2>`)
	for _, rec := range explanation.Recommendations {
		buf.WriteString(fmt.Sprintf(`
        <div class="recommendation">
            <strong>[%s] %s</strong><br>
            <em>Justificativa:</em> %s<br>
            <em>Prazo:</em> %s
        </div>`, pg.translateUrgency(rec.Urgency), rec.Action, rec.Rationale, rec.Timeframe))
	}

	// Footer
	buf.WriteString(`
    <div class="footer">
        <p class="confidential">DOCUMENTO CONFIDENCIAL - USO EXCLUSIVO PROFISSIONAL DE SAÚDE</p>
        <p>Este relatório foi gerado automaticamente pelo sistema EVA Healthcare utilizando algoritmos de Inteligência Artificial Explicável (XAI).
        As predições são baseadas em análise de múltiplas variáveis clínicas e comportamentais, e devem ser interpretadas em conjunto com avaliação clínica presencial.</p>
        <p>Modelo: v1.0.0 | LGPD Compliant | ID: ` + explanation.ID + `</p>
    </div>
</body>
</html>`)

	return buf.String()
}

// generateWeeklySummaryHTML gera HTML do resumo semanal
func (pg *PDFGenerator) generateWeeklySummaryHTML(patient *PatientInfo, metrics map[string]interface{}, alerts []map[string]interface{}) string {
	var buf bytes.Buffer

	buf.WriteString(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { border-bottom: 2px solid #0066cc; padding-bottom: 20px; }
        .metric { display: inline-block; width: 30%; text-align: center; padding: 15px; margin: 5px; background: #f5f5f5; }
        .metric-value { font-size: 24px; font-weight: bold; color: #0066cc; }
        .metric-label { font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Resumo Semanal - EVA Healthcare</h1>
        <p><strong>Paciente:</strong> ` + patient.Name + `</p>
        <p><strong>Período:</strong> ` + time.Now().AddDate(0, 0, -7).Format("02/01/2006") + ` a ` + time.Now().Format("02/01/2006") + `</p>
    </div>`)

	// Métricas
	buf.WriteString(`<h2>Indicadores da Semana</h2><div>`)

	if interactions, ok := metrics["interactions_count"].(int); ok {
		buf.WriteString(fmt.Sprintf(`<div class="metric"><div class="metric-value">%d</div><div class="metric-label">Interações</div></div>`, interactions))
	}

	if avgMood, ok := metrics["avg_mood"].(float64); ok {
		buf.WriteString(fmt.Sprintf(`<div class="metric"><div class="metric-value">%.1f</div><div class="metric-label">Humor Médio</div></div>`, avgMood))
	}

	if adherence, ok := metrics["medication_adherence"].(float64); ok {
		buf.WriteString(fmt.Sprintf(`<div class="metric"><div class="metric-value">%.0f%%</div><div class="metric-label">Adesão Medicamentosa</div></div>`, adherence*100))
	}

	buf.WriteString(`</div>`)

	// Alertas
	if len(alerts) > 0 {
		buf.WriteString(`<h2>Alertas da Semana</h2><ul>`)
		for _, alert := range alerts {
			buf.WriteString(fmt.Sprintf(`<li><strong>%s</strong> - %s (%s)</li>`,
				alert["type"], alert["description"], alert["date"]))
		}
		buf.WriteString(`</ul>`)
	} else {
		buf.WriteString(`<p>Nenhum alerta significativo nesta semana.</p>`)
	}

	buf.WriteString(`</body></html>`)

	return buf.String()
}

// htmlToPDF converte HTML para PDF usando wkhtmltopdf.
// Se wkhtmltopdf não estiver disponível, retorna o HTML como bytes (fallback).
func (pg *PDFGenerator) htmlToPDF(htmlContent string) ([]byte, error) {
	// Verificar se wkhtmltopdf está disponível no PATH
	wkhtmltopdfPath, err := exec.LookPath("wkhtmltopdf")
	if err != nil {
		log.Printf("⚠️ [PDF] wkhtmltopdf não encontrado, usando fallback HTML: %v", err)
		return []byte(htmlContent), nil
	}

	// Criar diretório temporário para os arquivos
	tmpDir, err := os.MkdirTemp("", "eva-pdf-*")
	if err != nil {
		log.Printf("⚠️ [PDF] Erro ao criar diretório temporário, usando fallback HTML: %v", err)
		return []byte(htmlContent), nil
	}
	defer os.RemoveAll(tmpDir)

	// Escrever HTML em arquivo temporário
	inputPath := filepath.Join(tmpDir, "input.html")
	if err := os.WriteFile(inputPath, []byte(htmlContent), 0644); err != nil {
		log.Printf("⚠️ [PDF] Erro ao escrever HTML temporário, usando fallback HTML: %v", err)
		return []byte(htmlContent), nil
	}

	// Definir caminho do PDF de saída
	outputPath := filepath.Join(tmpDir, "output.pdf")

	// Executar wkhtmltopdf com parâmetros de formatação
	cmd := exec.Command(wkhtmltopdfPath,
		"--page-size", "A4",
		"--margin-top", "15mm",
		"--margin-bottom", "15mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		"--encoding", "utf-8",
		inputPath,
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ [PDF] wkhtmltopdf falhou (stderr: %s), usando fallback HTML: %v", stderr.String(), err)
		return []byte(htmlContent), nil
	}

	// Ler o PDF gerado
	pdfBytes, err := os.ReadFile(outputPath)
	if err != nil {
		log.Printf("⚠️ [PDF] Erro ao ler PDF gerado, usando fallback HTML: %v", err)
		return []byte(htmlContent), nil
	}

	log.Printf("✅ [PDF] PDF gerado com sucesso via wkhtmltopdf (%d bytes)", len(pdfBytes))
	return pdfBytes, nil
}

// uploadToGCS faz upload do PDF para Google Cloud Storage.
// Se GCS_BUCKET não estiver configurado, salva localmente como fallback.
func (pg *PDFGenerator) uploadToS3(pdfBytes []byte, filename string) (string, error) {
	bucket := os.Getenv("GCS_BUCKET")

	if bucket == "" {
		// Fallback: salvar localmente
		localDir := filepath.Join(os.TempDir(), "eva-reports")
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return "", fmt.Errorf("erro ao criar diretório local: %w", err)
		}

		localPath := filepath.Join(localDir, filename+".pdf")
		if err := os.WriteFile(localPath, pdfBytes, 0644); err != nil {
			return "", fmt.Errorf("erro ao salvar PDF localmente: %w", err)
		}

		log.Printf("📁 [PDF] GCS_BUCKET não configurado, salvo localmente: %s", localPath)
		return "file://" + localPath, nil
	}

	// Upload para Google Cloud Storage via gsutil
	gsutilPath, err := exec.LookPath("gsutil")
	if err != nil {
		// Fallback se gsutil não está disponível: salvar localmente
		localDir := filepath.Join(os.TempDir(), "eva-reports")
		os.MkdirAll(localDir, 0755)
		localPath := filepath.Join(localDir, filename+".pdf")
		if writeErr := os.WriteFile(localPath, pdfBytes, 0644); writeErr != nil {
			return "", fmt.Errorf("gsutil não encontrado e falha ao salvar localmente: %w", writeErr)
		}
		log.Printf("⚠️ [PDF] gsutil não encontrado, salvo localmente: %s", localPath)
		return "file://" + localPath, nil
	}

	// Escrever PDF em arquivo temporário para upload
	tmpFile, err := os.CreateTemp("", "eva-upload-*.pdf")
	if err != nil {
		return "", fmt.Errorf("erro ao criar arquivo temporário para upload: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(pdfBytes); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("erro ao escrever arquivo temporário: %w", err)
	}
	tmpFile.Close()

	// Caminho no GCS
	gcsPath := fmt.Sprintf("gs://%s/reports/%s.pdf", bucket, filename)

	// Upload via gsutil
	cmd := exec.Command(gsutilPath, "cp", tmpFile.Name(), gcsPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("erro ao fazer upload para GCS (stderr: %s): %w", stderr.String(), err)
	}

	// URL pública (ou autenticada) do GCS
	gcsURL := fmt.Sprintf("https://storage.googleapis.com/%s/reports/%s.pdf", bucket, filename)
	log.Printf("☁️ [PDF] Upload para GCS concluído: %s", gcsURL)

	return gcsURL, nil
}

// getPatientInfo busca informações do paciente
func (pg *PDFGenerator) getPatientInfo(patientID int64) (*PatientInfo, error) {
	query := `
		SELECT id, nome, cpf, data_nascimento, telefone
		FROM idosos
		WHERE id = $1
	`

	var patient PatientInfo
	var birthDate time.Time

	err := pg.db.QueryRow(query, patientID).Scan(
		&patient.ID,
		&patient.Name,
		&patient.CPF,
		&birthDate,
		&patient.Phone,
	)

	if err != nil {
		return nil, err
	}

	// Calcular idade
	patient.Age = int(time.Since(birthDate).Hours() / 24 / 365)

	// Mascarar CPF
	if len(patient.CPF) >= 11 {
		patient.CPF = patient.CPF[:3] + ".***.**" + patient.CPF[len(patient.CPF)-2:]
	}

	// Buscar médico responsável
	patient.Doctor = pg.getResponsibleDoctor(patientID)

	return &patient, nil
}

// getResponsibleDoctor busca médico responsável
func (pg *PDFGenerator) getResponsibleDoctor(patientID int64) string {
	query := `SELECT nome FROM medicos WHERE id = (SELECT medico_id FROM idosos WHERE id = $1)`

	var doctor string
	err := pg.db.QueryRow(query, patientID).Scan(&doctor)
	if err != nil {
		return "Não atribuído"
	}
	return doctor
}

// getWeeklyMetrics busca métricas da semana
func (pg *PDFGenerator) getWeeklyMetrics(patientID int64) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Contar interações
	var count int
	pg.db.QueryRow(`
		SELECT COUNT(*) FROM interaction_cognitive_load
		WHERE patient_id = $1 AND timestamp > NOW() - INTERVAL '7 days'
	`, patientID).Scan(&count)
	metrics["interactions_count"] = count

	// Humor médio (placeholder)
	metrics["avg_mood"] = 6.5

	// Adesão medicamentosa (placeholder)
	metrics["medication_adherence"] = 0.78

	return metrics
}

// getWeeklyAlerts busca alertas da semana
func (pg *PDFGenerator) getWeeklyAlerts(patientID int64) []map[string]interface{} {
	var alerts []map[string]interface{}

	rows, err := pg.db.Query(`
		SELECT decision_type, severity, created_at
		FROM clinical_decision_explanations
		WHERE patient_id = $1
		  AND created_at > NOW() - INTERVAL '7 days'
		  AND severity IN ('high', 'critical')
		ORDER BY created_at DESC
	`, patientID)

	if err != nil {
		return alerts
	}
	defer rows.Close()

	for rows.Next() {
		var decisionType, severity string
		var createdAt time.Time
		rows.Scan(&decisionType, &severity, &createdAt)

		alerts = append(alerts, map[string]interface{}{
			"type":        pg.translateDecisionType(decisionType),
			"description": pg.translateSeverity(severity),
			"date":        createdAt.Format("02/01 15:04"),
		})
	}

	return alerts
}

// saveReport salva relatório no banco
func (pg *PDFGenerator) saveReport(report *PDFReport, explanationID string) error {
	query := `
		UPDATE clinical_decision_explanations
		SET explanation_pdf_url = $1, report_generated_at = NOW()
		WHERE id = $2
	`

	_, err := pg.db.Exec(query, report.S3URL, explanationID)
	if err != nil {
		return err
	}

	report.ID = explanationID
	return nil
}

// saveWeeklyReport salva relatório semanal
func (pg *PDFGenerator) saveWeeklyReport(report *PDFReport) error {
	// TODO: Criar tabela para relatórios semanais se necessário
	report.ID = fmt.Sprintf("weekly-%d-%s", report.PatientID, time.Now().Format("20060102"))
	return nil
}

// Helpers de tradução
func (pg *PDFGenerator) translateDecisionType(t string) string {
	translations := map[string]string{
		"crisis_prediction":    "Risco de Crise Mental",
		"depression_alert":     "Alerta de Depressão",
		"anxiety_alert":        "Alerta de Ansiedade",
		"medication_alert":     "Alerta de Medicação",
		"suicide_risk":         "Risco de Suicídio",
		"hospitalization_risk": "Risco de Hospitalização",
		"fall_risk":            "Risco de Queda",
	}
	if tr, ok := translations[t]; ok {
		return tr
	}
	return t
}

func (pg *PDFGenerator) translateSeverity(s string) string {
	translations := map[string]string{
		"low":      "Baixo",
		"medium":   "Médio",
		"high":     "Alto",
		"critical": "Crítico",
	}
	if tr, ok := translations[s]; ok {
		return tr
	}
	return s
}

func (pg *PDFGenerator) translateUrgency(u string) string {
	translations := map[string]string{
		"low":      "BAIXA",
		"medium":   "MÉDIA",
		"high":     "ALTA",
		"critical": "CRÍTICA",
	}
	if tr, ok := translations[u]; ok {
		return tr
	}
	return u
}
