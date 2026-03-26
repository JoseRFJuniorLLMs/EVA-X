package memory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	retryPkg "eva/internal/brainstem/infrastructure/retry"
	nietzsche "nietzsche-sdk"
)

// =============================================================================
// CONSTANTES DE SEGURANÇA
// =============================================================================

// CREATOR_CPF é o CPF do criador da EVA
// ÚNICA pessoa autorizada a usar funções administrativas de deleção de memórias
var CREATOR_CPF = getCreatorCPFMemory()

func getCreatorCPFMemory() string {
	if cpf := os.Getenv("CREATOR_CPF"); cpf != "" {
		return cpf
	}
	return "64525430249"
}

// ErrUnauthorized é retornado quando alguém não autorizado tenta usar funções admin
var ErrUnauthorized = errors.New("acesso negado: apenas o criador pode executar esta função")

// Memory representa uma memória episódica armazenada.
//
// Modelada como memória mamífera: cada episódio grava a CENA COMPLETA
// (tempo + espaço + circunstância), não apenas o conteúdo.
// O hipocampo dos mamíferos codifica contexto situacional junto com o evento,
// permitindo reconstrução da cena inteira a partir de qualquer fragmento.
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
	IsArchived    bool      `json:"is_archived"`          // Indica se está no Cold Path (S3)

	// ═══════════════════════════════════════════════════════════════════
	// CONTEXTO MAMÍFERO — Tempo, Espaço, Circunstância
	// Cada memória episódica grava a cena completa, como o hipocampo faz.
	// ═══════════════════════════════════════════════════════════════════

	// --- TEMPO (além de Timestamp/EventDate) ---
	TimeOfDay string `json:"time_of_day,omitempty"` // "madrugada", "manha", "tarde", "noite"
	DayOfWeek string `json:"day_of_week,omitempty"` // "segunda", "terça", ... "domingo"

	// --- ESPAÇO ---
	Location    string `json:"location,omitempty"`     // Lugar mencionado ou inferido: "casa", "hospital", "jardim"
	Environment string `json:"environment,omitempty"`   // Contexto ambiental: "silêncio", "barulho", "música"

	// --- CIRCUNSTÂNCIA ---
	SocialContext  string   `json:"social_context,omitempty"`  // "sozinho", "familia", "publico", "cuidador"
	Stressors      []string `json:"stressors,omitempty"`       // ["luto", "hospital", "crise", "aniversário"]
	Urgency        string   `json:"urgency,omitempty"`         // "CRÍTICA", "ALTA", "MÉDIA", "BAIXA"
	AudioIntensity int      `json:"audio_intensity,omitempty"` // Intensidade da voz 1-10
	EmotionScore   float64  `json:"emotion_score,omitempty"`   // -1.0 (negativo) a 1.0 (positivo)
	SitIntensity   float64  `json:"sit_intensity,omitempty"`   // Força do contexto situacional 0-1

	// --- PESSOAS PRESENTES (inferidas da conversa) ---
	MentionedPersons []string `json:"mentioned_persons,omitempty"` // ["filho", "médico", "vizinha"]
}

// MemoryStore gerencia o armazenamento de memórias
type MemoryStore struct {
	db            *database.DB
	graphStore    *GraphStore                   // Para salvar relações no NietzscheDB graph
	vectorAdapter *nietzscheInfra.VectorAdapter // Para salvar vetores no NietzscheDB
}

// NewMemoryStore cria um novo gerenciador de memórias
// graphStore é opcional - se nil, apenas NietzscheDB primary será usado
func NewMemoryStore(db *database.DB, graphStore *GraphStore, vectorAdapter *nietzscheInfra.VectorAdapter) *MemoryStore {
	if db == nil {
		log.Printf("⚠️ [STORAGE] NietzscheDB unavailable — running in degraded mode")
	}
	return &MemoryStore{
		db:            db,
		graphStore:    graphStore,
		vectorAdapter: vectorAdapter,
	}
}

