package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecallAtK(t *testing.T) {
	tests := []struct {
		name      string
		expected  []string
		retrieved []string
		k         int
		want      float64
	}{
		{
			name:      "perfect recall at 5",
			expected:  []string{"a", "b"},
			retrieved: []string{"a", "b", "c", "d", "e"},
			k:         5,
			want:      1.0,
		},
		{
			name:      "half recall at 5",
			expected:  []string{"a", "b", "c", "d"},
			retrieved: []string{"a", "b", "x", "y", "z"},
			k:         5,
			want:      0.5,
		},
		{
			name:      "zero recall",
			expected:  []string{"a", "b"},
			retrieved: []string{"x", "y", "z"},
			k:         3,
			want:      0.0,
		},
		{
			name:      "no expected items",
			expected:  []string{},
			retrieved: []string{"x", "y"},
			k:         5,
			want:      1.0,
		},
		{
			name:      "k larger than retrieved",
			expected:  []string{"a"},
			retrieved: []string{"a"},
			k:         10,
			want:      1.0,
		},
		{
			name:      "recall at 3 misses item at position 4",
			expected:  []string{"a", "d"},
			retrieved: []string{"x", "y", "z", "d", "a"},
			k:         3,
			want:      0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecallAtK(tt.expected, tt.retrieved, tt.k)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestPrecisionAtK(t *testing.T) {
	tests := []struct {
		name      string
		expected  []string
		retrieved []string
		k         int
		want      float64
	}{
		{
			name:      "all relevant",
			expected:  []string{"a", "b", "c"},
			retrieved: []string{"a", "b", "c"},
			k:         3,
			want:      1.0,
		},
		{
			name:      "half relevant",
			expected:  []string{"a", "b"},
			retrieved: []string{"a", "x", "b", "y"},
			k:         4,
			want:      0.5,
		},
		{
			name:      "none relevant",
			expected:  []string{"a"},
			retrieved: []string{"x", "y", "z"},
			k:         3,
			want:      0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrecisionAtK(tt.expected, tt.retrieved, tt.k)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestMeanReciprocalRank(t *testing.T) {
	tests := []struct {
		name      string
		expected  []string
		retrieved []string
		want      float64
	}{
		{
			name:      "first result relevant",
			expected:  []string{"a"},
			retrieved: []string{"a", "b", "c"},
			want:      1.0,
		},
		{
			name:      "second result relevant",
			expected:  []string{"b"},
			retrieved: []string{"x", "b", "c"},
			want:      0.5,
		},
		{
			name:      "third result relevant",
			expected:  []string{"c"},
			retrieved: []string{"x", "y", "c"},
			want:      1.0 / 3.0,
		},
		{
			name:      "no relevant results",
			expected:  []string{"a"},
			retrieved: []string{"x", "y", "z"},
			want:      0.0,
		},
		{
			name:      "multiple expected first at pos 2",
			expected:  []string{"a", "b"},
			retrieved: []string{"x", "a", "b"},
			want:      0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeanReciprocalRank(tt.expected, tt.retrieved)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestGenerateSyntheticDataset(t *testing.T) {
	memories, queries := GenerateSyntheticDataset()

	assert.GreaterOrEqual(t, len(memories), 20, "should have at least 20 memories")
	assert.GreaterOrEqual(t, len(queries), 20, "should have at least 20 queries")

	// Verify all expected IDs reference actual memories
	memIDs := make(map[string]bool)
	for _, m := range memories {
		memIDs[m.ID] = true
	}

	for _, q := range queries {
		for _, eid := range q.ExpectedIDs {
			assert.True(t, memIDs[eid], "query %s references non-existent memory %s", q.ID, eid)
		}
	}

	// Verify query type coverage
	typeCounts := make(map[QueryType]int)
	for _, q := range queries {
		typeCounts[q.Type]++
	}
	assert.GreaterOrEqual(t, typeCounts[QueryTemporal], 3)
	assert.GreaterOrEqual(t, typeCounts[QueryEntity], 3)
	assert.GreaterOrEqual(t, typeCounts[QuerySemantic], 3)
	assert.GreaterOrEqual(t, typeCounts[QueryMixed], 3)
}

func TestRunBenchmark(t *testing.T) {
	_, queries := GenerateSyntheticDataset()

	// Mock retrieval: always return first 2 expected IDs
	mockRetrieve := func(ctx context.Context, query string, k int) ([]RetrievedItem, error) {
		for _, q := range queries {
			if q.Query == query {
				var items []RetrievedItem
				for i, id := range q.ExpectedIDs {
					if i >= k {
						break
					}
					items = append(items, RetrievedItem{ID: id, Similarity: 0.9 - float64(i)*0.1})
				}
				return items, nil
			}
		}
		return nil, nil
	}

	ctx := context.Background()
	report, err := RunBenchmark(ctx, mockRetrieve, queries)
	require.NoError(t, err)

	assert.Equal(t, len(queries), report.TotalQueries)
	assert.Greater(t, report.Metrics.RecallAt5, 0.0)
	assert.Greater(t, report.Metrics.MRR, 0.0)
	assert.NotEmpty(t, report.Queries)
}

func TestCompareBenchmarks(t *testing.T) {
	before := &BenchmarkReport{
		Metrics: GlobalMetrics{RecallAt5: 0.5, RecallAt10: 0.6, RecallAt20: 0.7, MRR: 0.4},
		Latency: LatencyStats{P50: 10 * time.Millisecond},
	}
	after := &BenchmarkReport{
		Metrics: GlobalMetrics{RecallAt5: 0.7, RecallAt10: 0.8, RecallAt20: 0.85, MRR: 0.6},
		Latency: LatencyStats{P50: 8 * time.Millisecond},
	}

	diff := CompareBenchmarks(before, after)
	assert.InDelta(t, 0.2, diff.DeltaR5, 0.001)
	assert.InDelta(t, 0.2, diff.DeltaR10, 0.001)
	assert.InDelta(t, 0.2, diff.DeltaMRR, 0.001)
	assert.True(t, diff.Improved)
}

func TestFormatReport(t *testing.T) {
	report := &BenchmarkReport{
		Timestamp:    time.Now(),
		TotalQueries: 20,
		Metrics:      GlobalMetrics{RecallAt5: 0.75, RecallAt10: 0.85, MRR: 0.65},
		Latency:      LatencyStats{P50: 5 * time.Millisecond, P95: 15 * time.Millisecond, P99: 25 * time.Millisecond, Avg: 7 * time.Millisecond},
		ByType:       map[QueryType]*Metrics{QueryTemporal: {Count: 5, RecallAt5: 0.8, RecallAt10: 0.9, MRR: 0.7}},
	}

	output := FormatReport(report)
	assert.Contains(t, output, "EVA-Memory Benchmark Report")
	assert.Contains(t, output, "Recall@5")
	assert.Contains(t, output, "temporal")
}
