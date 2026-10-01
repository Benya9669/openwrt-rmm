package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

type FleetRule struct {
	RequestKey       string   `json:"request_key,omitempty"`
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	DeviceIDs        []string `json:"device_ids"`
	Kind             string   `json:"kind"`
	Threshold        float64  `json:"threshold"`
	Consecutive      int      `json:"consecutive"`
	CooldownSeconds  int      `json:"cooldown_seconds"`
	MaxActionsPerDay int      `json:"max_actions_per_day"`
	Action           string   `json:"action"`
	Service          string   `json:"service,omitempty"`
	Enabled          bool     `json:"enabled"`
}

func reactionCommandStillAllowed(ctx context.Context, tx *sqliteTx, deviceID, ruleID, service string) (bool, error) {
	var definition, userID string
	var enabled int
	err := tx.QueryRowContext(ctx, "SELECT definition_json,user_id,enabled FROM fleet_rules WHERE id=?", ruleID).Scan(&definition, &userID, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if enabled == 0 {
		return false, nil
	}
	var rule FleetRule
	if err = json.Unmarshal([]byte(definition), &rule); err != nil {
		return false, err
	}
	if rule.Action != "service_restart" || rule.Service != service || (service != "dnsmasq" && service != "uhttpd") || !slices.Contains(rule.DeviceIDs, deviceID) {
		return false, nil
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if errors.Is(err, ErrFleetAccess) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = fleetDeviceAccess(ctx, tx, userID, deviceID, admin); errors.Is(err, ErrFleetAccess) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err = permissionAllowed(ctx, tx, userID, deviceID, "maintenance", admin); errors.Is(err, ErrFleetAccess) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if rule.Kind == "service_down" {
		var raw string
		err = tx.QueryRowContext(ctx, "SELECT result_json FROM commands WHERE device_id=? AND type='diagnostic_report' AND status IN ('completed','failed') AND julianday(completed_at)>julianday(?) ORDER BY completed_at DESC LIMIT 1", deviceID, time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339Nano)).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var report struct {
			Diagnostics struct {
				Checks []struct{ Name, Status string } `json:"checks"`
			} `json:"diagnostics"`
		}
		if json.Unmarshal([]byte(raw), &report) != nil {
			return false, nil
		}
		for _, check := range report.Diagnostics.Checks {
			if check.Name == "service:"+service {
				return check.Status == "failed", nil
			}
		}
		return false, nil
	}
	var metrics string
	var seen sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT metrics_json,last_seen_at FROM devices WHERE id=?", deviceID).Scan(&metrics, &seen); err != nil {
		return false, err
	}
	if !seen.Valid || time.Since(parseTime(seen.String)) >= 2*time.Minute {
		return false, nil
	}
	violation, known := reactionViolation(rule, json.RawMessage(metrics))
	return violation && known, nil
}

