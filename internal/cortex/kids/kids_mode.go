// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package kids

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// EVA KIDS MODE - Assistente Gamificada para Criancas
// ============================================================================
// Transforma tarefas em missoes, controle parental, aprendizado gamificado

// KidsService gerencia o modo infantil da EVA
type KidsService struct {
	db         *database.DB
	notifyFunc func(userID int64, msgType string, payload interface{})
}

// ChildProfile perfil da crianca
type ChildProfile struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Age            int       `json:"age"`
	ParentID       int64     `json:"parent_id"`       // ID do responsavel
	TotalPoints    int       `json:"total_points"`    // Pontos acumulados
	CurrentLevel   int       `json:"current_level"`   // Nivel atual (1-100)
	CurrentStreak  int       `json:"current_streak"`  // Sequencia de dias
	AvatarURL      string    `json:"avatar_url"`      // Avatar customizado
	Preferences    string    `json:"preferences"`     // JSON com preferencias
	SafeContacts   []string  `json:"safe_contacts"`   // Contatos permitidos
	GeofenceRadius int       `json:"geofence_radius"` // Raio da cerca geografica (metros)
	GeofenceCenter string    `json:"geofence_center"` // Lat,Lng do centro (casa)
	BlockedApps    []string  `json:"blocked_apps"`    // Apps bloqueados durante estudo
	StudySchedule  string    `json:"study_schedule"`  // JSON com horarios de estudo
	CreatedAt      time.Time `json:"created_at"`
}

