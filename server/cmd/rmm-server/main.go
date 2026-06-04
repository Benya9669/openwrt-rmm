package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/store"
)

func main() {
	addr := env("RMM_ADDR", ":8080")
	dbPath := env("RMM_DB_PATH", "rmm.db")

	st, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	handler := httpapi.NewHandler(st, httpapi.Config{
		EnrollmentToken:  env("RMM_ENROLLMENT_TOKEN", "dev-enroll-token"),
		OperatorToken:    env("RMM_OPERATOR_TOKEN", "dev-operator-token"),
		OperatorUsername: env("RMM_OPERATOR_USERNAME", "admin"),
		OperatorPassword: env("RMM_OPERATOR_PASSWORD", "dev-operator-password"),
		SessionSecret:    env("RMM_SESSION_SECRET", "dev-session-secret-change-me"),
		CookieSecure:     envBool("RMM_COOKIE_SECURE", false),
		TunnelHTTPHost:   env("RMM_TUNNEL_HTTP_HOST", "tunnel-ssh"),
		StaticDir:        env("RMM_WEB_DIR", "web"),
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("rmm server listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
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
