package httpapi

import (
	"testing"

	"rmm-openwrt/server/internal/model"
)

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

func TestAgentPackageCommandArgsBootstrapCompatibility(t *testing.T) {
	feed := model.AgentFeed{TargetVersion: "0.6.10", FeedURL: "https://packages.example.test/feed", PackageVersion: "0.6.10-r1", ManifestURL: "https://packages.example.test/update-manifest.json", SignatureURL: "https://packages.example.test/update-manifest.sig"}
	legacy := agentPackageCommandArgs(feed, "apk", "0.6.9")
	if legacy["manifest_url"] != "" || legacy["signature_url"] != "" {
		t.Fatalf("0.6.9 cannot accept new manifest fields: %#v", legacy)
	}
	verified := agentPackageCommandArgs(feed, "apk", "0.6.10")
	if verified["manifest_url"] != feed.ManifestURL || verified["signature_url"] != feed.SignatureURL {
		t.Fatalf("0.6.10 must receive signed manifest coordinates: %#v", verified)
	}
}
