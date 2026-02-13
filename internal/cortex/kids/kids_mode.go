package kids

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

// ============================================================================
// EVA KIDS MODE - Assistente Gamificada para Crianças
// ============================================================================
// Transforma tarefas em missões, controle parental, aprendizado gamificado

// KidsService gerencia o modo infantil da EVA
type KidsService struct {
	db         *sql.DB
	notifyFunc func(userID int64, msgType string, payload interface{})
}

// ChildProfile perfil da criança
type ChildProfile struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Age            int       `json:"age"`
	ParentID       int64     `json:"parent_id"`       // ID do responsável
	TotalPoints    int       `json:"total_points"`    // Pontos acumulados
	CurrentLevel   int       `json:"current_level"`   // Nível atual (1-100)
	CurrentStreak  int       `json:"current_streak"`  // Sequência de dias
	AvatarURL      string    `json:"avatar_url"`      // Avatar customizado
	Preferences    string    `json:"preferences"`     // JSON com preferências
	SafeContacts   []string  `json:"safe_contacts"`   // Contatos permitidos
	GeofenceRadius int       `json:"geofence_radius"` // Raio da cerca geográfica (metros)
	GeofenceCenter string    `json:"geofence_center"` // Lat,Lng do centro (casa)
	BlockedApps    []string  `json:"blocked_apps"`    // Apps bloqueados durante estudo
	StudySchedule  string    `json:"study_schedule"`  // JSON com horários de estudo
	CreatedAt      time.Time `json:"created_at"`
}

// Mission representa uma missão/tarefa gamificada
type Mission struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	Title       string    `json:"title"`       // "Escovar os dentes"
	Description string    `json:"description"` // "Escove por 2 minutos!"
	Category    string    `json:"category"`    // hygiene, study, chores, health, social
	Points      int       `json:"points"`      // Pontos da missão
	XP          int       `json:"xp"`          // Experiência para subir de nível
	Difficulty  string    `json:"difficulty"`  // easy, medium, hard, epic
	Icon        string    `json:"icon"`        // Emoji da missão
	DueTime     *string   `json:"due_time"`    // Horário limite (HH:MM)
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
	Title       string    `json:"title"`       // "Uma Semana Campeã!"
	Description string    `json:"description"` // "Complete missões por 7 dias seguidos"
	Icon        string    `json:"icon"`        // 🏆
	Points      int       `json:"points"`      // Bônus de pontos
	Rarity      string    `json:"rarity"`      // common, rare, epic, legendary
	UnlockedAt  time.Time `json:"unlocked_at"`
}

// KnowledgeCard carta de conhecimento (Zettelkasten infantil)
type KnowledgeCard struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	Topic       string    `json:"topic"`       // "Leões"
	Content     string    `json:"content"`     // Explicação simples
	Category    string    `json:"category"`    // animals, science, history, language
	ImageURL    string    `json:"image_url"`   // Imagem ilustrativa
	LinkedCards []int64   `json:"linked_cards"` // Cartas relacionadas
	TimesAsked  int       `json:"times_asked"` // Quantas vezes perguntou
	Mastery     int       `json:"mastery"`     // 0-100 domínio do tema
	NextReview  time.Time `json:"next_review"` // Próxima revisão (spaced repetition)
	CreatedAt   time.Time `json:"created_at"`
}

// StorySession sessão de história interativa
type StorySession struct {
	ID          int64     `json:"id"`
	ChildID     int64     `json:"child_id"`
	StoryTitle  string    `json:"story_title"`
	CurrentPage int       `json:"current_page"`
	Choices     []string  `json:"choices"`     // Escolhas feitas
	IsComplete  bool      `json:"is_complete"`
	CreatedAt   time.Time `json:"created_at"`
}

// SafetyAlert alerta de segurança
type SafetyAlert struct {
	Type      string    `json:"type"`      // geofence, unknown_contact, danger_word, low_battery
	Severity  string    `json:"severity"`  // info, warning, critical
	Message   string    `json:"message"`
	Location  string    `json:"location"`  // Lat,Lng se aplicável
	Timestamp time.Time `json:"timestamp"`
}

