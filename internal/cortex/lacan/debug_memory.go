// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

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

// MemoryInvestigator fornece ferramentas de investigação de memória para o modo debug
type MemoryInvestigator struct {
	db *database.DB
}

// NewMemoryInvestigator cria uma nova instância do investigador de memória
func NewMemoryInvestigator(db *database.DB) *MemoryInvestigator {
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

	// Fetch all episodic memories
	allRows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar memórias: %w", err)
	}

	stats.TotalMemories = int64(len(allRows))

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, 0, -30)

	uniquePatients := make(map[int64]bool)
	var totalImportance float64
	var totalContentLen int64

	for _, row := range allRows {
		ts := database.GetTime(row, "timestamp")
		idosoID := database.GetInt64(row, "idoso_id")
		speaker := database.GetString(row, "speaker")
		emotion := database.GetString(row, "emotion")
		importance := database.GetFloat64(row, "importance")
		content := database.GetString(row, "content")

		// Time-based counts
		if !ts.Before(today) {
			stats.MemoriesHoje++
		}
		if !ts.Before(weekAgo) {
			stats.MemoriesSemana++
		}
		if !ts.Before(monthAgo) {
			stats.MemoriesMes++
		}

		// Unique patients
		uniquePatients[idosoID] = true

		// Oldest and newest
		if stats.MemoriasMaisAntiga.IsZero() || ts.Before(stats.MemoriasMaisAntiga) {
			stats.MemoriasMaisAntiga = ts
		}
		if ts.After(stats.MemoriaMaisRecente) {
			stats.MemoriaMaisRecente = ts
		}

		// Emotion counts
		if emotion == "" {
			emotion = "indefinido"
		}
		stats.PorEmotion[emotion]++

		// Speaker counts
		if speaker != "" {
			stats.PorSpeaker[speaker]++
		}

		// Importance
		totalImportance += importance

		// Content length
		totalContentLen += int64(len(content))
	}

	stats.TotalPacientes = int64(len(uniquePatients))
	if stats.TotalPacientes > 0 {
		stats.MediaPorPaciente = float64(stats.TotalMemories) / float64(stats.TotalPacientes)
	}
	if stats.TotalMemories > 0 {
		stats.ImportanciaMedia = totalImportance / float64(stats.TotalMemories)
		stats.TamanhoMedioBytes = totalContentLen / stats.TotalMemories
	}

	// Top topics
	stats.TopTopics = m.getTopTopics(ctx, 10)

	return stats, nil
}

// getTopTopics retorna os tópicos mais frequentes
func (m *MemoryInvestigator) getTopTopics(ctx context.Context, limit int) []TopicCount {
	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		log.Printf("⚠️ [MemoryInvestigator] Erro ao buscar top topics: %v", err)
		return nil
	}

	topicCounts := make(map[string]int64)
	for _, row := range rows {
		topicsStr := database.GetString(row, "topics")
		topics := parseTopicsArray(topicsStr)
		for _, t := range topics {
			if t != "" {
				topicCounts[t]++
			}
		}
	}

	// Convert to sorted slice
	var topics []TopicCount
	for topic, count := range topicCounts {
		topics = append(topics, TopicCount{Topic: topic, Count: count})
	}
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Count > topics[j].Count
	})

	if len(topics) > limit {
		topics = topics[:limit]
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

	// Build NietzscheDB query
	extraWhere := ""
	params := map[string]interface{}{}
	var filterParts []string

	if idosoID != nil {
		extraWhere += " AND n.idoso_id = $idoso_id"
		params["idoso_id"] = *idosoID
		filterParts = append(filterParts, fmt.Sprintf("idoso_id=%d", *idosoID))
	}

	if emotion != nil && *emotion != "" {
		extraWhere += " AND n.emotion = $emotion"
		params["emotion"] = *emotion
		filterParts = append(filterParts, fmt.Sprintf("emotion=%s", *emotion))
	}

	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", extraWhere, params, 0)
	if err != nil {
		return nil, fmt.Errorf("erro na busca: %w", err)
	}

	// Apply additional filters that can't be expressed in NQL
	var filtered []map[string]interface{}
	for _, row := range rows {
		// Filter by content query
		if query != "" {
			content := strings.ToLower(database.GetString(row, "content"))
			if !strings.Contains(content, strings.ToLower(query)) {
				continue
			}
		}

		// Filter by date range
		ts := database.GetTime(row, "timestamp")
		if startDate != nil && ts.Before(*startDate) {
			continue
		}
		if endDate != nil && ts.After(*endDate) {
			continue
		}

		filtered = append(filtered, row)
	}

	// Sort by timestamp descending
	sort.Slice(filtered, func(i, j int) bool {
		ti := database.GetTime(filtered[i], "timestamp")
		tj := database.GetTime(filtered[j], "timestamp")
		return ti.After(tj)
	})

	totalFound := int64(len(filtered))

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	// Convert to MemoryDetail with patient name lookup
	var memories []MemoryDetail
	for _, row := range filtered {
		mem := m.rowToMemoryDetail(ctx, row)
		memories = append(memories, mem)
	}

	return &MemorySearchResult{
		Memories:   memories,
		TotalFound: totalFound,
		Query:      query,
		Filters:    strings.Join(filterParts, ", "),
	}, nil
}

