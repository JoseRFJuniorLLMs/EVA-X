package lacan

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// MemoryInvestigator fornece ferramentas de investigação de memória para o modo debug
type MemoryInvestigator struct {
	db *sql.DB
}

// NewMemoryInvestigator cria uma nova instância do investigador de memória
func NewMemoryInvestigator(db *sql.DB) *MemoryInvestigator {
	return &MemoryInvestigator{db: db}
}

// ═══════════════════════════════════════════════════════════
// 📊 ESTRUTURAS DE DADOS
// ═══════════════════════════════════════════════════════════

// MemoryStats estatísticas gerais de memória
type MemoryStats struct {
	TotalMemories      int64              `json:"total_memories"`
	MemoriesHoje       int64              `json:"memories_hoje"`
	MemoriesSemana     int64              `json:"memories_semana"`
	MemoriesMes        int64              `json:"memories_mes"`
	TotalPacientes     int64              `json:"total_pacientes"`
	MediaPorPaciente   float64            `json:"media_por_paciente"`
	MemoriasMaisAntiga time.Time          `json:"memoria_mais_antiga"`
	MemoriaMaisRecente time.Time          `json:"memoria_mais_recente"`
	PorEmotion         map[string]int64   `json:"por_emotion"`
	PorSpeaker         map[string]int64   `json:"por_speaker"`
	TopTopics          []TopicCount       `json:"top_topics"`
	ImportanciaMedia   float64            `json:"importancia_media"`
	TamanhoMedioBytes  int64              `json:"tamanho_medio_bytes"`
}

// TopicCount contagem de tópico
type TopicCount struct {
	Topic string `json:"topic"`
	Count int64  `json:"count"`
}

// MemoryDetail detalhes de uma memória específica
type MemoryDetail struct {
	ID            int64     `json:"id"`
	IdosoID       int64     `json:"idoso_id"`
	IdosoNome     string    `json:"idoso_nome"`
	Timestamp     time.Time `json:"timestamp"`
	Speaker       string    `json:"speaker"`
	Content       string    `json:"content"`
	ContentLength int       `json:"content_length"`
	Emotion       string    `json:"emotion"`
	Importance    float64   `json:"importance"`
	Topics        []string  `json:"topics"`
	SessionID     string    `json:"session_id"`
	HasEmbedding  bool      `json:"has_embedding"`
}

// MemoryTimeline linha do tempo de memórias
type MemoryTimeline struct {
	Date          string `json:"date"`
	TotalMemories int64  `json:"total_memories"`
	UserMessages  int64  `json:"user_messages"`
	EVAMessages   int64  `json:"eva_messages"`
	Emotions      string `json:"emotions"`
}

// MemoryIntegrity verificação de integridade
type MemoryIntegrity struct {
	TotalChecked      int64    `json:"total_checked"`
	MemoriesOrfas     int64    `json:"memories_orfas"`      // Sem paciente associado
	MemoriasSemConteudo int64  `json:"memorias_sem_conteudo"`
	MemoriasDuplicadas int64   `json:"memorias_duplicadas"`
	MemoriasSemEmbedding int64 `json:"memorias_sem_embedding"`
	Problemas         []string `json:"problemas"`
	Status            string   `json:"status"`
}

// PatientMemoryProfile perfil de memória de um paciente
type PatientMemoryProfile struct {
	IdosoID           int64            `json:"idoso_id"`
	Nome              string           `json:"nome"`
	TotalMemories     int64            `json:"total_memories"`
	PrimeiraMemoria   time.Time        `json:"primeira_memoria"`
	UltimaMemoria     time.Time        `json:"ultima_memoria"`
	EmocoesMaisComuns []string         `json:"emocoes_mais_comuns"`
	TopicosFrequentes []string         `json:"topicos_frequentes"`
	ImportanciaMedia  float64          `json:"importancia_media"`
	SessoesUnicas     int64            `json:"sessoes_unicas"`
	MemoriasPorMes    map[string]int64 `json:"memorias_por_mes"`
}

// MemorySearchResult resultado de busca de memória
type MemorySearchResult struct {
	Memories   []MemoryDetail `json:"memories"`
	TotalFound int64          `json:"total_found"`
	Query      string         `json:"query"`
	Filters    string         `json:"filters"`
}

// ═══════════════════════════════════════════════════════════
// 📈 ESTATÍSTICAS GERAIS
// ═══════════════════════════════════════════════════════════

// GetMemoryStats retorna estatísticas completas de memória
func (m *MemoryInvestigator) GetMemoryStats(ctx context.Context) (*MemoryStats, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	stats := &MemoryStats{
		PorEmotion: make(map[string]int64),
		PorSpeaker: make(map[string]int64),
	}

	// Total de memórias
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episodic_memories`).Scan(&stats.TotalMemories)

	// Memórias hoje
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE timestamp >= CURRENT_DATE
	`).Scan(&stats.MemoriesHoje)

	// Memórias semana
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'
	`).Scan(&stats.MemoriesSemana)

	// Memórias mês
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days'
	`).Scan(&stats.MemoriesMes)

	// Total de pacientes com memórias
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT idoso_id) FROM episodic_memories
	`).Scan(&stats.TotalPacientes)

	// Média por paciente
	if stats.TotalPacientes > 0 {
		stats.MediaPorPaciente = float64(stats.TotalMemories) / float64(stats.TotalPacientes)
	}

	// Memória mais antiga e mais recente
	m.db.QueryRowContext(ctx, `SELECT MIN(timestamp) FROM episodic_memories`).Scan(&stats.MemoriasMaisAntiga)
	m.db.QueryRowContext(ctx, `SELECT MAX(timestamp) FROM episodic_memories`).Scan(&stats.MemoriaMaisRecente)

	// Por emoção
	emotionRows, err := m.db.QueryContext(ctx, `
		SELECT COALESCE(emotion, 'indefinido'), COUNT(*)
		FROM episodic_memories
		GROUP BY emotion
		ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer emotionRows.Close()
		for emotionRows.Next() {
			var emotion string
			var count int64
			if emotionRows.Scan(&emotion, &count) == nil {
				stats.PorEmotion[emotion] = count
			}
		}
	}

	// Por speaker
	speakerRows, err := m.db.QueryContext(ctx, `
		SELECT speaker, COUNT(*)
		FROM episodic_memories
		GROUP BY speaker
	`)
	if err == nil {
		defer speakerRows.Close()
		for speakerRows.Next() {
			var speaker string
			var count int64
			if speakerRows.Scan(&speaker, &count) == nil {
				stats.PorSpeaker[speaker] = count
			}
		}
	}

	// Top tópicos
	stats.TopTopics = m.getTopTopics(ctx, 10)

	// Importância média
	m.db.QueryRowContext(ctx, `SELECT AVG(importance) FROM episodic_memories`).Scan(&stats.ImportanciaMedia)

	// Tamanho médio
	m.db.QueryRowContext(ctx, `SELECT AVG(LENGTH(content)) FROM episodic_memories`).Scan(&stats.TamanhoMedioBytes)

	return stats, nil
}

