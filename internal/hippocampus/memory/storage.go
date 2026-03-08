package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	nietzsche "nietzsche-sdk"
)

// =============================================================================
// CONSTANTES DE SEGURANÇA
// =============================================================================

// CREATOR_CPF é o CPF do criador da EVA - Jose R F Junior
// ÚNICA pessoa autorizada a usar funções administrativas de deleção de memórias
const CREATOR_CPF = "64525430249"

// ErrUnauthorized é retornado quando alguém não autorizado tenta usar funções admin
var ErrUnauthorized = errors.New("acesso negado: apenas o criador pode executar esta função")

// Memory representa uma memória episódica armazenada
type Memory struct {
	ID            int64     `json:"id"`
	IdosoID       int64     `json:"idoso_id"`
	Timestamp     time.Time `json:"timestamp"`
	Speaker       string    `json:"speaker"` // "user" ou "assistant"
	Content       string    `json:"content"`
	Embedding     []float32 `json:"-"` // Não serializar embedding (muito grande)
	Emotion       string    `json:"emotion"`
	Importance    float64   `json:"importance"`
	Topics        []string  `json:"topics"`
	SessionID     string    `json:"session_id,omitempty"`
	CallHistoryID *int64    `json:"call_history_id,omitempty"`
	EventDate     time.Time `json:"event_date,omitempty"` // Data real do evento
	IsAtomic      bool      `json:"is_atomic"`            // Flag de atomicidade
	IsArchived    bool      `json:"is_archived"`          // ✅ NEW: Indica se está no Cold Path (S3)
}

// MemoryStore gerencia o armazenamento de memórias
type MemoryStore struct {
	db            *sql.DB
	graphStore    *GraphStore                   // Para salvar relações no NietzscheDB graph
	vectorAdapter *nietzscheInfra.VectorAdapter // Para salvar vetores no NietzscheDB
}

// NewMemoryStore cria um novo gerenciador de memórias
// graphStore é opcional - se nil, apenas Postgres será usado
func NewMemoryStore(db *sql.DB, graphStore *GraphStore, vectorAdapter *nietzscheInfra.VectorAdapter) *MemoryStore {
	if db == nil {
		log.Printf("⚠️ [STORAGE] NietzscheDB unavailable — running in degraded mode (NietzscheDB only)")
	}
	return &MemoryStore{
		db:            db,
		graphStore:    graphStore,
		vectorAdapter: vectorAdapter,
	}
}

