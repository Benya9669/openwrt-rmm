package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

// This suite uses a third, empty database so it cannot interfere with import
// fixtures or the schema-checksum rejection test.
func TestPostgresCriticalFlowsIntegration(t *testing.T) {
	dsn := os.Getenv("RMM_TEST_POSTGRES_CRITICAL_URL")
	if dsn == "" {
		t.Skip("set RMM_TEST_POSTGRES_CRITICAL_URL to an empty local test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Query().Get("sslmode") != "disable" {
		t.Fatal("requires a local test database with sslmode=disable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("critical-flow database must be empty; refusing to modify it")
	}
	if err := EnsurePostgres(ctx, db); err != nil {
		t.Fatal(err)
	}
	a, err := store.OpenPostgres(ctx, dsn, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := store.OpenPostgres(ctx, dsn, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	user, err := a.EnsureBootstrapUser(ctx, "Admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := a.EnrollDevice(ctx, "router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, found, err := b.GetUserByUsername(ctx, "ADMIN"); err != nil || !found {
		t.Fatalf("case-insensitive lookup: %v %v", found, err)
	}

	// Race independent connection pools, exactly as separate servers do.
	race := func(t *testing.T, operation func(*store.Store, int) (bool, error)) {
		t.Helper()
		start := make(chan struct{})
		results := make(chan error, 8)
		var mu sync.Mutex
		successes := 0
		for i := range 8 {
			go func() {
				<-start
				st := a
				if i%2 == 1 {
					st = b
				}
				won, err := operation(st, i)
				if won {
					mu.Lock()
					successes++
					mu.Unlock()
				}
				results <- err
			}()
		}
		close(start)
		for range 8 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if successes != 1 {
			t.Fatalf("expected exactly one winner, got %d", successes)
		}
	}
	t.Run("signed-command-single-claim", func(t *testing.T) {
		created, found, err := a.CreateCommand(ctx, device.DeviceID, "system.info", []byte(`{}`))
		if err != nil || !found {
			t.Fatalf("create: %v %v", found, err)
		}
		race(t, func(st *store.Store, _ int) (bool, error) {
			claimed, found, err := st.ClaimNextCommand(ctx, device.DeviceID)
			if err != nil || !found {
				return found, err
			}
			if claimed.ID != created.ID || claimed.ExpiresAt == nil {
				return true, fmt.Errorf("wrong claimed command")
			}
			return true, commandsig.Verify(a.CommandSigningPublicKey(), commandsig.Envelope{ID: claimed.ID, DeviceID: claimed.DeviceID, Type: claimed.Type, Args: claimed.Args, CreatedAt: claimed.CreatedAt, ExpiresAt: *claimed.ExpiresAt, Nonce: claimed.Nonce}, claimed.Signature)
		})
		if found, err := b.SaveCommandResult(ctx, created.ID, device.DeviceID, "completed", 0, "ok", []byte(`{}`)); err != nil || !found {
			t.Fatalf("result: %v %v", found, err)
		}
	})
	t.Run("heartbeat-and-null-client-timestamp", func(t *testing.T) {
		inventory := []byte(`{"dhcp_leases":[{"mac":"02:00:00:00:00:01","ip":"192.0.2.1"},{"mac":"02:00:00:00:00:02","ip":"192.0.2.2"}],"client_probes":[{"mac":"02:00:00:00:00:02","ip":"192.0.2.2","reachable":"true"}]}`)
		if _, err := a.SaveHeartbeat(ctx, device.DeviceID, inventory, []byte(`{"cpu":15}`)); err != nil {
			t.Fatal(err)
		}
		clients, found, err := b.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
		if err != nil || !found || len(clients) != 2 {
			t.Fatalf("LAN list: %v %d %v", found, len(clients), err)
		}
		if clients[0].IP != "192.0.2.2" || clients[1].LastSeenAt != nil {
			t.Fatalf("NULL last_seen ordering: %+v", clients)
		}
	})
	t.Run("notification-lease-and-recovery", func(t *testing.T) {
		delivery, found, err := a.CreateNotificationDelivery(ctx, model.NotificationDelivery{UserID: user.ID, Event: "test", Channel: "email", Title: "test", Body: "body", Destination: "test@example.test", MaxAttempts: 3}, "critical-delivery")
		if err != nil || !found {
			t.Fatalf("create delivery: %v %v", found, err)
		}
		now := time.Now().UTC().Add(time.Second)
		race(t, func(st *store.Store, _ int) (bool, error) {
			_, found, err := st.ClaimNotificationDelivery(ctx, delivery.ID, now, time.Minute)
			return found, err
		})
		reclaimed, found, err := b.ClaimNotificationDelivery(ctx, delivery.ID, now.Add(2*time.Minute), time.Minute)
		if err != nil || !found || reclaimed.AttemptCount != 2 {
			t.Fatalf("lease recovery: %+v %v %v", reclaimed, found, err)
		}
		if err := b.CompleteNotificationDelivery(ctx, delivery.ID, "sent", "", nil); err != nil {
			t.Fatal(err)
		}
		if _, found, err := a.ClaimNotificationDelivery(ctx, delivery.ID, now.Add(5*time.Minute), time.Minute); err != nil || found {
			t.Fatalf("sent delivery reclaimed: %v %v", found, err)
		}
	})
	t.Run("one-time-enrollment", func(t *testing.T) {
		if _, err := a.CreateEnrollmentGrant(ctx, user.ID, "grant-router", "critical-grant", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		race(t, func(st *store.Store, _ int) (bool, error) {
			_, found, err := st.EnrollDeviceWithGrant(ctx, "critical-grant", "new-router", "24.10")
			return found, err
		})
	})
	t.Run("metrics", func(t *testing.T) {
		m, err := a.OperationalMetrics(ctx)
		if err != nil || m.Backend != "postgres" || m.Commands["completed"] != 1 || m.Notifications["sent"] != 1 {
			t.Fatalf("metrics: %+v %v", m, err)
		}
	})
	t.Run("one-time-LuCI-grant", func(t *testing.T) {
		deviceInfo, found, err := a.GetDevice(ctx, device.DeviceID)
		if err != nil || !found {
			t.Fatalf("device: %v %v", found, err)
		}
		session, found, err := a.CreateRemoteSession(ctx, model.RemoteSession{DeviceID: device.DeviceID, Status: "active", RemotePort: 22001, LuCIPort: 22101, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil || !found {
			t.Fatalf("session: %v %v", found, err)
		}
		if err := a.CreateDeviceAccessGrant(ctx, "critical-luci-grant", user.ID, device.DeviceID, session.ID, time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		race(t, func(st *store.Store, index int) (bool, error) {
			_, found, err := st.ConsumeDeviceAccessGrant(ctx, "critical-luci-grant", fmt.Sprintf("critical-session-%d", index), deviceInfo.DNSLabel, time.Now().Add(time.Hour))
			return found, err
		})
	})
}