// getTopTopics retorna os tópicos mais frequentes
func (m *MemoryInvestigator) getTopTopics(ctx context.Context, limit int) []TopicCount {
	// Como topics é um array, precisamos unnest
	query := `
		SELECT topic, COUNT(*) as cnt
		FROM episodic_memories, unnest(topics) as topic
		GROUP BY topic
		ORDER BY cnt DESC
		LIMIT $1
	`

	rows, err := m.db.QueryContext(ctx, query, limit)
	if err != nil {
		log.Printf("⚠️ [MemoryInvestigator] Erro ao buscar top topics: %v", err)
		return nil
	}
	defer rows.Close()

	var topics []TopicCount
	for rows.Next() {
		var tc TopicCount
		if rows.Scan(&tc.Topic, &tc.Count) == nil {
			topics = append(topics, tc)
		}
	}

	return topics
}

// ═══════════════════════════════════════════════════════════
// 🔍 BUSCA E INVESTIGAÇÃO
// ═══════════════════════════════════════════════════════════

// SearchMemories busca memórias com filtros
func (m *MemoryInvestigator) SearchMemories(ctx context.Context, query string, idosoID *int64, emotion *string, startDate *time.Time, endDate *time.Time, limit int) (*MemorySearchResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	var conditions []string
	var args []interface{}
	argNum := 1

	// Filtro por conteúdo
	if query != "" {
		conditions = append(conditions, fmt.Sprintf("content ILIKE $%d", argNum))
		args = append(args, "%"+query+"%")
		argNum++
	}

	// Filtro por paciente
	if idosoID != nil {
		conditions = append(conditions, fmt.Sprintf("em.idoso_id = $%d", argNum))
		args = append(args, *idosoID)
		argNum++
	}

	// Filtro por emoção
	if emotion != nil && *emotion != "" {
		conditions = append(conditions, fmt.Sprintf("emotion = $%d", argNum))
		args = append(args, *emotion)
		argNum++
	}

	// Filtro por data início
	if startDate != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argNum))
		args = append(args, *startDate)
		argNum++
	}

	// Filtro por data fim
	if endDate != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argNum))
		args = append(args, *endDate)
		argNum++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Query principal
	sqlQuery := fmt.Sprintf(`
		SELECT em.id, em.idoso_id, COALESCE(i.nome, 'Desconhecido'),
		       em.timestamp, em.speaker, em.content, em.emotion,
		       em.importance, em.topics, COALESCE(em.session_id, ''),
		       em.embedding IS NOT NULL as has_embedding
		FROM episodic_memories em
		LEFT JOIN idosos i ON em.idoso_id = i.id
		%s
		ORDER BY em.timestamp DESC
		LIMIT $%d
	`, whereClause, argNum)

	args = append(args, limit)

	rows, err := m.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("erro na busca: %w", err)
	}
	defer rows.Close()

	var memories []MemoryDetail
	for rows.Next() {
		var mem MemoryDetail
		var topics string
		var emotion sql.NullString

		err := rows.Scan(
			&mem.ID, &mem.IdosoID, &mem.IdosoNome,
			&mem.Timestamp, &mem.Speaker, &mem.Content, &emotion,
			&mem.Importance, &topics, &mem.SessionID, &mem.HasEmbedding,
		)
		if err != nil {
			continue
		}

		mem.Emotion = emotion.String
		mem.ContentLength = len(mem.Content)
		mem.Topics = parseTopicsArray(topics)
		memories = append(memories, mem)
	}

	// Contar total
	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM episodic_memories em %s`, whereClause)
	m.db.QueryRowContext(ctx, countQuery, args[:len(args)-1]...).Scan(&total)

	return &MemorySearchResult{
		Memories:   memories,
		TotalFound: total,
		Query:      query,
		Filters:    strings.Join(conditions, ", "),
	}, nil
}

// GetMemoryByID retorna detalhes de uma memória específica
func (m *MemoryInvestigator) GetMemoryByID(ctx context.Context, memoryID int64) (*MemoryDetail, error) {
	query := `
		SELECT em.id, em.idoso_id, COALESCE(i.nome, 'Desconhecido'),
		       em.timestamp, em.speaker, em.content, em.emotion,
		       em.importance, em.topics, COALESCE(em.session_id, ''),
		       em.embedding IS NOT NULL as has_embedding
		FROM episodic_memories em
		LEFT JOIN idosos i ON em.idoso_id = i.id
		WHERE em.id = $1
	`

	var mem MemoryDetail
	var topics string
	var emotion sql.NullString

	err := m.db.QueryRowContext(ctx, query, memoryID).Scan(
		&mem.ID, &mem.IdosoID, &mem.IdosoNome,
		&mem.Timestamp, &mem.Speaker, &mem.Content, &emotion,
		&mem.Importance, &topics, &mem.SessionID, &mem.HasEmbedding,
	)
	if err != nil {
		return nil, fmt.Errorf("memória não encontrada: %w", err)
	}

	mem.Emotion = emotion.String
	mem.ContentLength = len(mem.Content)
	mem.Topics = parseTopicsArray(topics)

	return &mem, nil
}

// ═══════════════════════════════════════════════════════════
// 👤 PERFIL DE MEMÓRIA POR PACIENTE
// ═══════════════════════════════════════════════════════════

// GetPatientMemoryProfile retorna perfil de memória de um paciente
func (m *MemoryInvestigator) GetPatientMemoryProfile(ctx context.Context, idosoID int64) (*PatientMemoryProfile, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	profile := &PatientMemoryProfile{
		IdosoID:        idosoID,
		MemoriasPorMes: make(map[string]int64),
	}

	// Nome do paciente
	m.db.QueryRowContext(ctx, `SELECT nome FROM idosos WHERE id = $1`, idosoID).Scan(&profile.Nome)

	// Total de memórias
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories WHERE idoso_id = $1
	`, idosoID).Scan(&profile.TotalMemories)

	// Primeira e última memória
	m.db.QueryRowContext(ctx, `
		SELECT MIN(timestamp), MAX(timestamp) FROM episodic_memories WHERE idoso_id = $1
	`, idosoID).Scan(&profile.PrimeiraMemoria, &profile.UltimaMemoria)

	// Importância média
	m.db.QueryRowContext(ctx, `
		SELECT AVG(importance) FROM episodic_memories WHERE idoso_id = $1
	`, idosoID).Scan(&profile.ImportanciaMedia)

	// Sessões únicas
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT session_id) FROM episodic_memories
		WHERE idoso_id = $1 AND session_id IS NOT NULL
	`, idosoID).Scan(&profile.SessoesUnicas)

	// Emoções mais comuns
	emotionRows, _ := m.db.QueryContext(ctx, `
		SELECT emotion, COUNT(*) as cnt FROM episodic_memories
		WHERE idoso_id = $1 AND emotion IS NOT NULL
		GROUP BY emotion ORDER BY cnt DESC LIMIT 5
	`, idosoID)
	if emotionRows != nil {
		defer emotionRows.Close()
		for emotionRows.Next() {
			var emotion string
			var cnt int64
			if emotionRows.Scan(&emotion, &cnt) == nil {
				profile.EmocoesMaisComuns = append(profile.EmocoesMaisComuns, emotion)
			}
		}
	}

	// Tópicos frequentes
	topicRows, _ := m.db.QueryContext(ctx, `
		SELECT topic, COUNT(*) as cnt
		FROM episodic_memories, unnest(topics) as topic
		WHERE idoso_id = $1
		GROUP BY topic ORDER BY cnt DESC LIMIT 10
	`, idosoID)
	if topicRows != nil {
		defer topicRows.Close()
		for topicRows.Next() {
			var topic string
			var cnt int64
			if topicRows.Scan(&topic, &cnt) == nil {
				profile.TopicosFrequentes = append(profile.TopicosFrequentes, topic)
			}
		}
	}

	// Memórias por mês
	monthRows, _ := m.db.QueryContext(ctx, `
		SELECT TO_CHAR(timestamp, 'YYYY-MM') as month, COUNT(*) as cnt
		FROM episodic_memories WHERE idoso_id = $1
		GROUP BY month ORDER BY month DESC LIMIT 12
	`, idosoID)
	if monthRows != nil {
		defer monthRows.Close()
		for monthRows.Next() {
			var month string
			var cnt int64
			if monthRows.Scan(&month, &cnt) == nil {
				profile.MemoriasPorMes[month] = cnt
			}
		}
	}

	return profile, nil
}

// GetAllPatientsMemoryProfiles retorna perfil resumido de todos os pacientes
func (m *MemoryInvestigator) GetAllPatientsMemoryProfiles(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			i.id, i.nome,
			COUNT(em.id) as total_memories,
			MIN(em.timestamp) as primeira,
			MAX(em.timestamp) as ultima,
			AVG(em.importance) as importancia_media
		FROM idosos i
		LEFT JOIN episodic_memories em ON i.id = em.idoso_id
		WHERE i.ativo = true
		GROUP BY i.id, i.nome
		ORDER BY total_memories DESC
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []map[string]interface{}
	for rows.Next() {
		var id int64
		var nome string
		var total int64
		var primeira, ultima sql.NullTime
		var importancia sql.NullFloat64

		if rows.Scan(&id, &nome, &total, &primeira, &ultima, &importancia) == nil {
			primeiraStr := "N/A"
			if primeira.Valid {
				primeiraStr = primeira.Time.Format("02/01/2006")
			}
			ultimaStr := "N/A"
			if ultima.Valid {
				ultimaStr = ultima.Time.Format("02/01/2006 15:04")
			}

			profiles = append(profiles, map[string]interface{}{
				"id":          id,
				"nome":        nome,
				"memorias":    total,
				"primeira":    primeiraStr,
				"ultima":      ultimaStr,
				"importancia": fmt.Sprintf("%.2f", importancia.Float64),
			})
		}
	}

	return profiles, nil
}

// ═══════════════════════════════════════════════════════════
// 📅 TIMELINE DE MEMÓRIAS
// ═══════════════════════════════════════════════════════════

// GetMemoryTimeline retorna timeline de memórias
func (m *MemoryInvestigator) GetMemoryTimeline(ctx context.Context, idosoID *int64, days int) ([]MemoryTimeline, error) {
	var whereClause string
	var args []interface{}

	if idosoID != nil {
		whereClause = "WHERE idoso_id = $1 AND timestamp >= CURRENT_DATE - INTERVAL '1 day' * $2"
		args = []interface{}{*idosoID, days}
	} else {
		whereClause = "WHERE timestamp >= CURRENT_DATE - INTERVAL '1 day' * $1"
		args = []interface{}{days}
	}

	query := fmt.Sprintf(`
		SELECT
			TO_CHAR(timestamp, 'YYYY-MM-DD') as date,
			COUNT(*) as total,
			SUM(CASE WHEN speaker = 'user' THEN 1 ELSE 0 END) as user_msgs,
			SUM(CASE WHEN speaker = 'assistant' THEN 1 ELSE 0 END) as eva_msgs,
			STRING_AGG(DISTINCT COALESCE(emotion, ''), ', ') as emotions
		FROM episodic_memories
		%s
		GROUP BY date
		ORDER BY date DESC
	`, whereClause)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeline []MemoryTimeline
	for rows.Next() {
		var t MemoryTimeline
		if rows.Scan(&t.Date, &t.TotalMemories, &t.UserMessages, &t.EVAMessages, &t.Emotions) == nil {
			timeline = append(timeline, t)
		}
	}

	return timeline, nil
}

// ═══════════════════════════════════════════════════════════
// 🔧 VERIFICAÇÃO DE INTEGRIDADE
// ═══════════════════════════════════════════════════════════

// CheckMemoryIntegrity verifica integridade das memórias
func (m *MemoryInvestigator) CheckMemoryIntegrity(ctx context.Context) (*MemoryIntegrity, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	integrity := &MemoryIntegrity{
		Problemas: []string{},
	}

	// Total de memórias verificadas
	m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episodic_memories`).Scan(&integrity.TotalChecked)

	// Memórias órfãs (sem paciente válido)
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories em
		LEFT JOIN idosos i ON em.idoso_id = i.id
		WHERE i.id IS NULL
	`).Scan(&integrity.MemoriesOrfas)
	if integrity.MemoriesOrfas > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias órfãs (paciente não existe)", integrity.MemoriesOrfas))
	}

	// Memórias sem conteúdo
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE content IS NULL OR content = ''
	`).Scan(&integrity.MemoriasSemConteudo)
	if integrity.MemoriasSemConteudo > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias sem conteúdo", integrity.MemoriasSemConteudo))
	}

	// Memórias sem embedding
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episodic_memories
		WHERE embedding IS NULL
	`).Scan(&integrity.MemoriasSemEmbedding)
	if integrity.MemoriasSemEmbedding > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias sem embedding vetorial", integrity.MemoriasSemEmbedding))
	}

	// Memórias duplicadas (mesmo conteúdo, mesmo paciente, mesmo timestamp)
	m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) - COUNT(DISTINCT (idoso_id, content, DATE_TRUNC('minute', timestamp)))
		FROM episodic_memories
	`).Scan(&integrity.MemoriasDuplicadas)
	if integrity.MemoriasDuplicadas > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d possíveis memórias duplicadas", integrity.MemoriasDuplicadas))
	}

	// Definir status
	if len(integrity.Problemas) == 0 {
		integrity.Status = "✅ ÍNTEGRO - Nenhum problema encontrado"
	} else if len(integrity.Problemas) <= 2 {
		integrity.Status = "⚠️ ATENÇÃO - Alguns problemas detectados"
	} else {
		integrity.Status = "❌ CRÍTICO - Múltiplos problemas detectados"
	}

	return integrity, nil
}

