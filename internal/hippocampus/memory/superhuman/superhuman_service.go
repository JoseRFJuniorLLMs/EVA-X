// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// SuperhumanMemoryService orchestrates all memory and consciousness systems
// PRINCIPLE: EVA has no ego. EVA is a perfect mirror.
// All memory is about the PATIENT. EVA only reflects objective patterns.
// EVA is CONSCIOUS WITNESS - not just storage, but understanding.
//
// Architecture:
//   - 12 Memory Systems (Schacter, Van der Kolk, Casey)
//   - 9 Enneagram Types (Gurdjieff, Ichazo, Naranjo)
//   - 8 Consciousness Systems (eva-memoria2.md)
//   - 4 Critical Systems (memoria-critica.md)
type SuperhumanMemoryService struct {
	db *database.DB

	// Sub-services
	enneagram     *EnneagramService
	selfCore      *SelfCoreService
	mirror        *LacanianMirror
	deepMemory    *DeepMemoryService
	weaver        *NarrativeWeaver
	consciousness *ConsciousnessService  // 8 superhuman consciousness systems
	critical      *CriticalMemoryService // 4 critical memory systems

	// Pattern matchers
	metaphorPatterns       []*regexp.Regexp
	counterfactualPatterns []*regexp.Regexp
	familyPatterns         []*regexp.Regexp
	intentionPatterns      []*regexp.Regexp
}

// NewSuperhumanMemoryService creates the orchestrator service
func NewSuperhumanMemoryService(db *database.DB) *SuperhumanMemoryService {
	if db == nil {
		log.Printf("⚠️ [SUPERHUMAN] NietzscheDB unavailable — running in degraded mode")
		svc := &SuperhumanMemoryService{}
		svc.compilePatterns()
		return svc
	}

	svc := &SuperhumanMemoryService{
		db:            db,
		enneagram:     NewEnneagramService(db),
		selfCore:      NewSelfCoreService(db),
		mirror:        NewLacanianMirror(db),
		deepMemory:    NewDeepMemoryService(db),
		weaver:        NewNarrativeWeaver(db),
		consciousness: NewConsciousnessService(db),
		critical:      NewCriticalMemoryService(db),
	}
	svc.compilePatterns()
	return svc
}

// compilePatterns prepares regex patterns for text analysis
func (s *SuperhumanMemoryService) compilePatterns() {
	// Metaphor patterns
	metaphorPats := []string{
		`(?i)peso\s+no\s+peito`,
		`(?i)vazio\s+(?:por\s+)?dentro`,
		`(?i)(?:estou|me\s+sinto)\s+(?:num|no|em\s+um)\s+buraco`,
		`(?i)(?:estou|me\s+sinto)\s+perdido`,
		`(?i)relogio\s+parou`,
		`(?i)casa\s+(?:esta\s+)?vazia`,
		`(?i)sozinho\s+no\s+mundo`,
		`(?i)fim\s+do\s+tunel`,
		`(?i)luz\s+no\s+fim`,
		`(?i)coração\s+apertado`,
		`(?i)nó\s+na\s+garganta`,
		`(?i)vida\s+(?:não\s+tem|sem)\s+sentido`,
	}

	// Counterfactual patterns ("what if")
	cfPats := []string{
		`(?i)se\s+(?:eu\s+)?tivesse\s+(?:\w+\s+)?(\w+)`,
		`(?i)se\s+(?:eu\s+)?não\s+tivesse\s+(?:\w+\s+)?(\w+)`,
		`(?i)(?:poderia|deveria)\s+ter\s+(?:sido|feito)`,
		`(?i)arrependo\s+de\s+(?:não\s+)?ter`,
		`(?i)queria\s+ter\s+(?:\w+)`,
	}

	// Family/transgenerational patterns
	famPats := []string{
		`(?i)meu\s+(?:pai|mae|avo|avó)\s+(?:tambem|também)`,
		`(?i)na\s+minha\s+familia\s+(?:a\s+gente\s+)?(?:não|nunca)`,
		`(?i)(?:sempre\s+)?foi\s+assim\s+(?:na|em)\s+(?:minha\s+)?(?:familia|casa)`,
		`(?i)herdei\s+(?:isso|esse)`,
		`(?i)(?:meu\s+)?(?:pai|mae)\s+(?:me\s+)?ensinous`,
	}

	// Intention patterns
	intPats := []string{
		`(?i)(?:vou|preciso)\s+(?:ligar|falar|visitar)\s+(?:para|com|a)\s+(\w+)`,
		`(?i)(?:tenho|preciso)\s+(?:que|de)\s+(?:\w+)`,
		`(?i)(?:quero|gostaria\s+de)\s+(\w+)`,
		`(?i)(?:prometi|combinei)\s+(?:que\s+)?(?:ia|iria)`,
	}

	s.metaphorPatterns = compilePatterns(metaphorPats)
	s.counterfactualPatterns = compilePatterns(cfPats)
	s.familyPatterns = compilePatterns(famPats)
	s.intentionPatterns = compilePatterns(intPats)
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if r, err := regexp.Compile(p); err == nil {
			result = append(result, r)
		}
	}
	return result
}

