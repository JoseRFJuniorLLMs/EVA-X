package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Memory metrics
	MemoryTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_memory_total",
		Help: "Total number of memories stored",
	})

	MemoryImportanceAvg = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_memory_importance_avg",
		Help: "Average importance score of memories",
	})

	// Retrieval metrics
	RetrievalLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "eva_retrieval_latency_seconds",
		Help:    "Latency of memory retrieval operations",
		Buckets: prometheus.DefBuckets,
	})

	RetrievalTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eva_retrieval_total",
			Help: "Total number of retrieval operations",
		},
		[]string{"status"}, // success, error
	)

	// Krylov metrics
	KrylovDimension = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_krylov_dimension",
		Help: "Current dimension of Krylov subspace",
	})

	KrylovCompressionRatio = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_krylov_compression_ratio",
		Help: "Compression ratio (original_dim / krylov_dim)",
	})

	// Neo4j metrics
	Neo4jConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_neo4j_connections",
		Help: "Number of connections in Neo4j graph",
	})

	Neo4jNodes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_neo4j_nodes",
		Help: "Number of nodes in Neo4j graph",
	})

	// Request metrics
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eva_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "eva_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Consolidation metrics
	ConsolidationCycles = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eva_consolidation_cycles_total",
		Help: "Total number of REM consolidation cycles",
	})

	ConsolidationMemoriesPruned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eva_consolidation_memories_pruned_total",
		Help: "Total number of memories pruned during consolidation",
	})

	// Synaptic pruning metrics
	SynapticPruningCycles = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eva_synaptic_pruning_cycles_total",
		Help: "Total number of synaptic pruning cycles",
	})

	SynapticPruningEdgesPruned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "eva_synaptic_pruning_edges_pruned_total",
		Help: "Total number of edges pruned",
	})

	// Atomic facts metrics
	AtomicFactsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_atomic_facts_total",
		Help: "Total number of atomic facts",
	})

	AtomicFactsConfidenceAvg = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "eva_atomic_facts_confidence_avg",
		Help: "Average confidence score of atomic facts",
	})

	// MCP metrics
	MCPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eva_mcp_requests_total",
			Help: "Total number of MCP requests",
		},
		[]string{"endpoint", "status"},
	)
)

// PrometheusHandler returns HTTP handler for Prometheus metrics
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// RecordRetrieval records a memory retrieval operation
func RecordRetrieval(duration time.Duration, success bool) {
	RetrievalLatency.Observe(duration.Seconds())

	status := "success"
	if !success {
		status = "error"
	}
	RetrievalTotal.WithLabelValues(status).Inc()
}

// RecordRequest records an HTTP request
func RecordRequest(method, endpoint string, status int, duration time.Duration) {
	RequestsTotal.WithLabelValues(method, endpoint, http.StatusText(status)).Inc()
	RequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordConsolidation records a consolidation cycle
func RecordConsolidation(memoriesPruned int) {
	ConsolidationCycles.Inc()
	ConsolidationMemoriesPruned.Add(float64(memoriesPruned))
}

// RecordSynapticPruning records a synaptic pruning cycle
func RecordSynapticPruning(edgesPruned int) {
	SynapticPruningCycles.Inc()
	SynapticPruningEdgesPruned.Add(float64(edgesPruned))
}