// ═══════════════════════════════════════════════════════════
// 🧹 LIMPEZA E MANUTENÇÃO
// ═══════════════════════════════════════════════════════════

// GetOrphanMemories retorna memórias órfãs para análise
func (m *MemoryInvestigator) GetOrphanMemories(ctx context.Context, limit int) ([]MemoryDetail, error) {
	query := `
		SELECT em.id, em.idoso_id, 'PACIENTE REMOVIDO',
		       em.timestamp, em.speaker, em.content, em.emotion,
		       em.importance, em.topics, COALESCE(em.session_id, ''),
		       em.embedding IS NOT NULL
		FROM episodic_memories em
		LEFT JOIN idosos i ON em.idoso_id = i.id
		WHERE i.id IS NULL
		LIMIT $1
	`

	rows, err := m.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []MemoryDetail
	for rows.Next() {
		var mem MemoryDetail
		var topics string
		var emotion sql.NullString

		if rows.Scan(
			&mem.ID, &mem.IdosoID, &mem.IdosoNome,
			&mem.Timestamp, &mem.Speaker, &mem.Content, &emotion,
			&mem.Importance, &topics, &mem.SessionID, &mem.HasEmbedding,
		) == nil {
			mem.Emotion = emotion.String
			mem.ContentLength = len(mem.Content)
			mem.Topics = parseTopicsArray(topics)
			memories = append(memories, mem)
		}
	}

	return memories, nil
}

