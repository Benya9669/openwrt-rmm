package httpapi

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

type NotificationSender interface {
	SendNotification(ctx context.Context, destination, title, body string) error
}

const notificationDeliveryLease = 5 * time.Minute

type notificationSettingsRequest struct {
	EmailEnabled           bool   `json:"email_enabled"`
	TelegramEnabled        bool   `json:"telegram_enabled"`
	TelegramChatID         string `json:"telegram_chat_id"`
	NotifyWarning          bool   `json:"notify_warning"`
	NotifyCritical         bool   `json:"notify_critical"`
	NotifyResolved         bool   `json:"notify_resolved"`
	MemoryThresholdPercent int    `json:"memory_threshold_percent"`
	DiskThresholdPercent   int    `json:"disk_threshold_percent"`
	PacketLossPercent      int    `json:"packet_loss_percent"`
	LatencyThresholdMS     int    `json:"latency_threshold_ms"`
	RepeatMinutes          int    `json:"repeat_minutes"`
	Timezone               string `json:"timezone"`
	QuietHoursEnabled      bool   `json:"quiet_hours_enabled"`
	QuietHoursStart        string `json:"quiet_hours_start"`
	QuietHoursEnd          string `json:"quiet_hours_end"`
	AlertsPausedUntil      string `json:"alerts_paused_until"`
	WebhookEnabled         bool   `json:"webhook_enabled"`
	WebhookURL             string `json:"webhook_url"`
	WebhookSecret          string `json:"webhook_secret"`
}

type deviceNotificationSettingsRequest struct {
	Enabled        bool   `json:"enabled"`
	NotifyWarning  bool   `json:"notify_warning"`
	NotifyCritical bool   `json:"notify_critical"`
	NotifyResolved bool   `json:"notify_resolved"`
	PausedUntil    string `json:"paused_until"`
}

type contactVerificationRequest struct {
	Destination string `json:"destination"`
	Code        string `json:"code"`
}

func (a *App) handleRequestContactVerification(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	channel := parts[3]
	if channel != "email" && channel != "telegram" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req contactVerificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	principal, _ := principalFromContext(r.Context())
	destination := strings.TrimSpace(req.Destination)
	var sender NotificationSender
	if channel == "email" {
		destination = principal.User.Email
		sender = a.alertEmailSender
		if destination == "" || sender == nil {
			writeError(w, http.StatusConflict, "email verification is unavailable")
			return
		}
	} else {
		sender = a.telegramSender
		if sender == nil || !validTelegramChatID(destination) {
			writeError(w, http.StatusBadRequest, "Telegram chat ID is invalid or unavailable")
			return
		}
	}
	randomValue, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create verification")
		return
	}
	code := fmt.Sprintf("%06d", randomValue.Int64())
	codeHash, err := store.VerificationCodeHash(code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure verification")
		return
	}
	if err := a.store.BeginContactVerification(r.Context(), principal.User.ID, channel, destination, codeHash, time.Now().UTC().Add(10*time.Minute)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save verification")
		return
	}
	if err := sender.SendNotification(r.Context(), destination, "OpenWrt RMM verification", "Verification code: "+code+"\n\nThe code expires in 10 minutes."); err != nil {
		writeError(w, http.StatusBadGateway, "failed to deliver verification code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true, "destination": maskVerificationDestination(channel, destination)})
}

func (a *App) handleConfirmContactVerification(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	channel := parts[3]
	if channel != "email" && channel != "telegram" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req contactVerificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(strings.TrimSpace(req.Code)) != 6 {
		writeError(w, http.StatusBadRequest, "verification code is invalid")
		return
	}
	principal, _ := principalFromContext(r.Context())
	destination, confirmed, err := a.store.ConfirmContactVerification(r.Context(), principal.User.ID, channel, req.Code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to confirm verification")
		return
	}
	if !confirmed {
		writeError(w, http.StatusBadRequest, "verification code is invalid or expired")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "destination": maskVerificationDestination(channel, destination)})
}

func maskVerificationDestination(channel, destination string) string {
	if channel == "email" {
		return maskEmail(destination)
	}
	return maskTelegramChatID(destination)
}

func (a *App) handleListInboxNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, unread, err := a.store.ListInboxNotifications(r.Context(), principal.User.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notification center")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": items, "unread": unread})
}