// rowToMemoryDetail converts a NietzscheDB row to a MemoryDetail
func (m *MemoryInvestigator) rowToMemoryDetail(ctx context.Context, row map[string]interface{}) MemoryDetail {
	mem := MemoryDetail{
		ID:         database.GetInt64(row, "pg_id"),
		IdosoID:    database.GetInt64(row, "idoso_id"),
		Timestamp:  database.GetTime(row, "timestamp"),
		Speaker:    database.GetString(row, "speaker"),
		Content:    database.GetString(row, "content"),
		Emotion:    database.GetString(row, "emotion"),
		Importance: database.GetFloat64(row, "importance"),
		SessionID:  database.GetString(row, "session_id"),
	}

	if mem.ID == 0 {
		mem.ID = database.GetInt64(row, "id")
	}

	mem.ContentLength = len(mem.Content)
	mem.Topics = parseTopicsArray(database.GetString(row, "topics"))

	// Check if embedding exists
	_, hasEmb := row["embedding"]
	mem.HasEmbedding = hasEmb && row["embedding"] != nil

	// Look up patient name
	mem.IdosoNome = "Desconhecido"
	if mem.IdosoID > 0 {
		patient, err := m.db.GetNodeByID(ctx, "Idoso", mem.IdosoID)
		if err == nil && patient != nil {
			mem.IdosoNome = database.GetString(patient, "nome")
		}
	}

	return mem
}

// GetMemoryByID retorna detalhes de uma memória específica
func (m *MemoryInvestigator) GetMemoryByID(ctx context.Context, memoryID int64) (*MemoryDetail, error) {
	row, err := m.db.GetNodeByID(ctx, "EpisodicMemory", memoryID)
	if err != nil {
		return nil, fmt.Errorf("memória não encontrada: %w", err)
	}
	if row == nil {
		return nil, fmt.Errorf("memória não encontrada: id=%d", memoryID)
	}

	mem := m.rowToMemoryDetail(ctx, row)
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
	patient, err := m.db.GetNodeByID(ctx, "Idoso", idosoID)
	if err == nil && patient != nil {
		profile.Nome = database.GetString(patient, "nome")
	}

	// Get all memories for this patient
	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 0)
	if err != nil {
		return profile, nil
	}

	profile.TotalMemories = int64(len(rows))

	emotionCounts := make(map[string]int64)
	topicCounts := make(map[string]int64)
	sessionsSet := make(map[string]bool)
	var totalImportance float64

	for _, row := range rows {
		ts := database.GetTime(row, "timestamp")

		// First and last memory
		if profile.PrimeiraMemoria.IsZero() || ts.Before(profile.PrimeiraMemoria) {
			profile.PrimeiraMemoria = ts
		}
		if ts.After(profile.UltimaMemoria) {
			profile.UltimaMemoria = ts
		}

		// Importance
		totalImportance += database.GetFloat64(row, "importance")

		// Sessions
		sessionID := database.GetString(row, "session_id")
		if sessionID != "" {
			sessionsSet[sessionID] = true
		}

		// Emotions
		emotion := database.GetString(row, "emotion")
		if emotion != "" {
			emotionCounts[emotion]++
		}

		// Topics
		topicsStr := database.GetString(row, "topics")
		for _, t := range parseTopicsArray(topicsStr) {
			if t != "" {
				topicCounts[t]++
			}
		}

		// Monthly counts
		monthKey := ts.Format("2006-01")
		profile.MemoriasPorMes[monthKey]++
	}

	if profile.TotalMemories > 0 {
		profile.ImportanciaMedia = totalImportance / float64(profile.TotalMemories)
	}
	profile.SessoesUnicas = int64(len(sessionsSet))

	// Top 5 emotions
	profile.EmocoesMaisComuns = topNKeys(emotionCounts, 5)

	// Top 10 topics
	profile.TopicosFrequentes = topNKeys(topicCounts, 10)

	return profile, nil
}

