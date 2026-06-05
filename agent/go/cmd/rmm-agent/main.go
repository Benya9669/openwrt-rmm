package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const agentVersion = "0.5.0-go-preview"

type config struct {
	ServerURL       string
	EnrollmentToken string
	IntervalSeconds int
	DeviceID        string
	DeviceToken     string
	LockFile        string
	SpoolDir        string
	BackupDir       string
	CheckTargets    []string
	ConfigFile      string
}

type command struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Args json.RawMessage `json:"args"`
}

type heartbeatResponse struct {
	Commands []command `json:"commands"`
}

type enrollResponse struct {
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token"`
}

func main() {
	configFile := flag.String("config", envDefault("CONFIG_FILE", "/etc/rmm-agent.conf"), "agent config file")
	once := flag.Bool("once", false, "run one heartbeat and exit")
	flag.Parse()

	cfg, err := loadConfig(*configFile)
	if err != nil {
		logf("config error: %v", err)
		os.Exit(1)
	}

	unlock, err := acquireLock(cfg.LockFile)
	if err != nil {
		logf("%v", err)
		os.Exit(1)
	}
	defer unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		cancel()
	}()

	client := &http.Client{Timeout: 20 * time.Second}
	if cfg.DeviceID == "" || cfg.DeviceToken == "" {
		if err := enroll(ctx, client, &cfg); err != nil {
			logf("enrollment failed: %v", err)
			os.Exit(1)
		}
		if err := saveConfig(cfg); err != nil {
			logf("failed to save config: %v", err)
			os.Exit(1)
		}
		logf("enrolled as %s", cfg.DeviceID)
	}

	backoff := time.Duration(cfg.IntervalSeconds) * time.Second
	for {
		if err := heartbeatOnce(ctx, client, cfg); err != nil {
			logf("heartbeat failed: %v", err)
			backoff *= 2
			if backoff > 5*time.Minute {
				backoff = 5 * time.Minute
			}
		} else {
			backoff = time.Duration(cfg.IntervalSeconds) * time.Second
		}
		if *once {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func loadConfig(path string) (config, error) {
	cfg := config{
		ServerURL:       envDefault("SERVER_URL", "http://127.0.0.1:8080"),
		EnrollmentToken: envDefault("ENROLLMENT_TOKEN", "dev-enroll-token"),
		IntervalSeconds: intDefault(os.Getenv("INTERVAL_SECONDS"), 30),
		DeviceID:        os.Getenv("DEVICE_ID"),
		DeviceToken:     os.Getenv("DEVICE_TOKEN"),
		LockFile:        envDefault("LOCK_FILE", "/tmp/rmm-agent-go.lock"),
		SpoolDir:        envDefault("SPOOL_DIR", "/tmp/rmm-agent-go-results"),
		BackupDir:       envDefault("BACKUP_DIR", "/tmp/rmm-agent-backups"),
		CheckTargets:    splitWords(envDefault("CHECK_TARGETS", "1.1.1.1 8.8.8.8")),
		ConfigFile:      path,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	values := parseShellConfig(string(data))
	if value := values["SERVER_URL"]; value != "" {
		cfg.ServerURL = value
	}
	if value := values["ENROLLMENT_TOKEN"]; value != "" {
		cfg.EnrollmentToken = value
	}
	if value := values["INTERVAL_SECONDS"]; value != "" {
		cfg.IntervalSeconds = intDefault(value, cfg.IntervalSeconds)
	}
	if value := values["DEVICE_ID"]; value != "" {
		cfg.DeviceID = value
	}
	if value := values["DEVICE_TOKEN"]; value != "" {
		cfg.DeviceToken = value
	}
	if value := values["LOCK_FILE"]; value != "" {
		cfg.LockFile = value
	}
	if value := values["SPOOL_DIR"]; value != "" {
		cfg.SpoolDir = value
	}
	if value := values["BACKUP_DIR"]; value != "" {
		cfg.BackupDir = value
	}
	if value := values["CHECK_TARGETS"]; value != "" {
		cfg.CheckTargets = splitWords(value)
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 30
	}
	return cfg, nil
}

func parseShellConfig(data string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func saveConfig(cfg config) error {
	var b strings.Builder
	writeConfigLine(&b, "SERVER_URL", cfg.ServerURL)
	writeConfigLine(&b, "ENROLLMENT_TOKEN", cfg.EnrollmentToken)
	writeConfigLine(&b, "INTERVAL_SECONDS", strconv.Itoa(cfg.IntervalSeconds))
	writeConfigLine(&b, "CHECK_TARGETS", strings.Join(cfg.CheckTargets, " "))
	writeConfigLine(&b, "BACKUP_DIR", cfg.BackupDir)
	writeConfigLine(&b, "DEVICE_ID", cfg.DeviceID)
	writeConfigLine(&b, "DEVICE_TOKEN", cfg.DeviceToken)
	tmp := cfg.ConfigFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, cfg.ConfigFile)
}

func writeConfigLine(b *strings.Builder, key, value string) {
	_, _ = fmt.Fprintf(b, "%s=\"%s\"\n", key, strings.ReplaceAll(value, `"`, `\"`))
}

func acquireLock(path string) (func(), error) {
	if err := os.Mkdir(path, 0o700); err != nil {
		return nil, fmt.Errorf("another rmm-agent instance is running")
	}
	return func() { _ = os.RemoveAll(path) }, nil
}

func enroll(ctx context.Context, client *http.Client, cfg *config) error {
	body := map[string]string{
		"enrollment_token": cfg.EnrollmentToken,
		"hostname":         hostnameValue(),
		"openwrt_version":  openwrtVersion(),
	}
	var resp enrollResponse
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/agent/enroll", "", body, &resp); err != nil {
		return err
	}
	if resp.DeviceID == "" || resp.DeviceToken == "" {
		return errors.New("server returned empty device credentials")
	}
	cfg.DeviceID = resp.DeviceID
	cfg.DeviceToken = resp.DeviceToken
	return nil
}

func heartbeatOnce(ctx context.Context, client *http.Client, cfg config) error {
	if err := flushSpooledResults(ctx, client, cfg); err != nil {
		logf("spool flush warning: %v", err)
	}
	body := map[string]any{
		"device_id": cfg.DeviceID,
		"inventory": buildInventory(),
		"metrics":   buildMetrics(cfg.CheckTargets),
	}
	var resp heartbeatResponse
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/agent/heartbeat", cfg.DeviceToken, body, &resp); err != nil {
		return err
	}
	for _, cmd := range resp.Commands {
		processCommand(ctx, client, cfg, cmd)
	}
	return nil
}

func buildInventory() map[string]any {
	return map[string]any{
		"hostname":        hostnameValue(),
		"openwrt_version": openwrtVersion(),
		"agent_version":   agentVersion,
		"agent_runtime":   "go",
		"board":           jsonObjectOrEmpty(commandOutput("ubus", "call", "system", "board")),
		"interfaces":      interfaces(),
		"default_route":   firstLine(commandOutput("ip", "route", "show", "default")),
		"wan_ip":          wanIP(),
		"dhcp_leases":     dhcpLeases(),
		"wifi_clients":    []any{},
	}
}

func buildMetrics(targets []string) map[string]any {
	return map[string]any{
		"system":              jsonObjectOrEmpty(commandOutput("ubus", "call", "system", "info")),
		"loadavg":             strings.TrimSpace(readFileString("/proc/loadavg")),
		"uptime":              strings.TrimSpace(readFileString("/proc/uptime")),
		"memory":              memoryInfo(),
		"disk":                diskInfo(),
		"interface_counters":  interfaceCounters(),
		"connectivity_checks": connectivityChecks(targets),
	}
}

func processCommand(ctx context.Context, client *http.Client, cfg config, cmd command) {
	output, exitCode := runCommand(ctx, cfg, cmd)
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	output = redactSensitiveOutput(output)
	result := map[string]any{
		"device_id": cfg.DeviceID,
		"status":    status,
		"exit_code": exitCode,
		"output":    output,
		"result":    map[string]any{"agent_version": agentVersion, "agent_runtime": "go"},
	}
	if err := sendCommandResult(ctx, client, cfg, cmd.ID, result); err != nil {
		logf("failed to send result for %s: %v", cmd.ID, err)
		if err := spoolCommandResult(cfg.SpoolDir, cmd.ID, result); err != nil {
			logf("failed to spool result for %s: %v", cmd.ID, err)
		}
	}
}

func runCommand(ctx context.Context, cfg config, cmd command) (string, int) {
	args := map[string]string{}
	if len(cmd.Args) > 0 {
		_ = json.Unmarshal(cmd.Args, &args)
	}
	switch cmd.Type {
	case "ping":
		target := commandTarget(args, "1.1.1.1")
		if !safeHostName(target) {
			return "target is invalid\n", 2
		}
		return execCommand(ctx, 20*time.Second, "ping", "-c", "4", target)
	case "traceroute":
		target := commandTarget(args, "1.1.1.1")
		if !safeHostName(target) {
			return "target is invalid\n", 2
		}
		return execCommand(ctx, 45*time.Second, "traceroute", target)
	case "route_show":
		return execCommand(ctx, 10*time.Second, "ip", "route", "show")
	case "interfaces_show":
		return execCommand(ctx, 10*time.Second, "ip", "-o", "addr", "show")
	case "pkg_list_installed", "opkg_list_installed":
		return runPackageCommand(ctx, "list_installed")
	case "pkg_update", "opkg_update":
		return runPackageCommand(ctx, "update")
	case "pkg_list_upgradable", "opkg_list_upgradable":
		return runPackageCommand(ctx, "list_upgradable")
	case "pkg_install", "opkg_install":
		packageName := strings.TrimSpace(args["package"])
		if !safePackageName(packageName) {
			return "package name is invalid\n", 2
		}
		return runPackageCommand(ctx, "install", packageName)
	case "pkg_remove", "opkg_remove":
		packageName := strings.TrimSpace(args["package"])
		if !safePackageName(packageName) {
			return "package name is invalid\n", 2
		}
		return runPackageCommand(ctx, "remove", packageName)
	case "uci_show":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		return execCommand(ctx, 10*time.Second, "uci", "show", config)
	case "uci_backup":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		return uciBackupOutput(ctx, config, cfg.BackupDir)
	case "uci_preview":
		target, output, ok := uciTargetArgs(args)
		if !ok {
			return output, 2
		}
		return uciPreviewOutput(ctx, target, cfg.BackupDir)
	case "uci_set":
		target, output, ok := uciTargetArgs(args)
		if !ok {
			return output, 2
		}
		return uciSetOutput(ctx, target, cfg.BackupDir, strings.TrimSpace(args["commit"]) == "true")
	case "uci_commit":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		return execCommand(ctx, 20*time.Second, "uci", "commit", config)
	case "uci_commit_confirmed":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		confirmSeconds := intDefault(args["confirm_seconds"], 15)
		return uciCommitConfirmedOutput(ctx, cfg, config, confirmSeconds)
	case "uci_revert":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		return execCommand(ctx, 10*time.Second, "uci", "revert", config)
	case "uci_restore":
		config, ok := uciConfigArg(args)
		if !ok {
			return "uci config is not allowlisted\n", 2
		}
		return uciRestoreOutput(ctx, config, cfg.BackupDir)
	default:
		return fmt.Sprintf("go agent preview does not implement command %q yet; shell agent remains the production command runner\n", cmd.Type), 2
	}
}

func commandTarget(args map[string]string, fallback string) string {
	target := strings.TrimSpace(args["target"])
	if target == "" {
		return fallback
	}
	return target
}

func safeHostName(value string) bool {
	if value == "" || len(value) > 255 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '.', '-', '_', ':', '[', ']':
			continue
		default:
			return false
		}
	}
	return true
}

