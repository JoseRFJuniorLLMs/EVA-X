package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// EVAMetrics contains all Prometheus metrics for EVA-Mind
type EVAMetrics struct {
	// ========================================
	// Clinical Scales Metrics
	// ========================================

	// CSSRSAssessments counts C-SSRS assessments by risk level
	CSSRSAssessments *prometheus.CounterVec

	// CSSRSRiskLevel tracks current risk levels by patient
	CSSRSRiskLevel *prometheus.GaugeVec

	// PHQ9Assessments counts PHQ-9 assessments by severity
	PHQ9Assessments *prometheus.CounterVec

	// PHQ9Scores histogram of PHQ-9 scores
	PHQ9Scores prometheus.Histogram

	// GAD7Assessments counts GAD-7 assessments by severity
	GAD7Assessments *prometheus.CounterVec

	// GAD7Scores histogram of GAD-7 scores
	GAD7Scores prometheus.Histogram

	// SuicideRiskDetected counter for any suicide risk detection
	SuicideRiskDetected prometheus.Counter

	// ========================================
	// Alert System Metrics
	// ========================================

	// AlertsSent counts alerts by severity and channel
	AlertsSent *prometheus.CounterVec

	// AlertsAcknowledged counts acknowledged alerts
	AlertsAcknowledged prometheus.Counter

	// AlertDeliveryLatency measures time to deliver alerts
	AlertDeliveryLatency *prometheus.HistogramVec

	// AlertEscalations counts escalations by reason
	AlertEscalations *prometheus.CounterVec

	// ActiveAlerts tracks currently active (unacknowledged) alerts
	ActiveAlerts prometheus.Gauge

	// ========================================
	// Conversation Metrics
	// ========================================

	// ConversationsTotal total conversations
	ConversationsTotal prometheus.Counter

	// ConversationDuration histogram of conversation durations
	ConversationDuration prometheus.Histogram

	// MessagesProcessed counts messages by type
	MessagesProcessed *prometheus.CounterVec

	// ToolInvocations counts tool calls by tool name
	ToolInvocations *prometheus.CounterVec

	// ToolLatency measures tool execution time
	ToolLatency *prometheus.HistogramVec

	// ========================================
	// Memory System Metrics
	// ========================================

	// MemoryOperations counts memory operations by type
	MemoryOperations *prometheus.CounterVec

	// MemoryRetrievalLatency measures memory retrieval time
	MemoryRetrievalLatency prometheus.Histogram

	// VectorSearchLatency measures Qdrant search time
	VectorSearchLatency prometheus.Histogram

	// Neo4jQueryLatency measures Neo4j query time
	Neo4jQueryLatency prometheus.Histogram

	// ========================================
	// System Health Metrics
	// ========================================

	// ActiveWebSocketConnections current WebSocket connections
	ActiveWebSocketConnections prometheus.Gauge

	// ActivePatients currently active patients in system
	ActivePatients prometheus.Gauge

	// APIRequestsTotal total API requests
	APIRequestsTotal *prometheus.CounterVec

	// APIRequestDuration measures API response time
	APIRequestDuration *prometheus.HistogramVec

	// ErrorsTotal counts errors by type
	ErrorsTotal *prometheus.CounterVec

	// ========================================
	// LLM Metrics
	// ========================================

	// LLMRequestsTotal total LLM API calls
	LLMRequestsTotal *prometheus.CounterVec

	// LLMLatency measures LLM response time
	LLMLatency *prometheus.HistogramVec

	// LLMTokensUsed tracks token usage
	LLMTokensUsed *prometheus.CounterVec

	// LLMErrors counts LLM API errors
	LLMErrors *prometheus.CounterVec
}

var (
	instance *EVAMetrics
	once     sync.Once
)

// GetMetrics returns the singleton metrics instance
func GetMetrics() *EVAMetrics {
	once.Do(func() {
		instance = newEVAMetrics()
	})
	return instance
}

