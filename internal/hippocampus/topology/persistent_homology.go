package topology

import (
	"fmt"
	"math"
)

// TopologicalAnalysis represents the topological structure of memory/narrative
type TopologicalAnalysis struct {
	Holes        []Hole
	Persistence  []PersistencePair
	BettiNumbers []int // β0, β1, β2, ... (connected components, holes, voids)
}

// Hole represents a topological "hole" in the memory graph
type Hole struct {
	Dimension     int     // 0=component, 1=loop, 2=void
	Birth         float64 // When the hole appears
	Death         float64 // When the hole fills
	Persistence   float64 // Death - Birth (how "significant" the hole is)
	Significance  float64 // 0-1: normalized significance
	PossibleCause string  // "trauma", "repressao", "evitacao", "normal_gap"
}

// PersistencePair represents a birth-death pair in persistent homology
type PersistencePair struct {
	Dimension   int
	Birth       float64
	Death       float64
	Persistence float64
}

// MemoryPoint represents a memory in topological space
type MemoryPoint struct {
	ID        string
	Timestamp float64 // Normalized time
	Emotion   float64 // Emotional valence (-1 to +1)
	Intensity float64 // 0-1
}

// AnalyzeMemoryTopology analyzes the topological structure of a user's memories
func AnalyzeMemoryTopology(memories []MemoryPoint) TopologicalAnalysis {
	// Build distance matrix
	distMatrix := buildDistanceMatrix(memories)

	// Compute persistent homology (simplified Vietoris-Rips complex)
	persistence := computePersistentHomology(distMatrix)

	// Detect holes
	holes := detectHoles(persistence)

	// Calculate Betti numbers
	bettiNumbers := calculateBettiNumbers(persistence)

	return TopologicalAnalysis{
		Holes:        holes,
		Persistence:  persistence,
		BettiNumbers: bettiNumbers,
	}
}

// DetectTraumaHoles identifies holes that may indicate trauma or repression
func DetectTraumaHoles(analysis TopologicalAnalysis) []Hole {
	traumaHoles := []Hole{}

	for _, hole := range analysis.Holes {
		// Significant holes (high persistence) may indicate trauma
		if hole.Persistence > 0.3 && hole.Dimension == 1 {
			// 1-dimensional holes (loops) often indicate avoidance patterns
			hole.PossibleCause = classifyHoleCause(hole)
			if hole.PossibleCause == "trauma" || hole.PossibleCause == "repressao" {
				traumaHoles = append(traumaHoles, hole)
			}
		}
	}

	return traumaHoles
}

// MapNarrativeStructure maps the topological structure of a narrative
func MapNarrativeStructure(transcript string) TopologicalAnalysis {
	// Extract semantic segments from transcript
	segments := extractSemanticSegments(transcript)

	// Convert to memory points
	memories := []MemoryPoint{}
	for i, segment := range segments {
		memories = append(memories, MemoryPoint{
			ID:        fmt.Sprintf("segment_%d", i),
			Timestamp: float64(i),
			Emotion:   segment.Emotion,
			Intensity: segment.Intensity,
		})
	}

	return AnalyzeMemoryTopology(memories)
}

// Helper functions

func buildDistanceMatrix(memories []MemoryPoint) [][]float64 {
	n := len(memories)
	matrix := make([][]float64, n)

	for i := 0; i < n; i++ {
		matrix[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 0
			} else {
				matrix[i][j] = memoryDistance(memories[i], memories[j])
			}
		}
	}

	return matrix
}

func memoryDistance(a, b MemoryPoint) float64 {
	// Euclidean distance in (time, emotion, intensity) space
	timeDiff := a.Timestamp - b.Timestamp
	emotionDiff := a.Emotion - b.Emotion
	intensityDiff := a.Intensity - b.Intensity

	return math.Sqrt(timeDiff*timeDiff + emotionDiff*emotionDiff + intensityDiff*intensityDiff)
}

func computePersistentHomology(distMatrix [][]float64) []PersistencePair {
	// Simplified persistent homology computation
	// In production, would use GUDHI or Ripser library
	pairs := []PersistencePair{}

	n := len(distMatrix)

	// Find connected components (0-dimensional features)
	// Simplified: assume all points eventually connect
	pairs = append(pairs, PersistencePair{
		Dimension:   0,
		Birth:       0.0,
		Death:       math.Inf(1),
		Persistence: math.Inf(1),
	})

	// Find 1-dimensional holes (loops)
	// Simplified heuristic: look for triangular gaps in distance matrix
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				// Check if i, j, k form a "hole"
				dij := distMatrix[i][j]
				djk := distMatrix[j][k]
				dki := distMatrix[k][i]

				// If distances are similar, might be a loop
				avgDist := (dij + djk + dki) / 3.0
				variance := ((dij-avgDist)*(dij-avgDist) +
					(djk-avgDist)*(djk-avgDist) +
					(dki-avgDist)*(dki-avgDist)) / 3.0

				if variance < 0.1 && avgDist > 0.5 {
					// Potential hole
					birth := math.Min(math.Min(dij, djk), dki)
					death := math.Max(math.Max(dij, djk), dki)

					pairs = append(pairs, PersistencePair{
						Dimension:   1,
						Birth:       birth,
						Death:       death,
						Persistence: death - birth,
					})
				}
			}
		}
	}

	return pairs
}

func detectHoles(persistence []PersistencePair) []Hole {
	holes := []Hole{}

	for _, pair := range persistence {
		if pair.Dimension == 0 {
			continue // Skip connected components
		}

		// Only significant holes (persistence > threshold)
		if pair.Persistence > 0.2 {
			significance := math.Min(pair.Persistence, 1.0)

			holes = append(holes, Hole{
				Dimension:     pair.Dimension,
				Birth:         pair.Birth,
				Death:         pair.Death,
				Persistence:   pair.Persistence,
				Significance:  significance,
				PossibleCause: "unknown",
			})
		}
	}

	return holes
}

func calculateBettiNumbers(persistence []PersistencePair) []int {
	// Betti numbers count features at each dimension
	maxDim := 0
	for _, pair := range persistence {
		if pair.Dimension > maxDim {
			maxDim = pair.Dimension
		}
	}

	betti := make([]int, maxDim+1)

	for _, pair := range persistence {
		if pair.Persistence > 0.2 { // Only count significant features
			betti[pair.Dimension]++
		}
	}

	return betti
}

func classifyHoleCause(hole Hole) string {
	// Classify based on persistence and dimension
	if hole.Persistence > 0.7 {
		// Very persistent holes likely indicate trauma
		return "trauma"
	} else if hole.Persistence > 0.4 {
		// Medium persistence may indicate repression
		return "repressao"
	} else if hole.Persistence > 0.2 {
		// Lower persistence may be avoidance
		return "evitacao"
	}

	return "normal_gap"
}

type SemanticSegment struct {
	Text      string
	Emotion   float64
	Intensity float64
}

func extractSemanticSegments(transcript string) []SemanticSegment {
	// Simplified segmentation
	// In production, would use NLP to segment by topic/emotion shifts
	segments := []SemanticSegment{
		{Text: transcript, Emotion: 0.0, Intensity: 0.5},
	}

	return segments
}