func safePackageName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '.', '+', '-':
			continue
		default:
			return false
		}
	}
	return true
}

type uciTarget struct {
	Config  string
	Section string
	Option  string
	Value   string
}

func uciConfigArg(args map[string]string) (string, bool) {
	config := strings.TrimSpace(args["config"])
	if config == "" {
		config = "network"
	}
	return config, safeUCIConfig(config)
}

func uciTargetArgs(args map[string]string) (uciTarget, string, bool) {
	config, ok := uciConfigArg(args)
	if !ok {
		return uciTarget{}, "uci config is not allowlisted\n", false
	}
	target := uciTarget{
		Config:  config,
		Section: strings.TrimSpace(args["section"]),
		Option:  strings.TrimSpace(args["option"]),
		Value:   args["value"],
	}
	if !safeUCISection(target.Section) || !safeUCIOption(target.Option) {
		return uciTarget{}, "uci section or option is invalid\n", false
	}
	return target, "", true
}

func safeUCIConfig(value string) bool {
	switch value {
	case "network", "wireless", "dhcp", "firewall", "system":
		return true
	default:
		return false
	}
}

func safeUCISection(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '.', '@', '[', ']', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func safeUCIOption(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func uciBackupOutput(ctx context.Context, config, backupDir string) (string, int) {
	backup, exitCode := execCommand(ctx, 10*time.Second, "uci", "export", config)
	if exitCode == 0 {
		_ = os.MkdirAll(backupDir, 0o700)
		_ = os.WriteFile(filepath.Join(backupDir, config+".export"), []byte(backup), 0o600)
	}
	return fmt.Sprintf("BACKUP %s\n%s", config, backup), exitCode
}

func uciPreviewOutput(ctx context.Context, target uciTarget, backupDir string) (string, int) {
	before, beforeCode := execCommand(ctx, 10*time.Second, "uci", "show", target.Config)
	if beforeCode != 0 {
		return before, beforeCode
	}
	backup, backupCode := execCommand(ctx, 10*time.Second, "uci", "export", target.Config)
	if backupCode != 0 {
		return backup, backupCode
	}
	_ = os.MkdirAll(backupDir, 0o700)
	_ = os.WriteFile(filepath.Join(backupDir, target.Config+".export"), []byte(backup), 0o600)

	setArg := fmt.Sprintf("%s.%s.%s=%s", target.Config, target.Section, target.Option, target.Value)
	setOutput, setCode := execCommand(ctx, 10*time.Second, "uci", "set", setArg)
	defer func() {
		_, _ = execCommand(context.Background(), 10*time.Second, "uci", "revert", target.Config)
	}()
	if setCode != 0 {
		return setOutput, setCode
	}
	after, afterCode := execCommand(ctx, 10*time.Second, "uci", "show", target.Config)
	if afterCode != 0 {
		return after, afterCode
	}
	diff := unifiedDiff(ctx, before, after)
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "PREVIEW %s.%s.%s\n", target.Config, target.Section, target.Option)
	_, _ = fmt.Fprintf(&b, "\nCHANGE\n%s\n", setArg)
	if diff != "" {
		_, _ = fmt.Fprintf(&b, "\nDIFF\n%s\n", diff)
	}
	_, _ = fmt.Fprintf(&b, "\nBACKUP\n%s\n", backup)
	_, _ = fmt.Fprintf(&b, "\nBEFORE\n%s\n", before)
	_, _ = fmt.Fprintf(&b, "\nAFTER\n%s\n", after)
	return b.String(), 0
}

func uciSetOutput(ctx context.Context, target uciTarget, backupDir string, commit bool) (string, int) {
	backup, backupCode := execCommand(ctx, 10*time.Second, "uci", "export", target.Config)
	if backupCode != 0 {
		return backup, backupCode
	}
	_ = os.MkdirAll(backupDir, 0o700)
	_ = os.WriteFile(filepath.Join(backupDir, target.Config+".export"), []byte(backup), 0o600)
	setArg := fmt.Sprintf("%s.%s.%s=%s", target.Config, target.Section, target.Option, target.Value)
	setOutput, setCode := execCommand(ctx, 10*time.Second, "uci", "set", setArg)
	if setCode != 0 {
		return setOutput, setCode
	}
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "BACKUP\n%s\n\n", backup)
	if commit {
		commitOutput, commitCode := execCommand(ctx, 20*time.Second, "uci", "commit", target.Config)
		_, _ = b.WriteString(commitOutput)
		return b.String(), commitCode
	}
	_, _ = fmt.Fprintf(&b, "uci set staged: %s.%s.%s\n", target.Config, target.Section, target.Option)
	return b.String(), 0
}