// newEVAMetrics creates and registers all metrics
func newEVAMetrics() *EVAMetrics {
	m := &EVAMetrics{
		// Clinical Scales
		CSSRSAssessments: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_cssrs_assessments_total",
				Help: "Total C-SSRS assessments by risk level",
			},
			[]string{"risk_level"}, // none, low, moderate, high, critical
		),
		CSSRSRiskLevel: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "eva_cssrs_risk_level",
				Help: "Current C-SSRS risk level by patient (0=none, 1=low, 2=moderate, 3=high, 4=critical)",
			},
			[]string{"patient_id"},
		),
		PHQ9Assessments: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_phq9_assessments_total",
				Help: "Total PHQ-9 assessments by severity",
			},
			[]string{"severity"}, // minimal, mild, moderate, moderately_severe, severe
		),
		PHQ9Scores: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_phq9_scores",
				Help:    "Distribution of PHQ-9 scores",
				Buckets: []float64{0, 5, 10, 15, 20, 27}, // Score thresholds
			},
		),
		GAD7Assessments: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_gad7_assessments_total",
				Help: "Total GAD-7 assessments by severity",
			},
			[]string{"severity"}, // minimal, mild, moderate, severe
		),
		GAD7Scores: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_gad7_scores",
				Help:    "Distribution of GAD-7 scores",
				Buckets: []float64{0, 5, 10, 15, 21}, // Score thresholds
			},
		),
		SuicideRiskDetected: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "eva_suicide_risk_detected_total",
				Help: "Total suicide risk detections (C-SSRS or PHQ-9 Q9)",
			},
		),

		// Alert System
		AlertsSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_alerts_sent_total",
				Help: "Total alerts sent by severity and channel",
			},
			[]string{"severity", "channel"}, // critica/alta/media/baixa, push/sms/email/call
		),
		AlertsAcknowledged: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "eva_alerts_acknowledged_total",
				Help: "Total alerts acknowledged by caregivers",
			},
		),
		AlertDeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "eva_alert_delivery_latency_seconds",
				Help:    "Alert delivery latency by channel",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"channel"},
		),
		AlertEscalations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_alert_escalations_total",
				Help: "Total alert escalations by reason",
			},
			[]string{"reason"}, // timeout, failure, unacknowledged
		),
		ActiveAlerts: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "eva_active_alerts",
				Help: "Currently active (unacknowledged) alerts",
			},
		),

		// Conversations
		ConversationsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "eva_conversations_total",
				Help: "Total conversations started",
			},
		),
		ConversationDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_conversation_duration_seconds",
				Help:    "Duration of conversations",
				Buckets: []float64{60, 300, 600, 1800, 3600, 7200}, // 1m, 5m, 10m, 30m, 1h, 2h
			},
		),
		MessagesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_messages_processed_total",
				Help: "Total messages processed by type",
			},
			[]string{"type"}, // user, assistant, system
		),
		ToolInvocations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_tool_invocations_total",
				Help: "Total tool invocations by tool name",
			},
			[]string{"tool_name", "success"},
		),
		ToolLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "eva_tool_latency_seconds",
				Help:    "Tool execution latency",
				Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
			},
			[]string{"tool_name"},
		),

		// Memory System
		MemoryOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_memory_operations_total",
				Help: "Total memory operations by type",
			},
			[]string{"operation"}, // store, retrieve, search, consolidate
		),
		MemoryRetrievalLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_memory_retrieval_latency_seconds",
				Help:    "Memory retrieval latency",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1},
			},
		),
		VectorSearchLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_vector_search_latency_seconds",
				Help:    "Qdrant vector search latency",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1},
			},
		),
		Neo4jQueryLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "eva_neo4j_query_latency_seconds",
				Help:    "Neo4j query latency",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1},
			},
		),

		// System Health
		ActiveWebSocketConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "eva_active_websocket_connections",
				Help: "Current number of active WebSocket connections",
			},
		),
		ActivePatients: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "eva_active_patients",
				Help: "Number of currently active patients",
			},
		),
		APIRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_api_requests_total",
				Help: "Total API requests by endpoint and status",
			},
			[]string{"endpoint", "method", "status"},
		),
		APIRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "eva_api_request_duration_seconds",
				Help:    "API request duration",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"endpoint", "method"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_errors_total",
				Help: "Total errors by type",
			},
			[]string{"type", "component"},
		),

		// LLM
		LLMRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_llm_requests_total",
				Help: "Total LLM API requests by model",
			},
			[]string{"model", "success"},
		),
		LLMLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "eva_llm_latency_seconds",
				Help:    "LLM API response latency",
				Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60},
			},
			[]string{"model"},
		),
		LLMTokensUsed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_llm_tokens_used_total",
				Help: "Total tokens used by type",
			},
			[]string{"model", "type"}, // type: input, output
		),
		LLMErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "eva_llm_errors_total",
				Help: "Total LLM API errors",
			},
			[]string{"model", "error_type"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		// Clinical
		m.CSSRSAssessments,
		m.CSSRSRiskLevel,
		m.PHQ9Assessments,
		m.PHQ9Scores,
		m.GAD7Assessments,
		m.GAD7Scores,
		m.SuicideRiskDetected,
		// Alerts
		m.AlertsSent,
		m.AlertsAcknowledged,
		m.AlertDeliveryLatency,
		m.AlertEscalations,
		m.ActiveAlerts,
		// Conversations
		m.ConversationsTotal,
		m.ConversationDuration,
		m.MessagesProcessed,
		m.ToolInvocations,
		m.ToolLatency,
		// Memory
		m.MemoryOperations,
		m.MemoryRetrievalLatency,
		m.VectorSearchLatency,
		m.Neo4jQueryLatency,
		// System
		m.ActiveWebSocketConnections,
		m.ActivePatients,
		m.APIRequestsTotal,
		m.APIRequestDuration,
		m.ErrorsTotal,
		// LLM
		m.LLMRequestsTotal,
		m.LLMLatency,
		m.LLMTokensUsed,
		m.LLMErrors,
	)

	return m
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// ========================================
// Helper Methods for Recording Metrics
// ========================================

