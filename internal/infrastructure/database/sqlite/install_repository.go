package sqlite

import (
	"context"
	"database/sql"
	"errors"

	migrations "lightmonitor/database"
	"lightmonitor/internal/domain/system"
)

type InstallRepository struct {
	db *sql.DB
}

func NewInstallRepository(db *sql.DB) *InstallRepository {
	return &InstallRepository{db: db}
}

func (r *InstallRepository) IsInstalled(ctx context.Context) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1
		FROM sqlite_master
		WHERE type = 'table' AND name = 'schema_migrations'
		LIMIT 1
	`).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var version int
	err = r.db.QueryRowContext(ctx, `
		SELECT version
		FROM schema_migrations
		WHERE version = 1
		LIMIT 1
	`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *InstallRepository) Install(ctx context.Context, admin system.User) error {
	if _, err := r.db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, display_name, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, admin.Username, admin.PasswordHash, admin.Role, admin.Username, 1)
	return err
}