// ProcessMemory processes a new memory through all 12 systems
func (s *SuperhumanMemoryService) ProcessMemory(ctx context.Context, idosoID int64, memoryID int64, text string, timestamp time.Time, metadata map[string]interface{}) error {
	log.Printf("🧠 [SUPERHUMAN] Processing memory for patient %d", idosoID)

	// Guard: degraded mode (no DB) — skip all sub-service processing
	if s.db == nil {
		log.Printf("⚠️ [SUPERHUMAN] Degraded mode — skipping full memory processing for patient %d", idosoID)
		return nil
	}

	// 1. Enneagram Detection
	go func() {
		if s.enneagram == nil {
			return
		}
		if _, err := s.enneagram.AnalyzeText(ctx, idosoID, text, memoryID); err != nil {
			log.Printf("⚠️ [ENNEAGRAM] Error: %v", err)
		}
	}()

	// 2. Self-Core (Identity Memory)
	go func() {
		if s.selfCore == nil {
			return
		}
		if err := s.selfCore.ProcessText(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [SELF_CORE] Error: %v", err)
		}
	}()

	// 3. Metaphorical Memory
	go func() {
		if err := s.processMetaphors(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [METAPHOR] Error: %v", err)
		}
	}()

	// 4. Counterfactual Memory
	go func() {
		if err := s.processCounterfactuals(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [COUNTERFACTUAL] Error: %v", err)
		}
	}()

	// 5. Transgenerational Memory
	go func() {
		if err := s.processFamilyPatterns(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [FAMILY] Error: %v", err)
		}
	}()

	// 6. Prospective Memory (Intentions)
	go func() {
		if err := s.processIntentions(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [INTENTION] Error: %v", err)
		}
	}()

	// 7. World Mapping (Persons, Places, Objects)
	go func() {
		if err := s.processWorldMapping(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [WORLD] Error: %v", err)
		}
	}()

	// 8. Somatic Correlations (if biometric data present)
	if metadata != nil {
		go func() {
			if err := s.processSomaticCorrelations(ctx, idosoID, text, metadata); err != nil {
				log.Printf("⚠️ [SOMATIC] Error: %v", err)
			}
		}()
	}

	// 9. Update Risk Score
	go func() {
		if err := s.updateRiskScore(ctx, idosoID); err != nil {
			log.Printf("⚠️ [RISK] Error: %v", err)
		}
	}()

	// 10. Track Circadian Patterns
	go func() {
		if err := s.updateCircadianPattern(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [CIRCADIAN] Error: %v", err)
		}
	}()

	// 11. Deep Memory - Persistent Memory (Trauma Detection)
	if s.deepMemory != nil {
		go func() {
			topics := s.extractTopics(text)
			for _, topic := range topics {
				// Detect avoidance patterns
				if err := s.deepMemory.DetectAvoidance(ctx, idosoID, text, topic, timestamp); err != nil {
					log.Printf("⚠️ [PERSISTENT] Avoidance error: %v", err)
				}
			}
			// Detect returns to traumatic topics
			if err := s.deepMemory.DetectReturn(ctx, idosoID, text, timestamp); err != nil {
				log.Printf("⚠️ [PERSISTENT] Return error: %v", err)
			}
		}()

		// 12. Deep Memory - Body Memory (Somatic)
		go func() {
			topics := s.extractTopics(text)
			if err := s.deepMemory.DetectBodySymptom(ctx, idosoID, text, topics, timestamp); err != nil {
				log.Printf("⚠️ [BODY_MEMORY] Error: %v", err)
			}
		}()

		// 13. Deep Memory - Shared Memory (Commemoration)
		go func() {
			if err := s.deepMemory.DetectSharingDesire(ctx, idosoID, text, timestamp); err != nil {
				log.Printf("⚠️ [SHARED_MEMORY] Error: %v", err)
			}
		}()
	}

	// 14. Life Markers Detection
	go func() {
		if err := s.processLifeMarkers(ctx, idosoID, text, timestamp); err != nil {
			log.Printf("⚠️ [LIFE_MARKERS] Error: %v", err)
		}
	}()

	// =========================================
	// CONSCIOUSNESS SYSTEMS (eva-memoria2.md)
	// =========================================

	// 15. Record Interaction & Update Relationship Phase
	go func() {
		if s.consciousness == nil {
			return
		}
		if phase, err := s.consciousness.RecordInteraction(ctx, idosoID); err != nil {
			log.Printf("⚠️ [CONSCIOUSNESS] Interaction error: %v", err)
		} else {
			log.Printf("🧠 [PHASE] Patient %d in phase: %s", idosoID, phase)
		}
	}()

	// 16. Update Rapport (based on interaction quality)
	go func() {
		if s.consciousness == nil {
			return
		}
		// Detect positive/negative interaction
		eventType, delta := s.analyzeInteractionSentiment(text)
		if err := s.consciousness.RecordRapportEvent(ctx, idosoID, eventType, text[:min(100, len(text))], delta); err != nil {
			log.Printf("⚠️ [RAPPORT] Error: %v", err)
		}
	}()

	// 17. Detect Cycle Patterns
	go func() {
		s.detectBehavioralCycles(ctx, idosoID, text, timestamp)
	}()

	// 18-20: Consciousness systems (empathic load, emotional state, memory gravity)
	if s.consciousness != nil {
		// 18. Add Empathic Load (based on emotional content)
		go func() {
			emotionalWeight := s.calculateEmotionalWeight(text)
			eventType := "normal"
			if emotionalWeight > 0.7 {
				eventType = "heavy_memory"
			} else if emotionalWeight > 0.9 {
				eventType = "trauma"
			}
			if _, err := s.consciousness.AddEmpathicLoad(ctx, idosoID, eventType, emotionalWeight); err != nil {
				log.Printf("⚠️ [EMPATHIC_LOAD] Error: %v", err)
			}
		}()

		// 19. Update Emotional State & Mode
		go func() {
			emotionalState, crisisLevel, receptivity := s.detectEmotionalState(text, metadata)
			if mode, err := s.consciousness.UpdateEmotionalState(ctx, idosoID, emotionalState, crisisLevel, receptivity); err != nil {
				log.Printf("⚠️ [MODE] Error: %v", err)
			} else {
				log.Printf("🎭 [MODE] Patient %d mode: %s (crisis: %.2f, receptivity: %.2f)",
					idosoID, mode, crisisLevel, receptivity)
			}
		}()

		// 20. Register Memory Gravity
		go func() {
			valence, arousal := s.calculateValenceArousal(text)
			if valence < -0.5 || arousal > 0.7 {
				// Heavy memory - register gravity
				if err := s.consciousness.RegisterMemoryGravity(ctx, idosoID, memoryID, "episode",
					text[:min(200, len(text))], valence, arousal); err != nil {
					log.Printf("⚠️ [GRAVITY] Error: %v", err)
				}
			}
		}()
	}

	// =========================================
	// CRITICAL MEMORY SYSTEMS (memoria-critica.md)
	// =========================================

	// 21-23: Critical memory systems (decay, clustering)
	if s.critical != nil {
		// 21. Apply Temporal Decay (memories fade over time)
		go func() {
			if _, err := s.critical.ApplyTemporalDecay(ctx, idosoID); err != nil {
				log.Printf("⚠️ [DECAY] Error: %v", err)
			}
		}()

		// 22. Cluster Similar Memories (abstraction)
		go func() {
			if err := s.critical.ClusterSimilarMemories(ctx, idosoID); err != nil {
				log.Printf("⚠️ [CLUSTER] Error: %v", err)
			}
		}()

		// 23. Auto-cluster by detected topics
		go func() {
			topics := s.extractTopics(text)
			for _, topic := range topics {
				s.critical.CreateOrUpdateCluster(ctx, idosoID, topic, "topic")
			}
		}()
	}

	return nil
}

// processMetaphors extracts metaphorical language
func (s *SuperhumanMemoryService) processMetaphors(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	textLower := strings.ToLower(text)

	for _, pattern := range s.metaphorPatterns {
		matches := pattern.FindAllString(textLower, -1)
		for _, match := range matches {
			metaphorType := s.classifyMetaphor(match)

			// Check if metaphor already exists
			rows, _ := s.db.QueryByLabel(ctx, "patient_metaphors",
				" AND n.idoso_id = $idoso AND n.metaphor = $metaphor",
				map[string]interface{}{"idoso": idosoID, "metaphor": match}, 1)

			if len(rows) > 0 {
				// Update existing
				count := database.GetInt64(rows[0], "usage_count")
				s.db.Update(ctx, "patient_metaphors",
					map[string]interface{}{"idoso_id": idosoID, "metaphor": match},
					map[string]interface{}{
						"usage_count": count + 1,
						"last_used":   timestamp.Format(time.RFC3339),
						"updated_at":  time.Now().Format(time.RFC3339),
					})
			} else {
				// Insert new
				s.db.Insert(ctx, "patient_metaphors", map[string]interface{}{
					"idoso_id":      idosoID,
					"metaphor":      match,
					"metaphor_type": metaphorType,
					"usage_count":   1,
					"first_used":    timestamp.Format(time.RFC3339),
					"last_used":     timestamp.Format(time.RFC3339),
					"contexts":      "[]",
				})
			}
		}
	}

	return nil
}

