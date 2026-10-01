package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
)

var ErrFleetAccess = errors.New("fleet object is unavailable")
var ErrFleetConflict = errors.New("request key already describes another operation")
var ErrFleetFeatureUnavailable = errors.New("agent does not advertise the requested fleet feature")

type FleetOperation struct {
	ID            string               `json:"id"`
	UserID        string               `json:"user_id"`
	RequestKey    string               `json:"request_key"`
	Title         string               `json:"title"`
	Type          string               `json:"type"`
	Args          json.RawMessage      `json:"args"`
	Parallelism   int                  `json:"parallelism"`
	CanaryPending bool                 `json:"canary_pending"`
	DeadlineAt    string               `json:"deadline_at,omitempty"`
	StopOnFailure bool                 `json:"stop_on_failure"`
	Status        string               `json:"status"`
	CreatedAt     string               `json:"created_at"`
	UpdatedAt     string               `json:"updated_at"`
	Items         []FleetOperationItem `json:"items"`
}

type FleetOperationItem struct {
	DeviceID  string          `json:"device_id"`
	CommandID string          `json:"command_id"`
	Status    string          `json:"status"`
	Error     string          `json:"error,omitempty"`
	Args      json.RawMessage `json:"-"`
}

type FleetOperationInput struct {
	DeviceArgs    map[string]json.RawMessage `json:"-"`
	Canary        bool                       `json:"canary"`
	ScheduleRunID string                     `json:"-"`
	RequestKey    string                     `json:"request_key"`
	Title         string                     `json:"title"`
	DeviceIDs     []string                   `json:"device_ids"`
	Type          string                     `json:"type"`
	Args          json.RawMessage            `json:"args"`
	Parallelism   int                        `json:"parallelism"`
	DeadlineAt    string                     `json:"deadline_at,omitempty"`
	StopOnFailure bool                       `json:"stop_on_failure"`
}

func fleetLock(ctx context.Context, tx *sqliteTx) error {
	if tx.postgres {
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(7766442213)")
		return err
	}
	return nil
}

func fleetPrincipal(ctx context.Context, tx *sqliteTx, userID string) (bool, error) {
	var role string
	var disabled int
	err := tx.QueryRowContext(ctx, "SELECT role, disabled FROM users WHERE id=?", userID).Scan(&role, &disabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err != nil || disabled != 0 {
		return false, ErrFleetAccess
	}
	return role == "admin", nil
}

func fleetDeviceAccess(ctx context.Context, tx *sqliteTx, userID, deviceID string, admin bool) error {
	var allowed bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id=? AND (owner_user_id=? OR ?=1))", deviceID, userID, admin).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrFleetAccess
	}
	return permissionAllowed(ctx, tx, userID, deviceID, "view", admin)
}