func uciCommitConfirmedOutput(ctx context.Context, cfg config, configName string, confirmSeconds int) (string, int) {
	if confirmSeconds <= 0 {
		confirmSeconds = 15
	}
	commitOutput, commitCode := execCommand(ctx, 20*time.Second, "uci", "commit", configName)
	if commitCode != 0 {
		return commitOutput, commitCode
	}
	timer := time.NewTimer(time.Duration(confirmSeconds) * time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return commitOutput + "\ncommit confirmation cancelled\n", 1
	case <-timer.C:
	}
	if serverReachable(ctx, cfg.ServerURL) {
		return fmt.Sprintf("%scommit confirmed: server reachable after %d seconds\n", commitOutput, confirmSeconds), 0
	}
	backupFile := filepath.Join(cfg.BackupDir, configName+".export")
	backup, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Sprintf("%sserver unreachable and no backup file found for %s\n", commitOutput, configName), 1
	}
	importOutput, importCode := execCommandWithInput(ctx, 20*time.Second, string(backup), "uci", "import", configName)
	rollbackCommitOutput, rollbackCommitCode := execCommand(ctx, 20*time.Second, "uci", "commit", configName)
	output := fmt.Sprintf("%s%s%scommit rolled back: server unreachable after %d seconds\n", commitOutput, importOutput, rollbackCommitOutput, confirmSeconds)
	if importCode != 0 {
		return output, importCode
	}
	if rollbackCommitCode != 0 {
		return output, rollbackCommitCode
	}
	return output, 1
}

