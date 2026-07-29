package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rmm-openwrt/server/internal/authn"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

type Store interface {
	EnrollDevice(ctx context.Context, hostname, openwrtVersion string) (store.EnrolledDevice, error)
	EnrollDeviceWithGrant(ctx context.Context, tokenHash, hostname, openwrtVersion string) (store.EnrolledDevice, bool, error)
	AuthorizeDevice(ctx context.Context, deviceID, token string) (bool, error)
	EnsureBootstrapUser(ctx context.Context, username, passwordHash string) (model.User, error)
	CreateUser(ctx context.Context, username, displayName, email, passwordHash, role string) (model.User, error)
	UpdateUserSecurity(ctx context.Context, userID string, disabled *bool, passwordHash, role string) (model.User, bool, error)
	ListUsers(ctx context.Context) ([]model.User, error)
	GetUserByUsername(ctx context.Context, username string) (model.User, string, bool, error)
	GetUserByID(ctx context.Context, id string) (model.User, bool, error)
	CreateOperatorSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error
	AuthorizeOperatorSession(ctx context.Context, tokenHash string) (model.User, bool, error)
	RevokeOperatorSession(ctx context.Context, tokenHash string) error
	UpdateUserProfile(ctx context.Context, userID, displayName, email string) (model.User, bool, error)
	UpdateOwnPassword(ctx context.Context, userID, passwordHash, currentSessionHash string) error
	RevokeUserSessions(ctx context.Context, userID string) error
	GetUserForPasswordReset(ctx context.Context, identifier string) (model.User, bool, error)
	CreatePasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	ResetPassword(ctx context.Context, tokenHash, passwordHash string) (bool, error)
	CreateEnrollmentGrant(ctx context.Context, userID, dnsLabel, tokenHash string, expiresAt time.Time) (model.EnrollmentGrant, error)
	ListDevicesForUser(ctx context.Context, userID string, admin bool) ([]model.Device, error)
	DeviceAccessible(ctx context.Context, deviceID, userID string, admin bool) (bool, error)
	GetDeviceByDNSLabel(ctx context.Context, dnsLabel string) (model.Device, bool, error)
	TransferDevice(ctx context.Context, deviceID, targetUserID, requesterUserID string, admin bool) (model.Device, bool, error)
	CreateDeviceAccessGrant(ctx context.Context, tokenHash, userID, deviceID, remoteSessionID string, expiresAt time.Time) error
	ConsumeDeviceAccessGrant(ctx context.Context, grantHash, sessionHash, dnsLabel string, sessionExpiresAt time.Time) (store.AccessRoute, bool, error)
	AuthorizeDeviceAccessSession(ctx context.Context, sessionHash, dnsLabel string) (store.AccessRoute, bool, error)
	PurgeExpiredSecurityData(ctx context.Context) error
	SaveHeartbeat(ctx context.Context, deviceID string, inventory, metrics json.RawMessage) ([]model.Command, error)
	ClaimNextCommand(ctx context.Context, deviceID string) (model.Command, bool, error)
	SaveCommandResult(ctx context.Context, commandID, deviceID, status string, exitCode int, output string, result json.RawMessage) (bool, error)
	ListDevices(ctx context.Context) ([]model.Device, error)
	GetDevice(ctx context.Context, deviceID string) (model.Device, bool, error)
	UpdateDeviceFleet(ctx context.Context, deviceID, group string, tags []string) (model.Device, bool, error)
	DeleteDevice(ctx context.Context, deviceID string) (bool, error)
	ListMetricSamples(ctx context.Context, deviceID string, opts store.MetricHistoryOptions) ([]model.MetricSample, bool, error)
	SyncDeviceAlerts(ctx context.Context, deviceID string, active []model.Alert) ([]model.Alert, bool, error)
	ListAlerts(ctx context.Context, opts store.AlertListOptions) ([]model.Alert, error)
	AcknowledgeAlert(ctx context.Context, deviceID, alertID, actor string) (model.Alert, bool, error)
	PurgeAlerts(ctx context.Context, opts store.PurgeOptions) (int64, error)
	GetNotificationSettings(ctx context.Context, userID string) (model.NotificationSettings, bool, error)
	UpsertNotificationSettings(ctx context.Context, settings model.NotificationSettings) (model.NotificationSettings, error)
	CreateNotificationDelivery(ctx context.Context, delivery model.NotificationDelivery, dedupeKey string) (model.NotificationDelivery, bool, error)
	ClaimNotificationDelivery(ctx context.Context, id string, now time.Time, leaseDuration time.Duration) (model.NotificationDelivery, bool, error)
	ClaimNotificationDeliveries(ctx context.Context, now time.Time, leaseDuration time.Duration, limit int) ([]model.NotificationDelivery, error)
	CompleteNotificationDelivery(ctx context.Context, id, status, errorMessage string, nextAttemptAt *time.Time) error
	LatestNotificationDelivery(ctx context.Context, userID, alertID, channel string) (model.NotificationDelivery, bool, error)
	ListNotificationDeliveries(ctx context.Context, opts store.NotificationListOptions) ([]model.NotificationDelivery, error)
	PurgeNotificationDeliveriesBefore(ctx context.Context, cutoff time.Time) (int64, error)
	CreateCommand(ctx context.Context, deviceID, commandType string, args json.RawMessage) (model.Command, bool, error)
	ListCommands(ctx context.Context, deviceID string, opts store.CommandListOptions) ([]model.Command, bool, error)
	GetCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error)
	CancelCommand(ctx context.Context, deviceID, commandID string) (model.Command, bool, error)
	PurgeCommands(ctx context.Context, opts store.PurgeOptions) (int64, error)
	CreateRemoteSession(ctx context.Context, session model.RemoteSession) (model.RemoteSession, bool, error)
	ListRemoteSessions(ctx context.Context, deviceID string, opts store.RemoteSessionListOptions) ([]model.RemoteSession, bool, error)
	GetRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error)
	AttachRemoteSessionCommand(ctx context.Context, deviceID, sessionID, commandID string) (model.RemoteSession, bool, error)
	CloseRemoteSession(ctx context.Context, deviceID, sessionID string) (model.RemoteSession, bool, error)
	ExpireRemoteSessions(ctx context.Context) error
	AddAuditEvent(ctx context.Context, actor, action, deviceID, commandID string, details json.RawMessage) (model.AuditEvent, error)
	ListAuditEvents(ctx context.Context, opts store.AuditListOptions) ([]model.AuditEvent, error)
	PurgeAuditEvents(ctx context.Context, opts store.PurgeOptions) (int64, error)
}

