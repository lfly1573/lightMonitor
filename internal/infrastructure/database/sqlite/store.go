package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"lightmonitor/internal/application/core"
	"lightmonitor/internal/domain/system"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Authenticate(ctx context.Context, username string) (system.User, error) {
	var user system.User
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, display_name, enabled
		FROM users
		WHERE username = ? AND deleted_at IS NULL
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.DisplayName, &enabled)
	user.Enabled = enabled == 1
	return user, err
}

func (s *Store) TouchLogin(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	return err
}

func (s *Store) ListSettings(ctx context.Context) ([]core.Setting, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT setting_key, setting_value, value_type, description
		FROM system_settings
		ORDER BY setting_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []core.Setting
	for rows.Next() {
		var setting core.Setting
		if err := rows.Scan(&setting.Key, &setting.Value, &setting.ValueType, &setting.Description); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, rows.Err()
}

func (s *Store) UpdateSettings(ctx context.Context, settings []core.Setting) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, setting := range settings {
		if strings.TrimSpace(setting.Key) == "" {
			continue
		}
		valueType := setting.ValueType
		if valueType == "" {
			valueType = "string"
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO system_settings (setting_key, setting_value, value_type, description)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(setting_key) DO UPDATE SET
				setting_value = excluded.setting_value,
				value_type = excluded.value_type,
				description = excluded.description,
				updated_at = CURRENT_TIMESTAMP
		`, setting.Key, setting.Value, valueType, setting.Description)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListUsers(ctx context.Context) ([]core.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, role, display_name, enabled,
		       COALESCE(last_login_at, ''), created_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []core.User
	for rows.Next() {
		var user core.User
		var enabled int
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.LastLoginAt, &user.CreatedAt); err != nil {
			return nil, err
		}
		user.Enabled = enabled == 1
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, input core.UserInput) (core.User, error) {
	if input.Username == "" || input.Password == "" {
		return core.User{}, core.ErrInvalidInput
	}
	passwordHash, err := system.HashPassword(input.Password)
	if err != nil {
		return core.User{}, err
	}
	enabled := boolToInt(true)
	if input.Enabled != nil {
		enabled = boolToInt(*input.Enabled)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, display_name, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, input.Username, passwordHash, input.Role, input.DisplayName, enabled)
	if err != nil {
		return core.User{}, err
	}
	id, _ := res.LastInsertId()
	return s.userByID(ctx, id)
}

func (s *Store) UpdateUser(ctx context.Context, id int64, input core.UserInput) (core.User, error) {
	enabled := 1
	if input.Enabled != nil {
		enabled = boolToInt(*input.Enabled)
	}
	if input.Password != "" {
		hash, err := system.HashPassword(input.Password)
		if err != nil {
			return core.User{}, err
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE users
			SET username = ?, password_hash = ?, role = ?, display_name = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND deleted_at IS NULL
		`, input.Username, hash, input.Role, input.DisplayName, enabled, id)
		if err != nil {
			return core.User{}, err
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			UPDATE users
			SET username = ?, role = ?, display_name = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND deleted_at IS NULL
		`, input.Username, input.Role, input.DisplayName, enabled, id)
		if err != nil {
			return core.User{}, err
		}
	}
	return s.userByID(ctx, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) userByID(ctx context.Context, id int64) (core.User, error) {
	var user core.User
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, role, display_name, enabled, COALESCE(last_login_at, ''), created_at
		FROM users
		WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.LastLoginAt, &user.CreatedAt)
	user.Enabled = enabled == 1
	return user, err
}

func (s *Store) ListGroups(ctx context.Context) ([]core.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, name, icon, description, default_interval_seconds, missed_times_threshold,
		       alert_enabled, enabled, created_at, updated_at
		FROM monitor_groups
		WHERE deleted_at IS NULL
		ORDER BY code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []core.Group
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Store) CreateGroup(ctx context.Context, input core.GroupInput) (core.Group, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_groups
			(code, name, icon, description, default_interval_seconds, missed_times_threshold, alert_enabled, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, input.Code, input.Name, input.Icon, input.Description, input.DefaultIntervalSeconds, input.MissedTimesThreshold, boolToInt(*input.AlertEnabled), boolToInt(*input.Enabled))
	if err != nil {
		return core.Group{}, err
	}
	id, _ := res.LastInsertId()
	return s.groupByID(ctx, id)
}

func (s *Store) UpdateGroup(ctx context.Context, id int64, input core.GroupInput) (core.Group, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitor_groups
		SET code = ?, name = ?, icon = ?, description = ?, default_interval_seconds = ?,
		    missed_times_threshold = ?, alert_enabled = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`, input.Code, input.Name, input.Icon, input.Description, input.DefaultIntervalSeconds, input.MissedTimesThreshold, boolToInt(*input.AlertEnabled), boolToInt(*input.Enabled), id)
	if err != nil {
		return core.Group{}, err
	}
	return s.groupByID(ctx, id)
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitor_groups WHERE id = ?`, id)
	return err
}

func (s *Store) GetGroupByCode(ctx context.Context, code string) (core.Group, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, icon, description, default_interval_seconds, missed_times_threshold,
		       alert_enabled, enabled, created_at, updated_at
		FROM monitor_groups
		WHERE code = ? AND deleted_at IS NULL AND enabled = 1
	`, code)
	return scanGroup(row)
}

func (s *Store) groupByID(ctx context.Context, id int64) (core.Group, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, icon, description, default_interval_seconds, missed_times_threshold,
		       alert_enabled, enabled, created_at, updated_at
		FROM monitor_groups
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	return scanGroup(row)
}

