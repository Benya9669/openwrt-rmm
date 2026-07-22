package httpapi

import (
	"testing"
	"time"
)

func TestNotificationRetryDelay(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute}
	for index, expected := range want {
		if got := notificationRetryDelay(index + 1); got != expected {
			t.Fatalf("attempt %d: got %s want %s", index+1, got, expected)
		}
	}
	if got := notificationRetryDelay(20); got != 30*time.Minute {
		t.Fatalf("retry cap: got %s want 30m", got)
	}
}