// GetDuplicateMemories retorna possíveis memórias duplicadas
func (m *MemoryInvestigator) GetDuplicateMemories(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT
			idoso_id,
			content,
			COUNT(*) as duplicates,
			MIN(timestamp) as first_occurrence,
			MAX(timestamp) as last_occurrence
		FROM episodic_memories
		GROUP BY idoso_id, content
		HAVING COUNT(*) > 1
		ORDER BY duplicates DESC
		LIMIT $1
	`

	rows, err := m.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var duplicates []map[string]interface{}
	for rows.Next() {
		var idosoID int64
		var content string
		var count int64
		var first, last time.Time

		if rows.Scan(&idosoID, &content, &count, &first, &last) == nil {
			duplicates = append(duplicates, map[string]interface{}{
				"idoso_id":   idosoID,
				"conteudo":   truncateString(content, 100),
				"duplicatas": count,
				"primeira":   first.Format("02/01/2006 15:04"),
				"ultima":     last.Format("02/01/2006 15:04"),
			})
		}
	}

	return duplicates, nil
}

// ═══════════════════════════════════════════════════════════
// 📊 ANÁLISE AVANÇADA
// ═══════════════════════════════════════════════════════════

// GetEmotionAnalysis analisa emoções nas memórias
func (m *MemoryInvestigator) GetEmotionAnalysis(ctx context.Context, idosoID *int64) (map[string]interface{}, error) {
	analysis := make(map[string]interface{})

	var whereClause string
	var args []interface{}
	if idosoID != nil {
		whereClause = "WHERE idoso_id = $1"
		args = []interface{}{*idosoID}
	}

	// Distribuição de emoções
	query := fmt.Sprintf(`
		SELECT
			COALESCE(emotion, 'indefinido') as emotion,
			COUNT(*) as total,
			ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) as percentual
		FROM episodic_memories
		%s
		GROUP BY emotion
		ORDER BY total DESC
	`, whereClause)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emotions []map[string]interface{}
	for rows.Next() {
		var emotion string
		var total int64
		var percentual float64
		if rows.Scan(&emotion, &total, &percentual) == nil {
			emotions = append(emotions, map[string]interface{}{
				"emotion":    emotion,
				"total":      total,
				"percentual": fmt.Sprintf("%.1f%%", percentual),
			})
		}
	}
	analysis["distribuicao"] = emotions

	// Tendência emocional (últimos 7 dias vs anterior)
	var recentPositive, recentNegative int64

	positiveEmotions := "'feliz', 'alegre', 'satisfeito', 'calmo', 'esperançoso'"
	negativeEmotions := "'triste', 'ansioso', 'irritado', 'preocupado', 'frustrado'"

	m.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM episodic_memories
		WHERE emotion IN (%s) AND timestamp >= CURRENT_DATE - INTERVAL '7 days' %s
	`, positiveEmotions, andClause(whereClause)), args...).Scan(&recentPositive)

	m.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM episodic_memories
		WHERE emotion IN (%s) AND timestamp >= CURRENT_DATE - INTERVAL '7 days' %s
	`, negativeEmotions, andClause(whereClause)), args...).Scan(&recentNegative)

	analysis["tendencia"] = map[string]interface{}{
		"positivas_7dias":  recentPositive,
		"negativas_7dias":  recentNegative,
		"balanco":          recentPositive - recentNegative,
	}

	return analysis, nil
}

// GetTopicAnalysis analisa tópicos nas memórias
func (m *MemoryInvestigator) GetTopicAnalysis(ctx context.Context, idosoID *int64, limit int) ([]map[string]interface{}, error) {
	var whereClause string
	var args []interface{}
	argNum := 1

	if idosoID != nil {
		whereClause = fmt.Sprintf("WHERE idoso_id = $%d", argNum)
		args = append(args, *idosoID)
		argNum++
	}

	query := fmt.Sprintf(`
		SELECT
			topic,
			COUNT(*) as mentions,
			COUNT(DISTINCT idoso_id) as pacientes,
			MIN(timestamp) as primeira_mencao,
			MAX(timestamp) as ultima_mencao
		FROM episodic_memories, unnest(topics) as topic
		%s
		GROUP BY topic
		ORDER BY mentions DESC
		LIMIT $%d
	`, whereClause, argNum)

	args = append(args, limit)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []map[string]interface{}
	for rows.Next() {
		var topic string
		var mentions, pacientes int64
		var primeira, ultima time.Time

		if rows.Scan(&topic, &mentions, &pacientes, &primeira, &ultima) == nil {
			topics = append(topics, map[string]interface{}{
				"topico":    topic,
				"mencoes":   mentions,
				"pacientes": pacientes,
				"primeira":  primeira.Format("02/01/2006"),
				"ultima":    ultima.Format("02/01/2006"),
			})
		}
	}

	return topics, nil
}

// ═══════════════════════════════════════════════════════════
// 📤 EXPORTAÇÃO
// ═══════════════════════════════════════════════════════════

// ExportPatientMemories exporta memórias de um paciente em formato JSON
func (m *MemoryInvestigator) ExportPatientMemories(ctx context.Context, idosoID int64) (string, error) {
	// Buscar perfil
	profile, err := m.GetPatientMemoryProfile(ctx, idosoID)
	if err != nil {
		return "", err
	}

	// Buscar todas as memórias
	result, err := m.SearchMemories(ctx, "", &idosoID, nil, nil, nil, 10000)
	if err != nil {
		return "", err
	}

	export := map[string]interface{}{
		"export_date": time.Now().Format(time.RFC3339),
		"profile":     profile,
		"memories":    result.Memories,
		"total":       result.TotalFound,
	}

	jsonData, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// ═══════════════════════════════════════════════════════════
// 🔧 HELPERS
// ═══════════════════════════════════════════════════════════

func parseTopicsArray(s string) []string {
	if s == "{}" || s == "" || s == "NULL" {
		return []string{}
	}

	// Remove {} e parse
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	if s == "" {
		return []string{}
	}

	var result []string
	var current string
	inQuotes := false

	for _, c := range s {
		switch c {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				if current != "" {
					result = append(result, strings.TrimSpace(current))
					current = ""
				}
			} else {
				current += string(c)
			}
		default:
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, strings.TrimSpace(current))
	}

	return result
}

func andClause(where string) string {
	if where != "" {
		return " AND " + strings.TrimPrefix(where, "WHERE ")
	}
	return ""
}

// ═══════════════════════════════════════════════════════════
// 🎯 COMANDOS DE DEBUG PARA MEMÓRIA
// ═══════════════════════════════════════════════════════════

// GetMemoryCommands retorna os comandos de memória disponíveis
func (m *MemoryInvestigator) GetMemoryCommands() []DebugCommand {
	return []DebugCommand{
		{
			Command:     "memoria_stats",
			Description: "Estatísticas completas de memória do sistema",
			Example:     "Arquiteto, me mostra as estatísticas de memória",
		},
		{
			Command:     "memoria_buscar",
			Description: "Busca memórias por texto ou filtros",
			Example:     "Arquiteto, busca memórias sobre medicamentos",
		},
		{
			Command:     "memoria_paciente",
			Description: "Perfil de memória de um paciente específico",
			Example:     "Arquiteto, mostra as memórias do paciente X",
		},
		{
			Command:     "memoria_timeline",
			Description: "Timeline de memórias dos últimos dias",
			Example:     "Arquiteto, mostra a timeline de memórias",
		},
		{
			Command:     "memoria_integridade",
			Description: "Verificação de integridade das memórias",
			Example:     "Arquiteto, verifica a integridade das memórias",
		},
		{
			Command:     "memoria_emocoes",
			Description: "Análise de emoções nas memórias",
			Example:     "Arquiteto, analisa as emoções nas memórias",
		},
		{
			Command:     "memoria_topicos",
			Description: "Análise de tópicos mais mencionados",
			Example:     "Arquiteto, quais são os tópicos mais falados?",
		},
		{
			Command:     "memoria_orfas",
			Description: "Lista memórias órfãs (sem paciente)",
			Example:     "Arquiteto, tem memórias órfãs?",
		},
		{
			Command:     "memoria_duplicadas",
			Description: "Lista possíveis memórias duplicadas",
			Example:     "Arquiteto, tem memórias duplicadas?",
		},
		{
			Command:     "memoria_perfis",
			Description: "Perfil de memória de todos os pacientes",
			Example:     "Arquiteto, mostra os perfis de memória",
		},
	}
}

// DetectMemoryCommand detecta comandos de memória na fala
func (m *MemoryInvestigator) DetectMemoryCommand(text string) string {
	lower := strings.ToLower(text)

	keywords := map[string][]string{
		"memoria_stats":       {"estatísticas de memória", "estatisticas de memoria", "stats de memória", "status da memória"},
		"memoria_buscar":      {"busca memória", "buscar memórias", "procura memória", "pesquisa memória"},
		"memoria_paciente":    {"memórias do paciente", "memorias do paciente", "memória de"},
		"memoria_timeline":    {"timeline de memória", "linha do tempo", "histórico de memórias"},
		"memoria_integridade": {"integridade", "verificar memórias", "checar memórias"},
		"memoria_emocoes":     {"emoções nas memórias", "emocoes", "sentimentos"},
		"memoria_topicos":     {"tópicos", "topicos", "assuntos mais falados"},
		"memoria_orfas":       {"órfãs", "orfas", "sem paciente"},
		"memoria_duplicadas":  {"duplicadas", "repetidas", "duplicatas"},
		"memoria_perfis":      {"perfis de memória", "todos os pacientes"},
	}

	for cmd, words := range keywords {
		for _, word := range words {
			if strings.Contains(lower, word) {
				return cmd
			}
		}
	}

	// Comando genérico de memória
	if strings.Contains(lower, "memória") || strings.Contains(lower, "memoria") {
		return "memoria_stats"
	}

	return ""
}

// ExecuteMemoryCommand executa um comando de memória
func (m *MemoryInvestigator) ExecuteMemoryCommand(ctx context.Context, command string) *DebugResponse {
	log.Printf("🧠 [MEMORY DEBUG] Executando comando: %s", command)

	switch command {
	case "memoria_stats":
		stats, err := m.GetMemoryStats(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: stats}

	case "memoria_timeline":
		timeline, err := m.GetMemoryTimeline(ctx, nil, 14)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: timeline}

	case "memoria_integridade":
		integrity, err := m.CheckMemoryIntegrity(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: integrity}

	case "memoria_emocoes":
		analysis, err := m.GetEmotionAnalysis(ctx, nil)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: analysis}

	case "memoria_topicos":
		topics, err := m.GetTopicAnalysis(ctx, nil, 15)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: topics}

	case "memoria_orfas":
		orphans, err := m.GetOrphanMemories(ctx, 20)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		if len(orphans) == 0 {
			return &DebugResponse{Success: true, Command: command, Message: "Nenhuma memória órfã encontrada!"}
		}
		return &DebugResponse{Success: true, Command: command, Data: orphans}

	case "memoria_duplicadas":
		duplicates, err := m.GetDuplicateMemories(ctx, 20)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		if len(duplicates) == 0 {
			return &DebugResponse{Success: true, Command: command, Message: "Nenhuma memória duplicada encontrada!"}
		}
		return &DebugResponse{Success: true, Command: command, Data: duplicates}

	case "memoria_perfis":
		profiles, err := m.GetAllPatientsMemoryProfiles(ctx)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: profiles}

	default:
		return &DebugResponse{
			Success: false,
			Command: command,
			Message: "Comando de memória não reconhecido",
		}
	}
}

// FormatMemoryResponse formata resposta de memória para fala
func (m *MemoryInvestigator) FormatMemoryResponse(response *DebugResponse) string {
	var builder strings.Builder

	if !response.Success {
		builder.WriteString(fmt.Sprintf("Arquiteto, tive um problema: %s\n", response.Message))
		return builder.String()
	}

	if response.Message != "" {
		builder.WriteString(fmt.Sprintf("Arquiteto, %s\n", response.Message))
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("Arquiteto, aqui está o resultado de %s:\n\n", response.Command))

	switch data := response.Data.(type) {
	case *MemoryStats:
		builder.WriteString(fmt.Sprintf("Total de memórias: %d\n", data.TotalMemories))
		builder.WriteString(fmt.Sprintf("Memórias hoje: %d\n", data.MemoriesHoje))
		builder.WriteString(fmt.Sprintf("Memórias na semana: %d\n", data.MemoriesSemana))
		builder.WriteString(fmt.Sprintf("Pacientes com memórias: %d\n", data.TotalPacientes))
		builder.WriteString(fmt.Sprintf("Média por paciente: %.1f\n", data.MediaPorPaciente))
		builder.WriteString(fmt.Sprintf("Importância média: %.2f\n", data.ImportanciaMedia))
		if len(data.TopTopics) > 0 {
			builder.WriteString("\nTópicos mais frequentes:\n")
			for i, t := range data.TopTopics {
				if i >= 5 {
					break
				}
				builder.WriteString(fmt.Sprintf("  • %s (%d menções)\n", t.Topic, t.Count))
			}
		}

	case *MemoryIntegrity:
		builder.WriteString(fmt.Sprintf("Status: %s\n", data.Status))
		builder.WriteString(fmt.Sprintf("Total verificado: %d memórias\n", data.TotalChecked))
		if len(data.Problemas) > 0 {
			builder.WriteString("\nProblemas encontrados:\n")
			for _, p := range data.Problemas {
				builder.WriteString(fmt.Sprintf("  ⚠️ %s\n", p))
			}
		}

	case []MemoryTimeline:
		builder.WriteString("Timeline dos últimos dias:\n")
		for i, t := range data {
			if i >= 7 {
				break
			}
			builder.WriteString(fmt.Sprintf("  %s: %d memórias (%d usuário, %d EVA)\n",
				t.Date, t.TotalMemories, t.UserMessages, t.EVAMessages))
		}

	case map[string]interface{}:
		for k, v := range data {
			builder.WriteString(fmt.Sprintf("• %s: %v\n", k, v))
		}

	case []map[string]interface{}:
		for i, item := range data {
			if i >= 10 {
				builder.WriteString(fmt.Sprintf("\n... e mais %d itens\n", len(data)-10))
				break
			}
			for k, v := range item {
				builder.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
			builder.WriteString("\n")
		}

	case *CleanupResult:
		builder.WriteString(fmt.Sprintf("Operação: %s\n", data.Operation))
		builder.WriteString(fmt.Sprintf("Status: %s\n", data.Status))
		builder.WriteString(fmt.Sprintf("Itens afetados: %d\n", data.AffectedCount))
		if data.Message != "" {
			builder.WriteString(fmt.Sprintf("Detalhes: %s\n", data.Message))
		}
	}

	return builder.String()
}

// ═══════════════════════════════════════════════════════════
// 🧹 COMANDOS DE LIMPEZA E MANUTENÇÃO
// ═══════════════════════════════════════════════════════════

// CleanupResult resultado de operação de limpeza
type CleanupResult struct {
	Operation     string `json:"operation"`
	AffectedCount int64  `json:"affected_count"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	Details       []map[string]interface{} `json:"details,omitempty"`
}

