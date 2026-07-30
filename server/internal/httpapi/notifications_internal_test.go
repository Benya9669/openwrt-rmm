package httpapi

import (
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
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

func TestNotificationQuietHoursAcrossMidnight(t *testing.T) {
	settings := model.NotificationSettings{
		Timezone: "UTC", QuietHoursEnabled: true, QuietHoursStart: "22:00", QuietHoursEnd: "08:00",
	}
	if !notificationQuietNow(settings, time.Date(2026, 7, 30, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("23:00 must be quiet")
	}
	if notificationQuietNow(settings, time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("12:00 must not be quiet")
	}
}