// topNKeys returns top N keys from a count map sorted by count descending
func topNKeys(counts map[string]int64, n int) []string {
	type kv struct {
		Key   string
		Value int64
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})
	var result []string
	for i, item := range sorted {
		if i >= n {
			break
		}
		result = append(result, item.Key)
	}
	return result
}

// GetAllPatientsMemoryProfiles retorna perfil resumido de todos os pacientes
func (m *MemoryInvestigator) GetAllPatientsMemoryProfiles(ctx context.Context) ([]map[string]interface{}, error) {
	// Get all active patients
	patients, err := m.db.QueryByLabel(ctx, "Idoso", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return nil, err
	}

	// Get all memories
	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Group memories by patient
	memByPatient := make(map[int64][]map[string]interface{})
	for _, mem := range allMemories {
		idosoID := database.GetInt64(mem, "idoso_id")
		memByPatient[idosoID] = append(memByPatient[idosoID], mem)
	}

	var profiles []map[string]interface{}
	for _, patient := range patients {
		patientID := database.GetInt64(patient, "pg_id")
		if patientID == 0 {
			patientID = database.GetInt64(patient, "id")
		}
		nome := database.GetString(patient, "nome")
		mems := memByPatient[patientID]

		total := int64(len(mems))
		primeiraStr := "N/A"
		ultimaStr := "N/A"
		var totalImportance float64
		var primeira, ultima time.Time

		for _, mem := range mems {
			ts := database.GetTime(mem, "timestamp")
			if primeira.IsZero() || ts.Before(primeira) {
				primeira = ts
			}
			if ts.After(ultima) {
				ultima = ts
			}
			totalImportance += database.GetFloat64(mem, "importance")
		}

		if !primeira.IsZero() {
			primeiraStr = primeira.Format("02/01/2006")
		}
		if !ultima.IsZero() {
			ultimaStr = ultima.Format("02/01/2006 15:04")
		}

		avgImportance := 0.0
		if total > 0 {
			avgImportance = totalImportance / float64(total)
		}

		profiles = append(profiles, map[string]interface{}{
			"id":          patientID,
			"nome":        nome,
			"memorias":    total,
			"primeira":    primeiraStr,
			"ultima":      ultimaStr,
			"importancia": fmt.Sprintf("%.2f", avgImportance),
		})
	}

	// Sort by memory count descending
	sort.Slice(profiles, func(i, j int) bool {
		mi := profiles[i]["memorias"].(int64)
		mj := profiles[j]["memorias"].(int64)
		return mi > mj
	})

	return profiles, nil
}

// ═══════════════════════════════════════════════════════════
// 📅 TIMELINE DE MEMÓRIAS
// ═══════════════════════════════════════════════════════════

