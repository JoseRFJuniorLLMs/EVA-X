package brain

import (
	"strings"
	"time"
)

// MemoryContext contains metadata for memory storage
type MemoryContext struct {
	Emotion    string   // Detected emotion (e.g., "happy", "sad", "neutral")
	Urgency    string   // Urgency level (e.g., "high", "medium", "low")
	Keywords   []string // Extracted keywords
	Importance float64  // Calculated importance (0-1)
}

// SaveEpisodicMemoryWithContext saves memory with full context metadata
func (s *Service) SaveEpisodicMemoryWithContext(
	idosoID int64,
	role string,
	content string,
	eventDate time.Time,
	isAtomic bool,
	memCtx MemoryContext,
) {
	// Delegate to the existing SaveEpisodicMemory with enhanced metadata
	// For now, we'll use the old function but log the context
	// TODO: Enhance SaveEpisodicMemory to use memCtx

	// Calculate importance if not set
	if memCtx.Importance == 0 {
		memCtx.Importance = calculateImportance(content, memCtx.Emotion, memCtx.Urgency)
	}

	// Call existing function (will be enhanced later)
	s.SaveEpisodicMemory(idosoID, role, content, eventDate, isAtomic)
}

// summarizeText creates a simple summary of text
func summarizeText(text string) string {
	// Simple summarization: take first 200 chars or first 2 sentences
	text = strings.TrimSpace(text)

	if len(text) <= 200 {
		return text
	}

	// Try to find sentence boundaries
	sentences := strings.Split(text, ".")
	if len(sentences) >= 2 {
		summary := sentences[0] + "." + sentences[1] + "."
		if len(summary) <= 200 {
			return summary
		}
	}

	// Fallback: truncate at 200 chars
	return text[:200] + "..."
}

// calculateImportance calculates memory importance based on content and context
func calculateImportance(content string, emotion string, urgency string) float64 {
	importance := 0.5 // Base importance

	// Boost for emotional content
	switch emotion {
	case "happy", "excited":
		importance += 0.1
	case "sad", "angry", "fearful":
		importance += 0.2
	}

	// Boost for urgency
	switch urgency {
	case "high":
		importance += 0.2
	case "medium":
		importance += 0.1
	}

	// Boost for personal information keywords
	personalKeywords := []string{
		"filha", "filho", "esposa", "marido", "mãe", "pai",
		"daughter", "son", "wife", "husband", "mother", "father",
		"gosto", "amo", "prefiro", "like", "love", "prefer",
		"nome", "name", "chama", "called",
	}

	lowerContent := strings.ToLower(content)
	for _, keyword := range personalKeywords {
		if strings.Contains(lowerContent, keyword) {
			importance += 0.15
			break
		}
	}

	// Cap at 1.0
	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}
