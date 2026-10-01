package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"rmm-openwrt/server/internal/model"
	"slices"
	"time"
)

func (s *Store) CreateManagedRemoteCommand(ctx context.Context, userID, deviceID, sessionID string, args json.RawMessage) (model.Command, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Command{}, false, err
	}
	defer tx.Rollback()
	if err = tunnelPolicyLock(ctx, tx); err != nil {
		return model.Command{}, false, err
	}
	var remote, luci int
	var expires, status string
	err = tx.QueryRowContext(ctx, "SELECT remote_port,luci_port,expires_at,status FROM remote_sessions WHERE id=? AND device_id=?", sessionID, deviceID).Scan(&remote, &luci, &expires, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Command{}, false, nil
	}
	if err != nil {
		return model.Command{}, false, err
	}
	deadline, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return model.Command{}, false, err
	}
	if status != "requested" || !deadline.After(time.Now()) {
		return model.Command{}, false, ErrFleetAccess
	}
	if err = enforceFleetAccess(ctx, tx, userID, deviceID, remote > 0, luci > 0, deadline); err != nil {
		return model.Command{}, false, err
	}
	command, err := s.newCommand(deviceID, "remote_ssh_reverse", args, time.Until(deadline))
	if err != nil {
		return command, false, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO commands (id,device_id,type,args_json,status,max_attempts,created_at,expires_at,nonce,signature_key_id,signature) VALUES (?,?,?,?,'queued',3,?,?,?,?,?)", command.ID, deviceID, command.Type, string(command.Args), command.CreatedAt.Format(time.RFC3339Nano), command.ExpiresAt.Format(time.RFC3339Nano), command.Nonce, command.SignatureKeyID, command.Signature); err != nil {
		return command, false, err
	}
	if userID != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO management_command_permissions (command_id,user_id,permission) VALUES (?,?,'remote')", command.ID, userID); err != nil {
			return command, false, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE remote_sessions SET status='queued',command_id=?,updated_at=? WHERE id=?", command.ID, nowText(), sessionID); err != nil {
		return command, false, err
	}
	return command, true, tx.Commit()
}

type FleetAccessPolicy struct {
	DeviceID      string   `json:"device_id"`
	SSHAllowed    bool     `json:"ssh_allowed"`
	LuCIAllowed   bool     `json:"luci_allowed"`
	MaxTTLSeconds int      `json:"max_ttl_seconds"`
	RestrictUsers bool     `json:"restrict_users"`
	UserIDs       []string `json:"user_ids"`
	UpdatedAt     string   `json:"updated_at"`
}

func loadFleetAccessPolicy(ctx context.Context, tx *sqliteTx, deviceID string) (FleetAccessPolicy, bool, error) {
	policy := FleetAccessPolicy{DeviceID: deviceID, SSHAllowed: true, LuCIAllowed: true, MaxTTLSeconds: 7200, UserIDs: []string{}}
	var ssh, luci, restricted int
	var users string
	err := tx.QueryRowContext(ctx, "SELECT ssh_allowed,luci_allowed,max_ttl_seconds,restrict_users,user_ids_json,updated_at FROM fleet_access_policies WHERE device_id=?", deviceID).Scan(&ssh, &luci, &policy.MaxTTLSeconds, &restricted, &users, &policy.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, false, nil
	}
	if err != nil {
		return policy, false, err
	}
	policy.SSHAllowed = ssh != 0
	policy.LuCIAllowed = luci != 0
	policy.RestrictUsers = restricted != 0
	err = json.Unmarshal([]byte(users), &policy.UserIDs)
	return policy, true, err
}

func tunnelPolicyLock(ctx context.Context, tx *sqliteTx) error {
	if tx.postgres {
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(7766442212)")
		return err
	}
	return nil
}