func (s *SuperhumanMemoryService) classifyMetaphor(metaphor string) string {
	corporalKeywords := []string{"peso", "vazio", "coração", "nó", "garganta", "peito"}
	spatialKeywords := []string{"buraco", "perdido", "tunel", "fim"}
	temporalKeywords := []string{"relogio", "parou", "tempo"}
	existentialKeywords := []string{"sentido", "vida", "mundo", "sozinho"}

	metaphorLower := strings.ToLower(metaphor)

	for _, kw := range corporalKeywords {
		if strings.Contains(metaphorLower, kw) {
			return "corporal"
		}
	}
	for _, kw := range spatialKeywords {
		if strings.Contains(metaphorLower, kw) {
			return "spatial"
		}
	}
	for _, kw := range temporalKeywords {
		if strings.Contains(metaphorLower, kw) {
			return "temporal"
		}
	}
	for _, kw := range existentialKeywords {
		if strings.Contains(metaphorLower, kw) {
			return "existential"
		}
	}

	return "other"
}

// processCounterfactuals extracts "what if" ruminations
func (s *SuperhumanMemoryService) processCounterfactuals(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	for _, pattern := range s.counterfactualPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			// Check if counterfactual already exists
			rows, _ := s.db.QueryByLabel(ctx, "patient_counterfactuals",
				" AND n.idoso_id = $idoso AND n.verbatim = $verbatim",
				map[string]interface{}{"idoso": idosoID, "verbatim": match}, 1)

			if len(rows) > 0 {
				count := database.GetInt64(rows[0], "mention_count")
				s.db.Update(ctx, "patient_counterfactuals",
					map[string]interface{}{"idoso_id": idosoID, "verbatim": match},
					map[string]interface{}{
						"mention_count":  count + 1,
						"last_mentioned": timestamp.Format(time.RFC3339),
						"updated_at":     time.Now().Format(time.RFC3339),
					})
			} else {
				s.db.Insert(ctx, "patient_counterfactuals", map[string]interface{}{
					"idoso_id":        idosoID,
					"verbatim":        match,
					"mention_count":   1,
					"first_mentioned": timestamp.Format(time.RFC3339),
					"last_mentioned":  timestamp.Format(time.RFC3339),
				})
			}
		}
	}

	return nil
}

// processFamilyPatterns extracts transgenerational patterns
func (s *SuperhumanMemoryService) processFamilyPatterns(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	for _, pattern := range s.familyPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			patternType := s.classifyFamilyPattern(match)
			generations := s.extractGenerations(match)

			genJSON, _ := json.Marshal(generations)

			// Check if family pattern already exists
			rows, _ := s.db.QueryByLabel(ctx, "patient_family_patterns",
				" AND n.idoso_id = $idoso AND n.pattern_verbatim = $verbatim",
				map[string]interface{}{"idoso": idosoID, "verbatim": match}, 1)

			if len(rows) > 0 {
				count := database.GetInt64(rows[0], "mention_count")
				s.db.Update(ctx, "patient_family_patterns",
					map[string]interface{}{"idoso_id": idosoID, "pattern_verbatim": match},
					map[string]interface{}{
						"mention_count":  count + 1,
						"last_mentioned": timestamp.Format(time.RFC3339),
						"updated_at":     time.Now().Format(time.RFC3339),
					})
			} else {
				s.db.Insert(ctx, "patient_family_patterns", map[string]interface{}{
					"idoso_id":              idosoID,
					"pattern_verbatim":      match,
					"pattern_type":          patternType,
					"generations_mentioned": string(genJSON),
					"mention_count":         1,
					"first_mentioned":       timestamp.Format(time.RFC3339),
					"last_mentioned":        timestamp.Format(time.RFC3339),
				})
			}
		}
	}

	return nil
}

func (s *SuperhumanMemoryService) classifyFamilyPattern(pattern string) string {
	patternLower := strings.ToLower(pattern)

	if strings.Contains(patternLower, "tambem") || strings.Contains(patternLower, "também") {
		return "inherited_behavior"
	}
	if strings.Contains(patternLower, "não") || strings.Contains(patternLower, "nunca") {
		return "family_mandate"
	}
	if strings.Contains(patternLower, "herdei") {
		return "generational_trauma"
	}
	if strings.Contains(patternLower, "sempre foi") {
		return "repetition"
	}

	return "inherited_behavior"
}

func (s *SuperhumanMemoryService) extractGenerations(text string) []string {
	generations := []string{}
	textLower := strings.ToLower(text)

	genKeywords := map[string]string{
		"bisavo": "bisavo", "bisavó": "bisavo",
		"avo": "avo", "avó": "avo", "avô": "avo",
		"pai": "pai", "mae": "mae", "mãe": "mae",
		"eu": "eu",
		"filho": "filho", "filha": "filho",
		"neto": "neto", "neta": "neto",
	}

	seen := make(map[string]bool)
	for kw, gen := range genKeywords {
		if strings.Contains(textLower, kw) && !seen[gen] {
			generations = append(generations, gen)
			seen[gen] = true
		}
	}

	return generations
}

// processIntentions extracts declared intentions
func (s *SuperhumanMemoryService) processIntentions(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	for _, pattern := range s.intentionPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			intention := match[0]
			relatedPerson := ""
			if len(match) > 1 {
				relatedPerson = match[1]
			}

			category := s.classifyIntention(intention)

			// Check if intention already exists (case-insensitive partial match)
			rows, _ := s.db.QueryByLabel(ctx, "patient_intentions",
				" AND n.idoso_id = $idoso",
				map[string]interface{}{"idoso": idosoID}, 0)

			found := false
			for _, r := range rows {
				existing := strings.ToLower(database.GetString(r, "intention_verbatim"))
				prefix := strings.ToLower(intention)
				if len(prefix) > 20 {
					prefix = prefix[:20]
				}
				if strings.Contains(existing, prefix) {
					count := database.GetInt64(r, "declaration_count")
					s.db.Update(ctx, "patient_intentions",
						map[string]interface{}{"idoso_id": idosoID, "intention_verbatim": database.GetString(r, "intention_verbatim")},
						map[string]interface{}{
							"declaration_count": count + 1,
							"last_declared":     timestamp.Format(time.RFC3339),
							"updated_at":        time.Now().Format(time.RFC3339),
						})
					found = true
					break
				}
			}

			if !found {
				s.db.Insert(ctx, "patient_intentions", map[string]interface{}{
					"idoso_id":            idosoID,
					"intention_verbatim":  intention,
					"category":            category,
					"related_person":      relatedPerson,
					"status":              "declared",
					"declaration_count":   1,
					"first_declared":      timestamp.Format(time.RFC3339),
					"last_declared":       timestamp.Format(time.RFC3339),
				})
			}
		}
	}

	return nil
}