// Store salva uma nova memória no banco
// ✅ CORREÇÃO P5: Agora salva no Postgres E NietzscheDB graph
func (m *MemoryStore) Store(ctx context.Context, memory *Memory) error {
	// 1. ✅ Salvar no Postgres (Sem vetor)
	if m.db != nil {
		query := `
			INSERT INTO episodic_memories
			(idoso_id, speaker, content, emotion, importance, topics, session_id, call_history_id, event_date, is_atomic)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, timestamp
		`

		err := m.db.QueryRowContext(
			ctx,
			query,
			memory.IdosoID,
			memory.Speaker,
			memory.Content,
			memory.Emotion,
			memory.Importance,
			pqArray(memory.Topics),
			memory.SessionID,
			memory.CallHistoryID,
			memory.EventDate,
			memory.IsAtomic,
		).Scan(&memory.ID, &memory.Timestamp)

		if err != nil {
			return fmt.Errorf("postgres save failed: %w", err)
		}

		log.Printf("✅ [STORAGE] Memória salva no Postgres: ID=%d, idoso=%d, speaker=%s",
			memory.ID, memory.IdosoID, memory.Speaker)
	} else {
		log.Printf("⚠️ [STORAGE] NietzscheDB unavailable — skipping Postgres save for memory")
		memory.Timestamp = time.Now()
	}

	// 2. ✅ Salvar relações no NietzscheDB graph
	if m.graphStore != nil {
		if err := m.graphStore.AddEpisodicMemory(ctx, memory); err != nil {
			// NÃO falhar a operação, mas logar claramente
			log.Printf("❌ [GRAPH] Falha ao salvar relações para memória %d: %v", memory.ID, err)
			log.Printf("⚠️ [GRAPH] Memória salva no Postgres MAS relações NietzscheDB graph falharam!")
		} else {
			log.Printf("✅ [GRAPH] Relações salvas: %d topics, emoção=%s (memória %d)",
				len(memory.Topics), memory.Emotion, memory.ID)
		}
	}

	// 3. Salvar vetor no NietzscheDB
	if m.vectorAdapter != nil && len(memory.Embedding) > 0 {
		payload := map[string]interface{}{
			"id":              memory.ID,
			"idoso_id":        memory.IdosoID,
			"content":         memory.Content,
			"speaker":         memory.Speaker,
			"emotion":         memory.Emotion,
			"importance":      memory.Importance,
			"topics":          memory.Topics,
			"timestamp":       memory.Timestamp.Format(time.RFC3339),
			"event_date":      memory.EventDate.Format(time.RFC3339),
			"is_atomic":       memory.IsAtomic,
			"session_id":      memory.SessionID,
			"call_history_id": memory.CallHistoryID,
		}

		err := m.vectorAdapter.Upsert(ctx, "memories", fmt.Sprintf("%d", memory.ID), memory.Embedding, payload)
		if err != nil {
			log.Printf("❌ [VECTOR] Falha ao salvar vetor para memória %d: %v", memory.ID, err)
		} else {
			log.Printf("✅ [VECTOR] Vetor salvo com sucesso: %d", memory.ID)

			// 4. ✅ INTEGRAÇÃO SENSORIAL (Fase 11 - Compressão Latente)
			// Armazenamos uma versão comprimida/latente para reconstrução rápida.
			// Em produção, isso viria de um encoder VAE; aqui usamos o embedding original.
			errSensory := m.vectorAdapter.InsertSensory(ctx, nietzsche.InsertSensoryOpts{
				NodeID:     fmt.Sprintf("%d", memory.ID),
				Modality:   "text",
				Latent:     memory.Embedding,
				Collection: "memories",
			})
			if errSensory != nil {
				log.Printf("⚠️ [SENSORY] Falha ao anexar dados sensoriais para memória %d: %v", memory.ID, errSensory)
			} else {
				log.Printf("✅ [SENSORY] Dados sensoriais (Fase 11) anexados à memória %d", memory.ID)
			}
		}
	} else {
		log.Printf("⚠️ [VECTOR] VectorAdapter não disponível ou embedding vazio - vetor NÃO salvo (apenas Postgres)")
	}

	return nil
}

