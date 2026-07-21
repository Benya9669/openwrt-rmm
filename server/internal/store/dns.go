package store

import (
	"context"
	"database/sql"
	"errors"

	"rmm-openwrt/server/internal/model"
)

const dnsRecordSelect = `
SELECT r.device_id, d.dns_label, r.ipv4, r.ipv6, r.ttl, r.enabled,
       r.last_agent_update_at, r.created_at, r.updated_at
FROM device_dns_records r
JOIN devices d ON d.id = r.device_id
`

func (s *Store) ListDNSRecordsForUser(ctx context.Context, userID string, admin bool) ([]model.DNSRecord, error) {
	query := dnsRecordSelect + ` ORDER BY d.created_at DESC`
	args := []any{}
	if !admin {
		query = dnsRecordSelect + ` WHERE d.owner_user_id = ? ORDER BY d.created_at DESC`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]model.DNSRecord, 0)
	for rows.Next() {
		record, err := scanDNSRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) ListPublishableDNSRecords(ctx context.Context) ([]model.DNSRecord, error) {
	rows, err := s.db.QueryContext(ctx, dnsRecordSelect+`
WHERE r.enabled = 1 AND d.dns_label != '' AND (r.ipv4 != '' OR r.ipv6 != '')
ORDER BY d.dns_label
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]model.DNSRecord, 0)
	for rows.Next() {
		record, err := scanDNSRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) GetDNSRecord(ctx context.Context, deviceID string) (model.DNSRecord, bool, error) {
	record, err := scanDNSRecord(s.db.QueryRowContext(ctx, dnsRecordSelect+` WHERE r.device_id = ?`, deviceID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.DNSRecord{}, false, nil
	}
	return record, err == nil, err
}

func (s *Store) UpdateDNSRecordSettings(ctx context.Context, deviceID string, dnsLabel *string, ttl *int, enabled *bool) (model.DNSRecord, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.DNSRecord{}, false, err
	}
	defer tx.Rollback()
	var currentLabel string
	if err := tx.QueryRowContext(ctx, `SELECT dns_label FROM devices WHERE id = ?`, deviceID).Scan(&currentLabel); errors.Is(err, sql.ErrNoRows) {
		return model.DNSRecord{}, false, nil
	} else if err != nil {
		return model.DNSRecord{}, false, err
	}
	if dnsLabel != nil && *dnsLabel != currentLabel {
		var reserved bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM enrollment_grants WHERE dns_label = ? AND used_at IS NULL AND julianday(expires_at) > julianday(?))`, *dnsLabel, nowText()).Scan(&reserved); err != nil {
			return model.DNSRecord{}, false, err
		}
		if reserved {
			return model.DNSRecord{}, false, errors.New("dns label is reserved by an active enrollment grant")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE devices SET dns_label = ? WHERE id = ?`, *dnsLabel, deviceID); err != nil {
			return model.DNSRecord{}, false, err
		}
	}
	now := nowText()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO device_dns_records (device_id, created_at, updated_at) VALUES (?, ?, ?)`, deviceID, now, now); err != nil {
		return model.DNSRecord{}, false, err
	}
	if ttl != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE device_dns_records SET ttl = ?, updated_at = ? WHERE device_id = ?`, *ttl, now, deviceID); err != nil {
			return model.DNSRecord{}, false, err
		}
	}
	if enabled != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE device_dns_records SET enabled = ?, updated_at = ? WHERE device_id = ?`, *enabled, now, deviceID); err != nil {
			return model.DNSRecord{}, false, err
		}
	}
	if dnsLabel != nil && ttl == nil && enabled == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE device_dns_records SET updated_at = ? WHERE device_id = ?`, now, deviceID); err != nil {
			return model.DNSRecord{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.DNSRecord{}, false, err
	}
	return s.GetDNSRecord(ctx, deviceID)
}

func (s *Store) UpdateDNSAddresses(ctx context.Context, deviceID, ipv4, ipv6, source string) (model.DNSRecord, bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.DNSRecord{}, false, false, err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return model.DNSRecord{}, false, false, err
	}
	if !exists {
		return model.DNSRecord{}, false, false, nil
	}
	now := nowText()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO device_dns_records (device_id, created_at, updated_at) VALUES (?, ?, ?)`, deviceID, now, now); err != nil {
		return model.DNSRecord{}, false, false, err
	}
	var currentIPv4, currentIPv6 string
	if err := tx.QueryRowContext(ctx, `SELECT ipv4, ipv6 FROM device_dns_records WHERE device_id = ?`, deviceID).Scan(&currentIPv4, &currentIPv6); err != nil {
		return model.DNSRecord{}, false, false, err
	}
	changed := currentIPv4 != ipv4 || currentIPv6 != ipv6
	if _, err := tx.ExecContext(ctx, `
UPDATE device_dns_records
SET ipv4 = ?, ipv6 = ?, last_agent_update_at = ?, updated_at = ?
WHERE device_id = ?
`, ipv4, ipv6, now, now, deviceID); err != nil {
		return model.DNSRecord{}, false, false, err
	}
	if changed {
		historyID, err := randomID("dns")
		if err != nil {
			return model.DNSRecord{}, false, false, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO dns_address_history (id, device_id, ipv4, ipv6, source, created_at) VALUES (?, ?, ?, ?, ?, ?)`, historyID, deviceID, ipv4, ipv6, source, now); err != nil {
			return model.DNSRecord{}, false, false, err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM dns_address_history
WHERE device_id = ? AND id IN (
	SELECT id FROM dns_address_history WHERE device_id = ? ORDER BY created_at DESC LIMIT -1 OFFSET 100
)
`, deviceID, deviceID); err != nil {
			return model.DNSRecord{}, false, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.DNSRecord{}, false, false, err
	}
	record, found, err := s.GetDNSRecord(ctx, deviceID)
	return record, found, changed, err
}

func (s *Store) ListDNSAddressHistory(ctx context.Context, deviceID string, limit int) ([]model.DNSAddressHistory, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`, deviceID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, ipv4, ipv6, source, created_at
FROM dns_address_history
WHERE device_id = ?
ORDER BY created_at DESC
LIMIT ?
`, deviceID, limit)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	history := make([]model.DNSAddressHistory, 0)
	for rows.Next() {
		var item model.DNSAddressHistory
		var createdAt string
		if err := rows.Scan(&item.ID, &item.DeviceID, &item.IPv4, &item.IPv6, &item.Source, &createdAt); err != nil {
			return nil, false, err
		}
		item.CreatedAt = parseTime(createdAt)
		history = append(history, item)
	}
	return history, true, rows.Err()
}

func scanDNSRecord(s scanner) (model.DNSRecord, error) {
	var record model.DNSRecord
	var enabled bool
	var lastAgentUpdate sql.NullString
	var createdAt, updatedAt string
	if err := s.Scan(&record.DeviceID, &record.DNSLabel, &record.IPv4, &record.IPv6, &record.TTL, &enabled, &lastAgentUpdate, &createdAt, &updatedAt); err != nil {
		return record, err
	}
	record.Enabled = enabled
	record.CreatedAt = parseTime(createdAt)
	record.UpdatedAt = parseTime(updatedAt)
	if lastAgentUpdate.Valid {
		value := parseTime(lastAgentUpdate.String)
		record.LastAgentUpdateAt = &value
	}
	return record, nil
}
