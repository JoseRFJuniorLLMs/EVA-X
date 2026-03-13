package lacan

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// DebugMode gerencia funcionalidades exclusivas para o Arquiteto da Matrix (Criador)
type DebugMode struct {
	db                 *database.DB
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
func NewDebugMode(db *database.DB) *DebugMode {
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

	// Buscar estatísticas do banco via NietzscheDB
	if d.db != nil {
		today := time.Now().Format("2006-01-02")

		// Total de conversas
		if c, err := d.db.Count(ctx, "analise_gemini", "", nil); err == nil {
			metrics.TotalConversas = int64(c)
		}

		// Conversas hoje - query all and filter in Go
		if rows, err := d.db.QueryByLabel(ctx, "analise_gemini", "", nil, 0); err == nil {
			var hojeCnt int64
			for _, m := range rows {
				t := database.GetTime(m, "created_at")
				if !t.IsZero() && t.Format("2006-01-02") == today {
					hojeCnt++
				}
			}
			metrics.ConversasHoje = hojeCnt
			metrics.AnalisesHoje = hojeCnt
		}

		// Total de idosos
		if c, err := d.db.Count(ctx, "idosos", "", nil); err == nil {
			metrics.TotalIdosos = int64(c)
		}

		// Idosos ativos
		if c, err := d.db.Count(ctx, "idosos", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}); err == nil {
			metrics.IdososAtivos = int64(c)
		}

		// Total medicamentos
		if c, err := d.db.Count(ctx, "agendamentos", " AND n.tipo = $tipo", map[string]interface{}{"tipo": "medicamento"}); err == nil {
			metrics.TotalMedicamentos = int64(c)
		}

		// Medicamentos agendados para hoje - query all medicamentos and filter by date
		if rows, err := d.db.QueryByLabel(ctx, "agendamentos", " AND n.tipo = $tipo", map[string]interface{}{"tipo": "medicamento"}, 0); err == nil {
			var hojeCnt int64
			for _, m := range rows {
				t := database.GetTime(m, "data_hora_agendada")
				if !t.IsZero() && t.Format("2006-01-02") == today {
					hojeCnt++
				}
			}
			metrics.MedicamentosHoje = hojeCnt
		}

		// Análises pendentes
		if c, err := d.db.Count(ctx, "agendamentos", " AND n.status = $status", map[string]interface{}{"status": "agendado"}); err == nil {
			metrics.AnalisesPendentes = int64(c)
		}
	}

	return metrics, nil
}

