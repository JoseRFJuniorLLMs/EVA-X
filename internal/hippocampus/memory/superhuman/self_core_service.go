// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// SelfCoreService manages the patient's identity memory
// PRINCIPLE: This records WHO THE PATIENT SAYS THEY ARE, not who EVA thinks they are
type SelfCoreService struct {
	db *database.DB

	// Patterns to detect self-descriptions
	selfDescriptionPatterns []*regexp.Regexp
	rolePatterns            []*regexp.Regexp
}

// NewSelfCoreService creates a new identity memory service
func NewSelfCoreService(db *database.DB) *SelfCoreService {
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
func (s *SelfCoreService) addSelfDescription(ctx context.Context, idosoID int64, text string, timestamp time.Time, descContext string) error {
	now := time.Now().Format(time.RFC3339)

	description := SelfDescription{
		Text:      text,
		Timestamp: timestamp,
		Context:   descContext,
	}
	descJSON, err := json.Marshal([]SelfDescription{description})
	if err != nil {
		return err
	}

	// Get or create self_core record
	rows, err2 := s.db.QueryByLabel(ctx, "patient_self_core",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err2 != nil {
		log.Printf("Error querying patient_self_core for idoso %d: %v", idosoID, err2)
		return err2
	}

	if len(rows) > 0 {
		m := rows[0]
		// Append to existing descriptions
		var existing []SelfDescription
		if raw, ok := m["self_descriptions"]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &existing); err != nil {
					log.Printf("Error unmarshaling self_descriptions string for idoso %d: %v", idosoID, err)
				}
			case []interface{}:
				b, err := json.Marshal(v)
				if err != nil {
					log.Printf("Error marshaling self_descriptions slice for idoso %d: %v", idosoID, err)
				} else {
					if err := json.Unmarshal(b, &existing); err != nil {
						log.Printf("Error unmarshaling self_descriptions slice for idoso %d: %v", idosoID, err)
					}
				}
			}
		}
		existing = append(existing, description)
		allDescsJSON, err3 := json.Marshal(existing)
		if err3 != nil {
			log.Printf("Error marshaling updated self_descriptions for idoso %d: %v", idosoID, err3)
			allDescsJSON = descJSON // fallback to just the new description
		}

		return s.db.Update(ctx, "patient_self_core",
			map[string]interface{}{"idoso_id": idosoID},
			map[string]interface{}{
				"self_descriptions": string(allDescsJSON),
				"updated_at":        now,
			})
	}

	// Create new record
	_, err = s.db.Insert(ctx, "patient_self_core", map[string]interface{}{
		"idoso_id":          idosoID,
		"self_descriptions": string(descJSON),
		"created_at":        now,
		"updated_at":        now,
	})
	return err
}

// addRole adds a self-attributed role
func (s *SelfCoreService) addRole(ctx context.Context, idosoID int64, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	now := time.Now().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "patient_self_core",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		log.Printf("Error querying patient_self_core for role, idoso %d: %v", idosoID, err)
		return err
	}

	if len(rows) > 0 {
		m := rows[0]
		// Get existing roles
		var existingRoles []string
		if raw, ok := m["self_attributed_roles"]; ok && raw != nil {
			parseJSONStringSlice(raw, &existingRoles)
		}

		// Add role if not already present
		found := false
		for _, r := range existingRoles {
			if r == role {
				found = true
				break
			}
		}
		if !found {
			existingRoles = append(existingRoles, role)
		}

		rolesJSON, err := json.Marshal(existingRoles)
		if err != nil {
			log.Printf("Error marshaling existingRoles for idoso %d: %v", idosoID, err)
			rolesJSON = []byte("[]")
		}
		return s.db.Update(ctx, "patient_self_core",
			map[string]interface{}{"idoso_id": idosoID},
			map[string]interface{}{
				"self_attributed_roles": string(rolesJSON),
				"updated_at":            now,
			})
	}

	// Create new record
	rolesJSON, err := json.Marshal([]string{role})
	if err != nil {
		log.Printf("Error marshaling new role for idoso %d: %v", idosoID, err)
		rolesJSON = []byte("[]")
	}
	_, err = s.db.Insert(ctx, "patient_self_core", map[string]interface{}{
		"idoso_id":             idosoID,
		"self_attributed_roles": string(rolesJSON),
		"created_at":           now,
		"updated_at":           now,
	})
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
	now := timestamp.Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "patient_master_signifiers",
		" AND n.idoso_id = $idoso AND n.signifier = $sig",
		map[string]interface{}{"idoso": idosoID, "sig": signifier}, 1)
	if err != nil {
		log.Printf("Error querying patient_master_signifiers for idoso %d, signifier %s: %v", idosoID, signifier, err)
		return err
	}

	if len(rows) > 0 {
		m := rows[0]
		totalCount := int(database.GetInt64(m, "total_count")) + 1

		// Update frequency_by_period
		freqByPeriod := make(map[string]int)
		if raw, ok := m["frequency_by_period"]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &freqByPeriod); err != nil {
					log.Printf("Error unmarshaling frequency_by_period string for signifier %s: %v", signifier, err)
				}
			case map[string]interface{}:
				for k, val := range v {
					if f, ok := val.(float64); ok {
						freqByPeriod[k] = int(f)
					}
				}
			}
		}
		freqByPeriod[period]++
		freqJSON, err2 := json.Marshal(freqByPeriod)
		if err2 != nil {
			log.Printf("Error marshaling frequency_by_period for signifier %s: %v", signifier, err2)
			freqJSON = []byte("{}")
		}

		return s.db.Update(ctx, "patient_master_signifiers",
			map[string]interface{}{"idoso_id": idosoID, "signifier": signifier},
			map[string]interface{}{
				"total_count":         totalCount,
				"last_seen":           now,
				"frequency_by_period": string(freqJSON),
				"updated_at":          now,
			})
	}

	// Insert new
	freqByPeriod := map[string]int{period: 1}
	freqJSON, err := json.Marshal(freqByPeriod)
	if err != nil {
		log.Printf("Error marshaling new frequency_by_period for signifier %s: %v", signifier, err)
		freqJSON = []byte("{}")
	}

	_, err = s.db.Insert(ctx, "patient_master_signifiers", map[string]interface{}{
		"idoso_id":            idosoID,
		"signifier":           signifier,
		"context_type":        contextType,
		"total_count":         1,
		"first_seen":          now,
		"last_seen":           now,
		"frequency_by_period": string(freqJSON),
		"created_at":          now,
		"updated_at":          now,
	})
	return err
}

