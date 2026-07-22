package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/store"
)

func main() {
	addr := env("RMM_ADDR", ":8080")
	dbPath := env("RMM_DB_PATH", "rmm.db")
	insecureDevMode := envBool("RMM_INSECURE_DEV_MODE", false)
	operatorPassword := strings.TrimSpace(os.Getenv("RMM_OPERATOR_PASSWORD"))
	if operatorPassword == "" {
		if !insecureDevMode {
			log.Fatal("RMM_OPERATOR_PASSWORD is required (or explicitly enable RMM_INSECURE_DEV_MODE for a local lab)")
		}
		operatorPassword = "dev-operator-password"
	}
	if !insecureDevMode && insecurePlaceholder(operatorPassword) {
		log.Fatal("RMM_OPERATOR_PASSWORD still contains an insecure example value")
	}
	operatorToken := strings.TrimSpace(os.Getenv("RMM_OPERATOR_TOKEN"))
	if operatorToken != "" && !insecureDevMode && insecurePlaceholder(operatorToken) {
		log.Fatal("RMM_OPERATOR_TOKEN still contains an insecure example value")
	}
	allowLegacyEnrollment := envBool("RMM_ALLOW_LEGACY_ENROLLMENT", false)
	enrollmentToken := ""
	if allowLegacyEnrollment {
		enrollmentToken = strings.TrimSpace(os.Getenv("RMM_ENROLLMENT_TOKEN"))
		if enrollmentToken == "" {
			log.Fatal("RMM_ENROLLMENT_TOKEN is required when legacy enrollment is enabled")
		}
		if !insecureDevMode && insecurePlaceholder(enrollmentToken) {
			log.Fatal("RMM_ENROLLMENT_TOKEN still contains an insecure example value")
		}
	}
	var passwordResetSender httpapi.PasswordResetSender
	var alertEmailSender httpapi.NotificationSender
	smtpHost := strings.TrimSpace(os.Getenv("RMM_SMTP_HOST"))
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("RMM_PUBLIC_URL")), "/")
	if smtpHost != "" {
		tlsMode := env("RMM_SMTP_TLS_MODE", "starttls")
		if !insecureDevMode && strings.EqualFold(tlsMode, "none") {
			log.Fatal("RMM_SMTP_TLS_MODE=none is allowed only in insecure development mode")
		}
		if publicURL == "" {
			log.Fatal("RMM_PUBLIC_URL is required when SMTP password recovery is enabled")
		}
		if !insecureDevMode && !strings.HasPrefix(strings.ToLower(publicURL), "https://") {
			log.Fatal("RMM_PUBLIC_URL must use https when SMTP password recovery is enabled")
		}
		sender, smtpErr := httpapi.NewSMTPPasswordResetSender(httpapi.SMTPConfig{
			Host:       smtpHost,
			Port:       envInt("RMM_SMTP_PORT", 587, 1, 65535),
			Username:   strings.TrimSpace(os.Getenv("RMM_SMTP_USERNAME")),
			Password:   os.Getenv("RMM_SMTP_PASSWORD"),
			From:       strings.TrimSpace(os.Getenv("RMM_SMTP_FROM")),
			TLSMode:    tlsMode,
			ServerName: strings.TrimSpace(os.Getenv("RMM_SMTP_SERVER_NAME")),
		})
		if smtpErr != nil {
			log.Fatalf("invalid SMTP configuration: %v", smtpErr)
		}
		passwordResetSender = sender
		alertEmailSender = sender
	}
	var telegramSender httpapi.NotificationSender
	if telegramToken := strings.TrimSpace(os.Getenv("RMM_TELEGRAM_BOT_TOKEN")); telegramToken != "" {
		sender, telegramErr := httpapi.NewTelegramNotificationSender(telegramToken)
		if telegramErr != nil {
			log.Fatalf("invalid Telegram configuration: %v", telegramErr)
		}
		telegramSender = sender
	}

	st, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	if err := runMaintenance(context.Background(), st); err != nil {
		log.Printf("initial maintenance failed: %v", err)
	}
	go maintenanceLoop(st)

	handler := httpapi.NewHandler(st, httpapi.Config{
		EnrollmentToken:         enrollmentToken,
		AllowLegacyEnrollment:   allowLegacyEnrollment,
		AllowLegacyLuCIProxy:    envBool("RMM_ALLOW_LEGACY_LUCI_PROXY", false),
		OperatorToken:           operatorToken,
		OperatorUsername:        env("RMM_OPERATOR_USERNAME", "admin"),
		OperatorPassword:        operatorPassword,
		CookieSecure:            envBool("RMM_COOKIE_SECURE", !insecureDevMode),
		TunnelHTTPHost:          env("RMM_TUNNEL_HTTP_HOST", "tunnel-ssh"),
		TunnelPublicHost:        strings.TrimSpace(os.Getenv("RMM_TUNNEL_PUBLIC_HOST")),
		TunnelPublicPort:        envInt("RMM_TUNNEL_PUBLIC_PORT", 2222, 1, 65535),
		DeviceDomain:            strings.TrimSpace(os.Getenv("RMM_DEVICE_DOMAIN")),
		PublicScheme:            env("RMM_PUBLIC_SCHEME", "https"),
		PublicURL:               publicURL,
		PasswordResetSender:     passwordResetSender,
		AlertEmailSender:        alertEmailSender,
		TelegramSender:          telegramSender,
		BackgroundTasks:         true,
		NotificationMaxAttempts: envInt("RMM_NOTIFICATION_MAX_ATTEMPTS", 5, 1, 20),
		StaticDir:               env("RMM_WEB_DIR", "web"),
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("rmm server listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

func insecurePlaceholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "dev-") || strings.HasPrefix(value, "replace-with-") || strings.Contains(value, "change-me")
}

type maintenanceStore interface {
	PurgeExpiredSecurityData(ctx context.Context) error
	PurgeMetricSamplesBefore(ctx context.Context, cutoff time.Time) (int64, error)
	PurgeNotificationDeliveriesBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

func maintenanceLoop(st maintenanceStore) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if err := runMaintenance(context.Background(), st); err != nil {
			log.Printf("scheduled maintenance failed: %v", err)
		}
	}
}

func runMaintenance(ctx context.Context, st maintenanceStore) error {
	if err := st.PurgeExpiredSecurityData(ctx); err != nil {
		return err
	}
	retentionDays := envInt("RMM_METRIC_RETENTION_DAYS", 30, 1, 3650)
	deleted, err := st.PurgeMetricSamplesBefore(ctx, time.Now().UTC().Add(-time.Duration(retentionDays)*24*time.Hour))
	if err != nil {
		return err
	}
	if deleted > 0 {
		log.Printf("maintenance removed %d expired metric samples", deleted)
	}
	notificationRetentionDays := envInt("RMM_NOTIFICATION_RETENTION_DAYS", 90, 1, 3650)
	deleted, err = st.PurgeNotificationDeliveriesBefore(ctx, time.Now().UTC().Add(-time.Duration(notificationRetentionDays)*24*time.Hour))
	if err != nil {
		return err
	}
	if deleted > 0 {
		log.Printf("maintenance removed %d expired notification deliveries", deleted)
	}
	return nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback, minimum, maximum int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		log.Print(fmt.Sprintf("invalid %s=%q; using %d", key, raw, fallback))
		return fallback
	}
	return value
}
