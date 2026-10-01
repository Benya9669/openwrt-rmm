package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
)

func fleetObjectID(prefix, userID, key string) (string, error) {
	if len(key) < 8 || len(key) > 128 {
		return "", errors.New("invalid request key")
	}
	digest := sha256.Sum256([]byte(userID + "\x00" + key))
	return prefix + "_" + hex.EncodeToString(digest[:16]), nil
}

type FleetUpdateArgs struct {
	Targets map[string]map[string]string `json:"targets"`
}

func fleetArgsLimit(kind string) int {
	if kind == "agent_update" {
		return 256 << 10
	}
	return 16384
}
func fleetUpdateDeviceArgs(raw json.RawMessage, ids []string) (map[string]json.RawMessage, error) {
	var resolved FleetUpdateArgs
	if json.Unmarshal(raw, &resolved) != nil || len(resolved.Targets) != len(ids) {
		return nil, errors.New("resolved update targets required")
	}
	result := map[string]json.RawMessage{}
	for _, id := range ids {
		args := resolved.Targets[id]
		if args == nil || args["package"] != "rmm-agent-go-production" || args["target_version"] == "" || args["package_version"] == "" || args["expected_release"] == "" || args["expected_target"] == "" || (args["package_manager"] != "apk" && args["package_manager"] != "opkg") {
			return nil, errors.New("invalid resolved update")
		}
		for _, key := range []string{"feed_url", "manifest_url", "signature_url"} {
			parsed, err := url.Parse(args[key])
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
				return nil, errors.New("signed HTTPS update required")
			}
		}
		data, err := json.Marshal(args)
		if err != nil || len(data) > 16384 {
			return nil, errors.New("update arguments exceed limit")
		}
		result[id] = data
	}
	return result, nil
}
func validateFleetUpdateDevice(ctx context.Context, tx *sqliteTx, deviceID string, args json.RawMessage) (bool, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, "SELECT inventory_json FROM devices WHERE id=?", deviceID).Scan(&raw); err != nil {
		return false, err
	}
	var inventory struct {
		Runtime string `json:"agent_runtime"`
		Package string `json:"agent_package"`
		Version string `json:"agent_version"`
		Release string `json:"openwrt_release"`
		Target  string `json:"target"`
		Manager string `json:"package_manager"`
	}
	var expected map[string]string
	if json.Unmarshal(raw, &inventory) != nil || json.Unmarshal(args, &expected) != nil || inventory.Runtime != "go" || inventory.Package != "rmm-agent-go-production" || inventory.Release != expected["expected_release"] || inventory.Target != expected["expected_target"] || inventory.Manager != expected["package_manager"] {
		return false, ErrFleetFeatureUnavailable
	}
	return inventory.Version == expected["target_version"], nil
}
