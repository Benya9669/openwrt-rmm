package httpapi

import (
	"encoding/json"
	"rmm-openwrt/internal/openwrtcompat"
	"rmm-openwrt/server/internal/model"
)

// Cross-patch selection is only queued for agents advertising the matching
// manifest verifier. Older agents need a one-time native package update.
func agentSupportsFeed(raw json.RawMessage, feed model.AgentFeed) bool {
	if feed.OpenWrtRelease == "" {
		return true
	} // legacy resolver contracts
	var inventory struct {
		Release  string   `json:"openwrt_release"`
		Manager  string   `json:"package_manager"`
		Features []string `json:"rmm_features"`
	}
	if json.Unmarshal(raw, &inventory) != nil {
		return false
	}
	if inventory.Release == feed.OpenWrtRelease {
		return true
	}
	if !openwrtcompat.ReleaseMatches(feed.OpenWrtRelease, inventory.Release, map[string]string{"apk": "apk", "opkg": "ipk"}[inventory.Manager]) {
		return false
	}
	return containsString(inventory.Features, openwrtcompat.ReleaseLineFeature)
}