// CleanOrphanMemories remove memórias órfãs (sem paciente válido)
func (m *MemoryInvestigator) CleanOrphanMemories(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: "limpar_memorias_orfas",
	}

	if dryRun {
		// Apenas contar, não deletar
		var count int64
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episodic_memories em
			LEFT JOIN idosos i ON em.idoso_id = i.id
			WHERE i.id IS NULL
		`).Scan(&count)
		if err != nil {
			return nil, err
		}
		result.AffectedCount = count
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias órfãs seriam removidas (dry-run)", count)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias órfãs encontradas", count)
	} else {
		// Executar limpeza real
		res, err := m.db.ExecContext(ctx, `
			DELETE FROM episodic_memories em
			USING (
				SELECT em2.id FROM episodic_memories em2
				LEFT JOIN idosos i ON em2.idoso_id = i.id
				WHERE i.id IS NULL
			) orphans
			WHERE em.id = orphans.id
		`)
		if err != nil {
			return nil, fmt.Errorf("erro ao limpar memórias órfãs: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.AffectedCount = affected
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias órfãs removidas com sucesso", affected)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias órfãs", affected)
	}

	return result, nil
}

// CleanDuplicateMemories remove memórias duplicadas (mantém a mais antiga)
func (m *MemoryInvestigator) CleanDuplicateMemories(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: "limpar_memorias_duplicadas",
	}

	if dryRun {
		// Contar duplicatas
		var count int64
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM (
				SELECT id, ROW_NUMBER() OVER (
					PARTITION BY idoso_id, content, DATE_TRUNC('minute', timestamp)
					ORDER BY timestamp ASC
				) as rn
				FROM episodic_memories
			) duplicates
			WHERE rn > 1
		`).Scan(&count)
		if err != nil {
			return nil, err
		}
		result.AffectedCount = count
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias duplicadas seriam removidas (dry-run)", count)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias duplicadas encontradas", count)
	} else {
		// Remover duplicatas mantendo a mais antiga
		res, err := m.db.ExecContext(ctx, `
			DELETE FROM episodic_memories
			WHERE id IN (
				SELECT id FROM (
					SELECT id, ROW_NUMBER() OVER (
						PARTITION BY idoso_id, content, DATE_TRUNC('minute', timestamp)
						ORDER BY timestamp ASC
					) as rn
					FROM episodic_memories
				) duplicates
				WHERE rn > 1
			)
		`)
		if err != nil {
			return nil, fmt.Errorf("erro ao limpar duplicatas: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.AffectedCount = affected
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias duplicadas removidas com sucesso", affected)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias duplicadas", affected)
	}

	return result, nil
}