func (s *Store) ListItems(ctx context.Context, groupID int64) ([]core.Item, error) {
	query := `
		SELECT id, group_id, source_type, name, description, interval_seconds,
		       missed_times_threshold, alert_enabled, enabled, COALESCE(last_seen_at, ''), created_at
		FROM monitor_items
		WHERE deleted_at IS NULL
	`
	args := []interface{}{}
	if groupID > 0 {
		query += " AND group_id = ?"
		args = append(args, groupID)
	}
	query += " ORDER BY group_id, source_type, name"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []core.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertItem(ctx context.Context, input core.ItemInput) (core.Item, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM monitor_items
		WHERE group_id = ? AND source_type = ? AND name = ? AND deleted_at IS NULL
	`, input.GroupID, input.SourceType, input.Name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO monitor_items
				(group_id, source_type, name, description, interval_seconds, missed_times_threshold, alert_enabled, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, input.GroupID, input.SourceType, input.Name, input.Description, input.IntervalSeconds, input.MissedTimesThreshold, boolToInt(*input.AlertEnabled), boolToInt(*input.Enabled))
		if err != nil {
			return core.Item{}, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return core.Item{}, err
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE monitor_items
			SET interval_seconds = ?, missed_times_threshold = ?, alert_enabled = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, input.IntervalSeconds, input.MissedTimesThreshold, boolToInt(*input.AlertEnabled), boolToInt(*input.Enabled), id)
		if err != nil {
			return core.Item{}, err
		}
	}
	return s.itemByID(ctx, id)
}

func (s *Store) UpdateItem(ctx context.Context, id int64, input core.ItemInput) (core.Item, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitor_items
		SET group_id = ?, source_type = ?, name = ?, description = ?, interval_seconds = ?,
		    missed_times_threshold = ?, alert_enabled = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`, input.GroupID, input.SourceType, input.Name, input.Description, input.IntervalSeconds, input.MissedTimesThreshold, boolToInt(*input.AlertEnabled), boolToInt(*input.Enabled), id)
	if err != nil {
		return core.Item{}, err
	}
	return s.itemByID(ctx, id)
}

func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitor_items WHERE id = ?`, id)
	return err
}

func (s *Store) itemByID(ctx context.Context, id int64) (core.Item, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, source_type, name, description, interval_seconds,
		       missed_times_threshold, alert_enabled, enabled, COALESCE(last_seen_at, ''), created_at
		FROM monitor_items
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	return scanItem(row)
}

func (s *Store) ListActiveRequests(ctx context.Context) ([]core.ActiveRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ar.id, ar.group_id, ar.item_id, ar.name, ar.url, ar.method, ar.headers_json, ar.body_type,
		       ar.body_json, ar.interval_seconds, ar.timeout_seconds, ar.expected_status_code,
		       ar.enabled, COALESCE(mi.last_seen_at, '')
		FROM active_requests ar
		JOIN monitor_items mi ON mi.id = ar.item_id
		WHERE ar.deleted_at IS NULL
		ORDER BY ar.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []core.ActiveRequest
	for rows.Next() {
		req, err := scanActiveRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func (s *Store) CreateActiveRequest(ctx context.Context, input core.ActiveRequestInput) (core.ActiveRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.ActiveRequest{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO monitor_items
			(group_id, source_type, name, description, interval_seconds, missed_times_threshold, alert_enabled, enabled)
		VALUES (?, 'active', ?, '', ?, 3, 1, ?)
	`, input.GroupID, input.Name, input.IntervalSeconds, boolToInt(*input.Enabled))
	if err != nil {
		return core.ActiveRequest{}, err
	}
	itemID, _ := res.LastInsertId()
	res, err = tx.ExecContext(ctx, `
		INSERT INTO active_requests
			(group_id, item_id, name, url, method, headers_json, body_type, body_json,
			 interval_seconds, timeout_seconds, expected_status_code, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.GroupID, itemID, input.Name, input.URL, input.Method, input.HeadersJSON, input.BodyType, input.BodyJSON, input.IntervalSeconds, input.TimeoutSeconds, input.ExpectedStatusCode, boolToInt(*input.Enabled))
	if err != nil {
		return core.ActiveRequest{}, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return core.ActiveRequest{}, err
	}
	return s.activeRequestByID(ctx, id)
}

func (s *Store) UpdateActiveRequest(ctx context.Context, id int64, input core.ActiveRequestInput) (core.ActiveRequest, error) {
	var itemID int64
	if err := s.db.QueryRowContext(ctx, `SELECT item_id FROM active_requests WHERE id = ? AND deleted_at IS NULL`, id).Scan(&itemID); err != nil {
		return core.ActiveRequest{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.ActiveRequest{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		UPDATE active_requests
		SET group_id = ?, name = ?, url = ?, method = ?, headers_json = ?, body_type = ?, body_json = ?,
		    interval_seconds = ?, timeout_seconds = ?, expected_status_code = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.GroupID, input.Name, input.URL, input.Method, input.HeadersJSON, input.BodyType, input.BodyJSON, input.IntervalSeconds, input.TimeoutSeconds, input.ExpectedStatusCode, boolToInt(*input.Enabled), id)
	if err != nil {
		return core.ActiveRequest{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE monitor_items
		SET group_id = ?, name = ?, interval_seconds = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.GroupID, input.Name, input.IntervalSeconds, boolToInt(*input.Enabled), itemID)
	if err != nil {
		return core.ActiveRequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.ActiveRequest{}, err
	}
	return s.activeRequestByID(ctx, id)
}

func (s *Store) DeleteActiveRequest(ctx context.Context, id int64) error {
	var itemID int64
	_ = s.db.QueryRowContext(ctx, `SELECT item_id FROM active_requests WHERE id = ?`, id).Scan(&itemID)
	if itemID > 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM monitor_items WHERE id = ?`, itemID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM active_requests WHERE id = ?`, id)
	return err
}

func (s *Store) activeRequestByID(ctx context.Context, id int64) (core.ActiveRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT ar.id, ar.group_id, ar.item_id, ar.name, ar.url, ar.method, ar.headers_json, ar.body_type,
		       ar.body_json, ar.interval_seconds, ar.timeout_seconds, ar.expected_status_code,
		       ar.enabled, COALESCE(mi.last_seen_at, '')
		FROM active_requests ar
		JOIN monitor_items mi ON mi.id = ar.item_id
		WHERE ar.id = ? AND ar.deleted_at IS NULL
	`, id)
	return scanActiveRequest(row)
}