func (s *Store) CreateFleetOperation(ctx context.Context, userID string, input FleetOperationInput) (FleetOperation, error) {
	if !FleetCommandAllowed(input.Type) {
		return FleetOperation{}, errors.New("command is not available for fleet operations")
	}
	if input.Parallelism < 1 || input.Parallelism > 20 || len(input.DeviceIDs) == 0 || len(input.DeviceIDs) > 100 || len(input.Title) > 160 || len(input.RequestKey) < 8 || len(input.RequestKey) > 128 || len(input.Args) > fleetArgsLimit(input.Type) {
		return FleetOperation{}, errors.New("invalid operation limits")
	}
	ids := slices.Clone(input.DeviceIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	args := NormalizeRawJSON(input.Args)
	if !json.Valid(args) {
		return FleetOperation{}, errors.New("invalid operation arguments")
	}
	if input.Type == "agent_update" {
		var err error
		input.DeviceArgs, err = fleetUpdateDeviceArgs(args, ids)
		if err != nil {
			return FleetOperation{}, err
		}
	}
	deviceArgs := map[string]json.RawMessage{}
	for _, id := range ids {
		deviceArgs[id] = args
		if input.DeviceArgs[id] != nil {
			deviceArgs[id] = input.DeviceArgs[id]
		}
	}
	payload, err := json.Marshal(struct {
		Title, Type, Deadline string
		Parallelism           int
		Stop, Canary          bool
		Args                  map[string]json.RawMessage
	}{input.Title, input.Type, input.DeadlineAt, input.Parallelism, input.StopOnFailure, input.Canary, deviceArgs})
	if err != nil {
		return FleetOperation{}, err
	}
	digest := sha256.Sum256(payload)
	requestHash := hex.EncodeToString(digest[:])
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FleetOperation{}, err
	}
	defer tx.Rollback()
	if err := fleetLock(ctx, tx); err != nil {
		return FleetOperation{}, err
	}
	if input.ScheduleRunID != "" {
		var valid bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM fleet_schedule_runs r JOIN fleet_schedules s ON s.id=r.schedule_id WHERE r.id=? AND r.status='pending' AND s.enabled=1 AND s.user_id=?)", input.ScheduleRunID, userID).Scan(&valid); err != nil {
			return FleetOperation{}, err
		}
		if !valid {
			return FleetOperation{}, ErrFleetAccess
		}
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return FleetOperation{}, err
	}
	// Authorize every target before inserting the operation or any command.
	for _, id := range ids {
		if err := fleetDeviceAccess(ctx, tx, userID, id, admin); err != nil {
			return FleetOperation{}, err
		}
		if err := permissionAllowed(ctx, tx, userID, id, CommandPermission(input.Type), admin); err != nil {
			return FleetOperation{}, err
		}
		if input.Type == "diagnostic_report" || strings.HasPrefix(input.Type, "uci_profile_") {
			var inventory []byte
			if err := tx.QueryRowContext(ctx, "SELECT inventory_json FROM devices WHERE id=?", id).Scan(&inventory); err != nil {
				return FleetOperation{}, err
			}
			var features struct {
				Features []string `json:"rmm_features"`
			}
			if json.Unmarshal(inventory, &features) != nil || !slices.Contains(features.Features, input.Type) {
				return FleetOperation{}, ErrFleetFeatureUnavailable
			}
		}
	}
	var existingID, existingHash string
	err = tx.QueryRowContext(ctx, "SELECT id,request_hash FROM fleet_operations WHERE user_id=? AND request_key=?", userID, input.RequestKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != requestHash {
			return FleetOperation{}, ErrFleetConflict
		}
		existing, err := loadFleetOperation(ctx, tx, existingID)
		if err != nil {
			return FleetOperation{}, err
		}
		var existingIDs []string
		for _, item := range existing.Items {
			existingIDs = append(existingIDs, item.DeviceID)
		}
		if (input.DeadlineAt != "" && existing.DeadlineAt != input.DeadlineAt) || existing.Title != input.Title || existing.Type != input.Type || string(existing.Args) != string(args) || existing.Parallelism != input.Parallelism || existing.StopOnFailure != input.StopOnFailure || !slices.Equal(ids, existingIDs) {
			return FleetOperation{}, ErrFleetConflict
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FleetOperation{}, err
	}
	var active int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fleet_operations WHERE user_id=? AND status IN ('running','paused','cancelling')", userID).Scan(&active); err != nil {
		return FleetOperation{}, err
	}
	if active >= 20 {
		return FleetOperation{}, errors.New("too many active fleet operations")
	}
	id, err := randomID("operation")
	if err != nil {
		return FleetOperation{}, err
	}
	deadline := time.Now().UTC().Add(24 * time.Hour)
	if input.DeadlineAt != "" {
		deadline, err = time.Parse(time.RFC3339Nano, input.DeadlineAt)
		if err != nil || !deadline.After(time.Now()) || deadline.After(time.Now().Add(24*time.Hour)) {
			return FleetOperation{}, errors.New("invalid operation deadline")
		}
	}
	now := nowText()
	if _, err := tx.ExecContext(ctx, "INSERT INTO fleet_operations (id,user_id,request_key,request_hash,title,command_type,args_json,parallelism,stop_on_failure,canary_pending,deadline_at,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,'running',?,?)", id, userID, input.RequestKey, requestHash, input.Title, input.Type, string(args), input.Parallelism, input.StopOnFailure, input.Canary, deadline.UTC().Format(time.RFC3339Nano), now, now); err != nil {
		return FleetOperation{}, err
	}
	for _, deviceID := range ids {
		itemArgs := args
		if input.DeviceArgs[deviceID] != nil {
			itemArgs = input.DeviceArgs[deviceID]
		}
		if !json.Valid(itemArgs) || len(itemArgs) > 16384 {
			return FleetOperation{}, errors.New("invalid device arguments")
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO fleet_operation_items (operation_id,device_id,status,args_json) VALUES (?,?,'pending',?)", id, deviceID, string(itemArgs)); err != nil {
			return FleetOperation{}, err
		}
	}
	operation, err := loadFleetOperation(ctx, tx, id)
	if err != nil {
		return FleetOperation{}, err
	}
	return operation, tx.Commit()
}