func uciRestoreOutput(ctx context.Context, config, backupDir string) (string, int) {
	backupFile := filepath.Join(backupDir, config+".export")
	backup, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Sprintf("no backup file found for %s\n", config), 1
	}
	importOutput, importCode := execCommandWithInput(ctx, 20*time.Second, string(backup), "uci", "import", config)
	if importCode != 0 {
		return importOutput, importCode
	}
	commitOutput, commitCode := execCommand(ctx, 20*time.Second, "uci", "commit", config)
	return importOutput + commitOutput, commitCode
}

func serverReachable(ctx context.Context, serverURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serverURL, "/")+"/healthz", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func unifiedDiff(ctx context.Context, before, after string) string {
	if _, err := exec.LookPath("diff"); err != nil {
		return ""
	}
	dir := os.TempDir()
	beforeFile, err := os.CreateTemp(dir, "rmm-agent-before-*")
	if err != nil {
		return ""
	}
	defer os.Remove(beforeFile.Name())
	afterFile, err := os.CreateTemp(dir, "rmm-agent-after-*")
	if err != nil {
		_ = beforeFile.Close()
		return ""
	}
	defer os.Remove(afterFile.Name())
	_, _ = beforeFile.WriteString(before)
	_, _ = afterFile.WriteString(after)
	_ = beforeFile.Close()
	_ = afterFile.Close()
	output, _ := execCommand(ctx, 10*time.Second, "diff", "-u", beforeFile.Name(), afterFile.Name())
	return output
}

