// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"eva/pkg/crypto"
	"fmt"
	"time"
)

// SaveGoogleTokens stores OAuth tokens for an idoso
func (db *DB) SaveGoogleTokens(idosoID int64, refreshToken, accessToken string, expiry time.Time) error {
	ctx := context.Background()
	// LGPD Art. 46: encrypt tokens at rest
	err := db.updateFields(ctx, "idosos",
		map[string]interface{}{"id": float64(idosoID)},
		map[string]interface{}{
			"google_refresh_token": crypto.Encrypt(refreshToken),
			"google_access_token":  crypto.Encrypt(accessToken),
			"google_token_expiry":  expiry.Format(time.RFC3339),
		})
	if err != nil {
		return fmt.Errorf("failed to save google tokens: %w", err)
	}
	return nil
}

// GetGoogleTokens retrieves OAuth tokens for an idoso
func (db *DB) GetGoogleTokens(idosoID int64) (refreshToken, accessToken string, expiry time.Time, err error) {
	ctx := context.Background()

	m, err := db.getNode(ctx, "idosos", idosoID)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to get google tokens: %w", err)
	}
	if m == nil {
		return "", "", time.Time{}, fmt.Errorf("failed to get google tokens: idoso not found")
	}

	rt := getString(m, "google_refresh_token")
	at := getString(m, "google_access_token")
	exp := getTime(m, "google_token_expiry")

	if rt != "" {
		refreshToken = crypto.Decrypt(rt)
	}
	if at != "" {
		accessToken = crypto.Decrypt(at)
	}
	expiry = exp

	return refreshToken, accessToken, expiry, nil
}

// SaveGoogleEmail stores the Google email for an idoso
func (db *DB) SaveGoogleEmail(idosoID int64, email string) error {
	ctx := context.Background()
	// LGPD Art. 46: encrypt email at rest
	err := db.updateFields(ctx, "idosos",
		map[string]interface{}{"id": float64(idosoID)},
		map[string]interface{}{
			"google_email": crypto.Encrypt(email),
		})
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
	ctx := context.Background()
	cpfHash := crypto.HashCPF(cpf)

	rows, err := db.queryNodesByLabel(ctx, "idosos",
		` AND n.cpf_hash = $hash`, map[string]interface{}{
			"hash": cpfHash,
		}, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get google status: %w", err)
	}

	// Fallback to stripped CPF match
	if len(rows) == 0 {
		allRows, err := db.queryNodesByLabel(ctx, "idosos", "", nil, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to get google status (fallback): %w", err)
		}
		strippedCPF := stripNonDigits(cpf)
		for _, m := range allRows {
			storedCPF := crypto.Decrypt(getString(m, "cpf"))
			if stripNonDigits(storedCPF) == strippedCPF {
				rows = append(rows, m)
				break
			}
		}
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("failed to get google status: idoso not found")
	}

	m := rows[0]
	email := getString(m, "google_email")
	refreshToken := getString(m, "google_refresh_token")

	status := &GoogleStatus{
		Connected: refreshToken != "",
		Email:     crypto.Decrypt(email),
	}

	return status, nil
}

// ClearGoogleTokens removes Google OAuth tokens for an idoso
func (db *DB) ClearGoogleTokens(idosoID int64) error {
	ctx := context.Background()
	err := db.updateFields(ctx, "idosos",
		map[string]interface{}{"id": float64(idosoID)},
		map[string]interface{}{
			"google_refresh_token": nil,
			"google_access_token":  nil,
			"google_token_expiry":  nil,
			"google_email":         nil,
		})
	if err != nil {
		return fmt.Errorf("failed to clear google tokens: %w", err)
	}
	return nil
}