func (a *App) handleInboxNotificationAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[3] != "read" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	principal, _ := principalFromContext(r.Context())
	found, err := a.store.MarkInboxNotificationRead(r.Context(), principal.User.ID, parts[2])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark notification read")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"read": true})
}

func (a *App) handleMarkAllInboxNotificationsRead(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if err := a.store.MarkAllInboxNotificationsRead(r.Context(), principal.User.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark notifications read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"read": true})
}

func (a *App) handleGetDeviceNotificationSettings(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	principal, _ := principalFromContext(r.Context())
	settings, _, err := a.store.GetDeviceNotificationSettings(r.Context(), principal.User.ID, parts[2])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device notification settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (a *App) handleUpdateDeviceNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var req deviceNotificationSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var pausedUntil *time.Time
	if value := strings.TrimSpace(req.PausedUntil); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil || parsed.After(time.Now().UTC().Add(30*24*time.Hour)) {
			writeError(w, http.StatusBadRequest, "paused_until must be RFC3339 and no more than 30 days")
			return
		}
		parsed = parsed.UTC()
		pausedUntil = &parsed
	}
	principal, _ := principalFromContext(r.Context())
	settings, err := a.store.UpsertDeviceNotificationSettings(r.Context(), principal.User.ID, model.DeviceNotificationSettings{
		DeviceID: parts[2], Enabled: req.Enabled, NotifyWarning: req.NotifyWarning,
		NotifyCritical: req.NotifyCritical, NotifyResolved: req.NotifyResolved, PausedUntil: pausedUntil,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save device notification settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (a *App) handleGetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	settings, _, err := a.store.GetNotificationSettings(r.Context(), principal.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notification settings")
		return
	}
	writeJSON(w, http.StatusOK, a.notificationSettingsResponse(r.Context(), settings, principal.User))
}

func (a *App) handleUpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var req notificationSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	principal, _ := principalFromContext(r.Context())
	var pausedUntil *time.Time
	if value := strings.TrimSpace(req.AlertsPausedUntil); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "alerts_paused_until must be RFC3339")
			return
		}
		parsed = parsed.UTC()
		pausedUntil = &parsed
	}
	settings := model.NotificationSettings{
		UserID:                 principal.User.ID,
		EmailEnabled:           req.EmailEnabled,
		TelegramEnabled:        req.TelegramEnabled,
		TelegramChatID:         strings.TrimSpace(req.TelegramChatID),
		NotifyWarning:          req.NotifyWarning,
		NotifyCritical:         req.NotifyCritical,
		NotifyResolved:         req.NotifyResolved,
		MemoryThresholdPercent: req.MemoryThresholdPercent,
		DiskThresholdPercent:   req.DiskThresholdPercent,
		PacketLossPercent:      req.PacketLossPercent,
		LatencyThresholdMS:     req.LatencyThresholdMS,
		RepeatMinutes:          req.RepeatMinutes,
		Timezone:               strings.TrimSpace(req.Timezone),
		QuietHoursEnabled:      req.QuietHoursEnabled,
		QuietHoursStart:        strings.TrimSpace(req.QuietHoursStart),
		QuietHoursEnd:          strings.TrimSpace(req.QuietHoursEnd),
		AlertsPausedUntil:      pausedUntil,
		WebhookEnabled:         req.WebhookEnabled,
		WebhookURL:             strings.TrimSpace(req.WebhookURL),
		WebhookSecret:          strings.TrimSpace(req.WebhookSecret),
	}
	if settings.Timezone == "" {
		settings.Timezone = "UTC"
	}
	if settings.QuietHoursStart == "" {
		settings.QuietHoursStart = "22:00"
	}
	if settings.QuietHoursEnd == "" {
		settings.QuietHoursEnd = "08:00"
	}
	if current, _, currentErr := a.store.GetNotificationSettings(r.Context(), principal.User.ID); currentErr == nil {
		settings.WebhookSecretConfigured = current.WebhookSecretConfigured
	}
	if message := a.validateNotificationSettings(r.Context(), settings, principal.User); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	stored, err := a.store.UpsertNotificationSettings(r.Context(), settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save notification settings")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "notifications.settings_update", "", "", mustJSON(map[string]any{
		"email_enabled":    stored.EmailEnabled,
		"telegram_enabled": stored.TelegramEnabled,
		"request_id":       requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, a.notificationSettingsResponse(r.Context(), stored, principal.User))
}

