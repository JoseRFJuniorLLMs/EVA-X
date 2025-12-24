package scheduler

// Retry logic for failed calls
func ShouldRetry(attempts int, maxRetries int) bool {
	return attempts < maxRetries
}