func loadFleetOperation(ctx context.Context, tx *sqliteTx, id string) (FleetOperation, error) {
	var op FleetOperation
	var stop, canary int
	var args string
	err := tx.QueryRowContext(ctx, "SELECT id,user_id,request_key,title,command_type,args_json,parallelism,stop_on_failure,canary_pending,deadline_at,status,created_at,updated_at FROM fleet_operations WHERE id=?", id).Scan(&op.ID, &op.UserID, &op.RequestKey, &op.Title, &op.Type, &args, &op.Parallelism, &stop, &canary, &op.DeadlineAt, &op.Status, &op.CreatedAt, &op.UpdatedAt)
	if err != nil {
		return op, err
	}
	op.Args = json.RawMessage(args)
	op.StopOnFailure = stop != 0
	op.CanaryPending = canary != 0
	op.Items = []FleetOperationItem{}
	rows, err := tx.QueryContext(ctx, "SELECT device_id,command_id,status,error,args_json FROM fleet_operation_items WHERE operation_id=? ORDER BY device_id", id)
	if err != nil {
		return op, err
	}
	defer rows.Close()
	for rows.Next() {
		var item FleetOperationItem
		var args string
		if err := rows.Scan(&item.DeviceID, &item.CommandID, &item.Status, &item.Error, &args); err != nil {
			return op, err
		}
		item.Args = json.RawMessage(args)
		op.Items = append(op.Items, item)
	}
	return op, rows.Err()
}

