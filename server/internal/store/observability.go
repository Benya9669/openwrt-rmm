package store

import (
	"context"
	"database/sql"
	"time"
)

// OperationalMetrics contains only aggregate counts, never destinations or IDs.
type OperationalMetrics struct {
	Backend         string
	Pool            sql.DBStats
	Commands        map[string]int64
	Notifications   map[string]int64
	Tunnels         int64
	QueueAgeSeconds float64
}

func (s *Store) OperationalMetrics(ctx context.Context) (OperationalMetrics, error) {
	m := OperationalMetrics{Backend: "sqlite", Pool: s.db.db.Stats(), Commands: map[string]int64{}, Notifications: map[string]int64{}}
	if s.db.postgres {
		m.Backend = "postgres"
	}
	for _, table := range []string{"commands", "notification_deliveries"} {
		rows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM "+table+" GROUP BY status")
		if err != nil {
			return m, err
		}
		counts := m.Commands
		if table == "notification_deliveries" {
			counts = m.Notifications
		}
		for rows.Next() {
			var status string
			var count int64
			if err := rows.Scan(&status, &count); err != nil {
				rows.Close()
				return m, err
			}
			counts[status] = count
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return m, err
		}
	}
	now := time.Now().UTC()
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM remote_sessions WHERE status IN ('requested', 'queued', 'active') AND julianday(expires_at) > julianday(?)`, notificationTimeText(now)).Scan(&m.Tunnels); err != nil {
		return m, err
	}
	var oldest sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(created_at) FROM notification_deliveries WHERE status IN ('queued', 'retry', 'sending')`).Scan(&oldest); err != nil {
		return m, err
	}
	if oldest.Valid {
		timestamp, err := time.Parse(time.RFC3339Nano, oldest.String)
		if err != nil {
			return m, err
		}
		m.QueueAgeSeconds = max(0, now.Sub(timestamp).Seconds())
	}
	return m, nil
}
