package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// GET /api/v1/idosos/by-cpf/{cpf}
func (s *SignalingServer) handleGetIdosoByCpf(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cpf := vars["cpf"]

	if cpf == "" {
		http.Error(w, `{"error":"CPF is required"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil || s.db.Conn == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	row := s.db.Conn.QueryRow(`
		SELECT id, nome, cpf, telefone, data_nascimento, endereco, device_token, ativo
		FROM idosos WHERE cpf = $1 AND ativo = true
	`, cpf)

	var idoso struct {
		ID             int     `json:"id"`
		Nome           string  `json:"nome"`
		CPF            *string `json:"cpf"`
		Telefone       *string `json:"telefone"`
		DataNascimento *string `json:"data_nascimento"`
		Endereco       *string `json:"endereco"`
		DeviceToken    *string `json:"device_token"`
		Ativo          bool    `json:"ativo"`
	}

	err := row.Scan(&idoso.ID, &idoso.Nome, &idoso.CPF, &idoso.Telefone,
		&idoso.DataNascimento, &idoso.Endereco, &idoso.DeviceToken, &idoso.Ativo)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"CPF not found"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idoso)
}

// GET /api/v1/idosos/{id}
func (s *SignalingServer) handleGetIdoso(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"Invalid ID"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil || s.db.Conn == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	row := s.db.Conn.QueryRow(`
		SELECT id, nome, cpf, telefone, data_nascimento, endereco, device_token, ativo
		FROM idosos WHERE id = $1
	`, id)

	var idoso struct {
		ID             int     `json:"id"`
		Nome           string  `json:"nome"`
		CPF            *string `json:"cpf"`
		Telefone       *string `json:"telefone"`
		DataNascimento *string `json:"data_nascimento"`
		Endereco       *string `json:"endereco"`
		DeviceToken    *string `json:"device_token"`
		Ativo          bool    `json:"ativo"`
	}

	err = row.Scan(&idoso.ID, &idoso.Nome, &idoso.CPF, &idoso.Telefone,
		&idoso.DataNascimento, &idoso.Endereco, &idoso.DeviceToken, &idoso.Ativo)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Idoso not found"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idoso)
}

// PATCH /api/v1/idosos/sync-token-by-cpf?cpf=XXX&token=YYY
func (s *SignalingServer) handleSyncTokenByCpf(w http.ResponseWriter, r *http.Request) {
	cpf := r.URL.Query().Get("cpf")
	token := r.URL.Query().Get("token")

	if cpf == "" || token == "" {
		http.Error(w, `{"error":"cpf and token are required"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil || s.db.Conn == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	result, err := s.db.Conn.Exec(`
		UPDATE idosos SET device_token = $1, device_token_valido = true,
		device_token_atualizado_em = NOW() WHERE cpf = $2 AND ativo = true
	`, token, cpf)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, `{"error":"CPF not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}