// GetMemoryTimeline retorna timeline de memórias
func (m *MemoryInvestigator) GetMemoryTimeline(ctx context.Context, idosoID *int64, days int) ([]MemoryTimeline, error) {
	extraWhere := ""
	params := map[string]interface{}{}

	if idosoID != nil {
		extraWhere = " AND n.idoso_id = $idoso_id"
		params["idoso_id"] = *idosoID
	}

	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", extraWhere, params, 0)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Group by date
	type dayStats struct {
		Total    int64
		User     int64
		EVA      int64
		Emotions map[string]bool
	}
	byDate := make(map[string]*dayStats)

	for _, row := range rows {
		ts := database.GetTime(row, "timestamp")
		if ts.Before(cutoff) {
			continue
		}

		dateKey := ts.Format("2006-01-02")
		if byDate[dateKey] == nil {
			byDate[dateKey] = &dayStats{Emotions: make(map[string]bool)}
		}

		ds := byDate[dateKey]
		ds.Total++

		speaker := database.GetString(row, "speaker")
		if speaker == "user" {
			ds.User++
		} else if speaker == "assistant" {
			ds.EVA++
		}

		emotion := database.GetString(row, "emotion")
		if emotion != "" {
			ds.Emotions[emotion] = true
		}
	}

	// Convert to sorted slice
	var timeline []MemoryTimeline
	for date, ds := range byDate {
		var emotionList []string
		for e := range ds.Emotions {
			emotionList = append(emotionList, e)
		}
		timeline = append(timeline, MemoryTimeline{
			Date:          date,
			TotalMemories: ds.Total,
			UserMessages:  ds.User,
			EVAMessages:   ds.EVA,
			Emotions:      strings.Join(emotionList, ", "),
		})
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Date > timeline[j].Date // descending
	})

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

	// Fetch all memories
	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	integrity.TotalChecked = int64(len(allMemories))

	// Get all patient IDs for orphan detection
	allPatients, _ := m.db.QueryByLabel(ctx, "Idoso", "", nil, 0)
	patientIDs := make(map[int64]bool)
	for _, p := range allPatients {
		pid := database.GetInt64(p, "pg_id")
		if pid == 0 {
			pid = database.GetInt64(p, "id")
		}
		patientIDs[pid] = true
	}

	// Track duplicates
	type dupKey struct {
		IdosoID   int64
		Content   string
		MinuteTS  string
	}
	dupTracker := make(map[dupKey]int)

	for _, row := range allMemories {
		idosoID := database.GetInt64(row, "idoso_id")
		content := database.GetString(row, "content")
		ts := database.GetTime(row, "timestamp")

		// Orphan check
		if !patientIDs[idosoID] {
			integrity.MemoriesOrfas++
		}

		// Empty content check
		if content == "" || len(strings.TrimSpace(content)) == 0 {
			integrity.MemoriasSemConteudo++
		}

		// Embedding check
		_, hasEmb := row["embedding"]
		if !hasEmb || row["embedding"] == nil {
			integrity.MemoriasSemEmbedding++
		}

		// Duplicate tracking
		key := dupKey{
			IdosoID:  idosoID,
			Content:  content,
			MinuteTS: ts.Truncate(time.Minute).Format(time.RFC3339),
		}
		dupTracker[key]++
	}

	// Count duplicates (entries with count > 1, sum of extras)
	for _, count := range dupTracker {
		if count > 1 {
			integrity.MemoriasDuplicadas += int64(count - 1)
		}
	}

	// Build problem list
	if integrity.MemoriesOrfas > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias órfãs (paciente não existe)", integrity.MemoriesOrfas))
	}
	if integrity.MemoriasSemConteudo > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias sem conteúdo", integrity.MemoriasSemConteudo))
	}
	if integrity.MemoriasSemEmbedding > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d memórias sem embedding vetorial", integrity.MemoriasSemEmbedding))
	}
	if integrity.MemoriasDuplicadas > 0 {
		integrity.Problemas = append(integrity.Problemas,
			fmt.Sprintf("%d possíveis memórias duplicadas", integrity.MemoriasDuplicadas))
	}

	// Define status
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
	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Get all patient IDs
	allPatients, _ := m.db.QueryByLabel(ctx, "Idoso", "", nil, 0)
	patientIDs := make(map[int64]bool)
	for _, p := range allPatients {
		pid := database.GetInt64(p, "pg_id")
		if pid == 0 {
			pid = database.GetInt64(p, "id")
		}
		patientIDs[pid] = true
	}

	var orphans []MemoryDetail
	for _, row := range allMemories {
		idosoID := database.GetInt64(row, "idoso_id")
		if patientIDs[idosoID] {
			continue
		}

		mem := m.rowToMemoryDetail(ctx, row)
		mem.IdosoNome = "PACIENTE REMOVIDO"
		orphans = append(orphans, mem)

		if len(orphans) >= limit {
			break
		}
	}

	return orphans, nil
}

