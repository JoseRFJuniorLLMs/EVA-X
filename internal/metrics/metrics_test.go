package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure sync is used
var _ = sync.Once{}

// resetMetrics resets the singleton for testing
func resetMetrics() {
	// Create a new registry for each test
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	instance = nil
	once = sync.Once{}
}

// Note: These tests use a separate registry to avoid conflicts

func TestGetMetrics_Singleton(t *testing.T) {
	// Test that GetMetrics returns the same instance
	m1 := GetMetrics()
	m2 := GetMetrics()

	assert.Same(t, m1, m2, "GetMetrics should return the same instance")
}

func TestEVAMetrics_AllMetricsRegistered(t *testing.T) {
	m := GetMetrics()

	// Clinical metrics
	assert.NotNil(t, m.CSSRSAssessments, "CSSRSAssessments should be registered")
	assert.NotNil(t, m.CSSRSRiskLevel, "CSSRSRiskLevel should be registered")
	assert.NotNil(t, m.PHQ9Assessments, "PHQ9Assessments should be registered")
	assert.NotNil(t, m.PHQ9Scores, "PHQ9Scores should be registered")
	assert.NotNil(t, m.GAD7Assessments, "GAD7Assessments should be registered")
	assert.NotNil(t, m.GAD7Scores, "GAD7Scores should be registered")
	assert.NotNil(t, m.SuicideRiskDetected, "SuicideRiskDetected should be registered")

	// Alert metrics
	assert.NotNil(t, m.AlertsSent, "AlertsSent should be registered")
	assert.NotNil(t, m.AlertsAcknowledged, "AlertsAcknowledged should be registered")
	assert.NotNil(t, m.AlertDeliveryLatency, "AlertDeliveryLatency should be registered")
	assert.NotNil(t, m.AlertEscalations, "AlertEscalations should be registered")
	assert.NotNil(t, m.ActiveAlerts, "ActiveAlerts should be registered")

	// Conversation metrics
	assert.NotNil(t, m.ConversationsTotal, "ConversationsTotal should be registered")
	assert.NotNil(t, m.ConversationDuration, "ConversationDuration should be registered")
	assert.NotNil(t, m.MessagesProcessed, "MessagesProcessed should be registered")
	assert.NotNil(t, m.ToolInvocations, "ToolInvocations should be registered")
	assert.NotNil(t, m.ToolLatency, "ToolLatency should be registered")

	// Memory metrics
	assert.NotNil(t, m.MemoryOperations, "MemoryOperations should be registered")
	assert.NotNil(t, m.MemoryRetrievalLatency, "MemoryRetrievalLatency should be registered")
	assert.NotNil(t, m.VectorSearchLatency, "VectorSearchLatency should be registered")
	assert.NotNil(t, m.Neo4jQueryLatency, "Neo4jQueryLatency should be registered")

	// System metrics
	assert.NotNil(t, m.ActiveWebSocketConnections, "ActiveWebSocketConnections should be registered")
	assert.NotNil(t, m.ActivePatients, "ActivePatients should be registered")
	assert.NotNil(t, m.APIRequestsTotal, "APIRequestsTotal should be registered")
	assert.NotNil(t, m.APIRequestDuration, "APIRequestDuration should be registered")
	assert.NotNil(t, m.ErrorsTotal, "ErrorsTotal should be registered")

	// LLM metrics
	assert.NotNil(t, m.LLMRequestsTotal, "LLMRequestsTotal should be registered")
	assert.NotNil(t, m.LLMLatency, "LLMLatency should be registered")
	assert.NotNil(t, m.LLMTokensUsed, "LLMTokensUsed should be registered")
	assert.NotNil(t, m.LLMErrors, "LLMErrors should be registered")
}

func TestRecordCSSRSAssessment(t *testing.T) {
	m := GetMetrics()

	// Record assessments with different risk levels
	m.RecordCSSRSAssessment("none", "patient1")
	m.RecordCSSRSAssessment("low", "patient2")
	m.RecordCSSRSAssessment("moderate", "patient3")
	m.RecordCSSRSAssessment("high", "patient4")
	m.RecordCSSRSAssessment("critical", "patient5")

	// Verify counters were incremented
	assert.Greater(t, testutil.ToFloat64(m.CSSRSAssessments.WithLabelValues("none")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.CSSRSAssessments.WithLabelValues("moderate")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.CSSRSAssessments.WithLabelValues("critical")), float64(0))
}

