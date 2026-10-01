package store

import (
	"context"
	"encoding/json"
	"errors"
	"rmm-openwrt/server/internal/model"
	"time"
)

func (s *Store) CreateAuthorizedCommand(ctx context.Context, userID, deviceID, kind string, args json.RawMessage) (model.Command, bool, error) {
	commands, err := s.CreateAuthorizedCommandBatch(ctx, userID, []string{deviceID}, kind, args)
	if err != nil {
		return model.Command{}, false, err
	}
	return commands[0], true, nil
}

// Authorize the entire selection under the same lock as permission edits before
// inserting anything. A denied target must not leave a partially started batch.
func (s *Store) CreateAuthorizedCommandBatch(ctx context.Context, userID string, deviceIDs []string, kind string, args json.RawMessage) ([]model.Command, error) {
	if len(deviceIDs) < 1 || len(deviceIDs) > 100 {
		return nil, errors.New("invalid batch size")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return nil, err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	permission := CommandPermission(kind)
	commands := []model.Command{}
	seen := map[string]bool{}
	for _, id := range deviceIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if err = fleetDeviceAccess(ctx, tx, userID, id, admin); err != nil {
			return nil, err
		}
		if err = permissionAllowed(ctx, tx, userID, id, permission, admin); err != nil {
			return nil, err
		}
		command, err := s.newCommand(id, kind, args, 24*time.Hour)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	for _, command := range commands {
		if _, err = tx.ExecContext(ctx, "INSERT INTO commands (id,device_id,type,args_json,status,max_attempts,created_at,expires_at,nonce,signature_key_id,signature) VALUES (?,?,?,?,'queued',3,?,?,?,?,?)", command.ID, command.DeviceID, kind, string(command.Args), command.CreatedAt.Format(time.RFC3339Nano), command.ExpiresAt.Format(time.RFC3339Nano), command.Nonce, command.SignatureKeyID, command.Signature); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO management_command_permissions (command_id,user_id,permission) VALUES (?,?,?)", command.ID, userID, permission); err != nil {
			return nil, err
		}
	}
	return commands, tx.Commit()
}