// GetByID recupera uma memória por ID.
// Tenta NietzscheDB primeiro (mais rápido, sem round-trip PG), fallback para Postgres.
func (m *MemoryStore) GetByID(ctx context.Context, id int64) (*Memory, error) {
	// 1. Tentar NietzscheDB primeiro
	if m.vectorAdapter != nil {
		payload, found, err := m.vectorAdapter.GetNodeByID(ctx, "memories", fmt.Sprintf("%d", id))
		if err == nil && found && payload != nil {
			mem := memoryFromPayload(payload)
			if mem != nil && mem.ID > 0 {
				log.Printf("✅ [STORAGE] GetByID %d servido do NietzscheDB", id)
				return mem, nil
			}
		}
	}

	// 2. Fallback para Postgres
	if m.db == nil {
		return nil, fmt.Errorf("memory %d not found (NietzscheDB unavailable)", id)
	}

	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion,
		       importance, topics, session_id, call_history_id, event_date, is_atomic
		FROM episodic_memories
		WHERE id = $1
	`

	memory := &Memory{}
	var topics string

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&memory.ID,
		&memory.IdosoID,
		&memory.Timestamp,
		&memory.Speaker,
		&memory.Content,
		&memory.Emotion,
		&memory.Importance,
		&topics,
		&memory.SessionID,
		&memory.IsAtomic,
	)

	if err != nil {
		return nil, err
	}

	memory.Topics = parsePostgresArray(topics)
	return memory, nil
}

// GetRecent retorna as N memórias mais recentes de um idoso.
// Tenta NietzscheDB NQL primeiro, fallback para Postgres.
func (m *MemoryStore) GetRecent(ctx context.Context, idosoID int64, limit int) ([]*Memory, error) {
	// 1. Tentar NietzscheDB NQL
	if m.vectorAdapter != nil {
		nql := `MATCH (n) WHERE n.idoso_id = $idoso_id RETURN n ORDER BY n.timestamp DESC LIMIT $limit`
		params := map[string]interface{}{
			"idoso_id": idosoID,
			"limit":    limit,
		}
		payloads, err := m.vectorAdapter.ExecuteNQL(ctx, nql, params, "memories")
		if err == nil && len(payloads) > 0 {
			var memories []*Memory
			for _, p := range payloads {
				if mem := memoryFromPayload(p); mem != nil && mem.ID > 0 {
					memories = append(memories, mem)
				}
			}
			if len(memories) > 0 {
				log.Printf("✅ [STORAGE] GetRecent idoso=%d: %d memórias do NietzscheDB", idosoID, len(memories))
				return memories, nil
			}
		}
	}

	// 2. Fallback para Postgres
	if m.db == nil {
		return nil, fmt.Errorf("no recent memories (NietzscheDB unavailable)")
	}

	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion,
		       importance, topics, session_id, call_history_id, event_date, is_atomic
		FROM episodic_memories
		WHERE idoso_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := m.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return m.scanMemories(rows)
}

// =============================================================================
// FUNÇÕES ADMINISTRATIVAS (RESTRITAS AO CRIADOR)
// =============================================================================

// isCreator verifica se o CPF pertence ao criador da EVA
func isCreator(cpf string) bool {
	// Remove pontuação do CPF para comparação
	cleanCPF := strings.ReplaceAll(strings.ReplaceAll(cpf, ".", ""), "-", "")
	return cleanCPF == CREATOR_CPF
}

// DeleteOld remove memórias mais antigas que X dias
//
// ⚠️  FUNÇÃO RESTRITA - Apenas Jose R F Junior (CPF: 64525430249) pode usar
//
// Esta função NÃO é chamada automaticamente pelo sistema.
// Memórias são mantidas indefinidamente para preservar o contexto do paciente.
// Usar apenas para manutenção manual quando necessário.
//
// Parâmetros:
//   - requesterCPF: CPF de quem está solicitando (deve ser o criador)
//   - idosoID: ID do paciente (0 = todos os pacientes)
//   - olderThanDays: deletar memórias mais antigas que N dias
//   - minImportance: deletar apenas memórias com importance < este valor (default 0.7)
//
// Retorna:
//   - int64: número de memórias deletadas
//   - error: ErrUnauthorized se não for o criador
func (m *MemoryStore) DeleteOld(ctx context.Context, requesterCPF string, idosoID int64, olderThanDays int, minImportance float64) (int64, error) {
	if m.db == nil {
		return 0, fmt.Errorf("NietzscheDB unavailable")
	}
	// ═══════════════════════════════════════════════════════════════════════
	// VERIFICAÇÃO DE AUTORIZAÇÃO - APENAS O CRIADOR PODE USAR ESTA FUNÇÃO
	// ═══════════════════════════════════════════════════════════════════════
	if !isCreator(requesterCPF) {
		log.Printf("🚫 [SECURITY] Tentativa não autorizada de DeleteOld por CPF: %s", requesterCPF)
		return 0, ErrUnauthorized
	}

	log.Printf("🔧 [ADMIN] DeleteOld autorizado para criador Jose R F Junior")
	log.Printf("🔧 [ADMIN] Parâmetros: idosoID=%d, olderThanDays=%d, minImportance=%.2f",
		idosoID, olderThanDays, minImportance)

	// Default para minImportance
	if minImportance <= 0 {
		minImportance = 0.7
	}

	var query string
	var result sql.Result
	var err error

	if idosoID == 0 {
		// Deletar de TODOS os pacientes (usar com cuidado!)
		query = `
			DELETE FROM episodic_memories
			WHERE timestamp < NOW() - INTERVAL '1 day' * $1
			  AND importance < $2
		`
		result, err = m.db.ExecContext(ctx, query, olderThanDays, minImportance)
	} else {
		// Deletar apenas de um paciente específico
		query = `
			DELETE FROM episodic_memories
			WHERE idoso_id = $1
			  AND timestamp < NOW() - INTERVAL '1 day' * $2
			  AND importance < $3
		`
		result, err = m.db.ExecContext(ctx, query, idosoID, olderThanDays, minImportance)
	}

	if err != nil {
		log.Printf("❌ [ADMIN] Erro em DeleteOld: %v", err)
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ [ADMIN] DeleteOld concluído: %d memórias removidas", rowsAffected)

	return rowsAffected, nil
}

// DeleteAllMemories remove TODAS as memórias de um paciente
//
// ⚠️  FUNÇÃO RESTRITA - Apenas Jose R F Junior (CPF: 64525430249) pode usar
// ⚠️  CUIDADO: Esta função é DESTRUTIVA e não pode ser desfeita!
//
// Usar apenas para:
//   - Testes de desenvolvimento
//   - Solicitação explícita de "direito ao esquecimento" (LGPD Art. 18, VI)
func (m *MemoryStore) DeleteAllMemories(ctx context.Context, requesterCPF string, idosoID int64) (int64, error) {
	if m.db == nil {
		return 0, fmt.Errorf("NietzscheDB unavailable")
	}
	if !isCreator(requesterCPF) {
		log.Printf("🚫 [SECURITY] Tentativa não autorizada de DeleteAllMemories por CPF: %s", requesterCPF)
		return 0, ErrUnauthorized
	}

	log.Printf("🔧 [ADMIN] DeleteAllMemories autorizado para criador Jose R F Junior")
	log.Printf("⚠️  [ADMIN] DELETANDO TODAS as memórias do idoso %d", idosoID)

	query := `DELETE FROM episodic_memories WHERE idoso_id = $1`
	result, err := m.db.ExecContext(ctx, query, idosoID)
	if err != nil {
		log.Printf("❌ [ADMIN] Erro em DeleteAllMemories: %v", err)
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ [ADMIN] DeleteAllMemories concluído: %d memórias removidas do idoso %d", rowsAffected, idosoID)

	return rowsAffected, nil
}

// GetMemoryStats retorna estatísticas de memórias (função admin)
//
// ⚠️  FUNÇÃO RESTRITA - Apenas Jose R F Junior (CPF: 64525430249) pode usar
func (m *MemoryStore) GetMemoryStats(ctx context.Context, requesterCPF string) (map[string]interface{}, error) {
	if m.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	if !isCreator(requesterCPF) {
		return nil, ErrUnauthorized
	}

	stats := make(map[string]interface{})

	// Total de memórias
	var totalMemories int64
	m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM episodic_memories").Scan(&totalMemories)
	stats["total_memories"] = totalMemories

	// Memórias por paciente
	var totalPatients int64
	m.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT idoso_id) FROM episodic_memories").Scan(&totalPatients)
	stats["total_patients_with_memories"] = totalPatients

	// Média por paciente
	if totalPatients > 0 {
		stats["avg_memories_per_patient"] = float64(totalMemories) / float64(totalPatients)
	}

	// Memórias por importance
	rows, _ := m.db.QueryContext(ctx, `
		SELECT
			CASE
				WHEN importance >= 0.9 THEN 'critical (>=0.9)'
				WHEN importance >= 0.7 THEN 'important (0.7-0.9)'
				WHEN importance >= 0.5 THEN 'normal (0.5-0.7)'
				ELSE 'low (<0.5)'
			END as category,
			COUNT(*) as count
		FROM episodic_memories
		GROUP BY category
		ORDER BY category
	`)
	if rows != nil {
		defer rows.Close()
		importanceStats := make(map[string]int64)
		for rows.Next() {
			var category string
			var count int64
			rows.Scan(&category, &count)
			importanceStats[category] = count
		}
		stats["by_importance"] = importanceStats
	}

	// Memória mais antiga e mais recente
	var oldest, newest time.Time
	m.db.QueryRowContext(ctx, "SELECT MIN(timestamp), MAX(timestamp) FROM episodic_memories").Scan(&oldest, &newest)
	stats["oldest_memory"] = oldest
	stats["newest_memory"] = newest

	log.Printf("🔧 [ADMIN] GetMemoryStats executado pelo criador")

	return stats, nil
}

// scanMemories helper para converter rows em slice de Memory
func (m *MemoryStore) scanMemories(rows *sql.Rows) ([]*Memory, error) {
	var memories []*Memory

	for rows.Next() {
		memory := &Memory{}
		var topics string

		err := rows.Scan(
			&memory.ID,
			&memory.IdosoID,
			&memory.Timestamp,
			&memory.Speaker,
			&memory.Content,
			&memory.Topics,
			&memory.SessionID,
			&memory.CallHistoryID,
			&memory.EventDate,
			&memory.IsAtomic,
		)

		if err != nil {
			return nil, err
		}

		memory.Topics = parsePostgresArray(topics)
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

// memoryFromPayload constructs a Memory from a NietzscheDB node content payload.
// This eliminates the need to hydrate from NietzscheDB for data that's already
// stored in NietzscheDB (see Store() step 3 — vector + payload save).
func memoryFromPayload(payload map[string]interface{}) *Memory {
	if payload == nil {
		return nil
	}
	mem := &Memory{}

	// ID (stored as int64 or float64 from JSON)
	if v, ok := payload["id"]; ok {
		switch id := v.(type) {
		case float64:
			mem.ID = int64(id)
		case int64:
			mem.ID = id
		}
	}

	// IdosoID
	if v, ok := payload["idoso_id"]; ok {
		switch id := v.(type) {
		case float64:
			mem.IdosoID = int64(id)
		case int64:
			mem.IdosoID = id
		}
	}

	// String fields
	if v, ok := payload["content"].(string); ok {
		mem.Content = v
	}
	if v, ok := payload["speaker"].(string); ok {
		mem.Speaker = v
	}
	if v, ok := payload["emotion"].(string); ok {
		mem.Emotion = v
	}
	if v, ok := payload["session_id"].(string); ok {
		mem.SessionID = v
	}

	// Importance
	if v, ok := payload["importance"].(float64); ok {
		mem.Importance = v
	}

	// IsAtomic
	if v, ok := payload["is_atomic"].(bool); ok {
		mem.IsAtomic = v
	}

	// Topics ([]interface{} from JSON deserialize)
	if v, ok := payload["topics"]; ok {
		switch topics := v.(type) {
		case []interface{}:
			for _, t := range topics {
				if s, ok := t.(string); ok {
					mem.Topics = append(mem.Topics, s)
				}
			}
		case []string:
			mem.Topics = topics
		}
	}
	if mem.Topics == nil {
		mem.Topics = []string{}
	}

	// Timestamps (RFC3339 strings)
	if v, ok := payload["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			mem.Timestamp = t
		}
	}
	if v, ok := payload["event_date"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			mem.EventDate = t
		}
	}

	// CallHistoryID (*int64, nullable)
	if v, ok := payload["call_history_id"]; ok && v != nil {
		switch id := v.(type) {
		case float64:
			cid := int64(id)
			mem.CallHistoryID = &cid
		case int64:
			mem.CallHistoryID = &id
		}
	}

	return mem
}

// Helpers para conversão de tipos NietzscheDB

func vectorToPostgres(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}

	result := "["
	for i, v := range vec {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", v)
	}
	result += "]"

	return result
}

func pqArray(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}

	result := "{"
	for i, s := range arr {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("\"%s\"", s)
	}
	result += "}"

	return result
}

func parsePostgresArray(s string) []string {
	if s == "{}" || s == "" {
		return []string{}
	}

	// Remove {} e split por vírgula
	s = s[1 : len(s)-1]
	var result []string

	// Parse manual para lidar com aspas
	var current string
	inQuotes := false

	for _, c := range s {
		switch c {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				if current != "" {
					result = append(result, current)
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
		result = append(result, current)
	}

	return result
}

// GetArchivalCandidates localiza memórias que podem ser movidas para o Cold Path
func (m *MemoryStore) GetArchivalCandidates(ctx context.Context, idosoID int64, daysOld int, minImportance float64) ([]*Memory, error) {
	if m.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion, importance, topics, session_id, call_history_id, event_date, is_atomic, is_archived
		FROM episodic_memories
		WHERE idoso_id = $1 
		  AND is_archived = false
		  AND importance <= $2
		  AND event_date < $3
		LIMIT 100
	`

	archivalDate := time.Now().AddDate(0, 0, -daysOld)
	rows, err := m.db.QueryContext(ctx, query, idosoID, minImportance, archivalDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var mem Memory
		var topics string
		err := rows.Scan(
			&mem.ID, &mem.IdosoID, &mem.Timestamp, &mem.Speaker, &mem.Content,
			&mem.Emotion, &mem.Importance, &topics, &mem.SessionID,
			&mem.CallHistoryID, &mem.EventDate, &mem.IsAtomic, &mem.IsArchived,
		)
		if err != nil {
			return nil, err
		}
		mem.Topics = parsePostgresArray(topics)
		memories = append(memories, &mem)
	}

	return memories, nil
}

// MarkAsArchived marca uma memória como arquivada e limpa o conteúdo pesado no buffer local
func (m *MemoryStore) MarkAsArchived(ctx context.Context, id int64) error {
	if m.db == nil {
		return fmt.Errorf("NietzscheDB unavailable")
	}
	query := `UPDATE episodic_memories SET is_archived = true WHERE id = $1`
	_, err := m.db.ExecContext(ctx, query, id)
	return err
}