func (s *Store) ListFields(ctx context.Context, groupID, itemID int64) ([]core.FieldDefinition, error) {
	query := `
		SELECT id, scope_type, group_id, item_id, field_path, display_name, value_type, unit, required, enabled
		FROM monitor_field_definitions
		WHERE deleted_at IS NULL
	`
	args := []interface{}{}
	if groupID > 0 {
		query += " AND group_id = ?"
		args = append(args, groupID)
	}
	if itemID > 0 {
		query += " AND (item_id = ? OR item_id IS NULL)"
		args = append(args, itemID)
	}
	query += " ORDER BY scope_type, field_path"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []core.FieldDefinition
	for rows.Next() {
		field, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, rows.Err()
}

func (s *Store) UpsertField(ctx context.Context, input core.FieldInput) (core.FieldDefinition, error) {
	var id int64
	var err error
	if input.ID > 0 {
		id = input.ID
	} else if input.ScopeType == "item" && input.ItemID != nil {
		err = s.db.QueryRowContext(ctx, `
			SELECT id FROM monitor_field_definitions
			WHERE scope_type = 'item' AND item_id = ? AND field_path = ? AND deleted_at IS NULL
		`, *input.ItemID, input.FieldPath).Scan(&id)
	} else {
		err = s.db.QueryRowContext(ctx, `
			SELECT id FROM monitor_field_definitions
			WHERE scope_type = 'group' AND group_id = ? AND field_path = ? AND deleted_at IS NULL
		`, input.GroupID, input.FieldPath).Scan(&id)
	}
	if input.ID > 0 {
		_, err = s.db.ExecContext(ctx, `
			UPDATE monitor_field_definitions
			SET scope_type = ?, group_id = ?, item_id = ?, field_path = ?, display_name = ?,
			    value_type = ?, unit = ?, required = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND deleted_at IS NULL
		`, input.ScopeType, input.GroupID, nullableInt(input.ItemID), input.FieldPath, input.DisplayName,
			input.ValueType, input.Unit, boolToInt(*input.Required), boolToInt(*input.Enabled), id)
		if err != nil {
			return core.FieldDefinition{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO monitor_field_definitions
				(scope_type, group_id, item_id, field_path, display_name, value_type, unit, required, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, input.ScopeType, input.GroupID, nullableInt(input.ItemID), input.FieldPath, input.DisplayName, input.ValueType, input.Unit, boolToInt(*input.Required), boolToInt(*input.Enabled))
		if err != nil {
			return core.FieldDefinition{}, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return core.FieldDefinition{}, err
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE monitor_field_definitions
			SET display_name = ?, value_type = ?, unit = ?, required = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, input.DisplayName, input.ValueType, input.Unit, boolToInt(*input.Required), boolToInt(*input.Enabled), id)
		if err != nil {
			return core.FieldDefinition{}, err
		}
	}
	return s.fieldByID(ctx, id)
}

func (s *Store) DeleteField(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var scopeType, fieldPath string
	var groupID int64
	var itemID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT scope_type, group_id, item_id, field_path
		FROM monitor_field_definitions
		WHERE id = ?
	`, id).Scan(&scopeType, &groupID, &itemID, &fieldPath); err != nil {
		return err
	}
	if scopeType == "item" && itemID.Valid {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM alert_rules
			WHERE scope_type = 'item' AND item_id = ? AND field_path = ?
		`, itemID.Int64, fieldPath); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM alert_rules
			WHERE scope_type = 'group' AND group_id = ? AND field_path = ?
		`, groupID, fieldPath); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM monitor_field_definitions WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) fieldByID(ctx context.Context, id int64) (core.FieldDefinition, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, scope_type, group_id, item_id, field_path, display_name, value_type, unit, required, enabled
		FROM monitor_field_definitions
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	return scanField(row)
}

func (s *Store) ListChannels(ctx context.Context) ([]core.Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, name, channel_type, config_json, enabled, is_default
		FROM notification_channels
		WHERE deleted_at IS NULL
		ORDER BY is_default DESC, code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []core.Channel
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (s *Store) UpsertChannel(ctx context.Context, input core.ChannelInput) (core.Channel, error) {
	var id int64
	err := error(nil)
	if input.ID > 0 {
		id = input.ID
	} else {
		err = s.db.QueryRowContext(ctx, `
			SELECT id FROM notification_channels WHERE code = ? AND deleted_at IS NULL
		`, input.Code).Scan(&id)
	}
	if input.ID == 0 && errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO notification_channels (code, name, channel_type, config_json, enabled, is_default)
			VALUES (?, ?, ?, ?, ?, ?)
		`, input.Code, input.Name, input.Type, input.ConfigJSON, boolToInt(*input.Enabled), boolToInt(*input.IsDefault))
		if err != nil {
			return core.Channel{}, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return core.Channel{}, err
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE notification_channels
			SET name = ?, channel_type = ?, config_json = ?, enabled = ?, is_default = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, input.Name, input.Type, input.ConfigJSON, boolToInt(*input.Enabled), boolToInt(*input.IsDefault), id)
		if err != nil {
			return core.Channel{}, err
		}
	}
	return s.channelByID(ctx, id)
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
	return err
}

