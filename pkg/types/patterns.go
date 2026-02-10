package types

import "time"

type RecurrentPattern struct {
	Topic         string    `json:"topic"`
	Frequency     int       `json:"frequency"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	AvgInterval   float64   `json:"avg_interval_days"`
	Emotions      []string  `json:"associated_emotions"`
	SeverityTrend string    `json:"severity_trend"` // "increasing", "stable", "decreasing"
	Confidence    float64   `json:"confidence"`
}

type TemporalPattern struct {
	Topic       string `json:"topic"`
	TimeOfDay   string `json:"time_of_day"` // "morning", "afternoon", "evening", "night"
	DayOfWeek   string `json:"day_of_week"` // "monday", "weekend", etc.
	Occurrences int    `json:"occurrences"`
}