func runPackageCommand(ctx context.Context, action string, packageName ...string) (string, int) {
	pm := packageManager()
	pkg := ""
	if len(packageName) > 0 {
		pkg = packageName[0]
	}
	switch pm + ":" + action {
	case "apk:list_installed":
		return execCommand(ctx, 30*time.Second, "apk", "list", "-I")
	case "apk:update":
		return execCommand(ctx, 60*time.Second, "apk", "update")
	case "apk:list_upgradable":
		return execCommand(ctx, 30*time.Second, "apk", "list", "--upgradeable")
	case "apk:install":
		return execCommand(ctx, 120*time.Second, "apk", "add", pkg)
	case "apk:remove":
		return execCommand(ctx, 120*time.Second, "apk", "del", pkg)
	case "opkg:list_installed":
		return execCommand(ctx, 30*time.Second, "opkg", "list-installed")
	case "opkg:update":
		return execCommand(ctx, 60*time.Second, "opkg", "update")
	case "opkg:list_upgradable":
		return execCommand(ctx, 30*time.Second, "opkg", "list-upgradable")
	case "opkg:install":
		return execCommand(ctx, 120*time.Second, "opkg", "install", pkg)
	case "opkg:remove":
		return execCommand(ctx, 120*time.Second, "opkg", "remove", pkg)
	default:
		return "no supported package manager found\n", 2
	}
}

