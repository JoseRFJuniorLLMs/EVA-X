package docs

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// CreateDocument creates a new Google Doc
func (s *Service) CreateDocument(accessToken, title, content string) (string, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := docs.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("unable to create docs client: %v", err)
	}

	doc := &docs.Document{
		Title: title,
	}

	createdDoc, err := srv.Documents.Create(doc).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create document: %v", err)
	}

	// Insert content
	requests := []*docs.Request{
		{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{
					Index: 1,
				},
				Text: content,
			},
		},
	}

	_, err = srv.Documents.BatchUpdate(createdDoc.DocumentId, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Do()
	if err != nil {
		return "", fmt.Errorf("unable to insert content: %v", err)
	}

	return fmt.Sprintf("https://docs.google.com/document/d/%s/edit", createdDoc.DocumentId), nil
}