// GetDuplicateMemories retorna possíveis memórias duplicadas
func (m *MemoryInvestigator) GetDuplicateMemories(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Group by (idoso_id, content)
	type dupInfo struct {
		IdosoID int64
		Content string
		Count   int64
		First   time.Time
		Last    time.Time
	}
	type dupKey struct {
		IdosoID int64
		Content string
	}
	dupMap := make(map[dupKey]*dupInfo)

	for _, row := range allMemories {
		idosoID := database.GetInt64(row, "idoso_id")
		content := database.GetString(row, "content")
		ts := database.GetTime(row, "timestamp")

		key := dupKey{IdosoID: idosoID, Content: content}
		if d, exists := dupMap[key]; exists {
			d.Count++
			if ts.Before(d.First) {
				d.First = ts
			}
			if ts.After(d.Last) {
				d.Last = ts
			}
		} else {
			dupMap[key] = &dupInfo{
				IdosoID: idosoID,
				Content: content,
				Count:   1,
				First:   ts,
				Last:    ts,
			}
		}
	}

	// Filter to only duplicates and sort by count
	var dups []*dupInfo
	for _, d := range dupMap {
		if d.Count > 1 {
			dups = append(dups, d)
		}
	}
	sort.Slice(dups, func(i, j int) bool {
		return dups[i].Count > dups[j].Count
	})

	var duplicates []map[string]interface{}
	for i, d := range dups {
		if i >= limit {
			break
		}
		duplicates = append(duplicates, map[string]interface{}{
			"idoso_id":   d.IdosoID,
			"conteudo":   truncateString(d.Content, 100),
			"duplicatas": d.Count,
			"primeira":   d.First.Format("02/01/2006 15:04"),
			"ultima":     d.Last.Format("02/01/2006 15:04"),
		})
	}

	return duplicates, nil
}

// ═══════════════════════════════════════════════════════════
// 📊 ANÁLISE AVANÇADA
// ═══════════════════════════════════════════════════════════

// GetEmotionAnalysis analisa emoções nas memórias
func (m *MemoryInvestigator) GetEmotionAnalysis(ctx context.Context, idosoID *int64) (map[string]interface{}, error) {
	analysis := make(map[string]interface{})

	extraWhere := ""
	params := map[string]interface{}{}
	if idosoID != nil {
		extraWhere = " AND n.idoso_id = $idoso_id"
		params["idoso_id"] = *idosoID
	}

	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", extraWhere, params, 0)
	if err != nil {
		return nil, err
	}

	// Emotion distribution
	emotionCounts := make(map[string]int64)
	total := int64(len(rows))
	weekAgo := time.Now().AddDate(0, 0, -7)

	positiveEmotions := map[string]bool{"feliz": true, "alegre": true, "satisfeito": true, "calmo": true, "esperançoso": true}
	negativeEmotions := map[string]bool{"triste": true, "ansioso": true, "irritado": true, "preocupado": true, "frustrado": true}

	var recentPositive, recentNegative int64

	for _, row := range rows {
		emotion := database.GetString(row, "emotion")
		if emotion == "" {
			emotion = "indefinido"
		}
		emotionCounts[emotion]++

		ts := database.GetTime(row, "timestamp")
		if !ts.Before(weekAgo) {
			if positiveEmotions[emotion] {
				recentPositive++
			}
			if negativeEmotions[emotion] {
				recentNegative++
			}
		}
	}

	var emotions []map[string]interface{}
	for emotion, count := range emotionCounts {
		pct := float64(0)
		if total > 0 {
			pct = float64(count) * 100.0 / float64(total)
		}
		emotions = append(emotions, map[string]interface{}{
			"emotion":    emotion,
			"total":      count,
			"percentual": fmt.Sprintf("%.1f%%", pct),
		})
	}

	// Sort by total descending
	sort.Slice(emotions, func(i, j int) bool {
		return emotions[i]["total"].(int64) > emotions[j]["total"].(int64)
	})

	analysis["distribuicao"] = emotions
	analysis["tendencia"] = map[string]interface{}{
		"positivas_7dias": recentPositive,
		"negativas_7dias": recentNegative,
		"balanco":         recentPositive - recentNegative,
	}

	return analysis, nil
}