type Config struct {
	EnrollmentToken            string
	AllowLegacyEnrollment      bool
	AllowLegacyLuCIProxy       bool
	OperatorToken              string
	OperatorUsername           string
	OperatorPassword           string
	SessionSecret              string
	CookieSecure               bool
	TunnelHTTPHost             string
	TunnelPublicHost           string
	TunnelPublicPort           int
	DeviceDomain               string
	PublicScheme               string
	PublicURL                  string
	PasswordResetSender        PasswordResetSender
	AlertEmailSender           NotificationSender
	TelegramSender             NotificationSender
	BackgroundTasks            bool
	NotificationMaxAttempts    int
	NotificationWorkerInterval time.Duration
	StaticDir                  string
}

type App struct {
	store                      Store
	enrollmentToken            string
	allowLegacyEnrollment      bool
	operatorToken              string
	operatorUsername           string
	dummyPasswordHash          string
	cookieSecure               bool
	tunnelHTTPHost             string
	tunnelPublicHost           string
	tunnelPublicPort           int
	deviceDomain               string
	publicScheme               string
	publicURL                  string
	passwordResetSender        PasswordResetSender
	alertEmailSender           NotificationSender
	telegramSender             NotificationSender
	notificationMaxAttempts    int
	notificationWorkerInterval time.Duration
	loginLimiter               *loginRateLimiter
	passwordResetLimiter       *loginRateLimiter
	loginSlots                 chan struct{}
	events                     *eventHub
}

type contextKey string

const requestIDContextKey contextKey = "request_id"
const luciRouteCookie = "rmm_luci_route"
const deviceAccessCookie = "rmm_device_access"

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

type remoteSessionCreateError struct {
	Status  int
	Message string
}

func (e *remoteSessionCreateError) Error() string { return e.Message }

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

type createUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type updateUserRequest struct {
	Disabled *bool  `json:"disabled"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordResetRequest struct {
	Identifier string `json:"identifier"`
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type deviceTransferRequest struct {
	TargetUsername  string `json:"target_username"`
	CurrentPassword string `json:"current_password"`
}

type enrollmentGrantRequest struct {
	DNSLabel       string `json:"dns_label"`
	ExpiresSeconds int    `json:"expires_seconds"`
}

func NewHandler(s Store, cfg Config) http.Handler {
	if strings.TrimSpace(cfg.OperatorUsername) == "" {
		cfg.OperatorUsername = "admin"
	}
	if cfg.OperatorPassword == "" {
		cfg.OperatorPassword = cfg.OperatorToken
	}
	if cfg.PasswordResetSender != nil {
		publicURL, err := url.Parse(strings.TrimSpace(cfg.PublicURL))
		if err != nil || publicURL.Host == "" || (publicURL.Scheme != "https" && publicURL.Scheme != "http") || publicURL.RawQuery != "" || publicURL.Fragment != "" {
			panic("password recovery requires an absolute RMM public URL without query or fragment")
		}
	}
	passwordHash, err := authn.HashPassword(cfg.OperatorPassword)
	if err != nil {
		panic("invalid bootstrap operator password: " + err.Error())
	}
	if _, err := s.EnsureBootstrapUser(context.Background(), cfg.OperatorUsername, passwordHash); err != nil {
		panic("initialize bootstrap operator: " + err.Error())
	}
	dummyPasswordHash, err := authn.HashPassword("this-password-is-never-valid")
	if err != nil {
		panic("initialize password verifier: " + err.Error())
	}
	a := &App{
		store:                      s,
		enrollmentToken:            cfg.EnrollmentToken,
		allowLegacyEnrollment:      cfg.AllowLegacyEnrollment || strings.TrimSpace(cfg.EnrollmentToken) != "",
		operatorToken:              cfg.OperatorToken,
		operatorUsername:           cfg.OperatorUsername,
		dummyPasswordHash:          dummyPasswordHash,
		cookieSecure:               cfg.CookieSecure,
		tunnelHTTPHost:             strings.TrimSpace(cfg.TunnelHTTPHost),
		tunnelPublicHost:           strings.TrimSpace(cfg.TunnelPublicHost),
		tunnelPublicPort:           cfg.TunnelPublicPort,
		deviceDomain:               strings.Trim(strings.ToLower(strings.TrimSpace(cfg.DeviceDomain)), "."),
		publicScheme:               strings.ToLower(strings.TrimSpace(cfg.PublicScheme)),
		publicURL:                  strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/"),
		passwordResetSender:        cfg.PasswordResetSender,
		alertEmailSender:           cfg.AlertEmailSender,
		telegramSender:             cfg.TelegramSender,
		notificationMaxAttempts:    cfg.NotificationMaxAttempts,
		notificationWorkerInterval: cfg.NotificationWorkerInterval,
		loginLimiter:               newLoginRateLimiter(5, 5*time.Minute),
		passwordResetLimiter:       newLoginRateLimiter(3, time.Hour),
		loginSlots:                 make(chan struct{}, 4),
		events:                     newEventHub(),
	}
	if a.notificationMaxAttempts <= 0 {
		a.notificationMaxAttempts = 5
	}
	if a.notificationWorkerInterval <= 0 {
		a.notificationWorkerInterval = 30 * time.Second
	}
	if a.tunnelHTTPHost == "" {
		a.tunnelHTTPHost = "tunnel-ssh"
	}
	if a.tunnelPublicPort <= 0 {
		a.tunnelPublicPort = 2222
	}
	if a.tunnelPublicHost == "" && a.publicURL != "" {
		if parsed, err := url.Parse(a.publicURL); err == nil {
			a.tunnelPublicHost = parsed.Hostname()
		}
	}
	if a.publicScheme != "http" && a.publicScheme != "https" {
		if a.cookieSecure {
			a.publicScheme = "https"
		} else {
			a.publicScheme = "http"
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/password-reset/request", a.handlePasswordResetRequest)
	mux.HandleFunc("POST /api/auth/password-reset/confirm", a.handlePasswordResetConfirm)
	mux.Handle("POST /api/auth/logout", a.operatorAuth(http.HandlerFunc(a.handleLogout)))
	mux.Handle("GET /api/auth/me", a.operatorAuth(http.HandlerFunc(a.handleAuthMe)))
	mux.Handle("PATCH /api/auth/profile", a.operatorAuth(http.HandlerFunc(a.handleUpdateProfile)))
	mux.Handle("POST /api/auth/change-password", a.operatorAuth(http.HandlerFunc(a.handleChangePassword)))
	mux.Handle("POST /api/auth/logout-all", a.operatorAuth(http.HandlerFunc(a.handleLogoutAll)))
	mux.Handle("GET /api/notifications/settings", a.operatorAuth(http.HandlerFunc(a.handleGetNotificationSettings)))
	mux.Handle("PUT /api/notifications/settings", a.operatorAuth(http.HandlerFunc(a.handleUpdateNotificationSettings)))
	mux.Handle("GET /api/notifications", a.operatorAuth(http.HandlerFunc(a.handleListNotifications)))
	mux.Handle("POST /api/notifications/test", a.operatorAuth(http.HandlerFunc(a.handleTestNotifications)))
	mux.Handle("GET /api/users", a.operatorAuth(a.adminOnly(http.HandlerFunc(a.handleListUsers))))
	mux.Handle("POST /api/users", a.operatorAuth(a.adminOnly(http.HandlerFunc(a.handleCreateUser))))
	mux.Handle("PATCH /api/users/", a.operatorAuth(a.adminOnly(http.HandlerFunc(a.handleUpdateUser))))
	mux.Handle("POST /api/enrollment-grants", a.operatorAuth(http.HandlerFunc(a.handleCreateEnrollmentGrant)))
	mux.HandleFunc("POST /api/agent/enroll", a.handleEnroll)
	mux.HandleFunc("POST /api/agent/heartbeat", a.handleHeartbeat)
	mux.HandleFunc("POST /api/agent/commands/next", a.handleNextCommand)
	mux.HandleFunc("POST /api/agent/commands/", a.handleCommandResult)
	mux.Handle("GET /api/devices", a.operatorAuth(http.HandlerFunc(a.handleListDevices)))
	mux.Handle("GET /api/events", a.operatorAuth(http.HandlerFunc(a.handleEvents)))
	mux.Handle("POST /api/devices/bulk-commands", a.operatorAuth(http.HandlerFunc(a.handleCreateBulkCommand)))
	mux.Handle("GET /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("POST /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("PATCH /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("DELETE /api/devices/", a.operatorAuth(http.HandlerFunc(a.handleDeviceSubtree)))
	mux.Handle("GET /api/audit-events", a.operatorAuth(http.HandlerFunc(a.handleListAuditEvents)))
	mux.Handle("DELETE /api/audit-events", a.operatorAuth(a.adminOnly(http.HandlerFunc(a.handlePurgeAuditEvents))))
	if cfg.AllowLegacyLuCIProxy {
		mux.Handle("GET /luci/", a.operatorAuth(http.HandlerFunc(a.handleLuCIProxy)))
		mux.Handle("POST /luci/", a.operatorAuth(http.HandlerFunc(a.handleLuCIProxy)))
		for _, path := range []string{"/cgi-bin/", "/luci-static/", "/ubus/", "/ubus"} {
			mux.Handle("GET "+path, a.operatorAuth(http.HandlerFunc(a.handleLuCIFallback)))
			mux.Handle("POST "+path, a.operatorAuth(http.HandlerFunc(a.handleLuCIFallback)))
		}
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if cfg.StaticDir != "" {
		mux.Handle("GET /", staticHandler(cfg.StaticDir))
	}
	if cfg.BackgroundTasks {
		go a.alertNotificationLoop()
	}

	return withRequestLogging(a.routeByHost(mux))
}

func (a *App) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.EnrollmentToken) > 512 || len(req.Hostname) > 255 || len(req.OpenWrtVersion) > 512 {
		writeError(w, http.StatusBadRequest, "enrollment fields exceed allowed length")
		return
	}
	enrolled, ok, err := a.store.EnrollDeviceWithGrant(r.Context(), store.TokenHash(req.EnrollmentToken), req.Hostname, req.OpenWrtVersion)
	if err == nil && !ok && a.allowLegacyEnrollment && a.enrollmentToken != "" && constantTimeEqual(req.EnrollmentToken, a.enrollmentToken) {
		enrolled, err = a.store.EnrollDevice(r.Context(), req.Hostname, req.OpenWrtVersion)
		ok = err == nil
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enroll device")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid or expired enrollment grant")
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
	} else if deliveries, queueErr := a.queueDeviceNotifications(r.Context(), req.DeviceID); queueErr != nil {
		log.Printf("queue notifications for %s: %v", req.DeviceID, queueErr)
	} else if len(deliveries) > 0 {
		go func() {
			if err := a.processNotificationQueue(context.Background()); err != nil {
				log.Printf("process notification queue: %v", err)
			}
		}()
	}
	a.events.publish("devices")

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
	a.events.publish("devices")

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleListDevices(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	devices, err := a.store.ListDevicesForUser(r.Context(), principal.User.ID, principal.IsAdmin())
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
	devices, err = a.store.ListDevicesForUser(r.Context(), principal.User.ID, principal.IsAdmin())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load devices")
		return
	}

	a.decorateDevices(devices)
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
	a.decorateDevice(&d)
	writeJSON(w, http.StatusOK, d)
}

func (a *App) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := deviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deleted, err := a.store.DeleteDevice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete device")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "device.delete", id, "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (a *App) handleDeviceSubtree(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid operator credentials")
		return
	}
	accessible, err := a.store.DeviceAccessible(r.Context(), parts[2], principal.User.ID, principal.IsAdmin())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authorize device")
		return
	}
	if !accessible {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodGet {
		a.handleGetDevice(w, r)
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		a.handleDeleteDevice(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "fleet" && r.Method == http.MethodPatch {
		a.handleUpdateDeviceFleet(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "transfer" && r.Method == http.MethodPost {
		a.handleTransferDevice(w, r)
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
	if len(parts) == 4 && parts[3] == "alerts" && r.Method == http.MethodDelete {
		a.handlePurgeDeviceAlerts(w, r)
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
	if len(parts) == 4 && parts[3] == "commands" && r.Method == http.MethodDelete {
		a.handlePurgeDeviceCommands(w, r)
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
	if len(parts) == 4 && parts[3] == "cloud-access" && r.Method == http.MethodGet {
		a.handleCloudAccessStatus(w, r)
		return
	}
	if len(parts) == 4 && parts[3] == "cloud-access" && r.Method == http.MethodPost {
		a.handleOpenCloudAccess(w, r)
		return
	}
	if len(parts) == 6 && parts[3] == "remote-sessions" && parts[5] == "close" && r.Method == http.MethodPost {
		a.handleCloseRemoteSession(w, r)
		return
	}
	if len(parts) == 6 && parts[3] == "remote-sessions" && parts[5] == "access" && r.Method == http.MethodPost {
		a.handleCreateDeviceAccess(w, r)
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
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "device.fleet_update", deviceID, "", mustJSON(map[string]any{
		"group":      req.Group,
		"tags":       req.Tags,
		"request_id": requestID(r.Context()),
	}))
	a.decorateDevice(&d)
	a.events.publish("devices")
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
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" || status == "open" {
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
		return
	}
	_, found, err := a.store.GetDevice(r.Context(), parts[2])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	list, err := a.store.ListAlerts(r.Context(), store.AlertListOptions{DeviceID: parts[2], Status: status, Limit: 200})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load alerts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": list})
}

func (a *App) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	deviceID, alertID, ok := alertAcknowledgeIDsFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	alert, changed, err := a.store.AcknowledgeAlert(r.Context(), deviceID, alertID, actorName(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to acknowledge alert")
		return
	}
	if !changed {
		writeError(w, http.StatusNotFound, "active alert not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "alert.acknowledge", deviceID, "", mustJSON(map[string]string{
		"alert_id":   alertID,
		"type":       alert.Type,
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, alert)
}

func (a *App) handlePurgeDeviceAlerts(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "alerts" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deviceID := parts[2]
	if _, found, err := a.store.GetDevice(r.Context(), deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device")
		return
	} else if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	deleted, err := a.store.PurgeAlerts(r.Context(), store.PurgeOptions{DeviceID: deviceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to purge alerts")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "alerts.purge", deviceID, "", mustJSON(map[string]any{
		"deleted":    deleted,
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
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
	settings := store.DefaultNotificationSettings(device.OwnerUserID)
	if device.OwnerUserID != "" {
		if configured, _, settingsErr := a.store.GetNotificationSettings(ctx, device.OwnerUserID); settingsErr != nil {
			return nil, true, settingsErr
		} else {
			settings = configured
		}
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
	if ratio, ok := jsonRatio(device.Metrics, "memory", "used_kb", "total_kb"); ok && ratio >= float64(settings.MemoryThresholdPercent)/100 {
		alerts = append(alerts, newUsageAlert(deviceID, "memory_high", "Memory usage is high", ratio, settings.MemoryThresholdPercent, now))
	}
	if ratio, ok := diskUsageRatio(device.Metrics); ok && ratio >= float64(settings.DiskThresholdPercent)/100 {
		alerts = append(alerts, newUsageAlert(deviceID, "disk_high", "Disk usage is high", ratio, settings.DiskThresholdPercent, now))
	}
	alerts = append(alerts, connectivityAlerts(deviceID, device.Metrics, settings, now)...)
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

func connectivityAlerts(deviceID string, raw json.RawMessage, settings model.NotificationSettings, now time.Time) []model.Alert {
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
	if worstLoss >= float64(settings.PacketLossPercent) {
		severity := "warning"
		if worstLoss >= float64(min(100, settings.PacketLossPercent*3)) {
			severity = "critical"
		}
		alerts = append(alerts, newAlert(deviceID, "packet_loss_high", severity, "Packet loss is high", map[string]any{
			"target":               worstLossTarget,
			"packet_loss_percent":  int(worstLoss),
			"reachable_target_cnt": reachableCount,
		}, now))
	}
	if worstLatency >= float64(settings.LatencyThresholdMS) {
		severity := "warning"
		if worstLatency >= float64(settings.LatencyThresholdMS*2) {
			severity = "critical"
		}
		alerts = append(alerts, newAlert(deviceID, "latency_high", severity, "Latency is high", map[string]any{
			"target":     worstLatencyTarget,
			"latency_ms": int(worstLatency),
		}, now))
	}
	return alerts
}

func newUsageAlert(deviceID, alertType, message string, ratio float64, warningThreshold int, now time.Time) model.Alert {
	severity := "warning"
	criticalThreshold := min(99, warningThreshold+10)
	if ratio >= float64(criticalThreshold)/100 {
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

	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "command.create", deviceID, c.ID, mustJSON(map[string]string{
		"type":       req.Type,
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
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
	principal, _ := principalFromContext(r.Context())
	for _, deviceID := range uniqueStrings(req.DeviceIDs) {
		accessible, err := a.store.DeviceAccessible(r.Context(), deviceID, principal.User.ID, principal.IsAdmin())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to authorize device")
			return
		}
		if !accessible {
			writeError(w, http.StatusNotFound, "device not found: "+deviceID)
			return
		}
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
		_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "command.bulk_create", deviceID, c.ID, mustJSON(map[string]string{
			"type":       req.Type,
			"request_id": requestID(r.Context()),
		}))
	}
	a.events.publish("devices")
	writeJSON(w, http.StatusCreated, map[string]any{"commands": commands})
}

func (a *App) handleListCommands(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := commandListDeviceIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	commands, found, err := a.store.ListCommands(r.Context(), deviceID, store.CommandListOptions{Limit: limit, Offset: offset})
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

	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "command.cancel", deviceID, commandID, mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, c)
}

func (a *App) handlePurgeDeviceCommands(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "commands" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deviceID := parts[2]
	if _, found, err := a.store.GetDevice(r.Context(), deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device")
		return
	} else if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	deleted, err := a.store.PurgeCommands(r.Context(), store.PurgeOptions{DeviceID: deviceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to purge command history")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "commands.purge", deviceID, "", mustJSON(map[string]any{
		"deleted":    deleted,
		"request_id": requestID(r.Context()),
	}))
	a.events.publish("devices")
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
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
	for i := range sessions {
		sessions[i].AccessState = a.remoteSessionAccessState(sessions[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"remote_sessions": sessions})
}

func (a *App) handleCloudAccessStatus(w http.ResponseWriter, r *http.Request) {
	a.writeCloudAccessState(w, r, false)
}

func (a *App) handleOpenCloudAccess(w http.ResponseWriter, r *http.Request) {
	a.writeCloudAccessState(w, r, true)
}

func (a *App) writeCloudAccessState(w http.ResponseWriter, r *http.Request, create bool) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	deviceID := parts[2]
	if a.deviceDomain == "" || a.tunnelPublicHost == "" {
		writeCloudError(w, http.StatusConflict, "not_configured", "cloud access is not configured")
		return
	}
	device, found, err := a.store.GetDevice(r.Context(), deviceID)
	if err != nil {
		writeCloudError(w, http.StatusInternalServerError, "server_error", "failed to load device")
		return
	}
	if !found || device.DNSLabel == "" {
		writeCloudError(w, http.StatusNotFound, "device_unavailable", "device DNS name is unavailable")
		return
	}
	if !device.Online {
		writeCloudError(w, http.StatusServiceUnavailable, "device_offline", "router is offline")
		return
	}
	sessions, _, err := a.store.ListRemoteSessions(r.Context(), deviceID, store.RemoteSessionListOptions{Limit: 10})
	if err != nil {
		writeCloudError(w, http.StatusInternalServerError, "server_error", "failed to load remote sessions")
		return
	}
	now := time.Now().UTC()
	for _, session := range sessions {
		if !session.ExpiresAt.After(now) || !containsString([]string{"requested", "queued", "active"}, session.Status) {
			continue
		}
		state := a.remoteSessionAccessState(session)
		if state == "ready" {
			if !create {
				writeJSON(w, http.StatusOK, map[string]any{"status": state, "session": session})
				return
			}
			principal, _ := principalFromContext(r.Context())
			accessURL, expiresAt, grantErr := a.createDeviceAccessGrant(r.Context(), principal.User.ID, principal.User.Username, device, session)
			if grantErr != nil {
				if strings.Contains(strings.ToLower(grantErr.Error()), "limit") {
					writeCloudError(w, http.StatusTooManyRequests, "grant_limit", "active LuCI access grant limit reached")
					return
				}
				writeCloudError(w, http.StatusInternalServerError, "server_error", "failed to create LuCI access grant")
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"status": "ready", "session": session, "url": accessURL, "expires_at": expiresAt})
			return
		}
		status := http.StatusAccepted
		if state == "unavailable" {
			if create {
				if _, closeErr := a.closeRemoteSession(r.Context(), actorName(r), deviceID, session); closeErr != nil {
					writeCloudError(w, http.StatusInternalServerError, "restart_failed", "failed to restart cloud tunnel")
					return
				}
				break
			}
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]any{"status": state, "session": session, "retry_after_seconds": 2})
		return
	}
	if !create {
		writeJSON(w, http.StatusOK, map[string]any{"status": "closed"})
		return
	}
	session, err := a.createRemoteSession(r.Context(), actorName(r), deviceID, remoteSessionRequest{
		Target:          "ssh",
		DurationSeconds: 15 * 60,
		ServerHost:      a.tunnelPublicHost,
		ServerPort:      a.tunnelPublicPort,
		LocalPort:       22,
		LuCIScheme:      "http",
	}, a.tunnelPublicHost)
	if err != nil {
		var createErr *remoteSessionCreateError
		if errors.As(err, &createErr) {
			writeCloudError(w, createErr.Status, "create_failed", createErr.Message)
		} else {
			writeCloudError(w, http.StatusInternalServerError, "create_failed", "failed to create cloud tunnel")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "starting", "session": session, "retry_after_seconds": 2})
}

func (a *App) remoteSessionAccessState(session model.RemoteSession) string {
	if !session.ExpiresAt.After(time.Now().UTC()) || containsString([]string{"closed", "expired", "failed"}, session.Status) {
		return "closed"
	}
	if session.Status != "active" || session.LuCIPort <= 0 {
		return "starting"
	}
	if a.luciTunnelReachable(session) {
		return "ready"
	}
	if session.StartedAt != nil && time.Since(*session.StartedAt) > 20*time.Second {
		return "unavailable"
	}
	return "starting"
}

func (a *App) luciTunnelReachable(session model.RemoteSession) bool {
	scheme := session.LuCIScheme
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	upstream := scheme + "://" + net.JoinHostPort(a.tunnelHTTPHost, strconv.Itoa(session.LuCIPort)) + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream, nil)
	if err != nil {
		return false
	}
	req.Host = "127.0.0.1"

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.DialContext = (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext
	transport.ResponseHeaderTimeout = 1500 * time.Millisecond
	if scheme == "https" {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // LuCI commonly uses a router-local self-signed certificate.
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

func writeCloudError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
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
	session, err := a.createRemoteSession(r.Context(), actorName(r), deviceID, req, hostWithoutPort(r.Host))
	if err != nil {
		var createErr *remoteSessionCreateError
		if errors.As(err, &createErr) {
			writeError(w, createErr.Status, createErr.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to create remote session")
		}
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (a *App) createRemoteSession(ctx context.Context, actor, deviceID string, req remoteSessionRequest, fallbackHost string) (model.RemoteSession, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "ssh"
	}
	if target != "ssh" {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusBadRequest, "only ssh remote sessions are supported"}
	}
	duration := req.DurationSeconds
	if duration <= 0 {
		duration = 15 * 60
	}
	if duration < 60 || duration > 2*60*60 {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusBadRequest, "duration_seconds must be between 60 and 7200"}
	}
	serverHost := strings.TrimSpace(req.ServerHost)
	if serverHost == "" {
		serverHost = fallbackHost
	}
	if serverHost == "" || !safeEndpointHost(serverHost) {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusBadRequest, "server_host is invalid"}
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
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusBadRequest, "luci_scheme must be http or https"}
	}
	if !validTCPPort(serverPort) || !validTCPPort(localPort) || !validTCPPort(remotePort) || !validTCPPort(luciPort) {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusBadRequest, "ports must be between 1 and 65535"}
	}

	expiresAt := time.Now().UTC().Add(time.Duration(duration) * time.Second)
	session, found, err := a.store.CreateRemoteSession(ctx, model.RemoteSession{
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
		return model.RemoteSession{}, err
	}
	if !found {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusNotFound, "device not found"}
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
	command, commandFound, err := a.store.CreateCommand(ctx, deviceID, "remote_ssh_reverse", args)
	if err != nil {
		return model.RemoteSession{}, err
	}
	if !commandFound {
		return model.RemoteSession{}, &remoteSessionCreateError{http.StatusNotFound, "device not found"}
	}
	session, _, err = a.store.AttachRemoteSessionCommand(ctx, deviceID, session.ID, command.ID)
	if err != nil {
		return model.RemoteSession{}, err
	}

	_, _ = a.store.AddAuditEvent(ctx, actor, "remote_session.create", deviceID, command.ID, mustJSON(map[string]any{
		"session_id":  session.ID,
		"target":      session.Target,
		"server_host": session.ServerHost,
		"server_port": session.ServerPort,
		"remote_port": session.RemotePort,
		"luci_port":   session.LuCIPort,
		"luci_scheme": session.LuCIScheme,
		"local_port":  session.LocalPort,
		"expires_at":  session.ExpiresAt,
		"request_id":  requestID(ctx),
	}))
	a.events.publish("devices")
	return session, nil
}

func (a *App) handleLuCIProxy(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/luci/")
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "LuCI session not found")
		return
	}
	deviceID, sessionID := parts[0], parts[1]
	if !a.authorizeOperatorDevice(w, r, deviceID) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     luciRouteCookie,
		Value:    deviceID + "|" + sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   2 * 60 * 60,
	})
	upstreamPath := "/"
	if len(parts) == 3 && parts[2] != "" {
		upstreamPath += parts[2]
	}
	a.proxyLuCI(w, r, deviceID, sessionID, upstreamPath)
}

func (a *App) handleLuCIFallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(luciRouteCookie)
	if err != nil {
		writeError(w, http.StatusNotFound, "no active LuCI route selected")
		return
	}
	deviceID, sessionID, ok := strings.Cut(cookie.Value, "|")
	if !ok || deviceID == "" || sessionID == "" {
		writeError(w, http.StatusNotFound, "invalid LuCI route")
		return
	}
	if !a.authorizeOperatorDevice(w, r, deviceID) {
		return
	}
	a.proxyLuCI(w, r, deviceID, sessionID, r.URL.Path)
}

func (a *App) authorizeOperatorDevice(w http.ResponseWriter, r *http.Request, deviceID string) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid operator credentials")
		return false
	}
	accessible, err := a.store.DeviceAccessible(r.Context(), deviceID, principal.User.ID, principal.IsAdmin())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authorize device")
		return false
	}
	if !accessible {
		writeError(w, http.StatusNotFound, "device not found")
		return false
	}
	return true
}

func (a *App) proxyLuCI(w http.ResponseWriter, r *http.Request, deviceID, sessionID, upstreamPath string) {
	a.proxyLuCIWithPrefix(w, r, deviceID, sessionID, upstreamPath, "/luci/"+deviceID+"/"+sessionID)
}

func (a *App) proxyLuCIWithPrefix(w http.ResponseWriter, r *http.Request, deviceID, sessionID, upstreamPath, prefix string) {
	session, found, err := a.store.GetRemoteSession(r.Context(), deviceID, sessionID)
	if err != nil {
		a.writeLuCIError(w, r, http.StatusInternalServerError, "failed to load LuCI session")
		return
	}
	if !found || session.Status != "active" || session.LuCIPort <= 0 {
		a.writeLuCIError(w, r, http.StatusNotFound, "LuCI session is not active")
		return
	}

	scheme := session.LuCIScheme
	if scheme == "" {
		scheme = "http"
	}
	upstream, _ := url.Parse(scheme + "://" + a.tunnelHTTPHost + ":" + strconv.Itoa(session.LuCIPort))
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 15 * time.Second
	if scheme == "https" {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // LuCI commonly uses a router-local self-signed certificate.
	}
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = upstreamPath
		// uhttpd's RFC1918/DNS-rebinding protection rejects LuCI CGI requests
		// whose Host points at the Docker tunnel service instead of the router.
		req.Host = "127.0.0.1"
		req.Header.Set("Host", "127.0.0.1")
		req.Header.Del("Accept-Encoding")
		req.Header.Del("Forwarded")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Port")
		req.Header.Del("X-Forwarded-Proto")
		req.Header.Del("X-Real-IP")
		req.Header.Del("Origin")
		req.Header.Del("Referer")
		// A nil slice tells ReverseProxy not to append the operator address.
		req.Header["X-Forwarded-For"] = nil
		removeCookie(req, operatorSessionCookie)
		removeCookie(req, luciRouteCookie)
		removeCookie(req, deviceAccessCookie)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= 400 && resp.Header.Get("X-LuCI-Login-Required") == "" {
			logStructured(map[string]any{
				"event":        "luci.upstream_error",
				"request_id":   requestID(r.Context()),
				"device_id":    deviceID,
				"session_id":   sessionID,
				"path":         upstreamPath,
				"status":       resp.StatusCode,
				"content_type": resp.Header.Get("Content-Type"),
			})
		}
		rewriteLuCIHeaders(resp.Header, prefix, a.cookieSecure)
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, proxyRequest *http.Request, err error) {
		logStructured(map[string]any{
			"event":      "luci.proxy_failed",
			"request_id": requestID(r.Context()),
			"device_id":  deviceID,
			"session_id": sessionID,
			"error":      err.Error(),
		})
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			status = http.StatusGatewayTimeout
		}
		a.writeLuCIError(w, proxyRequest, status, "LuCI is unreachable through this session")
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

func rewriteLuCIHeaders(header http.Header, prefix string, secure bool) {
	if location := header.Get("Location"); strings.HasPrefix(location, "/") {
		header.Set("Location", prefix+location)
	}
	cookies := make([]string, 0, len(header.Values("Set-Cookie")))
	for _, cookie := range header.Values("Set-Cookie") {
		if rewritten, ok := rewriteLuCICookie(cookie, secure); ok {
			cookies = append(cookies, rewritten)
		}
	}
	header.Del("Set-Cookie")
	for _, cookie := range cookies {
		header.Add("Set-Cookie", cookie)
	}
}

func rewriteLuCICookie(cookie string, secure bool) (string, bool) {
	parts := strings.Split(cookie, ";")
	name, _, ok := strings.Cut(strings.TrimSpace(parts[0]), "=")
	if !ok {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case operatorSessionCookie, deviceAccessCookie, luciRouteCookie:
		return "", false
	}
	pathFound := false
	httpOnlyFound := false
	sameSiteFound := false
	secureFound := false
	filtered := []string{parts[0]}
	for index := 1; index < len(parts); index++ {
		attribute := strings.TrimSpace(parts[index])
		lower := strings.ToLower(attribute)
		if strings.HasPrefix(lower, "domain=") {
			continue
		}
		if strings.HasPrefix(lower, "path=") {
			filtered = append(filtered, " Path=/")
			pathFound = true
			continue
		}
		if strings.HasPrefix(lower, "samesite=") {
			filtered = append(filtered, " SameSite=Strict")
			sameSiteFound = true
			continue
		}
		httpOnlyFound = httpOnlyFound || lower == "httponly"
		secureFound = secureFound || lower == "secure"
		filtered = append(filtered, " "+attribute)
	}
	if !pathFound {
		filtered = append(filtered, " Path=/")
	}
	if !httpOnlyFound {
		filtered = append(filtered, " HttpOnly")
	}
	if !sameSiteFound {
		filtered = append(filtered, " SameSite=Strict")
	}
	if secure && !secureFound {
		filtered = append(filtered, " Secure")
	}
	return strings.Join(filtered, ";"), true
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
	session, err = a.closeRemoteSession(r.Context(), actorName(r), deviceID, session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close remote session")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *App) closeRemoteSession(ctx context.Context, actor, deviceID string, session model.RemoteSession) (model.RemoteSession, error) {
	closeCommand, commandFound, err := a.store.CreateCommand(ctx, deviceID, "remote_ssh_close", mustJSON(map[string]string{
		"session_id":  session.ID,
		"remote_port": strconv.Itoa(session.RemotePort),
		"luci_port":   strconv.Itoa(session.LuCIPort),
	}))
	if err != nil || !commandFound {
		if err != nil {
			return model.RemoteSession{}, err
		}
		return model.RemoteSession{}, errors.New("device not found while queueing remote session close")
	}
	session, _, err = a.store.CloseRemoteSession(ctx, deviceID, session.ID)
	if err != nil {
		return model.RemoteSession{}, err
	}
	_, _ = a.store.AddAuditEvent(ctx, actor, "remote_session.close", deviceID, closeCommand.ID, mustJSON(map[string]string{
		"session_id":       session.ID,
		"open_command_id":  session.CommandID,
		"close_command_id": closeCommand.ID,
		"request_id":       requestID(ctx),
	}))
	a.events.publish("devices")
	return session, nil
}

func (a *App) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	principal, _ := principalFromContext(r.Context())
	if !principal.IsAdmin() {
		if deviceID == "" {
			writeError(w, http.StatusForbidden, "device_id is required for this account")
			return
		}
		accessible, err := a.store.DeviceAccessible(r.Context(), deviceID, principal.User.ID, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to authorize device")
			return
		}
		if !accessible {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
	}
	events, err := a.store.ListAuditEvents(r.Context(), store.AuditListOptions{
		DeviceID: deviceID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audit events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"audit_events": events})
}

func (a *App) handlePurgeAuditEvents(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	deleted, err := a.store.PurgeAuditEvents(r.Context(), store.PurgeOptions{DeviceID: deviceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to purge audit events")
		return
	}
	action := "audit.purge"
	if strings.TrimSpace(deviceID) != "" {
		action = "device_audit.purge"
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), action, deviceID, "", mustJSON(map[string]any{
		"deleted":    deleted,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
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
		principal, ok := a.authenticateOperator(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid operator credentials")
			return
		}
		if principal.Method == "cookie" && unsafeMethod(r.Method) && !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey, principal)))
	})
}

func actorName(r *http.Request) string {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.User.Username == "" {
		return "operator"
	}
	return principal.User.Username
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
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
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request body must contain a single JSON value")
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

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
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
		requestPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		assetPath := strings.TrimPrefix(requestPath, "/")
		switch requestPath {
		case "/":
			assetPath = "landing.html"
		case "/login", "/app":
			assetPath = "index.html"
		}
		fullPath := filepath.Join(dir, filepath.FromSlash(assetPath))
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			if strings.HasSuffix(assetPath, ".html") {
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeFile(w, r, fullPath)
			} else {
				fs.ServeHTTP(w, r)
			}
			return
		}
		writeError(w, http.StatusNotFound, "not found")
	})
}
