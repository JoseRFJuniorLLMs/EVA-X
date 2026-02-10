package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Service struct {
	ctx context.Context
}

func NewService(ctx context.Context) *Service {
	return &Service{ctx: ctx}
}

// SearchTracks searches for tracks on Spotify
func (s *Service) SearchTracks(accessToken, query string, limit int) ([]map[string]string, error) {
	baseURL := "https://api.spotify.com/v1/search"
	params := url.Values{}
	params.Add("q", query)
	params.Add("type", "track")
	params.Add("limit", fmt.Sprintf("%d", limit))

	req, _ := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Tracks struct {
			Items []struct {
				Name    string `json:"name"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				URI string `json:"uri"`
			} `json:"items"`
		} `json:"tracks"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	var tracks []map[string]string
	for _, item := range result.Tracks.Items {
		artistNames := []string{}
		for _, artist := range item.Artists {
			artistNames = append(artistNames, artist.Name)
		}
		tracks = append(tracks, map[string]string{
			"name":   item.Name,
			"artist": strings.Join(artistNames, ", "),
			"uri":    item.URI,
		})
	}

	return tracks, nil
}

// PlayTrack plays a track on user's active device
func (s *Service) PlayTrack(accessToken, trackURI string) error {
	req, _ := http.NewRequest("PUT", "https://api.spotify.com/v1/me/player/play",
		strings.NewReader(fmt.Sprintf(`{"uris":["%s"]}`, trackURI)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("play failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("play failed with status: %d", resp.StatusCode)
	}

	return nil
}