func (s *Store) channelByID(ctx context.Context, id int64) (core.Channel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, channel_type, config_json, enabled, is_default
		FROM notification_channels
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	return scanChannel(row)
}

func (s *Store) ListRules(ctx context.Context) ([]core.AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, scope_type, group_id, item_id, field_definition_id, source_type, rule_type,
		       COALESCE(field_path, ''), COALESCE(value_type, ''), COALESCE(operator, ''),
		       COALESCE(threshold_value, ''), COALESCE(aggregate_func, ''),
		       aggregate_window_seconds, aggregate_sample_count, consecutive_count, recovery_count,
		       severity, message_template, enabled
		FROM alert_rules
		WHERE deleted_at IS NULL
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []core.AlertRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rule.ChannelIDs, _ = s.ruleChannelIDs(ctx, rule.ID)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) UpsertRule(ctx context.Context, input core.AlertRuleInput) (core.AlertRule, error) {
	rule := core.AlertRule{
		ID:                     input.ID,
		Name:                   input.Name,
		ScopeType:              input.ScopeType,
		GroupID:                input.GroupID,
		ItemID:                 input.ItemID,
		FieldDefinitionID:      input.FieldDefinitionID,
		SourceType:             input.SourceType,
		RuleType:               input.RuleType,
		FieldPath:              input.FieldPath,
		ValueType:              input.ValueType,
		Operator:               input.Operator,
		ThresholdValue:         input.ThresholdValue,
		AggregateFunc:          input.AggregateFunc,
		AggregateWindowSeconds: input.AggregateWindowSeconds,
		AggregateSampleCount:   input.AggregateSampleCount,
		ConsecutiveCount:       input.ConsecutiveCount,
		RecoveryCount:          input.RecoveryCount,
		Severity:               input.Severity,
		MessageTemplate:        input.MessageTemplate,
		Enabled:                input.Enabled != nil && *input.Enabled,
		ChannelIDs:             input.ChannelIDs,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.AlertRule{}, err
	}
	defer tx.Rollback()

	id := rule.ID
	if id == 0 {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO alert_rules
				(name, scope_type, group_id, item_id, field_definition_id, source_type, rule_type,
				 field_path, value_type, operator, threshold_value, aggregate_func,
				 aggregate_window_seconds, aggregate_sample_count, consecutive_count, recovery_count,
				 severity, message_template, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.Name, rule.ScopeType, nullableInt(rule.GroupID), nullableInt(rule.ItemID), nullableInt(rule.FieldDefinitionID),
			rule.SourceType, rule.RuleType, nullableString(rule.FieldPath), nullableString(rule.ValueType),
			nullableString(rule.Operator), nullableString(rule.ThresholdValue), nullableString(rule.AggregateFunc),
			nullableIntFromInt(rule.AggregateWindowSeconds), nullableIntFromInt(rule.AggregateSampleCount),
			rule.ConsecutiveCount, rule.RecoveryCount, rule.Severity, rule.MessageTemplate, boolToInt(rule.Enabled))
		if err != nil {
			return core.AlertRule{}, err
		}
		id, _ = res.LastInsertId()
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE alert_rules
			SET name = ?, scope_type = ?, group_id = ?, item_id = ?, field_definition_id = ?,
			    source_type = ?, rule_type = ?, field_path = ?, value_type = ?, operator = ?,
			    threshold_value = ?, aggregate_func = ?, aggregate_window_seconds = ?,
			    aggregate_sample_count = ?, consecutive_count = ?, recovery_count = ?,
			    severity = ?, message_template = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND deleted_at IS NULL
		`, rule.Name, rule.ScopeType, nullableInt(rule.GroupID), nullableInt(rule.ItemID), nullableInt(rule.FieldDefinitionID),
			rule.SourceType, rule.RuleType, nullableString(rule.FieldPath), nullableString(rule.ValueType),
			nullableString(rule.Operator), nullableString(rule.ThresholdValue), nullableString(rule.AggregateFunc),
			nullableIntFromInt(rule.AggregateWindowSeconds), nullableIntFromInt(rule.AggregateSampleCount),
			rule.ConsecutiveCount, rule.RecoveryCount, rule.Severity, rule.MessageTemplate, boolToInt(rule.Enabled), id)
		if err != nil {
			return core.AlertRule{}, err
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM alert_rule_channels WHERE rule_id = ?`, id)
	}

	for _, channelID := range rule.ChannelIDs {
		if channelID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO alert_rule_channels (rule_id, channel_id) VALUES (?, ?)
		`, id, channelID); err != nil {
			return core.AlertRule{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return core.AlertRule{}, err
	}
	return s.ruleByID(ctx, id)
}

func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = ?`, id)
	return err
}

func (s *Store) ruleByID(ctx context.Context, id int64) (core.AlertRule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, scope_type, group_id, item_id, field_definition_id, source_type, rule_type,
		       COALESCE(field_path, ''), COALESCE(value_type, ''), COALESCE(operator, ''),
		       COALESCE(threshold_value, ''), COALESCE(aggregate_func, ''),
		       aggregate_window_seconds, aggregate_sample_count, consecutive_count, recovery_count,
		       severity, message_template, enabled
		FROM alert_rules
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	rule, err := scanRule(row)
	if err != nil {
		return core.AlertRule{}, err
	}
	rule.ChannelIDs, _ = s.ruleChannelIDs(ctx, id)
	return rule, nil
}

