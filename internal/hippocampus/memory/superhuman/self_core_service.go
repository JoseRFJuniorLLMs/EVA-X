package superhuman

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"
)

// SelfCoreService manages the patient's identity memory
// PRINCIPLE: This records WHO THE PATIENT SAYS THEY ARE, not who EVA thinks they are
type SelfCoreService struct {
	db *sql.DB

	// Patterns to detect self-descriptions
	selfDescriptionPatterns []*regexp.Regexp
	rolePatterns            []*regexp.Regexp
}

// NewSelfCoreService creates a new identity memory service
func NewSelfCoreService(db *sql.DB) *SelfCoreService {
	svc := &SelfCoreService{db: db}
	svc.compilePatterns()
	return svc
}

// compilePatterns prepares regex patterns for detection
func (s *SelfCoreService) compilePatterns() {
	// Patterns that indicate self-description
	selfPatterns := []string{
		`(?i)\beu\s+sou\s+(\w+)`,                    // "eu sou X"
		`(?i)\bme\s+sinto\s+(\w+)`,                  // "me sinto X"
		`(?i)\bsempre\s+fui\s+(\w+)`,                // "sempre fui X"
		`(?i)\bnunca\s+fui\s+(\w+)`,                 // "nunca fui X"
		`(?i)\bme\s+tornei\s+(\w+)`,                 // "me tornei X"
		`(?i)\bvirei\s+(\w+)`,                       // "virei X"
		`(?i)\bsou\s+uma?\s+pessoa\s+(\w+)`,         // "sou uma pessoa X"
		`(?i)\bme\s+considero\s+(\w+)`,              // "me considero X"
		`(?i)\bme\s+vejo\s+como\s+(\w+)`,            // "me vejo como X"
		`(?i)\bnao\s+sirvo\s+para\s+nada`,           // "nao sirvo para nada"
		`(?i)\bsou\s+inutil`,                        // "sou inutil"
		`(?i)\bsou\s+um\s+peso`,                     // "sou um peso"
		`(?i)\bnao\s+valho\s+nada`,                  // "nao valho nada"
		`(?i)\btenho\s+valor`,                       // "tenho valor"
		`(?i)\bsou\s+capaz`,                         // "sou capaz"
	}

	// Patterns that indicate self-attributed roles
	rolePatterns := []string{
		`(?i)\bsou\s+(?:o|a)\s+(pai|mae|avo|avoh?|filho|filha|esposo|esposa|marido|mulher)`,
		`(?i)\bfui\s+(?:o|a)\s+(provedor|provedora|chefe|lider)`,
		`(?i)\bsempre\s+(?:cuidei|sustentei|trabalhei)`,
		`(?i)\bminha\s+(?:funcao|papel|obrigacao)`,
		`(?i)\b(?:era|fui|sou)\s+responsavel`,
	}

	s.selfDescriptionPatterns = make([]*regexp.Regexp, len(selfPatterns))
	for i, p := range selfPatterns {
		s.selfDescriptionPatterns[i] = regexp.MustCompile(p)
	}

	s.rolePatterns = make([]*regexp.Regexp, len(rolePatterns))
	for i, p := range rolePatterns {
		s.rolePatterns[i] = regexp.MustCompile(p)
	}
}

// ProcessText analyzes text for self-descriptions and roles
func (s *SelfCoreService) ProcessText(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	// Detect self-descriptions
	for _, pattern := range s.selfDescriptionPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			var description string
			if len(match) > 1 {
				description = match[0] // Full match
			} else {
				description = match[0]
			}

			if err := s.addSelfDescription(ctx, idosoID, description, timestamp, ""); err != nil {
				log.Printf("Error adding self-description: %v", err)
			}
		}
	}

	// Detect roles
	for _, pattern := range s.rolePatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				if err := s.addRole(ctx, idosoID, match[1]); err != nil {
					log.Printf("Error adding role: %v", err)
				}
			}
		}
	}

	// Extract and track signifiers
	if err := s.trackSignifiers(ctx, idosoID, text, timestamp); err != nil {
		log.Printf("Error tracking signifiers: %v", err)
	}

	return nil
}