// GetRecentLogs retorna os logs mais recentes
func (d *DebugMode) GetRecentLogs(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	rows, err := d.db.QueryByLabel(ctx, "analise_gemini", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Sort by created_at DESC in Go
	sort.Slice(rows, func(i, j int) bool {
		ti := database.GetTime(rows[i], "created_at")
		tj := database.GetTime(rows[j], "created_at")
		return ti.After(tj)
	})

	// Apply limit
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	var logs []map[string]interface{}
	for _, m := range rows {
		createdAt := database.GetTime(m, "created_at")
		createdAtStr := ""
		if !createdAt.IsZero() {
			createdAtStr = createdAt.Format("02/01/2006 15:04:05")
		}

		logs = append(logs, map[string]interface{}{
			"id":         database.GetInt64(m, "id"),
			"idoso_id":   database.GetInt64(m, "idoso_id"),
			"tipo":       database.GetString(m, "tipo"),
			"conteudo":   truncateString(database.GetString(m, "conteudo"), 200),
			"created_at": createdAtStr,
		})
	}

	return logs, nil
}

// GetRecentErrors retorna erros recentes do sistema
func (d *DebugMode) GetRecentErrors(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	rows, err := d.db.QueryByLabel(ctx, "analise_gemini", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Filter for entries containing "error" or "erro" (case-insensitive)
	var filtered []map[string]interface{}
	for _, m := range rows {
		conteudo := strings.ToLower(database.GetString(m, "conteudo"))
		if strings.Contains(conteudo, "error") || strings.Contains(conteudo, "erro") {
			filtered = append(filtered, m)
		}
	}

	// Sort by created_at DESC
	sort.Slice(filtered, func(i, j int) bool {
		ti := database.GetTime(filtered[i], "created_at")
		tj := database.GetTime(filtered[j], "created_at")
		return ti.After(tj)
	})

	// Limit to 10
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	var errors []map[string]interface{}
	for _, m := range filtered {
		createdAt := database.GetTime(m, "created_at")
		createdAtStr := ""
		if !createdAt.IsZero() {
			createdAtStr = createdAt.Format("02/01/2006 15:04:05")
		}

		errors = append(errors, map[string]interface{}{
			"id":         database.GetInt64(m, "id"),
			"idoso_id":   database.GetInt64(m, "idoso_id"),
			"tipo":       database.GetString(m, "tipo"),
			"erro":       extractError(database.GetString(m, "conteudo")),
			"created_at": createdAtStr,
		})
	}

	return errors, nil
}

// GetPatientsStatus retorna status resumido dos pacientes
func (d *DebugMode) GetPatientsStatus(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	// Query active patients
	idososRows, err := d.db.QueryByLabel(ctx, "idosos", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return nil, err
	}

	// Query agendamentos for medication counts
	agendRows, err := d.db.QueryByLabel(ctx, "agendamentos", " AND n.tipo = $tipo", map[string]interface{}{"tipo": "medicamento"}, 0)
	if err != nil {
		agendRows = nil // non-fatal
	}

	// Query analise_gemini for last conversation dates
	conversaRows, err := d.db.QueryByLabel(ctx, "analise_gemini", "", nil, 0)
	if err != nil {
		conversaRows = nil // non-fatal
	}

	// Build medication count map: idoso_id -> count of active meds
	medCountByIdoso := make(map[int64]int64)
	for _, m := range agendRows {
		status := database.GetString(m, "status")
		if status == "agendado" || status == "ativo" {
			idosoID := database.GetInt64(m, "idoso_id")
			medCountByIdoso[idosoID]++
		}
	}

	// Build last conversation map: idoso_id -> latest created_at
	lastConversaByIdoso := make(map[int64]time.Time)
	for _, m := range conversaRows {
		idosoID := database.GetInt64(m, "idoso_id")
		t := database.GetTime(m, "created_at")
		if !t.IsZero() {
			if existing, ok := lastConversaByIdoso[idosoID]; !ok || t.After(existing) {
				lastConversaByIdoso[idosoID] = t
			}
		}
	}

	// Sort patients by nome
	sort.Slice(idososRows, func(i, j int) bool {
		return database.GetString(idososRows[i], "nome") < database.GetString(idososRows[j], "nome")
	})

	// Limit to 20
	if len(idososRows) > 20 {
		idososRows = idososRows[:20]
	}

	var patients []map[string]interface{}
	for _, m := range idososRows {
		id := database.GetInt64(m, "id")

		ultimaConversaStr := "Nunca"
		if t, ok := lastConversaByIdoso[id]; ok {
			ultimaConversaStr = t.Format("02/01/2006 15:04")
		}

		patients = append(patients, map[string]interface{}{
			"id":              id,
			"nome":            database.GetString(m, "nome"),
			"ativo":           true,
			"nivel_cognitivo": database.GetString(m, "nivel_cognitivo"),
			"medicamentos":    medCountByIdoso[id],
			"ultima_conversa": ultimaConversaStr,
		})
	}

	return patients, nil
}

// GetMedicationsStatus retorna status dos medicamentos
func (d *DebugMode) GetMedicationsStatus(ctx context.Context) ([]map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	// Query agendamentos for medicamentos with active statuses
	agendRows, err := d.db.QueryByLabel(ctx, "agendamentos", " AND n.tipo = $tipo", map[string]interface{}{"tipo": "medicamento"}, 0)
	if err != nil {
		return nil, err
	}

	// Filter by status in Go
	var filtered []map[string]interface{}
	for _, m := range agendRows {
		status := database.GetString(m, "status")
		if status == "agendado" || status == "ativo" || status == "pendente" {
			filtered = append(filtered, m)
		}
	}

	// Sort by data_hora_agendada ASC
	sort.Slice(filtered, func(i, j int) bool {
		ti := database.GetTime(filtered[i], "data_hora_agendada")
		tj := database.GetTime(filtered[j], "data_hora_agendada")
		return ti.Before(tj)
	})

	// Limit to 30
	if len(filtered) > 30 {
		filtered = filtered[:30]
	}

	// Build idoso name lookup: query idosos to map id -> nome
	idososRows, err := d.db.QueryByLabel(ctx, "idosos", "", nil, 0)
	if err != nil {
		idososRows = nil // non-fatal, will show "Desconhecido"
	}
	idosoNames := make(map[int64]string)
	for _, m := range idososRows {
		idosoNames[database.GetInt64(m, "id")] = database.GetString(m, "nome")
	}

	var meds []map[string]interface{}
	for _, m := range filtered {
		dataHora := database.GetTime(m, "data_hora_agendada")
		dadosTarefa := database.GetString(m, "dados_tarefa")
		idosoID := database.GetInt64(m, "idoso_id")

		paciente := idosoNames[idosoID]
		if paciente == "" {
			paciente = "Desconhecido"
		}

		// Parse dados_tarefa JSON
		var medData map[string]interface{}
		json.Unmarshal([]byte(dadosTarefa), &medData)

		nomeMed := "Desconhecido"
		dosagem := ""
		if medData != nil {
			if n, ok := medData["nome"].(string); ok {
				nomeMed = n
			}
			if dsg, ok := medData["dosagem"].(string); ok {
				dosagem = dsg
			}
		}

		meds = append(meds, map[string]interface{}{
			"id":          database.GetInt64(m, "id"),
			"paciente":    paciente,
			"medicamento": nomeMed,
			"dosagem":     dosagem,
			"status":      database.GetString(m, "status"),
			"horario":     dataHora.Format("15:04"),
			"data":        dataHora.Format("02/01/2006"),
		})
	}

	return meds, nil
}

// GetConversationStats retorna estatísticas de conversas
func (d *DebugMode) GetConversationStats(ctx context.Context) (map[string]interface{}, error) {
	if d.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	rows, err := d.db.QueryByLabel(ctx, "analise_gemini", "", nil, 0)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})

	now := time.Now()
	today := now.Format("2006-01-02")
	sevenDaysAgo := now.AddDate(0, 0, -7)
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	var total, hoje, semana, mes int64
	porTipo := make(map[string]int64)
	dailyCounts := make(map[string]int64) // date -> count for avg calculation

	for _, m := range rows {
		total++
		t := database.GetTime(m, "created_at")
		tipo := database.GetString(m, "tipo")

		if tipo != "" {
			porTipo[tipo]++
		}

		if !t.IsZero() {
			dateStr := t.Format("2006-01-02")
			if dateStr == today {
				hoje++
			}
			if !t.Before(sevenDaysAgo) {
				semana++
			}
			if !t.Before(thirtyDaysAgo) {
				mes++
				dailyCounts[dateStr]++
			}
		}
	}

	stats["total"] = total
	stats["hoje"] = hoje
	stats["semana"] = semana
	stats["mes"] = mes
	stats["por_tipo"] = porTipo

	// Average per day (last 30 days)
	var mediaDia float64
	if len(dailyCounts) > 0 {
		var sum int64
		for _, c := range dailyCounts {
			sum += c
		}
		mediaDia = float64(sum) / float64(len(dailyCounts))
	}
	stats["media_por_dia"] = fmt.Sprintf("%.1f", mediaDia)

	return stats, nil
}

// RunSystemTest executa testes básicos do sistema
func (d *DebugMode) RunSystemTest(ctx context.Context) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Teste 1: Conexão com banco (NietzscheDB) - try a simple Count query
	dbOk := false
	if d.db != nil {
		if _, err := d.db.Count(ctx, "idosos", "", nil); err == nil {
			dbOk = true
		}
	}
	results["banco_dados"] = map[string]interface{}{
		"status": boolToStatus(dbOk),
		"ok":     dbOk,
	}

	// Teste 2: Verificar "tabelas" (labels) - NietzscheDB always has collections, just check queryability
	tablesOk := true
	tables := []string{"idosos", "agendamentos", "analise_gemini"}
	tableResults := make(map[string]bool)
	for _, table := range tables {
		_, err := d.db.Count(ctx, table, "", nil)
		tableResults[table] = err == nil
		if !tableResults[table] {
			tablesOk = false
		}
	}
	results["tabelas"] = map[string]interface{}{
		"status":   boolToStatus(tablesOk),
		"ok":       tablesOk,
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
