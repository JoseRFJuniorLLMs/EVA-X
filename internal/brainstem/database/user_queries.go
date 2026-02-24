// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
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

func contentToUser(m map[string]interface{}) *User {
	return &User{
		ID:           getInt64(m, "id"),
		Name:         getString(m, "nome"),
		Email:        getString(m, "email"),
		PasswordHash: getString(m, "senha_hash"),
		Role:         getString(m, "tipo"),
		LastLogin:    getTimePtr(m, "last_login"),
		CreatedAt:    getTime(m, "criado_em"),
		UpdatedAt:    getTime(m, "atualizado_em"),
	}
}

func (db *DB) CreateUser(name, email, passwordHash, role string) error {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	_, err := db.insertRow(ctx, "usuarios", map[string]interface{}{
		"nome":          name,
		"email":         email,
		"senha_hash":    passwordHash,
		"tipo":          role,
		"criado_em":     now,
		"atualizado_em": now,
		"ativo":         true,
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (db *DB) GetUserByEmail(email string) (*User, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "usuarios",
		` AND n.email = $email`, map[string]interface{}{
			"email": email,
		}, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil // Not found
	}
	return contentToUser(rows[0]), nil
}

func (db *DB) GetUserByID(id int64) (*User, error) {
	ctx := context.Background()
	m, err := db.getNode(ctx, "usuarios", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if m == nil {
		return nil, nil // Not found
	}
	return contentToUser(m), nil
}

func (db *DB) UpdateLastLogin(userID int64) error {
	// No-op (column not confirmed in schema)
	return nil
}