// Store salva uma nova memória no banco
// ✅ NietzscheDB primary storage + graph + vector
func (m *MemoryStore) Store(ctx context.Context, memory *Memory) error {
	// 1. ✅ Salvar no NietzscheDB
	if m.db != nil {
		memory.Timestamp = time.Now()

		var callHistoryID interface{}
		if memory.CallHistoryID != nil {
			callHistoryID = float64(*memory.CallHistoryID)
		}

		// ═══════════════════════════════════════════════════════════
		// Auto-preencher campos temporais se vazios (sempre gravar)
		// ═══════════════════════════════════════════════════════════
		if memory.TimeOfDay == "" {
			memory.TimeOfDay = GetTimeOfDayPT(memory.Timestamp)
		}
		if memory.DayOfWeek == "" {
			memory.DayOfWeek = GetDayOfWeekPT(memory.Timestamp)
		}

		content := map[string]interface{}{
			// --- Core ---
			"idoso_id":        float64(memory.IdosoID),
			"speaker":         memory.Speaker,
			"content":         memory.Content,
			"emotion":         memory.Emotion,
			"importance":      memory.Importance,
			"topics":          memory.Topics,
			"session_id":      memory.SessionID,
			"call_history_id": callHistoryID,
			"event_date":      memory.EventDate.Format(time.RFC3339),
			"is_atomic":       memory.IsAtomic,
			"is_archived":     false,
			"timestamp":       memory.Timestamp.Format(time.RFC3339),

			// --- TEMPO ---
			"time_of_day": memory.TimeOfDay,
			"day_of_week": memory.DayOfWeek,

			// --- ESPAÇO ---
			"location":    memory.Location,
			"environment": memory.Environment,

			// --- CIRCUNSTÂNCIA ---
			"social_context":   memory.SocialContext,
			"stressors":        memory.Stressors,
			"urgency":          memory.Urgency,
			"audio_intensity":  memory.AudioIntensity,
			"emotion_score":    memory.EmotionScore,
			"sit_intensity":    memory.SitIntensity,
			"mentioned_persons": memory.MentionedPersons,
		}

		id, err := m.db.Insert(ctx, "episodic_memories", content)
		if err != nil {
			return fmt.Errorf("NietzscheDB save failed: %w", err)
		}
		memory.ID = id

		log.Printf("✅ [STORAGE] Memória salva no NietzscheDB: ID=%d, idoso=%d, speaker=%s",
			memory.ID, memory.IdosoID, memory.Speaker)
	} else {
		log.Printf("⚠️ [STORAGE] NietzscheDB unavailable — skipping save for memory")
		memory.Timestamp = time.Now()
	}

	// 2. ✅ Salvar relações no NietzscheDB graph
	if m.graphStore != nil {
		if err := m.graphStore.AddEpisodicMemory(ctx, memory); err != nil {
			// NÃO falhar a operação, mas logar claramente
			log.Printf("❌ [GRAPH] Falha ao salvar relações para memória %d: %v", memory.ID, err)
			log.Printf("⚠️ [GRAPH] Memória salva no NietzscheDB MAS relações graph falharam!")
		} else {
			log.Printf("✅ [GRAPH] Relações salvas: %d topics, emoção=%s (memória %d)",
				len(memory.Topics), memory.Emotion, memory.ID)
		}
	}

	// 3. Salvar vetor no NietzscheDB
	if m.vectorAdapter != nil && len(memory.Embedding) > 0 {
		payload := map[string]interface{}{
			// --- Core ---
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

			// --- TEMPO ---
			"time_of_day": memory.TimeOfDay,
			"day_of_week": memory.DayOfWeek,

			// --- ESPAÇO ---
			"location":    memory.Location,
			"environment": memory.Environment,

			// --- CIRCUNSTÂNCIA ---
			"social_context":    memory.SocialContext,
			"stressors":         memory.Stressors,
			"urgency":           memory.Urgency,
			"audio_intensity":   memory.AudioIntensity,
			"emotion_score":     memory.EmotionScore,
			"sit_intensity":     memory.SitIntensity,
			"mentioned_persons": memory.MentionedPersons,
		}

		// FASE 7 FIX: Retry vector upsert for transient NietzscheDB failures
		err := retryPkg.Do(ctx, retryPkg.FastConfig(), func(ctx context.Context) error {
			return m.vectorAdapter.Upsert(ctx, "memories", fmt.Sprintf("%d", memory.ID), memory.Embedding, payload)
		})
		if err != nil {
			log.Printf("❌ [VECTOR] Falha ao salvar vetor para memória %d após retries: %v", memory.ID, err)
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
		log.Printf("⚠️ [VECTOR] VectorAdapter não disponível ou embedding vazio - vetor NÃO salvo")
	}

	return nil
}

// GetByID recupera uma memória por ID.
// Tenta VectorAdapter primeiro, fallback para NietzscheDB primary.
func (m *MemoryStore) GetByID(ctx context.Context, id int64) (*Memory, error) {
	// 1. Tentar VectorAdapter primeiro (cache de vetores)
	if m.vectorAdapter != nil {
		payload, found, err := m.vectorAdapter.GetNodeByID(ctx, "memories", fmt.Sprintf("%d", id))
		if err == nil && found && payload != nil {
			mem := memoryFromPayload(payload)
			if mem != nil && mem.ID > 0 {
				log.Printf("✅ [STORAGE] GetByID %d servido do VectorAdapter", id)
				return mem, nil
			}
		}
	}

	// 2. Fallback para NietzscheDB primary
	if m.db == nil {
		return nil, fmt.Errorf("memory %d not found (NietzscheDB unavailable)", id)
	}

	content, err := m.db.GetNodeByID(ctx, "episodic_memories", id)
	if err != nil {
		return nil, fmt.Errorf("GetByID %d failed: %w", id, err)
	}
	if content == nil {
		return nil, fmt.Errorf("memory %d not found", id)
	}

	mem := memoryFromPayload(content)
	if mem == nil {
		return nil, fmt.Errorf("memory %d: failed to parse content", id)
	}

	log.Printf("✅ [STORAGE] GetByID %d servido do NietzscheDB primary", id)
	return mem, nil
}

// GetRecent retorna as N memórias mais recentes de um idoso.
// Tenta NietzscheDB NQL (VectorAdapter) primeiro, fallback para NietzscheDB primary.
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

	// 2. Fallback para NietzscheDB primary (QueryByLabel)
	if m.db == nil {
		return nil, fmt.Errorf("no recent memories (NietzscheDB unavailable)")
	}

	rows, err := m.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": float64(idosoID)}, 0)
	if err != nil {
		return nil, fmt.Errorf("GetRecent query failed: %w", err)
	}

	var memories []*Memory
	for _, row := range rows {
		if mem := memoryFromPayload(row); mem != nil && mem.ID > 0 {
			memories = append(memories, mem)
		}
	}

	// Sort by timestamp DESC in Go
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].Timestamp.After(memories[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(memories) > limit {
		memories = memories[:limit]
	}

	log.Printf("✅ [STORAGE] GetRecent idoso=%d: %d memórias do NietzscheDB primary", idosoID, len(memories))
	return memories, nil
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
// ⚠️  FUNÇÃO RESTRITA - Apenas o Criador (CPF via CREATOR_CPF env) pode usar
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

	log.Printf("🔧 [ADMIN] DeleteOld autorizado para o Criador")
	log.Printf("🔧 [ADMIN] Parâmetros: idosoID=%d, olderThanDays=%d, minImportance=%.2f",
		idosoID, olderThanDays, minImportance)

	// Default para minImportance
	if minImportance <= 0 {
		minImportance = 0.7
	}

	// Query all episodic_memories (optionally filtered by idoso_id)
	extraWhere := ""
	params := map[string]interface{}{}
	if idosoID != 0 {
		extraWhere = " AND n.idoso_id = $idoso_id"
		params["idoso_id"] = float64(idosoID)
	}

	rows, err := m.db.QueryByLabel(ctx, "episodic_memories", extraWhere, params, 0)
	if err != nil {
		log.Printf("❌ [ADMIN] Erro em DeleteOld query: %v", err)
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	var deleted int64

	for _, row := range rows {
		mem := memoryFromPayload(row)
		if mem == nil {
			continue
		}
		// Filter: older than cutoff AND importance below threshold
		if mem.Timestamp.Before(cutoff) && mem.Importance < minImportance {
			err := m.db.SoftDelete(ctx, "episodic_memories", map[string]interface{}{
				"id": float64(mem.ID),
			})
			if err != nil {
				log.Printf("⚠️ [ADMIN] Falha ao soft-delete memória %d: %v", mem.ID, err)
				continue
			}
			deleted++
		}
	}

	log.Printf("✅ [ADMIN] DeleteOld concluído: %d memórias removidas (soft-delete)", deleted)
	return deleted, nil
}

// DeleteAllMemories remove TODAS as memórias de um paciente
//
// ⚠️  FUNÇÃO RESTRITA - Apenas o Criador (CPF via CREATOR_CPF env) pode usar
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

	log.Printf("🔧 [ADMIN] DeleteAllMemories autorizado para o Criador")
	log.Printf("⚠️  [ADMIN] DELETANDO TODAS as memórias do idoso %d", idosoID)

	rows, err := m.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": float64(idosoID)}, 0)
	if err != nil {
		log.Printf("❌ [ADMIN] Erro em DeleteAllMemories query: %v", err)
		return 0, err
	}

	var deleted int64
	for _, row := range rows {
		memID := database.GetInt64(row, "id")
		if memID == 0 {
			continue
		}
		err := m.db.SoftDelete(ctx, "episodic_memories", map[string]interface{}{
			"id": float64(memID),
		})
		if err != nil {
			log.Printf("⚠️ [ADMIN] Falha ao soft-delete memória %d: %v", memID, err)
			continue
		}
		deleted++
	}

	log.Printf("✅ [ADMIN] DeleteAllMemories concluído: %d memórias removidas do idoso %d (soft-delete)", deleted, idosoID)
	return deleted, nil
}

// GetMemoryStats retorna estatísticas de memórias (função admin)
//
// ⚠️  FUNÇÃO RESTRITA - Apenas o Criador (CPF via CREATOR_CPF env) pode usar
func (m *MemoryStore) GetMemoryStats(ctx context.Context, requesterCPF string) (map[string]interface{}, error) {
	if m.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}
	if !isCreator(requesterCPF) {
		return nil, ErrUnauthorized
	}

	stats := make(map[string]interface{})

	// Query all episodic_memories from NietzscheDB
	allRows, err := m.db.QueryByLabel(ctx, "episodic_memories", "", nil, 0)
	if err != nil {
		return nil, fmt.Errorf("GetMemoryStats query failed: %w", err)
	}

	// Total de memórias
	totalMemories := int64(len(allRows))
	stats["total_memories"] = totalMemories

	// Compute stats in Go
	patientsSet := make(map[int64]bool)
	importanceStats := map[string]int64{
		"critical (>=0.9)":  0,
		"important (0.7-0.9)": 0,
		"normal (0.5-0.7)":   0,
		"low (<0.5)":         0,
	}
	var oldest, newest time.Time

	for _, row := range allRows {
		mem := memoryFromPayload(row)
		if mem == nil {
			continue
		}

		// Distinct patients
		patientsSet[mem.IdosoID] = true

		// Importance categories
		switch {
		case mem.Importance >= 0.9:
			importanceStats["critical (>=0.9)"]++
		case mem.Importance >= 0.7:
			importanceStats["important (0.7-0.9)"]++
		case mem.Importance >= 0.5:
			importanceStats["normal (0.5-0.7)"]++
		default:
			importanceStats["low (<0.5)"]++
		}

		// Oldest / newest
		if oldest.IsZero() || mem.Timestamp.Before(oldest) {
			oldest = mem.Timestamp
		}
		if newest.IsZero() || mem.Timestamp.After(newest) {
			newest = mem.Timestamp
		}
	}

	totalPatients := int64(len(patientsSet))
	stats["total_patients_with_memories"] = totalPatients

	if totalPatients > 0 {
		stats["avg_memories_per_patient"] = float64(totalMemories) / float64(totalPatients)
	}

	stats["by_importance"] = importanceStats
	stats["oldest_memory"] = oldest
	stats["newest_memory"] = newest

	log.Printf("🔧 [ADMIN] GetMemoryStats executado pelo criador")

	return stats, nil
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

	// ═══════════════════════════════════════════════════════════
	// CONTEXTO MAMÍFERO — Tempo, Espaço, Circunstância
	// ═══════════════════════════════════════════════════════════

	// --- TEMPO ---
	if v, ok := payload["time_of_day"].(string); ok {
		mem.TimeOfDay = v
	}
	if v, ok := payload["day_of_week"].(string); ok {
		mem.DayOfWeek = v
	}

	// --- ESPAÇO ---
	if v, ok := payload["location"].(string); ok {
		mem.Location = v
	}
	if v, ok := payload["environment"].(string); ok {
		mem.Environment = v
	}

	// --- CIRCUNSTÂNCIA ---
	if v, ok := payload["social_context"].(string); ok {
		mem.SocialContext = v
	}
	if v, ok := payload["stressors"]; ok {
		switch s := v.(type) {
		case []interface{}:
			for _, item := range s {
				if str, ok := item.(string); ok {
					mem.Stressors = append(mem.Stressors, str)
				}
			}
		case []string:
			mem.Stressors = s
		}
	}
	if v, ok := payload["urgency"].(string); ok {
		mem.Urgency = v
	}
	if v, ok := payload["audio_intensity"]; ok {
		switch ai := v.(type) {
		case float64:
			mem.AudioIntensity = int(ai)
		case int:
			mem.AudioIntensity = ai
		}
	}
	if v, ok := payload["emotion_score"].(float64); ok {
		mem.EmotionScore = v
	}
	if v, ok := payload["sit_intensity"].(float64); ok {
		mem.SitIntensity = v
	}
	if v, ok := payload["mentioned_persons"]; ok {
		switch p := v.(type) {
		case []interface{}:
			for _, item := range p {
				if str, ok := item.(string); ok {
					mem.MentionedPersons = append(mem.MentionedPersons, str)
				}
			}
		case []string:
			mem.MentionedPersons = p
		}
	}

	return mem
}

