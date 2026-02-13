package superhuman

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// DeepMemoryService handles the deep memory extensions
// Based on: Schacter (Persistence), Van der Kolk (Body), Casey (Place, Commemoration)
type DeepMemoryService struct {
	db *sql.DB
	weaver *NarrativeWeaver

	// Detection patterns
	avoidancePatterns   []*regexp.Regexp
	sharingPatterns     []*regexp.Regexp
	bodySymptomPatterns []*regexp.Regexp
	placePatterns       []*regexp.Regexp
	sensoryPatterns     []*regexp.Regexp
}

// NewDeepMemoryService creates a new deep memory service
func NewDeepMemoryService(db *sql.DB) *DeepMemoryService {
	svc := &DeepMemoryService{
		db:     db,
		weaver: NewNarrativeWeaver(db),
	}
	svc.compilePatterns()
	return svc
}

// compilePatterns prepares regex patterns
func (s *DeepMemoryService) compilePatterns() {
	// Avoidance patterns
	avoidance := []string{
		`(?i)nao\s+quero\s+falar`,
		`(?i)deixa\s+(?:isso\s+)?pra\s+la`,
		`(?i)(?:vamos|pode)\s+mudar\s+de\s+assunto`,
		`(?i)nao\s+(?:vamos|precisa)\s+falar\s+(?:disso|nisso)`,
		`(?i)esquece\s+(?:isso)?`,
		`(?i)(?:isso\s+)?ja\s+passou`,
		`(?i)nao\s+adianta\s+falar`,
	}

	// Sharing desire patterns
	sharing := []string{
		`(?i)queria\s+(?:que\s+)?(?:meus?\s+)?(\w+)\s+(?:soubesse|entendesse|conhecesse)`,
		`(?i)(?:preciso|tenho\s+que)\s+contar\s+(?:para|pra)\s+(\w+)`,
		`(?i)(?:eles|ela|ele)\s+(?:precisa|precisam)\s+saber`,
		`(?i)antes\s+(?:de\s+)?(?:eu\s+)?(?:morrer|ir|partir)`,
		`(?i)quero\s+(?:deixar|passar)\s+(?:isso\s+)?(?:para|pra)`,
		`(?i)(?:minha|essa)\s+historia`,
	}

	// Body symptom patterns
	bodySymptom := []string{
		`(?i)(?:sinto|tenho)\s+(?:um\s+)?(?:aperto|peso|dor|no|pontada)\s+(?:no|na|em)\s+(\w+)`,
		`(?i)(?:meu|minha)\s+(\w+)\s+(?:doi|aperta|treme|pesa)`,
		`(?i)(?:da|deu)\s+(?:uma?\s+)?(?:dor|pontada|aperto)\s+(?:no|na)\s+(\w+)`,
		`(?i)(?:comeca|comecou)\s+a\s+(?:doer|apertar|tremer)`,
		`(?i)(?:nao\s+)?(?:consigo|consegui)\s+(?:respirar|engolir)`,
		`(?i)(?:falta|faltou)\s+(?:o\s+)?ar`,
	}

	// Place patterns
	place := []string{
		`(?i)(?:la\s+(?:na|no|em)|na\s+(?:minha|nossa))\s+(\w+)`,
		`(?i)(?:quando\s+)?(?:eu\s+)?morava\s+(?:em|no|na)\s+(\w+)`,
		`(?i)(?:a|o)\s+(?:casa|sitio|fazenda|apartamento)\s+(?:do|da|em)\s+(\w+)`,
		`(?i)(?:minha|nossa)\s+(?:terra|cidade|rua)`,
		`(?i)(?:voltei|voltar)\s+(?:para|pra)\s+(\w+)`,
	}

	// Sensory memory patterns
	sensory := []string{
		`(?i)(?:o\s+)?cheiro\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:o\s+)?(?:som|barulho)\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:o\s+)?gosto\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:lembro|lembrava)\s+(?:do|da)\s+(\w+)`,
		`(?i)(?:parece|parecia)\s+que\s+(?:eu\s+)?(?:sinto|sentia)\s+(\w+)`,
		`(?i)(?:ainda\s+)?(?:sinto|senti)\s+(?:o|a)\s+(\w+)`,
	}

	s.avoidancePatterns = compilePatterns(avoidance)
	s.sharingPatterns = compilePatterns(sharing)
	s.bodySymptomPatterns = compilePatterns(bodySymptom)
	s.placePatterns = compilePatterns(place)
	s.sensoryPatterns = compilePatterns(sensory)
}

