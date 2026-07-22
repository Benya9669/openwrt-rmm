package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
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
}

func (a *App) handleGetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	settings, _, err := a.store.GetNotificationSettings(r.Context(), principal.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notification settings")
		return
	}
	writeJSON(w, http.StatusOK, a.notificationSettingsResponse(settings, principal.User))
}

func (a *App) handleUpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var req notificationSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	principal, _ := principalFromContext(r.Context())
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
	}
	if message := a.validateNotificationSettings(settings, principal.User); message != "" {
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
	writeJSON(w, http.StatusOK, a.notificationSettingsResponse(stored, principal.User))
}

func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	deliveries, err := a.store.ListNotificationDeliveries(r.Context(), store.NotificationListOptions{UserID: principal.User.ID, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load notifications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": deliveries})
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

func (a *App) notificationSettingsResponse(settings model.NotificationSettings, user model.User) map[string]any {
	return map[string]any{
		"settings": settings,
		"channels": map[string]any{
			"email":    map[string]any{"available": a.alertEmailSender != nil, "destination": maskEmail(user.Email), "profile_email_configured": user.Email != ""},
			"telegram": map[string]any{"available": a.telegramSender != nil},
		},
	}
}

func (a *App) validateNotificationSettings(settings model.NotificationSettings, user model.User) string {
	if settings.EmailEnabled && a.alertEmailSender == nil {
		return "email notifications are not configured on the server"
	}
	if settings.EmailEnabled && strings.TrimSpace(user.Email) == "" {
		return "add an email address to the profile before enabling email notifications"
	}
	if settings.TelegramEnabled && a.telegramSender == nil {
		return "Telegram notifications are not configured on the server"
	}
	if settings.TelegramEnabled && !validTelegramChatID(settings.TelegramChatID) {
		return "Telegram chat ID is invalid"
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
	return ""
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
		candidates := a.notificationDeliveriesForMessage(user, settings, event, device.ID, alert.ID, title, body)
		for _, candidate := range candidates {
			lifecycle := alert.FirstSeenAt.UTC().Format(time.RFC3339Nano)
			if event == "resolved" && alert.ResolvedAt != nil {
				lifecycle = alert.ResolvedAt.UTC().Format(time.RFC3339Nano)
			}
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
	deliveries := make([]model.NotificationDelivery, 0, 2)
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
	if sender != nil {
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
