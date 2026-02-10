package gmail

import (
	"context"
	"encoding/base64"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// SendEmail sends an email using user's OAuth token
func (s *Service) SendEmail(accessToken, to, subject, body string) error {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := gmail.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("unable to create gmail client: %v", err)
	}

	// Create email message
	emailContent := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	message := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(emailContent)),
	}

	_, err = srv.Users.Messages.Send("me", message).Do()
	if err != nil {
		return fmt.Errorf("unable to send email: %v", err)
	}

	return nil
}
