package lacan

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// DebugMode gerencia funcionalidades exclusivas para o Arquiteto da Matrix (José R F Junior)
type DebugMode struct {
	db                 *sql.DB
	startTime          time.Time
	metrics            *DebugMetrics
	memoryInvestigator *MemoryInvestigator // Investigador de memórias
	alertSystem        *AlertSystem        // Sistema de alertas proativos
}

// DebugMetrics contém métricas em tempo real do sistema
type DebugMetrics struct {
	// Sistema
	Uptime            string `json:"uptime"`
	MemoryUsageMB     uint64 `json:"memory_usage_mb"`
	NumGoroutines     int    `json:"num_goroutines"`
	GoVersion         string `json:"go_version"`

	// EVA Stats
	TotalConversas    int64  `json:"total_conversas"`
	ConversasHoje     int64  `json:"conversas_hoje"`
	TotalIdosos       int64  `json:"total_idosos"`
	IdososAtivos      int64  `json:"idosos_ativos"`

	// Medicamentos
	TotalMedicamentos int64  `json:"total_medicamentos"`
	MedicamentosHoje  int64  `json:"medicamentos_hoje"`

	// Erros
	ErrosUltimas24h   int64  `json:"erros_ultimas_24h"`
	UltimoErro        string `json:"ultimo_erro"`

	// Análises
	AnalisesPendentes int64  `json:"analises_pendentes"`
	AnalisesHoje      int64  `json:"analises_hoje"`
}

