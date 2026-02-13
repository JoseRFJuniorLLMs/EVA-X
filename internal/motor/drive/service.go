package drive

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// SaveFile uploads a text file to Google Drive
func (s *Service) SaveFile(accessToken, filename, content, folderName string) (string, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := drive.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("unable to create drive client: %v", err)
	}

	// Create or find folder
	var parentID string
	if folderName != "" {
		parentID, err = s.findOrCreateFolder(srv, folderName)
		if err != nil {
			return "", err
		}
	}

	// Create file metadata
	file := &drive.File{
		Name:     filename,
		MimeType: "text/plain",
	}
	if parentID != "" {
		file.Parents = []string{parentID}
	}

	// Upload file
	reader := strings.NewReader(content)
	createdFile, err := srv.Files.Create(file).Media(reader).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create file: %v", err)
	}

	return createdFile.Id, nil
}

func (s *Service) findOrCreateFolder(srv *drive.Service, folderName string) (string, error) {
	// Search for existing folder
	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", folderName)
	fileList, err := srv.Files.List().Q(query).Spaces("drive").Do()
	if err != nil {
		return "", err
	}

	if len(fileList.Files) > 0 {
		return fileList.Files[0].Id, nil
	}

	// Create new folder
	folder := &drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
	}
	createdFolder, err := srv.Files.Create(folder).Do()
	if err != nil {
		return "", err
	}

	return createdFolder.Id, nil
}
