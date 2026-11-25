package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestConfirmedRecoveryEmailInvalidatesOldLinksAndQueuesNotice(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	user, err := st.CreateUser(ctx, "owner", "", "old@example.test", "password-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Minute)
	if ok, err := st.CreatePasswordReset(ctx, user.ID, user.Email, "old-link", expiry); err != nil || !ok {
		t.Fatalf("create reset: %v %v", ok, err)
	}
	if err := st.BeginRecoveryEmailChange(ctx, user.ID, "password-hash", user.Email, "new@example.test", "code-1", expiry); err != nil {
		t.Fatal(err)
	}
	if err := st.BeginRecoveryEmailChange(ctx, user.ID, "password-hash", user.Email, "new@example.test", "code-2", expiry); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.ConfirmRecoveryEmailChange(ctx, user.ID, "code-1"); err != nil || ok {
		t.Fatalf("replaced code: %v %v", ok, err)
	}
	if ok, err := st.PasswordResetValid(ctx, "old-link"); err != nil || !ok {
		t.Fatalf("old address before confirmation: %v %v", ok, err)
	}
	updated, ok, err := st.ConfirmRecoveryEmailChange(ctx, user.ID, "code-2")
	if err != nil || !ok || updated.Email != "new@example.test" {
		t.Fatalf("confirm: %v %v", ok, err)
	}
	if ok, err := st.ResetPassword(ctx, "old-link", "unwanted-hash"); err != nil || ok {
		t.Fatalf("old reset consumed: %v %v", ok, err)
	}
	if ok, err := st.CreatePasswordReset(ctx, user.ID, user.Email, "stale-request", expiry); err != nil || ok {
		t.Fatalf("stale address used to issue a link: %v %v", ok, err)
	}
	notices, err := st.ClaimNotificationDeliveries(ctx, time.Now().Add(time.Second), time.Minute, 10)
	if err != nil || len(notices) != 1 || notices[0].Destination != user.Email || notices[0].Event != "recovery_email_changed" {
		t.Fatalf("missing previous-address notice: %v", err)
	}
	if err := st.BeginRecoveryEmailChange(ctx, user.ID, "password-hash", updated.Email, "", "remove-code", expiry); err != nil {
		t.Fatal(err)
	}
	updated, ok, err = st.ConfirmRecoveryEmailChange(ctx, user.ID, "remove-code")
	if err != nil || !ok || updated.Email != "" {
		t.Fatalf("remove confirmed email: %v %v", ok, err)
	}
}

func TestRevokedCredentialsCannotMintNewSessions(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "revoke.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	user, err := st.CreateUser(ctx, "owner", "", "owner@example.test", "previous", "user")
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Minute)
	if err = st.CreateOperatorSession(ctx, "session", user.ID, "previous", expiry); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.CreatePasswordReset(ctx, user.ID, user.Email, "token", expiry); err != nil || !ok {
		t.Fatalf("issue reset: %v %v", ok, err)
	}
	if ok, err := st.ResetPassword(ctx, "token", "current"); err != nil || !ok {
		t.Fatalf("consume reset: %v %v", ok, err)
	}
	if err := st.CreateOperatorSession(ctx, "late-login", user.ID, "previous", expiry); !errors.Is(err, ErrRecoveryAccountChanged) {
		t.Fatalf("stale password check accepted: %v", err)
	}
	if err := st.UpdateOwnPassword(ctx, user.ID, "previous", "late-change", "session"); !errors.Is(err, ErrRecoveryAccountChanged) {
		t.Fatalf("stale password change accepted: %v", err)
	}
	if err := st.CreateDeviceAccessGrant(ctx, "late-grant", user.ID, "session", "device", "tunnel", expiry); !errors.Is(err, ErrRecoveryAccountChanged) {
		t.Fatalf("revoked browser session minted grant: %v", err)
	}
}
