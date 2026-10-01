package fleetprofile

import "testing"

func TestProfilePathsAreBoundedAndMaskCredentials(t *testing.T) {
	for _, section := range []string{"core", "@system[0]", "@wifi-iface[12]"} {
		if err := Validate(Profile{Config: "system", Options: []Option{{Section: section, Option: "hostname", Value: "branch"}}}); err != nil {
			t.Fatalf("valid %s rejected: %v", section, err)
		}
	}
	for _, section := range []string{"../etc/shadow", "@system[-1]", "@system[1000]", "core.hostname", "core;reboot"} {
		if err := Validate(Profile{Config: "system", Options: []Option{{Section: section, Option: "hostname", Value: "branch"}}}); err == nil {
			t.Fatalf("unsafe %s accepted", section)
		}
	}
	profile := Profile{Config: "wireless", Options: []Option{{Section: "default_radio0", Option: "key", Value: "synthetic"}}}
	masked := Mask(profile)
	if masked.Options[0].Value != "[redacted]" || profile.Options[0].Value != "synthetic" {
		t.Fatal("mask changed original or exposed credentials")
	}
}