func (s *Store) ruleChannelIDs(ctx context.Context, ruleID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT channel_id FROM alert_rule_channels WHERE rule_id = ?`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SaveSample(ctx context.Context, input core.SaveSampleInput, values []core.SampleValue) (core.Sample, error) {
	rawBytes, _ := json.Marshal(input.Raw)
	var reportedAt interface{}
	if input.ReportedAt != nil {
		reportedAt = input.ReportedAt.UTC().Format(time.RFC3339Nano)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Sample{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO monitor_samples
			(group_id, item_id, source_type, active_request_id, name, reported_at, interval_seconds,
			 status, http_status_code, latency_ms, raw_json, error_message, received_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.GroupID, input.ItemID, input.SourceType, nullableInt(input.ActiveRequestID), input.Name, reportedAt,
		nullablePositive(input.IntervalSeconds), input.Status, nullablePositive(input.HTTPStatusCode),
		nullablePositiveInt64(input.LatencyMS), string(rawBytes), input.ErrorMessage, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return core.Sample{}, err
	}
	sampleID, _ := res.LastInsertId()

	var receivedAt string
	if err := tx.QueryRowContext(ctx, `SELECT received_at FROM monitor_samples WHERE id = ?`, sampleID).Scan(&receivedAt); err != nil {
		return core.Sample{}, err
	}
	for _, value := range values {
		rawValue, _ := json.Marshal(value.RawValue)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO monitor_sample_values
				(sample_id, group_id, item_id, field_path, value_type, string_value, integer_value,
				 float_value, boolean_value, numeric_value, raw_value, received_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, sampleID, input.GroupID, input.ItemID, value.FieldPath, value.ValueType,
			nullableString(value.StringValue), nullableInt(value.IntegerValue), nullableFloat(value.FloatValue),
			nullableBool(value.BooleanValue), nullableFloat(value.NumericValue), string(rawValue), receivedAt)
		if err != nil {
			return core.Sample{}, err
		}
	}

	if input.Status == "ok" {
		_, err = tx.ExecContext(ctx, `
			UPDATE monitor_items SET last_seen_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
		`, receivedAt, input.ItemID)
		if err != nil {
			return core.Sample{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return core.Sample{}, err
	}
	return s.sampleByID(ctx, sampleID)
}

func (s *Store) LastSample(ctx context.Context, itemID int64) (core.Sample, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM monitor_samples WHERE item_id = ? ORDER BY received_at DESC, id DESC LIMIT 1
	`, itemID).Scan(&id)
	if err != nil {
		return core.Sample{}, err
	}
	return s.sampleByID(ctx, id)
}

func (s *Store) ListSamples(ctx context.Context, groupID, itemID int64, limit int) ([]core.Sample, error) {
	query := `
		SELECT id
		FROM monitor_samples
		WHERE 1 = 1
	`
	args := []interface{}{}
	if groupID > 0 {
		query += " AND group_id = ?"
		args = append(args, groupID)
	}
	if itemID > 0 {
		query += " AND item_id = ?"
		args = append(args, itemID)
	}
	query += " ORDER BY received_at DESC, id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []core.Sample
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		sample, err := s.sampleByID(ctx, id)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}

