CREATE TABLE fleet_operations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    request_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    title TEXT NOT NULL,
    command_type TEXT NOT NULL,
    args_json TEXT NOT NULL,
    parallelism INTEGER NOT NULL CHECK (parallelism BETWEEN 1 AND 20),
    stop_on_failure INTEGER NOT NULL DEFAULT 0,
    canary_pending INTEGER NOT NULL DEFAULT 0,
    deadline_at TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX fleet_operations_request_uq ON fleet_operations(user_id, request_key);
CREATE TABLE fleet_operation_items (
    operation_id TEXT NOT NULL REFERENCES fleet_operations(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    command_id TEXT NOT NULL DEFAULT '',
    args_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (operation_id, device_id)
);
CREATE INDEX fleet_operations_status_idx ON fleet_operations(status, created_at);
CREATE INDEX fleet_operation_items_status_idx ON fleet_operation_items(operation_id, status);
CREATE TABLE fleet_assets (
    device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    model TEXT NOT NULL DEFAULT '',
    serial_number TEXT NOT NULL DEFAULT '',
    site TEXT NOT NULL DEFAULT '',
    responsible TEXT NOT NULL DEFAULT '',
    warranty_until TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE TABLE fleet_schedules (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    operation_json TEXT NOT NULL,
    timezone TEXT NOT NULL,
    weekdays_json TEXT NOT NULL,
    minute_of_day INTEGER NOT NULL,
    window_minutes INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    next_run_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE fleet_schedule_runs (
    id TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL REFERENCES fleet_schedules(id) ON DELETE CASCADE,
    scheduled_at TEXT NOT NULL,
    deadline_at TEXT NOT NULL,
    operation_json TEXT NOT NULL,
    operation_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX fleet_schedule_runs_slot_uq ON fleet_schedule_runs(schedule_id, scheduled_at);
CREATE TABLE fleet_access_policies (
 device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
 ssh_allowed INTEGER NOT NULL,
 luci_allowed INTEGER NOT NULL,
 max_ttl_seconds INTEGER NOT NULL,
 restrict_users INTEGER NOT NULL,
 user_ids_json TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE fleet_profiles (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 title TEXT NOT NULL,
 config TEXT NOT NULL,
 definition_encrypted TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE fleet_rules (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 title TEXT NOT NULL,
 definition_json TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE fleet_rule_states (
 rule_id TEXT NOT NULL REFERENCES fleet_rules(id) ON DELETE CASCADE,
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 last_sample TEXT NOT NULL DEFAULT '',
 violations INTEGER NOT NULL DEFAULT 0,
 next_action_at TEXT NOT NULL DEFAULT '',
 action_day TEXT NOT NULL DEFAULT '',
 action_count INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY (rule_id,device_id)
);
CREATE TABLE fleet_rule_events (
 id TEXT PRIMARY KEY,
 rule_id TEXT NOT NULL REFERENCES fleet_rules(id) ON DELETE CASCADE,
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 action TEXT NOT NULL,
 status TEXT NOT NULL,
 command_id TEXT NOT NULL DEFAULT '',
 message TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
