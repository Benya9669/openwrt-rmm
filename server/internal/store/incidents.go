package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Incident struct {
	ID          string          `json:"id"`
	DeviceID    string          `json:"device_id"`
	Category    string          `json:"category"`
	Title       string          `json:"title"`
	Status      string          `json:"status"`
	AssigneeID  string          `json:"assignee_id"`
	Occurrences int             `json:"occurrences"`
	OpenedAt    string          `json:"opened_at"`
	UpdatedAt   string          `json:"updated_at"`
	ResolvedAt  string          `json:"resolved_at"`
	Events      []IncidentEvent `json:"events"`
}
type IncidentEvent struct {
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func incidentCategory(kind, service string) string {
	switch kind {
	case "offline", "wan_unreachable", "wan_down", "packet_loss_high", "latency_high", "connectivity", "dns", "wan":
		return "network"
	case "load_high", "memory_high":
		return "resources"
	case "service_down":
		return "service:" + service
	default:
		return kind
	}
}

func incidentEvent(ctx context.Context, tx *sqliteTx, id, actor, action, body string) error {
	key, err := randomID("incident_event")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO management_incident_events (id,incident_id,actor,action,body,created_at) VALUES (?,?,?,?,?,?)", key, id, actor, action, body, nowText())
	return err
}

// Correlate confirmed conditions on the same device/category. Only a new
// activation reopens a manually resolved incident. Unknown observations do not
// resolve incidents, and repeated observations do not append duplicate events.
func syncIncidentSource(ctx context.Context, tx *sqliteTx, device, source, category, title string, active bool) error {
	wasActive := 0
	err := tx.QueryRowContext(ctx, "SELECT active FROM management_incident_sources WHERE device_id=? AND source_key=?", device, source).Scan(&wasActive)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	now := nowText()
	if _, err = tx.ExecContext(ctx, "INSERT INTO management_incident_sources (device_id,source_key,category,active,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(device_id,source_key) DO UPDATE SET category=excluded.category,active=excluded.active,updated_at=excluded.updated_at", device, source, category, active, now); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM management_incident_sources WHERE device_id=? AND category=? AND active=1", device, category).Scan(&count); err != nil {
		return err
	}
	var id, status string
	err = tx.QueryRowContext(ctx, "SELECT id,status FROM management_incidents WHERE device_id=? AND source_key=?", device, category).Scan(&id, &status)
	if errors.Is(err, sql.ErrNoRows) {
		if count == 0 {
			return nil
		}
		id, err = randomID("incident")
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO management_incidents (id,device_id,source_key,title,status,opened_at,updated_at) VALUES (?,?,?,?,'open',?,?)", id, device, category, title, now, now); err != nil {
			return err
		}
		return incidentEvent(ctx, tx, id, "system", "opened", source)
	}
	if err != nil {
		return err
	}
	if count == 0 && status != "resolved" {
		if _, err = tx.ExecContext(ctx, "UPDATE management_incidents SET status='resolved',resolved_at=?,updated_at=? WHERE id=?", now, now, id); err != nil {
			return err
		}
		return incidentEvent(ctx, tx, id, "system", "recovered", "Все связанные наблюдаемые условия восстановились")
	}
	if active && wasActive == 0 {
		action := "condition_added"
		if status == "resolved" {
			action = "reopened"
			if _, err = tx.ExecContext(ctx, "UPDATE management_incidents SET status='open',resolved_at='',opened_at=?,occurrences=occurrences+1,updated_at=? WHERE id=?", now, now, id); err != nil {
				return err
			}
		}
		return incidentEvent(ctx, tx, id, "system", action, source)
	}
	return nil
}

