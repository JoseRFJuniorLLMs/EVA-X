// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// ============================================================================
// FHIR R4 HTTP HANDLERS
// ============================================================================
// Endpoints REST que expõem recursos EVA-Mind no formato HL7 FHIR R4.
// Referência: https://www.hl7.org/fhir/R4/

// FHIRHandler handles FHIR R4 endpoints
type FHIRHandler struct {
	db *sql.DB
}

// NewFHIRHandler creates a new handler with a database connection
func NewFHIRHandler(db *sql.DB) *FHIRHandler {
	return &FHIRHandler{db: db}
}

// ============================================================================
// GET /api/v1/fhir/Patient/{id}
// ============================================================================
// Returns a single patient resource in FHIR R4 Patient format.

func (h *FHIRHandler) GetPatient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeFHIRError(w, http.StatusBadRequest, "invalid-id", "Patient ID must be a valid integer")
		return
	}

	patient, err := h.queryPatientDTO(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeFHIRError(w, http.StatusNotFound, "not-found", fmt.Sprintf("Patient/%d not found", id))
			return
		}
		writeFHIRError(w, http.StatusInternalServerError, "database-error", "Failed to retrieve patient data")
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
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeFHIRError(w, http.StatusBadRequest, "invalid-id", "Patient ID must be a valid integer")
		return
	}

	patient, err := h.queryPatientDTO(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeFHIRError(w, http.StatusNotFound, "not-found", fmt.Sprintf("Patient/%d not found", id))
			return
		}
		writeFHIRError(w, http.StatusInternalServerError, "database-error", "Failed to retrieve patient data")
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

// queryPatientDTO loads a patient from the idosos table and maps it to a PatientDTO.
func (h *FHIRHandler) queryPatientDTO(id int64) (*PatientDTO, error) {
	query := `
		SELECT
			id,
			nome,
			COALESCE(TO_CHAR(data_nascimento, 'YYYY-MM-DD'), ''),
			COALESCE(EXTRACT(YEAR FROM AGE(data_nascimento))::int, 0),
			COALESCE(sexo, ''),
			telefone,
			endereco,
			COALESCE(criado_em, NOW()),
			COALESCE(atualizado_em, NOW())
		FROM idosos
		WHERE id = $1
	`

	dto := &PatientDTO{}
	var phone, address sql.NullString

	err := h.db.QueryRow(query, id).Scan(
		&dto.ID,
		&dto.Name,
		&dto.DateOfBirth,
		&dto.Age,
		&dto.Gender,
		&phone,
		&address,
		&dto.CreatedAt,
		&dto.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if phone.Valid {
		dto.Phone = &phone.String
	}
	if address.Valid {
		dto.Address = &address.String
	}

	return dto, nil
}

// queryAssessmentDTOs loads all clinical assessments for a patient and maps
// them to AssessmentDTO slices suitable for FHIR conversion.
func (h *FHIRHandler) queryAssessmentDTOs(patientID int64) ([]*AssessmentDTO, error) {
	query := `
		SELECT
			id,
			patient_id,
			assessment_type,
			status,
			total_score,
			severity_level,
			clinical_interpretation,
			created_at,
			completed_at
		FROM clinical_assessments
		WHERE patient_id = $1
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(query, patientID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assessments: %w", err)
	}
	defer rows.Close()

	var assessments []*AssessmentDTO
	for rows.Next() {
		var (
			rawID          interface{} // UUID or int — scanned as string
			idStr          string
			totalScore     sql.NullInt64
			severityLevel  sql.NullString
			interpretation sql.NullString
			completedAt    sql.NullTime
		)

		dto := &AssessmentDTO{}

		err := rows.Scan(
			&rawID,
			&dto.PatientID,
			&dto.AssessmentType,
			&dto.Status,
			&totalScore,
			&severityLevel,
			&interpretation,
			&dto.CreatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan assessment row: %w", err)
		}

		// Convert raw ID (could be UUID []byte or int64) to string
		switch v := rawID.(type) {
		case []byte:
			idStr = string(v)
		case int64:
			idStr = fmt.Sprintf("%d", v)
		case string:
			idStr = v
		default:
			idStr = fmt.Sprintf("%v", v)
		}
		dto.ID = idStr

		if totalScore.Valid {
			score := int(totalScore.Int64)
			dto.TotalScore = &score
		}
		if severityLevel.Valid {
			dto.Severity = &severityLevel.String
		}
		if interpretation.Valid {
			dto.Notes = &interpretation.String
		}
		if completedAt.Valid {
			dto.CompletedAt = &completedAt.Time
		}

		dto.AdministeredBy = "eva"

		assessments = append(assessments, dto)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating assessment rows: %w", err)
	}

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