// CleanOldMemories remove memórias antigas com baixa importância
func (m *MemoryInvestigator) CleanOldMemories(ctx context.Context, olderThanDays int, maxImportance float64, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: fmt.Sprintf("limpar_memorias_antigas_%d_dias", olderThanDays),
	}

	if dryRun {
		var count int64
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episodic_memories
			WHERE timestamp < CURRENT_DATE - INTERVAL '1 day' * $1
			AND importance < $2
		`, olderThanDays, maxImportance).Scan(&count)
		if err != nil {
			return nil, err
		}
		result.AffectedCount = count
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias antigas (>%d dias, importância <%.2f) seriam removidas", count, olderThanDays, maxImportance)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias antigas encontradas", count)
	} else {
		res, err := m.db.ExecContext(ctx, `
			DELETE FROM episodic_memories
			WHERE timestamp < CURRENT_DATE - INTERVAL '1 day' * $1
			AND importance < $2
		`, olderThanDays, maxImportance)
		if err != nil {
			return nil, fmt.Errorf("erro ao limpar memórias antigas: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.AffectedCount = affected
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias antigas removidas com sucesso", affected)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias antigas", affected)
	}

	return result, nil
}

// CleanEmptyMemories remove memórias sem conteúdo
func (m *MemoryInvestigator) CleanEmptyMemories(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: "limpar_memorias_vazias",
	}

	if dryRun {
		var count int64
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episodic_memories
			WHERE content IS NULL OR content = '' OR LENGTH(TRIM(content)) < 3
		`).Scan(&count)
		if err != nil {
			return nil, err
		}
		result.AffectedCount = count
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias vazias/inválidas seriam removidas", count)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias vazias encontradas", count)
	} else {
		res, err := m.db.ExecContext(ctx, `
			DELETE FROM episodic_memories
			WHERE content IS NULL OR content = '' OR LENGTH(TRIM(content)) < 3
		`)
		if err != nil {
			return nil, fmt.Errorf("erro ao limpar memórias vazias: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.AffectedCount = affected
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias vazias removidas com sucesso", affected)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias vazias", affected)
	}

	return result, nil
}