// =====================================================
// PERSISTENT MEMORY (Traumatic Persistence)
// =====================================================

// PersistentMemory represents a memory that persists despite avoidance
type PersistentMemory struct {
	ID                   int64     `json:"id"`
	IdosoID              int64     `json:"idoso_id"`
	PersistentTopic      string    `json:"persistent_topic"`
	AvoidanceAttempts    int       `json:"avoidance_attempts"`
	ReturnCount          int       `json:"return_count"`
	PersistenceScore     float64   `json:"persistence_score"`
	AvoidanceScore       float64   `json:"avoidance_score"`
	VoiceTremorPct       float64   `json:"voice_tremor_percentage"`
	InvolvedPersons      []string  `json:"involved_persons"`
	TypicalTriggers      []string  `json:"typical_triggers"`
	FirstDetected        time.Time `json:"first_detected"`
	LastOccurrence       time.Time `json:"last_occurrence"`
}

// DetectAvoidance checks if patient is trying to avoid a topic
func (s *DeepMemoryService) DetectAvoidance(ctx context.Context, idosoID int64, text string, currentTopic string, timestamp time.Time) error {
	for _, pattern := range s.avoidancePatterns {
		if pattern.MatchString(text) {
			// Record avoidance attempt
			query := `
				INSERT INTO patient_persistent_memories
				(idoso_id, persistent_topic, avoidance_attempts, first_detected, last_occurrence)
				VALUES ($1, $2, 1, $3, $3)
				ON CONFLICT (idoso_id, persistent_topic) DO UPDATE SET
					avoidance_attempts = patient_persistent_memories.avoidance_attempts + 1,
					last_occurrence = $3,
					updated_at = NOW()
			`
			if _, err := s.db.ExecContext(ctx, query, idosoID, currentTopic, timestamp); err != nil {
				return err
			}

			// Record occurrence
			occQuery := `
				INSERT INTO persistent_memory_occurrences
				(persistent_memory_id, occurrence_type, verbatim, occurred_at)
				SELECT id, 'avoidance', $2, $3
				FROM patient_persistent_memories
				WHERE idoso_id = $1 AND persistent_topic = $4
			`
			s.db.ExecContext(ctx, occQuery, idosoID, text, timestamp, currentTopic)

			log.Printf("🔄 [PERSISTENCE] Avoidance detected for topic '%s'", currentTopic)
			break
		}
	}

	return nil
}

// DetectReturn checks if patient returned to a previously avoided topic
func (s *DeepMemoryService) DetectReturn(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	// Check if any words match previously avoided topics
	query := `
		SELECT persistent_topic FROM patient_persistent_memories
		WHERE idoso_id = $1 AND avoidance_attempts > 0
	`
	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return err
	}
	defer rows.Close()

	textLower := strings.ToLower(text)
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			continue
		}

		if strings.Contains(textLower, strings.ToLower(topic)) {
			// Record return
			updateQuery := `
				UPDATE patient_persistent_memories
				SET return_count = return_count + 1,
				    last_occurrence = $2,
				    updated_at = NOW()
				WHERE idoso_id = $1 AND persistent_topic = $3
			`
			s.db.ExecContext(ctx, updateQuery, idosoID, timestamp, topic)

			// Update scores
			s.db.ExecContext(ctx, "SELECT update_persistence_scores($1)", idosoID)

			log.Printf("🔄 [PERSISTENCE] Return detected to topic '%s'", topic)
		}
	}

	return nil
}

