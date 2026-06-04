package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

type Store interface {
	EnrollDevice(ctx context.Context, hostname, openwrtVersion string) (store.EnrolledDevice, error)
	AuthorizeDevice(ctx context.Context, deviceID, token string) (bool, error)
	SaveHeartbeat(ctx context.Context, deviceID string, inventory, metrics json.RawMessage) ([]model.Command, error)
	ClaimNextCommand(ctx context.Context, deviceID string) (model.Command, bool, error)
	SaveCommandResult(ctx context.Context, commandID, deviceID, status string, exitCode int, output string, result json.RawMessage) (bool, error)
	ListDevices(ctx context.Context) ([]model.Device, error)
	GetDevice(ctx context.Context, deviceID string) (model.Device, bool, error)
	UpdateDeviceFleet(ctx context.Context, deviceID, group string, tags []string) (model.Device, bool, error)
	ListMetricSamples(ctx context.Context, deviceID string, opts store.MetricHistoryOptions) ([]model.MetricSample, bool, error)
	SyncDeviceAlerts(ctx context.Context, deviceID string, active []model.Alert) ([]model.Alert, bool, error)
	ListAlerts(ctx context.Context, opts store.AlertListOptions) ([]model.Alert, error)
	AcknowledgeAlert(ctx context.Context, deviceID, alertID, actor string) (model.Alert, bool, error)
	CreateCommand(ctx context.Context, deviceID, commandType string, args json.RawMessage) (model.Command, bool, error)
	ListCommands(ctx context.Context, deviceID string, opts store.CommandListOptions) ([]model.Command, bool, error)
	GetCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error)
	CancelCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error)
	CreateRemoteSession(ctx context.Context, session model.RemoteSession) (model.RemoteSession, bool, error)
	ListRemoteSessions(ctx context.Context, deviceID string, opts store.RemoteSessionListOptions) ([]model.RemoteSession, bool, error)
	GetRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error)
	AttachRemoteSessionCommand(ctx context.Context, deviceID, sessionID, commandID string) (model.RemoteSession, bool, error)
	CloseRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error)
	ExpireRemoteSessions(ctx context.Context) error
	AddAuditEvent(ctx context.Context, actor, action, deviceID, commandID string, details json.RawMessage) (model.AuditEvent, error)
	ListAuditEvents(ctx context.Context, opts store.AuditListOptions) ([]model.AuditEvent, error)
}

type Config struct {
	EnrollmentToken  string
	OperatorToken    string
	OperatorUsername string
	OperatorPassword string
	SessionSecret    string
	CookieSecure     bool
	TunnelHTTPHost   string
	StaticDir        string
}

type App struct {
	store            Store
	enrollmentToken  string
	operatorToken    string
	operatorUsername string
	operatorPassword string
	sessionSecret    []byte
	cookieSecure     bool
	tunnelHTTPHost   string
}

type contextKey string

const requestIDContextKey contextKey = "request_id"

type enrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	Hostname        string `json:"hostname"`
	OpenWrtVersion  string `json:"openwrt_version"`
}

type enrollResponse struct {
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token"`
}

type heartbeatRequest struct {
	DeviceID  string          `json:"device_id"`
	Inventory json.RawMessage `json:"inventory"`
	Metrics   json.RawMessage `json:"metrics"`
}

type heartbeatResponse struct {
	Commands []model.Command `json:"commands"`
}

type commandRequest struct {
	Type string          `json:"type"`
	Args json.RawMessage `json:"args"`
}

type remoteSessionRequest struct {
	Target          string `json:"target"`
	DurationSeconds int    `json:"duration_seconds"`
	ServerHost      string `json:"server_host"`
	ServerPort      int    `json:"server_port"`
	RemotePort      int    `json:"remote_port"`
	LocalPort       int    `json:"local_port"`
	LuCIScheme      string `json:"luci_scheme"`
}

type fleetRequest struct {
	Group string   `json:"group"`
	Tags  []string `json:"tags"`
}

type bulkCommandRequest struct {
	DeviceIDs []string        `json:"device_ids"`
	Type      string          `json:"type"`
	Args      json.RawMessage `json:"args"`
}

type nextCommandRequest struct {
	DeviceID string `json:"device_id"`
}

