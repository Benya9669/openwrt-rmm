package openwrtcompat

import "testing"

func TestReleaseMatches(t *testing.T) {
	for _, tc := range []struct {
		feed, device, format string
		want                 bool
	}{
		{"25.12.4", "25.12.5", "apk", true}, {"25.12", "25.12.5", "apk", true},
		{"25.12.5", "25.12", "apk", true}, {"25.12.4", "25.12.5", "ipk", false},
		{"25.12.4", "25.12.5-rc1", "apk", false}, {"25.12.4", "25.12.5.extra", "apk", false},
		{"25.12.4", "25.12.", "apk", false}, {"25.12.4", "25.13.0", "apk", false},
		{"25.12.4", "SNAPSHOT", "apk", false}, {"24.10.7", "24.10.8", "ipk", false},
		{"24.10.7", "24.10.7", "ipk", true}, {"", "", "apk", false},
	} {
		if got := ReleaseMatches(tc.feed, tc.device, tc.format); got != tc.want {
			t.Errorf("ReleaseMatches(%q,%q,%q)=%v, want %v", tc.feed, tc.device, tc.format, got, tc.want)
		}
	}
}