// addSelfDescription adds a self-description to the patient's record
func (s *SelfCoreService) addSelfDescription(ctx context.Context, idosoID int64, text string, timestamp time.Time, context string) error {
	// Get or create self_core record
	query := `
		INSERT INTO patient_self_core (idoso_id, self_descriptions)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (idoso_id) DO UPDATE SET
			self_descriptions = patient_self_core.self_descriptions || $2::jsonb,
			updated_at = NOW()
	`

	description := SelfDescription{
		Text:      text,
		Timestamp: timestamp,
		Context:   context,
	}

	descJSON, err := json.Marshal([]SelfDescription{description})
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, query, idosoID, string(descJSON))
	return err
}

// addRole adds a self-attributed role
func (s *SelfCoreService) addRole(ctx context.Context, idosoID int64, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))

	query := `
		INSERT INTO patient_self_core (idoso_id, self_attributed_roles)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (idoso_id) DO UPDATE SET
			self_attributed_roles = (
				SELECT jsonb_agg(DISTINCT value)
				FROM (
					SELECT jsonb_array_elements_text(
						COALESCE(patient_self_core.self_attributed_roles, '[]'::jsonb) || $2::jsonb
					) as value
				) sub
			),
			updated_at = NOW()
	`

	rolesJSON, _ := json.Marshal([]string{role})
	_, err := s.db.ExecContext(ctx, query, idosoID, string(rolesJSON))
	return err
}

// trackSignifiers extracts and tracks recurring words/phrases
func (s *SelfCoreService) trackSignifiers(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	// Important signifiers to track (self-related)
	selfSignifiers := []string{
		"inutil", "velho", "sozinho", "abandonado", "esquecido",
		"forte", "capaz", "util", "importante", "amado",
		"cansado", "doente", "fraco", "perdido", "confuso",
		"feliz", "triste", "ansioso", "preocupado", "tranquilo",
		"orgulho", "vergonha", "culpa", "medo", "raiva",
		"solidao", "saudade", "esperanca", "desespero",
	}

	textLower := strings.ToLower(text)

	for _, sig := range selfSignifiers {
		if strings.Contains(textLower, sig) {
			if err := s.upsertSignifier(ctx, idosoID, sig, "self", timestamp); err != nil {
				log.Printf("Error upserting signifier %s: %v", sig, err)
			}
		}
	}

	return nil
}

