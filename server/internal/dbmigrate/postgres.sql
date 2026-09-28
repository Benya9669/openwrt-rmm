CREATE TABLE schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    checksum TEXT NOT NULL
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    disabled SMALLINT NOT NULL DEFAULT 0 CHECK (disabled IN (0, 1)),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX users_username_lower_uq ON users (lower(username));
CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email)) WHERE email <> '';

CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL UNIQUE,
    token_hash TEXT NOT NULL DEFAULT '',
    next_token_hash TEXT NOT NULL DEFAULT '',
    next_token_ciphertext TEXT NOT NULL DEFAULT '',
    token_epoch INTEGER NOT NULL DEFAULT 1,
    tunnel_public_key TEXT NOT NULL DEFAULT '',
    tunnel_key_fingerprint TEXT NOT NULL DEFAULT '',
    tunnel_key_epoch INTEGER NOT NULL DEFAULT 1,
    tunnel_credential_updated_at TIMESTAMPTZ,
    owner_user_id TEXT NOT NULL DEFAULT '',
    dns_label TEXT NOT NULL DEFAULT '',
    hostname TEXT NOT NULL,
    openwrt_version TEXT NOT NULL,
    inventory_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    group_name TEXT NOT NULL DEFAULT '',
    tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX devices_dns_label_uq ON devices (dns_label) WHERE dns_label <> '';
CREATE UNIQUE INDEX devices_tunnel_fingerprint_uq ON devices (tunnel_key_fingerprint)
    WHERE tunnel_key_fingerprint <> '';
CREATE INDEX devices_owner_created_idx ON devices (owner_user_id, created_at);

CREATE TABLE commands (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id),
    type TEXT NOT NULL,
    args_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    output TEXT NOT NULL DEFAULT '',
    exit_code INTEGER,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    claimed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    nonce TEXT NOT NULL DEFAULT '',
    signature_key_id TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT ''
);
CREATE INDEX commands_device_status_idx ON commands (device_id, status);
CREATE INDEX commands_device_created_idx ON commands (device_id, created_at);

