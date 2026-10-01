CREATE TABLE management_permissions (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 policy_json TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE management_incidents (
 id TEXT PRIMARY KEY,
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 source_key TEXT NOT NULL,
 title TEXT NOT NULL,
 status TEXT NOT NULL,
 assignee_id TEXT NOT NULL DEFAULT '',
 occurrences INTEGER NOT NULL DEFAULT 1,
 opened_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 resolved_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX management_incidents_source_uq ON management_incidents(device_id,source_key);
CREATE TABLE management_incident_events (
 id TEXT PRIMARY KEY,
 incident_id TEXT NOT NULL REFERENCES management_incidents(id) ON DELETE CASCADE,
 actor TEXT NOT NULL,
 action TEXT NOT NULL,
 body TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE INDEX management_incident_events_idx ON management_incident_events(incident_id,created_at);
CREATE TABLE management_incident_sources (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 source_key TEXT NOT NULL,
 category TEXT NOT NULL,
 active INTEGER NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY (device_id,source_key)
);
CREATE TABLE management_rollout_guards (
 rollout_id TEXT PRIMARY KEY REFERENCES agent_rollouts(id) ON DELETE CASCADE,
 definition_json TEXT NOT NULL,
 request_hash TEXT NOT NULL DEFAULT '',
 wave_observed INTEGER NOT NULL DEFAULT 0,
 healthy_since TEXT NOT NULL DEFAULT '',
 last_checked_at TEXT NOT NULL DEFAULT '',
 pause_reason TEXT NOT NULL DEFAULT ''
);
CREATE TABLE management_command_permissions (
 command_id TEXT PRIMARY KEY REFERENCES commands(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 permission TEXT NOT NULL
);
CREATE INDEX management_command_permissions_user_idx ON management_command_permissions(user_id,permission);
