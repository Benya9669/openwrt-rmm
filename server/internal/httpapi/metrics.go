package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"rmm-openwrt/server/internal/store"
)

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	source, ok := a.store.(interface {
		OperationalMetrics(context.Context) (store.OperationalMetrics, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "metrics unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	m, err := source.OperationalMetrics(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "metrics unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	gauge := func(name, help string, value any) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, help, name, name, value)
	}
	fmt.Fprintf(w, "# HELP rmm_database_info Selected database backend.\n# TYPE rmm_database_info gauge\nrmm_database_info{backend=%q} 1\n", m.Backend)
	gauge("rmm_database_connections_open", "Open database connections.", m.Pool.OpenConnections)
	gauge("rmm_database_connections_idle", "Idle database connections.", m.Pool.Idle)
	gauge("rmm_database_connections_in_use", "Database connections in use.", m.Pool.InUse)
	fmt.Fprintf(w, "# HELP rmm_database_wait_total Connection pool waits.\n# TYPE rmm_database_wait_total counter\nrmm_database_wait_total %d\n", m.Pool.WaitCount)
	fmt.Fprintf(w, "# HELP rmm_database_wait_seconds_total Time spent waiting for a connection.\n# TYPE rmm_database_wait_seconds_total counter\nrmm_database_wait_seconds_total %g\n", m.Pool.WaitDuration.Seconds())
	for _, family := range []struct {
		name     string
		counts   map[string]int64
		statuses []string
	}{
		{"rmm_commands", m.Commands, []string{"queued", "claimed", "completed", "failed", "cancelled", "expired"}},
		{"rmm_notification_deliveries", m.Notifications, []string{"queued", "retry", "sending", "sent", "failed", "dead_letter"}},
	} {
		fmt.Fprintf(w, "# HELP %s Records by status.\n# TYPE %s gauge\n", family.name, family.name)
		for _, status := range family.statuses {
			fmt.Fprintf(w, "%s{status=%q} %d\n", family.name, status, family.counts[status])
		}
	}
	gauge("rmm_notification_queue_oldest_age_seconds", "Age of the oldest unfinished delivery.", m.QueueAgeSeconds)
	gauge("rmm_tunnel_sessions", "Unexpired requested, queued or active tunnel sessions.", m.Tunnels)
}