// DebugCommand representa um comando de debug que o Arquiteto pode usar
type DebugCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// DebugResponse é a resposta formatada para o modo debug
type DebugResponse struct {
	Success bool        `json:"success"`
	Command string      `json:"command"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// NewDebugMode cria uma nova instância do modo debug
func NewDebugMode(db *sql.DB) *DebugMode {
	memInvestigator := NewMemoryInvestigator(db)
	return &DebugMode{
		db:                 db,
		startTime:          time.Now(),
		metrics:            &DebugMetrics{},
		memoryInvestigator: memInvestigator,
		alertSystem:        NewAlertSystem(db, memInvestigator),
	}
}

// GetAvailableCommands retorna todos os comandos de debug disponíveis
func (d *DebugMode) GetAvailableCommands() []DebugCommand {
	commands := []DebugCommand{
		// === SISTEMA ===
		{
			Command:     "status",
			Description: "Mostra status geral do sistema EVA",
			Example:     "EVA,me mostra o status",
		},
		{
			Command:     "metricas",
			Description: "Exibe métricas detalhadas em tempo real",
			Example:     "EVA,quero ver as métricas",
		},
		{
			Command:     "logs",
			Description: "Mostra últimos logs do sistema",
			Example:     "EVA,me mostra os logs recentes",
		},
		{
			Command:     "erros",
			Description: "Lista erros recentes e suas causas",
			Example:     "EVA,teve algum erro?",
		},
		{
			Command:     "pacientes",
			Description: "Resumo dos pacientes ativos",
			Example:     "EVA,como estão os pacientes?",
		},
		{
			Command:     "medicamentos",
			Description: "Status dos medicamentos agendados",
			Example:     "EVA,como estão os medicamentos?",
		},
		{
			Command:     "recursos",
			Description: "Uso de CPU/RAM e recursos do sistema",
			Example:     "EVA,como estão os recursos?",
		},
		{
			Command:     "conversas",
			Description: "Estatísticas de conversas",
			Example:     "EVA,quantas conversas tivemos?",
		},
		{
			Command:     "teste",
			Description: "Executa teste de funcionalidades",
			Example:     "EVA,faz um teste do sistema",
		},
		// === MEMÓRIA EVA ===
		{
			Command:     "memoria_stats",
			Description: "Estatísticas completas de memória da EVA",
			Example:     "EVA,mostra estatísticas de memória",
		},
		{
			Command:     "memoria_timeline",
			Description: "Timeline de memórias dos últimos dias",
			Example:     "EVA,mostra timeline de memórias",
		},
		{
			Command:     "memoria_integridade",
			Description: "Verifica integridade das memórias",
			Example:     "EVA,verifica integridade das memórias",
		},
		{
			Command:     "memoria_emocoes",
			Description: "Análise de emoções nas memórias",
			Example:     "EVA,analisa emoções nas memórias",
		},
		{
			Command:     "memoria_topicos",
			Description: "Tópicos mais mencionados nas memórias",
			Example:     "EVA,quais tópicos mais falamos?",
		},
		{
			Command:     "memoria_perfis",
			Description: "Perfil de memória de todos pacientes",
			Example:     "EVA,mostra perfis de memória",
		},
		{
			Command:     "memoria_orfas",
			Description: "Lista memórias órfãs (sem paciente)",
			Example:     "EVA,tem memórias órfãs?",
		},
		{
			Command:     "memoria_duplicadas",
			Description: "Lista memórias possivelmente duplicadas",
			Example:     "EVA,tem memórias duplicadas?",
		},
		// === ALERTAS ===
		{
			Command:     "alertas",
			Description: "Verifica todos os alertas do sistema",
			Example:     "EVA,tem algum alerta?",
		},
		{
			Command:     "alertas_criticos",
			Description: "Mostra apenas alertas críticos",
			Example:     "EVA,tem algo crítico?",
		},
		// === LIMPEZA E MANUTENÇÃO ===
		{
			Command:     "limpar_orfas",
			Description: "Remove memórias órfãs (simulação)",
			Example:     "EVA,limpa as memórias órfãs",
		},
		{
			Command:     "limpar_duplicadas",
			Description: "Remove memórias duplicadas (simulação)",
			Example:     "EVA,limpa as duplicadas",
		},
		{
			Command:     "limpeza_completa",
			Description: "Limpeza completa (simulação)",
			Example:     "EVA,faz uma limpeza completa",
		},
		{
			Command:     "limpeza_executar",
			Description: "Executa limpeza REAL (cuidado!)",
			Example:     "EVA,executa a limpeza de verdade",
		},
		// === AJUDA ===
		{
			Command:     "ajuda",
			Description: "Mostra esta lista de comandos",
			Example:     "EVA,o que você pode fazer no modo debug?",
		},
	}

	return commands
}

// GetSystemMetrics coleta métricas do sistema em tempo real
func (d *DebugMode) GetSystemMetrics(ctx context.Context) (*DebugMetrics, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := &DebugMetrics{
		Uptime:        time.Since(d.startTime).Round(time.Second).String(),
		MemoryUsageMB: m.Alloc / 1024 / 1024,
		NumGoroutines: runtime.NumGoroutine(),
		GoVersion:     runtime.Version(),
	}

	// Buscar estatísticas do banco
	if d.db != nil {
		// Total de conversas
		d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini`).Scan(&metrics.TotalConversas)

		// Conversas hoje
		d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini WHERE created_at >= CURRENT_DATE`).Scan(&metrics.ConversasHoje)

		// Total de idosos
		d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM idosos`).Scan(&metrics.TotalIdosos)

		// Idosos ativos
		d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM idosos WHERE ativo = true`).Scan(&metrics.IdososAtivos)

		// Total medicamentos
		d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agendamentos WHERE tipo = 'medicamento'`).Scan(&metrics.TotalMedicamentos)

		// Medicamentos agendados para hoje
		d.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM agendamentos
			WHERE tipo = 'medicamento'
			AND DATE(data_hora_agendada) = CURRENT_DATE
		`).Scan(&metrics.MedicamentosHoje)

		// Análises pendentes
		d.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM agendamentos
			WHERE status = 'agendado'
		`).Scan(&metrics.AnalisesPendentes)

		// Análises hoje
		d.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM analise_gemini
			WHERE created_at >= CURRENT_DATE
		`).Scan(&metrics.AnalisesHoje)
	}

	return metrics, nil
}

// GetRecentLogs retorna os logs mais recentes
func (d *DebugMode) GetRecentLogs(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	query := `
		SELECT
			id,
			idoso_id,
			tipo,
			conteudo,
			created_at
		FROM analise_gemini
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := d.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id, idosoID int64
		var tipo, conteudo string
		var createdAt time.Time

		if err := rows.Scan(&id, &idosoID, &tipo, &conteudo, &createdAt); err != nil {
			continue
		}

		logs = append(logs, map[string]interface{}{
			"id":         id,
			"idoso_id":   idosoID,
			"tipo":       tipo,
			"conteudo":   truncateString(conteudo, 200),
			"created_at": createdAt.Format("02/01/2006 15:04:05"),
		})
	}

	return logs, nil
}

