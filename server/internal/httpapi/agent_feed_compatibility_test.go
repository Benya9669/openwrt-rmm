package httpapi

import (
	"encoding/json"
	"rmm-openwrt/internal/openwrtcompat"
	"rmm-openwrt/server/internal/model"
	"testing"
)

func TestAgentSupportsReleaseLineFeedRequiresCapability(t *testing.T) {
	feed := model.AgentFeed{OpenWrtRelease: "25.12.4"}
	for _, tc := range []struct {
		release, manager string
		features         []string
		want             bool
	}{
		{"25.12.4", "apk", nil, true}, {"25.12.5", "apk", nil, false},
		{"25.12.5", "apk", []string{openwrtcompat.ReleaseLineFeature}, true},
		{"25.13.0", "apk", []string{openwrtcompat.ReleaseLineFeature}, false},
		{"25.12.5", "opkg", []string{openwrtcompat.ReleaseLineFeature}, false},
	} {
		raw, _ := json.Marshal(map[string]any{"openwrt_release": tc.release, "package_manager": tc.manager, "rmm_features": tc.features})
		if got := agentSupportsFeed(raw, feed); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}