func (s *Store) ListFleetOperations(ctx context.Context, userID string) ([]FleetOperation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := fleetPrincipal(ctx, tx, userID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM fleet_operations WHERE user_id=? ORDER BY created_at DESC LIMIT 100", userID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	operations := []FleetOperation{}
	for _, id := range ids {
		op, err := loadFleetOperation(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		admin, err := fleetPrincipal(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if err := permissionAllowed(ctx, tx, userID, "", CommandPermission(op.Type), admin); errors.Is(err, ErrFleetAccess) {
			op.Args = json.RawMessage(`{}`)
		} else if err != nil {
			return nil, err
		}
		visible := op.Items[:0]
		for _, item := range op.Items {
			if err := permissionAllowed(ctx, tx, userID, item.DeviceID, "view", admin); errors.Is(err, ErrFleetAccess) {
				continue
			} else if err != nil {
				return nil, err
			}
			visible = append(visible, item)
		}
		op.Items = visible
		operations = append(operations, op)
	}
	return operations, nil
}

// Cancel stops pending work and unclaimed commands. Claimed commands remain
// active until the agent reports a result; cancellation is not an execution kill.
func (s *Store) SetFleetOperationAction(ctx context.Context, userID, id, action string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fleetLock(ctx, tx); err != nil {
		return err
	}
	if _, err := fleetPrincipal(ctx, tx, userID); err != nil {
		return err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	var kind string
	if err = tx.QueryRowContext(ctx, "SELECT command_type FROM fleet_operations WHERE id=? AND user_id=?", id, userID).Scan(&kind); err != nil {
		return ErrFleetAccess
	}
	if action == "resume" {
		if err = permissionAllowed(ctx, tx, userID, "", CommandPermission(kind), admin); err != nil {
			return err
		}
	}
	var owner, status string
	if err := tx.QueryRowContext(ctx, "SELECT user_id,status FROM fleet_operations WHERE id=?", id).Scan(&owner, &status); err != nil || owner != userID {
		return ErrFleetAccess
	}
	if action == "cancel" {
		if status != "running" && status != "paused" && status != "cancelling" {
			return nil
		}
		if _, err := tx.ExecContext(ctx, "UPDATE fleet_operations SET status='cancelling',updated_at=? WHERE id=?", nowText(), id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status='cancelled' WHERE operation_id=? AND status='pending'", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE commands SET status='cancelled',cancelled_at=? WHERE status='queued' AND id IN (SELECT command_id FROM fleet_operation_items WHERE operation_id=?)", nowText(), id); err != nil {
			return err
		}
	} else if action == "resume" && status == "paused" {
		if _, err := tx.ExecContext(ctx, "UPDATE fleet_operations SET status='running',updated_at=? WHERE id=?", nowText(), id); err != nil {
			return err
		}
	} else {
		return errors.New("operation action is unavailable")
	}
	return tx.Commit()
}

func (s *Store) AdvanceFleetOperations(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM fleet_operations WHERE status IN ('running','paused','cancelling') ORDER BY updated_at LIMIT 100")
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.advanceFleetOperation(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) advanceFleetOperation(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fleetLock(ctx, tx); err != nil {
		return err
	}
	op, err := loadFleetOperation(ctx, tx, id)
	if err != nil {
		return err
	}
	if op.Status != "running" && op.Status != "paused" && op.Status != "cancelling" {
		return nil
	}
	admin, accessErr := fleetPrincipal(ctx, tx, op.UserID)
	if _, err := tx.ExecContext(ctx, "UPDATE commands SET status='expired',expired_at=? WHERE status IN ('queued','claimed') AND expires_at IS NOT NULL AND julianday(expires_at)<=julianday(?) AND id IN (SELECT command_id FROM fleet_operation_items WHERE operation_id=?)", nowText(), nowText(), id); err != nil {
		return err
	}
	deadline, err := time.Parse(time.RFC3339Nano, op.DeadlineAt)
	if err != nil {
		return err
	}
	if !deadline.After(time.Now()) {
		if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status='failed',error='maintenance window expired' WHERE operation_id=? AND status='pending'", id); err != nil {
			return err
		}
		for i := range op.Items {
			if op.Items[i].Status == "pending" {
				op.Items[i].Status = "failed"
				op.Items[i].Error = "maintenance window expired"
			}
		}
	}
	active, failed := 0, 0
	newFailure := false
	for i := range op.Items {
		item := &op.Items[i]
		if item.Status == "queued" || item.Status == "claimed" {
			var status, result string
			var completed sql.NullString
			var exit sql.NullInt64
			err := tx.QueryRowContext(ctx, "SELECT status,exit_code,result_json,completed_at FROM commands WHERE id=?", item.CommandID).Scan(&status, &exit, &result, &completed)
			if errors.Is(err, sql.ErrNoRows) {
				status = "failed"
			} else if err != nil {
				return err
			}
			if status == "completed" && (!exit.Valid || exit.Int64 != 0) {
				status = "failed"
			}
			if status == "cancelled" && op.Status != "cancelling" {
				status = "failed"
			}
			if op.Type == "agent_update" && status == "completed" {
				var health struct {
					Status string `json:"health_status"`
				}
				_ = json.Unmarshal([]byte(result), &health)
				if health.Status != "healthy" {
					status = "claimed"
					if !deadline.After(time.Now()) || (completed.Valid && time.Since(parseTime(completed.String)) > 5*time.Minute) {
						status = "failed"
					}
				}
			}
			if status == "queued" || status == "claimed" {
				active++
			} else {
				if status != "completed" && status != "cancelled" {
					status = "failed"
					newFailure = true
					item.Error = "command failed, expired or was removed"
				}
			}
			item.Status = status
			if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status=?,error=? WHERE operation_id=? AND device_id=?", item.Status, item.Error, id, item.DeviceID); err != nil {
				return err
			}
		}
		if item.Status == "failed" {
			failed++
		}
	}
	if newFailure && op.StopOnFailure && op.Status == "running" {
		op.Status = "paused"
	}
	if op.CanaryPending {
		for _, item := range op.Items {
			if item.CommandID != "" && item.Status != "queued" && item.Status != "claimed" {
				op.CanaryPending = false
				if op.Status == "running" {
					op.Status = "paused"
				}
				break
			}
		}
	}
	parallelism := op.Parallelism
	if op.CanaryPending {
		parallelism = 1
	}
	if op.Status == "running" {
		for i := range op.Items {
			item := &op.Items[i]
			if item.Status != "pending" || active >= parallelism {
				continue
			}
			targetErr := accessErr
			if targetErr == nil {
				targetErr = fleetDeviceAccess(ctx, tx, op.UserID, item.DeviceID, admin)
			}
			if targetErr == nil {
				targetErr = permissionAllowed(ctx, tx, op.UserID, item.DeviceID, CommandPermission(op.Type), admin)
			}
			if targetErr != nil && !errors.Is(targetErr, ErrFleetAccess) {
				return targetErr
			}
			if targetErr != nil {
				item.Status = "failed"
				item.Error = "device access changed"
				failed++
				if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status='failed',error=? WHERE operation_id=? AND device_id=?", item.Error, id, item.DeviceID); err != nil {
					return err
				}
				if op.StopOnFailure {
					op.Status = "paused"
					break
				}
				continue
			}
			if op.Type == "agent_update" {
				alreadyCurrent, updateErr := validateFleetUpdateDevice(ctx, tx, item.DeviceID, item.Args)
				if updateErr != nil && !errors.Is(updateErr, ErrFleetFeatureUnavailable) {
					return updateErr
				}
				if alreadyCurrent || updateErr != nil {
					item.Status = "completed"
					if updateErr != nil {
						item.Status = "failed"
						item.Error = "update platform changed; recreate schedule"
						failed++
						if op.StopOnFailure {
							op.Status = "paused"
						}
					}
					if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status=?,error=? WHERE operation_id=? AND device_id=?", item.Status, item.Error, id, item.DeviceID); err != nil {
						return err
					}
					if op.Status == "paused" {
						break
					}
					continue
				}
			}
			if op.Type == "system_backup_create" {
				var inProgress int
				if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_backups WHERE device_id=? AND status='creating'", item.DeviceID).Scan(&inProgress); err != nil {
					return err
				}
				if inProgress > 0 {
					continue
				}
			}
			command, err := s.newCommand(item.DeviceID, op.Type, item.Args, time.Until(deadline))
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO commands (id,device_id,type,args_json,status,max_attempts,created_at,expires_at,nonce,signature_key_id,signature) VALUES (?,?,?,?,'queued',3,?,?,?,?,?)", command.ID, command.DeviceID, command.Type, string(command.Args), command.CreatedAt.Format(time.RFC3339Nano), command.ExpiresAt.Format(time.RFC3339Nano), command.Nonce, command.SignatureKeyID, command.Signature); err != nil {
				return err
			}
			if op.Type == "system_backup_create" {
				backupID, err := randomID("bkp")
				if err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO device_backups (id,device_id,command_id,status,created_at) VALUES (?,?,?,'creating',?)", backupID, item.DeviceID, command.ID, command.CreatedAt.Format(time.RFC3339Nano)); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, "UPDATE fleet_operation_items SET status='queued',command_id=? WHERE operation_id=? AND device_id=?", command.ID, id, item.DeviceID); err != nil {
				return err
			}
			item.Status = "queued"
			active++
			if _, err := tx.ExecContext(ctx, "INSERT INTO audit_events (id,actor,action,device_id,command_id,details_json,created_at) VALUES (?,?, 'fleet.command_create',?,?,?,?)", command.ID+"-fleet", op.UserID, item.DeviceID, command.ID, `{"operation_id":"`+op.ID+`"}`, nowText()); err != nil {
				return err
			}
		}
	}
	pending := 0
	for _, item := range op.Items {
		if item.Status == "pending" {
			pending++
		}
	}
	if pending == 0 && active == 0 {
		if op.Status == "cancelling" {
			op.Status = "cancelled"
		} else if failed > 0 {
			op.Status = "failed"
		} else {
			op.Status = "completed"
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE fleet_operations SET status=?,canary_pending=?,updated_at=? WHERE id=?", op.Status, op.CanaryPending, nowText(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func FleetCommandAllowed(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "ping", "traceroute", "route_show", "interfaces_show", "pkg_list_installed", "system_backup_create", "diagnostic_report", "uci_profile_preview", "uci_profile_apply", "uci_profile_rollback", "agent_update":
		return true
	}
	return false
}
