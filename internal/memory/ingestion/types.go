package ingestion

import "time"

// AtomicFact represents a single, verifiable piece of information
// extracted from a larger text block.
type AtomicFact struct {
	ResolvedText string    `json:"resolved_text"` // Text with ambiguities resolved (e.g., "he" -> "John")
	Subject      string    `json:"subject"`       // Who/What the fact is about
	Predicate    string    `json:"predicate"`     // The action or relationship
	Object       string    `json:"object"`        // The target of the action
	EventDate    time.Time `json:"event_date"`    // When the event actually occurred
	DocumentDate time.Time `json:"document_date"` // When the fact was recorded
	Confidence   float64   `json:"confidence"`    // Extraction confidence (0.0 to 1.0)
	IsAtomic     bool      `json:"is_atomic"`     // Flag to distinguish from raw chunks
}
