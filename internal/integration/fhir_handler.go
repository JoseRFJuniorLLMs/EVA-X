// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"eva/internal/brainstem/database"

	"github.com/gorilla/mux"
)

// ============================================================================
// FHIR R4 HTTP HANDLERS
// ============================================================================
// Endpoints REST que expõem recursos EVA-Mind no formato HL7 FHIR R4.
// Referência: https://www.hl7.org/fhir/R4/

// FHIRHandler handles FHIR R4 endpoints
type FHIRHandler struct {
	db *database.DB
}

// NewFHIRHandler creates a new handler with a NietzscheDB connection
func NewFHIRHandler(db *database.DB) *FHIRHandler {
	if db == nil {
		log.Printf("⚠️ [FHIR] NietzscheDB unavailable — running in degraded mode")
	}
	return &FHIRHandler{db: db}
}

// ============================================================================
// GET /api/v1/fhir/Patient/{id}
// ============================================================================
// Returns a single patient resource in FHIR R4 Patient format.

func (h *FHIRHandler) GetPatient(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeFHIRError(w, http.StatusServiceUnavailable, "unavailable", "NietzscheDB unavailable")
		return
	}
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeFHIRError(w, http.StatusBadRequest, "invalid-id", "Patient ID must be a valid integer")
		return
	}

	patient, err := h.queryPatientDTO(id)
	if err != nil {
		writeFHIRError(w, http.StatusInternalServerError, "database-error", "Failed to retrieve patient data")
		return
	}
	if patient == nil {
		writeFHIRError(w, http.StatusNotFound, "not-found", fmt.Sprintf("Patient/%d not found", id))
		return
	}

	fhirPatient := PatientDTOToFHIR(patient)

	writeFHIRJSON(w, http.StatusOK, fhirPatient)
}

// ============================================================================
// GET /api/v1/fhir/Patient/{id}/$everything
// ============================================================================
// Returns a FHIR Bundle containing the patient resource and all related
// observations (clinical assessments converted to FHIR Observation).

func (h *FHIRHandler) GetPatientBundle(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeFHIRError(w, http.StatusServiceUnavailable, "unavailable", "NietzscheDB unavailable")
		return
	}
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeFHIRError(w, http.StatusBadRequest, "invalid-id", "Patient ID must be a valid integer")
		return
	}

	patient, err := h.queryPatientDTO(id)
	if err != nil {
		writeFHIRError(w, http.StatusInternalServerError, "database-error", "Failed to retrieve patient data")
		return
	}
	if patient == nil {
		writeFHIRError(w, http.StatusNotFound, "not-found", fmt.Sprintf("Patient/%d not found", id))
		return
	}

	assessments, err := h.queryAssessmentDTOs(id)
	if err != nil {
		writeFHIRError(w, http.StatusInternalServerError, "database-error", "Failed to retrieve assessment data")
		return
	}

	bundle, err := ExportPatientAsFHIRBundle(patient, assessments)
	if err != nil {
		writeFHIRError(w, http.StatusInternalServerError, "conversion-error", "Failed to build FHIR Bundle")
		return
	}

	// Set bundle ID and self-link
	bundle.ID = fmt.Sprintf("patient-%d-bundle", id)
	bundle.Meta = &FHIRMeta{
		LastUpdated: time.Now(),
		Source:      "EVA-Mind",
	}
	bundle.Link = []FHIRBundleLink{
		{
			Relation: "self",
			URL:      fmt.Sprintf("/api/v1/fhir/Patient/%d/$everything", id),
		},
	}

	writeFHIRJSON(w, http.StatusOK, bundle)
}

// ============================================================================
// ROUTE REGISTRATION
// ============================================================================

// RegisterFHIRRoutes registers all FHIR R4 endpoints on a gorilla/mux router.
func RegisterFHIRRoutes(router *mux.Router, handler *FHIRHandler) {
	fhir := router.PathPrefix("/api/v1/fhir").Subrouter()

	fhir.HandleFunc("/Patient/{id}", handler.GetPatient).Methods("GET")
	fhir.HandleFunc("/Patient/{id}/$everything", handler.GetPatientBundle).Methods("GET")
}

// ============================================================================
// DATABASE QUERIES (private)
// ============================================================================