func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	query := r.URL.Query()
	opts := store.NotificationListOptions{
		UserID:   principal.User.ID,
		DeviceID: strings.TrimSpace(query.Get("device_id")),
		Severity: strings.TrimSpace(query.Get("severity")),
		Event:    strings.TrimSpace(query.Get("event")),
		Channel:  strings.TrimSpace(query.Get("channel")),
		Status:   strings.TrimSpace(query.Get("status")),
		Limit:    limit,
	}
	if !allowedNotificationFilter(opts.Severity, "", "warning", "critical") ||
		!allowedNotificationFilter(opts.Event, "", "active", "repeat", "resolved", "test") ||
		!allowedNotificationFilter(opts.Channel, "", "email", "telegram", "webhook") ||
		!allowedNotificationFilter(opts.Status, "", "queued", "sending", "retry", "sent", "failed", "dead_letter") {
		writeError(w, http.StatusBadRequest, "invalid notification filter")
		return
	}
	deliveries, err := a.store.ListNotificationDeliveries(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notifications")
		return
	}
	metrics, err := a.store.NotificationDeliveryMetrics(r.Context(), principal.User.ID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notification metrics")
		return
	}
	sanitizeNotificationMetrics(&metrics)
	writeJSON(w, http.StatusOK, map[string]any{"notifications": deliveries, "metrics": metrics})
}

func allowedNotificationFilter(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (a *App) handleTestNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	settings, found, err := a.store.GetNotificationSettings(r.Context(), principal.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notification settings")
		return
	}
	if !found || (!settings.EmailEnabled && !settings.TelegramEnabled) {
		writeError(w, http.StatusConflict, "enable and save at least one notification channel")
		return
	}
	deliveries := a.notificationDeliveriesForMessage(principal.User, settings, "test", "", "", "Проверка уведомлений OpenWrt RMM", "Канал настроен правильно. Тестовое уведомление отправлено из личного кабинета.")
	if len(deliveries) == 0 {
		writeError(w, http.StatusConflict, "no available notification channels")
		return
	}
	results := make([]model.NotificationDelivery, 0, len(deliveries))
	for _, candidate := range deliveries {
		created, inserted, createErr := a.store.CreateNotificationDelivery(r.Context(), candidate, "test:"+principal.User.ID+":"+candidate.Channel+":"+time.Now().UTC().Format(time.RFC3339Nano))
		if createErr != nil || !inserted {
			writeError(w, http.StatusInternalServerError, "failed to create test notification")
			return
		}
		claimed, found, claimErr := a.store.ClaimNotificationDelivery(r.Context(), created.ID, time.Now().UTC(), notificationDeliveryLease)
		if claimErr != nil || !found {
			writeError(w, http.StatusInternalServerError, "failed to claim test notification")
			return
		}
		results = append(results, a.deliverNotification(r.Context(), claimed))
	}
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "notifications.test", "", "", mustJSON(map[string]string{"request_id": requestID(r.Context())}))
	writeJSON(w, http.StatusOK, map[string]any{"notifications": results})
}

