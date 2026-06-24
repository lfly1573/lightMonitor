-- lightMonitor initial SQLite schema.
-- Time fields are stored as UTC-compatible TEXT timestamps so they keep
-- lexical time order and can be cleaned with simple range predicates.

PRAGMA foreign_keys = ON;

BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS system_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    setting_key TEXT NOT NULL UNIQUE,
    setting_value TEXT NOT NULL,
    value_type TEXT NOT NULL DEFAULT 'string'
        CHECK (value_type IN ('string', 'integer', 'float', 'boolean', 'json')),
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer'
        CHECK (role IN ('admin', 'viewer')),
    display_name TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_login_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS monitor_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT 'Monitor',
    description TEXT NOT NULL DEFAULT '',
    default_interval_seconds INTEGER NOT NULL DEFAULT 60
        CHECK (default_interval_seconds > 0),
    missed_times_threshold INTEGER NOT NULL DEFAULT 3
        CHECK (missed_times_threshold > 0),
    alert_enabled INTEGER NOT NULL DEFAULT 1 CHECK (alert_enabled IN (0, 1)),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    response_settings_json TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_groups_code_active
    ON monitor_groups (code)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_monitor_groups_enabled
    ON monitor_groups (enabled, deleted_at);

CREATE TABLE IF NOT EXISTS monitor_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    source_type TEXT NOT NULL
        CHECK (source_type IN ('passive', 'active')),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (interval_seconds > 0),
    missed_times_threshold INTEGER NOT NULL DEFAULT 3
        CHECK (missed_times_threshold > 0),
    alert_enabled INTEGER NOT NULL DEFAULT 1 CHECK (alert_enabled IN (0, 1)),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    response_settings_json TEXT NOT NULL DEFAULT '{}',
    ref_item_id INTEGER DEFAULT NULL,
    last_seen_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (ref_item_id) REFERENCES monitor_items (id)
        ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_items_group_source_name_active
    ON monitor_items (group_id, source_type, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_monitor_items_group_enabled
    ON monitor_items (group_id, enabled, deleted_at);

CREATE TABLE IF NOT EXISTS active_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'GET'
        CHECK (method IN ('GET', 'POST')),
    headers_json TEXT NOT NULL DEFAULT '{}',
    body_type TEXT NOT NULL DEFAULT 'none'
        CHECK (body_type IN ('none', 'json', 'form-data')),
    body_json TEXT NOT NULL DEFAULT '{}',
    interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (interval_seconds > 0),
    timeout_seconds INTEGER NOT NULL DEFAULT 10 CHECK (timeout_seconds > 0),
    expected_status_code INTEGER NOT NULL DEFAULT 200,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_active_requests_schedule
    ON active_requests (enabled, interval_seconds, deleted_at);

CREATE TABLE IF NOT EXISTS monitor_field_definitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scope_type TEXT NOT NULL
        CHECK (scope_type IN ('group', 'item')),
    group_id INTEGER NOT NULL,
    item_id INTEGER,
    field_path TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    value_type TEXT NOT NULL
        CHECK (value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
    unit TEXT NOT NULL DEFAULT '',
    required INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    ref_group_id INTEGER,
    ref_name_path TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    CHECK (
        (scope_type = 'group' AND item_id IS NULL)
        OR (scope_type = 'item' AND item_id IS NOT NULL)
    ),
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (ref_group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_group_path_active
    ON monitor_field_definitions (group_id, field_path)
    WHERE scope_type = 'group' AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_item_path_active
    ON monitor_field_definitions (item_id, field_path)
    WHERE scope_type = 'item' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_field_defs_lookup
    ON monitor_field_definitions (group_id, item_id, enabled, deleted_at);

CREATE TABLE IF NOT EXISTS monitor_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    source_type TEXT NOT NULL
        CHECK (source_type IN ('passive', 'active')),
    active_request_id INTEGER,
    name TEXT NOT NULL,
    reported_at TEXT,
    received_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    interval_seconds INTEGER CHECK (interval_seconds IS NULL OR interval_seconds > 0),
    status TEXT NOT NULL DEFAULT 'ok'
        CHECK (status IN ('ok', 'error', 'missing', 'type_error')),
    http_status_code INTEGER,
    latency_ms INTEGER CHECK (latency_ms IS NULL OR latency_ms >= 0),
    raw_json TEXT NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (active_request_id) REFERENCES active_requests (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_samples_received_at
    ON monitor_samples (received_at);

CREATE INDEX IF NOT EXISTS idx_samples_item_time
    ON monitor_samples (group_id, item_id, received_at);

CREATE INDEX IF NOT EXISTS idx_samples_status_time
    ON monitor_samples (status, received_at);

CREATE TABLE IF NOT EXISTS monitor_sample_values (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sample_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    field_definition_id INTEGER,
    field_path TEXT NOT NULL,
    value_type TEXT NOT NULL
        CHECK (value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
    string_value TEXT,
    integer_value INTEGER,
    float_value REAL,
    boolean_value INTEGER CHECK (boolean_value IS NULL OR boolean_value IN (0, 1)),
    numeric_value REAL,
    raw_value TEXT,
    received_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (sample_id) REFERENCES monitor_samples (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (field_definition_id) REFERENCES monitor_field_definitions (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sample_values_field_time
    ON monitor_sample_values (group_id, item_id, field_path, received_at);

CREATE INDEX IF NOT EXISTS idx_sample_values_item_field_time
    ON monitor_sample_values (item_id, field_path, received_at);

CREATE INDEX IF NOT EXISTS idx_sample_values_numeric_time
    ON monitor_sample_values (group_id, item_id, field_path, numeric_value, received_at);

CREATE TABLE IF NOT EXISTS monitor_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    field_path TEXT NOT NULL,
    bucket_type TEXT NOT NULL
        CHECK (bucket_type IN ('minute', 'hour', 'day')),
    bucket_start_at TEXT NOT NULL,
    bucket_end_at TEXT NOT NULL,
    sample_count INTEGER NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
    avg_value REAL,
    max_value REAL,
    min_value REAL,
    median_value REAL,
    last_value REAL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_statistics_bucket
    ON monitor_statistics (group_id, item_id, field_path, bucket_type, bucket_start_at);

CREATE INDEX IF NOT EXISTS idx_statistics_cleanup
    ON monitor_statistics (bucket_end_at);

CREATE TABLE IF NOT EXISTS notification_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    config_json TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_channels_code_active
    ON notification_channels (code)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_channels_enabled
    ON notification_channels (enabled, deleted_at);

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    scope_type TEXT NOT NULL
        CHECK (scope_type IN ('global', 'group', 'item', 'field')),
    group_id INTEGER,
    item_id INTEGER,
    field_definition_id INTEGER,
    source_type TEXT NOT NULL DEFAULT 'any'
        CHECK (source_type IN ('any', 'passive', 'active')),
    rule_type TEXT NOT NULL
        CHECK (rule_type IN ('missing_data', 'request_failed', 'field_condition', 'aggregate_condition')),
    field_path TEXT,
    value_type TEXT
        CHECK (value_type IS NULL OR value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
    operator TEXT
        CHECK (
            operator IS NULL
            OR operator IN ('gt', 'gte', 'lt', 'lte', 'eq', 'ne', 'contains', 'not_contains', 'exists', 'not_exists', 'len_eq', 'len_gt', 'len_lt', 'len_ne')
        ),
    threshold_value TEXT,
    aggregate_func TEXT
        CHECK (aggregate_func IS NULL OR aggregate_func IN ('avg', 'max', 'min', 'median', 'count')),
    aggregate_window_seconds INTEGER
        CHECK (aggregate_window_seconds IS NULL OR aggregate_window_seconds > 0),
    aggregate_sample_count INTEGER
        CHECK (aggregate_sample_count IS NULL OR aggregate_sample_count > 0),
    consecutive_count INTEGER NOT NULL DEFAULT 1 CHECK (consecutive_count > 0),
    recovery_count INTEGER NOT NULL DEFAULT 1 CHECK (recovery_count > 0),
    severity TEXT NOT NULL DEFAULT 'warning'
        CHECK (severity IN ('info', 'warning', 'critical')),
    message_template TEXT NOT NULL DEFAULT '',
    combine_group TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TEXT,
    CHECK (
        (scope_type = 'global' AND group_id IS NULL AND item_id IS NULL AND field_definition_id IS NULL)
        OR (scope_type = 'group' AND group_id IS NOT NULL AND item_id IS NULL AND field_definition_id IS NULL)
        OR (scope_type = 'item' AND group_id IS NOT NULL AND item_id IS NOT NULL AND field_definition_id IS NULL)
        OR (scope_type = 'field' AND group_id IS NOT NULL AND item_id IS NOT NULL)
    ),
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (field_definition_id) REFERENCES monitor_field_definitions (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_scope
    ON alert_rules (scope_type, group_id, item_id, enabled, deleted_at);

CREATE INDEX IF NOT EXISTS idx_alert_rules_type
    ON alert_rules (rule_type, enabled, deleted_at);

CREATE INDEX IF NOT EXISTS idx_alert_rules_group_id
    ON alert_rules (group_id, deleted_at);

CREATE TABLE IF NOT EXISTS alert_rule_channels (
    rule_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (rule_id, channel_id),
    FOREIGN KEY (rule_id) REFERENCES alert_rules (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES notification_channels (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS alert_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    group_id INTEGER,
    item_id INTEGER,
    field_path TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ok'
        CHECK (status IN ('ok', 'alerting')),
    consecutive_hits INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_hits >= 0),
    consecutive_recovers INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_recovers >= 0),
    first_hit_at TEXT,
    last_hit_at TEXT,
    last_alert_at TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rule_id) REFERENCES alert_rules (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_states_unique_target
    ON alert_states (rule_id, group_id, item_id, field_path);

CREATE INDEX IF NOT EXISTS idx_alert_states_status
    ON alert_states (status, updated_at);

CREATE TABLE IF NOT EXISTS alert_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    group_id INTEGER,
    item_id INTEGER,
    sample_id INTEGER,
    event_type TEXT NOT NULL
        CHECK (event_type IN ('triggered', 'recovered')),
    severity TEXT NOT NULL
        CHECK (severity IN ('info', 'warning', 'critical')),
    title TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    field_path TEXT NOT NULL DEFAULT '',
    current_value TEXT,
    threshold_value TEXT,
    occurred_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rule_id) REFERENCES alert_rules (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES monitor_groups (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    FOREIGN KEY (item_id) REFERENCES monitor_items (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    FOREIGN KEY (sample_id) REFERENCES monitor_samples (id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_events_time
    ON alert_events (occurred_at);

CREATE INDEX IF NOT EXISTS idx_alert_events_target_time
    ON alert_events (group_id, item_id, occurred_at);

CREATE TABLE IF NOT EXISTS alert_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    request_json TEXT NOT NULL DEFAULT '{}',
    response_text TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    sent_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (event_id) REFERENCES alert_events (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES notification_channels (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_alert_notifications_status
    ON alert_notifications (status, created_at);

CREATE INDEX IF NOT EXISTS idx_alert_notifications_event
    ON alert_notifications (event_id, channel_id);

INSERT OR IGNORE INTO system_settings
    (setting_key, setting_value, value_type, description)
VALUES
    ('data_retention_days', '30', 'integer', 'Days to keep raw monitor samples and alert logs.'),
    ('upload_token', '', 'string', 'Global token required by passive data receiver.'),
    ('default_locale', 'zh-CN', 'string', 'Default UI language.'),
    ('app_timezone', 'Asia/Shanghai', 'string', 'Time zone used for all displayed timestamps.'),
    ('session_timeout_minutes', '120', 'integer', 'Management session timeout in minutes.');

INSERT OR IGNORE INTO schema_migrations (version, name)
VALUES (1, 'initial_schema');

COMMIT;
