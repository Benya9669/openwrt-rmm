package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
)

type clientInventory struct {
	DHCPLeases []map[string]string `json:"dhcp_leases"`
	Neighbors  []map[string]string `json:"neighbors"`
	WiFi       []map[string]string `json:"wifi_clients"`
	Probes     []map[string]string `json:"client_probes"`
}

func (s *Store) SyncLANClients(ctx context.Context, deviceID string, inventory json.RawMessage, checkedAt time.Time) error {
	var payload clientInventory
	if err := json.Unmarshal(inventory, &payload); err != nil {
		return nil
	}
	type observed struct {
		key, mac, ip, hostname, iface, connection, confirmation string
		confirmed                                               bool
	}
	byKey := map[string]*observed{}
	for _, lease := range payload.DHCPLeases {
		mac := normalizeMAC(lease["mac"])
		ip := strings.TrimSpace(lease["ip"])
		key := clientKey(mac, ip)
		if key == "" {
			continue
		}
		byKey[key] = &observed{key: key, mac: mac, ip: ip, hostname: cleanHostname(lease["hostname"]), connection: "dhcp", confirmation: "lease"}
	}
	for _, neighbor := range payload.Neighbors {
		mac := normalizeMAC(neighbor["mac"])
		ip := strings.TrimSpace(neighbor["ip"])
		key := clientKey(mac, ip)
		if key == "" {
			continue
		}
		state := strings.ToUpper(strings.TrimSpace(neighbor["state"]))
		item := byKey[key]
		if item == nil {
			// Kernel neighbour tables retain failed and stale probes for a while.
			// They are useful for enriching a DHCP lease, but must not create a
			// standalone LAN client until the kernel has actively confirmed it.
			if mac == "" || !activeNeighborState(state) {
				continue
			}
			item = &observed{key: key, mac: mac, ip: ip}
			byKey[key] = item
		}
		item.ip = preferredClientIP(item.ip, ip)
		item.iface = strings.TrimSpace(neighbor["interface"])
		if activeNeighborState(state) {
			item.confirmed = true
			item.confirmation = "neighbor:" + strings.ToLower(state)
			item.connection = "wired"
		}
	}
	for _, station := range payload.WiFi {
		mac := normalizeMAC(station["mac"])
		key := clientKey(mac, "")
		if key == "" {
			continue
		}
		item := byKey[key]
		if item == nil {
			item = &observed{key: key, mac: mac}
			byKey[key] = item
		}
		item.iface = strings.TrimSpace(station["interface"])
		item.connection = "wifi"
		item.confirmation = "wifi"
		item.confirmed = true
	}
	for _, probe := range payload.Probes {
		if !strings.EqualFold(probe["reachable"], "true") {
			continue
		}
		mac := normalizeMAC(probe["mac"])
		ip := strings.TrimSpace(probe["ip"])
		key := clientKey(mac, ip)
		if key == "" {
			continue
		}
		item := byKey[key]
		if item == nil {
			item = &observed{key: key, mac: mac, ip: ip}
			byKey[key] = item
		}
		item.confirmed = true
		item.confirmation = "active_probe"
		if item.connection == "" || item.connection == "dhcp" {
			item.connection = "wired"
		}
	}
	now := notificationTimeText(checkedAt.UTC())
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM lan_clients
WHERE device_id = ?
  AND last_seen_at IS NULL
  AND confirmation != 'lease'
`, deviceID); err != nil {
		return err
	}
	for _, item := range byKey {
		var lastSeen any
		if item.confirmed {
			lastSeen = now
		}
		_, err := s.db.ExecContext(ctx, `
INSERT INTO lan_clients (
  device_id, client_key, mac, ip, hostname, interface, connection, confirmation,
  first_seen_at, last_seen_at, last_checked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(device_id, client_key) DO UPDATE SET
  mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE lan_clients.mac END,
  ip = CASE WHEN excluded.ip != '' THEN excluded.ip ELSE lan_clients.ip END,
  hostname = CASE WHEN excluded.hostname != '' THEN excluded.hostname ELSE lan_clients.hostname END,
  interface = CASE WHEN excluded.interface != '' THEN excluded.interface ELSE lan_clients.interface END,
  connection = CASE WHEN excluded.connection != '' THEN excluded.connection ELSE lan_clients.connection END,
  confirmation = CASE WHEN excluded.confirmation != '' THEN excluded.confirmation ELSE lan_clients.confirmation END,
  last_seen_at = COALESCE(excluded.last_seen_at, lan_clients.last_seen_at),
  last_checked_at = excluded.last_checked_at
`, deviceID, item.key, item.mac, item.ip, item.hostname, item.iface, item.connection, item.confirmation, now, lastSeen, now)
		if err != nil {
			return err
		}
	}
	unconfirmedCutoff := notificationTimeText(checkedAt.UTC().Add(-24 * time.Hour))
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM lan_clients
WHERE device_id = ?
  AND last_seen_at IS NULL
  AND last_checked_at < ?
`, deviceID, unconfirmedCutoff); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListLANClients(ctx context.Context, deviceID string, recentFor time.Duration) ([]model.LANClient, bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil || !exists {
		return nil, exists, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT device_id, client_key, mac, ip, hostname, interface, connection, confirmation,
       first_seen_at, last_seen_at, last_checked_at
FROM lan_clients WHERE device_id = ?
ORDER BY COALESCE(last_seen_at, '') DESC, hostname, ip
`, deviceID)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	clients := make([]model.LANClient, 0)
	for rows.Next() {
		var client model.LANClient
		var firstSeen, checked string
		var lastSeen sql.NullString
		if err := rows.Scan(&client.DeviceID, &client.Key, &client.MAC, &client.IP, &client.Hostname, &client.Interface,
			&client.Connection, &client.Confirmation, &firstSeen, &lastSeen, &checked); err != nil {
			return nil, true, err
		}
		client.FirstSeenAt = parseTime(firstSeen)
		client.LastCheckedAt = parseTime(checked)
		client.Status = "unconfirmed"
		if lastSeen.Valid && lastSeen.String != "" {
			value := parseTime(lastSeen.String)
			client.LastSeenAt = &value
			if now.Sub(value) <= 2*time.Minute {
				client.Status = "online"
			} else if now.Sub(value) <= recentFor {
				client.Status = "recent"
			}
		}
		clients = append(clients, client)
	}
	return clients, true, rows.Err()
}

func clientKey(mac, ip string) string {
	if mac != "" {
		return "mac:" + strings.ToLower(mac)
	}
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() != nil {
		return "ip:" + parsed.String()
	}
	return ""
}

func activeNeighborState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "REACHABLE", "DELAY", "PROBE":
		return true
	default:
		return false
	}
}

func preferredClientIP(current, candidate string) string {
	current = strings.TrimSpace(current)
	candidate = strings.TrimSpace(candidate)
	if current == "" {
		return candidate
	}
	currentIP := net.ParseIP(current)
	candidateIP := net.ParseIP(candidate)
	if currentIP != nil && currentIP.To4() == nil && candidateIP != nil && candidateIP.To4() != nil {
		return candidateIP.String()
	}
	return current
}

func normalizeMAC(value string) string {
	mac, err := net.ParseMAC(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return strings.ToLower(mac.String())
}

func cleanHostname(value string) string {
	value = strings.TrimSpace(value)
	if value == "*" {
		return ""
	}
	if len(value) > 253 {
		return value[:253]
	}
	return value
}