func (a *App) notificationSettingsResponse(ctx context.Context, settings model.NotificationSettings, user model.User) map[string]any {
	emailVerified, _ := a.store.ContactVerified(ctx, user.ID, "email", user.Email)
	telegramVerified, _ := a.store.ContactVerified(ctx, user.ID, "telegram", settings.TelegramChatID)
	metrics, _ := a.store.NotificationDeliveryMetrics(ctx, user.ID, time.Now().UTC())
	sanitizeNotificationMetrics(&metrics)
	channelMetrics := map[string]model.NotificationChannelMetrics{}
	for _, item := range metrics.Channels {
		channelMetrics[item.Channel] = item
	}
	emailStatus, emailMessage := notificationChannelDiagnostic(
		a.alertEmailSender != nil,
		user.Email != "",
		emailVerified,
		settings.EmailEnabled,
		"SMTP не настроен на сервере",
		"Добавьте e-mail в профиле",
		"Подтвердите e-mail кодом",
	)
	telegramStatus, telegramMessage := notificationChannelDiagnostic(
		a.telegramSender != nil,
		validTelegramChatID(settings.TelegramChatID),
		telegramVerified,
		settings.TelegramEnabled,
		"Telegram Bot Token не настроен на сервере",
		"Укажите Telegram Chat ID",
		"Подтвердите Telegram кодом",
	)
	webhookConfigured := settings.WebhookURL != "" && settings.WebhookSecretConfigured
	webhookStatus, webhookMessage := notificationChannelDiagnostic(
		true,
		webhookConfigured,
		webhookConfigured,
		settings.WebhookEnabled,
		"",
		"Укажите HTTPS URL и секрет не короче 32 символов",
		"",
	)
	return map[string]any{
		"settings": settings,
		"channels": map[string]any{
			"email": map[string]any{
				"available": a.alertEmailSender != nil, "destination": maskEmail(user.Email),
				"profile_email_configured": user.Email != "", "verified": emailVerified,
				"status": emailStatus, "message": emailMessage, "delivery": channelMetrics["email"],
			},
			"telegram": map[string]any{
				"available": a.telegramSender != nil, "verified": telegramVerified,
				"status": telegramStatus, "message": telegramMessage, "delivery": channelMetrics["telegram"],
			},
			"webhook": map[string]any{
				"available": true, "configured": webhookConfigured,
				"status": webhookStatus, "message": webhookMessage, "delivery": channelMetrics["webhook"],
			},
		},
	}
}

func sanitizeNotificationMetrics(metrics *model.NotificationDeliveryMetrics) {
	if metrics == nil {
		return
	}
	for index := range metrics.Channels {
		if metrics.Channels[index].LastError != "" {
			metrics.Channels[index].LastError = "Канал не подтвердил доставку. Проверьте настройки и отправьте тест."
		}
	}
}

func notificationChannelDiagnostic(available, configured, verified, enabled bool, unavailableMessage, configureMessage, verifyMessage string) (string, string) {
	switch {
	case !available:
		return "unavailable", unavailableMessage
	case !enabled:
		if configured && verified {
			return "disabled", "Канал настроен, но выключен"
		}
		return "disabled", "Канал выключен"
	case !configured:
		return "attention", configureMessage
	case !verified:
		return "attention", verifyMessage
	default:
		return "ready", "Канал готов к отправке"
	}
}

func (a *App) validateNotificationSettings(ctx context.Context, settings model.NotificationSettings, user model.User) string {
	if settings.EmailEnabled && a.alertEmailSender == nil {
		return "email notifications are not configured on the server"
	}
	if settings.EmailEnabled && strings.TrimSpace(user.Email) == "" {
		return "add an email address to the profile before enabling email notifications"
	}
	if settings.EmailEnabled {
		verified, _ := a.store.ContactVerified(ctx, user.ID, "email", user.Email)
		if !verified {
			return "verify the profile email before enabling notifications"
		}
	}
	if settings.TelegramEnabled && a.telegramSender == nil {
		return "Telegram notifications are not configured on the server"
	}
	if settings.TelegramEnabled && !validTelegramChatID(settings.TelegramChatID) {
		return "Telegram chat ID is invalid"
	}
	if settings.TelegramEnabled {
		verified, _ := a.store.ContactVerified(ctx, user.ID, "telegram", settings.TelegramChatID)
		if !verified {
			return "verify Telegram ownership before enabling notifications"
		}
	}
	if settings.MemoryThresholdPercent < 50 || settings.MemoryThresholdPercent > 99 || settings.DiskThresholdPercent < 50 || settings.DiskThresholdPercent > 99 {
		return "memory and disk thresholds must be between 50 and 99 percent"
	}
	if settings.PacketLossPercent < 1 || settings.PacketLossPercent > 100 {
		return "packet loss threshold must be between 1 and 100 percent"
	}
	if settings.LatencyThresholdMS < 10 || settings.LatencyThresholdMS > 5000 {
		return "latency threshold must be between 10 and 5000 milliseconds"
	}
	if settings.RepeatMinutes != 0 && (settings.RepeatMinutes < 15 || settings.RepeatMinutes > 10080) {
		return "repeat interval must be 0 or between 15 and 10080 minutes"
	}
	if settings.Timezone == "" {
		settings.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return "timezone is invalid"
	}
	if !validClock(settings.QuietHoursStart) || !validClock(settings.QuietHoursEnd) {
		return "quiet hours must use HH:MM"
	}
	if settings.AlertsPausedUntil != nil && settings.AlertsPausedUntil.After(time.Now().UTC().Add(30*24*time.Hour)) {
		return "alerts can be paused for at most 30 days"
	}
	if settings.WebhookEnabled {
		if message := validateWebhookEndpoint(settings.WebhookURL); message != "" {
			return message
		}
		if !settings.WebhookSecretConfigured && len(settings.WebhookSecret) < 32 {
			return "webhook secret must contain at least 32 characters"
		}
	}
	return ""
}

