package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	migrations "lightmonitor/database"
	"lightmonitor/internal/application/core"
)

func TestPassiveReceiveCoercesFieldsAndTriggersAlert(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	service := core.NewService(store)

	group, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "host",
		Name:                   "Host",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "cpu.load",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "host",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"cpu": map[string]interface{}{"load": "2.5"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	items, err := service.Items(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}

	samples, err := service.Samples(ctx, group.ID, items[0].ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || len(samples[0].Values) == 0 {
		t.Fatalf("missing sample values: %#v", samples)
	}
	got := samples[0].Values[0]
	if got.FieldPath != "cpu.load" || got.ValueType != "float" || got.NumericValue == nil || *got.NumericValue != 2.5 {
		t.Fatalf("unexpected coerced value: %#v", got)
	}

	if _, err := service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "cpu high",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &items[0].ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "cpu.load",
		Operator:         "gt",
		ThresholdValue:   "1",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "host",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"cpu": map[string]interface{}{"load": "3.1"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, err := service.Events(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "triggered" {
		t.Fatalf("events = %#v, want one triggered event", events)
	}
}