// Dificuldades e pontos
var difficultyPoints = map[string]struct{ points, xp int }{
	"easy":   {10, 5},
	"medium": {25, 15},
	"hard":   {50, 30},
	"epic":   {100, 75},
}

// Níveis e XP necessário
func xpForLevel(level int) int {
	return level * 100 // Simples: nível 2 = 200 XP, nível 10 = 1000 XP
}

// NewKidsService cria novo serviço
func NewKidsService(db *sql.DB) *KidsService {
	svc := &KidsService{db: db}
	if err := svc.createTables(); err != nil {
		log.Printf("⚠️ [KIDS] Erro ao criar tabelas: %v", err)
	}
	return svc
}

// SetNotifyFunc configura função de notificação
func (k *KidsService) SetNotifyFunc(fn func(userID int64, msgType string, payload interface{})) {
	k.notifyFunc = fn
}

// ============================================================================
// GESTÃO DE MISSÕES
// ============================================================================

// CreateMission cria uma nova missão
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

	query := `
		INSERT INTO kids_missions (child_id, title, description, category, points, xp, difficulty, icon, due_time, recurring, repeat_days, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending', NOW())
		RETURNING id, created_at
	`

	var mission Mission
	err := k.db.QueryRowContext(ctx, query,
		childID, title, description, category, dp.points, dp.xp, difficulty, icon, dueTime, recurring, string(repeatDaysJSON),
	).Scan(&mission.ID, &mission.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar missão: %w", err)
	}

	mission.ChildID = childID
	mission.Title = title
	mission.Description = description
	mission.Category = category
	mission.Points = dp.points
	mission.XP = dp.xp
	mission.Difficulty = difficulty
	mission.Icon = icon
	mission.DueTime = dueTime
	mission.Recurring = recurring
	mission.RepeatDays = repeatDays
	mission.Status = "pending"

	log.Printf("🎮 [KIDS] Missão criada ID=%d: '%s' (%s, %d pts)", mission.ID, title, difficulty, dp.points)

	return &mission, nil
}

// CompleteMission marca missão como concluída e dá pontos
func (k *KidsService) CompleteMission(ctx context.Context, missionID int64) (*Mission, error) {
	// Buscar missão
	query := `
		SELECT id, child_id, title, points, xp, difficulty, status
		FROM kids_missions
		WHERE id = $1
	`
	var mission Mission
	err := k.db.QueryRowContext(ctx, query, missionID).Scan(
		&mission.ID, &mission.ChildID, &mission.Title, &mission.Points, &mission.XP, &mission.Difficulty, &mission.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("missão não encontrada: %w", err)
	}

	if mission.Status == "completed" {
		return &mission, nil // Já foi completada
	}

	// Marcar como completada
	now := time.Now()
	_, err = k.db.ExecContext(ctx, `
		UPDATE kids_missions SET status = 'completed', completed_at = $1 WHERE id = $2
	`, now, missionID)
	if err != nil {
		return nil, err
	}

	// Dar pontos e XP
	err = k.addPointsAndXP(ctx, mission.ChildID, mission.Points, mission.XP)
	if err != nil {
		log.Printf("⚠️ [KIDS] Erro ao adicionar pontos: %v", err)
	}

	// Verificar conquistas
	go k.checkAchievements(mission.ChildID)

	mission.Status = "completed"
	mission.CompletedAt = &now

	log.Printf("🎉 [KIDS] Missão completada ID=%d: '%s' (+%d pts, +%d xp)", missionID, mission.Title, mission.Points, mission.XP)

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

	return &mission, nil
}

