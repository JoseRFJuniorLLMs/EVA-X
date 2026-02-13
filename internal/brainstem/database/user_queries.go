package database

import (
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never send password hash in JSON
	Role         string     `json:"role"`
	LastLogin    *time.Time `json:"last_login"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (db *DB) CreateUser(name, email, passwordHash, role string) error {
	query := `
		INSERT INTO usuarios (nome, email, senha_hash, tipo, criado_em, atualizado_em, ativo)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, true)
	`
	_, err := db.Conn.Exec(query, name, email, passwordHash, role)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (db *DB) GetUserByEmail(email string) (*User, error) {
	query := `
		SELECT id, nome, email, senha_hash, tipo, NULL as last_login, criado_em, atualizado_em
		FROM usuarios
		WHERE email = $1
	`
	// Note: last_login might not exist in usuarios yet, passing NULL for now or we need to add it to schema
	var u User
	err := db.Conn.QueryRow(query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &u, nil
}

func (db *DB) GetUserByID(id int64) (*User, error) {
	query := `
		SELECT id, nome, email, senha_hash, tipo, NULL as last_login, criado_em, atualizado_em
		FROM usuarios
		WHERE id = $1
	`
	var u User
	err := db.Conn.QueryRow(query, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &u, nil
}

func (db *DB) UpdateLastLogin(userID int64) error {
	// Assuming there is no last_login column in usuarios yet based on the request which didn't list it.
	// We will skip this or commented it out until confirmed.
	// If we must implement, we might need to add it or use updated_at.
	// query := `UPDATE usuarios SET atualizado_em = CURRENT_TIMESTAMP WHERE id = $1`
	// _, err := db.conn.Exec(query, userID)
	return nil
}