func packageManager() string {
	if _, err := exec.LookPath("apk"); err == nil {
		return "apk"
	}
	if _, err := exec.LookPath("opkg"); err == nil {
		return "opkg"
	}
	return "none"
}

func execCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, int) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, name, args...)
	data, err := cmd.CombinedOutput()
	output := string(data)
	if cmdCtx.Err() == context.DeadlineExceeded {
		return output + "\ncommand timed out\n", 124
	}
	if err == nil {
		return output, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return output, exitErr.ExitCode()
	}
	if output != "" {
		return output, 1
	}
	return err.Error() + "\n", 1
}

func execCommandWithInput(ctx context.Context, timeout time.Duration, input, name string, args ...string) (string, int) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	data, err := cmd.CombinedOutput()
	output := string(data)
	if cmdCtx.Err() == context.DeadlineExceeded {
		return output + "\ncommand timed out\n", 124
	}
	if err == nil {
		return output, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return output, exitErr.ExitCode()
	}
	if output != "" {
		return output, 1
	}
	return err.Error() + "\n", 1
}

func redactSensitiveOutput(output string) string {
	words := []string{"private_key", "password", "passwd", "secret", "psk", "token"}
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, word := range words {
			if strings.Contains(lower, word) {
				lines[i] = redactLine(line)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func redactLine(line string) string {
	if idx := strings.Index(line, "="); idx >= 0 {
		return line[:idx+1] + "[redacted]"
	}
	return "[redacted]"
}

func sendCommandResult(ctx context.Context, client *http.Client, cfg config, commandID string, body any) error {
	status, err := postJSONStatus(ctx, client, cfg.ServerURL+"/api/agent/commands/"+commandID+"/result", cfg.DeviceToken, body)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status >= 400 && status < 500 {
		logf("server rejected result for %s with HTTP %d", commandID, status)
		return nil
	}
	return fmt.Errorf("server returned HTTP %d", status)
}

func spoolCommandResult(dir, commandID string, body any) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, commandID+".json"), data, 0o600)
}