// ═══════════════════════════════════════════════════════════════════════
// HELPERS TEMPORAIS — Auto-preenchimento de tempo/espaço
// ═══════════════════════════════════════════════════════════════════════

// GetTimeOfDayPT retorna o período do dia em português
func GetTimeOfDayPT(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 0 && hour < 6:
		return "madrugada"
	case hour >= 6 && hour < 12:
		return "manha"
	case hour >= 12 && hour < 18:
		return "tarde"
	default:
		return "noite"
	}
}

// GetDayOfWeekPT retorna o dia da semana em português
func GetDayOfWeekPT(t time.Time) string {
	days := map[time.Weekday]string{
		time.Sunday:    "domingo",
		time.Monday:    "segunda",
		time.Tuesday:   "terça",
		time.Wednesday: "quarta",
		time.Thursday:  "quinta",
		time.Friday:    "sexta",
		time.Saturday:  "sábado",
	}
	return days[t.Weekday()]
}

// InferLocationFromText tenta extrair local mencionado no texto
func InferLocationFromText(text string) string {
	lower := strings.ToLower(text)
	locationPatterns := map[string][]string{
		"hospital":  {"hospital", "internado", "internação", "uti", "emergência", "enfermaria"},
		"casa":      {"em casa", "minha casa", "na sala", "no quarto", "na cozinha", "na varanda"},
		"rua":       {"na rua", "caminhando", "passeando", "mercado", "supermercado", "padaria"},
		"igreja":    {"igreja", "missa", "culto", "templo"},
		"médico":    {"consultório", "clínica", "posto de saúde", "médico"},
		"farmácia":  {"farmácia", "drogaria"},
		"jardim":    {"jardim", "quintal", "horta", "parque"},
	}
	for location, keywords := range locationPatterns {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return location
			}
		}
	}
	return ""
}

