package computeruse

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
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
	db *sql.DB
}

// NewService cria novo serviço de automação
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateTask cria nova tarefa de automação
func (s *Service) CreateTask(ctx context.Context, idosoID int64, taskType TaskType, serviceName string, params interface{}, requiresApproval bool) (int64, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return 0, fmt.Errorf("erro ao serializar parâmetros: %w", err)
	}

	query := `SELECT create_automation_task($1, $2, $3, $4, $5)`

	var taskID int64
	err = s.db.QueryRowContext(ctx, query, idosoID, taskType, serviceName, paramsJSON, requiresApproval).Scan(&taskID)

	if err != nil {
		return 0, fmt.Errorf("erro ao criar tarefa: %w", err)
	}

	log.Printf("🤖 [COMPUTER USE] Tarefa criada: ID=%d, Tipo=%s, Serviço=%s, Aprovação=%v",
		taskID, taskType, serviceName, requiresApproval)

	return taskID, nil
}

// ApproveTask aprova uma tarefa
func (s *Service) ApproveTask(ctx context.Context, taskID, approverID int64) error {
	query := `SELECT approve_automation_task($1, $2)`

	_, err := s.db.ExecContext(ctx, query, taskID, approverID)
	if err != nil {
		return fmt.Errorf("erro ao aprovar tarefa: %w", err)
	}

	log.Printf("✅ [COMPUTER USE] Tarefa %d aprovada por usuário %d", taskID, approverID)
	return nil
}

// RejectTask rejeita uma tarefa
func (s *Service) RejectTask(ctx context.Context, taskID, approverID int64, reason string) error {
	query := `SELECT reject_automation_task($1, $2, $3)`

	_, err := s.db.ExecContext(ctx, query, taskID, approverID, reason)
	if err != nil {
		return fmt.Errorf("erro ao rejeitar tarefa: %w", err)
	}

	log.Printf("❌ [COMPUTER USE] Tarefa %d rejeitada por usuário %d: %s", taskID, approverID, reason)
	return nil
}

// LogStep registra um passo da execução
func (s *Service) LogStep(ctx context.Context, taskID int64, stepNumber int, stepName, stepStatus string, screenshotURL *string, stepData interface{}, errorMsg *string) error {
	var stepDataJSON interface{}
	if stepData != nil {
		stepDataJSON = stepData
	}

	query := `SELECT log_automation_step($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.db.ExecContext(ctx, query, taskID, stepNumber, stepName, stepStatus, screenshotURL, stepDataJSON, errorMsg)
	if err != nil {
		return fmt.Errorf("erro ao registrar passo: %w", err)
	}

	return nil
}

// UpdateTaskStatus atualiza status da tarefa
func (s *Service) UpdateTaskStatus(ctx context.Context, taskID int64, status TaskStatus, result interface{}, errorMsg *string) error {
	resultJSON, _ := json.Marshal(result)

	query := `
		UPDATE automation_tasks
		SET status = $1,
		    result = $2,
		    error_message = $3,
		    updated_at = NOW(),
		    completed_at = CASE WHEN $1 IN ('completed', 'failed', 'cancelled') THEN NOW() ELSE NULL END
		WHERE id = $4
	`

	_, err := s.db.ExecContext(ctx, query, status, resultJSON, errorMsg, taskID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar status: %w", err)
	}

	log.Printf("📝 [COMPUTER USE] Tarefa %d atualizada: status=%s", taskID, status)
	return nil
}

// GetPendingApprovals retorna tarefas aguardando aprovação
func (s *Service) GetPendingApprovals(ctx context.Context) ([]AutomationTask, error) {
	query := `
		SELECT 
			id, idoso_id, task_type, service_name, task_params,
			status, approval_required, created_at, updated_at
		FROM automation_tasks
		WHERE status = 'pending' AND approval_required = true
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar aprovações pendentes: %w", err)
	}
	defer rows.Close()

	var tasks []AutomationTask
	for rows.Next() {
		var task AutomationTask
		err := rows.Scan(
			&task.ID,
			&task.IdosoID,
			&task.TaskType,
			&task.ServiceName,
			&task.TaskParams,
			&task.Status,
			&task.ApprovalRequired,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear tarefa: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTask retorna uma tarefa específica
func (s *Service) GetTask(ctx context.Context, taskID int64) (*AutomationTask, error) {
	query := `
		SELECT 
			id, idoso_id, task_type, service_name, task_params,
			status, approval_required, approved_by, approved_at,
			execution_log, screenshots, result, error_message,
			created_at, updated_at, executed_at, completed_at
		FROM automation_tasks
		WHERE id = $1
	`

	var task AutomationTask
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&task.ID,
		&task.IdosoID,
		&task.TaskType,
		&task.ServiceName,
		&task.TaskParams,
		&task.Status,
		&task.ApprovalRequired,
		&task.ApprovedBy,
		&task.ApprovedAt,
		&task.ExecutionLog,
		&task.Screenshots,
		&task.Result,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.ExecutedAt,
		&task.CompletedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao buscar tarefa: %w", err)
	}

	return &task, nil
}