// GetRecentErrors retorna erros recentes do sistema
func (d *DebugMode) GetRecentErrors(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	// Buscar análises com erros (conteúdo contém "error" ou "erro")
	query := `
		SELECT
			id,
			idoso_id,
			tipo,
			conteudo,
			created_at
		FROM analise_gemini
		WHERE conteudo::text ILIKE '%error%' OR conteudo::text ILIKE '%erro%'
		ORDER BY created_at DESC
		LIMIT 10
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []map[string]interface{}
	for rows.Next() {
		var id, idosoID int64
		var tipo, conteudo string
		var createdAt time.Time

		if err := rows.Scan(&id, &idosoID, &tipo, &conteudo, &createdAt); err != nil {
			continue
		}

		errors = append(errors, map[string]interface{}{
			"id":         id,
			"idoso_id":   idosoID,
			"tipo":       tipo,
			"erro":       extractError(conteudo),
			"created_at": createdAt.Format("02/01/2006 15:04:05"),
		})
	}

	return errors, nil
}

// GetPatientsStatus retorna status resumido dos pacientes
func (d *DebugMode) GetPatientsStatus(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	query := `
		SELECT
			i.id,
			i.nome,
			i.ativo,
			i.nivel_cognitivo,
			(SELECT COUNT(*) FROM agendamentos WHERE idoso_id = i.id AND tipo = 'medicamento' AND status IN ('agendado', 'ativo')) as medicamentos_ativos,
			(SELECT MAX(created_at) FROM analise_gemini WHERE idoso_id = i.id) as ultima_conversa
		FROM idosos i
		WHERE i.ativo = true
		ORDER BY i.nome
		LIMIT 20
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patients []map[string]interface{}
	for rows.Next() {
		var id int64
		var nome string
		var ativo bool
		var nivelCognitivo sql.NullString
		var medAtivos int64
		var ultimaConversa sql.NullTime

		if err := rows.Scan(&id, &nome, &ativo, &nivelCognitivo, &medAtivos, &ultimaConversa); err != nil {
			continue
		}

		ultimaConversaStr := "Nunca"
		if ultimaConversa.Valid {
			ultimaConversaStr = ultimaConversa.Time.Format("02/01/2006 15:04")
		}

		patients = append(patients, map[string]interface{}{
			"id":               id,
			"nome":             nome,
			"ativo":            ativo,
			"nivel_cognitivo":  nivelCognitivo.String,
			"medicamentos":     medAtivos,
			"ultima_conversa":  ultimaConversaStr,
		})
	}

	return patients, nil
}

