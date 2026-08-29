package audio

import (
	"testing"
	"time"
)

func TestTranscribeClient_TimeoutIsFourHours(t *testing.T) {
	c := NewHTTPClient("http://x")
	if c.transcribeClient.Timeout != 4*time.Hour {
		t.Fatalf("timeout = %v, want 4h", c.transcribeClient.Timeout)
	}
}