func validClock(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

func notificationQuietNow(settings model.NotificationSettings, now time.Time) bool {
	if settings.AlertsPausedUntil != nil && now.Before(*settings.AlertsPausedUntil) {
		return true
	}
	if !settings.QuietHoursEnabled {
		return false
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return false
	}
	local := now.In(location)
	start, startErr := time.Parse("15:04", settings.QuietHoursStart)
	end, endErr := time.Parse("15:04", settings.QuietHoursEnd)
	if startErr != nil || endErr != nil {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()
	if startMinute == endMinute {
		return true
	}
	if startMinute < endMinute {
		return minute >= startMinute && minute < endMinute
	}
	return minute >= startMinute || minute < endMinute
}

func deviceNotificationEnabled(settings model.DeviceNotificationSettings, severity, event string, now time.Time) bool {
	if !settings.Enabled || (settings.PausedUntil != nil && now.Before(*settings.PausedUntil)) {
		return false
	}
	if event == "resolved" {
		return settings.NotifyResolved
	}
	if severity == "critical" {
		return settings.NotifyCritical
	}
	return settings.NotifyWarning
}

func validTelegramChatID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 32 {
		return false
	}
	if strings.HasPrefix(value, "-") {
		value = value[1:]
	}
	_, err := strconv.ParseInt(value, 10, 64)
	return err == nil
}

func (a *App) alertNotificationLoop() {
	ticker := time.NewTicker(a.notificationWorkerInterval)
	defer ticker.Stop()
	for {
		if err := a.runAlertNotificationCycle(context.Background()); err != nil {
			log.Printf("alert notification cycle failed: %v", err)
		}
		<-ticker.C
	}
}

func (a *App) runAlertNotificationCycle(ctx context.Context) error {
	devices, err := a.store.ListDevices(ctx)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if _, _, refreshErr := a.refreshDeviceAlerts(ctx, device.ID); refreshErr != nil {
			log.Printf("refresh alerts for %s: %v", device.ID, refreshErr)
			continue
		}
		_, queueErr := a.queueDeviceNotifications(ctx, device.ID)
		if queueErr != nil {
			log.Printf("queue notifications for %s: %v", device.ID, queueErr)
			continue
		}
	}
	return a.processNotificationQueue(ctx)
}

