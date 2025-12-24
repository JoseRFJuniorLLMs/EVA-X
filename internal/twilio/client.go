package twilio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"eva-mind/internal/config"
)

type Client struct {
	cfg *config.Config
}

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) CreateCall(to string, twimlURL string) (string, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json", c.cfg.TwilioAccountSID)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", c.cfg.TwilioPhoneNumber)
	data.Set("Url", twimlURL)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(c.cfg.TwilioAccountSID, c.cfg.TwilioAuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errData map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errData)
		return "", fmt.Errorf("twilio error: %v (status %d)", errData["message"], resp.StatusCode)
	}

	var result struct {
		Sid string `json:"sid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Sid, nil
}
