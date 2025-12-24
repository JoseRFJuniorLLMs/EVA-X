package api

import (
	"net/http"

	"eva-mind/internal/database"
)

// handleHealth verifica se o banco de dados está saudável
func handleHealth(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(); err != nil {
			http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","service":"eva-mind"}`))
	}
}

// handleLiveness retorna apenas se o serviço está no ar
func handleLiveness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"alive"}`))
	}
}

// handleReadiness verifica se o serviço está pronto para receber tráfego
func handleReadiness(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(); err != nil {
			http.Error(w, "Not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"status":"ready"}`))
	}
}