func (s *SuperhumanMemoryService) classifyIntention(intention string) string {
	intentionLower := strings.ToLower(intention)

	if strings.Contains(intentionLower, "ligar") || strings.Contains(intentionLower, "falar") ||
		strings.Contains(intentionLower, "visitar") {
		return "contact"
	}
	if strings.Contains(intentionLower, "remedio") || strings.Contains(intentionLower, "medico") ||
		strings.Contains(intentionLower, "exame") {
		return "health"
	}
	if strings.Contains(intentionLower, "passear") || strings.Contains(intentionLower, "sair") {
		return "activity"
	}
	if strings.Contains(intentionLower, "pazes") || strings.Contains(intentionLower, "desculp") {
		return "relationship"
	}
	if strings.Contains(intentionLower, "descansar") || strings.Contains(intentionLower, "cuidar") {
		return "self_care"
	}

	return "other"
}

// processWorldMapping extracts persons, places, objects from text
func (s *SuperhumanMemoryService) processWorldMapping(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	// Extract person names (simplified - in production would use NER)
	personPatterns := []struct {
		pattern *regexp.Regexp
		role    string
	}{
		{regexp.MustCompile(`(?i)(?:minha?\s+)?(filha?|filho)\s+(\w+)`), "filho"},
		{regexp.MustCompile(`(?i)(?:minha?\s+)?(esposa?|esposo|marido|mulher)\s+(\w+)`), "conjuge"},
		{regexp.MustCompile(`(?i)(?:minha?\s+)?(neta?|neto)\s+(\w+)`), "neto"},
		{regexp.MustCompile(`(?i)(?:minha?\s+)?(irma?|irmao)\s+(\w+)`), "irmao"},
		{regexp.MustCompile(`(?i)(?:o|a)\s+(\w+)\s+(?:me\s+)?(?:disse|falou|ligou|visitou)`), ""},
	}

	for _, pp := range personPatterns {
		matches := pp.pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			var personName string
			if len(match) > 2 {
				personName = match[2]
			} else if len(match) > 1 {
				personName = match[1]
			}

			if len(personName) < 2 {
				continue
			}

			personName = strings.Title(strings.ToLower(personName))

			// Check if person already exists
			rows, _ := s.db.QueryByLabel(ctx, "patient_world_persons",
				" AND n.idoso_id = $idoso AND n.person_name = $name",
				map[string]interface{}{"idoso": idosoID, "name": personName}, 1)

			if len(rows) > 0 {
				count := database.GetInt64(rows[0], "mention_count")
				s.db.Update(ctx, "patient_world_persons",
					map[string]interface{}{"idoso_id": idosoID, "person_name": personName},
					map[string]interface{}{
						"mention_count":  count + 1,
						"last_mentioned": timestamp.Format(time.RFC3339),
						"updated_at":     time.Now().Format(time.RFC3339),
					})
			} else {
				s.db.Insert(ctx, "patient_world_persons", map[string]interface{}{
					"idoso_id":        idosoID,
					"person_name":     personName,
					"role":            pp.role,
					"mention_count":   1,
					"first_mentioned": timestamp.Format(time.RFC3339),
					"last_mentioned":  timestamp.Format(time.RFC3339),
				})
			}
		}
	}

	return nil
}

// processSomaticCorrelations correlates biometric data with speech
func (s *SuperhumanMemoryService) processSomaticCorrelations(ctx context.Context, idosoID int64, text string, metadata map[string]interface{}) error {
	// Extract topics from text (simplified)
	topics := s.extractTopics(text)

	// Check for biometric data in metadata
	somaticTypes := map[string]string{
		"glucose":     "blood_glucose",
		"glicemia":    "blood_glucose",
		"pressure":    "blood_pressure",
		"pressao":     "blood_pressure",
		"heart_rate":  "heart_rate",
		"sleep":       "sleep_quality",
		"sono":        "sleep_quality",
		"pain":        "pain_level",
		"dor":         "pain_level",
	}

	for key, somaticType := range somaticTypes {
		if value, ok := metadata[key]; ok {
			condition := s.categorizeCondition(somaticType, value)

			for _, topic := range topics {
				// Check if somatic correlation already exists
				rows, _ := s.db.QueryByLabel(ctx, "patient_somatic_correlations",
					" AND n.idoso_id = $idoso AND n.somatic_type = $stype AND n.condition_range = $cond AND n.correlated_topic = $topic",
					map[string]interface{}{"idoso": idosoID, "stype": somaticType, "cond": condition, "topic": topic}, 1)

				now := time.Now().Format(time.RFC3339)
				if len(rows) > 0 {
					count := database.GetInt64(rows[0], "observation_count")
					strength := database.GetFloat64(rows[0], "correlation_strength")
					newStrength := strength + 0.05
					if newStrength > 1.0 {
						newStrength = 1.0
					}
					s.db.Update(ctx, "patient_somatic_correlations",
						map[string]interface{}{
							"idoso_id":         idosoID,
							"somatic_type":     somaticType,
							"condition_range":  condition,
							"correlated_topic": topic,
						},
						map[string]interface{}{
							"observation_count":    count + 1,
							"correlation_strength": newStrength,
							"last_observed":        now,
							"updated_at":           now,
						})
				} else {
					s.db.Insert(ctx, "patient_somatic_correlations", map[string]interface{}{
						"idoso_id":              idosoID,
						"somatic_type":          somaticType,
						"condition_range":       condition,
						"correlated_topic":      topic,
						"correlation_strength":  0.5,
						"observation_count":     1,
						"first_observed":        now,
						"last_observed":         now,
					})
				}
			}
		}
	}

	return nil
}

func (s *SuperhumanMemoryService) categorizeCondition(somaticType string, value interface{}) string {
	// Simplified categorization
	switch somaticType {
	case "blood_glucose":
		if v, ok := value.(float64); ok {
			if v > 180 {
				return "high"
			} else if v < 70 {
				return "low"
			}
		}
	case "blood_pressure":
		if v, ok := value.(float64); ok {
			if v > 140 {
				return "high"
			} else if v < 90 {
				return "low"
			}
		}
	case "sleep_quality":
		if v, ok := value.(float64); ok {
			if v < 5 {
				return "poor"
			} else if v > 7 {
				return "good"
			}
		}
	case "pain_level":
		if v, ok := value.(float64); ok {
			if v > 6 {
				return "high"
			} else if v < 3 {
				return "low"
			}
		}
	}

	return "normal"
}