// GetTopicAnalysis analisa tópicos nas memórias
func (m *MemoryInvestigator) GetTopicAnalysis(ctx context.Context, idosoID *int64, limit int) ([]map[string]interface{}, error) {
	extraWhere := ""
	params := map[string]interface{}{}
	if idosoID != nil {
		extraWhere = " AND n.idoso_id = $idoso_id"
		params["idoso_id"] = *idosoID
	}

	rows, err := m.db.QueryByLabel(ctx, "EpisodicMemory", extraWhere, params, 0)
	if err != nil {
		return nil, err
	}

	// Aggregate topic data
	type topicInfo struct {
		Mentions  int64
		Patients  map[int64]bool
		FirstSeen time.Time
		LastSeen  time.Time
	}
	topicMap := make(map[string]*topicInfo)

	for _, row := range rows {
		idoso := database.GetInt64(row, "idoso_id")
		ts := database.GetTime(row, "timestamp")
		topicsStr := database.GetString(row, "topics")
		for _, topic := range parseTopicsArray(topicsStr) {
			if topic == "" {
				continue
			}
			if ti, exists := topicMap[topic]; exists {
				ti.Mentions++
				ti.Patients[idoso] = true
				if ts.Before(ti.FirstSeen) {
					ti.FirstSeen = ts
				}
				if ts.After(ti.LastSeen) {
					ti.LastSeen = ts
				}
			} else {
				topicMap[topic] = &topicInfo{
					Mentions:  1,
					Patients:  map[int64]bool{idoso: true},
					FirstSeen: ts,
					LastSeen:  ts,
				}
			}
		}
	}

	// Convert and sort
	type sortable struct {
		Topic string
		Info  *topicInfo
	}
	var sorted []sortable
	for t, info := range topicMap {
		sorted = append(sorted, sortable{t, info})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Info.Mentions > sorted[j].Info.Mentions
	})

	var topics []map[string]interface{}
	for i, s := range sorted {
		if i >= limit {
			break
		}
		topics = append(topics, map[string]interface{}{
			"topico":    s.Topic,
			"mencoes":   s.Info.Mentions,
			"pacientes": int64(len(s.Info.Patients)),
			"primeira":  s.Info.FirstSeen.Format("02/01/2006"),
			"ultima":    s.Info.LastSeen.Format("02/01/2006"),
		})
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

	// Try JSON array first
	var jsonTopics []string
	if err := json.Unmarshal([]byte(s), &jsonTopics); err == nil {
		return jsonTopics
	}

	// Remove {} and parse (PostgreSQL array format)
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
			Example:     "EVA,me mostra as estatísticas de memória",
		},
		{
			Command:     "memoria_buscar",
			Description: "Busca memórias por texto ou filtros",
			Example:     "EVA,busca memórias sobre medicamentos",
		},
		{
			Command:     "memoria_paciente",
			Description: "Perfil de memória de um paciente específico",
			Example:     "EVA,mostra as memórias do paciente X",
		},
		{
			Command:     "memoria_timeline",
			Description: "Timeline de memórias dos últimos dias",
			Example:     "EVA,mostra a timeline de memórias",
		},
		{
			Command:     "memoria_integridade",
			Description: "Verificação de integridade das memórias",
			Example:     "EVA,verifica a integridade das memórias",
		},
		{
			Command:     "memoria_emocoes",
			Description: "Análise de emoções nas memórias",
			Example:     "EVA,analisa as emoções nas memórias",
		},
		{
			Command:     "memoria_topicos",
			Description: "Análise de tópicos mais mencionados",
			Example:     "EVA,quais são os tópicos mais falados?",
		},
		{
			Command:     "memoria_orfas",
			Description: "Lista memórias órfãs (sem paciente)",
			Example:     "EVA,tem memórias órfãs?",
		},
		{
			Command:     "memoria_duplicadas",
			Description: "Lista possíveis memórias duplicadas",
			Example:     "EVA,tem memórias duplicadas?",
		},
		{
			Command:     "memoria_perfis",
			Description: "Perfil de memória de todos os pacientes",
			Example:     "EVA,mostra os perfis de memória",
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
		builder.WriteString(fmt.Sprintf("Problema: %s\n", response.Message))
		return builder.String()
	}

	if response.Message != "" {
		builder.WriteString(fmt.Sprintf("%s\n", response.Message))
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("Resultado de %s:\n\n", response.Command))

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

	orphans, err := m.GetOrphanMemories(ctx, 100000)
	if err != nil {
		return nil, err
	}

	result.AffectedCount = int64(len(orphans))

	if dryRun {
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias órfãs seriam removidas (dry-run)", result.AffectedCount)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias órfãs encontradas", result.AffectedCount)
	} else {
		// Delete orphan memories via SoftDelete
		deleted := int64(0)
		for _, orphan := range orphans {
			err := m.db.SoftDelete(ctx, "EpisodicMemory", map[string]interface{}{"pg_id": orphan.ID})
			if err == nil {
				deleted++
			}
		}
		result.AffectedCount = deleted
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias órfãs removidas com sucesso", deleted)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias órfãs", deleted)
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

	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	// Find duplicates: same idoso_id + content + minute-truncated timestamp
	type dupKey struct {
		IdosoID  int64
		Content  string
		MinuteTS string
	}
	groups := make(map[dupKey][]map[string]interface{})

	for _, row := range allMemories {
		key := dupKey{
			IdosoID:  database.GetInt64(row, "idoso_id"),
			Content:  database.GetString(row, "content"),
			MinuteTS: database.GetTime(row, "timestamp").Truncate(time.Minute).Format(time.RFC3339),
		}
		groups[key] = append(groups[key], row)
	}

	// Identify duplicates to remove (keep oldest)
	var toDelete []int64
	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}
		// Sort by timestamp ascending
		sort.Slice(group, func(i, j int) bool {
			ti := database.GetTime(group[i], "timestamp")
			tj := database.GetTime(group[j], "timestamp")
			return ti.Before(tj)
		})
		// Mark all except first for deletion
		for _, row := range group[1:] {
			id := database.GetInt64(row, "pg_id")
			if id == 0 {
				id = database.GetInt64(row, "id")
			}
			toDelete = append(toDelete, id)
		}
	}

	result.AffectedCount = int64(len(toDelete))

	if dryRun {
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias duplicadas seriam removidas (dry-run)", result.AffectedCount)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias duplicadas encontradas", result.AffectedCount)
	} else {
		deleted := int64(0)
		for _, id := range toDelete {
			err := m.db.SoftDelete(ctx, "EpisodicMemory", map[string]interface{}{"pg_id": id})
			if err == nil {
				deleted++
			}
		}
		result.AffectedCount = deleted
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias duplicadas removidas com sucesso", deleted)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias duplicadas", deleted)
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

	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	var toDelete []int64

	for _, row := range allMemories {
		ts := database.GetTime(row, "timestamp")
		importance := database.GetFloat64(row, "importance")

		if ts.Before(cutoff) && importance < maxImportance {
			id := database.GetInt64(row, "pg_id")
			if id == 0 {
				id = database.GetInt64(row, "id")
			}
			toDelete = append(toDelete, id)
		}
	}

	result.AffectedCount = int64(len(toDelete))

	if dryRun {
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias antigas (>%d dias, importância <%.2f) seriam removidas", result.AffectedCount, olderThanDays, maxImportance)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias antigas encontradas", result.AffectedCount)
	} else {
		deleted := int64(0)
		for _, id := range toDelete {
			err := m.db.SoftDelete(ctx, "EpisodicMemory", map[string]interface{}{"pg_id": id})
			if err == nil {
				deleted++
			}
		}
		result.AffectedCount = deleted
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias antigas removidas com sucesso", deleted)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias antigas", deleted)
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

	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	var toDelete []int64
	for _, row := range allMemories {
		content := database.GetString(row, "content")
		if content == "" || len(strings.TrimSpace(content)) < 3 {
			id := database.GetInt64(row, "pg_id")
			if id == 0 {
				id = database.GetInt64(row, "id")
			}
			toDelete = append(toDelete, id)
		}
	}

	result.AffectedCount = int64(len(toDelete))

	if dryRun {
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias vazias/inválidas seriam removidas", result.AffectedCount)
		log.Printf("🧹 [CLEANUP] Simulação: %d memórias vazias encontradas", result.AffectedCount)
	} else {
		deleted := int64(0)
		for _, id := range toDelete {
			err := m.db.SoftDelete(ctx, "EpisodicMemory", map[string]interface{}{"pg_id": id})
			if err == nil {
				deleted++
			}
		}
		result.AffectedCount = deleted
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias vazias removidas com sucesso", deleted)
		log.Printf("🧹 [CLEANUP] Removidas %d memórias vazias", deleted)
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

// ArchiveOldMemories arquiva memórias antigas para label de histórico
func (m *MemoryInvestigator) ArchiveOldMemories(ctx context.Context, olderThanDays int, dryRun bool) (*CleanupResult, error) {
	if m.db == nil {
		return nil, fmt.Errorf("banco de dados não disponível")
	}

	result := &CleanupResult{
		Operation: fmt.Sprintf("arquivar_memorias_%d_dias", olderThanDays),
	}

	allMemories, err := m.db.QueryByLabel(ctx, "EpisodicMemory", "", nil, 0)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	var toArchive []map[string]interface{}

	for _, row := range allMemories {
		ts := database.GetTime(row, "timestamp")
		if ts.Before(cutoff) {
			toArchive = append(toArchive, row)
		}
	}

	result.AffectedCount = int64(len(toArchive))

	if dryRun {
		result.Status = "✅ SIMULAÇÃO"
		result.Message = fmt.Sprintf("%d memórias seriam arquivadas (>%d dias)", result.AffectedCount, olderThanDays)
	} else {
		archived := int64(0)
		for _, row := range toArchive {
			// Copy to archive label
			archiveContent := make(map[string]interface{})
			for k, v := range row {
				archiveContent[k] = v
			}
			archiveContent["node_label"] = "EpisodicMemoryArchive"
			archiveContent["archived_at"] = time.Now()

			_, insertErr := m.db.Insert(ctx, "EpisodicMemoryArchive", archiveContent)
			if insertErr != nil {
				continue
			}

			// Soft-delete original
			id := database.GetInt64(row, "pg_id")
			if id == 0 {
				id = database.GetInt64(row, "id")
			}
			m.db.SoftDelete(ctx, "EpisodicMemory", map[string]interface{}{"pg_id": id})
			archived++
		}
		result.AffectedCount = archived
		result.Status = "✅ CONCLUÍDO"
		result.Message = fmt.Sprintf("%d memórias arquivadas com sucesso", archived)
		log.Printf("🧹 [ARCHIVE] %d memórias movidas para arquivo", archived)
	}

	return result, nil
}

// GetCleanupCommands retorna comandos de limpeza disponíveis
func (m *MemoryInvestigator) GetCleanupCommands() []DebugCommand {
	return []DebugCommand{
		{
			Command:     "limpar_orfas",
			Description: "Remove memórias órfãs (sem paciente)",
			Example:     "EVA,limpa as memórias órfãs",
		},
		{
			Command:     "limpar_duplicadas",
			Description: "Remove memórias duplicadas",
			Example:     "EVA,limpa as memórias duplicadas",
		},
		{
			Command:     "limpar_vazias",
			Description: "Remove memórias sem conteúdo",
			Example:     "EVA,limpa as memórias vazias",
		},
		{
			Command:     "limpar_antigas",
			Description: "Remove memórias antigas (>90 dias, baixa importância)",
			Example:     "EVA,limpa as memórias antigas",
		},
		{
			Command:     "limpeza_completa",
			Description: "Executa limpeza completa (simulação)",
			Example:     "EVA,faz uma limpeza completa",
		},
		{
			Command:     "limpeza_executar",
			Description: "Executa limpeza completa (REAL - cuidado!)",
			Example:     "EVA,executa a limpeza de verdade",
		},
		{
			Command:     "arquivar_memorias",
			Description: "Arquiva memórias antigas (>180 dias)",
			Example:     "EVA,arquiva as memórias antigas",
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
