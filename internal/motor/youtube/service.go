package youtube

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// SearchVideos searches for YouTube videos
func (s *Service) SearchVideos(accessToken, query string, maxResults int64) ([]map[string]string, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	})

	srv, err := youtube.NewService(s.ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("unable to create youtube client: %v", err)
	}

	call := srv.Search.List([]string{"snippet"}).
		Q(query).
		MaxResults(maxResults).
		Type("video")

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to search videos: %v", err)
	}

	var videos []map[string]string
	for _, item := range response.Items {
		videos = append(videos, map[string]string{
			"title":       item.Snippet.Title,
			"video_id":    item.Id.VideoId,
			"url":         fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.Id.VideoId),
			"description": item.Snippet.Description,
		})
	}

	return videos, nil
}