// queryPatientDTO loads a patient from the idosos table via NietzscheDB
// and maps it to a PatientDTO. Returns (nil, nil) when not found.
func (h *FHIRHandler) queryPatientDTO(id int64) (*PatientDTO, error) {
	ctx := context.Background()

	m, err := h.db.GetNodeByID(ctx, "idosos", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query patient %d: %w", id, err)
	}
	if m == nil {
		return nil, nil
	}

	// Compute date of birth string and age from data_nascimento
	dob := database.GetTime(m, "data_nascimento")
	var dobStr string
	var age int
	if !dob.IsZero() {
		dobStr = dob.Format("2006-01-02")
		age = int(time.Since(dob).Hours() / 24 / 365.25)
	}

	dto := &PatientDTO{
		ID:          database.GetInt64(m, "id"),
		Name:        database.GetString(m, "nome"),
		DateOfBirth: dobStr,
		Age:         age,
		Gender:      database.GetString(m, "sexo"),
		CreatedAt:   database.GetTime(m, "criado_em"),
		UpdatedAt:   database.GetTime(m, "atualizado_em"),
	}

	// Handle nullable fields
	phoneNS := database.GetNullString(m, "telefone")
	if phoneNS.Valid {
		dto.Phone = &phoneNS.String
	}

	addressNS := database.GetNullString(m, "endereco")
	if addressNS.Valid {
		dto.Address = &addressNS.String
	}

	// Default CreatedAt/UpdatedAt to now if missing
	if dto.CreatedAt.IsZero() {
		dto.CreatedAt = time.Now()
	}
	if dto.UpdatedAt.IsZero() {
		dto.UpdatedAt = time.Now()
	}

	return dto, nil
}

// queryAssessmentDTOs loads all clinical assessments for a patient from
// NietzscheDB and maps them to AssessmentDTO slices suitable for FHIR conversion.
func (h *FHIRHandler) queryAssessmentDTOs(patientID int64) ([]*AssessmentDTO, error) {
	ctx := context.Background()

	rows, err := h.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{
			"patient_id": patientID,
		}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query assessments: %w", err)
	}

	var assessments []*AssessmentDTO
	for _, m := range rows {
		dto := &AssessmentDTO{}

		// Extract ID — may be stored as string, int, or float
		idRaw := database.GetString(m, "id")
		if idRaw == "" || idRaw == "0" {
			idRaw = fmt.Sprintf("%v", m["id"])
		}
		dto.ID = idRaw

		dto.PatientID = database.GetInt64(m, "patient_id")
		dto.AssessmentType = database.GetString(m, "assessment_type")
		dto.Status = database.GetString(m, "status")
		dto.CreatedAt = database.GetTime(m, "created_at")

		// Handle nullable total_score
		if scoreVal, ok := m["total_score"]; ok && scoreVal != nil {
			score := int(database.GetInt64(m, "total_score"))
			dto.TotalScore = &score
		}

		// Handle nullable severity_level
		sevNS := database.GetNullString(m, "severity_level")
		if sevNS.Valid {
			dto.Severity = &sevNS.String
		}

		// Handle nullable clinical_interpretation -> Notes
		interpNS := database.GetNullString(m, "clinical_interpretation")
		if interpNS.Valid {
			dto.Notes = &interpNS.String
		}

		// Handle nullable completed_at
		completedAt := database.GetTimePtr(m, "completed_at")
		if completedAt != nil {
			dto.CompletedAt = completedAt
		}

		dto.AdministeredBy = "eva"

		assessments = append(assessments, dto)
	}

	// Sort by created_at DESC (most recent first), matching the original ORDER BY
	sort.Slice(assessments, func(i, j int) bool {
		return assessments[i].CreatedAt.After(assessments[j].CreatedAt)
	})

	return assessments, nil
}

// ============================================================================
// HTTP RESPONSE HELPERS (private)
// ============================================================================

// writeFHIRJSON encodes a FHIR resource as JSON and writes it to the response
// with the standard FHIR content type.
func writeFHIRJSON(w http.ResponseWriter, statusCode int, resource interface{}) {
	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resource)
}

// writeFHIRError writes a FHIR-style OperationOutcome error response.
// See: https://www.hl7.org/fhir/R4/operationoutcome.html
func writeFHIRError(w http.ResponseWriter, statusCode int, code string, diagnostics string) {
	outcome := map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{
			{
				"severity":    "error",
				"code":        code,
				"diagnostics": diagnostics,
			},
		},
	}
	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(outcome)
}