// Mission representa uma missao/tarefa gamificada
type Mission struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	Title       string    `json:"title"`       // "Escovar os dentes"
	Description string    `json:"description"` // "Escove por 2 minutos!"
	Category    string    `json:"category"`    // hygiene, study, chores, health, social
	Points      int       `json:"points"`      // Pontos da missao
	XP          int       `json:"xp"`          // Experiencia para subir de nivel
	Difficulty  string    `json:"difficulty"`  // easy, medium, hard, epic
	Icon        string    `json:"icon"`        // Emoji da missao
	DueTime     *string   `json:"due_time"`    // Horario limite (HH:MM)
	Recurring   bool      `json:"recurring"`   // Se repete diariamente
	RepeatDays  []int     `json:"repeat_days"` // Dias da semana [0-6]
	Status      string    `json:"status"`      // pending, completed, failed, skipped
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// Achievement conquista desbloqueada
type Achievement struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`        // "streak_7", "missions_100"
	Title       string    `json:"title"`       // "Uma Semana Campea!"
	Description string    `json:"description"` // "Complete missoes por 7 dias seguidos"
	Icon        string    `json:"icon"`        // trophy
	Points      int       `json:"points"`      // Bonus de pontos
	Rarity      string    `json:"rarity"`      // common, rare, epic, legendary
	UnlockedAt  time.Time `json:"unlocked_at"`
}

// KnowledgeCard carta de conhecimento (Zettelkasten infantil)
type KnowledgeCard struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	Topic       string    `json:"topic"`       // "Leoes"
	Content     string    `json:"content"`     // Explicacao simples
	Category    string    `json:"category"`    // animals, science, history, language
	ImageURL    string    `json:"image_url"`   // Imagem ilustrativa
	LinkedCards []int64   `json:"linked_cards"` // Cartas relacionadas
	TimesAsked  int       `json:"times_asked"` // Quantas vezes perguntou
	Mastery     int       `json:"mastery"`     // 0-100 dominio do tema
	NextReview  time.Time `json:"next_review"` // Proxima revisao (spaced repetition)
	CreatedAt   time.Time `json:"created_at"`
}

// StorySession sessao de historia interativa
type StorySession struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	StoryTitle  string    `json:"story_title"`
	CurrentPage int       `json:"current_page"`
	Choices     []string  `json:"choices"`     // Escolhas feitas
	IsComplete  bool      `json:"is_complete"`
	CreatedAt   time.Time `json:"created_at"`
}

// SafetyAlert alerta de seguranca
type SafetyAlert struct {
	Type      string    `json:"type"`      // geofence, unknown_contact, danger_word, low_battery
	Severity  string    `json:"severity"`  // info, warning, critical
	Message   string    `json:"message"`
	Location  string    `json:"location"`  // Lat,Lng se aplicavel
	Timestamp time.Time `json:"timestamp"`
}

// Dificuldades e pontos
var difficultyPoints = map[string]struct{ points, xp int }{
	"easy":   {10, 5},
	"medium": {25, 15},
	"hard":   {50, 30},
	"epic":   {100, 75},
}

// Niveis e XP necessario
func xpForLevel(level int) int {
	return level * 100 // Simples: nivel 2 = 200 XP, nivel 10 = 1000 XP
}

// NewKidsService cria novo servico
func NewKidsService(db *database.DB) *KidsService {
	return &KidsService{db: db}
}

// SetNotifyFunc configura funcao de notificacao
func (k *KidsService) SetNotifyFunc(fn func(userID int64, msgType string, payload interface{})) {
	k.notifyFunc = fn
}

// ============================================================================
// GESTAO DE MISSOES
// ============================================================================

// CreateMission cria uma nova missao
func (k *KidsService) CreateMission(ctx context.Context, childID int64, title, description, category, difficulty string, dueTime *string, recurring bool, repeatDays []int) (*Mission, error) {
	if difficulty == "" {
		difficulty = "easy"
	}

	dp, ok := difficultyPoints[difficulty]
	if !ok {
		dp = difficultyPoints["easy"]
	}

	icon := k.getMissionIcon(category)
	repeatDaysJSON, _ := json.Marshal(repeatDays)

	content := map[string]interface{}{
		"child_id":    childID,
		"title":       title,
		"description": description,
		"category":    category,
		"points":      dp.points,
		"xp":          dp.xp,
		"difficulty":  difficulty,
		"icon":        icon,
		"due_time":    "",
		"recurring":   recurring,
		"repeat_days": string(repeatDaysJSON),
		"status":      "pending",
		"created_at":  time.Now().Format(time.RFC3339),
	}
	if dueTime != nil {
		content["due_time"] = *dueTime
	}

	id, err := k.db.Insert(ctx, "kids_missions", content)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar missao: %w", err)
	}

	mission := &Mission{
		ID:          id,
		ChildID:     childID,
		Title:       title,
		Description: description,
		Category:    category,
		Points:      dp.points,
		XP:          dp.xp,
		Difficulty:  difficulty,
		Icon:        icon,
		DueTime:     dueTime,
		Recurring:   recurring,
		RepeatDays:  repeatDays,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	log.Printf("[KIDS] Missao criada ID=%d: '%s' (%s, %d pts)", mission.ID, title, difficulty, dp.points)

	return mission, nil
}

// CompleteMission marca missao como concluida e da pontos
func (k *KidsService) CompleteMission(ctx context.Context, missionID int64) (*Mission, error) {
	// Buscar missao
	rows, err := k.db.QueryByLabel(ctx, "kids_missions",
		" AND n.id = $mid",
		map[string]interface{}{"mid": missionID}, 1)
	if err != nil || len(rows) == 0 {
		return nil, fmt.Errorf("missao nao encontrada: %v", err)
	}

	m := rows[0]
	mission := &Mission{
		ID:         database.GetInt64(m, "id"),
		ChildID:    database.GetInt64(m, "child_id"),
		Title:      database.GetString(m, "title"),
		Points:     int(database.GetInt64(m, "points")),
		XP:         int(database.GetInt64(m, "xp")),
		Difficulty: database.GetString(m, "difficulty"),
		Status:     database.GetString(m, "status"),
	}

	if mission.Status == "completed" {
		return mission, nil // Ja foi completada
	}

	// Marcar como completada
	now := time.Now()
	err = k.db.Update(ctx, "kids_missions",
		map[string]interface{}{"id": missionID},
		map[string]interface{}{
			"status":       "completed",
			"completed_at": now.Format(time.RFC3339),
		})
	if err != nil {
		return nil, err
	}

	// Dar pontos e XP
	err = k.addPointsAndXP(ctx, mission.ChildID, mission.Points, mission.XP)
	if err != nil {
		log.Printf("[KIDS] Erro ao adicionar pontos: %v", err)
	}

	// Verificar conquistas
	go k.checkAchievements(mission.ChildID)

	mission.Status = "completed"
	mission.CompletedAt = &now

	log.Printf("[KIDS] Missao completada ID=%d: '%s' (+%d pts, +%d xp)", missionID, mission.Title, mission.Points, mission.XP)

	// Notificar
	if k.notifyFunc != nil {
		k.notifyFunc(mission.ChildID, "mission_completed", map[string]interface{}{
			"mission_id": mission.ID,
			"title":      mission.Title,
			"points":     mission.Points,
			"xp":         mission.XP,
			"message":    k.getCelebrationMessage(),
		})
	}

	return mission, nil
}

// GetPendingMissions retorna missoes pendentes do dia
func (k *KidsService) GetPendingMissions(ctx context.Context, childID int64) ([]Mission, error) {
	rows, err := k.db.QueryByLabel(ctx, "kids_missions",
		" AND n.child_id = $child_id AND n.status = $status",
		map[string]interface{}{"child_id": childID, "status": "pending"}, 0)
	if err != nil {
		return nil, err
	}

	today := int(time.Now().Weekday())
	var missions []Mission
	for _, m := range rows {
		recurring := database.GetBool(m, "recurring")

		// Filter by repeat days if recurring
		if recurring {
			repeatDaysStr := database.GetString(m, "repeat_days")
			if repeatDaysStr != "" && repeatDaysStr != "[]" {
				var repeatDays []int
				json.Unmarshal([]byte(repeatDaysStr), &repeatDays)
				if len(repeatDays) > 0 {
					found := false
					for _, d := range repeatDays {
						if d == today {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
			}
		}

		dueTime := database.GetString(m, "due_time")
		var dueTimePtr *string
		if dueTime != "" {
			dueTimePtr = &dueTime
		}

		mission := Mission{
			ID:          database.GetInt64(m, "id"),
			ChildID:     childID,
			Title:       database.GetString(m, "title"),
			Description: database.GetString(m, "description"),
			Category:    database.GetString(m, "category"),
			Points:      int(database.GetInt64(m, "points")),
			XP:          int(database.GetInt64(m, "xp")),
			Difficulty:  database.GetString(m, "difficulty"),
			Icon:        database.GetString(m, "icon"),
			DueTime:     dueTimePtr,
			Status:      "pending",
			CreatedAt:   database.GetTime(m, "created_at"),
		}
		missions = append(missions, mission)
	}

	return missions, nil
}

// ============================================================================
// PONTOS E NIVEIS
// ============================================================================

func (k *KidsService) addPointsAndXP(ctx context.Context, childID int64, points, xp int) error {
	// Get current profile
	rows, err := k.db.QueryByLabel(ctx, "kids_profiles",
		" AND n.id = $child_id",
		map[string]interface{}{"child_id": childID}, 1)
	if err != nil || len(rows) == 0 {
		// Se nao existe profile, criar
		return k.ensureProfile(ctx, childID)
	}

	m := rows[0]
	currentPoints := int(database.GetInt64(m, "total_points"))
	currentXP := int(database.GetInt64(m, "current_xp"))
	currentLevel := int(database.GetInt64(m, "current_level"))

	newPoints := currentPoints + points
	newXP := currentXP + xp

	err = k.db.Update(ctx, "kids_profiles",
		map[string]interface{}{"id": childID},
		map[string]interface{}{
			"total_points": newPoints,
			"current_xp":   newXP,
			"updated_at":   time.Now().Format(time.RFC3339),
		})
	if err != nil {
		return err
	}

	// Verificar level up
	neededXP := xpForLevel(currentLevel + 1)
	if newXP >= neededXP {
		newLevel := currentLevel + 1
		k.db.Update(ctx, "kids_profiles",
			map[string]interface{}{"id": childID},
			map[string]interface{}{
				"current_level": newLevel,
				"current_xp":   newXP - neededXP,
			})

		if k.notifyFunc != nil {
			k.notifyFunc(childID, "level_up", map[string]interface{}{
				"new_level": newLevel,
				"message":   fmt.Sprintf("Parabens! Voce subiu para o Nivel %d!", newLevel),
			})
		}
	}

	return nil
}

func (k *KidsService) ensureProfile(ctx context.Context, childID int64) error {
	// Check if profile already exists
	rows, _ := k.db.QueryByLabel(ctx, "kids_profiles",
		" AND n.id = $child_id",
		map[string]interface{}{"child_id": childID}, 1)
	if len(rows) > 0 {
		return nil // Already exists
	}

	_, err := k.db.Insert(ctx, "kids_profiles", map[string]interface{}{
		"id":             childID,
		"total_points":   0,
		"current_level":  1,
		"current_xp":     0,
		"current_streak": 0,
		"created_at":     time.Now().Format(time.RFC3339),
	})
	return err
}

// GetStats retorna estatisticas da crianca
func (k *KidsService) GetStats(ctx context.Context, childID int64) (map[string]interface{}, error) {
	rows, err := k.db.QueryByLabel(ctx, "kids_profiles",
		" AND n.id = $child_id",
		map[string]interface{}{"child_id": childID}, 1)
	if err != nil || len(rows) == 0 {
		k.ensureProfile(ctx, childID)
		return map[string]interface{}{
			"points": 0, "level": 1, "xp": 0, "streak": 0,
			"next_level_xp": 200,
		}, nil
	}

	m := rows[0]
	points := int(database.GetInt64(m, "total_points"))
	level := int(database.GetInt64(m, "current_level"))
	xp := int(database.GetInt64(m, "current_xp"))
	streak := int(database.GetInt64(m, "current_streak"))

	// Contar missoes completadas hoje
	completedToday := 0
	todayStr := time.Now().Format("2006-01-02")
	missionRows, _ := k.db.QueryByLabel(ctx, "kids_missions",
		" AND n.child_id = $child_id AND n.status = $status",
		map[string]interface{}{"child_id": childID, "status": "completed"}, 0)
	for _, mr := range missionRows {
		completedAt := database.GetString(mr, "completed_at")
		if strings.HasPrefix(completedAt, todayStr) {
			completedToday++
		}
	}

	// Contar conquistas
	achRows, _ := k.db.QueryByLabel(ctx, "kids_achievements",
		" AND n.child_id = $child_id",
		map[string]interface{}{"child_id": childID}, 0)
	achievements := len(achRows)

	return map[string]interface{}{
		"points":          points,
		"level":           level,
		"xp":              xp,
		"next_level_xp":   xpForLevel(level + 1),
		"streak":          streak,
		"completed_today": completedToday,
		"achievements":    achievements,
	}, nil
}

// ============================================================================
// CONQUISTAS
// ============================================================================

func (k *KidsService) checkAchievements(childID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Definir conquistas disponiveis
	achievements := []struct {
		code        string
		title       string
		description string
		icon        string
		rarity      string
		points      int
		checkLabel  string
		checkWhere  string
		checkParams map[string]interface{}
		threshold   int
		useProfile  bool // if true, check profile field instead of counting
		profileField string
	}{
		{"first_mission", "Primeira Missao!", "Complete sua primeira missao", "star", "common", 50,
			"kids_missions", " AND n.child_id = $child_id AND n.status = $status",
			map[string]interface{}{"child_id": childID, "status": "completed"}, 1, false, ""},
		{"missions_10", "Heroi em Treinamento", "Complete 10 missoes", "hero", "common", 100,
			"kids_missions", " AND n.child_id = $child_id AND n.status = $status",
			map[string]interface{}{"child_id": childID, "status": "completed"}, 10, false, ""},
		{"missions_50", "Super Heroi!", "Complete 50 missoes", "superhero", "rare", 250,
			"kids_missions", " AND n.child_id = $child_id AND n.status = $status",
			map[string]interface{}{"child_id": childID, "status": "completed"}, 50, false, ""},
		{"missions_100", "Lenda Viva", "Complete 100 missoes", "trophy", "epic", 500,
			"kids_missions", " AND n.child_id = $child_id AND n.status = $status",
			map[string]interface{}{"child_id": childID, "status": "completed"}, 100, false, ""},
		{"streak_3", "Tres Dias Seguidos!", "Complete missoes por 3 dias", "fire", "common", 75,
			"kids_profiles", " AND n.id = $child_id",
			map[string]interface{}{"child_id": childID}, 3, true, "current_streak"},
		{"streak_7", "Uma Semana Campea!", "Complete missoes por 7 dias seguidos", "fire2", "rare", 200,
			"kids_profiles", " AND n.id = $child_id",
			map[string]interface{}{"child_id": childID}, 7, true, "current_streak"},
		{"streak_30", "Mes Imbativel!", "Complete missoes por 30 dias seguidos", "star2", "legendary", 1000,
			"kids_profiles", " AND n.id = $child_id",
			map[string]interface{}{"child_id": childID}, 30, true, "current_streak"},
	}

	for _, ach := range achievements {
		// Verificar se ja tem
		existRows, _ := k.db.QueryByLabel(ctx, "kids_achievements",
			" AND n.child_id = $child_id AND n.code = $code",
			map[string]interface{}{"child_id": childID, "code": ach.code}, 1)
		if len(existRows) > 0 {
			continue
		}

		// Verificar condicao
		var count int
		if ach.useProfile {
			rows, _ := k.db.QueryByLabel(ctx, ach.checkLabel, ach.checkWhere, ach.checkParams, 1)
			if len(rows) > 0 {
				count = int(database.GetInt64(rows[0], ach.profileField))
			}
		} else {
			rows, _ := k.db.QueryByLabel(ctx, ach.checkLabel, ach.checkWhere, ach.checkParams, 0)
			count = len(rows)
		}

		if count >= ach.threshold {
			// Desbloquear conquista
			_, err := k.db.Insert(ctx, "kids_achievements", map[string]interface{}{
				"child_id":    childID,
				"code":        ach.code,
				"title":       ach.title,
				"description": ach.description,
				"icon":        ach.icon,
				"points":      ach.points,
				"rarity":      ach.rarity,
				"unlocked_at": time.Now().Format(time.RFC3339),
			})

			if err == nil {
				// Dar pontos bonus
				k.addPointsAndXP(ctx, childID, ach.points, ach.points/2)

				// Notificar
				if k.notifyFunc != nil {
					k.notifyFunc(childID, "achievement_unlocked", map[string]interface{}{
						"code":        ach.code,
						"title":       ach.title,
						"description": ach.description,
						"icon":        ach.icon,
						"rarity":      ach.rarity,
						"points":      ach.points,
						"message":     fmt.Sprintf("%s Conquista desbloqueada: %s!", ach.icon, ach.title),
					})
				}

				log.Printf("[KIDS] Conquista desbloqueada para %d: %s", childID, ach.title)
			}
		}
	}
}

// ============================================================================
// SEGURANCA E CONTROLE PARENTAL
// ============================================================================

// CheckGeofence verifica se a crianca esta dentro da cerca
func (k *KidsService) CheckGeofence(ctx context.Context, childID int64, lat, lng float64) (*SafetyAlert, error) {
	rows, err := k.db.QueryByLabel(ctx, "kids_profiles",
		" AND n.id = $child_id",
		map[string]interface{}{"child_id": childID}, 1)
	if err != nil || len(rows) == 0 {
		return nil, nil
	}

	m := rows[0]
	radius := int(database.GetInt64(m, "geofence_radius"))
	centerStr := database.GetString(m, "geofence_center")

	if radius == 0 || centerStr == "" {
		return nil, nil // Geofence nao configurado
	}

	// Parse center (lat,lng)
	var centerLat, centerLng float64
	fmt.Sscanf(centerStr, "%f,%f", &centerLat, &centerLng)

	// Calcular distancia (formula simplificada)
	distance := k.haversineDistance(centerLat, centerLng, lat, lng)

	if distance > float64(radius) {
		alert := &SafetyAlert{
			Type:      "geofence",
			Severity:  "critical",
			Message:   fmt.Sprintf("Crianca saiu da area segura! Distancia: %.0fm", distance),
			Location:  fmt.Sprintf("%.6f,%.6f", lat, lng),
			Timestamp: time.Now(),
		}

		// Notificar pais
		if k.notifyFunc != nil {
			parentID := database.GetInt64(m, "parent_id")
			if parentID > 0 {
				k.notifyFunc(parentID, "geofence_alert", map[string]interface{}{
					"child_id": childID,
					"alert":    alert,
					"message":  alert.Message,
				})
			}
		}

		log.Printf("[KIDS] ALERTA GEOFENCE: Crianca %d fora da area!", childID)
		return alert, nil
	}

	return nil, nil
}

// CheckContactAllowed verifica se um contato e permitido
func (k *KidsService) CheckContactAllowed(ctx context.Context, childID int64, phoneNumber string) (bool, error) {
	rows, err := k.db.QueryByLabel(ctx, "kids_profiles",
		" AND n.id = $child_id",
		map[string]interface{}{"child_id": childID}, 1)
	if err != nil || len(rows) == 0 {
		return true, nil // Se nao tem config, permite
	}

	m := rows[0]
	contactsJSON := database.GetString(m, "safe_contacts")
	if contactsJSON == "" {
		return true, nil
	}

	var contacts []string
	json.Unmarshal([]byte(contactsJSON), &contacts)

	// Normalizar numero
	phone := strings.ReplaceAll(phoneNumber, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")

	for _, allowed := range contacts {
		allowed = strings.ReplaceAll(allowed, " ", "")
		allowed = strings.ReplaceAll(allowed, "-", "")
		if strings.HasSuffix(phone, allowed) || strings.HasSuffix(allowed, phone) {
			return true, nil
		}
	}

	// Contato nao permitido - alertar pais
	if k.notifyFunc != nil {
		parentID := database.GetInt64(m, "parent_id")
		if parentID > 0 {
			k.notifyFunc(parentID, "unknown_contact_blocked", map[string]interface{}{
				"child_id":     childID,
				"phone_number": phoneNumber,
				"message":      fmt.Sprintf("Tentativa de contato bloqueada: %s", phoneNumber),
			})
		}
	}

	return false, nil
}

// DetectDangerWords verifica palavras de perigo no audio
func (k *KidsService) DetectDangerWords(ctx context.Context, childID int64, transcript string) (*SafetyAlert, error) {
	dangerWords := []string{
		"socorro", "me ajuda", "ajuda", "estou perdido", "perdida",
		"machucou", "cai", "doi muito", "sangue", "medo", "assustado",
		"estranho", "desconhecido", "nao conheco", "sumiu",
	}

	lower := strings.ToLower(transcript)
	for _, word := range dangerWords {
		if strings.Contains(lower, word) {
			alert := &SafetyAlert{
				Type:      "danger_word",
				Severity:  "warning",
				Message:   fmt.Sprintf("Palavra de alerta detectada: '%s'", word),
				Timestamp: time.Now(),
			}

			// Notificar pais
			if k.notifyFunc != nil {
				rows, _ := k.db.QueryByLabel(ctx, "kids_profiles",
					" AND n.id = $child_id",
					map[string]interface{}{"child_id": childID}, 1)
				if len(rows) > 0 {
					parentID := database.GetInt64(rows[0], "parent_id")
					if parentID > 0 {
						k.notifyFunc(parentID, "danger_word_alert", map[string]interface{}{
							"child_id":   childID,
							"transcript": transcript,
							"word":       word,
							"alert":      alert,
						})
					}
				}
			}

			log.Printf("[KIDS] Palavra de perigo detectada para %d: '%s'", childID, word)
			return alert, nil
		}
	}

	return nil, nil
}

// ============================================================================
// ZETTELKASTEN EDUCATIVO
// ============================================================================

// CreateKnowledgeCard cria uma carta de conhecimento
func (k *KidsService) CreateKnowledgeCard(ctx context.Context, childID int64, topic, content, category string) (*KnowledgeCard, error) {
	// Verificar se ja existe carta similar
	rows, _ := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.child_id = $child_id AND n.topic = $topic",
		map[string]interface{}{"child_id": childID, "topic": topic}, 1)

	if len(rows) > 0 {
		// Atualizar existente
		existingID := database.GetInt64(rows[0], "id")
		err := k.db.Update(ctx, "kids_knowledge_cards",
			map[string]interface{}{"id": existingID},
			map[string]interface{}{
				"content":     content,
				"times_asked": database.GetInt64(rows[0], "times_asked") + 1,
				"updated_at":  time.Now().Format(time.RFC3339),
			})
		if err != nil {
			return nil, err
		}
		return k.getKnowledgeCard(ctx, existingID)
	}

	// Criar nova
	now := time.Now()
	nextReview := now.Add(24 * time.Hour)

	id, err := k.db.Insert(ctx, "kids_knowledge_cards", map[string]interface{}{
		"child_id":    childID,
		"topic":       topic,
		"content":     content,
		"category":    category,
		"times_asked": 1,
		"mastery":     0,
		"next_review": nextReview.Format(time.RFC3339),
		"created_at":  now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}

	card := &KnowledgeCard{
		ID:        id,
		ChildID:   childID,
		Topic:     topic,
		Content:   content,
		Category:  category,
		CreatedAt: now,
	}

	// Buscar cartas relacionadas
	go k.linkRelatedCards(childID, card.ID, topic, category)

	log.Printf("[KIDS] Carta de conhecimento criada: '%s' para crianca %d", topic, childID)

	return card, nil
}

// GetRelatedTopics retorna topicos relacionados para estimular conexoes
func (k *KidsService) GetRelatedTopics(ctx context.Context, childID int64, topic string) ([]string, error) {
	// Buscar a carta pelo topico
	rows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.child_id = $child_id AND n.topic = $topic",
		map[string]interface{}{"child_id": childID, "topic": topic}, 1)
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	m := rows[0]
	category := database.GetString(m, "category")

	// Buscar cartas da mesma categoria
	relatedRows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.child_id = $child_id AND n.category = $category AND n.topic != $topic",
		map[string]interface{}{"child_id": childID, "category": category, "topic": topic}, 5)
	if err != nil {
		return nil, err
	}

	var topics []string
	for _, r := range relatedRows {
		topics = append(topics, database.GetString(r, "topic"))
	}

	return topics, nil
}

// GetCardsForReview retorna cartas para revisao (spaced repetition)
func (k *KidsService) GetCardsForReview(ctx context.Context, childID int64, limit int) ([]KnowledgeCard, error) {
	if limit == 0 {
		limit = 5
	}

	// Get all cards for child, filter by next_review in Go
	rows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.child_id = $child_id",
		map[string]interface{}{"child_id": childID}, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var cards []KnowledgeCard
	for _, m := range rows {
		nextReview := database.GetTime(m, "next_review")
		if !nextReview.IsZero() && nextReview.After(now) {
			continue // Not yet due for review
		}

		c := KnowledgeCard{
			ID:         database.GetInt64(m, "id"),
			ChildID:    childID,
			Topic:      database.GetString(m, "topic"),
			Content:    database.GetString(m, "content"),
			Category:   database.GetString(m, "category"),
			Mastery:    int(database.GetInt64(m, "mastery")),
			TimesAsked: int(database.GetInt64(m, "times_asked")),
		}
		cards = append(cards, c)

		if len(cards) >= limit {
			break
		}
	}

	return cards, nil
}

// RecordReview registra resultado de revisao
func (k *KidsService) RecordReview(ctx context.Context, cardID int64, correct bool) error {
	// Get current card state
	rows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.id = $card_id",
		map[string]interface{}{"card_id": cardID}, 1)
	if err != nil || len(rows) == 0 {
		return err
	}

	m := rows[0]
	mastery := int(database.GetInt64(m, "mastery"))
	timesAsked := int(database.GetInt64(m, "times_asked"))

	var nextReview time.Time
	now := time.Now()

	if correct {
		mastery += 10
		if mastery > 100 {
			mastery = 100
		}
		// Spaced repetition intervals
		if mastery < 30 {
			nextReview = now.Add(24 * time.Hour)
		} else if mastery < 60 {
			nextReview = now.Add(3 * 24 * time.Hour)
		} else if mastery < 80 {
			nextReview = now.Add(7 * 24 * time.Hour)
		} else {
			nextReview = now.Add(14 * 24 * time.Hour)
		}
	} else {
		mastery -= 5
		if mastery < 0 {
			mastery = 0
		}
		nextReview = now.Add(4 * time.Hour)
	}

	return k.db.Update(ctx, "kids_knowledge_cards",
		map[string]interface{}{"id": cardID},
		map[string]interface{}{
			"mastery":     mastery,
			"next_review": nextReview.Format(time.RFC3339),
			"times_asked": timesAsked + 1,
			"updated_at":  now.Format(time.RFC3339),
		})
}

func (k *KidsService) getKnowledgeCard(ctx context.Context, cardID int64) (*KnowledgeCard, error) {
	rows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.id = $card_id",
		map[string]interface{}{"card_id": cardID}, 1)
	if err != nil || len(rows) == 0 {
		return nil, fmt.Errorf("card not found")
	}

	m := rows[0]
	card := &KnowledgeCard{
		ID:         database.GetInt64(m, "id"),
		ChildID:    database.GetInt64(m, "child_id"),
		Topic:      database.GetString(m, "topic"),
		Content:    database.GetString(m, "content"),
		Category:   database.GetString(m, "category"),
		Mastery:    int(database.GetInt64(m, "mastery")),
		TimesAsked: int(database.GetInt64(m, "times_asked")),
		NextReview: database.GetTime(m, "next_review"),
		CreatedAt:  database.GetTime(m, "created_at"),
	}
	return card, nil
}

func (k *KidsService) linkRelatedCards(childID, cardID int64, topic, category string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Buscar cartas da mesma categoria
	rows, err := k.db.QueryByLabel(ctx, "kids_knowledge_cards",
		" AND n.child_id = $child_id AND n.category = $category AND n.id != $card_id",
		map[string]interface{}{"child_id": childID, "category": category, "card_id": cardID}, 5)
	if err != nil {
		return
	}

	var relatedIDs []int64
	for _, m := range rows {
		relatedIDs = append(relatedIDs, database.GetInt64(m, "id"))
	}

	if len(relatedIDs) > 0 {
		idsJSON, _ := json.Marshal(relatedIDs)
		k.db.Update(ctx, "kids_knowledge_cards",
			map[string]interface{}{"id": cardID},
			map[string]interface{}{
				"linked_cards": string(idsJSON),
			})
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func (k *KidsService) getMissionIcon(category string) string {
	icons := map[string]string{
		"hygiene": "toothbrush",
		"study":   "book",
		"chores":  "broom",
		"health":  "muscle",
		"social":  "wave",
		"food":    "apple",
		"sleep":   "sleepy",
	}
	if icon, ok := icons[category]; ok {
		return icon
	}
	return "star"
}

func (k *KidsService) getCelebrationMessage() string {
	messages := []string{
		"Incrivel! Voce e demais!",
		"Missao cumprida, campeao!",
		"Voce esta arrasando!",
		"Que orgulho de voce!",
		"Continue assim, heroi!",
		"Fantastico! Mais uma vitoria!",
		"Voce e uma estrela!",
	}
	return messages[rand.Intn(len(messages))]
}

func (k *KidsService) haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	// Formula simplificada de Haversine
	const R = 6371000 // Raio da Terra em metros

	lat1Rad := lat1 * 3.14159 / 180
	lat2Rad := lat2 * 3.14159 / 180
	deltaLat := (lat2 - lat1) * 3.14159 / 180
	deltaLng := (lng2 - lng1) * 3.14159 / 180

	a := (1 - (deltaLat*deltaLat + deltaLng*deltaLng*lat1Rad*lat2Rad)) / 2
	if a < 0 {
		a = 0
	}

	return 2 * R * a // Aproximacao
}
