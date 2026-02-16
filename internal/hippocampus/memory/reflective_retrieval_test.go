package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "removes stopwords",
			text: "o que aconteceu ontem com a Maria?",
			want: []string{"aconteceu", "ontem", "maria"},
		},
		{
			name: "removes punctuation",
			text: "como está a saúde?",
			want: []string{"saúde"},
		},
		{
			name: "empty text",
			text: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKeywords(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractTemporalOffset(t *testing.T) {
	tests := []struct {
		name string
		text string
		want *int
	}{
		{name: "hoje", text: "como foi hoje?", want: intPtr(0)},
		{name: "ontem", text: "o que aconteceu ontem?", want: intPtr(-1)},
		{name: "semana", text: "na semana passada", want: intPtr(-7)},
		{name: "no temporal", text: "quem é Maria?", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTemporalOffset(tt.text)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, *tt.want, *got)
			}
		})
	}
}

func TestComputeTemporalCoherence(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		offset    *int
		eventDate time.Time
		wantMin   float64
		wantMax   float64
	}{
		{
			name:      "exact match yesterday",
			offset:    intPtr(-1),
			eventDate: now.AddDate(0, 0, -1),
			wantMin:   0.9,
			wantMax:   1.0,
		},
		{
			name:      "close match (2 days off)",
			offset:    intPtr(-1),
			eventDate: now.AddDate(0, 0, -3),
			wantMin:   0.6,
			wantMax:   0.8,
		},
		{
			name:      "no temporal query",
			offset:    nil,
			eventDate: now,
			wantMin:   0.5,
			wantMax:   0.5,
		},
		{
			name:      "no event date",
			offset:    intPtr(0),
			eventDate: time.Time{},
			wantMin:   0.3,
			wantMax:   0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeTemporalCoherence(tt.offset, tt.eventDate)
			assert.GreaterOrEqual(t, got, tt.wantMin)
			assert.LessOrEqual(t, got, tt.wantMax)
		})
	}
}

func TestComputeEmotionalAlignment(t *testing.T) {
	tests := []struct {
		name       string
		queryFam   string
		resultEmo  string
		wantMin    float64
	}{
		{name: "same positive", queryFam: "positive", resultEmo: "alegria", wantMin: 0.9},
		{name: "same negative", queryFam: "negative", resultEmo: "tristeza", wantMin: 0.9},
		{name: "opposite", queryFam: "positive", resultEmo: "tristeza", wantMin: 0.1},
		{name: "unknown query", queryFam: "unknown", resultEmo: "alegria", wantMin: 0.4},
		{name: "empty result", queryFam: "negative", resultEmo: "", wantMin: 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEmotionalAlignment(tt.queryFam, tt.resultEmo)
			assert.GreaterOrEqual(t, got, tt.wantMin)
		})
	}
}

func TestDetectContradictions(t *testing.T) {
	results := []*ReflectedResult{
		{
			SearchResult: &SearchResult{
				Memory:     &Memory{Content: "hoje foi um dia muito bom com a família", Emotion: "alegria"},
				Similarity: 0.9,
			},
		},
		{
			SearchResult: &SearchResult{
				Memory:     &Memory{Content: "hoje foi um dia muito ruim com a família", Emotion: "tristeza"},
				Similarity: 0.85,
			},
		},
		{
			SearchResult: &SearchResult{
				Memory:     &Memory{Content: "fui ao mercado comprar frutas", Emotion: ""},
				Similarity: 0.5,
			},
		},
	}

	detectContradictions(results)

	// First two should be flagged (similar content, opposing emotions)
	assert.Greater(t, results[0].Reflection.ContradictionFlag, 0.0)
	assert.Greater(t, results[1].Reflection.ContradictionFlag, 0.0)
	// Third should not be flagged (different content)
	assert.Equal(t, 0.0, results[2].Reflection.ContradictionFlag)
}

func TestReflectAndRerank(t *testing.T) {
	now := time.Now()

	results := []*SearchResult{
		{
			Memory:     &Memory{Content: "Maria tomou remédio ontem", EventDate: now.AddDate(0, 0, -1), Emotion: ""},
			Similarity: 0.7,
		},
		{
			Memory:     &Memory{Content: "Carlos visitou semana passada", EventDate: now.AddDate(0, 0, -7), Emotion: "alegria"},
			Similarity: 0.8,
		},
		{
			Memory:     &Memory{Content: "Maria fez aniversário ontem e ficou feliz", EventDate: now.AddDate(0, 0, -1), Emotion: "alegria"},
			Similarity: 0.6,
		},
	}

	reflected := ReflectAndRerank("o que aconteceu com Maria ontem?", results, 3)

	assert.Len(t, reflected, 3)
	// First result should mention Maria + match temporal (ontem)
	assert.Contains(t, reflected[0].Memory.Content, "Maria")
	// All should have valid final scores
	for _, r := range reflected {
		assert.Greater(t, r.FinalScore, 0.0)
		assert.LessOrEqual(t, r.FinalScore, 1.5) // max possible
	}
	// Should be sorted by FinalScore DESC
	for i := 1; i < len(reflected); i++ {
		assert.GreaterOrEqual(t, reflected[i-1].FinalScore, reflected[i].FinalScore)
	}
}

func TestDetectEmotionFamily(t *testing.T) {
	assert.Equal(t, "negative", detectEmotionFamily("o que te deixa triste?"))
	assert.Equal(t, "negative", detectEmotionFamily("tenho muito medo"))
	assert.Equal(t, "positive", detectEmotionFamily("algo que te deixa feliz?"))
	assert.Equal(t, "unknown", detectEmotionFamily("quem é Maria?"))
}

func intPtr(i int) *int {
	return &i
}