// GetPendingMissions retorna missões pendentes do dia
func (k *KidsService) GetPendingMissions(ctx context.Context, childID int64) ([]Mission, error) {
	today := int(time.Now().Weekday())

	query := `
		SELECT id, child_id, title, description, category, points, xp, difficulty, icon, due_time, status, created_at
		FROM kids_missions
		WHERE child_id = $1 AND status = 'pending'
		  AND (recurring = false OR repeat_days::jsonb @> $2::jsonb OR repeat_days = '[]')
		ORDER BY due_time ASC NULLS LAST, points DESC
	`

	rows, err := k.db.QueryContext(ctx, query, childID, fmt.Sprintf("[%d]", today))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var missions []Mission
	for rows.Next() {
		var m Mission
		var dueTime sql.NullString
		if err := rows.Scan(&m.ID, &m.ChildID, &m.Title, &m.Description, &m.Category, &m.Points, &m.XP, &m.Difficulty, &m.Icon, &dueTime, &m.Status, &m.CreatedAt); err != nil {
			continue
		}
		if dueTime.Valid {
			m.DueTime = &dueTime.String
		}
		missions = append(missions, m)
	}

	return missions, nil
}

// ============================================================================
// PONTOS E NÍVEIS
// ============================================================================

func (k *KidsService) addPointsAndXP(ctx context.Context, childID int64, points, xp int) error {
	// Atualizar pontos e XP
	query := `
		UPDATE kids_profiles
		SET total_points = total_points + $1,
		    current_xp = current_xp + $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING current_xp, current_level
	`

	var currentXP, currentLevel int
	err := k.db.QueryRowContext(ctx, query, childID, points, xp).Scan(&currentXP, &currentLevel)
	if err != nil {
		// Se não existe profile, criar
		return k.ensureProfile(ctx, childID)
	}

	// Verificar level up
	neededXP := xpForLevel(currentLevel + 1)
	if currentXP >= neededXP {
		newLevel := currentLevel + 1
		_, err = k.db.ExecContext(ctx, `
			UPDATE kids_profiles SET current_level = $1, current_xp = current_xp - $2 WHERE id = $3
		`, newLevel, neededXP, childID)

		if err == nil && k.notifyFunc != nil {
			k.notifyFunc(childID, "level_up", map[string]interface{}{
				"new_level": newLevel,
				"message":   fmt.Sprintf("🎊 Parabéns! Você subiu para o Nível %d!", newLevel),
			})
		}
	}

	return nil
}

func (k *KidsService) ensureProfile(ctx context.Context, childID int64) error {
	query := `
		INSERT INTO kids_profiles (id, total_points, current_level, current_xp, current_streak, created_at)
		VALUES ($1, 0, 1, 0, 0, NOW())
		ON CONFLICT (id) DO NOTHING
	`
	_, err := k.db.ExecContext(ctx, query, childID)
	return err
}

