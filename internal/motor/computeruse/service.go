// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package computeruse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// TaskType define os tipos de tarefas suportadas
type TaskType string

const (
	TaskBuyMedication       TaskType = "buy_medication"
	TaskScheduleAppointment TaskType = "schedule_appointment"
	TaskOrderFood           TaskType = "order_food"
	TaskRequestRide         TaskType = "request_ride"
	TaskOther               TaskType = "other"
)

// TaskStatus define os status possíveis
type TaskStatus string

const (
	StatusPending          TaskStatus = "pending"
	StatusApproved         TaskStatus = "approved"
	StatusExecuting        TaskStatus = "executing"
	StatusCompleted        TaskStatus = "completed"
	StatusFailed           TaskStatus = "failed"
	StatusCancelled        TaskStatus = "cancelled"
	StatusRequiresApproval TaskStatus = "requires_approval"
)

// AutomationTask representa uma tarefa de automação
type AutomationTask struct {
	ID               int64           `json:"id"`
	IdosoID          int64           `json:"idoso_id"`
	TaskType         TaskType        `json:"task_type"`
	ServiceName      string          `json:"service_name"`
	TaskParams       json.RawMessage `json:"task_params"`
	Status           TaskStatus      `json:"status"`
	ApprovalRequired bool            `json:"approval_required"`
	ApprovedBy       *int64          `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time      `json:"approved_at,omitempty"`
	ExecutionLog     json.RawMessage `json:"execution_log,omitempty"`
	Screenshots      json.RawMessage `json:"screenshots,omitempty"`
	Result           json.RawMessage `json:"result,omitempty"`
	ErrorMessage     *string         `json:"error_message,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ExecutedAt       *time.Time      `json:"executed_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

// MedicationPurchaseParams parâmetros para compra de medicamento
type MedicationPurchaseParams struct {
	MedicationName string  `json:"medication_name"`
	Dosage         string  `json:"dosage"`
	Quantity       int     `json:"quantity"`
	Address        string  `json:"address"`
	MaxPrice       float64 `json:"max_price,omitempty"`
}

// AppointmentScheduleParams parâmetros para agendamento
type AppointmentScheduleParams struct {
	DoctorName      string `json:"doctor_name,omitempty"`
	Specialty       string `json:"specialty"`
	PreferredDate   string `json:"preferred_date"`
	PreferredTime   string `json:"preferred_time"`
	Location        string `json:"location,omitempty"`
	HealthInsurance string `json:"health_insurance,omitempty"`
}

// FoodOrderParams parâmetros para pedido de comida
type FoodOrderParams struct {
	Restaurant string   `json:"restaurant,omitempty"`
	Items      []string `json:"items"`
	Address    string   `json:"address"`
	MaxPrice   float64  `json:"max_price,omitempty"`
}

// RideRequestParams parâmetros para solicitação de corrida
type RideRequestParams struct {
	PickupAddress      string  `json:"pickup_address"`
	DestinationAddress string  `json:"destination_address"`
	RideType           string  `json:"ride_type"` // "economy", "comfort", "premium"
	MaxPrice           float64 `json:"max_price,omitempty"`
}

// Service gerencia tarefas de automação
type Service struct {
	db *database.DB
}

// NewService cria novo serviço de automação
func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// CreateTask cria nova tarefa de automação
func (s *Service) CreateTask(ctx context.Context, idosoID int64, taskType TaskType, serviceName string, params interface{}, requiresApproval bool) (int64, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar parametros: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := string(StatusPending)
	if requiresApproval {
		status = string(StatusRequiresApproval)
	}

	taskID, err := s.db.Insert(ctx, "automation_tasks", map[string]interface{}{
		"idoso_id":          idosoID,
		"task_type":         string(taskType),
		"service_name":      serviceName,
		"task_params":       string(paramsJSON),
		"status":            status,
		"approval_required": requiresApproval,
		"created_at":        now,
		"updated_at":        now,
	})

	if err != nil {
		return 0, fmt.Errorf("erro ao criar tarefa: %w", err)
	}

	log.Printf("[COMPUTER USE] Tarefa criada: ID=%d, Tipo=%s, Servico=%s, Aprovacao=%v",
		taskID, taskType, serviceName, requiresApproval)

	return taskID, nil
}

// ApproveTask aprova uma tarefa
func (s *Service) ApproveTask(ctx context.Context, taskID, approverID int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.Update(ctx, "automation_tasks",
		map[string]interface{}{"id": taskID},
		map[string]interface{}{
			"status":      string(StatusApproved),
			"approved_by": approverID,
			"approved_at": now,
			"updated_at":  now,
		})
	if err != nil {
		return fmt.Errorf("erro ao aprovar tarefa: %w", err)
	}

	log.Printf("[COMPUTER USE] Tarefa %d aprovada por usuario %d", taskID, approverID)
	return nil
}

// RejectTask rejeita uma tarefa
func (s *Service) RejectTask(ctx context.Context, taskID, approverID int64, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.Update(ctx, "automation_tasks",
		map[string]interface{}{"id": taskID},
		map[string]interface{}{
			"status":        string(StatusCancelled),
			"approved_by":   approverID,
			"error_message": reason,
			"updated_at":    now,
			"completed_at":  now,
		})
	if err != nil {
		return fmt.Errorf("erro ao rejeitar tarefa: %w", err)
	}

	log.Printf("[COMPUTER USE] Tarefa %d rejeitada por usuario %d: %s", taskID, approverID, reason)
	return nil
}

// LogStep registra um passo da execução
func (s *Service) LogStep(ctx context.Context, taskID int64, stepNumber int, stepName, stepStatus string, screenshotURL *string, stepData interface{}, errorMsg *string) error {
	stepDataJSON, _ := json.Marshal(stepData)

	now := time.Now().UTC().Format(time.RFC3339)
	content := map[string]interface{}{
		"task_id":     taskID,
		"step_number": stepNumber,
		"step_name":   stepName,
		"step_status": stepStatus,
		"step_data":   string(stepDataJSON),
		"created_at":  now,
	}
	if screenshotURL != nil {
		content["screenshot_url"] = *screenshotURL
	}
	if errorMsg != nil {
		content["error_message"] = *errorMsg
	}

	_, err := s.db.Insert(ctx, "automation_steps", content)
	if err != nil {
		return fmt.Errorf("erro ao registrar passo: %w", err)
	}

	return nil
}

// UpdateTaskStatus atualiza status da tarefa
func (s *Service) UpdateTaskStatus(ctx context.Context, taskID int64, status TaskStatus, result interface{}, errorMsg *string) error {
	resultJSON, _ := json.Marshal(result)
	now := time.Now().UTC().Format(time.RFC3339)

	updates := map[string]interface{}{
		"status":     string(status),
		"result":     string(resultJSON),
		"updated_at": now,
	}
	if errorMsg != nil {
		updates["error_message"] = *errorMsg
	}

	// Set completed_at for terminal states
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		updates["completed_at"] = now
	}

	err := s.db.Update(ctx, "automation_tasks",
		map[string]interface{}{"id": taskID},
		updates)
	if err != nil {
		return fmt.Errorf("erro ao atualizar status: %w", err)
	}

	log.Printf("[COMPUTER USE] Tarefa %d atualizada: status=%s", taskID, status)
	return nil
}