// GetSelfCore retrieves the patient's identity memory
func (s *SelfCoreService) GetSelfCore(ctx context.Context, idosoID int64) (*PatientSelfCore, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_self_core",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return &PatientSelfCore{IdosoID: idosoID}, nil
	}

	m := rows[0]
	psc := &PatientSelfCore{
		IdosoID:          idosoID,
		NarrativeSummary: database.GetString(m, "narrative_summary"),
		NarrativeLastUpdated: database.GetTime(m, "narrative_last_updated"),
	}

	// Parse self_descriptions
	if raw, ok := m["self_descriptions"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &psc.SelfDescriptions); err != nil {
				log.Printf("Error unmarshaling self_descriptions string for idoso %d: %v", idosoID, err)
			}
		case []interface{}:
			b, err := json.Marshal(v)
			if err != nil {
				log.Printf("Error marshaling self_descriptions slice for idoso %d: %v", idosoID, err)
			} else {
				if err := json.Unmarshal(b, &psc.SelfDescriptions); err != nil {
					log.Printf("Error unmarshaling self_descriptions slice for idoso %d: %v", idosoID, err)
				}
			}
		}
	}

	// Parse self_attributed_roles
	if raw, ok := m["self_attributed_roles"]; ok && raw != nil {
		parseJSONStringSlice(raw, &psc.SelfAttributedRoles)
	}

	return psc, nil
}

// GetTopSignifiers returns the most frequent signifiers
func (s *SelfCoreService) GetTopSignifiers(ctx context.Context, idosoID int64, limit int) ([]*MasterSignifier, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_master_signifiers",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var signifiers []*MasterSignifier
	for _, m := range rows {
		ms := &MasterSignifier{
			IdosoID:             idosoID,
			Signifier:           database.GetString(m, "signifier"),
			ContextType:         database.GetString(m, "context_type"),
			TotalCount:          int(database.GetInt64(m, "total_count")),
			FirstSeen:           database.GetTime(m, "first_seen"),
			LastSeen:            database.GetTime(m, "last_seen"),
			AvgEmotionalValence: database.GetFloat64(m, "avg_emotional_valence"),
		}

		// Parse frequency_by_period
		if raw, ok := m["frequency_by_period"]; ok && raw != nil {
			ms.FrequencyByPeriod = make(map[string]int)
			switch v := raw.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &ms.FrequencyByPeriod); err != nil {
					log.Printf("Error unmarshaling frequency_by_period for signifier %s: %v", ms.Signifier, err)
				}
			case map[string]interface{}:
				for k, val := range v {
					if f, ok := val.(float64); ok {
						ms.FrequencyByPeriod[k] = int(f)
					}
				}
			}
		}

		// Parse co_occurring_signifiers
		if raw, ok := m["co_occurring_signifiers"]; ok && raw != nil {
			parseJSONStringSlice(raw, &ms.CoOccurringSignifiers)
		}

		signifiers = append(signifiers, ms)
	}

	// Sort by total count descending and limit
	// (NietzscheDB doesn't have ORDER BY in WHERE)
	for i := 0; i < len(signifiers); i++ {
		for j := i + 1; j < len(signifiers); j++ {
			if signifiers[j].TotalCount > signifiers[i].TotalCount {
				signifiers[i], signifiers[j] = signifiers[j], signifiers[i]
			}
		}
	}
	if len(signifiers) > limit {
		signifiers = signifiers[:limit]
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
