package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path
	if !strings.Contains(path, "?") {
		dsn = path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, stmt := range pragmas {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply sqlite pragma %q: %w", stmt, err)
		}
	}
	if err := migrateCompat(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func migrateCompat(ctx context.Context, db *sql.DB) error {
	var exists int
	err := db.QueryRowContext(ctx, `
		SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'monitor_groups' LIMIT 1
	`).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	// 1. Check monitor_groups for icon and response_settings_json
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(monitor_groups)`)
	if err != nil {
		return err
	}
	hasIcon := false
	hasGroupResponseSettings := false
	hasSortOrder := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "icon" {
			hasIcon = true
		}
		if name == "response_settings_json" {
			hasGroupResponseSettings = true
		}
		if name == "sort_order" {
			hasSortOrder = true
		}
	}
	rows.Close()

	if !hasIcon {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_groups ADD COLUMN icon TEXT NOT NULL DEFAULT 'Monitor'`); err != nil {
			return err
		}
	}
	if !hasGroupResponseSettings {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_groups ADD COLUMN response_settings_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !hasSortOrder {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_groups ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}

	// 2. Check monitor_items for response_settings_json and ref_item_id
	rowsItems, err := db.QueryContext(ctx, `PRAGMA table_info(monitor_items)`)
	if err != nil {
		return err
	}
	hasItemResponseSettings := false
	hasRefItemID := false
	for rowsItems.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue, pk interface{}
		if err := rowsItems.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rowsItems.Close()
			return err
		}
		if name == "response_settings_json" {
			hasItemResponseSettings = true
		}
		if name == "ref_item_id" {
			hasRefItemID = true
		}
	}
	rowsItems.Close()

	if !hasItemResponseSettings {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_items ADD COLUMN response_settings_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !hasRefItemID {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_items ADD COLUMN ref_item_id INTEGER DEFAULT NULL`); err != nil {
			return err
		}
	}

	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO system_settings
			(setting_key, setting_value, value_type, description)
		VALUES
			('app_timezone', 'Asia/Shanghai', 'string', 'Time zone used for all displayed timestamps.')
	`)
	if err != nil {
		return err
	}

	// 3. Migrate monitor_field_definitions (add ref_group_id, ref_name_path, and update value_type CHECK)
	var hasRefGroupID bool
	rowsFields, err := db.QueryContext(ctx, `PRAGMA table_info(monitor_field_definitions)`)
	if err != nil {
		return err
	}
	for rowsFields.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue, pk interface{}
		if err := rowsFields.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err == nil {
			if name == "ref_group_id" {
				hasRefGroupID = true
			}
		}
	}
	rowsFields.Close()

	if !hasRefGroupID {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			CREATE TABLE monitor_field_definitions_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				scope_type TEXT NOT NULL CHECK (scope_type IN ('group', 'item')),
				group_id INTEGER NOT NULL,
				item_id INTEGER,
				field_path TEXT NOT NULL,
				display_name TEXT NOT NULL DEFAULT '',
				value_type TEXT NOT NULL CHECK (value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
				unit TEXT NOT NULL DEFAULT '',
				required INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
				enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
				ref_group_id INTEGER,
				ref_name_path TEXT,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deleted_at TEXT,
				CHECK ((scope_type = 'group' AND item_id IS NULL) OR (scope_type = 'item' AND item_id IS NOT NULL)),
				FOREIGN KEY (group_id) REFERENCES monitor_groups (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (item_id) REFERENCES monitor_items (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (ref_group_id) REFERENCES monitor_groups (id) ON UPDATE CASCADE ON DELETE SET NULL
			)
		`)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO monitor_field_definitions_new (
				id, scope_type, group_id, item_id, field_path, display_name, value_type, unit, required, enabled, created_at, updated_at, deleted_at
			)
			SELECT id, scope_type, group_id, item_id, field_path, display_name, value_type, unit, required, enabled, created_at, updated_at, deleted_at
			FROM monitor_field_definitions
		`)
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `DROP TABLE monitor_field_definitions`); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `ALTER TABLE monitor_field_definitions_new RENAME TO monitor_field_definitions`); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_group_path_active ON monitor_field_definitions (group_id, field_path) WHERE scope_type = 'group' AND deleted_at IS NULL`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_item_path_active ON monitor_field_definitions (item_id, field_path) WHERE scope_type = 'item' AND deleted_at IS NULL`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_field_defs_lookup ON monitor_field_definitions (group_id, item_id, enabled, deleted_at)`); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// 4. Migrate monitor_sample_values (update value_type CHECK)
	var sampleValuesSQL string
	_ = db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'monitor_sample_values'`).Scan(&sampleValuesSQL)
	if !strings.Contains(sampleValuesSQL, "string_array") {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			CREATE TABLE monitor_sample_values_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				sample_id INTEGER NOT NULL,
				group_id INTEGER NOT NULL,
				item_id INTEGER NOT NULL,
				field_definition_id INTEGER,
				field_path TEXT NOT NULL,
				value_type TEXT NOT NULL CHECK (value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
				string_value TEXT,
				integer_value INTEGER,
				float_value REAL,
				boolean_value INTEGER CHECK (boolean_value IS NULL OR boolean_value IN (0, 1)),
				numeric_value REAL,
				raw_value TEXT,
				received_at TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (sample_id) REFERENCES monitor_samples (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (group_id) REFERENCES monitor_groups (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (item_id) REFERENCES monitor_items (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (field_definition_id) REFERENCES monitor_field_definitions (id) ON UPDATE CASCADE ON DELETE SET NULL
			)
		`)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO monitor_sample_values_new SELECT * FROM monitor_sample_values`)
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `DROP TABLE monitor_sample_values`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `ALTER TABLE monitor_sample_values_new RENAME TO monitor_sample_values`); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// 5. Migrate alert_rules (update value_type and operator CHECKs)
	var alertRulesSQL string
	_ = db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'alert_rules'`).Scan(&alertRulesSQL)
	if !strings.Contains(alertRulesSQL, "string_array") || !strings.Contains(alertRulesSQL, "len_eq") {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			CREATE TABLE alert_rules_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				scope_type TEXT NOT NULL CHECK (scope_type IN ('global', 'group', 'item', 'field')),
				group_id INTEGER,
				item_id INTEGER,
				field_definition_id INTEGER,
				source_type TEXT NOT NULL DEFAULT 'any' CHECK (source_type IN ('any', 'passive', 'active')),
				rule_type TEXT NOT NULL CHECK (rule_type IN ('missing_data', 'request_failed', 'field_condition', 'aggregate_condition')),
				field_path TEXT,
				value_type TEXT CHECK (value_type IS NULL OR value_type IN ('string', 'integer', 'float', 'boolean', 'object_array', 'string_array')),
				operator TEXT CHECK (
					operator IS NULL
					OR operator IN ('gt', 'gte', 'lt', 'lte', 'eq', 'ne', 'contains', 'not_contains', 'exists', 'not_exists', 'len_eq', 'len_gt', 'len_lt', 'len_ne')
				),
				threshold_value TEXT,
				aggregate_func TEXT CHECK (aggregate_func IS NULL OR aggregate_func IN ('avg', 'max', 'min', 'median', 'count')),
				aggregate_window_seconds INTEGER CHECK (aggregate_window_seconds IS NULL OR aggregate_window_seconds > 0),
				aggregate_sample_count INTEGER CHECK (aggregate_sample_count IS NULL OR aggregate_sample_count > 0),
				consecutive_count INTEGER NOT NULL DEFAULT 1 CHECK (consecutive_count > 0),
				recovery_count INTEGER NOT NULL DEFAULT 1 CHECK (recovery_count > 0),
				severity TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('info', 'warning', 'critical')),
				message_template TEXT NOT NULL DEFAULT '',
				enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
				created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deleted_at TEXT,
				FOREIGN KEY (group_id) REFERENCES monitor_groups (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (item_id) REFERENCES monitor_items (id) ON UPDATE CASCADE ON DELETE CASCADE,
				FOREIGN KEY (field_definition_id) REFERENCES monitor_field_definitions (id) ON UPDATE CASCADE ON DELETE CASCADE
			)
		`)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO alert_rules_new SELECT * FROM alert_rules`)
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `DROP TABLE alert_rules`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `ALTER TABLE alert_rules_new RENAME TO alert_rules`); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// 6. Check alert_rules for combine_group column
	rowsRules, err := db.QueryContext(ctx, `PRAGMA table_info(alert_rules)`)
	if err != nil {
		return err
	}
	hasCombineGroup := false
	for rowsRules.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue, pk interface{}
		if err := rowsRules.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			rowsRules.Close()
			return err
		}
		if name == "combine_group" {
			hasCombineGroup = true
		}
	}
	rowsRules.Close()

	if !hasCombineGroup {
		if _, err := db.ExecContext(ctx, `ALTER TABLE alert_rules ADD COLUMN combine_group TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}

	// 7. Create index idx_sample_values_item_field_time to optimize WindowValues query
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_sample_values_item_field_time
		ON monitor_sample_values (item_id, field_path, received_at)
	`); err != nil {
		return err
	}

	// 8. Create index idx_sample_values_field_time to optimize stats queries
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_sample_values_field_time
		ON monitor_sample_values (group_id, item_id, field_path, received_at)
	`); err != nil {
		return err
	}

	// 9. Create index idx_alert_rules_group_id to optimize group-scoped rule queries
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_alert_rules_group_id
		ON alert_rules (group_id, deleted_at)
	`); err != nil {
		return err
	}

	// 10. Create index idx_alert_events_target_time to optimize group events queries
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_alert_events_target_time
		ON alert_events (group_id, item_id, occurred_at)
	`); err != nil {
		return err
	}

	return nil
}
