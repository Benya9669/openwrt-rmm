package httpapi

import (
	"testing"
	"time"
)

func TestEventHubPublishesToSubscribers(t *testing.T) {
	hub := newEventHub()
	updates, unsubscribe := hub.subscribe()
	defer unsubscribe()

	hub.publish("devices")
	select {
	case event := <-updates:
		if event != "devices" {
			t.Fatalf("unexpected event %q", event)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not published")
	}
}