type commandResultRequest struct {
	DeviceID string          `json:"device_id"`
	Status   string          `json:"status"`
	ExitCode int             `json:"exit_code"`
	Output   string          `json:"output"`
	Result   json.RawMessage `json:"result"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHandler(s Store, cfg Config) http.Handler {
	if strings.TrimSpace(cfg.OperatorUsername) == "" {
		cfg.OperatorUsername = "admin"
	}
	if cfg.OperatorPassword == "" {
		cfg.OperatorPassword = cfg.OperatorToken
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = cfg.OperatorToken
	}
	a := &App{
		store:            s,
		enrollmentToken:  cfg.EnrollmentToken,
		operatorToken:    cfg.OperatorToken,
		operatorUsername: cfg.OperatorUsername,
		operatorPassword: cfg.OperatorPassword,
		sessionSecret:    []byte(cfg.SessionSecret),
		cookieSecure:     cfg.CookieSecure,
		tunnelHTTPHost:   strings.TrimSpace(cfg.TunnelHTTPHost),
	}
	if a.tunnelHTTPHost == "" {
		a.tunnelHTTPHost = "tunnel-ssh"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.Handle("GET /api/auth/me", a.operatorAuth(http.HandlerFunc(a.handleAuthMe)))
	mux.HandleFunc("POST /api/agent/enroll", a.handleEnroll)
	mux.HandleFunc("POST /api/agent/heartbeat", a.handleHeartbeat)
	mux.HandleFunc("POST /api/agent/commands/next", a.handleNextCommand)
	mux.HandleFunc("POST /api/agent/commands/", a.handleCommandResult)
	mux.Handle("GET /api/devices", a.operatorAuth(http.HandlerFunc(a.handleListDevices)))
	mux.Handle("POST /api/devices/bulk-commands", a.operatorAuth(http.HandlerFunc(a.handleCreateBulkCommand)))
	mux.Handle("GET /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("POST /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("PATCH /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("GET /api/audit-events", a.operatorAuth(http.HandlerFunc(a.handleListAuditEvents)))
	mux.Handle("GET /luci/", a.operatorAuth(http.HandlerFunc(a.handleLuCIProxy)))
	mux.Handle("POST /luci/", a.operatorAuth(http.HandlerFunc(a.handleLuCIProxy)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if cfg.StaticDir != "" {
		mux.Handle("GET /", staticHandler(cfg.StaticDir))
	}

	return withRequestLogging(mux)
}

func (a *App) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.EnrollmentToken != a.enrollmentToken {
		writeError(w, http.StatusUnauthorized, "invalid enrollment token")
		return
	}

	enrolled, err := a.store.EnrollDevice(r.Context(), req.Hostname, req.OpenWrtVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enroll device")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "agent", "agent.enroll", enrolled.DeviceID, "", mustJSON(map[string]string{
		"hostname":        req.Hostname,
		"openwrt_version": req.OpenWrtVersion,
		"request_id":      requestID(r.Context()),
	}))

	writeJSON(w, http.StatusCreated, enrollResponse{DeviceID: enrolled.DeviceID, DeviceToken: enrolled.DeviceToken})
}

func (a *App) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req heartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.authorizeDevice(r, req.DeviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	commands, err := a.store.SaveHeartbeat(r.Context(), req.DeviceID, req.Inventory, req.Metrics)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save heartbeat")
		return
	}
	if _, _, err := a.refreshDeviceAlerts(r.Context(), req.DeviceID); err != nil {
		log.Printf("refresh alerts for %s: %v", req.DeviceID, err)
	}

	writeJSON(w, http.StatusOK, heartbeatResponse{Commands: commands})
}

func (a *App) handleNextCommand(w http.ResponseWriter, r *http.Request) {
	var req nextCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.authorizeDevice(r, req.DeviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	c, ok, err := a.store.ClaimNextCommand(r.Context(), req.DeviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load command")
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "agent", "command.claim", req.DeviceID, c.ID, mustJSON(map[string]string{
		"type":       c.Type,
		"request_id": requestID(r.Context()),
	}))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(c.ID + "\t" + c.Type + "\t" + string(c.Args) + "\n"))
}

func (a *App) handleCommandResult(w http.ResponseWriter, r *http.Request) {
	id, ok := commandIDFromResultPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req commandResultRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.authorizeDevice(r, req.DeviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	status := req.Status
	if status == "" {
		status = "completed"
	}
	if status != "completed" && status != "failed" {
		writeError(w, http.StatusBadRequest, "invalid command status")
		return
	}

	found, err := a.store.SaveCommandResult(r.Context(), id, req.DeviceID, status, req.ExitCode, req.Output, req.Result)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save command result")
		return
	}
	if !found {
		_, _ = a.store.AddAuditEvent(r.Context(), "agent", "command.result_rejected", req.DeviceID, id, mustJSON(map[string]string{
			"status":     status,
			"request_id": requestID(r.Context()),
		}))
		writeError(w, http.StatusNotFound, "command not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "agent", "command.result", req.DeviceID, id, mustJSON(map[string]string{
		"status":     status,
		"request_id": requestID(r.Context()),
	}))

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := a.store.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load devices")
		return
	}
	for _, device := range devices {
		if _, _, err := a.refreshDeviceAlerts(r.Context(), device.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to refresh device alerts")
			return
		}
	}
	devices, err = a.store.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load devices")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (a *App) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := deviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	d, found, err := a.store.GetDevice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	writeJSON(w, http.StatusOK, d)
}

func (a *App) handleDeviceSubtree(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 3 && r.Method == http.MethodGet {
		a.handleGetDevice(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "fleet" && r.Method == http.MethodPatch {
		a.handleUpdateDeviceFleet(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "metrics-history" && r.Method == http.MethodGet {
		a.handleListMetricSamples(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "alerts" && r.Method == http.MethodGet {
		a.handleListAlerts(w, r)
		return
	}
	if len(parts) == 6 && parts[3] == "alerts" && parts[5] == "acknowledge" && r.Method == http.MethodPost {
		a.handleAcknowledgeAlert(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "commands" && r.Method == http.MethodGet {
		a.handleListCommands(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "commands" && r.Method == http.MethodPost {
		a.handleCreateCommand(w, r)
		return
	}
	if len(parts) == 5 && parts[3] == "commands" && r.Method == http.MethodGet {
		a.handleGetCommand(w, r)
		return
	}
	if len(parts) == 6 && parts[3] == "commands" && parts[5] == "cancel" && r.Method == http.MethodPost {
		a.handleCancelCommand(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "remote-sessions" && r.Method == http.MethodGet {
		a.handleListRemoteSessions(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "remote-sessions" && r.Method == http.MethodPost {
		a.handleCreateRemoteSession(w, r)
		return
	}
	if len(parts) == 6 && parts[3] == "remote-sessions" && parts[5] == "close" && r.Method == http.MethodPost {
		a.handleCloseRemoteSession(w, r)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (a *App) handleUpdateDeviceFleet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "fleet" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deviceID := parts[2]
	var req fleetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	d, found, err := a.store.UpdateDeviceFleet(r.Context(), deviceID, req.Group, req.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update device fleet metadata")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "device.fleet_update", deviceID, "", mustJSON(map[string]any{
		"group":      req.Group,
		"tags":       req.Tags,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, d)
}

func (a *App) handleListMetricSamples(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "metrics-history" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	samples, found, err := a.store.ListMetricSamples(r.Context(), parts[2], store.MetricHistoryOptions{Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load metrics history")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"samples": samples})
}

func (a *App) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "alerts" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	alerts, found, err := a.refreshDeviceAlerts(r.Context(), parts[2])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load alerts")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts})
}

func (a *App) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	deviceID, alertID, ok := alertAcknowledgeIDsFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	alert, changed, err := a.store.AcknowledgeAlert(r.Context(), deviceID, alertID, "operator")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to acknowledge alert")
		return
	}
	if !changed {
		writeError(w, http.StatusNotFound, "active alert not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "alert.acknowledge", deviceID, "", mustJSON(map[string]string{
		"alert_id":   alertID,
		"type":       alert.Type,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, alert)
}

func (a *App) refreshDeviceAlerts(ctx context.Context, deviceID string) ([]model.Alert, bool, error) {
	active, found, err := a.computeDeviceAlerts(ctx, deviceID)
	if err != nil || !found {
		return nil, found, err
	}
	return a.store.SyncDeviceAlerts(ctx, deviceID, active)
}

func (a *App) computeDeviceAlerts(ctx context.Context, deviceID string) ([]model.Alert, bool, error) {
	device, found, err := a.store.GetDevice(ctx, deviceID)
	if err != nil || !found {
		return nil, found, err
	}
	samples, samplesFound, err := a.store.ListMetricSamples(ctx, deviceID, store.MetricHistoryOptions{Limit: 3})
	if err != nil {
		return nil, true, err
	}
	if !samplesFound {
		samples = nil
	}
	commands, commandsFound, err := a.store.ListCommands(ctx, deviceID, store.CommandListOptions{Limit: 50})
	if err != nil {
		return nil, true, err
	}
	if !commandsFound {
		commands = nil
	}

	now := time.Now().UTC()
	alerts := make([]model.Alert, 0, 4)
	if !device.Online {
		details := map[string]any{"last_seen_at": device.LastSeenAt}
		alerts = append(alerts, newAlert(deviceID, "offline", "critical", "Device is offline", details, now))
	}
	if ratio, ok := jsonRatio(device.Metrics, "memory", "used_kb", "total_kb"); ok && ratio >= 0.85 {
		alerts = append(alerts, newUsageAlert(deviceID, "memory_high", "Memory usage is high", ratio, now))
	}
	if ratio, ok := diskUsageRatio(device.Metrics); ok && ratio >= 0.85 {
		alerts = append(alerts, newUsageAlert(deviceID, "disk_high", "Disk usage is high", ratio, now))
	}
	alerts = append(alerts, connectivityAlerts(deviceID, device.Metrics, now)...)
	if len(samples) >= 2 {
		latestWAN := jsonString(samples[0].Inventory, "wan_ip")
		previousWAN := jsonString(samples[1].Inventory, "wan_ip")
		if latestWAN != "" && previousWAN != "" && latestWAN != previousWAN {
			alerts = append(alerts, newAlert(deviceID, "wan_ip_changed", "warning", "WAN IP changed", map[string]string{
				"current":  latestWAN,
				"previous": previousWAN,
			}, now))
		}
	}
	if failedCount, latest := commandAttention(commands); failedCount > 0 {
		alerts = append(alerts, newAlert(deviceID, "command_attention", "warning", "Recent commands need attention", map[string]any{
			"count":     failedCount,
			"latest_at": latest,
		}, now))
	}
	return alerts, true, nil
}

func connectivityAlerts(deviceID string, raw json.RawMessage, now time.Time) []model.Alert {
	checks := jsonObjectArray(raw, "connectivity_checks")
	if len(checks) == 0 {
		return nil
	}

	reachableCount := 0
	worstLoss := 0.0
	worstLatency := 0.0
	worstLossTarget := ""
	worstLatencyTarget := ""
	for _, check := range checks {
		target, _ := check["target"].(string)
		if reachable, ok := check["reachable"].(bool); ok && reachable {
			reachableCount++
		}
		if loss, ok := numberValue(check["packet_loss_percent"]); ok && loss > worstLoss {
			worstLoss = loss
			worstLossTarget = target
		}
		if latency, ok := numberValue(check["latency_ms"]); ok && latency > worstLatency {
			worstLatency = latency
			worstLatencyTarget = target
		}
	}

	alerts := make([]model.Alert, 0, 3)
	if reachableCount == 0 {
		alerts = append(alerts, newAlert(deviceID, "wan_down", "critical", "WAN connectivity checks are failing", map[string]any{
			"targets": len(checks),
		}, now))
	}
	if worstLoss >= 20 {
		severity := "warning"
		if worstLoss >= 80 {
			severity = "critical"
		}
		alerts = append(alerts, newAlert(deviceID, "packet_loss_high", severity, "Packet loss is high", map[string]any{
			"target":               worstLossTarget,
			"packet_loss_percent":  int(worstLoss),
			"reachable_target_cnt": reachableCount,
		}, now))
	}
	if worstLatency >= 200 {
		severity := "warning"
		if worstLatency >= 500 {
			severity = "critical"
		}
		alerts = append(alerts, newAlert(deviceID, "latency_high", severity, "Latency is high", map[string]any{
			"target":     worstLatencyTarget,
			"latency_ms": int(worstLatency),
		}, now))
	}
	return alerts
}

func newUsageAlert(deviceID, alertType, message string, ratio float64, now time.Time) model.Alert {
	severity := "warning"
	if ratio >= 0.95 {
		severity = "critical"
	}
	return newAlert(deviceID, alertType, severity, message, map[string]any{
		"used_percent": int(ratio * 100),
	}, now)
}

func newAlert(deviceID, alertType, severity, message string, details any, createdAt time.Time) model.Alert {
	return model.Alert{
		ID:          alertType + ":" + deviceID,
		DeviceID:    deviceID,
		Type:        alertType,
		Severity:    severity,
		Status:      "active",
		Message:     message,
		Details:     mustJSON(details),
		FirstSeenAt: createdAt,
		LastSeenAt:  createdAt,
		CreatedAt:   createdAt,
	}
}

func commandAttention(commands []model.Command) (int, *time.Time) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	count := 0
	var latest *time.Time
	for _, command := range commands {
		if command.Status != "failed" && command.Status != "expired" {
			continue
		}
		at := command.CompletedAt
		if at == nil {
			at = command.ExpiredAt
		}
		if at == nil {
			at = &command.CreatedAt
		}
		if at.Before(cutoff) {
			continue
		}
		count++
		if latest == nil || at.After(*latest) {
			latestAt := *at
			latest = &latestAt
		}
	}
	return count, latest
}

func jsonRatio(raw json.RawMessage, objectKey, usedKey, totalKey string) (float64, bool) {
	var root map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return 0, false
	}
	obj, ok := root[objectKey].(map[string]any)
	if !ok {
		return 0, false
	}
	used, usedOK := numberValue(obj[usedKey])
	total, totalOK := numberValue(obj[totalKey])
	if !usedOK || !totalOK || total <= 0 {
		return 0, false
	}
	return used / total, true
}

func diskUsageRatio(raw json.RawMessage) (float64, bool) {
	var root map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return 0, false
	}
	disk, ok := root["disk"].(map[string]any)
	if !ok {
		return 0, false
	}
	if value, ok := numberValue(disk["used_percent"]); ok {
		if value > 1 {
			return value / 100, true
		}
		return value, true
	}
	used, usedOK := numberValue(disk["used_kb"])
	total, totalOK := numberValue(disk["total_kb"])
	if !usedOK || !totalOK || total <= 0 {
		return 0, false
	}
	return used / total, true
}

func jsonString(raw json.RawMessage, key string) string {
	var root map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return ""
	}
	value, ok := root[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func jsonObjectArray(raw json.RawMessage, key string) []map[string]any {
	var root map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return nil
	}
	items, ok := root[key].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if ok {
			out = append(out, obj)
		}
	}
	return out
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		trimmed := strings.TrimSuffix(strings.TrimSpace(v), "%")
		parsed, err := strconv.ParseFloat(trimmed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (a *App) handleCreateCommand(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := commandDeviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req commandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "command type is required")
		return
	}
	if !AllowedCommandType(req.Type) {
		writeError(w, http.StatusBadRequest, "command type is not allowed")
		return
	}

	c, found, err := a.store.CreateCommand(r.Context(), deviceID, req.Type, req.Args)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create command")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "command.create", deviceID, c.ID, mustJSON(map[string]string{
		"type":       req.Type,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusCreated, c)
}

func (a *App) handleCreateBulkCommand(w http.ResponseWriter, r *http.Request) {
	var req bulkCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "command type is required")
		return
	}
	if !AllowedCommandType(req.Type) {
		writeError(w, http.StatusBadRequest, "command type is not allowed")
		return
	}
	if len(req.DeviceIDs) == 0 {
		writeError(w, http.StatusBadRequest, "device_ids are required")
		return
	}
	if len(req.DeviceIDs) > 100 {
		writeError(w, http.StatusBadRequest, "too many devices")
		return
	}

	commands := make([]model.Command, 0, len(req.DeviceIDs))
	for _, deviceID := range uniqueStrings(req.DeviceIDs) {
		c, found, err := a.store.CreateCommand(r.Context(), deviceID, req.Type, req.Args)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create command")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "device not found: "+deviceID)
			return
		}
		commands = append(commands, c)
		_, _ = a.store.AddAuditEvent(r.Context(), "operator", "command.bulk_create", deviceID, c.ID, mustJSON(map[string]string{
			"type":       req.Type,
			"request_id": requestID(r.Context()),
		}))
	}
	writeJSON(w, http.StatusCreated, map[string]any{"commands": commands})
}

func (a *App) handleListCommands(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := commandListDeviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	commands, found, err := a.store.ListCommands(r.Context(), deviceID, store.CommandListOptions{Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load commands")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"commands": commands})
}

func (a *App) handleGetCommand(w http.ResponseWriter, r *http.Request) {
	deviceID, commandID, ok := commandDetailIDsFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	c, found, err := a.store.GetCommand(r.Context(), deviceID, commandID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load command")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "command not found")
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (a *App) handleCancelCommand(w http.ResponseWriter, r *http.Request) {
	deviceID, commandID, ok := commandCancelIDsFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	c, found, err := a.store.CancelCommand(r.Context(), deviceID, commandID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel command")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "command not found")
		return
	}

	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "command.cancel", deviceID, commandID, mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, c)
}

func (a *App) handleListRemoteSessions(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := remoteSessionListDeviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	sessions, found, err := a.store.ListRemoteSessions(r.Context(), deviceID, store.RemoteSessionListOptions{Limit: limit})
	if err != nil {
		logStructured(map[string]any{
			"event":      "remote_sessions.list_failed",
			"request_id": requestID(r.Context()),
			"device_id":  deviceID,
			"error":      err.Error(),
		})
		writeError(w, http.StatusInternalServerError, "failed to load remote sessions")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"remote_sessions": sessions})
}

func (a *App) handleCreateRemoteSession(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := remoteSessionListDeviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req remoteSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "ssh"
	}
	if target != "ssh" {
		writeError(w, http.StatusBadRequest, "only ssh remote sessions are supported")
		return
	}
	duration := req.DurationSeconds
	if duration <= 0 {
		duration = 15 * 60
	}
	if duration < 60 || duration > 2*60*60 {
		writeError(w, http.StatusBadRequest, "duration_seconds must be between 60 and 7200")
		return
	}
	serverHost := strings.TrimSpace(req.ServerHost)
	if serverHost == "" {
		serverHost = hostWithoutPort(r.Host)
	}
	if serverHost == "" || !safeEndpointHost(serverHost) {
		writeError(w, http.StatusBadRequest, "server_host is invalid")
		return
	}
	serverPort := req.ServerPort
	if serverPort <= 0 {
		serverPort = 22
	}
	localPort := req.LocalPort
	if localPort <= 0 {
		localPort = 22
	}
	remotePort := req.RemotePort
	if remotePort <= 0 {
		remotePort = randomRemotePort()
	}
	luciPort := randomLuCIPort()
	luciScheme := strings.ToLower(strings.TrimSpace(req.LuCIScheme))
	if luciScheme == "" {
		luciScheme = "http"
	}
	if luciScheme != "http" && luciScheme != "https" {
		writeError(w, http.StatusBadRequest, "luci_scheme must be http or https")
		return
	}
	if !validTCPPort(serverPort) || !validTCPPort(localPort) || !validTCPPort(remotePort) || !validTCPPort(luciPort) {
		writeError(w, http.StatusBadRequest, "ports must be between 1 and 65535")
		return
	}

	expiresAt := time.Now().UTC().Add(time.Duration(duration) * time.Second)
	session, found, err := a.store.CreateRemoteSession(r.Context(), model.RemoteSession{
		DeviceID:   deviceID,
		Target:     target,
		Status:     "requested",
		ServerHost: serverHost,
		ServerPort: serverPort,
		RemotePort: remotePort,
		LuCIPort:   luciPort,
		LuCIScheme: luciScheme,
		LocalHost:  "127.0.0.1",
		LocalPort:  localPort,
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create remote session")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	luciLocalPort := "80"
	if session.LuCIScheme == "https" {
		luciLocalPort = "443"
	}
	args := mustJSON(map[string]any{
		"session_id":       session.ID,
		"server_host":      session.ServerHost,
		"server_port":      strconv.Itoa(session.ServerPort),
		"remote_port":      strconv.Itoa(session.RemotePort),
		"luci_port":        strconv.Itoa(session.LuCIPort),
		"luci_local_port":  luciLocalPort,
		"local_host":       session.LocalHost,
		"local_port":       strconv.Itoa(session.LocalPort),
		"server_user":      "rmm-tunnel",
		"duration_seconds": strconv.Itoa(duration),
		"expires_at":       session.ExpiresAt.Format(time.RFC3339Nano),
	})
	command, commandFound, err := a.store.CreateCommand(r.Context(), deviceID, "remote_ssh_reverse", args)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue remote tunnel command")
		return
	}
	if !commandFound {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	session, _, err = a.store.AttachRemoteSessionCommand(r.Context(), deviceID, session.ID, command.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to attach remote session command")
		return
	}

	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "remote_session.create", deviceID, command.ID, mustJSON(map[string]any{
		"session_id":  session.ID,
		"target":      session.Target,
		"server_host": session.ServerHost,
		"server_port": session.ServerPort,
		"remote_port": session.RemotePort,
		"luci_port":   session.LuCIPort,
		"luci_scheme": session.LuCIScheme,
		"local_port":  session.LocalPort,
		"expires_at":  session.ExpiresAt,
		"request_id":  requestID(r.Context()),
	}))
	writeJSON(w, http.StatusCreated, session)
}

func (a *App) handleLuCIProxy(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/luci/")
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "LuCI session not found")
		return
	}
	deviceID, sessionID := parts[0], parts[1]
	session, found, err := a.store.GetRemoteSession(r.Context(), deviceID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LuCI session")
		return
	}
	if !found || session.Status != "active" || session.LuCIPort <= 0 {
		writeError(w, http.StatusNotFound, "LuCI session is not active")
		return
	}

	prefix := "/luci/" + deviceID + "/" + sessionID
	scheme := session.LuCIScheme
	if scheme == "" {
		scheme = "http"
	}
	upstream, _ := url.Parse(scheme + "://" + a.tunnelHTTPHost + ":" + strconv.Itoa(session.LuCIPort))
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	if scheme == "https" {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // LuCI commonly uses a router-local self-signed certificate.
		proxy.Transport = transport
	}
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/"
		if len(parts) == 3 && parts[2] != "" {
			req.URL.Path += parts[2]
		}
		req.Host = upstream.Host
		req.Header.Del("Accept-Encoding")
		removeCookie(req, operatorSessionCookie)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		rewriteLuCIHeaders(resp.Header, prefix)
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "javascript") && !strings.Contains(contentType, "text/css") {
			return nil
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		body = rewriteLuCIBody(body, prefix)
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		logStructured(map[string]any{
			"event":      "luci.proxy_failed",
			"request_id": requestID(r.Context()),
			"device_id":  deviceID,
			"session_id": sessionID,
			"error":      err.Error(),
		})
		writeError(w, http.StatusBadGateway, "LuCI is unreachable through this session")
	}
	proxy.ServeHTTP(w, r)
}

func removeCookie(r *http.Request, name string) {
	cookies := make([]string, 0)
	for _, cookie := range r.Cookies() {
		if cookie.Name != name {
			cookies = append(cookies, cookie.Name+"="+cookie.Value)
		}
	}
	if len(cookies) == 0 {
		r.Header.Del("Cookie")
		return
	}
	r.Header.Set("Cookie", strings.Join(cookies, "; "))
}

func rewriteLuCIHeaders(header http.Header, prefix string) {
	if location := header.Get("Location"); strings.HasPrefix(location, "/") {
		header.Set("Location", prefix+location)
	}
	for index, cookie := range header.Values("Set-Cookie") {
		cookie = strings.ReplaceAll(cookie, "Path=/;", "Path="+prefix+"/;")
		cookie = strings.ReplaceAll(cookie, "Path=/ ", "Path="+prefix+"/ ")
		header["Set-Cookie"][index] = cookie
	}
}

func rewriteLuCIBody(body []byte, prefix string) []byte {
	for _, marker := range []string{`="/`, `'/`, `url(/`} {
		replacement := marker[:len(marker)-1] + prefix + "/"
		body = bytes.ReplaceAll(body, []byte(marker), []byte(replacement))
	}
	return body
}

