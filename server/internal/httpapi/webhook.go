package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
)

func validateWebhookEndpoint(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "webhook URL must be an absolute HTTPS URL without credentials"
	}
	if parsed.Fragment != "" {
		return "webhook URL must not contain a fragment"
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !publicWebhookIP(ip) {
		return "webhook URL must not target a private or local address"
	}
	return ""
}

func sendSignedWebhook(ctx context.Context, endpoint, secret string, delivery model.NotificationDelivery) error {
	if message := validateWebhookEndpoint(endpoint); message != "" {
		return fmt.Errorf("%s", message)
	}
	parsed, _ := url.Parse(endpoint)
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return fmt.Errorf("resolve webhook endpoint: %w", err)
	}
	for _, address := range addresses {
		if !publicWebhookIP(address.IP) {
			return fmt.Errorf("webhook endpoint resolved to a private or local address")
		}
	}
	pinnedIP := addresses[0].IP.String()
	payload, err := json.Marshal(map[string]any{
		"id": delivery.ID, "device_id": delivery.DeviceID, "alert_id": delivery.AlertID,
		"event": delivery.Event, "title": delivery.Title, "body": delivery.Body,
		"created_at": delivery.CreatedAt,
	})
	if err != nil {
		return err
	}
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RMM-Timestamp", timestamp)
	req.Header.Set("X-RMM-Signature", "sha256="+signature)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse webhook dial address: %w", err)
		}
		if !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(parsed.Hostname(), ".")) {
			return nil, fmt.Errorf("webhook redirect to another host is not allowed")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(pinnedIP, port))
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

func publicWebhookIP(ip net.IP) bool {
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func maskWebhookURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