func (s *SuperhumanMemoryService) extractTopics(text string) []string {
	topicKeywords := map[string]string{
		"solidao": "solidao", "sozinho": "solidao",
		"familia": "familia", "filho": "familia", "filha": "familia",
		"morte": "morte", "morrer": "morte",
		"saude": "saude", "doenca": "saude", "dor": "saude",
		"dinheiro": "dinheiro", "financeiro": "dinheiro",
		"abandono": "abandono", "abandonado": "abandono",
	}

	textLower := strings.ToLower(text)
	seen := make(map[string]bool)
	topics := []string{}

	for kw, topic := range topicKeywords {
		if strings.Contains(textLower, kw) && !seen[topic] {
			topics = append(topics, topic)
			seen[topic] = true
		}
	}

	return topics
}

// updateRiskScore recalculates risk scores in Go (replaces PG calculate_risk_score)
func (s *SuperhumanMemoryService) updateRiskScore(ctx context.Context, idosoID int64) error {
	riskScore := 0.0

	// Check counterfactual frequency (rumination indicator)
	cfRows, _ := s.db.QueryByLabel(ctx, "patient_counterfactuals",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	totalMentions := 0
	for _, r := range cfRows {
		totalMentions += int(database.GetInt64(r, "mention_count"))
	}
	if totalMentions > 10 {
		riskScore += 0.2
	} else if totalMentions > 5 {
		riskScore += 0.1
	}

	// Check somatic correlations strength
	somaticRows, _ := s.db.QueryByLabel(ctx, "patient_somatic_correlations",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	for _, r := range somaticRows {
		strength := database.GetFloat64(r, "correlation_strength")
		if strength > 0.8 {
			riskScore += 0.1
		}
	}

	// Check persistent memories (trauma indicators)
	persistentRows, _ := s.db.QueryByLabel(ctx, "patient_persistent_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	for _, r := range persistentRows {
		if database.GetFloat64(r, "avoidance_count") > 3 {
			riskScore += 0.15
		}
	}

	// Cap at 1.0
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Store risk score
	rows, _ := s.db.QueryByLabel(ctx, "patient_risk_scores",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)

	now := time.Now().Format(time.RFC3339)
	if len(rows) > 0 {
		s.db.Update(ctx, "patient_risk_scores",
			map[string]interface{}{"idoso_id": idosoID},
			map[string]interface{}{
				"risk_score":  riskScore,
				"updated_at":  now,
			})
	} else {
		s.db.Insert(ctx, "patient_risk_scores", map[string]interface{}{
			"idoso_id":    idosoID,
			"risk_score":  riskScore,
			"created_at":  now,
			"updated_at":  now,
		})
	}

	return nil
}

// updateCircadianPattern updates time-based patterns
func (s *SuperhumanMemoryService) updateCircadianPattern(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	hour := timestamp.Hour()
	var timePeriod string

	switch {
	case hour >= 5 && hour < 8:
		timePeriod = "early_morning"
	case hour >= 8 && hour < 12:
		timePeriod = "morning"
	case hour >= 12 && hour < 18:
		timePeriod = "afternoon"
	case hour >= 18 && hour < 22:
		timePeriod = "evening"
	case hour >= 22 || hour < 2:
		timePeriod = "night"
	default:
		timePeriod = "late_night"
	}

	topics := s.extractTopics(text)

	// Check if circadian pattern already exists for this time period
	rows, _ := s.db.QueryByLabel(ctx, "patient_circadian_patterns",
		" AND n.idoso_id = $idoso AND n.time_period = $period",
		map[string]interface{}{"idoso": idosoID, "period": timePeriod}, 1)

	now := time.Now().Format(time.RFC3339)
	if len(rows) > 0 {
		// Merge themes (deduplicate)
		var existingThemes []string
		parseJSONStringSlice(rows[0]["recurring_themes"], &existingThemes)

		seen := make(map[string]bool)
		for _, t := range existingThemes {
			seen[t] = true
		}
		for _, t := range topics {
			if !seen[t] {
				existingThemes = append(existingThemes, t)
				seen[t] = true
			}
		}
		themesJSON, _ := json.Marshal(existingThemes)

		count := database.GetInt64(rows[0], "observation_count")
		s.db.Update(ctx, "patient_circadian_patterns",
			map[string]interface{}{"idoso_id": idosoID, "time_period": timePeriod},
			map[string]interface{}{
				"recurring_themes":  string(themesJSON),
				"observation_count": count + 1,
				"updated_at":        now,
			})
	} else {
		themesJSON, _ := json.Marshal(topics)
		s.db.Insert(ctx, "patient_circadian_patterns", map[string]interface{}{
			"idoso_id":          idosoID,
			"time_period":       timePeriod,
			"recurring_themes":  string(themesJSON),
			"observation_count": 1,
		})
	}

	return nil
}

// GetEnneagramService returns the Enneagram service
func (s *SuperhumanMemoryService) GetEnneagramService() *EnneagramService {
	return s.enneagram
}

// GetSelfCoreService returns the Self-Core service
func (s *SuperhumanMemoryService) GetSelfCoreService() *SelfCoreService {
	return s.selfCore
}

// GetMirror returns the Lacanian mirror service
func (s *SuperhumanMemoryService) GetMirror() *LacanianMirror {
	return s.mirror
}

// GetDeepMemoryService returns the Deep Memory service
func (s *SuperhumanMemoryService) GetDeepMemoryService() *DeepMemoryService {
	return s.deepMemory
}

// GetNarrativeWeaver returns the Narrative Weaver service
func (s *SuperhumanMemoryService) GetNarrativeWeaver() *NarrativeWeaver {
	return s.weaver
}

// GetConsciousnessService returns the Consciousness service (8 superhuman systems)
func (s *SuperhumanMemoryService) GetConsciousnessService() *ConsciousnessService {
	return s.consciousness
}

// GetCriticalMemoryService returns the Critical Memory service (4 critical systems)
func (s *SuperhumanMemoryService) GetCriticalMemoryService() *CriticalMemoryService {
	return s.critical
}

// =====================================================
// CONSCIOUSNESS HELPER FUNCTIONS
// =====================================================

// analyzeInteractionSentiment determines if interaction is positive/negative
func (s *SuperhumanMemoryService) analyzeInteractionSentiment(text string) (string, float64) {
	textLower := strings.ToLower(text)

	// Positive indicators
	positiveWords := []string{
		"obrigado", "obrigada", "agradeço", "gosto", "amo", "feliz",
		"alegre", "bom", "boa", "legal", "incrível", "maravilh",
		"ajudou", "entende", "compreende", "confiança", "confio",
	}

	// Negative indicators
	negativeWords := []string{
		"não entende", "inútil", "burra", "idiota", "odeio",
		"raiva", "frustrad", "decepcio", "mentira", "não funciona",
		"desist", "cansei", "lixo", "pior",
	}

	// Disclosure indicators (deep trust)
	disclosureWords := []string{
		"nunca contei", "segredo", "primeira vez que falo",
		"ninguém sabe", "só você", "confidencial", "vergonha de contar",
	}

	positiveCount := 0
	negativeCount := 0

	for _, word := range positiveWords {
		if strings.Contains(textLower, word) {
			positiveCount++
		}
	}

	for _, word := range negativeWords {
		if strings.Contains(textLower, word) {
			negativeCount++
		}
	}

	for _, word := range disclosureWords {
		if strings.Contains(textLower, word) {
			return "disclosure", 0.15 // Big trust boost
		}
	}

	if positiveCount > negativeCount {
		return "positive", 0.02 * float64(positiveCount)
	} else if negativeCount > positiveCount {
		return "negative", -0.01 * float64(negativeCount)
	}

	return "neutral", 0.005 // Small positive for any interaction
}

// detectBehavioralCycles detects patterns in text
func (s *SuperhumanMemoryService) detectBehavioralCycles(ctx context.Context, idosoID int64, text string, timestamp time.Time) {
	textLower := strings.ToLower(text)

	// Pattern signatures with their components
	cyclePatterns := []struct {
		signature   string
		description string
		patternType string
		triggers    []string
		indicator   string
	}{
		{
			signature:   "promise_break_diet",
			description: "Promete emagrecer mas não cumpre",
			patternType: "health",
			triggers:    []string{"vou emagrecer", "vou fazer dieta", "preciso perder peso"},
			indicator:   "starting_cycle",
		},
		{
			signature:   "promise_break_exercise",
			description: "Promete exercitar mas não cumpre",
			patternType: "health",
			triggers:    []string{"vou caminhar", "vou fazer exercicio", "vou malhar"},
			indicator:   "starting_cycle",
		},
		{
			signature:   "avoidance_contact",
			description: "Evita contato com pessoas importantes",
			patternType: "relational",
			triggers:    []string{"preciso ligar", "tenho que visitar", "vou falar com"},
			indicator:   "intention_declared",
		},
		{
			signature:   "rumination_past",
			description: "Ruminação sobre decisões passadas",
			patternType: "emotional",
			triggers:    []string{"se eu tivesse", "devia ter", "por que eu não"},
			indicator:   "ruminating",
		},
		{
			signature:   "self_sabotage",
			description: "Auto-sabotagem quando as coisas melhoram",
			patternType: "behavioral",
			triggers:    []string{"estava indo bem mas", "tava bom demais", "estraguei tudo"},
			indicator:   "post_sabotage",
		},
		{
			signature:   "victim_loop",
			description: "Loop de vitimização - rejeita ajuda oferecida",
			patternType: "relational",
			triggers:    []string{"ninguém me ajuda", "ninguém entende", "sempre eu que"},
			indicator:   "victim_statement",
		},
	}

	for _, pattern := range cyclePatterns {
		for _, trigger := range pattern.triggers {
			if strings.Contains(textLower, trigger) {
				// Detect the cycle
				s.consciousness.DetectCyclePattern(ctx, idosoID,
					pattern.signature, pattern.description, pattern.patternType,
					trigger, pattern.indicator, "cycle_continuation")

				log.Printf("🔄 [CYCLE] Detected '%s' pattern for patient %d", pattern.signature, idosoID)
				break
			}
		}
	}
}

// calculateEmotionalWeight calculates the emotional weight of text
func (s *SuperhumanMemoryService) calculateEmotionalWeight(text string) float64 {
	textLower := strings.ToLower(text)
	weight := 0.3 // base weight

	// Heavy topics
	heavyTopics := map[string]float64{
		"morte": 0.3, "morreu": 0.3, "morrer": 0.25,
		"suicidio": 0.4, "suicidar": 0.4, "me matar": 0.4,
		"abuso": 0.35, "violencia": 0.3, "agressao": 0.3,
		"abandono": 0.25, "abandonado": 0.25,
		"trauma": 0.3, "traumat": 0.3,
		"cancer": 0.25, "doença terminal": 0.3,
		"perdi tudo": 0.25, "desespero": 0.2,
		"solidao": 0.15, "sozinho": 0.15,
		"depressao": 0.2, "ansiedade": 0.15,
		"medo": 0.1, "raiva": 0.1,
		"choro": 0.15, "chorar": 0.15,
	}

	for topic, topicWeight := range heavyTopics {
		if strings.Contains(textLower, topic) {
			weight += topicWeight
		}
	}

	// Cap at 1.0
	if weight > 1.0 {
		weight = 1.0
	}

	return weight
}

// detectEmotionalState detects current emotional state from text and metadata
func (s *SuperhumanMemoryService) detectEmotionalState(text string, metadata map[string]interface{}) (string, float64, float64) {
	textLower := strings.ToLower(text)

	// Crisis indicators
	crisisWords := []string{
		"não aguento mais", "quero morrer", "vou me matar",
		"não vejo saída", "desisto", "acabou pra mim",
		"não tenho forças", "socorro", "preciso de ajuda urgente",
	}

	// State detection
	var emotionalState string
	var crisisLevel float64
	var receptivity float64 = 0.5

	for _, word := range crisisWords {
		if strings.Contains(textLower, word) {
			crisisLevel = 0.9
			emotionalState = "crise"
			receptivity = 0.2 // Low receptivity in crisis
			return emotionalState, crisisLevel, receptivity
		}
	}

	// Distress indicators
	distressWords := []string{
		"muito triste", "muito mal", "desesperado", "angustiado",
		"não consigo dormir", "pesadelo", "não paro de chorar",
	}

	for _, word := range distressWords {
		if strings.Contains(textLower, word) {
			emotionalState = "sofrimento"
			crisisLevel = 0.5
			receptivity = 0.4
			return emotionalState, crisisLevel, receptivity
		}
	}

	// Openness indicators (high receptivity)
	opennessWords := []string{
		"quero entender", "me ajuda a ver", "o que você acha",
		"preciso mudar", "quero melhorar", "estou pronto",
		"percebi que", "acho que você tem razão",
	}

	for _, word := range opennessWords {
		if strings.Contains(textLower, word) {
			emotionalState = "aberto"
			crisisLevel = 0.1
			receptivity = 0.8
			return emotionalState, crisisLevel, receptivity
		}
	}

	// Check biometric data if available
	if metadata != nil {
		if hr, ok := metadata["heart_rate"].(float64); ok && hr > 100 {
			crisisLevel += 0.2
			receptivity -= 0.1
		}
		if cortisol, ok := metadata["cortisol"].(float64); ok && cortisol > 20 {
			crisisLevel += 0.15
		}
	}

	emotionalState = "neutro"
	return emotionalState, crisisLevel, receptivity
}

// calculateValenceArousal calculates emotional valence and arousal from text
func (s *SuperhumanMemoryService) calculateValenceArousal(text string) (float64, float64) {
	textLower := strings.ToLower(text)

	valence := 0.0  // -1 (negative) to +1 (positive)
	arousal := 0.5  // 0 (calm) to 1 (intense)

	// Positive valence words
	positiveWords := map[string]float64{
		"feliz": 0.3, "alegre": 0.25, "contente": 0.2,
		"amor": 0.3, "amo": 0.3, "adoro": 0.25,
		"paz": 0.2, "tranquilo": 0.15, "calmo": 0.1,
		"esperança": 0.2, "otimista": 0.2,
		"gratidão": 0.25, "agradeço": 0.2,
	}

	// Negative valence words
	negativeWords := map[string]float64{
		"triste": -0.25, "tristeza": -0.3, "deprimido": -0.35,
		"raiva": -0.3, "ódio": -0.35, "odeio": -0.3,
		"medo": -0.25, "terror": -0.35, "pânico": -0.35,
		"culpa": -0.25, "vergonha": -0.25,
		"solidão": -0.2, "abandonado": -0.3,
		"desesperado": -0.35, "angústia": -0.3,
	}

	// Arousal words
	arousalWords := map[string]float64{
		"muito": 0.1, "demais": 0.1, "extremamente": 0.15,
		"!": 0.05, "gritando": 0.2, "chorando": 0.15,
		"tremendo": 0.15, "não aguento": 0.2,
		"urgente": 0.15, "desespero": 0.2,
	}

	for word, val := range positiveWords {
		if strings.Contains(textLower, word) {
			valence += val
		}
	}

	for word, val := range negativeWords {
		if strings.Contains(textLower, word) {
			valence += val
		}
	}

	for word, val := range arousalWords {
		if strings.Contains(textLower, word) {
			arousal += val
		}
	}

	// Clamp values
	if valence > 1.0 {
		valence = 1.0
	} else if valence < -1.0 {
		valence = -1.0
	}

	if arousal > 1.0 {
		arousal = 1.0
	}

	return valence, arousal
}

// processLifeMarkers extracts significant life events
func (s *SuperhumanMemoryService) processLifeMarkers(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	// Patterns for life markers
	markerPatterns := []struct {
		pattern    *regexp.Regexp
		markerType string
	}{
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:me\s+)?casei`), "casamento"},
		{regexp.MustCompile(`(?i)quando\s+(?:meu|minha)\s+(?:pai|mae|filho|filha|esposo|esposa)\s+(?:morreu|faleceu)`), "luto"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:me\s+)?aposentei`), "aposentadoria"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:tive|nasceu)\s+(?:meu|minha)\s+(?:primeiro|primeira)?\s*(?:filho|filha)`), "nascimento_filho"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:me\s+)?mudei\s+(?:para|de)`), "mudanca"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:perdi|fiquei\s+sem)\s+(?:o\s+)?(?:emprego|trabalho)`), "perda_emprego"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:fiquei|adoeci|descobri\s+que\s+tinha)`), "doenca"},
		{regexp.MustCompile(`(?i)quando\s+(?:eu\s+)?(?:me\s+)?separei|(?:meu|minha)\s+(?:casamento|relacao)\s+acabou`), "separacao"},
		{regexp.MustCompile(`(?i)em\s+(\d{4})\s+(?:eu|a\s+gente|nos)`), "ano_especifico"},
		{regexp.MustCompile(`(?i)aos\s+(\d+)\s+anos`), "idade_especifica"},
	}

	// Year extraction pattern
	yearPattern := regexp.MustCompile(`(?:em\s+)?(\d{4})`)
	agePattern := regexp.MustCompile(`aos\s+(\d+)\s+anos`)

	for _, mp := range markerPatterns {
		matches := mp.pattern.FindAllString(text, -1)
		for _, match := range matches {
			var year, age int

			// Try to extract year
			if yearMatch := yearPattern.FindStringSubmatch(text); len(yearMatch) > 1 {
				var y int
				if _, err := fmt.Sscanf(yearMatch[1], "%d", &y); err == nil && y >= 1900 && y <= 2100 {
					year = y
				}
			}

			// Try to extract age
			if ageMatch := agePattern.FindStringSubmatch(text); len(ageMatch) > 1 {
				var a int
				if _, err := fmt.Sscanf(ageMatch[1], "%d", &a); err == nil && a > 0 && a < 120 {
					age = a
				}
			}

			// Check if marker already exists (case-insensitive partial match)
			rows, _ := s.db.QueryByLabel(ctx, "patient_life_markers",
				" AND n.idoso_id = $idoso",
				map[string]interface{}{"idoso": idosoID}, 0)

			found := false
			prefix := strings.ToLower(match)
			if len(prefix) > 30 {
				prefix = prefix[:30]
			}
			for _, r := range rows {
				desc := strings.ToLower(database.GetString(r, "marker_description"))
				if strings.Contains(desc, prefix) {
					count := database.GetInt64(r, "mention_count")
					s.db.Update(ctx, "patient_life_markers",
						map[string]interface{}{"idoso_id": idosoID, "marker_description": database.GetString(r, "marker_description")},
						map[string]interface{}{
							"mention_count":  count + 1,
							"last_mentioned": timestamp.Format(time.RFC3339),
							"updated_at":     time.Now().Format(time.RFC3339),
						})
					found = true
					break
				}
			}

			if !found {
				content := map[string]interface{}{
					"idoso_id":           idosoID,
					"marker_description": match,
					"marker_type":        mp.markerType,
					"mention_count":      1,
					"first_mentioned":    timestamp.Format(time.RFC3339),
					"last_mentioned":     timestamp.Format(time.RFC3339),
				}
				if year > 0 {
					content["marker_year"] = year
				}
				if age > 0 {
					content["marker_age"] = age
				}
				s.db.Insert(ctx, "patient_life_markers", content)
			}
		}
	}

	return nil
}

