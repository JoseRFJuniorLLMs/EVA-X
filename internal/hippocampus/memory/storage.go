package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
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
}

// MemoryStore gerencia o armazenamento de memórias
type MemoryStore struct {
	db *sql.DB
}

// NewMemoryStore cria um novo gerenciador de memórias
func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

// Store salva uma nova memória no banco
func (m *MemoryStore) Store(ctx context.Context, memory *Memory) error {
	query := `
		INSERT INTO episodic_memories 
		(idoso_id, speaker, content, embedding, emotion, importance, topics, session_id, call_history_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, timestamp
	`

	embeddingStr := vectorToPostgres(memory.Embedding)

	err := m.db.QueryRowContext(
		ctx,
		query,
		memory.IdosoID,
		memory.Speaker,
		memory.Content,
		embeddingStr,
		memory.Emotion,
		memory.Importance,
		pqArray(memory.Topics),
		memory.SessionID,
		memory.CallHistoryID,
	).Scan(&memory.ID, &memory.Timestamp)

	return err
}

// GetByID recupera uma memória por ID
func (m *MemoryStore) GetByID(ctx context.Context, id int64) (*Memory, error) {
	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion, 
		       importance, topics, session_id, call_history_id
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
		&memory.CallHistoryID,
	)

	if err != nil {
		return nil, err
	}

	// Parse topics array
	memory.Topics = parsePostgresArray(topics)

	return memory, nil
}

// GetRecent retorna as N memórias mais recentes de um idoso
func (m *MemoryStore) GetRecent(ctx context.Context, idosoID int64, limit int) ([]*Memory, error) {
	query := `
		SELECT id, idoso_id, timestamp, speaker, content, emotion, 
		       importance, topics, session_id
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
			&memory.Emotion,
			&memory.Importance,
			&topics,
			&memory.SessionID,
		)

		if err != nil {
			return nil, err
		}

		memory.Topics = parsePostgresArray(topics)
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

// Helpers para conversão de tipos PostgreSQL

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