// GetMedicationsStatus retorna status dos medicamentos
func (d *DebugMode) GetMedicationsStatus(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	query := `
		SELECT
			a.id,
			i.nome as paciente,
			a.dados_tarefa,
			a.status,
			a.data_hora_agendada
		FROM agendamentos a
		JOIN idosos i ON a.idoso_id = i.id
		WHERE a.tipo = 'medicamento'
		AND a.status IN ('agendado', 'ativo', 'pendente')
		ORDER BY a.data_hora_agendada
		LIMIT 30
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meds []map[string]interface{}
	for rows.Next() {
		var id int64
		var paciente, dadosTarefa, status string
		var dataHora time.Time

		if err := rows.Scan(&id, &paciente, &dadosTarefa, &status, &dataHora); err != nil {
			continue
		}

		// Parse dados_tarefa JSON
		var medData map[string]interface{}
		json.Unmarshal([]byte(dadosTarefa), &medData)

		nomeMed := "Desconhecido"
		dosagem := ""
		if n, ok := medData["nome"].(string); ok {
			nomeMed = n
		}
		if d, ok := medData["dosagem"].(string); ok {
			dosagem = d
		}

		meds = append(meds, map[string]interface{}{
			"id":           id,
			"paciente":     paciente,
			"medicamento":  nomeMed,
			"dosagem":      dosagem,
			"status":       status,
			"horario":      dataHora.Format("15:04"),
			"data":         dataHora.Format("02/01/2006"),
		})
	}

	return meds, nil
}

// GetConversationStats retorna estatísticas de conversas
func (d *DebugMode) GetConversationStats(ctx context.Context) (map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	stats := make(map[string]interface{})

	// Total geral
	var total int64
	d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini`).Scan(&total)
	stats["total"] = total

	// Hoje
	var hoje int64
	d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini WHERE created_at >= CURRENT_DATE`).Scan(&hoje)
	stats["hoje"] = hoje

	// Esta semana
	var semana int64
	d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'`).Scan(&semana)
	stats["semana"] = semana

	// Este mês
	var mes int64
	d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM analise_gemini WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'`).Scan(&mes)
	stats["mes"] = mes

	// Por tipo
	tipoQuery := `
		SELECT tipo, COUNT(*) as total
		FROM analise_gemini
		GROUP BY tipo
		ORDER BY total DESC
	`
	rows, err := d.db.QueryContext(ctx, tipoQuery)
	if err == nil {
		defer rows.Close()
		porTipo := make(map[string]int64)
		for rows.Next() {
			var tipo string
			var count int64
			if rows.Scan(&tipo, &count) == nil {
				porTipo[tipo] = count
			}
		}
		stats["por_tipo"] = porTipo
	}

	// Média por dia (últimos 30 dias)
	var mediaDia float64
	d.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(daily_count), 0) FROM (
			SELECT DATE(created_at), COUNT(*) as daily_count
			FROM analise_gemini
			WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY DATE(created_at)
		) subq
	`).Scan(&mediaDia)
	stats["media_por_dia"] = fmt.Sprintf("%.1f", mediaDia)

	return stats, nil
}

// RunSystemTest executa testes básicos do sistema
func (d *DebugMode) RunSystemTest(ctx context.Context) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Teste 1: Conexão com banco
	dbOk := false
	if d.db != nil {
		if err := d.db.PingContext(ctx); err == nil {
			dbOk = true
		}
	}
	results["banco_dados"] = map[string]interface{}{
		"status": boolToStatus(dbOk),
		"ok":     dbOk,
	}

	// Teste 2: Verificar tabelas principais
	tablesOk := true
	tables := []string{"idosos", "agendamentos", "analise_gemini"}
	tableResults := make(map[string]bool)
	for _, table := range tables {
		var exists bool
		err := d.db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_name = $1
			)
		`, table).Scan(&exists)
		tableResults[table] = err == nil && exists
		if !tableResults[table] {
			tablesOk = false
		}
	}
	results["tabelas"] = map[string]interface{}{
		"status":  boolToStatus(tablesOk),
		"ok":      tablesOk,
		"detalhes": tableResults,
	}

	// Teste 3: Verificar memória
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memOk := m.Alloc < 500*1024*1024 // Menos de 500MB
	results["memoria"] = map[string]interface{}{
		"status": boolToStatus(memOk),
		"ok":     memOk,
		"uso_mb": m.Alloc / 1024 / 1024,
	}

	// Teste 4: Verificar goroutines
	goroutines := runtime.NumGoroutine()
	goroutinesOk := goroutines < 1000
	results["goroutines"] = map[string]interface{}{
		"status": boolToStatus(goroutinesOk),
		"ok":     goroutinesOk,
		"count":  goroutines,
	}

	// Resumo
	allOk := dbOk && tablesOk && memOk && goroutinesOk
	results["resumo"] = map[string]interface{}{
		"status":    boolToStatus(allOk),
		"ok":        allOk,
		"timestamp": time.Now().Format("02/01/2006 15:04:05"),
	}

	return results, nil
}

