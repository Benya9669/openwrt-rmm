package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"rmm-openwrt/internal/fleetprofile"
	"strings"
	"time"
)

type profileTicket struct {
	Config      string    `json:"config"`
	BaseHash    string    `json:"base_hash"`
	DesiredHash string    `json:"desired_hash"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type profileTransaction struct {
	Config      string `json:"config"`
	Backup      string `json:"backup"`
	BackupHash  string `json:"backup_hash"`
	AppliedHash string `json:"applied_hash,omitempty"`
}
type profileChange struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Drift   bool   `json:"drift"`
	Before  string `json:"before"`
	Desired string `json:"desired"`
}

func profileDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func profileStateDir(cfg config) string { return filepath.Join(cfg.CommandStateDir, "profiles") }
func writeProfileState(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".profile-")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func runFleetProfile(ctx context.Context, cfg config, cmd command, configDir string, run diagnosticRunner, reachable func(context.Context, string) bool, wait func(context.Context) error) (string, int) {
	var args map[string]string
	if json.Unmarshal(cmd.Args, &args) != nil || !safeSessionID(cmd.ID) {
		return "invalid profile command\n", 2
	}
	if cmd.Type == "uci_profile_rollback" {
		return rollbackFleetProfile(cfg, args["apply_id"], configDir, run)
	}
	var profile fleetprofile.Profile
	if json.Unmarshal([]byte(args["profile_json"]), &profile) != nil || fleetprofile.Validate(profile) != nil {
		return "invalid UCI profile\n", 2
	}
	for _, option := range profile.Options {
		if fleetprofile.Sensitive(option.Option) {
			return "credential options require the credential workflow\n", 2
		}
	}
	desired, err := json.Marshal(profile)
	if err != nil {
		return "invalid UCI profile\n", 2
	}
	configPath := filepath.Join(configDir, profile.Config)
	original, err := os.ReadFile(configPath)
	if err != nil {
		return "configuration unavailable\n", 1
	}
	if len(original) > 1<<20 {
		return "configuration exceeds profile limit\n", 2
	}
	staged, code := run(ctx, 5*time.Second, "uci", "changes", profile.Config)
	if code != 0 || strings.TrimSpace(staged) != "" {
		return "configuration has staged changes; resolve them before using profiles\n", 1
	}
	ticket := profileTicket{Config: profile.Config, BaseHash: profileDigest(original), DesiredHash: profileDigest(desired), ExpiresAt: time.Now().Add(30 * time.Minute)}
	stateDir := profileStateDir(cfg)
	if cmd.Type == "uci_profile_preview" {
		changes := []profileChange{}
		for _, option := range profile.Options {
			key := profile.Config + "." + option.Section + "." + option.Option
			current, getCode := run(ctx, 5*time.Second, "uci", "-q", "get", key)
			if getCode != 0 && getCode != 1 {
				return "failed to read profile option\n", 1
			}
			current = strings.TrimSuffix(current, "\n")
			current = strings.TrimSuffix(current, "\r")
			changes = append(changes, profileChange{Path: key, Present: getCode == 0, Drift: getCode != 0 || current != option.Value, Before: current, Desired: option.Value})
		}
		data, _ := json.Marshal(ticket)
		if err = writeProfileState(filepath.Join(stateDir, cmd.ID+".preview"), data); err != nil {
			return "failed to persist preview\n", 1
		}
		report, _ := json.Marshal(map[string]any{"changes": changes, "expires_at": ticket.ExpiresAt, "preview_id": cmd.ID})
		return string(report), 0
	}
	if cmd.Type != "uci_profile_apply" || !safeSessionID(args["preview_id"]) {
		return "profile apply requires a successful preview\n", 2
	}
	saved, err := os.ReadFile(filepath.Join(stateDir, args["preview_id"]+".preview"))
	if err != nil {
		return "preview ticket unavailable\n", 1
	}
	var previous profileTicket
	if json.Unmarshal(saved, &previous) != nil || previous.Config != ticket.Config || previous.BaseHash != ticket.BaseHash || previous.DesiredHash != ticket.DesiredHash || !previous.ExpiresAt.After(time.Now()) {
		return "configuration or profile changed; preview again\n", 1
	}
	backupName := cmd.ID + ".backup"
	if err = writeProfileState(filepath.Join(stateDir, backupName), original); err != nil {
		return "backup failed; configuration unchanged\n", 1
	}
	transaction := profileTransaction{Config: profile.Config, Backup: backupName, BackupHash: profileDigest(original)}
	journalPath := filepath.Join(stateDir, cmd.ID+".journal")
	journal, _ := json.Marshal(transaction)
	if err = writeProfileState(journalPath, journal); err != nil {
		return "rollback journal failed; configuration unchanged\n", 1
	}
	confirmed := false
	defer func() {
		if !confirmed {
			if err := restoreProfileTransaction(cfg, transaction, configDir, run); err != nil {
				logf("profile rollback failed for %s: %v", cmd.ID, err)
			} else if err := os.Remove(journalPath); err != nil {
				logf("remove restored profile journal: %v", err)
			}
		}
	}()
	for _, option := range profile.Options {
		value := fmt.Sprintf("%s.%s.%s=%s", profile.Config, option.Section, option.Option, option.Value)
		if _, code = run(ctx, 10*time.Second, "uci", "set", value); code != 0 {
			return "profile staging failed; rolling back\n", 1
		}
	}
	if _, code = run(ctx, 20*time.Second, "uci", "commit", profile.Config); code != 0 {
		return "profile commit failed; rolling back\n", 1
	}
	if err = reloadProfileConfig(ctx, profile.Config, run); err != nil {
		return "configuration reload failed; rolling back\n", 1
	}
	if err = wait(ctx); err != nil || !reachable(ctx, cfg.ServerURL) {
		return "server unreachable after apply; rolling back\n", 1
	}
	applied, err := os.ReadFile(configPath)
	if err != nil {
		return "cannot verify applied configuration; rolling back\n", 1
	}
	transaction.AppliedHash = profileDigest(applied)
	receipt, _ := json.Marshal(transaction)
	if err = writeProfileState(filepath.Join(stateDir, cmd.ID+".receipt"), receipt); err != nil {
		return "cannot persist rollback receipt; rolling back\n", 1
	}
	if err = os.Remove(journalPath); err != nil {
		return "cannot confirm rollback journal; rolling back\n", 1
	}
	confirmed = true
	return "profile applied; backup retained; server connectivity confirmed\n", 0
}

func reloadProfileConfig(ctx context.Context, name string, run diagnosticRunner) error {
	service := map[string]string{"system": "system", "network": "network", "wireless": "network", "dhcp": "dnsmasq", "firewall": "firewall"}[name]
	if service == "" {
		return errors.New("invalid profile configuration")
	}
	if _, code := run(ctx, 30*time.Second, "/etc/init.d/"+service, "reload"); code != 0 {
		return errors.New("profile reload failed")
	}
	return nil
}

func restoreProfileTransaction(cfg config, transaction profileTransaction, configDir string, run diagnosticRunner) error {
	if !safeUCIConfig(transaction.Config) || filepath.Base(transaction.Backup) != transaction.Backup || !strings.HasSuffix(transaction.Backup, ".backup") {
		return errors.New("invalid rollback journal")
	}
	original, err := os.ReadFile(filepath.Join(profileStateDir(cfg), transaction.Backup))
	if err != nil {
		return err
	}
	if profileDigest(original) != transaction.BackupHash {
		return errors.New("rollback backup checksum mismatch")
	}
	if err = writeProfileState(filepath.Join(configDir, transaction.Config), original); err != nil {
		return err
	}
	recoveryCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, code := run(recoveryCtx, 10*time.Second, "uci", "revert", transaction.Config); code != 0 {
		return errors.New("rollback staging cleanup failed")
	}
	return reloadProfileConfig(recoveryCtx, transaction.Config, run)
}

func rollbackFleetProfile(cfg config, applyID, configDir string, run diagnosticRunner) (string, int) {
	if !safeSessionID(applyID) {
		return "invalid rollback receipt\n", 2
	}
	receipt, err := os.ReadFile(filepath.Join(profileStateDir(cfg), applyID+".receipt"))
	if err != nil {
		return "rollback receipt unavailable\n", 1
	}
	var transaction profileTransaction
	if json.Unmarshal(receipt, &transaction) != nil || !safeUCIConfig(transaction.Config) {
		return "invalid rollback receipt\n", 1
	}
	current, err := os.ReadFile(filepath.Join(configDir, transaction.Config))
	if err != nil || profileDigest(current) != transaction.AppliedHash {
		return "configuration changed since apply; refusing overwrite\n", 1
	}
	if err = restoreProfileTransaction(cfg, transaction, configDir, run); err != nil {
		return "profile rollback failed\n", 1
	}
	return "profile rolled back to retained backup\n", 0
}

func recoverFleetProfiles(cfg config, configDir string, run diagnosticRunner) error {
	entries, err := os.ReadDir(profileStateDir(cfg))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".journal") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(profileStateDir(cfg), entry.Name()))
		if err != nil {
			return err
		}
		var transaction profileTransaction
		if err = json.Unmarshal(data, &transaction); err != nil {
			return err
		}
		if err = restoreProfileTransaction(cfg, transaction, configDir, run); err != nil {
			return err
		}
		if err = os.Remove(filepath.Join(profileStateDir(cfg), entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
