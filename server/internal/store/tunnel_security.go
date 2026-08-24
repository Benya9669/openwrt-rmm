package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrTunnelPortUnavailable = errors.New("tunnel port is already reserved")
	ErrTunnelSessionLimit    = errors.New("active tunnel session limit reached")
	ErrTunnelRateLimit       = errors.New("tunnel session rate limit reached")
)

type TunnelAuthorization struct {
	DeviceID  string
	PublicKey string
	Ports     []int
}

func (s *Store) SyncTunnelCredential(ctx context.Context, deviceID string, epoch int, publicKey, fingerprint string) (int, bool, error) {
	publicKey = strings.TrimSpace(publicKey)
	fingerprint = strings.TrimSpace(fingerprint)
	var expected int
	if err := s.db.QueryRowContext(ctx, `SELECT tunnel_key_epoch FROM devices WHERE id = ?`, deviceID).Scan(&expected); err != nil {
		return 0, false, err
	}
	if epoch != expected || publicKey == "" || fingerprint == "" {
		return expected, false, nil
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE devices
SET tunnel_public_key = ?, tunnel_key_fingerprint = ?, tunnel_credential_updated_at = ?
WHERE id = ? AND tunnel_key_epoch = ?
`, publicKey, fingerprint, nowText(), deviceID, epoch)
	if err != nil {
		return expected, false, err
	}
	updated, _ := result.RowsAffected()
	return expected, updated == 1, nil
}

func (s *Store) TunnelKeyEpoch(ctx context.Context, deviceID string) (int, error) {
	var epoch int
	err := s.db.QueryRowContext(ctx, `SELECT tunnel_key_epoch FROM devices WHERE id = ?`, deviceID).Scan(&epoch)
	return epoch, err
}

func (s *Store) TunnelCredentialReady(ctx context.Context, deviceID string) (bool, error) {
	var ready bool
	err := s.db.QueryRowContext(ctx, `
SELECT tunnel_public_key != '' AND tunnel_key_fingerprint != ''
FROM devices WHERE id = ?
`, deviceID).Scan(&ready)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return ready, err
}

func (s *Store) TunnelAuthorization(ctx context.Context, fingerprint string, at time.Time) (TunnelAuthorization, bool, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return TunnelAuthorization{}, false, nil
	}
	var auth TunnelAuthorization
	err := s.db.QueryRowContext(ctx, `
SELECT id, tunnel_public_key
FROM devices
WHERE tunnel_key_fingerprint = ? AND tunnel_public_key != ''
`, fingerprint).Scan(&auth.DeviceID, &auth.PublicKey)
	if errors.Is(err, sql.ErrNoRows) {
		return TunnelAuthorization{}, false, nil
	}
	if err != nil {
		return TunnelAuthorization{}, false, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT remote_port, luci_port
FROM remote_sessions
WHERE device_id = ?
  AND status IN ('requested', 'queued', 'active')
  AND julianday(expires_at) > julianday(?)
`, auth.DeviceID, at.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return TunnelAuthorization{}, false, err
	}
	defer rows.Close()
	ports := make(map[int]struct{})
	for rows.Next() {
		var remotePort, luciPort int
		if err := rows.Scan(&remotePort, &luciPort); err != nil {
			return TunnelAuthorization{}, false, err
		}
		if remotePort > 0 && remotePort <= 65535 {
			ports[remotePort] = struct{}{}
		}
		if luciPort > 0 && luciPort <= 65535 {
			ports[luciPort] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return TunnelAuthorization{}, false, err
	}
	for port := range ports {
		auth.Ports = append(auth.Ports, port)
	}
	sort.Ints(auth.Ports)
	return auth, len(auth.Ports) > 0, nil
}

func (s *Store) ActiveTunnelPorts(ctx context.Context, at time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT remote_port, luci_port
FROM remote_sessions
WHERE status IN ('requested', 'queued', 'active')
  AND julianday(expires_at) > julianday(?)
`, at.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	unique := make(map[int]struct{})
	for rows.Next() {
		var remotePort, luciPort int
		if err := rows.Scan(&remotePort, &luciPort); err != nil {
			return nil, err
		}
		for _, port := range []int{remotePort, luciPort} {
			if port >= 22000 && port <= 22199 {
				unique[port] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ports := make([]int, 0, len(unique))
	for port := range unique {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}
