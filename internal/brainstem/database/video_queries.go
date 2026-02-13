package database

import (
	"database/sql"
	"fmt"
	"time"
)

type VideoSession struct {
	ID        string
	SessionID string
	IdosoID   int64
	Status    string
	SdpOffer  string
	SdpAnswer sql.NullString
	CreatedAt time.Time
}

type SignalingMessage struct {
	ID        int64
	SessionID string
	Sender    string
	Type      string
	Payload   string // JSON
	CreatedAt time.Time
}

func (db *DB) CreateVideoSession(sessionID string, idosoID int64, sdpOffer string) error {
	query := `
		INSERT INTO video_sessions (session_id, idoso_id, status, sdp_offer, created_em)
		VALUES ($1, $2, 'waiting_operator', $3, CURRENT_TIMESTAMP)
	`
	// Usamos ExecContext para boas práticas, mas aqui com context.Background() se não vier de cima
	_, err := db.Conn.Exec(query, sessionID, idosoID, sdpOffer)
	if err != nil {
		return fmt.Errorf("failed to create video session: %w", err)
	}
	return nil
}

func (db *DB) CreateSignalingMessage(sessionID string, sender string, msgType string, payload string) error {
	query := `
		INSERT INTO signaling_messages (session_id, sender, type, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, err := db.Conn.Exec(query, sessionID, sender, msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to insert signaling message: %w", err)
	}
	return nil
}

func (db *DB) GetVideoSessionAnswer(sessionID string) (string, error) {
	query := `SELECT sdp_answer FROM video_sessions WHERE session_id = $1`

	var sdpAnswer sql.NullString
	err := db.Conn.QueryRow(query, sessionID).Scan(&sdpAnswer)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Não encontrou a sessão ou não tem answer ainda
		}
		return "", fmt.Errorf("failed to get session answer: %w", err)
	}

	if sdpAnswer.Valid {
		return sdpAnswer.String, nil
	}
	return "", nil
}

// Opcional: Pegar candidatos do Operador para o Mobile
func (db *DB) GetOperatorCandidates(sessionID string, sinceID int64) ([]SignalingMessage, error) {
	query := `
		SELECT id, session_id, sender, type, payload 
		FROM signaling_messages 
		WHERE session_id = $1 AND sender = 'operator' AND id > $2
		ORDER BY id ASC
	`

	rows, err := db.Conn.Query(query, sessionID, sinceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []SignalingMessage
	for rows.Next() {
		var m SignalingMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Sender, &m.Type, &m.Payload); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// Retorna apenas a session para o Operador pegar o Offer
func (db *DB) GetVideoSession(sessionID string) (*VideoSession, error) {
	query := `SELECT id, session_id, idoso_id, status, sdp_offer, sdp_answer, created_em FROM video_sessions WHERE session_id = $1`

	var s VideoSession
	err := db.Conn.QueryRow(query, sessionID).Scan(
		&s.ID, &s.SessionID, &s.IdosoID, &s.Status, &s.SdpOffer, &s.SdpAnswer, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Atualiza a resposta (Answer) do operador e muda status para active
func (db *DB) UpdateVideoSessionAnswer(sessionID string, sdpAnswer string) error {
	query := `
		UPDATE video_sessions 
		SET sdp_answer = $1, status = 'active' 
		WHERE session_id = $2
	`
	_, err := db.Conn.Exec(query, sdpAnswer, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update video session answer: %w", err)
	}
	return nil
}

// Pegar candidatos do Mobile para o Operador
func (db *DB) GetMobileCandidates(sessionID string, sinceID int64) ([]SignalingMessage, error) {
	query := `
		SELECT id, session_id, sender, type, payload 
		FROM signaling_messages 
		WHERE session_id = $1 AND sender = 'mobile' AND id > $2
		ORDER BY id ASC
	`

	rows, err := db.Conn.Query(query, sessionID, sinceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []SignalingMessage
	for rows.Next() {
		var m SignalingMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Sender, &m.Type, &m.Payload); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

type VideoSessionDetail struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	IdosoID   int64     `json:"idoso_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// Enriched fields from Idoso
	Nome           string         `json:"nome"`
	Idade          int            `json:"idade"` // Calculado ou aproximado (ano)
	Telefone       string         `json:"telefone"`
	NivelCognitivo string         `json:"nivel_cognitivo"`
	FotoUrl        string         `json:"foto_url"`   // Placeholder ou real if added later
	Limitacoes     sql.NullString `json:"limitacoes"` // Concat de audição etc
}

// Retorna todas as sessões aguardando atendimento COM DADOS DO IDOSO
func (db *DB) GetPendingVideoSessions() ([]VideoSessionDetail, error) {
	// JOIN com idosos para pegar detalhes
	query := `
		SELECT 
			vs.id, vs.session_id, vs.idoso_id, vs.status, vs.created_em,
			i.nome, i.data_nascimento, i.telefone, i.nivel_cognitivo,
			CASE 
				WHEN i.limitacoes_auditivas = true THEN 'Deficiência Auditiva'
				ELSE ''
			END as limitacoes
		FROM video_sessions vs
		JOIN idosos i ON vs.idoso_id = i.id
		WHERE vs.status = 'waiting_operator'
		ORDER BY vs.created_em DESC
	`

	rows, err := db.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []VideoSessionDetail
	for rows.Next() {
		var s VideoSessionDetail
		var dataNasc time.Time
		var limitacoes sql.NullString

		if err := rows.Scan(
			&s.ID, &s.SessionID, &s.IdosoID, &s.Status, &s.CreatedAt,
			&s.Nome, &dataNasc, &s.Telefone, &s.NivelCognitivo, &limitacoes,
		); err != nil {
			return nil, err
		}

		// Calcular idade simples
		s.Idade = time.Now().Year() - dataNasc.Year()
		s.Limitacoes = limitacoes

		sessions = append(sessions, s)
	}
	return sessions, nil
}
