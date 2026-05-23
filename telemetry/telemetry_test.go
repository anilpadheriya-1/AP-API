package telemetry

import (
	"testing"
	"time"
)

// A basic test to make sure syntax is correct
func TestLogEntry(t *testing.T) {
	entry := LogEntry{
		Path:         "/test",
		Timestamp:    time.Now(),
		LatencyMs:    10,
		StatusCode:   200,
		PayloadBytes: 100,
	}
	if entry.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", entry.StatusCode)
	}
}
