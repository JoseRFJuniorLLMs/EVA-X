// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Service struct {
	config  *oauth2.Config
	hmacKey []byte
}

// NewService creates OAuth service with Google configuration and HMAC state signing
func NewService(clientID, clientSecret, redirectURL, hmacSecret string) *Service {
	return &Service{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://mail.google.com/",
				"https://www.googleapis.com/auth/gmail.readonly",
				"https://www.googleapis.com/auth/calendar",
				"https://www.googleapis.com/auth/drive",
				"https://www.googleapis.com/auth/spreadsheets",
				"https://www.googleapis.com/auth/documents",
				"https://www.googleapis.com/auth/youtube.readonly",
				"https://www.googleapis.com/auth/contacts.readonly",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
		hmacKey: []byte(hmacSecret),
	}
}

// SignState creates an HMAC-signed state parameter with embedded CPF
func (s *Service) SignState(cpf string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	payload := cpf + "|" + ts
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "|" + sig
}

// VerifyState verifies HMAC-signed state and returns the embedded CPF
func (s *Service) VerifyState(state string) (string, error) {
	parts := strings.SplitN(state, "|", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid state format")
	}
	cpf, ts, sig := parts[0], parts[1], parts[2]

	// Verify HMAC signature
	payload := cpf + "|" + ts
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", fmt.Errorf("invalid state signature")
	}

	// Check expiry (10 minutes)
	var timestamp int64
	fmt.Sscanf(ts, "%d", &timestamp)
	if time.Since(time.Unix(timestamp, 0)) > 10*time.Minute {
		return "", fmt.Errorf("state expired")
	}

	return cpf, nil
}

// GetAuthURL generates the Google OAuth authorization URL
func (s *Service) GetAuthURL(state string) string {
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges authorization code for tokens
func (s *Service) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// RefreshToken refreshes an expired access token
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := s.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newToken, nil
}

// GetTokenSource creates a token source for API calls
func (s *Service) GetTokenSource(ctx context.Context, accessToken, refreshToken string, expiry time.Time) oauth2.TokenSource {
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
	}
	return s.config.TokenSource(ctx, token)
}

// GetUserInfo retrieves user email from Google
func (s *Service) GetUserInfo(ctx context.Context, accessToken string) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Email, nil
}