// GetPersistentMemories retrieves persistent memories with high scores
func (s *DeepMemoryService) GetPersistentMemories(ctx context.Context, idosoID int64) ([]*PersistentMemory, error) {
	query := `
		SELECT id, persistent_topic, avoidance_attempts, return_count,
		       persistence_score, avoidance_score, voice_tremor_percentage,
		       involved_persons, typical_triggers, first_detected, last_occurrence
		FROM patient_persistent_memories
		WHERE idoso_id = $1 AND (persistence_score > 0.3 OR avoidance_score > 0.3)
		ORDER BY (persistence_score + avoidance_score) DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*PersistentMemory
	for rows.Next() {
		pm := &PersistentMemory{IdosoID: idosoID}
		var personsJSON, triggersJSON []byte
		var voiceTremor, persistence, avoidance sql.NullFloat64
		var firstDetected, lastOccurrence sql.NullTime

		err := rows.Scan(
			&pm.ID, &pm.PersistentTopic, &pm.AvoidanceAttempts, &pm.ReturnCount,
			&persistence, &avoidance, &voiceTremor,
			&personsJSON, &triggersJSON, &firstDetected, &lastOccurrence,
		)
		if err != nil {
			continue
		}

		if persistence.Valid {
			pm.PersistenceScore = persistence.Float64
		}
		if avoidance.Valid {
			pm.AvoidanceScore = avoidance.Float64
		}
		if voiceTremor.Valid {
			pm.VoiceTremorPct = voiceTremor.Float64
		}
		if firstDetected.Valid {
			pm.FirstDetected = firstDetected.Time
		}
		if lastOccurrence.Valid {
			pm.LastOccurrence = lastOccurrence.Time
		}

		json.Unmarshal(personsJSON, &pm.InvolvedPersons)
		json.Unmarshal(triggersJSON, &pm.TypicalTriggers)

		memories = append(memories, pm)
	}

	return memories, nil
}

// =====================================================
// BODY MEMORY (Van der Kolk)
// =====================================================

// BodyMemory represents a physical symptom that is a somatic memory
type BodyMemory struct {
	ID                         int64    `json:"id"`
	IdosoID                    int64    `json:"idoso_id"`
	PhysicalSymptom            string   `json:"physical_symptom"`
	BodyLocation               string   `json:"body_location"`
	PatientDescriptions        []string `json:"patient_descriptions"`
	CorrelatedTopics           []string `json:"correlated_topics"`
	CorrelatedPersons          []string `json:"correlated_persons"`
	StrongestCorrelationTopic  string   `json:"strongest_correlation_topic"`
	StrongestCorrelationStrength float64 `json:"strongest_correlation_strength"`
	OccurrenceCount            int      `json:"occurrence_count"`
	PatientAware               bool     `json:"patient_aware"`
	PatientVerbalization       string   `json:"patient_verbalization,omitempty"`
}

// DetectBodySymptom detects and records body symptoms mentioned
func (s *DeepMemoryService) DetectBodySymptom(ctx context.Context, idosoID int64, text string, precedingTopics []string, timestamp time.Time) error {
	for _, pattern := range s.bodySymptomPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			symptom := match[0]
			location := ""
			if len(match) > 1 {
				location = match[1]
			}

			// Insert or update body memory
			query := `
				INSERT INTO patient_body_memories
				(idoso_id, physical_symptom, body_location, occurrence_count,
				 first_reported, last_reported, correlated_topics)
				VALUES ($1, $2, $3, 1, $4, $4, $5)
				ON CONFLICT (idoso_id, physical_symptom, body_location) DO UPDATE SET
					occurrence_count = patient_body_memories.occurrence_count + 1,
					last_reported = $4,
					updated_at = NOW()
			`
			topicsJSON, _ := json.Marshal(precedingTopics)
			if _, err := s.db.ExecContext(ctx, query, idosoID, symptom, location, timestamp, string(topicsJSON)); err != nil {
				log.Printf("Error inserting body memory: %v", err)
				continue
			}

			// Record occurrence
			occQuery := `
				INSERT INTO body_memory_occurrences
				(body_memory_id, idoso_id, verbatim, occurred_at, preceding_topics)
				SELECT id, $1, $2, $3, $4
				FROM patient_body_memories
				WHERE idoso_id = $1 AND physical_symptom = $5
				LIMIT 1
			`
			s.db.ExecContext(ctx, occQuery, idosoID, text, timestamp, string(topicsJSON), symptom)

			// Update correlations
			s.updateBodyCorrelations(ctx, idosoID, symptom)

			log.Printf("🫀 [BODY] Symptom detected: '%s' at '%s'", symptom, location)
		}
	}

	return nil
}

// updateBodyCorrelations recalculates correlation strengths
func (s *DeepMemoryService) updateBodyCorrelations(ctx context.Context, idosoID int64, symptom string) {
	// Find most common preceding topic
	query := `
		WITH topic_counts AS (
			SELECT
				jsonb_array_elements_text(preceding_topics) as topic,
				COUNT(*) as cnt
			FROM body_memory_occurrences bmo
			JOIN patient_body_memories bm ON bmo.body_memory_id = bm.id
			WHERE bm.idoso_id = $1 AND bm.physical_symptom = $2
			GROUP BY topic
			ORDER BY cnt DESC
			LIMIT 1
		)
		UPDATE patient_body_memories bm
		SET strongest_correlation_topic = tc.topic,
		    strongest_correlation_strength = tc.cnt::decimal / GREATEST(1, bm.occurrence_count),
		    updated_at = NOW()
		FROM topic_counts tc
		WHERE bm.idoso_id = $1 AND bm.physical_symptom = $2
	`
	s.db.ExecContext(ctx, query, idosoID, symptom)
}

// GetBodyMemories retrieves body memories with correlations
func (s *DeepMemoryService) GetBodyMemories(ctx context.Context, idosoID int64) ([]*BodyMemory, error) {
	query := `
		SELECT id, physical_symptom, body_location, patient_descriptions,
		       correlated_topics, correlated_persons,
		       strongest_correlation_topic, strongest_correlation_strength,
		       occurrence_count, patient_aware, patient_verbalization
		FROM patient_body_memories
		WHERE idoso_id = $1
		ORDER BY strongest_correlation_strength DESC NULLS LAST, occurrence_count DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*BodyMemory
	for rows.Next() {
		bm := &BodyMemory{IdosoID: idosoID}
		var descsJSON, topicsJSON, personsJSON []byte
		var strongestTopic, verbalization sql.NullString
		var strongestStrength sql.NullFloat64

		err := rows.Scan(
			&bm.ID, &bm.PhysicalSymptom, &bm.BodyLocation, &descsJSON,
			&topicsJSON, &personsJSON,
			&strongestTopic, &strongestStrength,
			&bm.OccurrenceCount, &bm.PatientAware, &verbalization,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(descsJSON, &bm.PatientDescriptions)
		json.Unmarshal(topicsJSON, &bm.CorrelatedTopics)
		json.Unmarshal(personsJSON, &bm.CorrelatedPersons)

		if strongestTopic.Valid {
			bm.StrongestCorrelationTopic = strongestTopic.String
		}
		if strongestStrength.Valid {
			bm.StrongestCorrelationStrength = strongestStrength.Float64
		}
		if verbalization.Valid {
			bm.PatientVerbalization = verbalization.String
		}

		memories = append(memories, bm)
	}

	return memories, nil
}

// =====================================================
// SHARED MEMORY (Commemoration)
// =====================================================

// SharedMemory represents a memory the patient wants to share
type SharedMemory struct {
	ID               int64     `json:"id"`
	IdosoID          int64     `json:"idoso_id"`
	MemorySummary    string    `json:"memory_summary"`
	IntendedAudience []string  `json:"intended_audience"`
	SharedWith       []string  `json:"shared_with"`
	SharingStatus    string    `json:"sharing_status"`
	MemoryType       string    `json:"memory_type"`
	UrgencyScore     float64   `json:"urgency_score"`
	AssociatedRitual string    `json:"associated_ritual,omitempty"`
	MentionCount     int       `json:"mention_count"`
	FirstMentioned   time.Time `json:"first_mentioned"`
	LastMentioned    time.Time `json:"last_mentioned"`
}

// DetectSharingDesire detects when patient wants to share a memory
func (s *DeepMemoryService) DetectSharingDesire(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	for _, pattern := range s.sharingPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			audience := ""
			if len(match) > 1 {
				audience = match[1]
			}

			// Classify memory type
			memoryType := s.classifyMemoryType(text)

			query := `
				INSERT INTO patient_shared_memories
				(idoso_id, memory_summary, intended_audience, sharing_status,
				 memory_type, mention_count, first_mentioned, last_mentioned,
				 verbatim_mentions)
				VALUES ($1, $2, $3, 'wishes_to_share', $4, 1, $5, $5, $6)
				ON CONFLICT DO NOTHING
			`
			audienceJSON, _ := json.Marshal([]string{audience})
			verbatimJSON, _ := json.Marshal([]string{text})

			if _, err := s.db.ExecContext(ctx, query, idosoID, text[:minInt(200, len(text))],
				string(audienceJSON), memoryType, timestamp, string(verbatimJSON)); err != nil {
				// Try to update existing
				updateQuery := `
					UPDATE patient_shared_memories
					SET mention_count = mention_count + 1,
					    last_mentioned = $2,
					    urgency_score = LEAST(1.0, urgency_score + 0.1),
					    verbatim_mentions = verbatim_mentions || $3::jsonb,
					    updated_at = NOW()
					WHERE idoso_id = $1 AND memory_summary ILIKE '%' || $4 || '%'
				`
				s.db.ExecContext(ctx, updateQuery, idosoID, timestamp, string(verbatimJSON), text[:minInt(50, len(text))])
			}

			log.Printf("💬 [SHARING] Desire to share detected, audience: %s", audience)
		}
	}

	return nil
}

