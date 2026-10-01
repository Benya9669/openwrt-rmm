// Package openwrtcompat defines the release matching shared by server and agent.
package openwrtcompat

import "strings"

const ReleaseLineFeature = "agent_update_release_line_25_12"

// ReleaseMatches accepts stable APK releases in the 25.12 line. Other lines
// retain exact matching; targets, package identities and signatures are checked
// separately by the caller.
func ReleaseMatches(feed, device, format string) bool {
	if feed == device && feed != "" {
		return true
	}
	return format == "apk" && is2512(feed) && is2512(device)
}

func is2512(release string) bool {
	if release == "25.12" {
		return true
	}
	patch, ok := strings.CutPrefix(release, "25.12.")
	if !ok || patch == "" {
		return false
	}
	for _, digit := range patch {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