// RunFullCleanup executa limpeza completa (simulação por padrão)
func (m *MemoryInvestigator) RunFullCleanup(ctx context.Context, dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		Operation: "limpeza_completa",
		Details:   []map[string]interface{}{},
	}

	var totalAffected int64

	// 1. Limpar órfãs
	orphanResult, err := m.CleanOrphanMemories(ctx, dryRun)
	if err == nil {
		totalAffected += orphanResult.AffectedCount
		result.Details = append(result.Details, map[string]interface{}{
			"operacao": "orfas",
			"afetados": orphanResult.AffectedCount,
		})
	}

	// 2. Limpar duplicatas
	dupResult, err := m.CleanDuplicateMemories(ctx, dryRun)
	if err == nil {
		totalAffected += dupResult.AffectedCount
		result.Details = append(result.Details, map[string]interface{}{
			"operacao": "duplicadas",
			"afetados": dupResult.AffectedCount,
		})
	}

	// 3. Limpar vazias
	emptyResult, err := m.CleanEmptyMemories(ctx, dryRun)
	if err == nil {
		totalAffected += emptyResult.AffectedCount
		result.Details = append(result.Details, map[string]interface{}{
			"operacao": "vazias",
			"afetados": emptyResult.AffectedCount,
		})
	}

	result.AffectedCount = totalAffected
	if dryRun {
		result.Status = "✅ SIMULAÇÃO COMPLETA"
		result.Message = fmt.Sprintf("Total de %d memórias seriam afetadas (dry-run)", totalAffected)
	} else {
		result.Status = "✅ LIMPEZA COMPLETA"
		result.Message = fmt.Sprintf("Total de %d memórias removidas com sucesso", totalAffected)
	}

	log.Printf("🧹 [CLEANUP] Limpeza completa: %d itens %s", totalAffected, map[bool]string{true: "simulados", false: "removidos"}[dryRun])

	return result, nil
}

