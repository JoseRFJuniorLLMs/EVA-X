// Package benchmark implements a LongMemEval-inspired benchmark framework for EVA-Memory retrieval.
// Reference: Zep arXiv:2501.13956 (2025) - Temporal Knowledge Graph benchmarks
package benchmark

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// QueryType categorizes benchmark queries
type QueryType string

const (
	QueryTemporal QueryType = "temporal" // "o que aconteceu semana passada?"
	QueryEntity   QueryType = "entity"   // "quem é Maria?"
	QuerySemantic QueryType = "semantic" // "como está a saúde?"
	QueryMixed    QueryType = "mixed"    // paraphrases, negation, multi-hop
)

// SyntheticMemory represents a test memory with known ground truth
type SyntheticMemory struct {
	ID        string
	Content   string
	Speaker   string
	Timestamp time.Time
	EventDate time.Time
	Emotion   string
	Topics    []string
	IsAtomic  bool
}

// BenchmarkQuery represents a test query with expected results
type BenchmarkQuery struct {
	ID              string
	Query           string
	Type            QueryType
	ExpectedIDs     []string // Ground truth: IDs of relevant memories
	TemporalContext string   // e.g., "last_week", "today", "last_month"
}

// RetrievalFunc is the function signature that benchmark tests against.
// Returns list of (memoryID, similarity) pairs.
type RetrievalFunc func(ctx context.Context, query string, k int) ([]RetrievedItem, error)

// RetrievedItem is a single retrieval result
type RetrievedItem struct {
	ID         string
	Similarity float64
}

// BenchmarkReport holds all benchmark results
type BenchmarkReport struct {
	Timestamp    time.Time              `json:"timestamp"`
	TotalQueries int                    `json:"total_queries"`
	Metrics      GlobalMetrics          `json:"metrics"`
	ByType       map[QueryType]*Metrics `json:"by_type"`
	Latency      LatencyStats           `json:"latency"`
	Queries      []QueryResult          `json:"queries,omitempty"` // detailed per-query (optional)
}

// GlobalMetrics are aggregate metrics across all queries
type GlobalMetrics struct {
	RecallAt5  float64 `json:"recall_at_5"`
	RecallAt10 float64 `json:"recall_at_10"`
	RecallAt20 float64 `json:"recall_at_20"`
	MRR        float64 `json:"mrr"` // Mean Reciprocal Rank
}

// Metrics for a specific query type
type Metrics struct {
	Count      int     `json:"count"`
	RecallAt5  float64 `json:"recall_at_5"`
	RecallAt10 float64 `json:"recall_at_10"`
	RecallAt20 float64 `json:"recall_at_20"`
	Precision5 float64 `json:"precision_at_5"`
	MRR        float64 `json:"mrr"`
}