func (a *App) queueDeviceNotifications(ctx context.Context, deviceID string) ([]model.NotificationDelivery, error) {
	device, found, err := a.store.GetDevice(ctx, deviceID)
	if err != nil || !found || device.OwnerUserID == "" {
		return nil, err
	}
	settings, configured, err := a.store.GetNotificationSettings(ctx, device.OwnerUserID)
	if err != nil || !configured {
		return nil, err
	}
	user, found, err := a.store.GetUserByID(ctx, device.OwnerUserID)
	if err != nil || !found || user.Disabled {
		return nil, err
	}
	alerts, err := a.store.ListAlerts(ctx, store.AlertListOptions{DeviceID: deviceID, Status: "all", Limit: 200})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	queued := make([]model.NotificationDelivery, 0)
	for _, alert := range alerts {
		if alert.FirstSeenAt.Before(settings.CreatedAt) || !notificationSeverityEnabled(settings, alert.Severity) {
			continue
		}
		event := "active"
		if alert.Status == "resolved" {
			if !settings.NotifyResolved || alert.ResolvedAt == nil || alert.ResolvedAt.Before(settings.CreatedAt) {
				continue
			}
			event = "resolved"
		} else if alert.Status != "active" && alert.Status != "acknowledged" {
			continue
		}
		title, body := notificationCopy(device, alert, event, a.publicURL)
		lifecycle := alert.FirstSeenAt.UTC().Format(time.RFC3339Nano)
		if event == "resolved" && alert.ResolvedAt != nil {
			lifecycle = alert.ResolvedAt.UTC().Format(time.RFC3339Nano)
		}
		incidentID := device.ID + ":" + alert.FirstSeenAt.UTC().Truncate(5*time.Minute).Format("20060102T1504")
		_, inboxInserted, inboxErr := a.store.CreateInboxNotification(ctx, model.InboxNotification{
			UserID: user.ID, DeviceID: device.ID, IncidentID: incidentID, Severity: alert.Severity,
			Event: event, Title: title, Body: body,
		}, "inbox:"+alert.ID+":"+lifecycle+":"+event)
		if inboxErr != nil {
			return nil, inboxErr
		}
		if inboxInserted {
			a.events.publish("notifications")
		}
		deviceSettings, _, overrideErr := a.store.GetDeviceNotificationSettings(ctx, user.ID, device.ID)
		if overrideErr != nil {
			return nil, overrideErr
		}
		if !deviceNotificationEnabled(deviceSettings, alert.Severity, event, now) || notificationQuietNow(settings, now) {
			continue
		}
		candidates := a.notificationDeliveriesForMessage(user, settings, event, device.ID, alert.ID, title, body)
		for _, candidate := range candidates {
			dedupeKey := alert.ID + ":" + lifecycle + ":" + event + ":" + candidate.Channel
			created, inserted, createErr := a.store.CreateNotificationDelivery(ctx, candidate, dedupeKey)
			if createErr != nil {
				return nil, createErr
			}
			if inserted {
				queued = append(queued, created)
				continue
			}
			if event != "active" || settings.RepeatMinutes == 0 {
				continue
			}
			last, lastFound, lastErr := a.store.LatestNotificationDelivery(ctx, user.ID, alert.ID, candidate.Channel)
			if lastErr != nil {
				return nil, lastErr
			}
			retryAfter := time.Duration(settings.RepeatMinutes) * time.Minute
			if !lastFound || last.CreatedAt.Add(retryAfter).After(now) {
				continue
			}
			candidate.Event = "repeat"
			repeatBucket := now.Unix() / int64(retryAfter.Seconds())
			created, inserted, createErr = a.store.CreateNotificationDelivery(ctx, candidate, alert.ID+":"+lifecycle+":repeat:"+candidate.Channel+":"+strconv.FormatInt(repeatBucket, 10))
			if createErr != nil {
				return nil, createErr
			}
			if inserted {
				queued = append(queued, created)
			}
		}
	}
	return queued, nil
}

func notificationSeverityEnabled(settings model.NotificationSettings, severity string) bool {
	if severity == "critical" {
		return settings.NotifyCritical
	}
	return settings.NotifyWarning
}

func (a *App) notificationDeliveriesForMessage(user model.User, settings model.NotificationSettings, event, deviceID, alertID, title, body string) []model.NotificationDelivery {
	deliveries := make([]model.NotificationDelivery, 0, 3)
	base := model.NotificationDelivery{
		UserID: user.ID, DeviceID: deviceID, AlertID: alertID, Event: event,
		Title: title, Body: body, MaxAttempts: a.notificationMaxAttempts,
	}
	if settings.EmailEnabled && a.alertEmailSender != nil && user.Email != "" {
		delivery := base
		delivery.Channel = "email"
		delivery.Destination = user.Email
		delivery.DestinationMasked = maskEmail(user.Email)
		deliveries = append(deliveries, delivery)
	}
	if settings.TelegramEnabled && a.telegramSender != nil && settings.TelegramChatID != "" {
		delivery := base
		delivery.Channel = "telegram"
		delivery.Destination = settings.TelegramChatID
		delivery.DestinationMasked = maskTelegramChatID(settings.TelegramChatID)
		deliveries = append(deliveries, delivery)
	}
	if settings.WebhookEnabled && settings.WebhookURL != "" && settings.WebhookSecret != "" {
		delivery := base
		delivery.Channel = "webhook"
		delivery.Destination = settings.WebhookURL
		delivery.DestinationMasked = maskWebhookURL(settings.WebhookURL)
		deliveries = append(deliveries, delivery)
	}
	return deliveries
}

