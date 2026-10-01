package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
)

var ErrRolloutGuardInvalid = errors.New("invalid rollout guard")

func validateRolloutGuard(g *model.RolloutGuard, devices []model.RolloutDevice) error {
	if g != nil && g.RequestKey != "" && (len(g.RequestKey) < 8 || len(g.RequestKey) > 128) {
		return errors.New("invalid rollout request key")
	}
	if g == nil {
		return nil
	}
	if len(g.CanaryIDs) < 1 || len(g.CanaryIDs) > 10 || len(g.WaveSizes) < 1 || len(g.WaveSizes) > 8 || g.ObservationSeconds < 60 || g.ObservationSeconds > 3600 {
		return errors.New("invalid rollout waves or observation period")
	}
	ids := map[string]bool{}
	for _, d := range devices {
		ids[d.DeviceID] = true
	}
	seen := map[string]bool{}
	for _, id := range g.CanaryIDs {
		if !ids[id] || seen[id] {
			return errors.New("canary must be a unique eligible target")
		}
		seen[id] = true
	}
	for _, size := range g.WaveSizes {
		if size < 1 || size > 500 {
			return errors.New("invalid wave size")
		}
	}
	g.HealthySince = ""
	g.PauseReason = ""
	return nil
}

func loadRolloutGuard(ctx context.Context, tx *sqliteTx, id string) (*model.RolloutGuard, error) {
	var raw, healthy, reason string
	err := tx.QueryRowContext(ctx, "SELECT definition_json,healthy_since,pause_reason FROM management_rollout_guards WHERE rollout_id=?", id).Scan(&raw, &healthy, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var g model.RolloutGuard
	if err = json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, err
	}
	g.HealthySince = healthy
	g.PauseReason = reason
	return &g, nil
}

func pauseGuardedRollout(ctx context.Context, tx *sqliteTx, id, reason string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE agent_rollouts SET status='paused',updated_at=? WHERE id=? AND status='running'", nowText(), id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE management_rollout_guards SET healthy_since='',pause_reason=? WHERE rollout_id=?", reason, id)
	return err
}

// A wave is successful only after all its agents reconnect at the requested
// version and remain observable for the full period. A paused guard must be
// explicitly resumed; it does not restart itself on a later heartbeat.
func guardReady(ctx context.Context, tx *sqliteTx, id, version string, g *model.RolloutGuard) (bool, error) {
	var wave int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(batch),0) FROM agent_rollout_devices WHERE rollout_id=?", id).Scan(&wave); err != nil {
		return false, err
	}
	if wave == 0 {
		return true, nil
	}
	rows, err := tx.QueryContext(ctx, "SELECT rd.status,d.last_seen_at,d.inventory_json,d.metrics_json FROM agent_rollout_devices rd JOIN devices d ON d.id=rd.device_id WHERE rd.rollout_id=? AND rd.batch=?", id, wave)
	if err != nil {
		return false, err
	}
	type sample struct {
		status             string
		seen               sql.NullString
		inventory, metrics string
	}
	samples := []sample{}
	for rows.Next() {
		var v sample
		if err = rows.Scan(&v.status, &v.seen, &v.inventory, &v.metrics); err != nil {
			rows.Close()
			return false, err
		}
		samples = append(samples, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	for _, v := range samples {
		if slices.Contains([]string{"failed", "cancelled", "expired"}, v.status) {
			return false, pauseGuardedRollout(ctx, tx, id, "previous wave failed")
		}
	}
	for _, v := range samples {
		if slices.Contains([]string{"failed", "cancelled", "expired"}, v.status) {
			return false, pauseGuardedRollout(ctx, tx, id, "previous wave failed")
		}
		if v.status != "completed" {
			return false, nil
		}
		var inventory struct {
			Version string `json:"agent_version"`
		}
		if json.Unmarshal([]byte(v.inventory), &inventory) != nil || inventory.Version != version || !v.seen.Valid || now.Sub(parseTime(v.seen.String)) > 2*time.Minute {
			return false, pauseGuardedRollout(ctx, tx, id, "completed wave lost contact or changed version")
		}
		var metrics struct {
			Checks []struct {
				Reachable *bool `json:"reachable"`
			} `json:"connectivity_checks"`
		}
		if json.Unmarshal([]byte(v.metrics), &metrics) == nil && len(metrics.Checks) > 0 {
			known, reachable := false, false
			for _, check := range metrics.Checks {
				if check.Reachable != nil {
					known = true
					reachable = reachable || *check.Reachable
				}
			}
			if known && !reachable {
				return false, pauseGuardedRollout(ctx, tx, id, "completed wave reports loss of connectivity")
			}
		}
	}
	var lastCheck string
	if err = tx.QueryRowContext(ctx, "SELECT last_checked_at FROM management_rollout_guards WHERE rollout_id=?", id).Scan(&lastCheck); err != nil {
		return false, err
	}
	if lastCheck != "" && now.Sub(parseTime(lastCheck)) > 2*time.Minute {
		g.HealthySince = ""
	}
	if _, err = tx.ExecContext(ctx, "UPDATE management_rollout_guards SET last_checked_at=? WHERE rollout_id=?", now.Format(time.RFC3339Nano), id); err != nil {
		return false, err
	}
	if g.HealthySince == "" {
		_, err = tx.ExecContext(ctx, "UPDATE management_rollout_guards SET wave_observed=?,healthy_since=?,pause_reason='' WHERE rollout_id=?", wave, now.Format(time.RFC3339Nano), id)
		return false, err
	}
	return now.Sub(parseTime(g.HealthySince)) >= time.Duration(g.ObservationSeconds)*time.Second, nil
}

func (s *Store) AdvanceGuardedRollouts(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT r.id FROM agent_rollouts r JOIN management_rollout_guards g ON g.rollout_id=r.id WHERE r.status='running' ORDER BY r.created_at LIMIT 100")
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
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
		if err = s.queueRolloutBatch(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateGuardedAgentRollout(ctx context.Context, channel, version string, batchSize int, devices []model.RolloutDevice, g model.RolloutGuard) (model.AgentRollout, error) {
	if channel != "stable" && channel != "candidate" || strings.TrimSpace(version) == "" || len(devices) > 500 || len(devices) == 0 || batchSize < 1 || batchSize > 500 {
		return model.AgentRollout{}, errors.Join(ErrRolloutGuardInvalid, errors.New("invalid rollout"))
	}
	if err := validateRolloutGuard(&g, devices); err != nil {
		return model.AgentRollout{}, errors.Join(ErrRolloutGuardInvalid, err)
	}
	for _, d := range devices {
		if !strings.HasPrefix(d.ManifestURL, "https://") || !strings.HasPrefix(d.SignatureURL, "https://") {
			return model.AgentRollout{}, errors.Join(ErrRolloutGuardInvalid, errors.New("guarded waves require signed agent updates"))
		}
	}
	return s.createAgentRollout(ctx, channel, version, batchSize, 1, devices, &g)
}
