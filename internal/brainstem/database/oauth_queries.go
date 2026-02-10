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