func (a *App) handleCloseRemoteSession(w http.ResponseWriter, r *http.Request) {
	deviceID, sessionID, ok := remoteSessionCloseIDsFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	session, found, err := a.store.GetRemoteSession(r.Context(), deviceID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close remote session")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "remote session not found")
		return
	}
	closeCommand, commandFound, err := a.store.CreateCommand(r.Context(), deviceID, "remote_ssh_close", mustJSON(map[string]string{
		"session_id":  session.ID,
		"remote_port": strconv.Itoa(session.RemotePort),
		"luci_port":   strconv.Itoa(session.LuCIPort),
	}))
	if err != nil || !commandFound {
		writeError(w, http.StatusInternalServerError, "failed to queue remote session close")
		return
	}
	session, _, err = a.store.CloseRemoteSession(r.Context(), deviceID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close remote session")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "operator", "remote_session.close", deviceID, closeCommand.ID, mustJSON(map[string]string{
		"session_id":       sessionID,
		"open_command_id":  session.CommandID,
		"close_command_id": closeCommand.ID,
		"request_id":       requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, session)
}

func (a *App) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := a.store.ListAuditEvents(r.Context(), store.AuditListOptions{
		DeviceID: r.URL.Query().Get("device_id"),
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audit events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"audit_events": events})
}

