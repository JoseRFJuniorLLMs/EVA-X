// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// Medicamento represents a medication in the database
type Medicamento struct {
	ID           int64
	IdosoID      int64
	Nome         string
	Dosagem      string
	Frequencia   string
	Horarios     string // JSON ou string com horarios programados
	CorEmbalagem string
	Fabricante   string
	Ativo        bool
	DataInicio   time.Time
	DataFim      *time.Time
	Observacoes  string
}

func contentToMedicamento(m map[string]interface{}) Medicamento {
	return Medicamento{
		ID:           getInt64(m, "id"),
		IdosoID:      getInt64(m, "idoso_id"),
		Nome:         getString(m, "nome"),
		Dosagem:      getString(m, "dosagem"),
		Frequencia:   getString(m, "frequencia"),
		Horarios:     getString(m, "horarios"),
		CorEmbalagem: getString(m, "cor_embalagem"),
		Fabricante:   getString(m, "fabricante"),
		Ativo:        getBool(m, "ativo"),
		DataInicio:   getTime(m, "data_inicio"),
		DataFim:      getTimePtr(m, "data_fim"),
		Observacoes:  getString(m, "observacoes"),
	}
}

// isActiveNow checks if a medication is currently active (not expired).
func isActiveNow(med Medicamento) bool {
	if !med.Ativo {
		return false
	}
	if med.DataFim != nil && med.DataFim.Before(time.Now()) {
		return false
	}
	return true
}

// GetMedicationsBySchedule retrieves medications for a specific time of day
func (db *DB) GetMedicationsBySchedule(idosoID int64, timeOfDay string) ([]Medicamento, error) {
	// Map time_of_day to hour ranges
	timeRanges := map[string]string{
		"morning":   "06:00-11:59",
		"afternoon": "12:00-17:59",
		"evening":   "18:00-21:59",
		"night":     "22:00-05:59",
	}

	timeRange, ok := timeRanges[timeOfDay]
	if !ok {
		return db.GetActiveMedications(idosoID)
	}

	meds, err := db.GetActiveMedications(idosoID)
	if err != nil {
		return nil, err
	}

	// Filter by schedule (horarios LIKE timeRange)
	var filtered []Medicamento
	for _, med := range meds {
		if strings.Contains(med.Horarios, timeRange) {
			filtered = append(filtered, med)
		}
	}

	return filtered, nil
}

// GetActiveMedications retrieves all active medications for a patient
func (db *DB) GetActiveMedications(idosoID int64) ([]Medicamento, error) {
	ctx := context.Background()

	rows, err := db.queryNodesByLabel(ctx, "medicamentos",
		` AND n.idoso_id = $idoso_id`, map[string]interface{}{
			"idoso_id": idosoID,
		}, 0)
	if err != nil {
		log.Printf("[DB] Error querying active medications: %v", err)
		return nil, err
	}

	var medications []Medicamento
	for _, m := range rows {
		med := contentToMedicamento(m)
		if isActiveNow(med) {
			medications = append(medications, med)
		}
	}

	// Sort by nome
	sort.Slice(medications, func(i, j int) bool {
		return medications[i].Nome < medications[j].Nome
	})

	return medications, nil
}

// GetMedicationByID retrieves a specific medication by ID
func (db *DB) GetMedicationByID(medicationID int64) (*Medicamento, error) {
	ctx := context.Background()

	m, err := db.getNode(ctx, "medicamentos", medicationID)
	if err != nil {
		log.Printf("[DB] Error querying medication by ID: %v", err)
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("medication not found")
	}

	med := contentToMedicamento(m)
	return &med, nil
}

// LogMedicationTaken records that a medication was taken
func (db *DB) LogMedicationTaken(medicationID int64, takenAt time.Time, visualProofURL string) error {
	ctx := context.Background()

	_, err := db.insertRow(ctx, "medication_logs", map[string]interface{}{
		"medication_id":       medicationID,
		"taken_at":            takenAt.Format(time.RFC3339),
		"verification_method": "visual_scan",
		"image_proof_url":     visualProofURL,
		"created_at":          time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("[DB] Error logging medication taken: %v", err)
		return err
	}

	log.Printf("[NIETZSCHE] Medication %d logged as taken at %v", medicationID, takenAt)
	return nil
}

// CheckMedicationSafety verifies if it's safe to take a medication
func (db *DB) CheckMedicationSafety(medicationID int64) (*MedicationSafetyCheck, error) {
	ctx := context.Background()

	// 1. Count how many times taken today
	rows, err := db.queryNodesByLabel(ctx, "medication_logs",
		` AND n.medication_id = $med_id`, map[string]interface{}{
			"med_id": medicationID,
		}, 0)
	if err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	takenToday := 0
	var lastDoseTime *time.Time

	for _, m := range rows {
		takenAt := getTime(m, "taken_at")
		if takenAt.After(today) {
			takenToday++
		}
		if lastDoseTime == nil || takenAt.After(*lastDoseTime) {
			t := takenAt
			lastDoseTime = &t
		}
	}

	// 2. Get medication frequency
	med, err := db.GetMedicationByID(medicationID)
	if err != nil {
		return nil, err
	}

	// 3. Calculate hours since last dose
	var hoursSinceLastDose float64
	if lastDoseTime != nil {
		hoursSinceLastDose = time.Since(*lastDoseTime).Hours()
	}

	// Determine safety
	safeToTake := true
	var warnings []string

	if med.Frequencia == "2x/dia" && takenToday >= 2 {
		safeToTake = false
		warnings = append(warnings, "OVERDOSE: Medicamento ja foi tomado 2 vezes hoje")
	}

	if med.Frequencia == "2x/dia" && hoursSinceLastDose < 6 {
		safeToTake = false
		warnings = append(warnings, fmt.Sprintf("INTERVALO MINIMO: Aguarde mais %.1f horas", 6-hoursSinceLastDose))
	}

	return &MedicationSafetyCheck{
		SafeToTake:         safeToTake,
		TakenToday:         takenToday,
		HoursSinceLastDose: hoursSinceLastDose,
		Warnings:           warnings,
	}, nil
}

// MedicationSafetyCheck represents the result of a safety check
type MedicationSafetyCheck struct {
	SafeToTake         bool
	TakenToday         int
	HoursSinceLastDose float64
	Warnings           []string
}
