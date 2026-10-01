package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// PermissionPolicy restricts existing ownership; it never grants foreign devices.
// An absent policy preserves the legacy operator's rights. Administrators retain
// management rights so a policy edit cannot lock the last administrator out.
type PermissionPolicy struct {
	Permissions []string `json:"permissions"`
	Groups      []string `json:"groups"`
	Sites       []string `json:"sites"`
	Configured  bool     `json:"configured"`
}

var Permissions = []string{"view", "diagnostics", "updates", "uci", "remote", "backups", "maintenance", "incidents"}

func CommandPermission(kind string) string {
	switch {
	case strings.HasPrefix(kind, "uci_"):
		return "uci"
	case strings.HasPrefix(kind, "remote_"):
		return "remote"
	case strings.HasPrefix(kind, "system_backup_"):
		return "backups"
	case kind == "agent_update" || kind == "agent_rollback" || strings.HasPrefix(kind, "pkg_") || strings.HasPrefix(kind, "opkg_"):
		if strings.Contains(kind, "list") {
			return "diagnostics"
		}
		return "updates"
	case slices.Contains([]string{"ping", "traceroute", "route_show", "interfaces_show", "diagnostic_report"}, kind):
		return "diagnostics"
	default:
		return "maintenance"
	}
}

func permissionPolicy(ctx context.Context, tx *sqliteTx, userID string) (PermissionPolicy, error) {
	var raw string
	err := tx.QueryRowContext(ctx, "SELECT policy_json FROM management_permissions WHERE user_id=?", userID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return PermissionPolicy{Permissions: slices.Clone(Permissions)}, nil
	}
	if err != nil {
		return PermissionPolicy{}, err
	}
	var policy PermissionPolicy
	if err = json.Unmarshal([]byte(raw), &policy); err != nil {
		return policy, err
	}
	if policy.Permissions == nil {
		policy.Permissions = []string{}
	}
	policy.Configured = true
	return policy, nil
}

func permissionAllowed(ctx context.Context, tx *sqliteTx, userID, deviceID, permission string, admin bool) error {
	if admin {
		return nil
	}
	p, err := permissionPolicy(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !slices.Contains(p.Permissions, "view") || !slices.Contains(p.Permissions, permission) {
		return ErrFleetAccess
	}
	if deviceID == "" {
		return nil
	}
	var owner, group, site string
	if err = tx.QueryRowContext(ctx, "SELECT d.owner_user_id,d.group_name,COALESCE(a.site,'') FROM devices d LEFT JOIN fleet_assets a ON a.device_id=d.id WHERE d.id=?", deviceID).Scan(&owner, &group, &site); errors.Is(err, sql.ErrNoRows) {
		return ErrFleetAccess
	} else if err != nil {
		return err
	}
	if owner != userID || (len(p.Groups) > 0 && !slices.Contains(p.Groups, group)) || (len(p.Sites) > 0 && !slices.Contains(p.Sites, site)) {
		return ErrFleetAccess
	}
	return nil
}

func (s *Store) CheckPermission(ctx context.Context, userID, deviceID, permission string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	return permissionAllowed(ctx, tx, userID, deviceID, permission, admin)
}

func (s *Store) GetPermissionPolicy(ctx context.Context, userID string) (PermissionPolicy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PermissionPolicy{}, err
	}
	defer tx.Rollback()
	return permissionPolicy(ctx, tx, userID)
}

func (s *Store) SavePermissionPolicy(ctx context.Context, adminID, userID string, p PermissionPolicy) error {
	if len(p.Permissions) > len(Permissions) || len(p.Groups) > 100 || len(p.Sites) > 100 {
		return errors.New("invalid permission limits")
	}
	for _, v := range p.Permissions {
		if !slices.Contains(Permissions, v) {
			return errors.New("unknown permission")
		}
	}
	for _, v := range append(slices.Clone(p.Groups), p.Sites...) {
		if strings.TrimSpace(v) == "" || len(v) > 255 {
			return errors.New("invalid scope")
		}
	}
	if !slices.Contains(p.Permissions, "view") && len(p.Permissions) > 0 {
		return errors.New("view is required")
	}
	if p.Permissions == nil {
		p.Permissions = []string{}
	}
	p.Configured = true
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return err
	}
	admin, err := fleetPrincipal(ctx, tx, adminID)
	if err != nil {
		return err
	}
	if !admin {
		return ErrFleetAccess
	}
	if err = tunnelPolicyLock(ctx, tx); err != nil {
		return err
	}
	targetAdmin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	if targetAdmin {
		return errors.New("administrator permissions cannot be restricted")
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO management_permissions (user_id,policy_json,updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET policy_json=excluded.policy_json,updated_at=excluded.updated_at", userID, string(raw), nowText()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE remote_sessions SET status='closed',closed_at=?,updated_at=? WHERE status IN ('requested','queued','active') AND (command_id IN (SELECT command_id FROM management_command_permissions WHERE user_id=? AND permission='remote') OR command_id IN (SELECT a.command_id FROM audit_events a JOIN users u ON u.username=a.actor WHERE u.id=? AND a.action='remote_session.create') OR id IN (SELECT remote_session_id FROM device_access_sessions WHERE user_id=?) OR id IN (SELECT remote_session_id FROM device_access_grants WHERE user_id=?))", nowText(), nowText(), userID, userID, userID, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE commands SET status='cancelled',cancelled_at=? WHERE status='queued' AND id IN (SELECT command_id FROM remote_sessions WHERE status='closed') AND type='remote_ssh_reverse'", nowText()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM device_access_sessions WHERE user_id=?", userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM device_access_grants WHERE user_id=?", userID); err != nil {
		return err
	}
	id, err := randomID("audit")
	if err != nil {
		return err
	}
	auditDetails, err := json.Marshal(map[string]any{"user_id": userID, "policy": p})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events (id,actor,action,details_json,created_at) VALUES (?,?,'permissions.update',?,?)", id, adminID, string(auditDetails), nowText()); err != nil {
		return err
	}
	return tx.Commit()
}
