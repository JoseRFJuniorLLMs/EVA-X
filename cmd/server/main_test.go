package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"eva-mind/internal/config"
	"eva-mind/internal/database"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func TestTwiMLFlow(t *testing.T) {
	// 1. Setup Environment
	godotenv.Load("../../.env")
	cfg := config.Load()
	logger := zerolog.New(os.Stdout)

	// Skip if no DATABASE_URL (CI/Local without DB)
	if cfg.DatabaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Setup Router
	router := SetupRouter(db, cfg, logger)

	// 3. Test TwiML Endpoint
	// Simulates Twilio calling our endpoint with an agendamento_id
	req, _ := http.NewRequest("POST", "/calls/twiml?agendamento_id=123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 4. Validations
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/xml") {
		t.Errorf("Expected Content-Type text/xml, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<Connect>") || !strings.Contains(body, "<Stream") {
		t.Errorf("Response body does not contain expected TwiML tags: %s", body)
	}

	if !strings.Contains(body, "calls/stream/123") {
		t.Errorf("Response body does not contain correct stream URL: %s", body)
	}
}

func TestHealthCheck(t *testing.T) {
	godotenv.Load("../../.env")
	cfg := config.Load()
	logger := zerolog.New(os.Stdout)

	if cfg.DatabaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping health test")
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	router := SetupRouter(db, cfg, logger)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