// LatencyStats holds latency percentiles
type LatencyStats struct {
	P50 time.Duration `json:"p50"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
	Avg time.Duration `json:"avg"`
}

// QueryResult holds per-query detail
type QueryResult struct {
	QueryID      string        `json:"query_id"`
	Query        string        `json:"query"`
	Type         QueryType     `json:"type"`
	RecallAt5    float64       `json:"recall_at_5"`
	RecallAt10   float64       `json:"recall_at_10"`
	MRR          float64       `json:"mrr"`
	Latency      time.Duration `json:"latency"`
	RetrievedIDs []string      `json:"retrieved_ids"`
	ExpectedIDs  []string      `json:"expected_ids"`
}

// DiffReport compares two benchmark reports
type DiffReport struct {
	Before     GlobalMetrics `json:"before"`
	After      GlobalMetrics `json:"after"`
	DeltaR5    float64       `json:"delta_recall_at_5"`
	DeltaR10   float64       `json:"delta_recall_at_10"`
	DeltaR20   float64       `json:"delta_recall_at_20"`
	DeltaMRR   float64       `json:"delta_mrr"`
	Improved   bool          `json:"improved"`
	LatencyDiff time.Duration `json:"latency_diff"`
}

// GenerateSyntheticDataset creates a test dataset of elderly care conversations
func GenerateSyntheticDataset() ([]SyntheticMemory, []BenchmarkQuery) {
	now := time.Now()
	day := 24 * time.Hour

	memories := []SyntheticMemory{
		// --- Temporal memories (recent events) ---
		{ID: "m001", Content: "Dona Maria tomou o remédio da pressão às 8h da manhã", Speaker: "user", Timestamp: now.Add(-1 * day), EventDate: now.Add(-1 * day), Emotion: "", Topics: []string{"medicamento", "pressão"}, IsAtomic: true},
		{ID: "m002", Content: "O filho Carlos visitou ontem e trouxe frutas", Speaker: "user", Timestamp: now.Add(-2 * day), EventDate: now.Add(-2 * day), Emotion: "alegria", Topics: []string{"família", "visita"}, IsAtomic: true},
		{ID: "m003", Content: "Fui ao médico na semana passada e ele disse que a glicose está alta", Speaker: "user", Timestamp: now.Add(-5 * day), EventDate: now.Add(-5 * day), Emotion: "preocupação", Topics: []string{"saúde", "médico", "glicose"}, IsAtomic: true},
		{ID: "m004", Content: "Ontem choveu muito e não consegui ir à farmácia", Speaker: "user", Timestamp: now.Add(-1 * day), EventDate: now.Add(-1 * day), Emotion: "frustração", Topics: []string{"farmácia", "clima"}, IsAtomic: true},
		{ID: "m005", Content: "Amanhã tenho consulta com a cardiologista Dra. Ana", Speaker: "user", Timestamp: now, EventDate: now.Add(1 * day), Emotion: "", Topics: []string{"consulta", "cardiologista"}, IsAtomic: true},

		// --- Entity memories (people, places) ---
		{ID: "m010", Content: "Maria é minha neta, tem 8 anos e adora desenhar", Speaker: "user", Timestamp: now.Add(-10 * day), EventDate: now.Add(-10 * day), Emotion: "amor", Topics: []string{"família", "neta"}, IsAtomic: true},
		{ID: "m011", Content: "Meu amigo João mora em Belo Horizonte e sempre me liga aos domingos", Speaker: "user", Timestamp: now.Add(-15 * day), EventDate: now.Add(-15 * day), Emotion: "saudade", Topics: []string{"amizade", "telefone"}, IsAtomic: true},
		{ID: "m012", Content: "A fisioterapeuta Paula vem toda terça e quinta", Speaker: "user", Timestamp: now.Add(-3 * day), EventDate: now.Add(-3 * day), Emotion: "", Topics: []string{"fisioterapia", "rotina"}, IsAtomic: true},
		{ID: "m013", Content: "Carlos é meu filho mais velho, trabalha no banco", Speaker: "user", Timestamp: now.Add(-20 * day), EventDate: now.Add(-20 * day), Emotion: "orgulho", Topics: []string{"família", "filho"}, IsAtomic: true},
		{ID: "m014", Content: "A vizinha Dona Lúcia trouxe um bolo de milho ontem", Speaker: "user", Timestamp: now.Add(-1 * day), EventDate: now.Add(-1 * day), Emotion: "alegria", Topics: []string{"vizinhança", "comida"}, IsAtomic: true},

		// --- Semantic memories (health, routines) ---
		{ID: "m020", Content: "Tomo losartana 50mg de manhã e metformina 500mg após o almoço", Speaker: "user", Timestamp: now.Add(-7 * day), EventDate: now.Add(-7 * day), Emotion: "", Topics: []string{"medicamento", "rotina"}, IsAtomic: true},
		{ID: "m021", Content: "Não consigo dormir bem, fico pensando em coisas tristes", Speaker: "user", Timestamp: now.Add(-4 * day), EventDate: now.Add(-4 * day), Emotion: "tristeza", Topics: []string{"sono", "humor"}, IsAtomic: false},
		{ID: "m022", Content: "Minha pressão estava 14 por 9 quando medi hoje cedo", Speaker: "user", Timestamp: now, EventDate: now, Emotion: "preocupação", Topics: []string{"pressão", "saúde"}, IsAtomic: true},
		{ID: "m023", Content: "Gosto de caminhar no parque de manhã quando não está muito quente", Speaker: "user", Timestamp: now.Add(-12 * day), EventDate: now.Add(-12 * day), Emotion: "alegria", Topics: []string{"exercício", "lazer"}, IsAtomic: false},
		{ID: "m024", Content: "O doutor disse que preciso diminuir o sal na comida", Speaker: "user", Timestamp: now.Add(-5 * day), EventDate: now.Add(-5 * day), Emotion: "", Topics: []string{"dieta", "saúde"}, IsAtomic: true},

		// --- Mixed/complex memories ---
		{ID: "m030", Content: "Quando eu era criança, minha avó fazia um café com leite que nunca mais encontrei igual", Speaker: "user", Timestamp: now.Add(-30 * day), EventDate: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC), Emotion: "saudade", Topics: []string{"infância", "avó", "memória"}, IsAtomic: false},
		{ID: "m031", Content: "Meu marido faleceu há 3 anos, ainda sinto muita falta dele", Speaker: "user", Timestamp: now.Add(-8 * day), EventDate: now.Add(-3 * 365 * day), Emotion: "tristeza", Topics: []string{"luto", "marido"}, IsAtomic: false},
		{ID: "m032", Content: "A festa de aniversário da Maria foi muito bonita, ela ficou tão feliz", Speaker: "user", Timestamp: now.Add(-10 * day), EventDate: now.Add(-10 * day), Emotion: "alegria", Topics: []string{"família", "neta", "aniversário"}, IsAtomic: false},
		{ID: "m033", Content: "Hoje de manhã esqueci de tomar o remédio, só lembrei depois do almoço", Speaker: "user", Timestamp: now, EventDate: now, Emotion: "preocupação", Topics: []string{"medicamento", "esquecimento"}, IsAtomic: true},
		{ID: "m034", Content: "Não gosto de ir ao hospital, me dá muita ansiedade", Speaker: "user", Timestamp: now.Add(-6 * day), EventDate: now.Add(-6 * day), Emotion: "ansiedade", Topics: []string{"hospital", "medo"}, IsAtomic: false},
	}

	queries := []BenchmarkQuery{
		// Temporal queries
		{ID: "q001", Query: "o que aconteceu ontem?", Type: QueryTemporal, ExpectedIDs: []string{"m001", "m004", "m014"}, TemporalContext: "yesterday"},
		{ID: "q002", Query: "quando foi a última visita do Carlos?", Type: QueryTemporal, ExpectedIDs: []string{"m002", "m013"}, TemporalContext: "last_week"},
		{ID: "q003", Query: "qual foi o resultado do médico semana passada?", Type: QueryTemporal, ExpectedIDs: []string{"m003", "m024"}, TemporalContext: "last_week"},
		{ID: "q004", Query: "tem alguma consulta marcada?", Type: QueryTemporal, ExpectedIDs: []string{"m005"}, TemporalContext: "future"},
		{ID: "q005", Query: "como estava a pressão hoje?", Type: QueryTemporal, ExpectedIDs: []string{"m022", "m001"}, TemporalContext: "today"},

		// Entity queries
		{ID: "q010", Query: "quem é Maria?", Type: QueryEntity, ExpectedIDs: []string{"m010", "m032"}},
		{ID: "q011", Query: "quem é o Carlos?", Type: QueryEntity, ExpectedIDs: []string{"m013", "m002"}},
		{ID: "q012", Query: "quem é a fisioterapeuta?", Type: QueryEntity, ExpectedIDs: []string{"m012"}},
		{ID: "q013", Query: "me fale sobre o João", Type: QueryEntity, ExpectedIDs: []string{"m011"}},
		{ID: "q014", Query: "quem é Dona Lúcia?", Type: QueryEntity, ExpectedIDs: []string{"m014"}},

		// Semantic queries
		{ID: "q020", Query: "quais remédios eu tomo?", Type: QuerySemantic, ExpectedIDs: []string{"m020", "m001", "m033"}},
		{ID: "q021", Query: "como estou dormindo?", Type: QuerySemantic, ExpectedIDs: []string{"m021"}},
		{ID: "q022", Query: "como está minha saúde?", Type: QuerySemantic, ExpectedIDs: []string{"m003", "m022", "m024"}},
		{ID: "q023", Query: "quais exercícios eu faço?", Type: QuerySemantic, ExpectedIDs: []string{"m023", "m012"}},
		{ID: "q024", Query: "o que o médico recomendou?", Type: QuerySemantic, ExpectedIDs: []string{"m024", "m003"}},

		// Mixed/hard queries
		{ID: "q030", Query: "conte uma lembrança da sua infância", Type: QueryMixed, ExpectedIDs: []string{"m030"}},
		{ID: "q031", Query: "o que te deixa triste?", Type: QueryMixed, ExpectedIDs: []string{"m031", "m021"}},
		{ID: "q032", Query: "o que aconteceu na festa?", Type: QueryMixed, ExpectedIDs: []string{"m032"}},
		{ID: "q033", Query: "esqueci alguma coisa hoje?", Type: QueryMixed, ExpectedIDs: []string{"m033"}},
		{ID: "q034", Query: "o que te dá medo?", Type: QueryMixed, ExpectedIDs: []string{"m034"}},
	}

	return memories, queries
}

// RunBenchmark executes the full benchmark suite
func RunBenchmark(ctx context.Context, retrieveFunc RetrievalFunc, queries []BenchmarkQuery) (*BenchmarkReport, error) {
	report := &BenchmarkReport{
		Timestamp:    time.Now(),
		TotalQueries: len(queries),
		ByType:       make(map[QueryType]*Metrics),
	}

	var latencies []time.Duration
	typeResults := make(map[QueryType][]QueryResult)

	for _, q := range queries {
		start := time.Now()

		items, err := retrieveFunc(ctx, q.Query, 20)
		latency := time.Since(start)
		latencies = append(latencies, latency)

		if err != nil {
			continue
		}

		retrievedIDs := make([]string, len(items))
		for i, item := range items {
			retrievedIDs[i] = item.ID
		}

		qr := QueryResult{
			QueryID:      q.ID,
			Query:        q.Query,
			Type:         q.Type,
			RecallAt5:    RecallAtK(q.ExpectedIDs, retrievedIDs, 5),
			RecallAt10:   RecallAtK(q.ExpectedIDs, retrievedIDs, 10),
			MRR:          MeanReciprocalRank(q.ExpectedIDs, retrievedIDs),
			Latency:      latency,
			RetrievedIDs: retrievedIDs,
			ExpectedIDs:  q.ExpectedIDs,
		}

		report.Queries = append(report.Queries, qr)
		typeResults[q.Type] = append(typeResults[q.Type], qr)
	}

	// Aggregate global metrics
	report.Metrics = aggregateGlobalMetrics(report.Queries)
	report.Latency = computeLatencyStats(latencies)

	// Aggregate per-type metrics
	for qtype, results := range typeResults {
		m := &Metrics{Count: len(results)}
		for _, r := range results {
			m.RecallAt5 += r.RecallAt5
			m.RecallAt10 += r.RecallAt10
			m.MRR += r.MRR
		}
		if m.Count > 0 {
			m.RecallAt5 /= float64(m.Count)
			m.RecallAt10 /= float64(m.Count)
			m.MRR /= float64(m.Count)
		}
		report.ByType[qtype] = m
	}

	return report, nil
}

// CompareBenchmarks returns a diff between two reports
func CompareBenchmarks(before, after *BenchmarkReport) *DiffReport {
	diff := &DiffReport{
		Before:      before.Metrics,
		After:       after.Metrics,
		DeltaR5:     after.Metrics.RecallAt5 - before.Metrics.RecallAt5,
		DeltaR10:    after.Metrics.RecallAt10 - before.Metrics.RecallAt10,
		DeltaR20:    after.Metrics.RecallAt20 - before.Metrics.RecallAt20,
		DeltaMRR:    after.Metrics.MRR - before.Metrics.MRR,
		LatencyDiff: after.Latency.P50 - before.Latency.P50,
	}
	diff.Improved = diff.DeltaMRR > 0 && diff.DeltaR10 >= 0
	return diff
}

// RecallAtK computes recall@K: fraction of relevant items in top-K results
func RecallAtK(expected, retrieved []string, k int) float64 {
	if len(expected) == 0 {
		return 1.0 // no relevant items = perfect recall trivially
	}

	topK := retrieved
	if k < len(retrieved) {
		topK = retrieved[:k]
	}

	expectedSet := make(map[string]bool, len(expected))
	for _, id := range expected {
		expectedSet[id] = true
	}

	hits := 0
	for _, id := range topK {
		if expectedSet[id] {
			hits++
		}
	}

	return float64(hits) / float64(len(expected))
}

// PrecisionAtK computes precision@K: fraction of top-K results that are relevant
func PrecisionAtK(expected, retrieved []string, k int) float64 {
	topK := retrieved
	if k < len(retrieved) {
		topK = retrieved[:k]
	}
	if len(topK) == 0 {
		return 0.0
	}

	expectedSet := make(map[string]bool, len(expected))
	for _, id := range expected {
		expectedSet[id] = true
	}

	hits := 0
	for _, id := range topK {
		if expectedSet[id] {
			hits++
		}
	}

	return float64(hits) / float64(len(topK))
}

// MeanReciprocalRank computes MRR: 1/rank of first relevant result
func MeanReciprocalRank(expected, retrieved []string) float64 {
	expectedSet := make(map[string]bool, len(expected))
	for _, id := range expected {
		expectedSet[id] = true
	}

	for i, id := range retrieved {
		if expectedSet[id] {
			return 1.0 / float64(i+1)
		}
	}

	return 0.0
}

func aggregateGlobalMetrics(queries []QueryResult) GlobalMetrics {
	if len(queries) == 0 {
		return GlobalMetrics{}
	}

	var m GlobalMetrics
	for _, q := range queries {
		m.RecallAt5 += q.RecallAt5
		m.RecallAt10 += q.RecallAt10
		m.MRR += q.MRR
	}

	n := float64(len(queries))
	m.RecallAt5 /= n
	m.RecallAt10 /= n
	m.MRR /= n

	return m
}

func computeLatencyStats(latencies []time.Duration) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var total time.Duration
	for _, l := range latencies {
		total += l
	}

	n := len(latencies)
	return LatencyStats{
		P50: latencies[n*50/100],
		P95: latencies[n*95/100],
		P99: latencies[int(math.Min(float64(n*99/100), float64(n-1)))],
		Avg: total / time.Duration(n),
	}
}

// FormatReport returns a human-readable summary
func FormatReport(r *BenchmarkReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== EVA-Memory Benchmark Report (%s) ===\n", r.Timestamp.Format("2006-01-02 15:04")))
	sb.WriteString(fmt.Sprintf("Queries: %d\n\n", r.TotalQueries))
	sb.WriteString(fmt.Sprintf("GLOBAL METRICS:\n"))
	sb.WriteString(fmt.Sprintf("  Recall@5:  %.1f%%\n", r.Metrics.RecallAt5*100))
	sb.WriteString(fmt.Sprintf("  Recall@10: %.1f%%\n", r.Metrics.RecallAt10*100))
	sb.WriteString(fmt.Sprintf("  MRR:       %.3f\n\n", r.Metrics.MRR))
	sb.WriteString(fmt.Sprintf("LATENCY:\n"))
	sb.WriteString(fmt.Sprintf("  P50: %v | P95: %v | P99: %v | Avg: %v\n\n", r.Latency.P50, r.Latency.P95, r.Latency.P99, r.Latency.Avg))

	for qtype, m := range r.ByType {
		sb.WriteString(fmt.Sprintf("  [%s] (n=%d) Recall@5=%.1f%% Recall@10=%.1f%% MRR=%.3f\n",
			qtype, m.Count, m.RecallAt5*100, m.RecallAt10*100, m.MRR))
	}

	return sb.String()
}