func TestRecordCSSRSAssessment_SuicideRisk(t *testing.T) {
	m := GetMetrics()

	initialSuicideCount := testutil.ToFloat64(m.SuicideRiskDetected)

	// These should NOT increment suicide risk counter
	m.RecordCSSRSAssessment("none", "patient1")
	m.RecordCSSRSAssessment("low", "patient2")

	countAfterLow := testutil.ToFloat64(m.SuicideRiskDetected)
	assert.Equal(t, initialSuicideCount, countAfterLow, "none/low should not increment suicide risk")

	// These SHOULD increment suicide risk counter
	m.RecordCSSRSAssessment("moderate", "patient3")
	m.RecordCSSRSAssessment("high", "patient4")
	m.RecordCSSRSAssessment("critical", "patient5")

	finalCount := testutil.ToFloat64(m.SuicideRiskDetected)
	assert.Greater(t, finalCount, countAfterLow, "moderate/high/critical should increment suicide risk")
}

func TestRecordPHQ9Assessment(t *testing.T) {
	m := GetMetrics()

	// Record PHQ-9 assessments
	m.RecordPHQ9Assessment(3, "minimal", false)
	m.RecordPHQ9Assessment(8, "mild", false)
	m.RecordPHQ9Assessment(12, "moderate", false)
	m.RecordPHQ9Assessment(18, "moderately_severe", false)
	m.RecordPHQ9Assessment(24, "severe", false)

	// Verify severity counters
	assert.Greater(t, testutil.ToFloat64(m.PHQ9Assessments.WithLabelValues("minimal")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.PHQ9Assessments.WithLabelValues("severe")), float64(0))
}

func TestRecordPHQ9Assessment_WithSuicideRisk(t *testing.T) {
	m := GetMetrics()

	initialCount := testutil.ToFloat64(m.SuicideRiskDetected)

	// PHQ-9 with Q9 positive (suicide risk)
	m.RecordPHQ9Assessment(15, "moderately_severe", true)

	newCount := testutil.ToFloat64(m.SuicideRiskDetected)
	assert.Greater(t, newCount, initialCount, "PHQ-9 with suicide risk should increment counter")
}

func TestRecordGAD7Assessment(t *testing.T) {
	m := GetMetrics()

	// Record GAD-7 assessments
	m.RecordGAD7Assessment(3, "minimal")
	m.RecordGAD7Assessment(7, "mild")
	m.RecordGAD7Assessment(12, "moderate")
	m.RecordGAD7Assessment(18, "severe")

	// Verify severity counters
	assert.Greater(t, testutil.ToFloat64(m.GAD7Assessments.WithLabelValues("minimal")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.GAD7Assessments.WithLabelValues("severe")), float64(0))
}

func TestRecordAlertSent(t *testing.T) {
	m := GetMetrics()

	// Record alerts
	m.RecordAlertSent("critica", "push")
	m.RecordAlertSent("critica", "sms")
	m.RecordAlertSent("alta", "push")
	m.RecordAlertSent("media", "email")

	// Verify counters
	assert.Greater(t, testutil.ToFloat64(m.AlertsSent.WithLabelValues("critica", "push")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.AlertsSent.WithLabelValues("critica", "sms")), float64(0))
}

func TestRecordAlertDelivery(t *testing.T) {
	m := GetMetrics()

	// Record delivery latencies
	m.RecordAlertDelivery("push", 0.5)
	m.RecordAlertDelivery("sms", 2.0)
	m.RecordAlertDelivery("email", 1.5)

	// Histograms don't have simple assertions, but we verify no panic
}

func TestRecordToolInvocation(t *testing.T) {
	m := GetMetrics()

	// Record tool invocations
	m.RecordToolInvocation("search_memory", true, 0.1)
	m.RecordToolInvocation("send_alert", true, 0.5)
	m.RecordToolInvocation("schedule_appointment", false, 0.3)

	// Verify counters
	assert.Greater(t, testutil.ToFloat64(m.ToolInvocations.WithLabelValues("search_memory", "true")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.ToolInvocations.WithLabelValues("schedule_appointment", "false")), float64(0))
}

func TestRecordLLMRequest(t *testing.T) {
	m := GetMetrics()

	// Record LLM requests
	m.RecordLLMRequest("gemini-pro", true, 2.5, 100, 500)
	m.RecordLLMRequest("gemini-pro", true, 1.8, 200, 300)
	m.RecordLLMRequest("gemini-pro", false, 0.0, 0, 0)

	// Verify counters
	assert.Greater(t, testutil.ToFloat64(m.LLMRequestsTotal.WithLabelValues("gemini-pro", "true")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.LLMRequestsTotal.WithLabelValues("gemini-pro", "false")), float64(0))

	// Verify token counting
	assert.Greater(t, testutil.ToFloat64(m.LLMTokensUsed.WithLabelValues("gemini-pro", "input")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.LLMTokensUsed.WithLabelValues("gemini-pro", "output")), float64(0))
}