// classifyMemoryType determines what type of memory patient wants to share
func (s *DeepMemoryService) classifyMemoryType(text string) string {
	textLower := strings.ToLower(text)

	if strings.Contains(textLower, "licao") || strings.Contains(textLower, "aprendi") {
		return "life_lesson"
	}
	if strings.Contains(textLower, "familia") || strings.Contains(textLower, "avo") ||
		strings.Contains(textLower, "origem") {
		return "family_history"
	}
	if strings.Contains(textLower, "conheci") || strings.Contains(textLower, "casamos") ||
		strings.Contains(textLower, "amor") {
		return "love_story"
	}
	if strings.Contains(textLower, "consegui") || strings.Contains(textLower, "conquist") {
		return "achievement"
	}
	if strings.Contains(textLower, "cuidado") || strings.Contains(textLower, "nao faca") {
		return "warning"
	}
	if strings.Contains(textLower, "receita") || strings.Contains(textLower, "como faz") {
		return "recipe"
	}
	if strings.Contains(textLower, "tradicao") || strings.Contains(textLower, "sempre") {
		return "tradition"
	}

	return "other"
}

// GetSharedMemories retrieves memories patient wants to share
func (s *DeepMemoryService) GetSharedMemories(ctx context.Context, idosoID int64) ([]*SharedMemory, error) {
	query := `
		SELECT id, memory_summary, intended_audience, shared_with,
		       sharing_status, memory_type, urgency_score, associated_ritual,
		       mention_count, first_mentioned, last_mentioned
		FROM patient_shared_memories
		WHERE idoso_id = $1 AND sharing_status != 'fully_shared'
		ORDER BY urgency_score DESC NULLS LAST, mention_count DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*SharedMemory
	for rows.Next() {
		sm := &SharedMemory{IdosoID: idosoID}
		var audienceJSON, sharedJSON []byte
		var ritual sql.NullString
		var urgency sql.NullFloat64

		err := rows.Scan(
			&sm.ID, &sm.MemorySummary, &audienceJSON, &sharedJSON,
			&sm.SharingStatus, &sm.MemoryType, &urgency, &ritual,
			&sm.MentionCount, &sm.FirstMentioned, &sm.LastMentioned,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(audienceJSON, &sm.IntendedAudience)
		json.Unmarshal(sharedJSON, &sm.SharedWith)

		if ritual.Valid {
			sm.AssociatedRitual = ritual.String
		}
		if urgency.Valid {
			sm.UrgencyScore = urgency.Float64
		}

		memories = append(memories, sm)
	}

	return memories, nil
}

// =====================================================
// MIRROR OUTPUTS
// =====================================================

// GeneratePersistentMemoryMirror creates mirror output for persistent memories
func (s *DeepMemoryService) GeneratePersistentMemoryMirror(pm *PersistentMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("O tema '%s' apareceu %d vezes", pm.PersistentTopic, pm.AvoidanceAttempts+pm.ReturnCount),
		fmt.Sprintf("Voce tentou evitar %d vezes", pm.AvoidanceAttempts),
		fmt.Sprintf("Mas voltou ao tema %d vezes", pm.ReturnCount),
	}

	if pm.VoiceTremorPct > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Sua voz apresenta tremor em %.0f%% das vezes que fala disso", pm.VoiceTremorPct*100))
	}

	if len(pm.InvolvedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Pessoas envolvidas: %s", strings.Join(pm.InvolvedPersons, ", ")))
	}

	total := pm.AvoidanceAttempts + pm.ReturnCount
	return &MirrorOutput{
		Type:       "persistent_memory",
		DataPoints: dataPoints,
		Frequency:  &total,
		Question:   "Parece que esse assunto insiste em voltar mesmo quando voce tenta evitar. O que voce acha que ele precisa?",
		RawData: map[string]interface{}{
			"topic":             pm.PersistentTopic,
			"persistence_score": pm.PersistenceScore,
			"avoidance_score":   pm.AvoidanceScore,
			"triggers":          pm.TypicalTriggers,
		},
	}
}

// GenerateBodyMemoryMirror creates mirror output for body memories
func (s *DeepMemoryService) GenerateBodyMemoryMirror(bm *BodyMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce relatou '%s' %d vezes", bm.PhysicalSymptom, bm.OccurrenceCount),
	}

	if bm.BodyLocation != "" {
		dataPoints[0] = fmt.Sprintf("Voce relatou '%s' no/na %s %d vezes",
			bm.PhysicalSymptom, bm.BodyLocation, bm.OccurrenceCount)
	}

	if bm.StrongestCorrelationTopic != "" && bm.StrongestCorrelationStrength > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Em %.0f%% das vezes, voce havia falado de '%s' antes",
				bm.StrongestCorrelationStrength*100, bm.StrongestCorrelationTopic))
	}

	if len(bm.CorrelatedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Pessoas associadas: %s", strings.Join(bm.CorrelatedPersons, ", ")))
	}

	question := "Voce percebe alguma conexao entre seu corpo e esse assunto?"
	if bm.PatientAware {
		question = "Voce ja havia percebido essa conexao. O que voce acha que seu corpo esta tentando dizer?"
	}

	return &MirrorOutput{
		Type:       "body_memory",
		DataPoints: dataPoints,
		Frequency:  &bm.OccurrenceCount,
		Question:   question,
		RawData: map[string]interface{}{
			"symptom":            bm.PhysicalSymptom,
			"location":           bm.BodyLocation,
			"correlation_topic":  bm.StrongestCorrelationTopic,
			"correlation_strength": bm.StrongestCorrelationStrength,
			"patient_aware":      bm.PatientAware,
		},
	}
}

// GenerateSharedMemoryMirror creates mirror output for shared memories
func (s *DeepMemoryService) GenerateSharedMemoryMirror(sm *SharedMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce mencionou %d vezes que quer contar isso", sm.MentionCount),
	}

	if len(sm.IntendedAudience) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Para quem: %s", strings.Join(sm.IntendedAudience, ", ")))
	}

	if len(sm.SharedWith) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Ja contou para: %s", strings.Join(sm.SharedWith, ", ")))
	}

	daysSince := int(time.Since(sm.FirstMentioned).Hours() / 24)
	dataPoints = append(dataPoints,
		fmt.Sprintf("Primeira vez que mencionou: ha %d dias", daysSince))

	return &MirrorOutput{
		Type:       "shared_memory",
		DataPoints: dataPoints,
		Frequency:  &sm.MentionCount,
		Question:   "O que voce gostaria que eles soubessem dessa historia?",
		RawData: map[string]interface{}{
			"memory_type":      sm.MemoryType,
			"urgency":          sm.UrgencyScore,
			"sharing_status":   sm.SharingStatus,
			"intended_audience": sm.IntendedAudience,
		},
	}
}

// GetNarrativeWeaver returns the narrative weaver
func (s *DeepMemoryService) GetNarrativeWeaver() *NarrativeWeaver {
	return s.weaver
}

// =====================================================
// BATCH MIRROR GENERATION
// =====================================================

// GeneratePersistentMirrors generates mirrors for all significant persistent memories
func (s *DeepMemoryService) GeneratePersistentMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetPersistentMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, pm := range memories {
		if pm.PersistenceScore > 0.4 || pm.AvoidanceScore > 0.4 {
			outputs = append(outputs, s.GeneratePersistentMemoryMirror(pm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// GenerateBodyMemoryMirrors generates mirrors for all significant body memories
func (s *DeepMemoryService) GenerateBodyMemoryMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetBodyMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, bm := range memories {
		if bm.OccurrenceCount >= 2 && bm.StrongestCorrelationStrength > 0.4 {
			outputs = append(outputs, s.GenerateBodyMemoryMirror(bm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// GenerateSharedMemoryMirrors generates mirrors for memories patient wants to share
func (s *DeepMemoryService) GenerateSharedMemoryMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetSharedMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, sm := range memories {
		if sm.MentionCount >= 2 || sm.UrgencyScore > 0.5 {
			outputs = append(outputs, s.GenerateSharedMemoryMirror(sm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// Helper function
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