CREATE TABLE device_backups (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    command_id TEXT NOT NULL UNIQUE REFERENCES commands(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    archive_ciphertext BYTEA,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    openwrt_version TEXT NOT NULL DEFAULT '',
    target TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    manifest_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);
CREATE INDEX device_backups_device_created_idx ON device_backups (device_id, created_at DESC);
CREATE UNIQUE INDEX device_backups_one_creating_uq ON device_backups (device_id)
    WHERE status = 'creating';

CREATE TABLE security_metadata (
    name TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE agent_rollouts (
    id TEXT PRIMARY KEY,
    channel TEXT NOT NULL,
    target_version TEXT NOT NULL,
    batch_size INTEGER NOT NULL,
    failure_threshold INTEGER NOT NULL,
    status TEXT NOT NULL,
    failure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE agent_rollout_devices (
    rollout_id TEXT NOT NULL REFERENCES agent_rollouts(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    feed_url TEXT NOT NULL,
    package_manager TEXT NOT NULL,
    package_version TEXT NOT NULL,
    manifest_url TEXT NOT NULL DEFAULT '',
    signature_url TEXT NOT NULL DEFAULT '',
    batch INTEGER NOT NULL,
    status TEXT NOT NULL,
    command_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (rollout_id, device_id)
);
CREATE INDEX agent_rollout_devices_command_idx ON agent_rollout_devices (command_id);

CREATE TABLE metric_samples (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id),
    inventory_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX metric_samples_device_created_idx ON metric_samples (device_id, created_at);

CREATE TABLE alerts (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id),
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX alerts_device_status_idx ON alerts (device_id, status);
CREATE INDEX alerts_status_last_seen_idx ON alerts (status, last_seen_at);

CREATE TABLE notification_settings (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (email_enabled IN (0, 1)),
    telegram_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (telegram_enabled IN (0, 1)),
    telegram_chat_id TEXT NOT NULL DEFAULT '',
    notify_warning SMALLINT NOT NULL DEFAULT 1 CHECK (notify_warning IN (0, 1)),
    notify_critical SMALLINT NOT NULL DEFAULT 1 CHECK (notify_critical IN (0, 1)),
    notify_resolved SMALLINT NOT NULL DEFAULT 1 CHECK (notify_resolved IN (0, 1)),
    memory_threshold_percent INTEGER NOT NULL DEFAULT 85,
    disk_threshold_percent INTEGER NOT NULL DEFAULT 85,
    packet_loss_percent INTEGER NOT NULL DEFAULT 20,
    latency_threshold_ms INTEGER NOT NULL DEFAULT 200,
    repeat_minutes INTEGER NOT NULL DEFAULT 0,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    quiet_hours_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (quiet_hours_enabled IN (0, 1)),
    quiet_hours_start TEXT NOT NULL DEFAULT '22:00',
    quiet_hours_end TEXT NOT NULL DEFAULT '08:00',
    alerts_paused_until TIMESTAMPTZ,
    webhook_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (webhook_enabled IN (0, 1)),
    webhook_url TEXT NOT NULL DEFAULT '',
    webhook_secret TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE notification_deliveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT REFERENCES devices(id) ON DELETE CASCADE,
    alert_id TEXT NOT NULL DEFAULT '',
    dedupe_key TEXT NOT NULL UNIQUE,
    event TEXT NOT NULL,
    channel TEXT NOT NULL,
    status TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    destination TEXT NOT NULL,
    destination_masked TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMPTZ NOT NULL,
    last_attempt_at TIMESTAMPTZ,
    next_attempt_at TEXT NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX notification_deliveries_user_created_idx ON notification_deliveries (user_id, created_at DESC);
CREATE INDEX notification_deliveries_status_created_idx ON notification_deliveries (status, created_at);
CREATE INDEX notification_deliveries_ready_idx ON notification_deliveries (status, next_attempt_at, lease_expires_at);

CREATE TABLE device_notification_settings (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    enabled SMALLINT NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    notify_warning SMALLINT NOT NULL DEFAULT 1 CHECK (notify_warning IN (0, 1)),
    notify_critical SMALLINT NOT NULL DEFAULT 1 CHECK (notify_critical IN (0, 1)),
    notify_resolved SMALLINT NOT NULL DEFAULT 1 CHECK (notify_resolved IN (0, 1)),
    paused_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, device_id)
);

CREATE TABLE inbox_notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT REFERENCES devices(id) ON DELETE CASCADE,
    incident_id TEXT NOT NULL DEFAULT '',
    dedupe_key TEXT NOT NULL UNIQUE,
    severity TEXT NOT NULL,
    event TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX inbox_notifications_user_created_idx ON inbox_notifications (user_id, created_at DESC);
CREATE INDEX inbox_notifications_user_unread_idx ON inbox_notifications (user_id, read_at);

CREATE TABLE contact_verifications (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    destination TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, channel)
);

CREATE TABLE lan_clients (
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    client_key TEXT NOT NULL,
    mac TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    hostname TEXT NOT NULL DEFAULT '',
    interface TEXT NOT NULL DEFAULT '',
    connection TEXT NOT NULL DEFAULT '',
    confirmation TEXT NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (device_id, client_key)
);
CREATE INDEX lan_clients_device_seen_idx ON lan_clients (device_id, last_seen_at DESC);

CREATE TABLE remote_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id),
    target TEXT NOT NULL,
    status TEXT NOT NULL,
    server_host TEXT NOT NULL DEFAULT '',
    server_port INTEGER NOT NULL DEFAULT 22,
    remote_port INTEGER NOT NULL DEFAULT 0,
    luci_port INTEGER NOT NULL DEFAULT 0,
    luci_scheme TEXT NOT NULL DEFAULT 'http',
    local_host TEXT NOT NULL DEFAULT '127.0.0.1',
    local_port INTEGER NOT NULL DEFAULT 22,
    command_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX remote_sessions_device_created_idx ON remote_sessions (device_id, created_at);
CREATE INDEX remote_sessions_status_expires_idx ON remote_sessions (status, expires_at);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    device_id TEXT NOT NULL DEFAULT '',
    command_id TEXT NOT NULL DEFAULT '',
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX audit_events_device_created_idx ON audit_events (device_id, created_at);

CREATE TABLE operator_sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX operator_sessions_user_expires_idx ON operator_sessions (user_id, expires_at);

CREATE TABLE password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX password_reset_tokens_user_expires_idx ON password_reset_tokens (user_id, expires_at);

CREATE TABLE enrollment_grants (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL REFERENCES users(id),
    dns_label TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX enrollment_grants_user_created_idx ON enrollment_grants (user_id, created_at);

CREATE TABLE device_dns_records (
    device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    ipv4 TEXT NOT NULL DEFAULT '',
    ipv6 TEXT NOT NULL DEFAULT '',
    ttl INTEGER NOT NULL DEFAULT 60 CHECK (ttl BETWEEN 30 AND 86400),
    enabled SMALLINT NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_agent_update_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE dns_address_history (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    ipv4 TEXT NOT NULL DEFAULT '',
    ipv6 TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX dns_address_history_device_created_idx ON dns_address_history (device_id, created_at DESC);

CREATE TABLE device_access_grants (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    device_id TEXT NOT NULL REFERENCES devices(id),
    remote_session_id TEXT NOT NULL REFERENCES remote_sessions(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE device_access_sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    device_id TEXT NOT NULL REFERENCES devices(id),
    remote_session_id TEXT NOT NULL REFERENCES remote_sessions(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX device_access_sessions_device_expires_idx ON device_access_sessions (device_id, expires_at);
