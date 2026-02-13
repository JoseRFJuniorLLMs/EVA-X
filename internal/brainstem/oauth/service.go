package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Service struct {
	config *oauth2.Config
}

// NewService creates OAuth service with Google configuration
func NewService(clientID, clientSecret, redirectURL string) *Service {
	return &Service{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/calendar",
				"https://www.googleapis.com/auth/gmail.send",
				"https://www.googleapis.com/auth/drive.file",
				"https://www.googleapis.com/auth/spreadsheets",
				"https://www.googleapis.com/auth/documents",
				"https://www.googleapis.com/auth/youtube.readonly",
				"https://www.googleapis.com/auth/userinfo.email",
			},
			Endpoint: google.Endpoint,
		},
	}
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