func flushSpooledResults(ctx context.Context, client *http.Client, cfg config) error {
	entries, err := os.ReadDir(cfg.SpoolDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(cfg.SpoolDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		commandID := strings.TrimSuffix(entry.Name(), ".json")
		status, err := postRawJSONStatus(ctx, client, cfg.ServerURL+"/api/agent/commands/"+commandID+"/result", cfg.DeviceToken, data)
		if err != nil {
			continue
		}
		if (status >= 200 && status < 300) || (status >= 400 && status < 500) {
			_ = os.Remove(path)
		}
	}
	return nil
}

func postJSON(ctx context.Context, client *http.Client, url, token string, body any, out any) error {
	status, data, err := postJSONRaw(ctx, client, url, token, body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func postJSONStatus(ctx context.Context, client *http.Client, url, token string, body any) (int, error) {
	status, _, err := postJSONRaw(ctx, client, url, token, body)
	return status, err
}

func postRawJSONStatus(ctx context.Context, client *http.Client, url, token string, data []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func postJSONRaw(ctx context.Context, client *http.Client, url, token string, body any) (int, []byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respData, nil
}

func hostnameValue() string {
	if value, err := os.Hostname(); err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return "unknown"
}

func openwrtVersion() string {
	values := parseShellConfig(readFileString("/etc/openwrt_release"))
	if value := values["DISTRIB_DESCRIPTION"]; value != "" {
		return value
	}
	return "unknown"
}

func interfaces() []map[string]string {
	output := commandOutput("ip", "-o", "addr", "show")
	var result []map[string]string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		result = append(result, map[string]string{"name": strings.TrimSuffix(fields[1], ":"), "family": fields[2], "address": fields[3]})
	}
	if result == nil {
		return []map[string]string{}
	}
	return result
}

func wanIP() string {
	route := firstLine(commandOutput("ip", "route", "show", "default"))
	fields := strings.Fields(route)
	dev := ""
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "dev" {
			dev = fields[i+1]
			break
		}
	}
	if dev == "" {
		return ""
	}
	output := commandOutput("ip", "-4", "-o", "addr", "show", "dev", dev)
	fields = strings.Fields(output)
	if len(fields) >= 4 {
		return fields[3]
	}
	return ""
}

func dhcpLeases() []map[string]string {
	data := readFileString("/tmp/dhcp.leases")
	var leases []map[string]string
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		leases = append(leases, map[string]string{
			"expires": fields[0], "mac": fields[1], "ip": fields[2], "hostname": fields[3], "client_id": fields[4],
		})
	}
	if leases == nil {
		return []map[string]string{}
	}
	return leases
}

func memoryInfo() map[string]int64 {
	values := map[string]int64{}
	for _, line := range strings.Split(readFileString("/proc/meminfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseInt(fields[1], 10, 64)
		values[strings.TrimSuffix(fields[0], ":")] = value
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	used := total - available
	if available == 0 {
		used = total - values["MemFree"] - values["Buffers"] - values["Cached"]
	}
	return map[string]int64{"total_kb": total, "free_kb": values["MemFree"], "available_kb": available, "used_kb": used}
}

func diskInfo() map[string]any {
	output := commandOutput("df", "-k", "/")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return map[string]any{}
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return map[string]any{}
	}
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	used, _ := strconv.ParseInt(fields[2], 10, 64)
	available, _ := strconv.ParseInt(fields[3], 10, 64)
	return map[string]any{"filesystem": fields[0], "total_kb": total, "used_kb": used, "available_kb": available, "used_percent": fields[4]}
}

func interfaceCounters() []map[string]any {
	data := readFileString("/proc/net/dev")
	var counters []map[string]any
	for _, line := range strings.Split(data, "\n")[2:] {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 12 {
			continue
		}
		counters = append(counters, map[string]any{
			"name": name, "rx_bytes": intField(fields, 0), "rx_packets": intField(fields, 1), "rx_errors": intField(fields, 2),
			"tx_bytes": intField(fields, 8), "tx_packets": intField(fields, 9), "tx_errors": intField(fields, 10),
		})
	}
	if counters == nil {
		return []map[string]any{}
	}
	return counters
}

func connectivityChecks(targets []string) []map[string]any {
	var checks []map[string]any
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		output := commandOutput("ping", "-c", "3", "-W", "2", target)
		loss := parsePacketLoss(output)
		latency := parseLatency(output)
		checks = append(checks, map[string]any{"target": target, "reachable": loss < 100, "packet_loss_percent": loss, "latency_ms": latency})
	}
	if checks == nil {
		return []map[string]any{}
	}
	return checks
}

func parsePacketLoss(output string) float64 {
	for _, part := range strings.Split(output, ",") {
		if strings.Contains(part, "packet loss") {
			clean := strings.TrimSpace(strings.ReplaceAll(part, "% packet loss", ""))
			value, err := strconv.ParseFloat(clean, 64)
			if err == nil {
				return value
			}
		}
	}
	return 100
}

func parseLatency(output string) float64 {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "min/avg/max") && !strings.Contains(line, "round-trip") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) < 2 {
			continue
		}
		values := strings.Split(strings.TrimSpace(parts[1]), "/")
		if len(values) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(values[1]), 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func jsonObjectOrEmpty(output string) map[string]any {
	var value map[string]any
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		return map[string]any{}
	}
	return value
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	data, _ := cmd.CombinedOutput()
	return string(data)
}

func readFileString(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		return value[:idx]
	}
	return value
}

func intField(fields []string, index int) int64 {
	if index >= len(fields) {
		return 0
	}
	value, _ := strconv.ParseInt(fields[index], 10, 64)
	return value
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func splitWords(value string) []string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return []string{}
	}
	return fields
}

func randomID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