// GetStats retorna estatísticas da criança
func (k *KidsService) GetStats(ctx context.Context, childID int64) (map[string]interface{}, error) {
	query := `
		SELECT total_points, current_level, current_xp, current_streak
		FROM kids_profiles
		WHERE id = $1
	`

	var points, level, xp, streak int
	err := k.db.QueryRowContext(ctx, query, childID).Scan(&points, &level, &xp, &streak)
	if err != nil {
		k.ensureProfile(ctx, childID)
		return map[string]interface{}{
			"points": 0, "level": 1, "xp": 0, "streak": 0,
			"next_level_xp": 200,
		}, nil
	}

	// Contar missões completadas hoje
	var completedToday int
	k.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kids_missions
		WHERE child_id = $1 AND status = 'completed' AND completed_at::date = CURRENT_DATE
	`, childID).Scan(&completedToday)

	// Contar conquistas
	var achievements int
	k.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kids_achievements WHERE child_id = $1
	`, childID).Scan(&achievements)

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

	// Definir conquistas disponíveis
	achievements := []struct {
		code        string
		title       string
		description string
		icon        string
		rarity      string
		points      int
		checkSQL    string
		threshold   int
	}{
		{"first_mission", "Primeira Missão!", "Complete sua primeira missão", "⭐", "common", 50,
			"SELECT COUNT(*) FROM kids_missions WHERE child_id = $1 AND status = 'completed'", 1},
		{"missions_10", "Herói em Treinamento", "Complete 10 missões", "🦸", "common", 100,
			"SELECT COUNT(*) FROM kids_missions WHERE child_id = $1 AND status = 'completed'", 10},
		{"missions_50", "Super Herói!", "Complete 50 missões", "🦸‍♂️", "rare", 250,
			"SELECT COUNT(*) FROM kids_missions WHERE child_id = $1 AND status = 'completed'", 50},
		{"missions_100", "Lenda Viva", "Complete 100 missões", "🏆", "epic", 500,
			"SELECT COUNT(*) FROM kids_missions WHERE child_id = $1 AND status = 'completed'", 100},
		{"streak_3", "Três Dias Seguidos!", "Complete missões por 3 dias", "🔥", "common", 75,
			"SELECT current_streak FROM kids_profiles WHERE id = $1", 3},
		{"streak_7", "Uma Semana Campeã!", "Complete missões por 7 dias seguidos", "🔥🔥", "rare", 200,
			"SELECT current_streak FROM kids_profiles WHERE id = $1", 7},
		{"streak_30", "Mês Imbatível!", "Complete missões por 30 dias seguidos", "🌟", "legendary", 1000,
			"SELECT current_streak FROM kids_profiles WHERE id = $1", 30},
		{"early_bird", "Madrugador", "Complete uma missão antes das 8h", "🌅", "rare", 150,
			"SELECT COUNT(*) FROM kids_missions WHERE child_id = $1 AND status = 'completed' AND EXTRACT(HOUR FROM completed_at) < 8", 1},
	}

	for _, ach := range achievements {
		// Verificar se já tem
		var exists bool
		k.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM kids_achievements WHERE child_id = $1 AND code = $2)", childID, ach.code).Scan(&exists)
		if exists {
			continue
		}

		// Verificar condição
		var count int
		k.db.QueryRowContext(ctx, ach.checkSQL, childID).Scan(&count)
		if count >= ach.threshold {
			// Desbloquear conquista
			_, err := k.db.ExecContext(ctx, `
				INSERT INTO kids_achievements (child_id, code, title, description, icon, points, rarity, unlocked_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			`, childID, ach.code, ach.title, ach.description, ach.icon, ach.points, ach.rarity)

			if err == nil {
				// Dar pontos bônus
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

				log.Printf("🏆 [KIDS] Conquista desbloqueada para %d: %s", childID, ach.title)
			}
		}
	}
}

// ============================================================================
// SEGURANÇA E CONTROLE PARENTAL
// ============================================================================

// CheckGeofence verifica se a criança está dentro da cerca
func (k *KidsService) CheckGeofence(ctx context.Context, childID int64, lat, lng float64) (*SafetyAlert, error) {
	query := `
		SELECT geofence_radius, geofence_center
		FROM kids_profiles
		WHERE id = $1
	`

	var radius int
	var centerStr string
	err := k.db.QueryRowContext(ctx, query, childID).Scan(&radius, &centerStr)
	if err != nil || radius == 0 || centerStr == "" {
		return nil, nil // Geofence não configurado
	}

	// Parse center (lat,lng)
	var centerLat, centerLng float64
	fmt.Sscanf(centerStr, "%f,%f", &centerLat, &centerLng)

	// Calcular distância (fórmula simplificada)
	distance := k.haversineDistance(centerLat, centerLng, lat, lng)

	if distance > float64(radius) {
		alert := &SafetyAlert{
			Type:      "geofence",
			Severity:  "critical",
			Message:   fmt.Sprintf("Criança saiu da área segura! Distância: %.0fm", distance),
			Location:  fmt.Sprintf("%.6f,%.6f", lat, lng),
			Timestamp: time.Now(),
		}

		// Notificar pais
		if k.notifyFunc != nil {
			var parentID int64
			k.db.QueryRowContext(ctx, "SELECT parent_id FROM kids_profiles WHERE id = $1", childID).Scan(&parentID)
			if parentID > 0 {
				k.notifyFunc(parentID, "geofence_alert", map[string]interface{}{
					"child_id": childID,
					"alert":    alert,
					"message":  alert.Message,
				})
			}
		}

		log.Printf("🚨 [KIDS] ALERTA GEOFENCE: Criança %d fora da área!", childID)
		return alert, nil
	}

	return nil, nil
}

