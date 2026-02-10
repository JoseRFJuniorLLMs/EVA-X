package maps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Service struct {
	ctx    context.Context
	apiKey string
}

func NewService(ctx context.Context, apiKey string) *Service {
	return &Service{ctx: ctx, apiKey: apiKey}
}

// FindNearbyPlaces searches for nearby places using Google Places API
func (s *Service) FindNearbyPlaces(placeType, location string, radius int) ([]map[string]string, error) {
	baseURL := "https://maps.googleapis.com/maps/api/place/nearbysearch/json"

	params := url.Values{}
	params.Add("location", location) // Format: "lat,lng"
	params.Add("radius", fmt.Sprintf("%d", radius))
	params.Add("type", placeType) // pharmacy, hospital, etc.
	params.Add("key", s.apiKey)

	resp, err := http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("unable to search places: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Name     string  `json:"name"`
			Vicinity string  `json:"vicinity"`
			Rating   float64 `json:"rating"`
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unable to decode response: %v", err)
	}

	var places []map[string]string
	for _, place := range result.Results {
		places = append(places, map[string]string{
			"name":    place.Name,
			"address": place.Vicinity,
			"rating":  fmt.Sprintf("%.1f", place.Rating),
			"lat":     fmt.Sprintf("%.6f", place.Geometry.Location.Lat),
			"lng":     fmt.Sprintf("%.6f", place.Geometry.Location.Lng),
		})
	}

	return places, nil
}