// InferMentionedPersons extrai pessoas mencionadas no texto
func InferMentionedPersons(text string) []string {
	lower := strings.ToLower(text)
	personKeywords := map[string][]string{
		"filho":    {"filho", "meu filho", "o menino"},
		"filha":    {"filha", "minha filha", "a menina"},
		"esposa":   {"esposa", "mulher", "minha mulher"},
		"marido":   {"marido", "meu marido", "meu esposo"},
		"mãe":      {"minha mãe", "mamãe", "a mãe"},
		"pai":      {"meu pai", "papai", "o pai"},
		"neto":     {"neto", "meu neto", "netinho"},
		"neta":     {"neta", "minha neta", "netinha"},
		"médico":   {"médico", "doutor", "doutora", "dr.", "dra."},
		"vizinha":  {"vizinha", "vizinho"},
		"cuidador": {"cuidador", "cuidadora", "enfermeira", "enfermeiro"},
		"amigo":    {"amigo", "amiga", "colega", "compadre", "comadre"},
	}
	var persons []string
	seen := map[string]bool{}
	for person, keywords := range personKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) && !seen[person] {
				persons = append(persons, person)
				seen[person] = true
				break
			}
		}
	}
	return persons
}

// GetArchivalCandidates localiza memórias que podem ser movidas para o Cold Path
func (m *MemoryStore) GetArchivalCandidates(ctx context.Context, idosoID int64, daysOld int, minImportance float64) ([]*Memory, error) {
	if m.db == nil {
		return nil, fmt.Errorf("NietzscheDB unavailable")
	}

	rows, err := m.db.QueryByLabel(ctx, "episodic_memories",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": float64(idosoID)}, 0)
	if err != nil {
		return nil, fmt.Errorf("GetArchivalCandidates query failed: %w", err)
	}

	archivalDate := time.Now().AddDate(0, 0, -daysOld)
	var memories []*Memory

	for _, row := range rows {
		mem := memoryFromPayload(row)
		if mem == nil {
			continue
		}
		// Read is_archived from payload
		if v, ok := row["is_archived"].(bool); ok {
			mem.IsArchived = v
		}
		// Filter: not archived, importance <= threshold, event_date older than cutoff
		if !mem.IsArchived && mem.Importance <= minImportance && mem.EventDate.Before(archivalDate) {
			memories = append(memories, mem)
		}
		if len(memories) >= 100 {
			break
		}
	}

	return memories, nil
}

// MarkAsArchived marca uma memória como arquivada e limpa o conteúdo pesado no buffer local
func (m *MemoryStore) MarkAsArchived(ctx context.Context, id int64) error {
	if m.db == nil {
		return fmt.Errorf("NietzscheDB unavailable")
	}
	return m.db.Update(ctx, "episodic_memories",
		map[string]interface{}{"id": float64(id)},
		map[string]interface{}{"is_archived": true})
}