func (s *Store) SyncAlertIncidents(ctx context.Context, deviceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,type,message,status FROM alerts WHERE device_id=?", deviceID)
	if err != nil {
		return err
	}
	type source struct{ id, kind, title, status string }
	sources := []source{}
	for rows.Next() {
		var v source
		if err = rows.Scan(&v.id, &v.kind, &v.title, &v.status); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range sources {
		if err = syncIncidentSource(ctx, tx, deviceID, "alert:"+v.id, incidentCategory(v.kind, ""), v.title, v.status != "resolved"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListIncidents(ctx context.Context, userID string) ([]Incident, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT i.id,i.device_id,i.source_key,i.title,i.status,i.assignee_id,i.occurrences,i.opened_at,i.updated_at,i.resolved_at FROM management_incidents i JOIN devices d ON d.id=i.device_id WHERE d.owner_user_id=? OR ?=1 ORDER BY i.updated_at DESC LIMIT 200", userID, admin)
	if err != nil {
		return nil, err
	}
	result := []Incident{}
	for rows.Next() {
		var i Incident
		if err = rows.Scan(&i.ID, &i.DeviceID, &i.Category, &i.Title, &i.Status, &i.AssigneeID, &i.Occurrences, &i.OpenedAt, &i.UpdatedAt, &i.ResolvedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	visible := result[:0]
	for _, i := range result {
		if err = permissionAllowed(ctx, tx, userID, i.DeviceID, "view", admin); errors.Is(err, ErrFleetAccess) {
			continue
		} else if err != nil {
			return nil, err
		}
		events, err := tx.QueryContext(ctx, "SELECT actor,action,body,created_at FROM management_incident_events WHERE incident_id=? ORDER BY created_at DESC,id DESC LIMIT 100", i.ID)
		if err != nil {
			return nil, err
		}
		i.Events = []IncidentEvent{}
		for events.Next() {
			var e IncidentEvent
			if err = events.Scan(&e.Actor, &e.Action, &e.Body, &e.CreatedAt); err != nil {
				events.Close()
				return nil, err
			}
			i.Events = append(i.Events, e)
		}
		err = events.Err()
		events.Close()
		if err != nil {
			return nil, err
		}
		visible = append(visible, i)
	}
	return visible, nil
}

func (s *Store) UpdateIncident(ctx context.Context, userID, id, action, body string) error {
	body = strings.TrimSpace(body)
	if len(body) > 2048 {
		return errors.New("incident text too long")
	}
	if action != "acknowledge" && action != "resolve" && action != "assign" && action != "comment" {
		return errors.New("invalid incident action")
	}
	if action == "comment" && body == "" {
		return errors.New("comment required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	var device, status string
	if err = tx.QueryRowContext(ctx, "SELECT device_id,status FROM management_incidents WHERE id=?", id).Scan(&device, &status); errors.Is(err, sql.ErrNoRows) {
		return ErrFleetAccess
	} else if err != nil {
		return err
	}
	if err = permissionAllowed(ctx, tx, userID, device, "incidents", admin); err != nil {
		return err
	}
	switch action {
	case "assign":
		if body != "" {
			targetAdmin, err := fleetPrincipal(ctx, tx, body)
			if err != nil {
				return err
			}
			if err = permissionAllowed(ctx, tx, body, device, "incidents", targetAdmin); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE management_incidents SET assignee_id=?,updated_at=? WHERE id=?", body, nowText(), id)
	case "acknowledge":
		if status == "resolved" {
			return errors.New("incident already resolved")
		}
		_, err = tx.ExecContext(ctx, "UPDATE management_incidents SET status='acknowledged',updated_at=? WHERE id=?", nowText(), id)
	case "resolve":
		_, err = tx.ExecContext(ctx, "UPDATE management_incidents SET status='resolved',resolved_at=?,updated_at=? WHERE id=?", nowText(), nowText(), id)
	case "comment":
		_, err = tx.ExecContext(ctx, "UPDATE management_incidents SET updated_at=? WHERE id=?", nowText(), id)
	}
	if err != nil {
		return err
	}
	if err = incidentEvent(ctx, tx, id, userID, action, body); err != nil {
		return err
	}
	auditID, err := randomID("audit")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events (id,actor,action,device_id,details_json,created_at) VALUES (?,?,?,?,?,?)", auditID, userID, "incident."+action, device, `{"incident_id":"`+id+`"}`, nowText()); err != nil {
		return err
	}
	return tx.Commit()
}
