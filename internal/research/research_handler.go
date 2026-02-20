// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package research

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes registra rotas HTTP para o Research Engine
func RegisterRoutes(router *mux.Router, engine *ResearchEngine) {
	r := router.PathPrefix("/research").Subrouter()

	r.HandleFunc("/cohorts", createCohortHandler(engine)).Methods("POST")
	r.HandleFunc("/cohorts/{id}", getCohortHandler(engine)).Methods("GET")
	r.HandleFunc("/cohorts/{id}/report", generateReportHandler(engine)).Methods("GET")
	r.HandleFunc("/cohorts/{id}/export", exportDatasetHandler(engine)).Methods("POST")
}

func createCohortHandler(engine *ResearchEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cohort ResearchCohort
		if err := json.NewDecoder(r.Body).Decode(&cohort); err != nil {
			writeResearchErr(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
			return
		}
		if err := engine.CreateCohort(&cohort); err != nil {
			writeResearchErr(w, http.StatusInternalServerError, "Falha ao criar coorte: "+err.Error())
			return
		}
		writeResearchJSON(w, http.StatusCreated, cohort)
	}
}

func getCohortHandler(engine *ResearchEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		cohort, err := engine.GetCohort(id)
		if err != nil {
			writeResearchErr(w, http.StatusNotFound, "Coorte nao encontrada: "+err.Error())
			return
		}
		writeResearchJSON(w, http.StatusOK, cohort)
	}
}

func generateReportHandler(engine *ResearchEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		report, err := engine.GenerateStudyReport(id)
		if err != nil {
			writeResearchErr(w, http.StatusInternalServerError, "Falha ao gerar relatorio: "+err.Error())
			return
		}
		writeResearchJSON(w, http.StatusOK, report)
	}
}

func exportDatasetHandler(engine *ResearchEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		var req struct {
			FilePath string `json:"file_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeResearchErr(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
			return
		}
		if req.FilePath == "" {
			req.FilePath = "/tmp/eva_research_" + id + ".csv"
		}
		if err := engine.ExportDatasetToCSV(id, req.FilePath); err != nil {
			writeResearchErr(w, http.StatusInternalServerError, "Falha ao exportar: "+err.Error())
			return
		}
		writeResearchJSON(w, http.StatusOK, map[string]string{
			"status":    "exported",
			"file_path": req.FilePath,
		})
	}
}

func writeResearchJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeResearchErr(w http.ResponseWriter, status int, msg string) {
	writeResearchJSON(w, status, map[string]string{"error": msg})
}