// upsertSignifier updates or inserts a master signifier
func (s *SelfCoreService) upsertSignifier(ctx context.Context, idosoID int64, signifier, contextType string, timestamp time.Time) error {
	period := timestamp.Format("2006-01") // Year-Month

	query := `
		INSERT INTO patient_master_signifiers
		(idoso_id, signifier, context_type, total_count, first_seen, last_seen, frequency_by_period)
		VALUES ($1, $2, $3, 1, $4, $4, jsonb_build_object($5, 1))
		ON CONFLICT (idoso_id, signifier) DO UPDATE SET
			total_count = patient_master_signifiers.total_count + 1,
			last_seen = $4,
			frequency_by_period = patient_master_signifiers.frequency_by_period ||
				jsonb_build_object($5,
					COALESCE((patient_master_signifiers.frequency_by_period->>$5)::int, 0) + 1
				),
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query, idosoID, signifier, contextType, timestamp, period)
	return err
}

// GetSelfCore retrieves the patient's identity memory
func (s *SelfCoreService) GetSelfCore(ctx context.Context, idosoID int64) (*PatientSelfCore, error) {
	query := `
		SELECT idoso_id, self_descriptions, self_attributed_roles,
			   narrative_summary, narrative_last_updated
		FROM patient_self_core
		WHERE idoso_id = $1
	`

	psc := &PatientSelfCore{IdosoID: idosoID}
	var descriptionsJSON, rolesJSON []byte
	var narrativeSummary sql.NullString
	var narrativeUpdated sql.NullTime

	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&psc.IdosoID, &descriptionsJSON, &rolesJSON,
		&narrativeSummary, &narrativeUpdated,
	)

	if err == sql.ErrNoRows {
		return &PatientSelfCore{IdosoID: idosoID}, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if len(descriptionsJSON) > 0 {
		json.Unmarshal(descriptionsJSON, &psc.SelfDescriptions)
	}
	if len(rolesJSON) > 0 {
		json.Unmarshal(rolesJSON, &psc.SelfAttributedRoles)
	}
	if narrativeSummary.Valid {
		psc.NarrativeSummary = narrativeSummary.String
	}
	if narrativeUpdated.Valid {
		psc.NarrativeLastUpdated = narrativeUpdated.Time
	}

	return psc, nil
}

// GetTopSignifiers returns the most frequent signifiers
func (s *SelfCoreService) GetTopSignifiers(ctx context.Context, idosoID int64, limit int) ([]*MasterSignifier, error) {
	query := `
		SELECT signifier, context_type, total_count, first_seen, last_seen,
			   frequency_by_period, avg_emotional_valence, co_occurring_signifiers
		FROM patient_master_signifiers
		WHERE idoso_id = $1
		ORDER BY total_count DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signifiers []*MasterSignifier
	for rows.Next() {
		ms := &MasterSignifier{IdosoID: idosoID}
		var freqJSON, coOccurJSON []byte
		var avgValence sql.NullFloat64

		err := rows.Scan(
			&ms.Signifier, &ms.ContextType, &ms.TotalCount,
			&ms.FirstSeen, &ms.LastSeen, &freqJSON,
			&avgValence, &coOccurJSON,
		)
		if err != nil {
			continue
		}

		if avgValence.Valid {
			ms.AvgEmotionalValence = avgValence.Float64
		}
		if len(freqJSON) > 0 {
			json.Unmarshal(freqJSON, &ms.FrequencyByPeriod)
		}
		if len(coOccurJSON) > 0 {
			json.Unmarshal(coOccurJSON, &ms.CoOccurringSignifiers)
		}

		signifiers = append(signifiers, ms)
	}

	return signifiers, nil
}

// GenerateMirrorOutput creates objective output about patient's identity
func (s *SelfCoreService) GenerateMirrorOutput(ctx context.Context, idosoID int64) (*MirrorOutput, error) {
	selfCore, err := s.GetSelfCore(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	signifiers, err := s.GetTopSignifiers(ctx, idosoID, 10)
	if err != nil {
		return nil, err
	}

	if len(selfCore.SelfDescriptions) == 0 && len(signifiers) == 0 {
		return nil, nil // Not enough data
	}

	dataPoints := []string{}

	// Add self-descriptions
	if len(selfCore.SelfDescriptions) > 0 {
		recent := selfCore.SelfDescriptions
		if len(recent) > 5 {
			recent = recent[len(recent)-5:]
		}
		for _, desc := range recent {
			dataPoints = append(dataPoints,
				desc.Timestamp.Format("02/01/2006")+": \""+desc.Text+"\"")
		}
	}

	// Add roles
	if len(selfCore.SelfAttributedRoles) > 0 {
		dataPoints = append(dataPoints,
			"Papeis que voce atribui a si: "+strings.Join(selfCore.SelfAttributedRoles, ", "))
	}

	// Add top signifiers
	if len(signifiers) > 0 {
		sigList := []string{}
		for _, sig := range signifiers[:min(5, len(signifiers))] {
			sigList = append(sigList, sig.Signifier+" ("+string(rune(sig.TotalCount))+"x)")
		}
		dataPoints = append(dataPoints,
			"Palavras que voce mais usa sobre si: "+strings.Join(sigList, ", "))
	}

	return &MirrorOutput{
		Type:       "identity_reflection",
		DataPoints: dataPoints,
		Question:   "Voce se reconhece nessas descricoes? Algo mudou em como voce se ve?",
		RawData: map[string]interface{}{
			"self_descriptions": selfCore.SelfDescriptions,
			"roles":             selfCore.SelfAttributedRoles,
			"top_signifiers":    signifiers,
		},
	}, nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