func (s *Store) sampleByID(ctx context.Context, id int64) (core.Sample, error) {
	var sample core.Sample
	var rawJSON string
	var reportedAt sql.NullString
	var interval, statusCode sql.NullInt64
	var latency sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, item_id, source_type, name, reported_at, received_at,
		       interval_seconds, status, http_status_code, latency_ms, raw_json, error_message
		FROM monitor_samples
		WHERE id = ?
	`, id).Scan(&sample.ID, &sample.GroupID, &sample.ItemID, &sample.SourceType, &sample.Name, &reportedAt,
		&sample.ReceivedAt, &interval, &sample.Status, &statusCode, &latency, &rawJSON, &sample.ErrorMessage)
	if err != nil {
		return core.Sample{}, err
	}
	if reportedAt.Valid {
		sample.ReportedAt = reportedAt.String
	}
	if interval.Valid {
		sample.IntervalSeconds = int(interval.Int64)
	}
	if statusCode.Valid {
		sample.HTTPStatusCode = int(statusCode.Int64)
	}
	if latency.Valid {
		sample.LatencyMS = latency.Int64
	}
	_ = json.Unmarshal([]byte(rawJSON), &sample.Raw)
	sample.Values, _ = s.sampleValues(ctx, sample.ID)
	return sample, nil
}

func (s *Store) sampleValues(ctx context.Context, sampleID int64) ([]core.SampleValue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT field_path, value_type, string_value, integer_value, float_value, boolean_value, numeric_value, raw_value
		FROM monitor_sample_values
		WHERE sample_id = ?
		ORDER BY field_path
	`, sampleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []core.SampleValue
	for rows.Next() {
		var value core.SampleValue
		var stringValue sql.NullString
		var intValue sql.NullInt64
		var floatValue, numericValue sql.NullFloat64
		var boolValue sql.NullInt64
		var rawValue sql.NullString
		if err := rows.Scan(&value.FieldPath, &value.ValueType, &stringValue, &intValue, &floatValue, &boolValue, &numericValue, &rawValue); err != nil {
			return nil, err
		}
		if stringValue.Valid {
			value.StringValue = stringValue.String
		}
		if intValue.Valid {
			value.IntegerValue = &intValue.Int64
		}
		if floatValue.Valid {
			value.FloatValue = &floatValue.Float64
		}
		if boolValue.Valid {
			v := boolValue.Int64 == 1
			value.BooleanValue = &v
		}
		if numericValue.Valid {
			value.NumericValue = &numericValue.Float64
		}
		if rawValue.Valid {
			_ = json.Unmarshal([]byte(rawValue.String), &value.RawValue)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) Stats(ctx context.Context, groupID, itemID int64, fieldPath string, since time.Time) (core.StatResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT received_at, numeric_value, raw_value
		FROM monitor_sample_values
		WHERE group_id = ? AND item_id = ? AND field_path = ? AND received_at >= ?
		ORDER BY received_at
	`, groupID, itemID, fieldPath, sqliteTime(since))
	if err != nil {
		return core.StatResult{}, err
	}
	defer rows.Close()

	result := core.StatResult{
		GroupID:     groupID,
		ItemID:      itemID,
		FieldPath:   fieldPath,
		GeneratedAt: time.Now().Format(time.RFC3339Nano),
	}
	var nums []float64
	for rows.Next() {
		var at string
		var numeric sql.NullFloat64
		var raw string
		if err := rows.Scan(&at, &numeric, &raw); err != nil {
			return core.StatResult{}, err
		}
		result.Count++
		result.LatestAt = at
		_ = json.Unmarshal([]byte(raw), &result.Latest)
		if numeric.Valid {
			value := numeric.Float64
			nums = append(nums, value)
			result.Series = append(result.Series, core.Point{Time: at, Value: &value})
		}
	}
	if len(nums) > 0 {
		result.Avg = floatPtr(avg(nums))
		result.Max = floatPtr(max(nums))
		result.Min = floatPtr(min(nums))
		result.Median = floatPtr(median(nums))
	}
	return result, rows.Err()
}

func (s *Store) AlertRulesForSample(ctx context.Context, sample core.Sample) ([]core.AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, scope_type, group_id, item_id, field_definition_id, source_type, rule_type,
		       COALESCE(field_path, ''), COALESCE(value_type, ''), COALESCE(operator, ''),
		       COALESCE(threshold_value, ''), COALESCE(aggregate_func, ''),
		       aggregate_window_seconds, aggregate_sample_count, consecutive_count, recovery_count,
		       severity, message_template, enabled
		FROM alert_rules
		WHERE deleted_at IS NULL AND enabled = 1
		  AND (source_type = 'any' OR source_type = ?)
		  AND (
			scope_type = 'global'
			OR (scope_type = 'group' AND group_id = ?)
			OR (scope_type IN ('item', 'field') AND item_id = ?)
		  )
	`, sample.SourceType, sample.GroupID, sample.ItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []core.AlertRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) WindowValues(ctx context.Context, itemID int64, fieldPath string, since time.Time, limit int) ([]float64, error) {
	query := `
		SELECT numeric_value
		FROM monitor_sample_values
		WHERE item_id = ? AND field_path = ? AND received_at >= ? AND numeric_value IS NOT NULL
		ORDER BY received_at DESC
	`
	args := []interface{}{itemID, fieldPath, sqliteTime(since)}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []float64
	for rows.Next() {
		var value float64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) ApplyAlertEvaluation(ctx context.Context, rule core.AlertRule, sample core.Sample, matched bool, currentValue, threshold string) (*core.AlertEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	state, err := getAlertState(ctx, tx, rule.ID, sample.GroupID, sample.ItemID, rule.FieldPath)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339Nano)
	var eventType string
	if matched {
		state.ConsecutiveHits++
		state.ConsecutiveRecovers = 0
		if state.FirstHitAt == "" {
			state.FirstHitAt = now
		}
		state.LastHitAt = now
		if state.Status != "alerting" && state.ConsecutiveHits >= rule.ConsecutiveCount {
			eventType = "triggered"
			state.Status = "alerting"
			state.LastAlertAt = now
		}
	} else {
		state.ConsecutiveHits = 0
		state.ConsecutiveRecovers++
		if state.Status == "alerting" && state.ConsecutiveRecovers >= rule.RecoveryCount {
			eventType = "recovered"
			state.Status = "ok"
			state.FirstHitAt = ""
		}
	}

	if state.ID == 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO alert_states
				(rule_id, group_id, item_id, field_path, status, consecutive_hits, consecutive_recovers,
				 first_hit_at, last_hit_at, last_alert_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.ID, nullableInt(&sample.GroupID), nullableInt(&sample.ItemID), rule.FieldPath, state.Status,
			state.ConsecutiveHits, state.ConsecutiveRecovers, nullableString(state.FirstHitAt),
			nullableString(state.LastHitAt), nullableString(state.LastAlertAt), now)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE alert_states
			SET status = ?, consecutive_hits = ?, consecutive_recovers = ?, first_hit_at = ?,
			    last_hit_at = ?, last_alert_at = ?, updated_at = ?
			WHERE id = ?
		`, state.Status, state.ConsecutiveHits, state.ConsecutiveRecovers, nullableString(state.FirstHitAt),
			nullableString(state.LastHitAt), nullableString(state.LastAlertAt), now, state.ID)
	}
	if err != nil {
		return nil, err
	}

	var eventID int64
	if eventType != "" {
		title := fmt.Sprintf("%s %s", rule.Name, localizedEventType(ctx, tx, eventType))
		message := rule.MessageTemplate
		if message == "" {
			message = defaultAlertMessage(ctx, tx)
		}
		message = renderAlertMessage(message, rule, sample, currentValue, threshold)
		res, err := tx.ExecContext(ctx, `
			INSERT INTO alert_events
				(rule_id, group_id, item_id, sample_id, event_type, severity, title, message,
				 field_path, current_value, threshold_value, occurred_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.ID, nullableInt(&sample.GroupID), nullableInt(&sample.ItemID), nullableInt(&sample.ID),
			eventType, rule.Severity, title, message, rule.FieldPath, nullableString(currentValue), nullableString(threshold), now)
		if err != nil {
			return nil, err
		}
		eventID, _ = res.LastInsertId()
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if eventID == 0 {
		return nil, nil
	}
	event, err := s.eventByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func localizedEventType(ctx context.Context, tx *sql.Tx, eventType string) string {
	locale := settingValueTx(ctx, tx, "default_locale")
	if locale == "zh-CN" {
		switch eventType {
		case "triggered":
			return "触发"
		case "recovered":
			return "恢复"
		}
	}
	switch eventType {
	case "triggered":
		return "Triggered"
	case "recovered":
		return "Recovered"
	default:
		return eventType
	}
}

func defaultAlertMessage(ctx context.Context, tx *sql.Tx) string {
	if settingValueTx(ctx, tx, "default_locale") == "zh-CN" {
		return "规则={{rule}} 条目={{item}} 字段={{field}} 当前值={{current}} 阈值={{threshold}}"
	}
	return "rule={{rule}} item={{item}} field={{field}} current={{current}} threshold={{threshold}}"
}

func settingValueTx(ctx context.Context, tx *sql.Tx, key string) string {
	var value string
	_ = tx.QueryRowContext(ctx, `SELECT setting_value FROM system_settings WHERE setting_key = ?`, key).Scan(&value)
	return value
}

func renderAlertMessage(message string, rule core.AlertRule, sample core.Sample, currentValue, threshold string) string {
	replacements := map[string]string{
		"{{rule}}":      rule.Name,
		"{{item}}":      sample.Name,
		"{{field}}":     rule.FieldPath,
		"{{current}}":   currentValue,
		"{{threshold}}": threshold,
		"{{severity}}":  rule.Severity,
	}
	for key, value := range replacements {
		message = strings.ReplaceAll(message, key, value)
	}
	return message
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]core.AlertEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, rule_id, group_id, item_id, sample_id, event_type, severity, title,
		       message, field_path, current_value, threshold_value, occurred_at
		FROM alert_events
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []core.AlertEvent
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) eventByID(ctx context.Context, id int64) (core.AlertEvent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, rule_id, group_id, item_id, sample_id, event_type, severity, title,
		       message, field_path, current_value, threshold_value, occurred_at
		FROM alert_events
		WHERE id = ?
	`, id)
	return scanEvent(row)
}

func (s *Store) ListEnabledChannelsForRule(ctx context.Context, ruleID int64) ([]core.Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.code, c.name, c.channel_type, c.config_json, c.enabled, c.is_default
		FROM notification_channels c
		JOIN alert_rule_channels rc ON rc.channel_id = c.id
		WHERE rc.rule_id = ? AND c.enabled = 1 AND c.deleted_at IS NULL
		ORDER BY c.is_default DESC, c.code
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var channels []core.Channel
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (s *Store) CreateNotification(ctx context.Context, eventID, channelID int64, status, requestJSON, responseText, errorMessage string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_notifications
			(event_id, channel_id, status, request_json, response_text, error_message, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, CASE WHEN ? = 'sent' THEN CURRENT_TIMESTAMP ELSE NULL END)
	`, eventID, channelID, status, requestJSON, responseText, errorMessage, status)
	return err
}

func (s *Store) Dashboard(ctx context.Context) (core.Dashboard, error) {
	var dash core.Dashboard
	queries := []struct {
		target *int64
		sql    string
	}{
		{&dash.Groups, `SELECT COUNT(*) FROM monitor_groups WHERE deleted_at IS NULL`},
		{&dash.Items, `SELECT COUNT(*) FROM monitor_items WHERE deleted_at IS NULL`},
		{&dash.Samples24h, `SELECT COUNT(*) FROM monitor_samples WHERE received_at >= datetime('now', '-1 day')`},
		{&dash.AlertingRules, `SELECT COUNT(*) FROM alert_states WHERE status = 'alerting'`},
	}
	for _, query := range queries {
		if err := s.db.QueryRowContext(ctx, query.sql).Scan(query.target); err != nil {
			return core.Dashboard{}, err
		}
	}
	events, err := s.ListEvents(ctx, 8)
	if err != nil {
		return core.Dashboard{}, err
	}
	dash.RecentEvents = events
	return dash, nil
}

func (s *Store) Cleanup(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitor_samples WHERE received_at < ?`, before.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM alert_events WHERE occurred_at < ?`, before.Format(time.RFC3339Nano))
	return err
}

