package uber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Service struct {
	ctx         context.Context
	serverToken string
}

func NewService(ctx context.Context, serverToken string) *Service {
	return &Service{ctx: ctx, serverToken: serverToken}
}

// EstimatePrice estimates ride price
func (s *Service) EstimatePrice(startLat, startLng, endLat, endLng float64) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.uber.com/v1.2/estimates/price?start_latitude=%.6f&start_longitude=%.6f&end_latitude=%.6f&end_longitude=%.6f",
		startLat, startLng, endLat, endLng)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+s.serverToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("estimate failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Prices []struct {
			DisplayName string  `json:"display_name"`
			Estimate    string  `json:"estimate"`
			Duration    int     `json:"duration"`
			Distance    float64 `json:"distance"`
		} `json:"prices"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Prices) == 0 {
		return nil, fmt.Errorf("no rides available")
	}

	return map[string]interface{}{
		"ride_type": result.Prices[0].DisplayName,
		"estimate":  result.Prices[0].Estimate,
		"duration":  result.Prices[0].Duration,
		"distance":  result.Prices[0].Distance,
	}, nil
}

// RequestRide requests an Uber ride (requires user OAuth token)
func (s *Service) RequestRide(accessToken string, startLat, startLng, endLat, endLng float64, productID string) (string, error) {
	payload := map[string]interface{}{
		"start_latitude":  startLat,
		"start_longitude": startLng,
		"end_latitude":    endLat,
		"end_longitude":   endLng,
		"product_id":      productID,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.uber.com/v1.2/requests", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
	}

	json.NewDecoder(resp.Body).Decode(&result)
	return result.RequestID, nil
}
