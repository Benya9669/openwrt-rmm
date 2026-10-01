package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
	_ "time/tzdata"
)

type FleetSchedule struct {
	RequestKey    string              `json:"request_key,omitempty"`
	ID            string              `json:"id"`
	UserID        string              `json:"user_id"`
	Title         string              `json:"title"`
	Operation     FleetOperationInput `json:"operation"`
	Timezone      string              `json:"timezone"`
	Weekdays      []int               `json:"weekdays"`
	MinuteOfDay   int                 `json:"minute_of_day"`
	WindowMinutes int                 `json:"window_minutes"`
	Enabled       bool                `json:"enabled"`
	NextRunAt     string              `json:"next_run_at"`
}

// Calendar arithmetic avoids accumulating UTC offsets across daylight-saving changes.
// A nonexistent local time is skipped; a repeated local time runs only once.
func nextFleetSlot(after time.Time, timezone string, weekdays []int, minute int) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil || minute < 0 || minute >= 1440 || len(weekdays) == 0 || len(weekdays) > 7 {
		return time.Time{}, errors.New("invalid schedule calendar")
	}
	for _, day := range weekdays {
		if day < 0 || day > 6 {
			return time.Time{}, errors.New("invalid weekday")
		}
	}
	local := after.In(loc)
	for offset := 0; offset <= 14; offset++ {
		date := time.Date(local.Year(), local.Month(), local.Day()+offset, 12, 0, 0, 0, loc)
		slot := time.Date(date.Year(), date.Month(), date.Day(), minute/60, minute%60, 0, 0, loc)
		if slot.Hour() != minute/60 || slot.Minute() != minute%60 {
			continue
		}
		if slices.Contains(weekdays, int(date.Weekday())) && slot.After(after) {
			return slot.UTC(), nil
		}
	}
	return time.Time{}, errors.New("no schedule slot found")
}