// GenerateComprehensiveMirror generates all relevant mirror outputs for a patient
func (s *SuperhumanMemoryService) GenerateComprehensiveMirror(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	var outputs []*MirrorOutput

	// 1. Enneagram reflection
	if mo, err := s.enneagram.GenerateMirrorOutput(ctx, idosoID); err == nil && mo != nil {
		outputs = append(outputs, mo)
	}

	// 2. Identity reflection
	if mo, err := s.selfCore.GenerateMirrorOutput(ctx, idosoID); err == nil && mo != nil {
		outputs = append(outputs, mo)
	}

	// 3. Top unrealized intentions
	intentionRows, err := s.db.QueryByLabel(ctx, "patient_intentions",
		" AND n.idoso_id = $idoso AND (n.status = 'declared' OR n.status = 'blocked')",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil && len(intentionRows) > 0 {
		// Sort by declaration_count DESC
		sort.Slice(intentionRows, func(i, j int) bool {
			return database.GetInt64(intentionRows[i], "declaration_count") > database.GetInt64(intentionRows[j], "declaration_count")
		})
		limit := 3
		if len(intentionRows) < limit {
			limit = len(intentionRows)
		}
		for _, m := range intentionRows[:limit] {
			pi := &PatientIntention{
				IdosoID:            idosoID,
				IntentionVerbatim:  database.GetString(m, "intention_verbatim"),
				Category:           database.GetString(m, "category"),
				RelatedPerson:      database.GetString(m, "related_person"),
				Status:             database.GetString(m, "status"),
				DeclarationCount:   int(database.GetInt64(m, "declaration_count")),
				FirstDeclared:      database.GetTime(m, "first_declared"),
				LastDeclared:       database.GetTime(m, "last_declared"),
				StatedBlocker:      database.GetString(m, "stated_blocker"),
			}
			outputs = append(outputs, s.mirror.ReflectIntention(pi))
		}
	}

	// 4. Top metaphors
	metaphorRows, err := s.db.QueryByLabel(ctx, "patient_metaphors",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil && len(metaphorRows) > 0 {
		// Sort by usage_count DESC
		sort.Slice(metaphorRows, func(i, j int) bool {
			return database.GetInt64(metaphorRows[i], "usage_count") > database.GetInt64(metaphorRows[j], "usage_count")
		})
		limit := 3
		if len(metaphorRows) < limit {
			limit = len(metaphorRows)
		}
		for _, m := range metaphorRows[:limit] {
			pm := &PatientMetaphor{
				IdosoID:      idosoID,
				Metaphor:     database.GetString(m, "metaphor"),
				MetaphorType: database.GetString(m, "metaphor_type"),
				UsageCount:   int(database.GetInt64(m, "usage_count")),
				FirstUsed:    database.GetTime(m, "first_used"),
				LastUsed:     database.GetTime(m, "last_used"),
			}
			parseJSONStringSlice(m["correlated_topics"], &pm.CorrelatedTopics)
			parseJSONStringSlice(m["correlated_persons"], &pm.CorrelatedPersons)
			outputs = append(outputs, s.mirror.ReflectMetaphor(pm))
		}
	}

	// 5. Strong somatic correlations
	somaticRows, err := s.db.QueryByLabel(ctx, "patient_somatic_correlations",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil && len(somaticRows) > 0 {
		// Filter by strength >= 0.6 and sort by correlation_strength DESC
		var strongCorrelations []map[string]interface{}
		for _, m := range somaticRows {
			if database.GetFloat64(m, "correlation_strength") >= 0.6 {
				strongCorrelations = append(strongCorrelations, m)
			}
		}
		sort.Slice(strongCorrelations, func(i, j int) bool {
			return database.GetFloat64(strongCorrelations[i], "correlation_strength") > database.GetFloat64(strongCorrelations[j], "correlation_strength")
		})
		limit := 3
		if len(strongCorrelations) < limit {
			limit = len(strongCorrelations)
		}
		for _, m := range strongCorrelations[:limit] {
			sc := &SomaticCorrelation{
				IdosoID:             idosoID,
				SomaticType:         database.GetString(m, "somatic_type"),
				ConditionRange:      database.GetString(m, "condition_range"),
				CorrelatedTopic:     database.GetString(m, "correlated_topic"),
				CorrelationStrength: database.GetFloat64(m, "correlation_strength"),
				ObservationCount:    int(database.GetInt64(m, "observation_count")),
			}
			outputs = append(outputs, s.mirror.ReflectSomaticCorrelation(sc))
		}
	}

	// 6. Persistent Memories (Traumatic topics that persist)
	persistentMirrors, err := s.deepMemory.GeneratePersistentMirrors(ctx, idosoID)
	if err == nil {
		outputs = append(outputs, persistentMirrors...)
	}

	// 7. Body Memories (Somatic patterns with awareness check)
	bodyMirrors, err := s.deepMemory.GenerateBodyMemoryMirrors(ctx, idosoID)
	if err == nil {
		outputs = append(outputs, bodyMirrors...)
	}

	// 8. Shared Memories (Commemoration desires)
	sharedMirrors, err := s.deepMemory.GenerateSharedMemoryMirrors(ctx, idosoID)
	if err == nil {
		outputs = append(outputs, sharedMirrors...)
	}

	// 9. Life Narrative (Schacter's reconstruction)
	if lifeNarrative, err := s.weaver.BuildLifeNarrative(ctx, idosoID); err == nil && lifeNarrative != nil {
		outputs = append(outputs, lifeNarrative)
	}

	// 10. Top counterfactuals
	cfAllRows, err := s.db.QueryByLabel(ctx, "patient_counterfactuals",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil && len(cfAllRows) > 0 {
		// Sort by mention_count DESC
		sort.Slice(cfAllRows, func(i, j int) bool {
			return database.GetInt64(cfAllRows[i], "mention_count") > database.GetInt64(cfAllRows[j], "mention_count")
		})
		limit := 3
		if len(cfAllRows) < limit {
			limit = len(cfAllRows)
		}
		for _, m := range cfAllRows[:limit] {
			cf := &PatientCounterfactual{
				IdosoID:              idosoID,
				Verbatim:             database.GetString(m, "verbatim"),
				MentionCount:         int(database.GetInt64(m, "mention_count")),
				FirstMentioned:       database.GetTime(m, "first_mentioned"),
				LastMentioned:        database.GetTime(m, "last_mentioned"),
				VoiceTremorDetected:  database.GetBool(m, "voice_tremor_detected"),
				AvgEmotionalValence:  database.GetFloat64(m, "avg_emotional_valence"),
			}
			outputs = append(outputs, s.mirror.ReflectCounterfactual(cf))
		}
	}

	// =========================================
	// 11. CONSCIOUSNESS MIRRORS
	// =========================================
	consciousnessMirrors, err := s.consciousness.GenerateConsciousnessMirror(ctx, idosoID)
	if err == nil {
		outputs = append(outputs, consciousnessMirrors...)
	}

	// 12. Check Intervention Readiness
	if readiness, err := s.consciousness.CheckInterventionReadiness(ctx, idosoID); err == nil {
		if readiness.CanIntervene && readiness.PatternStrength > 0.6 {
			// Add intervention suggestion to outputs
			outputs = append(outputs, &MirrorOutput{
				Type: "intervention_ready",
				DataPoints: []string{
					fmt.Sprintf("Prontidao para intervencao: %.0f%%", readiness.ReadinessScore*100),
					fmt.Sprintf("Forca do padrao: %.0f%%", readiness.PatternStrength*100),
					fmt.Sprintf("Rapport: %.0f%%", readiness.Rapport*100),
					fmt.Sprintf("Acao recomendada: %s", readiness.RecommendedAction),
				},
				Question: "EVA detectou um padrao importante. Voce quer que eu compartilhe o que estou vendo?",
				RawData: map[string]interface{}{
					"readiness":          readiness.ReadinessScore,
					"pattern_strength":   readiness.PatternStrength,
					"rapport":            readiness.Rapport,
					"mode":               readiness.CurrentMode,
					"recommended_action": readiness.RecommendedAction,
				},
			})
		}
	}

	// =========================================
	// 13. CRITICAL MEMORY MIRRORS
	// =========================================
	criticalMirrors, err := s.critical.GenerateCriticalMirrors(ctx, idosoID)
	if err == nil {
		outputs = append(outputs, criticalMirrors...)
	}

	return outputs, nil
}