// ArchiveOldMemories arquiva memórias antigas para tabela de histórico
func (m *MemoryInvestigator) ArchiveOldMemories(ctx context.Context, olderThanDays int, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: fmt.Sprintf("arquivar_memorias_%d_dias", olderThanDays),
	}

	// Verificar se tabela de arquivo existe
	var tableExists bool
	m.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'episodic_memories_archive'
		)
	`).Scan(&tableExists)

	if !tableExists {
		// Criar tabela de arquivo se não existir
		_, err := m.db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS episodic_memories_archive (
				LIKE episodic_memories INCLUDING ALL,
				archived_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return nil, fmt.Errorf("erro ao criar tabela de arquivo: %w", err)
		}
		log.Printf("🧹 [ARCHIVE] Tabela episodic_memories_archive criada")
	}

	if dryRun {
		var count int64
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episodic_memories
			WHERE timestamp < CURRENT_DATE - INTERVAL '1 day' * $1
		`, olderThanDays).Scan(&count)
		if err != nil {
			return nil, err
		}
		result.AffectedCount = count
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias seriam arquivadas (>%d dias)", count, olderThanDays)
	} else {
		// Mover para arquivo
		res, err := m.db.ExecContext(ctx, `
			WITH moved AS (
				DELETE FROM episodic_memories
				WHERE timestamp < CURRENT_DATE - INTERVAL '1 day' * $1
				RETURNING *
			)
			INSERT INTO episodic_memories_archive
			SELECT *, CURRENT_TIMESTAMP FROM moved
		`, olderThanDays)
		if err != nil {
			return nil, fmt.Errorf("erro ao arquivar memórias: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.AffectedCount = affected
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias arquivadas com sucesso", affected)
		log.Printf("🧹 [ARCHIVE] %d memórias movidas para arquivo", affected)
	}

	return result, nil
}

// GetCleanupCommands retorna comandos de limpeza disponíveis
func (m *MemoryInvestigator) GetCleanupCommands() []DebugCommand {
	return []DebugCommand{
		{
			Command:     "limpar_orfas",
			Description: "Remove memórias órfãs (sem paciente)",
			Example:     "Arquiteto, limpa as memórias órfãs",
		},
		{
			Command:     "limpar_duplicadas",
			Description: "Remove memórias duplicadas",
			Example:     "Arquiteto, limpa as memórias duplicadas",
		},
		{
			Command:     "limpar_vazias",
			Description: "Remove memórias sem conteúdo",
			Example:     "Arquiteto, limpa as memórias vazias",
		},
		{
			Command:     "limpar_antigas",
			Description: "Remove memórias antigas (>90 dias, baixa importância)",
			Example:     "Arquiteto, limpa as memórias antigas",
		},
		{
			Command:     "limpeza_completa",
			Description: "Executa limpeza completa (simulação)",
			Example:     "Arquiteto, faz uma limpeza completa",
		},
		{
			Command:     "limpeza_executar",
			Description: "Executa limpeza completa (REAL - cuidado!)",
			Example:     "Arquiteto, executa a limpeza de verdade",
		},
		{
			Command:     "arquivar_memorias",
			Description: "Arquiva memórias antigas (>180 dias)",
			Example:     "Arquiteto, arquiva as memórias antigas",
		},
	}
}

// DetectCleanupCommand detecta comandos de limpeza
func (m *MemoryInvestigator) DetectCleanupCommand(text string) string {
	lower := strings.ToLower(text)

	keywords := map[string][]string{
		"limpar_orfas":      {"limpa órfãs", "limpar orfas", "remove órfãs", "deletar orfas"},
		"limpar_duplicadas": {"limpa duplicadas", "limpar duplicadas", "remove duplicadas"},
		"limpar_vazias":     {"limpa vazias", "limpar vazias", "remove vazias"},
		"limpar_antigas":    {"limpa antigas", "limpar antigas", "remove antigas"},
		"limpeza_completa":  {"limpeza completa", "limpa tudo", "faz limpeza"},
		"limpeza_executar":  {"executa limpeza", "limpeza de verdade", "limpar de verdade"},
		"arquivar_memorias": {"arquiva", "arquivar memórias", "mover para arquivo"},
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

// ExecuteCleanupCommand executa um comando de limpeza
func (m *MemoryInvestigator) ExecuteCleanupCommand(ctx context.Context, command string) *DebugResponse {
	log.Printf("🧹 [CLEANUP] Executando comando: %s", command)

	switch command {
	case "limpar_orfas":
		result, err := m.CleanOrphanMemories(ctx, true) // dry-run
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "limpar_duplicadas":
		result, err := m.CleanDuplicateMemories(ctx, true)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "limpar_vazias":
		result, err := m.CleanEmptyMemories(ctx, true)
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "limpar_antigas":
		result, err := m.CleanOldMemories(ctx, 90, 0.5, true) // >90 dias, importância <0.5
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "limpeza_completa":
		result, err := m.RunFullCleanup(ctx, true) // simulação
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "limpeza_executar":
		result, err := m.RunFullCleanup(ctx, false) // REAL
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	case "arquivar_memorias":
		result, err := m.ArchiveOldMemories(ctx, 180, true) // >180 dias, simulação
		if err != nil {
			return &DebugResponse{Success: false, Command: command, Message: err.Error()}
		}
		return &DebugResponse{Success: true, Command: command, Data: result}

	default:
		return &DebugResponse{
			Success: false,
			Command: command,
			Message: "Comando de limpeza não reconhecido",
		}
	}
}