// RecordCSSRSAssessment records a C-SSRS assessment
func (m *EVAMetrics) RecordCSSRSAssessment(riskLevel string, patientID string) {
	m.CSSRSAssessments.WithLabelValues(riskLevel).Inc()

	// Map risk level to numeric value
	riskValue := float64(0)
	switch riskLevel {
	case "low":
		riskValue = 1
	case "moderate":
		riskValue = 2
	case "high":
		riskValue = 3
	case "critical":
		riskValue = 4
	}
	m.CSSRSRiskLevel.WithLabelValues(patientID).Set(riskValue)

	if riskLevel != "none" && riskLevel != "low" {
		m.SuicideRiskDetected.Inc()
	}
}

// RecordPHQ9Assessment records a PHQ-9 assessment
func (m *EVAMetrics) RecordPHQ9Assessment(score int, severity string, hasSuicideRisk bool) {
	m.PHQ9Assessments.WithLabelValues(severity).Inc()
	m.PHQ9Scores.Observe(float64(score))

	if hasSuicideRisk {
		m.SuicideRiskDetected.Inc()
	}
}

// RecordGAD7Assessment records a GAD-7 assessment
func (m *EVAMetrics) RecordGAD7Assessment(score int, severity string) {
	m.GAD7Assessments.WithLabelValues(severity).Inc()
	m.GAD7Scores.Observe(float64(score))
}

// RecordAlertSent records an alert being sent
func (m *EVAMetrics) RecordAlertSent(severity, channel string) {
	m.AlertsSent.WithLabelValues(severity, channel).Inc()
}

// RecordAlertDelivery records alert delivery with latency
func (m *EVAMetrics) RecordAlertDelivery(channel string, latencySeconds float64) {
	m.AlertDeliveryLatency.WithLabelValues(channel).Observe(latencySeconds)
}

// RecordToolInvocation records a tool being invoked
func (m *EVAMetrics) RecordToolInvocation(toolName string, success bool, latencySeconds float64) {
	successStr := "true"
	if !success {
		successStr = "false"
	}
	m.ToolInvocations.WithLabelValues(toolName, successStr).Inc()
	m.ToolLatency.WithLabelValues(toolName).Observe(latencySeconds)
}

// RecordLLMRequest records an LLM API call
func (m *EVAMetrics) RecordLLMRequest(model string, success bool, latencySeconds float64, inputTokens, outputTokens int) {
	successStr := "true"
	if !success {
		successStr = "false"
	}
	m.LLMRequestsTotal.WithLabelValues(model, successStr).Inc()
	m.LLMLatency.WithLabelValues(model).Observe(latencySeconds)
	m.LLMTokensUsed.WithLabelValues(model, "input").Add(float64(inputTokens))
	m.LLMTokensUsed.WithLabelValues(model, "output").Add(float64(outputTokens))
}

// RecordError records an error occurrence
func (m *EVAMetrics) RecordError(errorType, component string) {
	m.ErrorsTotal.WithLabelValues(errorType, component).Inc()
}