// GetPendingApprovals retorna tarefas aguardando aprovação
func (s *Service) GetPendingApprovals(ctx context.Context) ([]AutomationTask, error) {
	rows, err := s.db.QueryByLabel(ctx, "automation_tasks",
		" AND n.status = $status AND n.approval_required = $approval",
		map[string]interface{}{"status": string(StatusPending), "approval": true}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar aprovacoes pendentes: %w", err)
	}

	var tasks []AutomationTask
	for _, m := range rows {
		task := mapToAutomationTask(m)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTask retorna uma tarefa específica
func (s *Service) GetTask(ctx context.Context, taskID int64) (*AutomationTask, error) {
	m, err := s.db.GetNodeByID(ctx, "automation_tasks", taskID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar tarefa: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("tarefa nao encontrada: %d", taskID)
	}

	task := mapToAutomationTask(m)
	return &task, nil
}

// mapToAutomationTask converts a NietzscheDB content map to an AutomationTask
func mapToAutomationTask(m map[string]interface{}) AutomationTask {
	task := AutomationTask{
		ID:               database.GetInt64(m, "id"),
		IdosoID:          database.GetInt64(m, "idoso_id"),
		TaskType:         TaskType(database.GetString(m, "task_type")),
		ServiceName:      database.GetString(m, "service_name"),
		Status:           TaskStatus(database.GetString(m, "status")),
		ApprovalRequired: database.GetBool(m, "approval_required"),
		CreatedAt:        database.GetTime(m, "created_at"),
		UpdatedAt:        database.GetTime(m, "updated_at"),
		ApprovedAt:       database.GetTimePtr(m, "approved_at"),
		ExecutedAt:       database.GetTimePtr(m, "executed_at"),
		CompletedAt:      database.GetTimePtr(m, "completed_at"),
	}

	if raw := database.GetString(m, "task_params"); raw != "" {
		task.TaskParams = json.RawMessage(raw)
	}
	if raw := database.GetString(m, "execution_log"); raw != "" {
		task.ExecutionLog = json.RawMessage(raw)
	}
	if raw := database.GetString(m, "screenshots"); raw != "" {
		task.Screenshots = json.RawMessage(raw)
	}
	if raw := database.GetString(m, "result"); raw != "" {
		task.Result = json.RawMessage(raw)
	}
	if errMsg := database.GetString(m, "error_message"); errMsg != "" {
		task.ErrorMessage = &errMsg
	}

	approvedBy := database.GetInt64(m, "approved_by")
	if approvedBy > 0 {
		task.ApprovedBy = &approvedBy
	}

	return task
}
