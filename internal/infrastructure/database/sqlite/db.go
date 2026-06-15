package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
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

	rows, err := db.QueryContext(ctx, `PRAGMA table_info(monitor_groups)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasIcon := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "icon" {
			hasIcon = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasIcon {
		if _, err := db.ExecContext(ctx, `ALTER TABLE monitor_groups ADD COLUMN icon TEXT NOT NULL DEFAULT 'Monitor'`); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO system_settings
			(setting_key, setting_value, value_type, description)
		VALUES
			('app_timezone', 'Asia/Shanghai', 'string', 'Time zone used for all displayed timestamps.')
	`)
	return err
}