func (s *Store) SaveFleetSchedule(ctx context.Context, userID string, schedule FleetSchedule) (FleetSchedule, error) {
	newRequest := schedule.ID == "" && schedule.RequestKey != ""
	if newRequest {
		var err error
		schedule.ID, err = fleetObjectID("schedule", userID, schedule.RequestKey)
		if err != nil {
			return schedule, err
		}
	}
	if len(schedule.Title) == 0 || len(schedule.Title) > 160 || schedule.WindowMinutes < 1 || schedule.WindowMinutes > 1440 || !FleetCommandAllowed(schedule.Operation.Type) || len(schedule.Operation.DeviceIDs) == 0 || len(schedule.Operation.DeviceIDs) > 100 || schedule.Operation.Parallelism < 1 || schedule.Operation.Parallelism > 20 || len(schedule.Operation.Args) > fleetArgsLimit(schedule.Operation.Type) {
		return schedule, errors.New("invalid schedule")
	}
	if schedule.Operation.Type == "agent_update" {
		if _, err := fleetUpdateDeviceArgs(schedule.Operation.Args, schedule.Operation.DeviceIDs); err != nil {
			return schedule, err
		}
	}
	next, err := nextFleetSlot(time.Now(), schedule.Timezone, schedule.Weekdays, schedule.MinuteOfDay)
	if err != nil {
		return schedule, err
	}
	schedule.Operation.RequestKey = ""
	schedule.Operation.DeadlineAt = ""
	schedule.Operation.Title = schedule.Title
	schedule.Operation.Args = NormalizeRawJSON(schedule.Operation.Args)
	if !json.Valid(schedule.Operation.Args) {
		return schedule, errors.New("invalid operation arguments")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return schedule, err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return schedule, err
	}
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return schedule, err
	}
	for _, device := range schedule.Operation.DeviceIDs {
		if err = fleetDeviceAccess(ctx, tx, userID, device, admin); err != nil {
			return schedule, err
		}
	}
	if err = permissionAllowed(ctx, tx, userID, "", CommandPermission(schedule.Operation.Type), admin); err != nil {
		return schedule, err
	}
	if newRequest {
		var title, operation, timezone, days, next string
		var minute, window, enabled int
		err := tx.QueryRowContext(ctx, "SELECT title,operation_json,timezone,weekdays_json,minute_of_day,window_minutes,enabled,next_run_at FROM fleet_schedules WHERE id=? AND user_id=?", schedule.ID, userID).Scan(&title, &operation, &timezone, &days, &minute, &window, &enabled, &next)
		if err == nil {
			op, _ := json.Marshal(schedule.Operation)
			weekdays, _ := json.Marshal(schedule.Weekdays)
			if title != schedule.Title || operation != string(op) || timezone != schedule.Timezone || days != string(weekdays) || minute != schedule.MinuteOfDay || window != schedule.WindowMinutes || (enabled != 0) != schedule.Enabled {
				return schedule, ErrFleetConflict
			}
			schedule.UserID = userID
			schedule.NextRunAt = next
			return schedule, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return schedule, err
		}
	}
	if schedule.ID == "" || newRequest {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fleet_schedules WHERE user_id=?", userID).Scan(&count); err != nil {
			return schedule, err
		}
		if count >= 100 {
			return schedule, errors.New("too many schedules")
		}
		if schedule.ID == "" {
			schedule.ID, err = randomID("schedule")
			if err != nil {
				return schedule, err
			}
		}
	} else if !newRequest {
		var owner string
		if err = tx.QueryRowContext(ctx, "SELECT user_id FROM fleet_schedules WHERE id=?", schedule.ID).Scan(&owner); err != nil || owner != userID {
			return schedule, ErrFleetAccess
		}
	}
	schedule.UserID = userID
	schedule.NextRunAt = next.Format(time.RFC3339Nano)
	operation, _ := json.Marshal(schedule.Operation)
	days, _ := json.Marshal(schedule.Weekdays)
	_, err = tx.ExecContext(ctx, `INSERT INTO fleet_schedules (id,user_id,title,operation_json,timezone,weekdays_json,minute_of_day,window_minutes,enabled,next_run_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,operation_json=excluded.operation_json,timezone=excluded.timezone,weekdays_json=excluded.weekdays_json,minute_of_day=excluded.minute_of_day,window_minutes=excluded.window_minutes,enabled=excluded.enabled,next_run_at=excluded.next_run_at,updated_at=excluded.updated_at`, schedule.ID, userID, schedule.Title, string(operation), schedule.Timezone, string(days), schedule.MinuteOfDay, schedule.WindowMinutes, schedule.Enabled, schedule.NextRunAt, nowText(), nowText())
	if err != nil {
		return schedule, err
	}
	// Editing disables reservations from the old definition. Started operations remain visible.
	if _, err = tx.ExecContext(ctx, "UPDATE fleet_schedule_runs SET status='cancelled',error='schedule changed' WHERE schedule_id=? AND status='pending'", schedule.ID); err != nil {
		return schedule, err
	}
	return schedule, tx.Commit()
}

func (s *Store) ListFleetSchedules(ctx context.Context, userID string) ([]FleetSchedule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,user_id,title,operation_json,timezone,weekdays_json,minute_of_day,window_minutes,enabled,next_run_at FROM fleet_schedules WHERE user_id=? ORDER BY title", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FleetSchedule{}
	for rows.Next() {
		var item FleetSchedule
		var op, days string
		var enabled int
		if err = rows.Scan(&item.ID, &item.UserID, &item.Title, &op, &item.Timezone, &days, &item.MinuteOfDay, &item.WindowMinutes, &enabled, &item.NextRunAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(op), &item.Operation); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(days), &item.Weekdays); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		result = append(result, item)
	}
	return result, rows.Err()
}

