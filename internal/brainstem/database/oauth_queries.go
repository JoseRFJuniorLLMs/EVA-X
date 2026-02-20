// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"database/sql"
	"fmt"
	"time"
)

// SaveGoogleTokens stores OAuth tokens for an idoso
func (db *DB) SaveGoogleTokens(idosoID int64, refreshToken, accessToken string, expiry time.Time) error {
	query := `
		UPDATE idosos 
		SET google_refresh_token = $1, 
		    google_access_token = $2, 
		    google_token_expiry = $3
		WHERE id = $4
	`
	_, err := db.Conn.Exec(query, refreshToken, accessToken, expiry, idosoID)
	if err != nil {
		return fmt.Errorf("failed to save google tokens: %w", err)
	}
	return nil
}

// GetGoogleTokens retrieves OAuth tokens for an idoso
func (db *DB) GetGoogleTokens(idosoID int64) (refreshToken, accessToken string, expiry time.Time, err error) {
	query := `
		SELECT google_refresh_token, google_access_token, google_token_expiry
		FROM idosos
		WHERE id = $1
	`
	var rt, at sql.NullString
	var exp sql.NullTime

	err = db.Conn.QueryRow(query, idosoID).Scan(&rt, &at, &exp)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to get google tokens: %w", err)
	}

	if rt.Valid {
		refreshToken = rt.String
	}
	if at.Valid {
		accessToken = at.String
	}
	if exp.Valid {
		expiry = exp.Time
	}

	return refreshToken, accessToken, expiry, nil
}

// SaveGoogleEmail stores the Google email for an idoso
func (db *DB) SaveGoogleEmail(idosoID int64, email string) error {
	query := `UPDATE idosos SET google_email = $1 WHERE id = $2`
	_, err := db.Conn.Exec(query, email, idosoID)
	if err != nil {
		return fmt.Errorf("failed to save google email: %w", err)
	}
	return nil
}

// GoogleStatus represents Google account connection status
type GoogleStatus struct {
	Connected bool   `json:"connected"`
	Email     string `json:"email"`
}

// GetGoogleStatusByCPF returns Google account connection status for a CPF
func (db *DB) GetGoogleStatusByCPF(cpf string) (*GoogleStatus, error) {
	query := `
		SELECT COALESCE(google_email, '') AS email,
		       CASE WHEN google_refresh_token IS NOT NULL AND google_refresh_token != '' THEN true ELSE false END AS connected
		FROM idosos
		WHERE cpf = $1
		LIMIT 1
	`
	var status GoogleStatus
	err := db.Conn.QueryRow(query, cpf).Scan(&status.Email, &status.Connected)
	if err != nil {
		return nil, fmt.Errorf("failed to get google status: %w", err)
	}
	return &status, nil
}

// ClearGoogleTokens removes Google OAuth tokens for an idoso
func (db *DB) ClearGoogleTokens(idosoID int64) error {
	query := `
		UPDATE idosos
		SET google_refresh_token = NULL,
		    google_access_token = NULL,
		    google_token_expiry = NULL,
		    google_email = NULL
		WHERE id = $1
	`
	_, err := db.Conn.Exec(query, idosoID)
	if err != nil {
		return fmt.Errorf("failed to clear google tokens: %w", err)
	}
	return nil
}
