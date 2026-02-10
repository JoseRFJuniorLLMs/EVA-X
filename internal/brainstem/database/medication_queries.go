package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Medicamento represents a medication in the database
type Medicamento struct {
	ID            int64
	IdosoID       int64
	Nome          string
	Dosagem       string
	Frequencia    string
	Horarios      string // JSON ou string com horários programados
	CorEmbalagem  string
	Fabricante    string
	Ativo         bool
	DataInicio    time.Time
	DataFim       *time.Time
	Observacoes   string
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
		return db.GetActiveMedications(idosoID) // Fallback to all active
	}

	query := `
		SELECT
			id, idoso_id, nome, dosagem, frequencia, horarios,
			cor_embalagem, fabricante, ativo, data_inicio, data_fim, observacoes
		FROM medicamentos
		WHERE idoso_id = $1
		AND ativo = true
		AND (data_fim IS NULL OR data_fim > NOW())
		AND horarios LIKE $2
		ORDER BY horarios
	`

	rows, err := db.Conn.Query(query, idosoID, "%"+timeRange+"%")
	if err != nil {
		log.Printf("❌ [DB] Error querying medications by schedule: %v", err)
		return nil, err
	}
	defer rows.Close()

	return db.scanMedications(rows)
}

// GetActiveMedications retrieves all active medications for a patient
func (db *DB) GetActiveMedications(idosoID int64) ([]Medicamento, error) {
	query := `
		SELECT
			id, idoso_id, nome, dosagem, frequencia, horarios,
			cor_embalagem, fabricante, ativo, data_inicio, data_fim, observacoes
		FROM medicamentos
		WHERE idoso_id = $1
		AND ativo = true
		AND (data_fim IS NULL OR data_fim > NOW())
		ORDER BY nome
	`

	rows, err := db.Conn.Query(query, idosoID)
	if err != nil {
		log.Printf("❌ [DB] Error querying active medications: %v", err)
		return nil, err
	}
	defer rows.Close()

	return db.scanMedications(rows)
}

// GetMedicationByID retrieves a specific medication by ID
func (db *DB) GetMedicationByID(medicationID int64) (*Medicamento, error) {
	query := `
		SELECT
			id, idoso_id, nome, dosagem, frequencia, horarios,
			cor_embalagem, fabricante, ativo, data_inicio, data_fim, observacoes
		FROM medicamentos
		WHERE id = $1
	`

	var med Medicamento
	err := db.Conn.QueryRow(query, medicationID).Scan(
		&med.ID,
		&med.IdosoID,
		&med.Nome,
		&med.Dosagem,
		&med.Frequencia,
		&med.Horarios,
		&med.CorEmbalagem,
		&med.Fabricante,
		&med.Ativo,
		&med.DataInicio,
		&med.DataFim,
		&med.Observacoes,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("medication not found")
	}

	if err != nil {
		log.Printf("❌ [DB] Error querying medication by ID: %v", err)
		return nil, err
	}

	return &med, nil
}

// scanMedications is a helper function to scan rows into Medicamento slice
func (db *DB) scanMedications(rows *sql.Rows) ([]Medicamento, error) {
	var medications []Medicamento

	for rows.Next() {
		var med Medicamento
		err := rows.Scan(
			&med.ID,
			&med.IdosoID,
			&med.Nome,
			&med.Dosagem,
			&med.Frequencia,
			&med.Horarios,
			&med.CorEmbalagem,
			&med.Fabricante,
			&med.Ativo,
			&med.DataInicio,
			&med.DataFim,
			&med.Observacoes,
		)

		if err != nil {
			log.Printf("❌ [DB] Error scanning medication row: %v", err)
			continue
		}

		medications = append(medications, med)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return medications, nil
}

// LogMedicationTaken records that a medication was taken
func (db *DB) LogMedicationTaken(medicationID int64, takenAt time.Time, visualProofURL string) error {
	query := `
		INSERT INTO medication_logs (
			medication_id, taken_at, verification_method, image_proof_url, created_at
		) VALUES ($1, $2, 'visual_scan', $3, NOW())
	`

	_, err := db.Conn.Exec(query, medicationID, takenAt, visualProofURL)
	if err != nil {
		log.Printf("❌ [DB] Error logging medication taken: %v", err)
		return err
	}

	log.Printf("✅ [DB] Medication %d logged as taken at %v", medicationID, takenAt)
	return nil
}

// CheckMedicationSafety verifies if it's safe to take a medication
func (db *DB) CheckMedicationSafety(medicationID int64) (*MedicationSafetyCheck, error) {
	// 1. Check if already taken today
	queryToday := `
		SELECT COUNT(*)
		FROM medication_logs
		WHERE medication_id = $1
		AND DATE(taken_at) = CURRENT_DATE
	`

	var takenToday int
	err := db.Conn.QueryRow(queryToday, medicationID).Scan(&takenToday)
	if err != nil {
		return nil, err
	}

	// 2. Get medication frequency
	med, err := db.GetMedicationByID(medicationID)
	if err != nil {
		return nil, err
	}

	// 3. Check last dose time
	queryLastDose := `
		SELECT taken_at
		FROM medication_logs
		WHERE medication_id = $1
		ORDER BY taken_at DESC
		LIMIT 1
	`

	var lastDoseTime *time.Time
	err = db.Conn.QueryRow(queryLastDose, medicationID).Scan(&lastDoseTime)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var hoursSinceLastDose float64
	if lastDoseTime != nil {
		hoursSinceLastDose = time.Since(*lastDoseTime).Hours()
	}

	// Determine safety
	safeToTake := true
	warnings := []string{}

	// Check frequency (example: if 2x/day and already taken 2x today)
	if med.Frequencia == "2x/dia" && takenToday >= 2 {
		safeToTake = false
		warnings = append(warnings, "OVERDOSE: Medicamento já foi tomado 2 vezes hoje")
	}

	// Check minimum interval (example: 12h for 2x/day)
	if med.Frequencia == "2x/dia" && hoursSinceLastDose < 6 {
		safeToTake = false
		warnings = append(warnings, fmt.Sprintf("INTERVALO MÍNIMO: Aguarde mais %.1f horas", 6-hoursSinceLastDose))
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