func (a *App) authorizeDevice(r *http.Request, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device_id is required")
	}
	token, ok := bearerToken(r)
	if !ok {
		return errors.New("missing bearer token")
	}

	allowed, err := a.store.AuthorizeDevice(r.Context(), deviceID, token)
	if err != nil {
		return errors.New("failed to authorize device")
	}
	if !allowed {
		return errors.New("invalid device credentials")
	}
	return nil
}

func (a *App) operatorAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.operatorAuthorized(r) {
			writeError(w, http.StatusUnauthorized, "invalid operator credentials")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return "", false
	}
	return token, true
}

func AllowedCommandType(t string) bool {
	switch t {
	case "ping", "traceroute", "route_show", "interfaces_show", "reboot", "service_restart", "pkg_list_installed", "pkg_update", "pkg_list_upgradable", "pkg_install", "pkg_remove", "opkg_list_installed", "opkg_update", "opkg_list_upgradable", "opkg_install", "opkg_remove", "uci_show", "uci_backup", "uci_preview", "uci_set", "uci_commit", "uci_commit_confirmed", "uci_revert", "uci_restore", "remote_ssh_reverse", "remote_ssh_close":
		return true
	default:
		return false
	}
}

func commandIDFromResultPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "agent" || parts[2] != "commands" || parts[4] != "result" {
		return "", false
	}
	return parts[3], parts[3] != ""
}

func deviceIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "devices" {
		return "", false
	}
	return parts[2], parts[2] != ""
}

func commandDeviceIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "commands" {
		return "", false
	}
	return parts[2], parts[2] != ""
}

func commandListDeviceIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "commands" {
		return "", false
	}
	return parts[2], parts[2] != ""
}

func commandDetailIDsFromPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "commands" {
		return "", "", false
	}
	return parts[2], parts[4], parts[2] != "" && parts[4] != ""
}

func commandCancelIDsFromPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "commands" || parts[5] != "cancel" {
		return "", "", false
	}
	return parts[2], parts[4], parts[2] != "" && parts[4] != ""
}

func remoteSessionListDeviceIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "remote-sessions" {
		return "", false
	}
	return parts[2], parts[2] != ""
}

func remoteSessionCloseIDsFromPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "remote-sessions" || parts[5] != "close" {
		return "", "", false
	}
	return parts[2], parts[4], parts[2] != "" && parts[4] != ""
}

func alertAcknowledgeIDsFromPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "devices" || parts[3] != "alerts" || parts[5] != "acknowledge" {
		return "", "", false
	}
	return parts[2], parts[4], parts[2] != "" && parts[4] != ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end > 0 {
			return host[1:end]
		}
	}
	if value, _, ok := strings.Cut(host, ":"); ok {
		return value
	}
	return host
}

func safeEndpointHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 255 {
		return false
	}
	clean := strings.Trim(host, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_:[]")
	return clean == ""
}

func validTCPPort(port int) bool {
	return port > 0 && port <= 65535
}

func randomRemotePort() int {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 22022
	}
	value := int(b[0])<<8 | int(b[1])
	return 22000 + (value % 100)
}

func randomLuCIPort() int {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 22122
	}
	value := int(b[0])<<8 | int(b[1])
	return 22100 + (value % 100)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json at byte %d: %v", syntaxErr.Offset, err))
			return false
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		if strings.TrimSpace(reqID) == "" {
			reqID = newRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)

		rec := &statusRecorder{ResponseWriter: w}
		ctx := context.WithValue(r.Context(), requestIDContextKey, reqID)
		next.ServeHTTP(rec, r.WithContext(ctx))
		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		logStructured(map[string]any{
			"event":       "http.request",
			"request_id":  reqID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rec.status,
			"duration_ms": time.Since(start).Milliseconds(),
			"remote":      r.RemoteAddr,
		})
	})
}

func requestID(ctx context.Context) string {
	v, ok := ctx.Value(requestIDContextKey).(string)
	if !ok {
		return ""
	}
	return v
}

func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return "req_" + hex.EncodeToString(b[:])
}

func logStructured(fields map[string]any) {
	data, err := json.Marshal(fields)
	if err != nil {
		log.Printf("log encode failed: %v", err)
		return
	}
	log.Print(string(data))
}

func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if path == "." {
			path = "index.html"
		}
		fullPath := filepath.Join(dir, path)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		indexPath := filepath.Join(dir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}
