package httpapi

import "testing"

func TestTrustedHistoricalManifestURL(t *testing.T) {
	base := "https://packages.example.test/releases/update-manifest.json"
	for _, value := range []string{
		"https://packages.example.test/releases/0.6.8/manifest.json",
		"https://packages.example.test/releases/0.6.8/manifest.sig",
	} {
		if !trustedHistoricalManifestURL(base, value) {
			t.Fatalf("expected %q to be trusted", value)
		}
	}
	for _, value := range []string{
		"http://packages.example.test/releases/0.6.8/manifest.json",
		"https://other.example.test/releases/manifest.json",
		"https://packages.example.test/other/manifest.json",
		"https://packages.example.test/releases/manifest.json?x=1",
	} {
		if trustedHistoricalManifestURL(base, value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"0.6.8", "0.6.9", -1},
		{"0.6.9", "0.6.9", 0},
		{"0.7.0", "0.6.9", 1},
		{"0.6.9-rc.1", "0.6.9", -1},
		{"0.6.9-rc.10", "0.6.9-rc.2", 1},
	} {
		got := compareSemver(test.left, test.right)
		if (got < 0 && test.want < 0) || (got == 0 && test.want == 0) || (got > 0 && test.want > 0) {
			continue
		}
		t.Fatalf("compareSemver(%q, %q) = %d, want sign %d", test.left, test.right, got, test.want)
	}
	if compareSemver("invalid", "0.6.9") <= 0 {
		t.Fatal("invalid version must not be accepted as a rollback target")
	}
}