func enforceFleetAccess(ctx context.Context, tx *sqliteTx, userID, deviceID string, ssh, luci bool, expires time.Time) error {
	if userID != "" {
		admin, err := fleetPrincipal(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err = permissionAllowed(ctx, tx, userID, deviceID, "remote", admin); err != nil {
			return err
		}
	}
	policy, configured, err := loadFleetAccessPolicy(ctx, tx, deviceID)
	if err != nil || !configured {
		return err
	}
	if userID == "" {
		return ErrFleetAccess
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err = fleetDeviceAccess(ctx, tx, userID, deviceID, admin); err != nil {
		return err
	}
	if (ssh && !policy.SSHAllowed) || (luci && !policy.LuCIAllowed) || (policy.RestrictUsers && !slices.Contains(policy.UserIDs, userID)) || expires.After(time.Now().Add(time.Duration(policy.MaxTTLSeconds)*time.Second+time.Second)) {
		return ErrFleetAccess
	}
	return nil
}

func (s *Store) GetFleetAccessPolicy(ctx context.Context, userID, deviceID string) (FleetAccessPolicy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FleetAccessPolicy{}, err
	}
	defer tx.Rollback()
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return FleetAccessPolicy{}, err
	}
	if err = fleetDeviceAccess(ctx, tx, userID, deviceID, admin); err != nil {
		return FleetAccessPolicy{}, err
	}
	policy, _, err := loadFleetAccessPolicy(ctx, tx, deviceID)
	return policy, err
}

// A policy change revokes all prior access. The shared allocation lock ensures a
// concurrently created session sees either the old policy or the new policy.
func (s *Store) SaveFleetAccessPolicy(ctx context.Context, userID string, policy FleetAccessPolicy) error {
	if policy.MaxTTLSeconds < 60 || policy.MaxTTLSeconds > 7200 || len(policy.UserIDs) > 100 {
		return errors.New("invalid access policy")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = tunnelPolicyLock(ctx, tx); err != nil {
		return err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !admin {
		return ErrFleetAccess
	}
	if err = fleetDeviceAccess(ctx, tx, userID, policy.DeviceID, true); err != nil {
		return err
	}
	for _, id := range policy.UserIDs {
		var enabled bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=? AND disabled=0)", id).Scan(&enabled); err != nil {
			return err
		}
		if !enabled {
			return errors.New("policy user unavailable")
		}
	}
	ids := slices.Clone(policy.UserIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids == nil {
		ids = []string{}
	}
	users, _ := json.Marshal(ids)
	if _, err = tx.ExecContext(ctx, `INSERT INTO fleet_access_policies (device_id,ssh_allowed,luci_allowed,max_ttl_seconds,restrict_users,user_ids_json,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET ssh_allowed=excluded.ssh_allowed,luci_allowed=excluded.luci_allowed,max_ttl_seconds=excluded.max_ttl_seconds,restrict_users=excluded.restrict_users,user_ids_json=excluded.user_ids_json,updated_at=excluded.updated_at`, policy.DeviceID, policy.SSHAllowed, policy.LuCIAllowed, policy.MaxTTLSeconds, policy.RestrictUsers, string(users), nowText()); err != nil {
		return err
	}
	for _, query := range []string{
		"UPDATE commands SET status='cancelled' WHERE type='remote_ssh_reverse' AND status='queued' AND device_id=?",
		"DELETE FROM device_access_grants WHERE device_id=?",
		"DELETE FROM device_access_sessions WHERE device_id=?",
		"UPDATE remote_sessions SET status='closed',closed_at=?,updated_at=? WHERE device_id=? AND status IN ('requested','queued','active')",
	} {
		args := []any{policy.DeviceID}
		if query[0:22] == "UPDATE remote_sessions" {
			args = []any{nowText(), nowText(), policy.DeviceID}
		}
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	eventID, err := randomID("policy-audit")
	if err != nil {
		return err
	}
	details, _ := json.Marshal(policy)
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events (id,actor,action,device_id,details_json,created_at) VALUES (?,?,'remote_access.policy_update',?,?,?)", eventID, userID, policy.DeviceID, string(details), nowText()); err != nil {
		return err
	}
	return tx.Commit()
}
