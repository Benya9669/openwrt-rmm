package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfilePreviewDriftApplyRollbackAndRecovery(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "system")
	original := []byte("config system core\n option hostname 'old'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config{CommandStateDir: filepath.Join(dir, "state"), ServerURL: "https://rmm.example.test"}
	desired := `{"config":"system","options":[{"section":"core","option":"hostname","value":"new"}]}`
	arguments := func(preview string) json.RawMessage {
		data, _ := json.Marshal(map[string]string{"profile_json": desired, "preview_id": preview})
		return data
	}
	preview := command{ID: "cmd-profile-preview", Type: "uci_profile_preview", Args: arguments("")}
	apply := command{ID: "cmd-profile-apply", Type: "uci_profile_apply", Args: arguments(preview.ID)}
	calls := []string{}
	mutated := []byte("config system core\n option hostname 'new'\n")
	runner := func(ctx context.Context, timeout time.Duration, name string, args ...string) (string, int) {
		call := name + " " + strings.Join(args, " ")
		calls = append(calls, call)
		if call == "uci -q get system.core.hostname" {
			return "old\n", 0
		}
		if call == "uci commit system" {
			if err := os.WriteFile(path, mutated, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return "", 0
	}
	reachable := func(context.Context, string) bool { return true }
	wait := func(context.Context) error { return nil }
	report, code := runFleetProfile(context.Background(), cfg, preview, configDir, runner, reachable, wait)
	if code != 0 || !strings.Contains(report, `"drift":true`) {
		t.Fatalf("preview: %s %d", report, code)
	}
	for _, call := range calls {
		if strings.Contains(call, " set ") || strings.Contains(call, " commit ") || strings.Contains(call, " revert ") {
			t.Fatalf("preview mutated staging: %s", call)
		}
	}
	if err := os.WriteFile(path, []byte("external change"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, code = runFleetProfile(context.Background(), cfg, apply, configDir, runner, reachable, wait)
	if code == 0 {
		t.Fatal("stale preview accepted")
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	report, code = runFleetProfile(context.Background(), cfg, apply, configDir, runner, func(context.Context, string) bool { return false }, wait)
	if code == 0 {
		t.Fatal("lost connection accepted")
	}
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != string(original) {
		t.Fatalf("rollback did not restore config: %s %v", restored, err)
	}
	report, code = runFleetProfile(context.Background(), cfg, apply, configDir, runner, reachable, wait)
	if code != 0 {
		t.Fatalf("confirmed apply: %s", report)
	}
	report, code = rollbackFleetProfile(cfg, apply.ID, configDir, runner)
	if code != 0 {
		t.Fatalf("manual rollback: %s", report)
	}
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	tx := profileTransaction{Config: "system", Backup: apply.ID + ".backup", BackupHash: profileDigest(original)}
	journal, _ := json.Marshal(tx)
	if err := writeProfileState(filepath.Join(profileStateDir(cfg), apply.ID+".journal"), journal); err != nil {
		t.Fatal(err)
	}
	if err := recoverFleetProfiles(cfg, configDir, runner); err != nil {
		t.Fatal(err)
	}
	restored, err = os.ReadFile(path)
	if err != nil || string(restored) != string(original) {
		t.Fatal("interrupted transaction not recovered")
	}
	if err := os.WriteFile(path, []byte("new unrelated change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, code = rollbackFleetProfile(cfg, apply.ID, configDir, runner); code == 0 {
		t.Fatal("manual rollback overwrote subsequent changes")
	}
}

func TestProfileRejectsStagedChangesAndCredentials(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "system"), []byte("config system core\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config{CommandStateDir: filepath.Join(t.TempDir(), "state")}
	args, _ := json.Marshal(map[string]string{"profile_json": `{"config":"system","options":[{"section":"core","option":"hostname","value":"new"}]}`})
	cmd := command{ID: "cmd-profile-test", Type: "uci_profile_preview", Args: args}
	calls := 0
	run := func(context.Context, time.Duration, string, ...string) (string, int) {
		calls++
		return "system.core.hostname=other\n", 0
	}
	if _, code := runFleetProfile(context.Background(), cfg, cmd, dir, run, nil, nil); code == 0 || calls != 1 {
		t.Fatal("staged settings were changed")
	}
	args, _ = json.Marshal(map[string]string{"profile_json": `{"config":"system","options":[{"section":"core","option":"password","value":"synthetic"}]}`})
	cmd.Args = args
	calls = 0
	if _, code := runFleetProfile(context.Background(), cfg, cmd, dir, run, nil, nil); code == 0 || calls != 0 {
		t.Fatal("credential profile was accepted")
	}
}