type alertState struct {
	ID                  int64
	Status              string
	ConsecutiveHits     int
	ConsecutiveRecovers int
	FirstHitAt          string
	LastHitAt           string
	LastAlertAt         string
}

func getAlertState(ctx context.Context, tx *sql.Tx, ruleID, groupID, itemID int64, fieldPath string) (alertState, error) {
	var state alertState
	var first, last, alert sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, status, consecutive_hits, consecutive_recovers,
		       first_hit_at, last_hit_at, last_alert_at
		FROM alert_states
		WHERE rule_id = ? AND group_id = ? AND item_id = ? AND field_path = ?
	`, ruleID, groupID, itemID, fieldPath).Scan(&state.ID, &state.Status, &state.ConsecutiveHits, &state.ConsecutiveRecovers, &first, &last, &alert)
	if errors.Is(err, sql.ErrNoRows) {
		return alertState{Status: "ok"}, nil
	}
	if err != nil {
		return alertState{}, err
	}
	state.FirstHitAt = first.String
	state.LastHitAt = last.String
	state.LastAlertAt = alert.String
	return state, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanGroup(row scanner) (core.Group, error) {
	var group core.Group
	var alertEnabled, enabled int
	err := row.Scan(&group.ID, &group.Code, &group.Name, &group.Icon, &group.Description, &group.DefaultIntervalSeconds,
		&group.MissedTimesThreshold, &alertEnabled, &enabled, &group.CreatedAt, &group.UpdatedAt)
	group.AlertEnabled = alertEnabled == 1
	group.Enabled = enabled == 1
	return group, err
}

func scanItem(row scanner) (core.Item, error) {
	var item core.Item
	var alertEnabled, enabled int
	err := row.Scan(&item.ID, &item.GroupID, &item.SourceType, &item.Name, &item.Description,
		&item.IntervalSeconds, &item.MissedTimesThreshold, &alertEnabled, &enabled, &item.LastSeenAt, &item.CreatedAt)
	item.AlertEnabled = alertEnabled == 1
	item.Enabled = enabled == 1
	return item, err
}

func scanActiveRequest(row scanner) (core.ActiveRequest, error) {
	var req core.ActiveRequest
	var enabled int
	err := row.Scan(&req.ID, &req.GroupID, &req.ItemID, &req.Name, &req.URL, &req.Method, &req.HeadersJSON,
		&req.BodyType, &req.BodyJSON, &req.IntervalSeconds, &req.TimeoutSeconds, &req.ExpectedStatusCode,
		&enabled, &req.LastSeenAt)
	req.Enabled = enabled == 1
	return req, err
}

func scanField(row scanner) (core.FieldDefinition, error) {
	var field core.FieldDefinition
	var itemID sql.NullInt64
	var required, enabled int
	err := row.Scan(&field.ID, &field.ScopeType, &field.GroupID, &itemID, &field.FieldPath, &field.DisplayName,
		&field.ValueType, &field.Unit, &required, &enabled)
	if itemID.Valid {
		field.ItemID = &itemID.Int64
	}
	field.Required = required == 1
	field.Enabled = enabled == 1
	return field, err
}

func scanChannel(row scanner) (core.Channel, error) {
	var channel core.Channel
	var enabled, isDefault int
	err := row.Scan(&channel.ID, &channel.Code, &channel.Name, &channel.Type, &channel.ConfigJSON, &enabled, &isDefault)
	channel.Enabled = enabled == 1
	channel.IsDefault = isDefault == 1
	return channel, err
}

func scanRule(row scanner) (core.AlertRule, error) {
	var rule core.AlertRule
	var groupID, itemID, fieldID sql.NullInt64
	var window, sampleCount sql.NullInt64
	var enabled int
	err := row.Scan(&rule.ID, &rule.Name, &rule.ScopeType, &groupID, &itemID, &fieldID, &rule.SourceType,
		&rule.RuleType, &rule.FieldPath, &rule.ValueType, &rule.Operator, &rule.ThresholdValue,
		&rule.AggregateFunc, &window, &sampleCount, &rule.ConsecutiveCount, &rule.RecoveryCount,
		&rule.Severity, &rule.MessageTemplate, &enabled)
	if groupID.Valid {
		rule.GroupID = &groupID.Int64
	}
	if itemID.Valid {
		rule.ItemID = &itemID.Int64
	}
	if fieldID.Valid {
		rule.FieldDefinitionID = &fieldID.Int64
	}
	if window.Valid {
		v := int(window.Int64)
		rule.AggregateWindowSeconds = &v
	}
	if sampleCount.Valid {
		v := int(sampleCount.Int64)
		rule.AggregateSampleCount = &v
	}
	rule.Enabled = enabled == 1
	return rule, err
}

func scanEvent(row scanner) (core.AlertEvent, error) {
	var event core.AlertEvent
	var groupID, itemID, sampleID sql.NullInt64
	var current, threshold sql.NullString
	err := row.Scan(&event.ID, &event.RuleID, &groupID, &itemID, &sampleID, &event.EventType,
		&event.Severity, &event.Title, &event.Message, &event.FieldPath, &current, &threshold, &event.OccurredAt)
	if groupID.Valid {
		event.GroupID = &groupID.Int64
	}
	if itemID.Valid {
		event.ItemID = &itemID.Int64
	}
	if sampleID.Valid {
		event.SampleID = &sampleID.Int64
	}
	event.CurrentValue = current.String
	event.ThresholdValue = threshold.String
	return event, err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableInt(value *int64) interface{} {
	if value == nil || *value == 0 {
		return nil
	}
	return *value
}

func nullableIntFromInt(value *int) interface{} {
	if value == nil || *value == 0 {
		return nil
	}
	return *value
}

func nullablePositive(value int) interface{} {
	if value <= 0 {
		return nil
	}
	return value
}

func nullablePositiveInt64(value int64) interface{} {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func sqliteTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05")
}

func nullableFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableBool(value *bool) interface{} {
	if value == nil {
		return nil
	}
	return boolToInt(*value)
}

func avg(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func max(values []float64) float64 {
	out := math.Inf(-1)
	for _, value := range values {
		out = math.Max(out, value)
	}
	return out
}

func min(values []float64) float64 {
	out := math.Inf(1)
	for _, value := range values {
		out = math.Min(out, value)
	}
	return out
}

func median(values []float64) float64 {
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

func floatPtr(value float64) *float64 {
	return &value
}