// CheckContactAllowed verifica se um contato é permitido
func (k *KidsService) CheckContactAllowed(ctx context.Context, childID int64, phoneNumber string) (bool, error) {
	query := `
		SELECT safe_contacts
		FROM kids_profiles
		WHERE id = $1
	`

	var contactsJSON string
	err := k.db.QueryRowContext(ctx, query, childID).Scan(&contactsJSON)
	if err != nil {
		return true, nil // Se não tem config, permite
	}

	var contacts []string
	json.Unmarshal([]byte(contactsJSON), &contacts)

	// Normalizar número
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

	// Contato não permitido - alertar pais
	if k.notifyFunc != nil {
		var parentID int64
		k.db.QueryRowContext(ctx, "SELECT parent_id FROM kids_profiles WHERE id = $1", childID).Scan(&parentID)
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

// DetectDangerWords verifica palavras de perigo no áudio
func (k *KidsService) DetectDangerWords(ctx context.Context, childID int64, transcript string) (*SafetyAlert, error) {
	dangerWords := []string{
		"socorro", "me ajuda", "ajuda", "estou perdido", "perdida",
		"machucou", "caí", "dói muito", "sangue", "medo", "assustado",
		"estranho", "desconhecido", "não conheço", "sumiu",
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
				var parentID int64
				k.db.QueryRowContext(ctx, "SELECT parent_id FROM kids_profiles WHERE id = $1", childID).Scan(&parentID)
				if parentID > 0 {
					k.notifyFunc(parentID, "danger_word_alert", map[string]interface{}{
						"child_id":   childID,
						"transcript": transcript,
						"word":       word,
						"alert":      alert,
					})
				}
			}

			log.Printf("⚠️ [KIDS] Palavra de perigo detectada para %d: '%s'", childID, word)
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
	// Verificar se já existe carta similar
	var existingID int64
	k.db.QueryRowContext(ctx, `
		SELECT id FROM kids_knowledge_cards
		WHERE child_id = $1 AND LOWER(topic) = LOWER($2)
	`, childID, topic).Scan(&existingID)

	if existingID > 0 {
		// Atualizar existente
		_, err := k.db.ExecContext(ctx, `
			UPDATE kids_knowledge_cards
			SET content = $1, times_asked = times_asked + 1, updated_at = NOW()
			WHERE id = $2
		`, content, existingID)
		if err != nil {
			return nil, err
		}
		return k.getKnowledgeCard(ctx, existingID)
	}

	// Criar nova
	query := `
		INSERT INTO kids_knowledge_cards (child_id, topic, content, category, times_asked, mastery, next_review, created_at)
		VALUES ($1, $2, $3, $4, 1, 0, NOW() + INTERVAL '1 day', NOW())
		RETURNING id, created_at
	`

	var card KnowledgeCard
	err := k.db.QueryRowContext(ctx, query, childID, topic, content, category).Scan(&card.ID, &card.CreatedAt)
	if err != nil {
		return nil, err
	}

	card.ChildID = childID
	card.Topic = topic
	card.Content = content
	card.Category = category

	// Buscar cartas relacionadas
	go k.linkRelatedCards(childID, card.ID, topic, category)

	log.Printf("📚 [KIDS] Carta de conhecimento criada: '%s' para criança %d", topic, childID)

	return &card, nil
}

// GetRelatedTopics retorna tópicos relacionados para estimular conexões
func (k *KidsService) GetRelatedTopics(ctx context.Context, childID int64, topic string) ([]string, error) {
	// Buscar cartas relacionadas
	query := `
		SELECT DISTINCT kc2.topic
		FROM kids_knowledge_cards kc1
		JOIN kids_knowledge_cards kc2 ON kc1.child_id = kc2.child_id
		WHERE kc1.child_id = $1 AND LOWER(kc1.topic) = LOWER($2) AND kc1.id != kc2.id
		  AND (kc1.category = kc2.category OR kc1.linked_cards @> ARRAY[kc2.id])
		LIMIT 5
	`

	rows, err := k.db.QueryContext(ctx, query, childID, topic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			topics = append(topics, t)
		}
	}

	return topics, nil
}

// GetCardsForReview retorna cartas para revisão (spaced repetition)
func (k *KidsService) GetCardsForReview(ctx context.Context, childID int64, limit int) ([]KnowledgeCard, error) {
	if limit == 0 {
		limit = 5
	}

	query := `
		SELECT id, topic, content, category, mastery, times_asked
		FROM kids_knowledge_cards
		WHERE child_id = $1 AND next_review <= NOW()
		ORDER BY mastery ASC, times_asked DESC
		LIMIT $2
	`

	rows, err := k.db.QueryContext(ctx, query, childID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []KnowledgeCard
	for rows.Next() {
		var c KnowledgeCard
		if rows.Scan(&c.ID, &c.Topic, &c.Content, &c.Category, &c.Mastery, &c.TimesAsked) == nil {
			c.ChildID = childID
			cards = append(cards, c)
		}
	}

	return cards, nil
}

// RecordReview registra resultado de revisão
func (k *KidsService) RecordReview(ctx context.Context, cardID int64, correct bool) error {
	// Calcular novo intervalo baseado em acerto/erro
	var interval string
	var masteryDelta int

	if correct {
		interval = "CASE WHEN mastery < 30 THEN INTERVAL '1 day' WHEN mastery < 60 THEN INTERVAL '3 days' WHEN mastery < 80 THEN INTERVAL '1 week' ELSE INTERVAL '2 weeks' END"
		masteryDelta = 10
	} else {
		interval = "INTERVAL '4 hours'"
		masteryDelta = -5
	}

	query := fmt.Sprintf(`
		UPDATE kids_knowledge_cards
		SET mastery = GREATEST(0, LEAST(100, mastery + %d)),
		    next_review = NOW() + %s,
		    times_asked = times_asked + 1,
		    updated_at = NOW()
		WHERE id = $1
	`, masteryDelta, interval)

	_, err := k.db.ExecContext(ctx, query, cardID)
	return err
}

func (k *KidsService) getKnowledgeCard(ctx context.Context, cardID int64) (*KnowledgeCard, error) {
	query := `
		SELECT id, child_id, topic, content, category, mastery, times_asked, next_review, created_at
		FROM kids_knowledge_cards
		WHERE id = $1
	`
	var card KnowledgeCard
	err := k.db.QueryRowContext(ctx, query, cardID).Scan(
		&card.ID, &card.ChildID, &card.Topic, &card.Content, &card.Category,
		&card.Mastery, &card.TimesAsked, &card.NextReview, &card.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (k *KidsService) linkRelatedCards(childID, cardID int64, topic, category string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Buscar cartas da mesma categoria
	query := `
		SELECT id FROM kids_knowledge_cards
		WHERE child_id = $1 AND category = $2 AND id != $3
		LIMIT 5
	`
	rows, err := k.db.QueryContext(ctx, query, childID, category, cardID)
	if err != nil {
		return
	}
	defer rows.Close()

	var relatedIDs []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			relatedIDs = append(relatedIDs, id)
		}
	}

	if len(relatedIDs) > 0 {
		idsJSON, _ := json.Marshal(relatedIDs)
		k.db.ExecContext(ctx, `
			UPDATE kids_knowledge_cards SET linked_cards = $1 WHERE id = $2
		`, string(idsJSON), cardID)
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func (k *KidsService) getMissionIcon(category string) string {
	icons := map[string]string{
		"hygiene": "🪥",
		"study":   "📚",
		"chores":  "🧹",
		"health":  "💪",
		"social":  "👋",
		"food":    "🍎",
		"sleep":   "😴",
	}
	if icon, ok := icons[category]; ok {
		return icon
	}
	return "⭐"
}

func (k *KidsService) getCelebrationMessage() string {
	messages := []string{
		"Incrível! Você é demais! 🎉",
		"Missão cumprida, campeão! 🏆",
		"Você está arrasando! 💪",
		"Que orgulho de você! ⭐",
		"Continue assim, herói! 🦸",
		"Fantástico! Mais uma vitória! 🎊",
		"Você é uma estrela! 🌟",
	}
	return messages[rand.Intn(len(messages))]
}

func (k *KidsService) haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	// Fórmula simplificada de Haversine
	const R = 6371000 // Raio da Terra em metros

	lat1Rad := lat1 * 3.14159 / 180
	lat2Rad := lat2 * 3.14159 / 180
	deltaLat := (lat2 - lat1) * 3.14159 / 180
	deltaLng := (lng2 - lng1) * 3.14159 / 180

	a := (1 - (deltaLat*deltaLat + deltaLng*deltaLng*lat1Rad*lat2Rad)) / 2
	if a < 0 {
		a = 0
	}

	return 2 * R * a // Aproximação
}

// ============================================================================
// TABELAS
// ============================================================================

func (k *KidsService) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS kids_profiles (
			id BIGINT PRIMARY KEY,
			name VARCHAR(100),
			age INT,
			parent_id BIGINT,
			total_points INT DEFAULT 0,
			current_level INT DEFAULT 1,
			current_xp INT DEFAULT 0,
			current_streak INT DEFAULT 0,
			avatar_url TEXT,
			preferences JSONB DEFAULT '{}',
			safe_contacts JSONB DEFAULT '[]',
			geofence_radius INT DEFAULT 0,
			geofence_center VARCHAR(50),
			blocked_apps JSONB DEFAULT '[]',
			study_schedule JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS kids_missions (
			id SERIAL PRIMARY KEY,
			child_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			category VARCHAR(50) DEFAULT 'general',
			points INT DEFAULT 10,
			xp INT DEFAULT 5,
			difficulty VARCHAR(20) DEFAULT 'easy',
			icon VARCHAR(10) DEFAULT '⭐',
			due_time VARCHAR(5),
			recurring BOOLEAN DEFAULT false,
			repeat_days JSONB DEFAULT '[]',
			status VARCHAR(20) DEFAULT 'pending',
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS kids_achievements (
			id SERIAL PRIMARY KEY,
			child_id BIGINT NOT NULL,
			code VARCHAR(50) NOT NULL,
			title VARCHAR(100) NOT NULL,
			description TEXT,
			icon VARCHAR(10),
			points INT DEFAULT 0,
			rarity VARCHAR(20) DEFAULT 'common',
			unlocked_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(child_id, code)
		)`,
		`CREATE TABLE IF NOT EXISTS kids_knowledge_cards (
			id SERIAL PRIMARY KEY,
			child_id BIGINT NOT NULL,
			topic VARCHAR(255) NOT NULL,
			content TEXT,
			category VARCHAR(50) DEFAULT 'general',
			image_url TEXT,
			linked_cards JSONB DEFAULT '[]',
			times_asked INT DEFAULT 0,
			mastery INT DEFAULT 0,
			next_review TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kids_missions_child ON kids_missions(child_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kids_missions_status ON kids_missions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_kids_knowledge_child ON kids_knowledge_cards(child_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kids_knowledge_review ON kids_knowledge_cards(next_review)`,
	}

	for _, q := range queries {
		if _, err := k.db.Exec(q); err != nil && !strings.Contains(err.Error(), "already exists") {
			log.Printf("⚠️ [KIDS] Erro SQL: %v", err)
		}
	}

	log.Println("✅ [KIDS] Tabelas verificadas/criadas")
	return nil
}