func (a *App) processNotificationQueue(ctx context.Context) error {
	deliveries, err := a.store.ClaimNotificationDeliveries(ctx, time.Now().UTC(), notificationDeliveryLease, 10)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		deliveryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		a.deliverNotification(deliveryCtx, delivery)
		cancel()
	}
	if len(deliveries) > 0 {
		a.events.publish("notifications")
	}
	return nil
}

func (a *App) deliverNotification(ctx context.Context, delivery model.NotificationDelivery) model.NotificationDelivery {
	var sender NotificationSender
	if delivery.Channel == "email" {
		sender = a.alertEmailSender
	} else if delivery.Channel == "telegram" {
		sender = a.telegramSender
	}
	err := errors.New("notification channel is unavailable")
	if delivery.Channel == "webhook" {
		settings, _, settingsErr := a.store.GetNotificationSettings(ctx, delivery.UserID)
		if settingsErr != nil {
			err = settingsErr
		} else {
			err = sendSignedWebhook(ctx, settings.WebhookURL, settings.WebhookSecret, delivery)
		}
	} else if sender != nil {
		err = sender.SendNotification(ctx, delivery.Destination, delivery.Title, delivery.Body)
	}
	if err != nil {
		log.Printf("notification delivery %s via %s failed: %v", delivery.ID, delivery.Channel, err)
		delivery.Status = "retry"
		delivery.Error = "Канал не подтвердил доставку. Проверьте его настройки и повторите тест."
		var nextAttemptAt *time.Time
		if delivery.AttemptCount >= delivery.MaxAttempts {
			delivery.Status = "dead_letter"
		} else {
			next := time.Now().UTC().Add(notificationRetryDelay(delivery.AttemptCount))
			nextAttemptAt = &next
			delivery.NextAttemptAt = &next
		}
		if completeErr := a.store.CompleteNotificationDelivery(context.Background(), delivery.ID, delivery.Status, delivery.Error, nextAttemptAt); completeErr != nil {
			log.Printf("notification delivery %s completion failed: %v", delivery.ID, completeErr)
		}
		return delivery
	}
	now := time.Now().UTC()
	delivery.Status = "sent"
	delivery.SentAt = &now
	delivery.NextAttemptAt = nil
	if completeErr := a.store.CompleteNotificationDelivery(context.Background(), delivery.ID, "sent", "", nil); completeErr != nil {
		log.Printf("notification delivery %s completion failed: %v", delivery.ID, completeErr)
	}
	return delivery
}

func notificationRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 30 * time.Second
	for current := 1; current < attempt && delay < 30*time.Minute; current++ {
		delay *= 2
	}
	if delay > 30*time.Minute {
		return 30 * time.Minute
	}
	return delay
}

func notificationCopy(device model.Device, alert model.Alert, event, publicURL string) (string, string) {
	name := strings.TrimSpace(device.Hostname)
	if name == "" {
		name = device.ID
	}
	label := notificationAlertLabel(alert.Type)
	if event == "resolved" {
		return "Восстановлено: " + name, fmt.Sprintf("Проблема «%s» устранена. Роутер: %s.%s", label, name, notificationLink(publicURL))
	}
	prefix := "Предупреждение"
	if alert.Severity == "critical" {
		prefix = "Критическая проблема"
	}
	return prefix + ": " + name, fmt.Sprintf("%s. Роутер: %s. Состояние: %s.%s", label, name, alert.Message, notificationLink(publicURL))
}

func notificationAlertLabel(alertType string) string {
	labels := map[string]string{
		"offline": "роутер не на связи", "memory_high": "высокое использование памяти", "disk_high": "заканчивается место",
		"wan_down": "нет доступа в интернет", "packet_loss_high": "высокая потеря пакетов", "latency_high": "высокая задержка",
		"wan_ip_changed": "изменился WAN IP", "command_attention": "операция требует внимания",
	}
	if label := labels[alertType]; label != "" {
		return label
	}
	return alertType
}

func notificationLink(publicURL string) string {
	if strings.TrimSpace(publicURL) == "" {
		return ""
	}
	return "\n\nОткрыть RMM: " + strings.TrimRight(publicURL, "/") + "/app"
}

func maskEmail(value string) string {
	parts := strings.SplitN(strings.TrimSpace(value), "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}
	return parts[0][:1] + "***@" + parts[1]
}

func maskTelegramChatID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return "***"
	}
	return "***" + value[len(value)-4:]
}
