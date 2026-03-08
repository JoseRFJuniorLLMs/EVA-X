// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"eva/internal/brainstem/database"

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

	if s.db == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	rows, err := s.db.QueryByLabel(ctx, "idosos",
		" AND n.cpf = $cpf AND n.ativo = $ativo",
		map[string]interface{}{
			"cpf":  cpf,
			"ativo": true,
		}, 1)
	if err != nil || len(rows) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"CPF not found"}`))
		return
	}

	m := rows[0]
	idoso := struct {
		ID             int64  `json:"id"`
		Nome           string `json:"nome"`
		CPF            string `json:"cpf,omitempty"`
		Telefone       string `json:"telefone,omitempty"`
		DataNascimento string `json:"data_nascimento,omitempty"`
		Endereco       string `json:"endereco,omitempty"`
		DeviceToken    string `json:"device_token,omitempty"`
		Ativo          bool   `json:"ativo"`
	}{
		ID:             database.GetInt64(m, "id"),
		Nome:           database.GetString(m, "nome"),
		CPF:            database.GetString(m, "cpf"),
		Telefone:       database.GetString(m, "telefone"),
		DataNascimento: database.GetString(m, "data_nascimento"),
		Endereco:       database.GetString(m, "endereco"),
		DeviceToken:    database.GetString(m, "device_token"),
		Ativo:          database.GetBool(m, "ativo"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idoso)
}

// GET /api/v1/idosos/{id}
func (s *SignalingServer) handleGetIdoso(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"Invalid ID"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	m, err := s.db.GetNodeByID(ctx, "idosos", id)
	if err != nil || m == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Idoso not found"}`))
		return
	}

	idoso := struct {
		ID             int64  `json:"id"`
		Nome           string `json:"nome"`
		CPF            string `json:"cpf,omitempty"`
		Telefone       string `json:"telefone,omitempty"`
		DataNascimento string `json:"data_nascimento,omitempty"`
		Endereco       string `json:"endereco,omitempty"`
		DeviceToken    string `json:"device_token,omitempty"`
		Ativo          bool   `json:"ativo"`
	}{
		ID:             database.GetInt64(m, "id"),
		Nome:           database.GetString(m, "nome"),
		CPF:            database.GetString(m, "cpf"),
		Telefone:       database.GetString(m, "telefone"),
		DataNascimento: database.GetString(m, "data_nascimento"),
		Endereco:       database.GetString(m, "endereco"),
		DeviceToken:    database.GetString(m, "device_token"),
		Ativo:          database.GetBool(m, "ativo"),
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

	if s.db == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	// Find the idoso by CPF first
	rows, err := s.db.QueryByLabel(ctx, "idosos",
		" AND n.cpf = $cpf AND n.ativo = $ativo",
		map[string]interface{}{
			"cpf":  cpf,
			"ativo": true,
		}, 1)
	if err != nil || len(rows) == 0 {
		http.Error(w, `{"error":"CPF not found"}`, http.StatusNotFound)
		return
	}

	// Update token via NietzscheDB
	err = s.db.Update(ctx, "idosos",
		map[string]interface{}{"cpf": cpf},
		map[string]interface{}{
			"device_token":               token,
			"device_token_valido":        true,
			"device_token_atualizado_em": time.Now().Format(time.RFC3339),
		})
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}
