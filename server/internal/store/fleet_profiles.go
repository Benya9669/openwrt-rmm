package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"rmm-openwrt/internal/fleetprofile"
	"time"
)

type FleetProfile struct {
	ID         string               `json:"id"`
	Title      string               `json:"title"`
	Definition fleetprofile.Profile `json:"definition"`
	UpdatedAt  string               `json:"updated_at"`
}

func (s *Store) SaveFleetProfile(ctx context.Context, userID string, profile FleetProfile) (FleetProfile, error) {
	if err := fleetprofile.Validate(profile.Definition); err != nil {
		return profile, err
	}
	if len(profile.Title) == 0 || len(profile.Title) > 160 {
		return profile, errors.New("invalid profile title")
	}
	// Passwords and keys use the existing credential workflows. Keeping them out of
	// declarative profiles also keeps signed queue arguments free of credentials.
	for _, option := range profile.Definition.Options {
		if fleetprofile.Sensitive(option.Option) {
			return profile, errors.New("credential options cannot be stored in fleet profiles")
		}
	}
	if profile.ID == "" {
		var err error
		profile.ID, err = randomID("profile")
		if err != nil {
			return profile, err
		}
	}
	definition, _ := json.Marshal(profile.Definition)
	encrypted, err := s.encryptSensitive(sensitiveContext("fleet_profile", profile.ID, "definition"), string(definition))
	if err != nil {
		return profile, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profile, err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return profile, err
	}
	if _, err = fleetPrincipal(ctx, tx, userID); err != nil {
		return profile, err
	}
	var owner string
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM fleet_profiles WHERE id=?", profile.ID).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return profile, err
	}
	if err == nil && owner != userID {
		return profile, ErrFleetAccess
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fleet_profiles WHERE user_id=?", userID).Scan(&count); err != nil {
		return profile, err
	}
	if count >= 100 && owner == "" {
		return profile, errors.New("too many profiles")
	}
	profile.UpdatedAt = nowText()
	_, err = tx.ExecContext(ctx, "INSERT INTO fleet_profiles (id,user_id,title,config,definition_encrypted,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,config=excluded.config,definition_encrypted=excluded.definition_encrypted,updated_at=excluded.updated_at", profile.ID, userID, profile.Title, profile.Definition.Config, encrypted, nowText(), profile.UpdatedAt)
	if err != nil {
		return profile, err
	}
	return profile, tx.Commit()
}

func (s *Store) ListFleetProfiles(ctx context.Context, userID string) ([]FleetProfile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,title,definition_encrypted,updated_at FROM fleet_profiles WHERE user_id=? ORDER BY title", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FleetProfile{}
	for rows.Next() {
		var profile FleetProfile
		var encrypted string
		if err = rows.Scan(&profile.ID, &profile.Title, &encrypted, &profile.UpdatedAt); err != nil {
			return nil, err
		}
		definition, err := s.decryptSensitive(sensitiveContext("fleet_profile", profile.ID, "definition"), encrypted)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(definition), &profile.Definition); err != nil {
			return nil, err
		}
		profile.Definition = fleetprofile.Mask(profile.Definition)
		result = append(result, profile)
	}
	return result, rows.Err()
}

type FleetProfileOperation struct {
	ProfileID          string   `json:"profile_id"`
	DeviceIDs          []string `json:"device_ids"`
	RequestKey         string   `json:"request_key"`
	Action             string   `json:"action"`
	PreviewOperationID string   `json:"preview_operation_id"`
	Parallelism        int      `json:"parallelism"`
}

func (s *Store) CreateFleetProfileOperation(ctx context.Context, userID string, input FleetProfileOperation) (FleetOperation, error) {
	if input.Action != "preview" && input.Action != "apply" {
		return FleetOperation{}, errors.New("invalid profile action")
	}
	var encrypted, title string
	if err := s.db.QueryRowContext(ctx, "SELECT definition_encrypted,title FROM fleet_profiles WHERE id=? AND user_id=?", input.ProfileID, userID).Scan(&encrypted, &title); err != nil {
		return FleetOperation{}, ErrFleetAccess
	}
	definition, err := s.decryptSensitive(sensitiveContext("fleet_profile", input.ProfileID, "definition"), encrypted)
	if err != nil {
		return FleetOperation{}, err
	}
	var profile fleetprofile.Profile
	if err = json.Unmarshal([]byte(definition), &profile); err != nil {
		return FleetOperation{}, err
	}
	if err = fleetprofile.Validate(profile); err != nil {
		return FleetOperation{}, err
	}
	args, _ := json.Marshal(map[string]string{"profile_json": definition})
	operation := FleetOperationInput{RequestKey: input.RequestKey, Title: title + " / " + input.Action, DeviceIDs: input.DeviceIDs, Type: "uci_profile_" + input.Action, Args: args, Parallelism: input.Parallelism, StopOnFailure: true, Canary: input.Action == "apply", DeviceArgs: map[string]json.RawMessage{}}
	if input.Action == "apply" {
		for _, deviceID := range input.DeviceIDs {
			var commandID string
			err = s.db.QueryRowContext(ctx, `SELECT i.command_id FROM fleet_operation_items i JOIN fleet_operations o ON o.id=i.operation_id JOIN commands c ON c.id=i.command_id WHERE o.id=? AND o.user_id=? AND o.command_type='uci_profile_preview' AND i.device_id=? AND c.status='completed' AND c.exit_code=0 AND julianday(c.completed_at)>julianday(?)`, input.PreviewOperationID, userID, deviceID, time.Now().Add(-30*time.Minute).UTC().Format(time.RFC3339Nano)).Scan(&commandID)
			if err != nil {
				return FleetOperation{}, errors.New("a recent successful preview is required for every device")
			}
			operation.DeviceArgs[deviceID], err = json.Marshal(map[string]string{"profile_json": definition, "preview_id": commandID})
			if err != nil {
				return FleetOperation{}, err
			}
		}
	}
	return s.CreateFleetOperation(ctx, userID, operation)
}