func (s *Store) SaveFleetRule(ctx context.Context, userID string, rule FleetRule) (FleetRule, error) {
	newRequest := rule.ID == "" && rule.RequestKey != ""
	if newRequest {
		var err error
		rule.ID, err = fleetObjectID("rule", userID, rule.RequestKey)
		if err != nil {
			return rule, err
		}
	}
	if len(rule.Title) == 0 || len(rule.Title) > 160 || len(rule.DeviceIDs) == 0 || len(rule.DeviceIDs) > 100 || rule.Consecutive < 1 || rule.Consecutive > 20 || rule.CooldownSeconds < 300 || rule.CooldownSeconds > 86400 || rule.MaxActionsPerDay < 1 || rule.MaxActionsPerDay > 5 || math.IsNaN(rule.Threshold) || math.IsInf(rule.Threshold, 0) || rule.Threshold < 0 || rule.Threshold > 100 {
		return rule, errors.New("invalid reaction limits")
	}
	switch rule.Kind {
	case "memory_high", "load_high", "wan_unreachable", "offline", "service_down":
	default:
		return rule, errors.New("invalid reaction condition")
	}
	if rule.Action != "notify" && rule.Action != "service_restart" {
		return rule, errors.New("invalid reaction action")
	}
	if rule.Action == "service_restart" || rule.Kind == "service_down" {
		if rule.Service != "dnsmasq" && rule.Service != "uhttpd" {
			return rule, errors.New("automatic service must be dnsmasq or uhttpd")
		}
	}
	if rule.Action == "service_restart" && rule.Kind == "offline" {
		return rule, errors.New("offline rules can only notify")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return rule, err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return rule, err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return rule, err
	}
	if err = permissionAllowed(ctx, tx, userID, "", rulePermission(rule), admin); err != nil {
		return rule, err
	}
	for _, id := range rule.DeviceIDs {
		if err = fleetDeviceAccess(ctx, tx, userID, id, admin); err != nil {
			return rule, err
		}
	}
	if newRequest {
		var stored string
		err := tx.QueryRowContext(ctx, "SELECT definition_json FROM fleet_rules WHERE id=? AND user_id=?", rule.ID, userID).Scan(&stored)
		if err == nil {
			definition, _ := json.Marshal(rule)
			if stored != string(definition) {
				return rule, ErrFleetConflict
			}
			return rule, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return rule, err
		}
	}
	if rule.ID == "" || newRequest {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fleet_rules WHERE user_id=?", userID).Scan(&count); err != nil {
			return rule, err
		}
		if count >= 100 {
			return rule, errors.New("too many reaction rules")
		}
		if rule.ID == "" {
			rule.ID, err = randomID("rule")
			if err != nil {
				return rule, err
			}
		}
	} else if !newRequest {
		var owner string
		if err = tx.QueryRowContext(ctx, "SELECT user_id FROM fleet_rules WHERE id=?", rule.ID).Scan(&owner); err != nil || owner != userID {
			return rule, ErrFleetAccess
		}
	}
	definition, _ := json.Marshal(rule)
	if _, err = tx.ExecContext(ctx, "INSERT INTO fleet_rules (id,user_id,title,definition_json,enabled,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,definition_json=excluded.definition_json,enabled=excluded.enabled,updated_at=excluded.updated_at", rule.ID, userID, rule.Title, string(definition), rule.Enabled, nowText(), nowText()); err != nil {
		return rule, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM fleet_rule_states WHERE rule_id=?", rule.ID); err != nil {
		return rule, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE commands SET status='cancelled' WHERE status='queued' AND id IN (SELECT command_id FROM fleet_rule_events WHERE rule_id=?)", rule.ID); err != nil {
		return rule, err
	}
	return rule, tx.Commit()
}

func (s *Store) ListFleetRules(ctx context.Context, userID string) ([]FleetRule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT definition_json FROM fleet_rules WHERE user_id=? ORDER BY title", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FleetRule{}
	for rows.Next() {
		var text string
		var rule FleetRule
		if err = rows.Scan(&text); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(text), &rule); err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, rows.Err()
}

func reactionViolation(rule FleetRule, raw json.RawMessage) (bool, bool) {
	var metrics struct {
		Load   string `json:"loadavg"`
		Memory struct {
			Total float64 `json:"total_kb"`
			Used  float64 `json:"used_kb"`
		} `json:"memory"`
		Checks []struct {
			Reachable *bool `json:"reachable"`
		} `json:"connectivity_checks"`
	}
	if json.Unmarshal(raw, &metrics) != nil {
		return false, false
	}
	switch rule.Kind {
	case "memory_high":
		if metrics.Memory.Total <= 0 || metrics.Memory.Used < 0 || metrics.Memory.Used > metrics.Memory.Total {
			return false, false
		}
		return metrics.Memory.Used/metrics.Memory.Total*100 >= rule.Threshold, true
	case "load_high":
		fields := strings.Fields(metrics.Load)
		if len(fields) == 0 {
			return false, false
		}
		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false, false
		}
		return value >= rule.Threshold, true
	case "wan_unreachable":
		if len(metrics.Checks) == 0 {
			return false, false
		}
		for _, check := range metrics.Checks {
			if check.Reachable == nil {
				return false, false
			}
			if *check.Reachable {
				return false, true
			}
		}
		return true, true
	}
	return false, false
}

