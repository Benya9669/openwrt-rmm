// Package fleetprofile validates the bounded, declarative UCI profile contract.
package fleetprofile

import (
	"errors"
	"regexp"
	"strings"
)

type Option struct {
	Section string `json:"section"`
	Option  string `json:"option"`
	Value   string `json:"value"`
}
type Profile struct {
	Config  string   `json:"config"`
	Options []Option `json:"options"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$`)
var sectionIdentifier = regexp.MustCompile(`^(?:[A-Za-z0-9_][A-Za-z0-9_-]{0,63}|@[A-Za-z_][A-Za-z0-9_-]{0,63}\[[0-9]{1,3}\])$`)

func Validate(profile Profile) error {
	switch profile.Config {
	case "system", "network", "wireless", "dhcp", "firewall":
	default:
		return errors.New("unsupported UCI configuration")
	}
	if len(profile.Options) == 0 || len(profile.Options) > 32 {
		return errors.New("profile needs 1 to 32 options")
	}
	seen := map[string]bool{}
	for _, option := range profile.Options {
		key := option.Section + "." + option.Option
		if !sectionIdentifier.MatchString(option.Section) || !identifier.MatchString(option.Option) || len(option.Value) > 2048 || strings.ContainsAny(option.Value, "\x00\r\n") || seen[key] {
			return errors.New("invalid or duplicate UCI option")
		}
		seen[key] = true
	}
	return nil
}
func Sensitive(option string) bool {
	value := strings.ToLower(option)
	for _, part := range []string{"password", "passwd", "secret", "token", "key", "psk", "credential"} {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
func Mask(profile Profile) Profile {
	result := profile
	result.Options = append([]Option(nil), profile.Options...)
	for i := range result.Options {
		if Sensitive(result.Options[i].Option) {
			result.Options[i].Value = "[redacted]"
		}
	}
	return result
}