// Reservations and the next calendar slot commit together. The frozen run payload
// and its request key make a crash between command creation and acknowledgement safe.
func (s *Store) reserveFleetSchedules(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fleetLock(ctx, tx); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,operation_json,timezone,weekdays_json,minute_of_day,window_minutes,next_run_at FROM fleet_schedules WHERE enabled=1 AND julianday(next_run_at)<=julianday(?) ORDER BY next_run_at LIMIT 100", now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	type dueSchedule struct {
		id, operation, timezone, days, slot string
		minute, window                      int
	}
	due := []dueSchedule{}
	for rows.Next() {
		var item dueSchedule
		if err = rows.Scan(&item.id, &item.operation, &item.timezone, &item.days, &item.minute, &item.window, &item.slot); err != nil {
			rows.Close()
			return err
		}
		due = append(due, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range due {
		slot, err := time.Parse(time.RFC3339Nano, item.slot)
		if err != nil {
			return err
		}
		var days []int
		if err = json.Unmarshal([]byte(item.days), &days); err != nil {
			return err
		}
		next, err := nextFleetSlot(now, item.timezone, days, item.minute)
		if err != nil {
			return err
		}
		deadline := slot.Add(time.Duration(item.window) * time.Minute)
		runID, err := randomID("schedule-run")
		if err != nil {
			return err
		}
		var op FleetOperationInput
		if err = json.Unmarshal([]byte(item.operation), &op); err != nil {
			return err
		}
		op.RequestKey = "schedule:" + runID
		op.DeadlineAt = deadline.Format(time.RFC3339Nano)
		frozen, err := json.Marshal(op)
		if err != nil {
			return err
		}
		status := "pending"
		if !deadline.After(now) {
			status = "missed"
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO fleet_schedule_runs (id,schedule_id,scheduled_at,deadline_at,operation_json,status,created_at) VALUES (?,?,?,?,?,?,?)", runID, item.id, item.slot, op.DeadlineAt, string(frozen), status, nowText()); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE fleet_schedules SET next_run_at=?,updated_at=? WHERE id=?", next.Format(time.RFC3339Nano), nowText(), item.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AdvanceFleetSchedules(ctx context.Context) error {
	if err := s.reserveFleetSchedules(ctx, time.Now().UTC()); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT r.id,s.user_id,r.operation_json,r.deadline_at FROM fleet_schedule_runs r JOIN fleet_schedules s ON s.id=r.schedule_id WHERE r.status='pending' AND s.enabled=1 ORDER BY r.created_at LIMIT 100")
	if err != nil {
		return err
	}
	type pendingRun struct{ id, user, operation, deadline string }
	pending := []pendingRun{}
	for rows.Next() {
		var item pendingRun
		if err = rows.Scan(&item.id, &item.user, &item.operation, &item.deadline); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, run := range pending {
		var input FleetOperationInput
		if err = json.Unmarshal([]byte(run.operation), &input); err != nil {
			return err
		}
		input.ScheduleRunID = run.id
		deadline, err := time.Parse(time.RFC3339Nano, run.deadline)
		if err != nil {
			return err
		}
		status, message, operationID := "queued", "", ""
		if !deadline.After(time.Now()) {
			status = "missed"
		} else {
			operation, createErr := s.CreateFleetOperation(ctx, run.user, input)
			if createErr != nil {
				// Database and cancellation failures retry without consuming the reservation.
				var sqlErr interface{ SQLState() string }
				if errors.As(createErr, &sqlErr) || errors.Is(createErr, context.Canceled) || errors.Is(createErr, context.DeadlineExceeded) || errors.Is(createErr, sql.ErrConnDone) {
					return createErr
				}
				status = "failed"
				message = "operation unavailable; verify device access, capabilities and limits"
			} else {
				operationID = operation.ID
			}
		}
		if _, err = s.db.ExecContext(ctx, "UPDATE fleet_schedule_runs SET status=?,error=?,operation_id=? WHERE id=? AND status='pending'", status, message, operationID, run.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListFleetScheduleRuns(ctx context.Context, userID string) ([]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT r.id,r.schedule_id,r.scheduled_at,r.deadline_at,r.operation_id,r.status,r.error FROM fleet_schedule_runs r JOIN fleet_schedules s ON s.id=r.schedule_id WHERE s.user_id=? ORDER BY r.created_at DESC LIMIT 100", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var id, schedule, slot, deadline, op, status, message string
		if err = rows.Scan(&id, &schedule, &slot, &deadline, &op, &status, &message); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{"id": id, "schedule_id": schedule, "scheduled_at": slot, "deadline_at": deadline, "operation_id": op, "status": status, "error": message})
	}
	return result, rows.Err()
}