// BuildDebugPromptSection constrói a seção de debug para o prompt
func (d *DebugMode) BuildDebugPromptSection(ctx context.Context) string {
	var builder strings.Builder

	builder.WriteString("╔═══════════════════════════════════════════════════════════╗\n")
	builder.WriteString("🔓 MODO DEBUG ATIVADO\n\n")

	builder.WriteString("INSTRUÇÕES MODO DEBUG:\n")
	builder.WriteString("- Pode fornecer informações técnicas detalhadas se solicitado\n")
	builder.WriteString("- Pode discutir seu próprio funcionamento interno\n\n")

	// Adicionar métricas em tempo real
	if metrics, err := d.GetSystemMetrics(ctx); err == nil {
		builder.WriteString("📊 MÉTRICAS EM TEMPO REAL:\n")
		builder.WriteString(fmt.Sprintf("  • Uptime: %s\n", metrics.Uptime))
		builder.WriteString(fmt.Sprintf("  • Memória: %dMB\n", metrics.MemoryUsageMB))
		builder.WriteString(fmt.Sprintf("  • Goroutines: %d\n", metrics.NumGoroutines))
		builder.WriteString(fmt.Sprintf("  • Conversas hoje: %d\n", metrics.ConversasHoje))
		builder.WriteString(fmt.Sprintf("  • Pacientes ativos: %d\n", metrics.IdososAtivos))
		builder.WriteString(fmt.Sprintf("  • Medicamentos hoje: %d\n", metrics.MedicamentosHoje))
		builder.WriteString("\n")
	}

	builder.WriteString("🛠️ COMANDOS DEBUG DISPONÍVEIS:\n")
	for _, cmd := range d.GetAvailableCommands() {
		builder.WriteString(fmt.Sprintf("  • \"%s\" - %s\n", cmd.Example, cmd.Description))
	}
	builder.WriteString("\n")

	return builder.String()
}

