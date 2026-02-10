package personality

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// PersonalityState representa o estado emocional da relação EVA <-> Idoso
type PersonalityState struct {
	IdosoID            int64
	RelationshipLevel  int // 1-10
	ConversationCount  int
	LastInteraction    time.Time
	DominantEmotion    string
	FavoriteTopics     []string
	FirstMeetingDate   time.Time
	DaysSinceFirstMeet int
}

// PersonalityService gerencia o estado de personalidade
type PersonalityService struct {
	db *sql.DB
}

// NewPersonalityService cria um novo serviço
func NewPersonalityService(db *sql.DB) *PersonalityService {
	return &PersonalityService{db: db}
}

// GetState recupera o estado de personalidade de um idoso
func (p *PersonalityService) GetState(ctx context.Context, idosoID int64) (*PersonalityState, error) {
	query := `
		SELECT 
			idoso_id,
			relationship_level,
			conversation_count,
			last_interaction,
			dominant_emotion,
			favorite_topics,
			first_meeting_date
		FROM eva_personality_state
		WHERE idoso_id = $1
	`

	state := &PersonalityState{}
	var topics string

	err := p.db.QueryRowContext(ctx, query, idosoID).Scan(
		&state.IdosoID,
		&state.RelationshipLevel,
		&state.ConversationCount,
		&state.LastInteraction,
		&state.DominantEmotion,
		&topics,
		&state.FirstMeetingDate,
	)

	if err == sql.ErrNoRows {
		// Primeira vez: criar estado inicial
		return p.initializeState(ctx, idosoID)
	} else if err != nil {
		return nil, err
	}

	// Parse topics
	if topics != "" && topics != "{}" {
		// Parse PostgreSQL array format
		state.FavoriteTopics = parsePostgresArray(topics)
	}

	// Calcular dias desde primeira conversa
	state.DaysSinceFirstMeet = int(time.Since(state.FirstMeetingDate).Hours() / 24)

	return state, nil
}

// initializeState cria um novo estado para um idoso
func (p *PersonalityService) initializeState(ctx context.Context, idosoID int64) (*PersonalityState, error) {
	query := `
		INSERT INTO eva_personality_state 
		(idoso_id, relationship_level, conversation_count, last_interaction, dominant_emotion, favorite_topics, first_meeting_date)
		VALUES ($1, 1, 0, NOW(), 'neutro', '{}', NOW())
		RETURNING idoso_id, relationship_level, conversation_count, last_interaction, dominant_emotion, favorite_topics, first_meeting_date
	`

	state := &PersonalityState{}
	var topics string

	err := p.db.QueryRowContext(ctx, query, idosoID).Scan(
		&state.IdosoID,
		&state.RelationshipLevel,
		&state.ConversationCount,
		&state.LastInteraction,
		&state.DominantEmotion,
		&topics,
		&state.FirstMeetingDate,
	)

	if err != nil {
		return nil, err
	}

	state.DaysSinceFirstMeet = 0
	state.FavoriteTopics = []string{}

	return state, nil
}

// UpdateAfterConversation atualiza o estado após uma conversa
func (p *PersonalityService) UpdateAfterConversation(ctx context.Context, idosoID int64, detectedEmotion string, topics []string) error {
	// Incrementar contador
	newCount := 0
	err := p.db.QueryRowContext(ctx, `
		UPDATE eva_personality_state 
		SET conversation_count = conversation_count + 1,
		    last_interaction = NOW()
		WHERE idoso_id = $1
		RETURNING conversation_count
	`, idosoID).Scan(&newCount)

	if err != nil {
		return err
	}

	// Calcular novo nível de relacionamento
	newLevel := CalculateRelationshipLevel(newCount)

	// Atualizar nível e emoção dominante
	_, err = p.db.ExecContext(ctx, `
		UPDATE eva_personality_state 
		SET relationship_level = $1,
		    dominant_emotion = $2
		WHERE idoso_id = $3
	`, newLevel, detectedEmotion, idosoID)

	// Atualizar tópicos favoritos (merge)
	if len(topics) > 0 {
		p.updateFavoriteTopics(ctx, idosoID, topics)
	}

	return err
}

// updateFavoriteTopics atualiza os tópicos favoritos (mantém top 5)
func (p *PersonalityService) updateFavoriteTopics(ctx context.Context, idosoID int64, newTopics []string) error {
	// Buscar tópicos atuais
	var currentTopicsStr string
	err := p.db.QueryRowContext(ctx, `SELECT favorite_topics FROM eva_personality_state WHERE idoso_id = $1`, idosoID).Scan(&currentTopicsStr)
	if err != nil {
		return err
	}

	currentTopics := parsePostgresArray(currentTopicsStr)

	// Merge e manter top 5 (simples: adicionar novos)
	merged := append(currentTopics, newTopics...)
	unique := uniqueStrings(merged)

	if len(unique) > 5 {
		unique = unique[:5]
	}

	// Atualizar
	topicsStr := toPostgresArray(unique)
	_, err = p.db.ExecContext(ctx, `
		UPDATE eva_personality_state 
		SET favorite_topics = $1 
		WHERE idoso_id = $2
	`, topicsStr, idosoID)

	return err
}

// GetDaysSinceLastInteraction retorna quantos dias desde última conversa
func (p *PersonalityService) GetDaysSinceLastInteraction(ctx context.Context, idosoID int64) (int, error) {
	var lastInteraction time.Time
	err := p.db.QueryRowContext(ctx, `
		SELECT last_interaction 
		FROM eva_personality_state 
		WHERE idoso_id = $1
	`, idosoID).Scan(&lastInteraction)

	if err != nil {
		return 0, err
	}

	days := int(time.Since(lastInteraction).Hours() / 24)
	return days, nil
}

// CalculateRelationshipLevel calcula nível baseado em número de conversas
// Progressão logarítmica: 1->3->6->8->10
func CalculateRelationshipLevel(conversations int) int {
	if conversations == 0 {
		return 1
	}

	// Fórmula: level = min(10, log2(conversas) + 1)
	level := int(math.Log2(float64(conversations)) + 1)

	if level > 10 {
		return 10
	}
	if level < 1 {
		return 1
	}

	return level
}

// GetRelationshipStyle retorna o estilo de tratamento baseado no nível
func GetRelationshipStyle(level int) string {
	switch {
	case level <= 2:
		return "formal" // "Senhora Maria"
	case level <= 5:
		return "friendly" // "Dona Maria"
	case level <= 8:
		return "intimate" // "Maria" ou "Mariazinha"
	default:
		return "family" // Apelidos carinhosos
	}
}

// GetRelationshipLabel retorna label descritiva
func GetRelationshipLabel(level int) string {
	labels := map[int]string{
		1:  "Nos conhecendo",
		2:  "Conhecidas",
		3:  "Amigas",
		4:  "Boas amigas",
		5:  "Amigas próximas",
		6:  "Confidentes",
		7:  "Muito próximas",
		8:  "Inseparáveis",
		9:  "Como família",
		10: "Família do coração",
	}

	return labels[level]
}

// Helpers

func parsePostgresArray(s string) []string {
	if s == "{}" || s == "" {
		return []string{}
	}

	// Remove {}
	s = s[1 : len(s)-1]

	// Split simples (assumindo sem vírgulas nos valores)
	if s == "" {
		return []string{}
	}

	// Remover aspas se existirem
	result := []string{}
	for _, item := range splitRespectingQuotes(s) {
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

func splitRespectingQuotes(s string) []string {
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

func toPostgresArray(arr []string) string {
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

func uniqueStrings(arr []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range arr {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