func (s *Store) AdvanceFleetRules(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id,user_id FROM fleet_rules WHERE enabled=1 ORDER BY updated_at LIMIT 100")
	if err != nil {
		return err
	}
	type activeRule struct{ id, user string }
	rules := []activeRule{}
	for rows.Next() {
		var item activeRule
		if err = rows.Scan(&item.id, &item.user); err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if err = s.advanceFleetRule(ctx, rule.id, rule.user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) advanceFleetRule(ctx context.Context, id, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return err
	}
	var definition string
	var enabled int
	if err = tx.QueryRowContext(ctx, "SELECT definition_json,enabled FROM fleet_rules WHERE id=? AND user_id=?", id, userID).Scan(&definition, &enabled); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if enabled == 0 {
		return nil
	}
	var rule FleetRule
	if err = json.Unmarshal([]byte(definition), &rule); err != nil {
		return err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if errors.Is(err, ErrFleetAccess) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = permissionAllowed(ctx, tx, userID, "", rulePermission(rule), admin); errors.Is(err, ErrFleetAccess) {
		return nil
	} else if err != nil {
		return err
	}
	rule.DeviceIDs = slices.Compact(rule.DeviceIDs)
	now := time.Now().UTC()
	for _, deviceID := range rule.DeviceIDs {
		if err = fleetDeviceAccess(ctx, tx, userID, deviceID, admin); errors.Is(err, ErrFleetAccess) {
			continue
		} else if err != nil {
			return err
		}
		var lastSeen sql.NullString
		if err = tx.QueryRowContext(ctx, "SELECT last_seen_at FROM devices WHERE id=?", deviceID).Scan(&lastSeen); err != nil {
			return err
		}
		if !lastSeen.Valid {
			continue
		}
		online := now.Sub(parseTime(lastSeen.String)) < 2*time.Minute
		var sampleID, raw string
		violation, known := false, false
		switch rule.Kind {
		case "offline":
			sampleID = "offline:" + now.Truncate(time.Minute).Format(time.RFC3339)
			violation, known = !online, true
		case "service_down":
			err = tx.QueryRowContext(ctx, "SELECT id,result_json FROM commands WHERE device_id=? AND type='diagnostic_report' AND status IN ('completed','failed') AND julianday(completed_at)>julianday(?) ORDER BY completed_at DESC LIMIT 1", deviceID, now.Add(-10*time.Minute).Format(time.RFC3339Nano)).Scan(&sampleID, &raw)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			var report struct {
				Diagnostics struct {
					Checks []struct{ Name, Status string } `json:"checks"`
				} `json:"diagnostics"`
			}
			if json.Unmarshal([]byte(raw), &report) != nil {
				continue
			}
			for _, check := range report.Diagnostics.Checks {
				if check.Name == "service:"+rule.Service && (check.Status == "passed" || check.Status == "failed") {
					violation, known = check.Status == "failed", true
				}
			}
		default:
			err = tx.QueryRowContext(ctx, "SELECT id,metrics_json FROM metric_samples WHERE device_id=? AND julianday(created_at)>julianday(?) ORDER BY created_at DESC LIMIT 1", deviceID, now.Add(-2*time.Minute).Format(time.RFC3339Nano)).Scan(&sampleID, &raw)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			violation, known = reactionViolation(rule, json.RawMessage(raw))
		}
		if !known && sampleID == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO fleet_rule_states (rule_id,device_id) VALUES (?,?) ON CONFLICT(rule_id,device_id) DO NOTHING", id, deviceID); err != nil {
			return err
		}
		var last, next, day string
		var count, attempts int
		if err = tx.QueryRowContext(ctx, "SELECT last_sample,violations,next_action_at,action_day,action_count FROM fleet_rule_states WHERE rule_id=? AND device_id=?", id, deviceID).Scan(&last, &count, &next, &day, &attempts); err != nil {
			return err
		}
		if sampleID == last {
			continue
		}
		if day != now.Format("2006-01-02") {
			day = now.Format("2006-01-02")
			attempts = 0
		}
		if violation && known {
			count = min(count+1, rule.Consecutive)
		} else {
			count = 0
			if _, err = tx.ExecContext(ctx, "UPDATE commands SET status='cancelled' WHERE status='queued' AND id IN (SELECT command_id FROM fleet_rule_events WHERE rule_id=? AND device_id=?)", id, deviceID); err != nil {
				return err
			}
		}
		if known && (!violation || count >= rule.Consecutive) {
			if err = syncIncidentSource(ctx, tx, deviceID, "rule:"+id, incidentCategory(rule.Kind, rule.Service), rule.Title, violation); err != nil {
				return err
			}
		}
		if count >= rule.Consecutive && !parseTime(next).After(now) && attempts < rule.MaxActionsPerDay {
			var active int
			if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands WHERE status IN ('queued','claimed') AND id IN (SELECT command_id FROM fleet_rule_events WHERE rule_id=? AND device_id=?)", id, deviceID).Scan(&active); err != nil {
				return err
			}
			if active == 0 && (rule.Action == "notify" || online) {
				eventID, err := randomID("reaction")
				if err != nil {
					return err
				}
				commandID := ""
				status := "notified"
				if rule.Action == "service_restart" {
					args, _ := json.Marshal(map[string]string{"service": rule.Service, "rule_id": id})
					command, err := s.newCommand(deviceID, "service_restart", args, 5*time.Minute)
					if err != nil {
						return err
					}
					if _, err = tx.ExecContext(ctx, "INSERT INTO commands (id,device_id,type,args_json,status,max_attempts,created_at,expires_at,nonce,signature_key_id,signature) VALUES (?,?,?,?,'queued',1,?,?,?,?,?)", command.ID, deviceID, command.Type, string(args), command.CreatedAt.Format(time.RFC3339Nano), command.ExpiresAt.Format(time.RFC3339Nano), command.Nonce, command.SignatureKeyID, command.Signature); err != nil {
						return err
					}
					commandID = command.ID
					status = "queued"
				} else {
					if _, err = tx.ExecContext(ctx, "INSERT INTO inbox_notifications (id,user_id,device_id,dedupe_key,severity,event,title,body,created_at) VALUES (?,?,?,?,'warning','fleet.reaction',?,?,?)", eventID, userID, deviceID, eventID, rule.Title, "Условие правила подтверждено последовательными наблюдениями", nowText()); err != nil {
						return err
					}
				}
				if _, err = tx.ExecContext(ctx, "INSERT INTO fleet_rule_events (id,rule_id,device_id,action,status,command_id,message,created_at) VALUES (?,?,?,?,?,?,?,?)", eventID, id, deviceID, rule.Action, status, commandID, "condition: "+rule.Kind, nowText()); err != nil {
					return err
				}
				details, _ := json.Marshal(map[string]string{"rule_id": id, "action": rule.Action})
				if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events (id,actor,action,device_id,command_id,details_json,created_at) VALUES (?,?,'fleet.reaction',?,?,?,?)", eventID, userID, deviceID, commandID, string(details), nowText()); err != nil {
					return err
				}
				attempts++
				count = 0
				next = now.Add(time.Duration(rule.CooldownSeconds) * time.Second).Format(time.RFC3339Nano)
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE fleet_rule_states SET last_sample=?,violations=?,next_action_at=?,action_day=?,action_count=? WHERE rule_id=? AND device_id=?", sampleID, count, next, day, attempts, id, deviceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListFleetRuleEvents(ctx context.Context, userID string) ([]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT e.id,e.rule_id,e.device_id,e.action,CASE WHEN e.command_id='' THEN e.status WHEN c.id IS NULL THEN 'failed' ELSE c.status END,e.command_id,e.message,e.created_at FROM fleet_rule_events e JOIN fleet_rules r ON r.id=e.rule_id LEFT JOIN commands c ON c.id=e.command_id WHERE r.user_id=? ORDER BY e.created_at DESC LIMIT 100", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var id, rule, device, action, status, command, message, created string
		if err = rows.Scan(&id, &rule, &device, &action, &status, &command, &message, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{"id": id, "rule_id": rule, "device_id": device, "action": action, "status": status, "command_id": command, "message": message, "created_at": created})
	}
	return result, rows.Err()
}

func rulePermission(rule FleetRule) string {
	if rule.Action == "notify" {
		return "incidents"
	}
	return "maintenance"
}