// FormatDebugResponse formata uma resposta de debug para fala
func (d *DebugMode) FormatDebugResponse(response *DebugResponse) string {
	// Se é um comando de memória, delega para o MemoryInvestigator
	if strings.HasPrefix(response.Command, "memoria_") && d.memoryInvestigator != nil {
		return d.memoryInvestigator.FormatMemoryResponse(response)
	}

	var builder strings.Builder

	if response.Success {
		builder.WriteString(fmt.Sprintf("Resultado do comando %s:\n\n", response.Command))
	} else {
		builder.WriteString(fmt.Sprintf("Problema ao executar %s:\n", response.Command))
		builder.WriteString(response.Message + "\n")
		return builder.String()
	}

	// Formatar dados baseado no tipo
	switch data := response.Data.(type) {
	case *DebugMetrics:
		builder.WriteString(fmt.Sprintf("Sistema rodando há %s\n", data.Uptime))
		builder.WriteString(fmt.Sprintf("Usando %dMB de memória\n", data.MemoryUsageMB))
		builder.WriteString(fmt.Sprintf("%d goroutines ativas\n", data.NumGoroutines))
		builder.WriteString(fmt.Sprintf("Total de %d conversas, %d hoje\n", data.TotalConversas, data.ConversasHoje))
		builder.WriteString(fmt.Sprintf("%d pacientes ativos de %d cadastrados\n", data.IdososAtivos, data.TotalIdosos))

	case *MemoryStats:
		builder.WriteString(fmt.Sprintf("Total de memórias: %d\n", data.TotalMemories))
		builder.WriteString(fmt.Sprintf("Memórias hoje: %d\n", data.MemoriesHoje))
		builder.WriteString(fmt.Sprintf("Pacientes com memórias: %d\n", data.TotalPacientes))
		builder.WriteString(fmt.Sprintf("Média por paciente: %.1f\n", data.MediaPorPaciente))

	case *MemoryIntegrity:
		builder.WriteString(fmt.Sprintf("Status: %s\n", data.Status))
		builder.WriteString(fmt.Sprintf("Total verificado: %d\n", data.TotalChecked))
		if len(data.Problemas) > 0 {
			builder.WriteString("Problemas:\n")
			for _, p := range data.Problemas {
				builder.WriteString(fmt.Sprintf("  ⚠️ %s\n", p))
			}
		}

	case map[string]interface{}:
		for k, v := range data {
			builder.WriteString(fmt.Sprintf("• %s: %v\n", k, v))
		}

	case []map[string]interface{}:
		for i, item := range data {
			if i >= 5 {
				builder.WriteString(fmt.Sprintf("... e mais %d itens\n", len(data)-5))
				break
			}
			for k, v := range item {
				builder.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
			builder.WriteString("\n")
		}

	case []MemoryTimeline:
		builder.WriteString("Timeline de memórias:\n")
		for i, t := range data {
			if i >= 7 {
				break
			}
			builder.WriteString(fmt.Sprintf("  %s: %d memórias\n", t.Date, t.TotalMemories))
		}

	case []MemoryDetail:
		builder.WriteString(fmt.Sprintf("Encontradas %d memórias:\n", len(data)))
		for i, m := range data {
			if i >= 5 {
				builder.WriteString(fmt.Sprintf("... e mais %d\n", len(data)-5))
				break
			}
			builder.WriteString(fmt.Sprintf("  [%d] %s - %s\n", m.ID, m.IdosoNome, truncateString(m.Content, 50)))
		}
	}

	return builder.String()
}

// DetectDebugCommand detecta se a fala do usuário contém um comando de debug
func (d *DebugMode) DetectDebugCommand(text string) string {
	lower := strings.ToLower(text)

	// Primeiro, verifica comandos de limpeza (mais específicos)
	if d.memoryInvestigator != nil {
		cleanupCmd := d.memoryInvestigator.DetectCleanupCommand(text)
		if cleanupCmd != "" {
			return cleanupCmd
		}

		// Depois, comandos de memória
		memCmd := d.memoryInvestigator.DetectMemoryCommand(text)
		if memCmd != "" {
			return memCmd
		}
	}

	// Mapeamento de palavras-chave para comandos do sistema
	keywords := map[string][]string{
		"status":           {"status", "como você está", "como está o sistema", "como você tá"},
		"metricas":         {"métricas", "metricas", "números", "estatísticas do sistema"},
		"logs":             {"logs", "registros", "histórico de logs"},
		"erros":            {"erros", "erro", "problemas", "bugs"},
		"pacientes":        {"pacientes", "idosos", "usuários"},
		"medicamentos":     {"medicamentos", "remédios", "medicação"},
		"recursos":         {"recursos", "cpu", "ram", "uso de memória do sistema"},
		"conversas":        {"conversas", "diálogos", "interações"},
		"teste":            {"teste", "testar", "verificar sistema", "check"},
		"alertas":          {"alertas", "alerta", "avisos", "notificações"},
		"alertas_criticos": {"críticos", "criticos", "algo crítico", "urgente"},
		"ajuda":            {"ajuda", "comandos", "o que pode fazer", "help"},
	}

	for cmd, words := range keywords {
		for _, word := range words {
			if strings.Contains(lower, word) {
				return cmd
			}
		}
	}

	return ""
}

// ExecuteCommand executa um comando de debug e retorna a resposta
func (d *DebugMode) ExecuteCommand(ctx context.Context, command string) *DebugResponse {
	log.Printf("🔓 [DEBUG] Executando comando: %s", command)

	switch command {
	case "status", "metricas":
		metrics, err := d.GetSystemMetrics(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: metrics}

	case "logs":
		logs, err := d.GetRecentLogs(ctx, 10)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: logs}

	case "erros":
		errors, err := d.GetRecentErrors(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		if len(errors) == 0 {
			return &DebugResponse{Success: true, Command: command, Message: "Nenhum erro encontrado recentemente."}
		}
		return &DebugResponse{Success: true, Command: command, Data: errors}

	case "pacientes":
		patients, err := d.GetPatientsStatus(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: patients}

	case "medicamentos":
		meds, err := d.GetMedicationsStatus(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: meds}

	case "recursos":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memData := map[string]interface{}{
			"alocado_mb":     m.Alloc / 1024 / 1024,
			"total_mb":       m.TotalAlloc / 1024 / 1024,
			"sistema_mb":     m.Sys / 1024 / 1024,
			"gc_executados":  m.NumGC,
			"goroutines":     runtime.NumGoroutine(),
			"go_version":     runtime.Version(),
		}
		return &DebugResponse{Success: true, Command: command, Data: memData}

	// === COMANDOS DE MEMÓRIA DA EVA ===
	case "memoria_stats", "memoria_timeline", "memoria_integridade",
		"memoria_emocoes", "memoria_topicos", "memoria_perfis",
		"memoria_orfas", "memoria_duplicadas":
		if d.memoryInvestigator != nil {
			return d.memoryInvestigator.ExecuteMemoryCommand(ctx, command)
		}
		return &DebugResponse{Success: false, Command: command, Message: "Investigador de memória não disponível"}

	case "conversas":
		stats, err := d.GetConversationStats(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: stats}

	case "teste":
		results, err := d.RunSystemTest(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: results}

	case "ajuda":
		commands := d.GetAvailableCommands()
		var helpData []map[string]interface{}
		for _, cmd := range commands {
			helpData = append(helpData, map[string]interface{}{
				"comando":   cmd.Command,
				"descricao": cmd.Description,
				"exemplo":   cmd.Example,
			})
		}
		return &DebugResponse{Success: true, Command: command, Data: helpData}

	// === COMANDOS DE ALERTAS ===
	case "alertas":
		if d.alertSystem != nil {
			summary := d.alertSystem.CheckAllAlerts(ctx)
			return &DebugResponse{Success: true, Command: command, Data: summary}
		}
		return &DebugResponse{Success: false, Command: command, Message: "Sistema de alertas não disponível"}

	case "alertas_criticos":
		if d.alertSystem != nil {
			d.alertSystem.CheckAllAlerts(ctx)
			critical := d.alertSystem.GetCriticalAlerts()
			if len(critical) == 0 {
				return &DebugResponse{Success: true, Command: command, Message: "Nenhum alerta crítico no momento."}
			}
			return &DebugResponse{Success: true, Command: command, Data: critical}
		}
		return &DebugResponse{Success: false, Command: command, Message: "Sistema de alertas não disponível"}

	// === COMANDOS DE LIMPEZA ===
	case "limpar_orfas", "limpar_duplicadas", "limpar_vazias", "limpar_antigas",
		"limpeza_completa", "limpeza_executar", "arquivar_memorias":
		if d.memoryInvestigator != nil {
			return d.memoryInvestigator.ExecuteCleanupCommand(ctx, command)
		}
		return &DebugResponse{Success: false, Command: command, Message: "Investigador de memória não disponível"}

	default:
		return &DebugResponse{
			Success: false,
			Command: command,
			Message: "Comando não reconhecido. Diga 'ajuda' para ver os comandos disponíveis.",
		}
	}
}

// Funções auxiliares

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractError(content string) string {
	// Tenta extrair a mensagem de erro do JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		if errMsg, ok := data["error"].(string); ok {
			return truncateString(errMsg, 100)
		}
	}
	return truncateString(content, 100)
}

func boolToStatus(ok bool) string {
	if ok {
		return "✅ OK"
	}
	return "❌ ERRO"
}