func TestRecordError(t *testing.T) {
	m := GetMetrics()

	// Record errors
	m.RecordError("connection_failed", "database")
	m.RecordError("timeout", "llm")
	m.RecordError("validation", "api")

	// Verify counters
	assert.Greater(t, testutil.ToFloat64(m.ErrorsTotal.WithLabelValues("connection_failed", "database")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.ErrorsTotal.WithLabelValues("timeout", "llm")), float64(0))
}

func TestHandler(t *testing.T) {
	handler := Handler()
	require.NotNil(t, handler, "Handler should return a valid HTTP handler")
}

func TestActiveMetrics(t *testing.T) {
	m := GetMetrics()

	// Test gauge metrics
	m.ActiveWebSocketConnections.Set(5)
	assert.Equal(t, float64(5), testutil.ToFloat64(m.ActiveWebSocketConnections))

	m.ActiveWebSocketConnections.Inc()
	assert.Equal(t, float64(6), testutil.ToFloat64(m.ActiveWebSocketConnections))

	m.ActiveWebSocketConnections.Dec()
	assert.Equal(t, float64(5), testutil.ToFloat64(m.ActiveWebSocketConnections))

	m.ActivePatients.Set(10)
	assert.Equal(t, float64(10), testutil.ToFloat64(m.ActivePatients))

	m.ActiveAlerts.Set(3)
	assert.Equal(t, float64(3), testutil.ToFloat64(m.ActiveAlerts))
}

func TestConversationMetrics(t *testing.T) {
	m := GetMetrics()

	initialConversations := testutil.ToFloat64(m.ConversationsTotal)

	m.ConversationsTotal.Inc()
	m.ConversationsTotal.Inc()

	assert.Equal(t, initialConversations+2, testutil.ToFloat64(m.ConversationsTotal))

	// Record conversation duration
	m.ConversationDuration.Observe(300) // 5 minutes
	m.ConversationDuration.Observe(600) // 10 minutes
}

func TestMessageMetrics(t *testing.T) {
	m := GetMetrics()

	// Record messages by type
	m.MessagesProcessed.WithLabelValues("user").Inc()
	m.MessagesProcessed.WithLabelValues("user").Inc()
	m.MessagesProcessed.WithLabelValues("assistant").Inc()
	m.MessagesProcessed.WithLabelValues("system").Inc()

	assert.Greater(t, testutil.ToFloat64(m.MessagesProcessed.WithLabelValues("user")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.MessagesProcessed.WithLabelValues("assistant")), float64(0))
}

func TestMemoryMetrics(t *testing.T) {
	m := GetMetrics()

	// Record memory operations
	m.MemoryOperations.WithLabelValues("store").Inc()
	m.MemoryOperations.WithLabelValues("retrieve").Inc()
	m.MemoryOperations.WithLabelValues("search").Inc()

	assert.Greater(t, testutil.ToFloat64(m.MemoryOperations.WithLabelValues("store")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.MemoryOperations.WithLabelValues("retrieve")), float64(0))

	// Record latencies
	m.MemoryRetrievalLatency.Observe(0.05)
	m.VectorSearchLatency.Observe(0.1)
	m.Neo4jQueryLatency.Observe(0.08)
}

func TestAPIMetrics(t *testing.T) {
	m := GetMetrics()

	// Record API requests
	m.APIRequestsTotal.WithLabelValues("/api/chat", "POST", "200").Inc()
	m.APIRequestsTotal.WithLabelValues("/api/chat", "POST", "200").Inc()
	m.APIRequestsTotal.WithLabelValues("/api/health", "GET", "200").Inc()
	m.APIRequestsTotal.WithLabelValues("/api/chat", "POST", "500").Inc()

	assert.Greater(t, testutil.ToFloat64(m.APIRequestsTotal.WithLabelValues("/api/chat", "POST", "200")), float64(0))
	assert.Greater(t, testutil.ToFloat64(m.APIRequestsTotal.WithLabelValues("/api/chat", "POST", "500")), float64(0))

	// Record durations
	m.APIRequestDuration.WithLabelValues("/api/chat", "POST").Observe(0.5)
	m.APIRequestDuration.WithLabelValues("/api/health", "GET").Observe(0.01)
}
