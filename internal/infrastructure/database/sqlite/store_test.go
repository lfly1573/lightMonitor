package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		FieldPath: "$.cpu.load",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
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

	samples, err := service.Samples(ctx, group.ID, items[0].ID, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || len(samples[0].Values) == 0 {
		t.Fatalf("missing sample values: %#v", samples)
	}
	got := samples[0].Values[0]
	if got.FieldPath != "$.cpu.load" || got.ValueType != "float" || got.NumericValue == nil || *got.NumericValue != 2.5 {
		t.Fatalf("unexpected coerced value: %#v", got)
	}

	if _, err := service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "cpu high",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &items[0].ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "$.cpu.load",
		Operator:         "gt",
		ThresholdValue:   "1",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "host",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"cpu": map[string]interface{}{"load": "3.1"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "triggered" {
		t.Fatalf("events = %#v, want one triggered event", events)
	}
}

func TestCheckMissingAlertsNeverReportedPassiveItem(t *testing.T) {
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
		Code:                   "passive",
		Name:                   "Passive",
		DefaultIntervalSeconds: 1,
		MissedTimesThreshold:   1,
	})
	if err != nil {
		t.Fatal(err)
	}

	item, err := service.CreateItem(ctx, core.ItemInput{
		GroupID:              group.ID,
		SourceType:           "passive",
		Name:                 "agent-1",
		IntervalSeconds:      1,
		MissedTimesThreshold: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute).UTC().Format("2006-01-02 15:04:05")
	if _, err := db.ExecContext(ctx, `UPDATE monitor_items SET created_at = ? WHERE id = ?`, old, item.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "passive missing",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &item.ID,
		SourceType:       "passive",
		RuleType:         "missing_data",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
	}); err != nil {
		t.Fatal(err)
	}

	if err := service.CheckMissing(ctx); err != nil {
		t.Fatal(err)
	}

	samples, err := service.Samples(ctx, group.ID, item.ID, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].Status != "missing" {
		t.Fatalf("samples = %#v, want one missing sample", samples)
	}
	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "triggered" {
		t.Fatalf("events = %#v, want one triggered missing alert", events)
	}
}

func TestStatsUsesSQLiteComparableSinceTime(t *testing.T) {
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
		Code:                   "stats",
		Name:                   "Stats",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "latency",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "stats",
		Name:     "api",
		Interval: 10,
		Data:     map[string]interface{}{"latency": 12.3},
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

	stats, err := service.Stats(ctx, group.ID, items[0].ID, "latency", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 1 || len(stats.Series) != 1 {
		t.Fatalf("stats = %#v, want one point in last hour", stats)
	}
}

func TestPassiveReceiveSilencedWhenItemAlertDisabled(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "silence_test.db"))
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

	bFalse := false
	item, err := service.CreateItem(ctx, core.ItemInput{
		GroupID:              group.ID,
		SourceType:           "passive",
		Name:                 "node-1",
		IntervalSeconds:      10,
		MissedTimesThreshold: 3,
		AlertEnabled:         &bFalse,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "$.cpu.load",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "cpu high",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &item.ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "$.cpu.load",
		Operator:         "gt",
		ThresholdValue:   "1",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "host",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"cpu": map[string]interface{}{"load": "3.1"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events len = %d, want 0 (silenced alerts)", len(events))
	}
}

func TestPassiveReceiveBatchUpload(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "batch_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	service := core.NewService(store)

	// Create Group A and B
	groupA, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_a",
		Name:                   "Group A",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	groupB, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_b",
		Name:                   "Group B",
		DefaultIntervalSeconds: 20,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Payload with no root group/name, but 2 items in array
	payload := core.PassivePayload{
		Interval:  15,
		Timestamp: time.Now().Unix(),
		Items: []core.PassiveSubItem{
			{
				Group: "group_a",
				Name:  "device-1",
				Data:  map[string]interface{}{"temp": 25.5},
			},
			{
				Group: "group_b",
				Name:  "device-2",
				Data:  map[string]interface{}{"temp": 28.0},
			},
		},
	}

	interval, _, err := service.ReceivePassive(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}

	// It should return the last processed item's interval (groupB default interval or payload interval?)
	// Wait, UpsertItem gets payload.Interval (15). Since groupB default interval is 20, but the item's interval was set to payload.Interval (15).
	if interval != 15 {
		t.Fatalf("returned interval = %d, want 15", interval)
	}

	// Verify items are created in both groups
	itemsA, err := service.Items(ctx, groupA.ID)
	if err != nil || len(itemsA) != 1 || itemsA[0].Name != "device-1" {
		t.Fatalf("unexpected items in Group A: %#v", itemsA)
	}

	itemsB, err := service.Items(ctx, groupB.ID)
	if err != nil || len(itemsB) != 1 || itemsB[0].Name != "device-2" {
		t.Fatalf("unexpected items in Group B: %#v", itemsB)
	}

	// Now send payload WITH root and sub-items
	payloadWithRoot := core.PassivePayload{
		Group:     "group_a",
		Name:      "parent-device",
		Interval:  12,
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{"status": "active"},
		Items: []core.PassiveSubItem{
			{
				Group: "group_b",
				Name:  "device-2",
				Data:  map[string]interface{}{"temp": 30.0},
			},
		},
	}

	intervalWithRoot, _, err := service.ReceivePassive(ctx, payloadWithRoot)
	if err != nil {
		t.Fatal(err)
	}

	// Should return root's interval (12) instead of sub-item's interval
	if intervalWithRoot != 12 {
		t.Fatalf("returned interval with root = %d, want 12", intervalWithRoot)
	}
}

func TestPassiveReceiveSettingsResolution(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "settings_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	service := core.NewService(store)

	// Create Group A with some settings
	groupA, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_a",
		Name:                   "Group A",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
		ResponseSettingsJSON:   `{"key1":"groupVal1","key2":"groupVal2"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: Item doesn't have settings, should inherit Group's settings
	payloadInherited := core.PassivePayload{
		Group:    "group_a",
		Name:     "item-inherited",
		Interval: 10,
		Data:     map[string]interface{}{"temp": 25.5},
	}
	_, settingInherited, err := service.ReceivePassive(ctx, payloadInherited)
	if err != nil {
		t.Fatal(err)
	}
	if settingInherited["key1"] != "groupVal1" || settingInherited["key2"] != "groupVal2" {
		t.Fatalf("expected inherited settings, got: %#v", settingInherited)
	}

	// Fetch created item to modify settings
	items, err := service.Items(ctx, groupA.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 item, got: %#v", items)
	}
	itemInherited := items[0]

	// Update item settings to override group settings
	bTrue := true
	_, err = service.UpdateItem(ctx, itemInherited.ID, core.ItemInput{
		GroupID:              groupA.ID,
		SourceType:           "passive",
		Name:                 itemInherited.Name,
		IntervalSeconds:      itemInherited.IntervalSeconds,
		MissedTimesThreshold: itemInherited.MissedTimesThreshold,
		AlertEnabled:         &bTrue,
		Enabled:              &bTrue,
		ResponseSettingsJSON:   `{"key1":"overrideVal1"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Case 2: Item has settings, should override (only item settings returned)
	_, settingOverridden, err := service.ReceivePassive(ctx, payloadInherited)
	if err != nil {
		t.Fatal(err)
	}
	if settingOverridden["key1"] != "overrideVal1" {
		t.Fatalf("expected overridden setting key1=overrideVal1, got: %#v", settingOverridden)
	}
	if _, exists := settingOverridden["key2"]; exists {
		t.Fatalf("expected key2 to not exist in overridden setting, got: %#v", settingOverridden)
	}

	// Case 3: Items in the array should be ignored for settings evaluation
	payloadBatchOnly := core.PassivePayload{
		Interval: 10,
		Items: []core.PassiveSubItem{
			{
				Group: "group_a",
				Name:  "item-inherited",
				Data:  map[string]interface{}{"temp": 25.5},
			},
		},
	}
	_, settingBatch, err := service.ReceivePassive(ctx, payloadBatchOnly)
	if err != nil {
		t.Fatal(err)
	}
	if len(settingBatch) != 0 {
		t.Fatalf("expected empty settings for batch upload, got: %#v", settingBatch)
	}
}

func TestPassiveReceiveObjectArray(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "object_array_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	service := core.NewService(store)

	// 1. Create target group for sub-items
	childGroup, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "child_devices",
		Name:                   "Child Devices",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Create parent group
	parentGroup, err := service.CreateGroup(ctx, core.GroupInput{
		Code:                   "parent_gateways",
		Name:                   "Parent Gateways",
		DefaultIntervalSeconds: 15,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Setup a field definition under parent group that contains object_array, mapping to childGroup
	_, err = service.UpsertField(ctx, core.FieldInput{
		ScopeType:   "group",
		GroupID:     parentGroup.ID,
		FieldPath:   "$.sub_devices",
		ValueType:   "object_array",
		RefGroupID:  &childGroup.ID,
		RefNamePath: "device_name",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 4. Setup a field definition under child group to check nested property coercing
	_, err = service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   childGroup.ID,
		FieldPath: "$.temperature",
		ValueType: "float",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 5. Setup alert rule under child group
	_, err = service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "temp high warning",
		ScopeType:        "group",
		GroupID:          &childGroup.ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "$.temperature",
		Operator:         "gt",
		ThresholdValue:   "37.5",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 6. Call ReceivePassive with nested object_array under parent group
	payload := core.PassivePayload{
		Group:    "parent_gateways",
		Name:     "gateway-01",
		Interval: 15,
		Data: map[string]interface{}{
			"gateway_status": "ok",
			"sub_devices": []interface{}{
				map[string]interface{}{"device_name": "dev-alpha", "temperature": 36.2},
				map[string]interface{}{"device_name": "dev-beta", "temperature": 38.5},
			},
		},
	}

	_, _, err = service.ReceivePassive(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}

	// 7. Verify items are automatically created under child group
	childItems, err := service.Items(ctx, childGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(childItems) != 2 {
		t.Fatalf("expected 2 child items, got %d", len(childItems))
	}

	// Find parent gateway item ID
	parentItems, err := service.Items(ctx, parentGroup.ID)
	if err != nil || len(parentItems) != 1 {
		t.Fatalf("expected 1 parent item, got error: %v, items: %+v", err, parentItems)
	}
	parentItemID := parentItems[0].ID

	// Find the names
	var hasAlpha, hasBeta bool
	var betaID int64
	for _, it := range childItems {
		if it.Name == "dev-alpha" {
			hasAlpha = true
		}
		if it.Name == "dev-beta" {
			hasBeta = true
			betaID = it.ID
		}
		// Check ref_item_id and ref_item_name
		if it.RefItemID == nil || *it.RefItemID != parentItemID {
			t.Fatalf("expected ref_item_id %d, got %v", parentItemID, it.RefItemID)
		}
		if it.RefItemName != "gateway-01" {
			t.Fatalf("expected ref_item_name 'gateway-01', got '%s'", it.RefItemName)
		}
	}
	if !hasAlpha || !hasBeta {
		t.Fatalf("expected items for dev-alpha and dev-beta, got items: %+v", childItems)
	}

	// 8. Verify the sample value temperature under dev-beta was saved and evaluated
	samples, err := service.Samples(ctx, childGroup.ID, betaID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || len(samples[0].Values) == 0 {
		t.Fatalf("expected sample with values for dev-beta, got %+v", samples)
	}
	var tempVal *float64
	for _, v := range samples[0].Values {
		if v.FieldPath == "$.temperature" {
			tempVal = v.FloatValue
		}
	}
	if tempVal == nil || *tempVal != 38.5 {
		t.Fatalf("expected temperature 38.5, got %v", tempVal)
	}

	// 9. Verify high temperature warning alert was triggered
	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	var hasTriggered bool
	for _, ev := range events {
		if ev.EventType == "triggered" && ev.ItemID != nil && *ev.ItemID == betaID && strings.Contains(ev.Message, "temp high warning") {
			hasTriggered = true
		}
	}
	if !hasTriggered {
		t.Fatalf("expected a triggered alert for dev-beta warning, got events: %+v", events)
	}
}

func TestGroupSortOrder(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "sort_order_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, migrations.InstallSQL); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	service := core.NewService(store)

	// Create groups with different sort orders
	// Group A (sort_order = 10)
	_, err = service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_a",
		Name:                   "Group A",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
		SortOrder:              10,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Group B (sort_order = 5)
	_, err = service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_b",
		Name:                   "Group B",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
		SortOrder:              5,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Group C (sort_order = 20)
	_, err = service.CreateGroup(ctx, core.GroupInput{
		Code:                   "group_c",
		Name:                   "Group C",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
		SortOrder:              20,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve groups list and verify order
	groups, err := service.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Verify order: Group B (5) -> Group A (10) -> Group C (20)
	if groups[0].Code != "group_b" || groups[0].SortOrder != 5 {
		t.Errorf("expected group_b at index 0, got %s (sort_order=%d)", groups[0].Code, groups[0].SortOrder)
	}
	if groups[1].Code != "group_a" || groups[1].SortOrder != 10 {
		t.Errorf("expected group_a at index 1, got %s (sort_order=%d)", groups[1].Code, groups[1].SortOrder)
	}
	if groups[2].Code != "group_c" || groups[2].SortOrder != 20 {
		t.Errorf("expected group_c at index 2, got %s (sort_order=%d)", groups[2].Code, groups[2].SortOrder)
	}
}

func TestCombineAlertGroupLogic(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "combine_test.db"))
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
		Code:                   "combine_group_test",
		Name:                   "Combine Group Test",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "val1",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "val2",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	// Create passive item
	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "combine_group_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 5.0,
			"val2": 5.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	items, err := service.Items(ctx, group.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 item, got error: %v, items: %+v", err, items)
	}
	itemID := items[0].ID

	// Create Rule 1 with CombineGroup
	_, err = service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "val1 high",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &itemID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "val1",
		Operator:         "gt",
		ThresholdValue:   "10",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
		CombineGroup:     "my_combine_group",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create Rule 2 with CombineGroup
	_, err = service.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "val2 high",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &itemID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "val2",
		Operator:         "gt",
		ThresholdValue:   "20",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
		CombineGroup:     "my_combine_group",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Evaluate first sample: val1 = 15, val2 = 5 (Rule 1 matched, Rule 2 unmatched) -> Transient state, should NOT alert.
	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "combine_group_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 15.0,
			"val2": 5.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events during transient hit state, got: %+v", events)
	}

	// 2. Evaluate second sample: val1 = 15, val2 = 25 (Rule 1 matched, Rule 2 matched) -> All matched state, should trigger alerts for both.
	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "combine_group_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 15.0,
			"val2": 25.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after all matched state, got: %+v", events)
	}
	for _, ev := range events {
		if ev.EventType != "triggered" {
			t.Fatalf("expected event type to be triggered, got: %s", ev.EventType)
		}
	}

	// Clear events from DB for easy counts in subsequent steps
	if _, err := db.ExecContext(ctx, `DELETE FROM alert_events`); err != nil {
		t.Fatal(err)
	}

	// 3. Evaluate third sample: val1 = 5, val2 = 25 (Rule 1 unmatched, Rule 2 matched) -> Transient state, should NOT recover.
	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "combine_group_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 5.0,
			"val2": 25.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events during transient recovery state, got: %+v", events)
	}

	// 4. Evaluate fourth sample: val1 = 5, val2 = 5 (Rule 1 unmatched, Rule 2 unmatched) -> All recovered state, should trigger recovery for both.
	if _, _, err := service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "combine_group_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 5.0,
			"val2": 5.0,
		},
	}); err != nil {
		t.Fatal(err)
	}

	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after all recovered state, got: %+v", events)
	}
	for _, ev := range events {
		if ev.EventType != "recovered" {
			t.Fatalf("expected event type to be recovered, got: %s", ev.EventType)
		}
	}
}

func TestRuleEvaluationStatusSkipping(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "status_skip_test.db"))
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
		Code:                   "status_skip_test",
		Name:                   "Status Skip Test",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	item, err := service.CreateItem(ctx, core.ItemInput{
		GroupID:         group.ID,
		Name:            "node-1",
		SourceType:      "passive",
		IntervalSeconds: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "val1",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	bTrue := true
	// Create a field condition rule
	rule, err := store.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "Val1 High",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &item.ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "val1",
		ValueType:        "float",
		Operator:         "gt",
		ThresholdValue:   "10.0",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
		MessageTemplate:  "val1 is {{current}}",
		Enabled:          &bTrue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Trigger the alarm
	_, _, err = service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "status_skip_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 15.0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].RuleID != rule.ID || events[0].EventType != "triggered" {
		t.Fatalf("expected 1 triggered event, got: %+v", events)
	}

	// Clear events from DB for counting
	if _, err := db.ExecContext(ctx, `DELETE FROM alert_events`); err != nil {
		t.Fatal(err)
	}

	// 2. Evaluate a missing sample (this sets Status to "missing")
	// Save a missing sample and evaluate it
	sample, err := store.SaveSample(ctx, core.SaveSampleInput{
		GroupID:         group.ID,
		ItemID:          item.ID,
		SourceType:      item.SourceType,
		Name:            item.Name,
		IntervalSeconds: 10,
		Status:          "missing",
		Raw:             map[string]interface{}{"message": "data missing"},
		ErrorMessage:    "data missing",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.EvaluateSample(ctx, sample); err != nil {
		t.Fatal(err)
	}

	// Verify that NO recovery event has been generated, and rule state remains alerting
	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got: %+v", events)
	}

	var statusStr string
	err = db.QueryRowContext(ctx, "SELECT status FROM alert_states WHERE rule_id = ? AND item_id = ?", rule.ID, item.ID).Scan(&statusStr)
	if err != nil {
		t.Fatal(err)
	}
	if statusStr != "alerting" {
		t.Fatalf("expected rule to remain alerting, got: %s", statusStr)
	}

	// 3. Send a normal value below threshold (val1 = 5.0) -> should recover
	_, _, err = service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "status_skip_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 5.0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].RuleID != rule.ID || events[0].EventType != "recovered" {
		t.Fatalf("expected 1 recovered event, got: %+v", events)
	}
}

func TestRuleEvaluationJSONThreshold(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "json_threshold_test.db"))
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
		Code:                   "json_threshold_test",
		Name:                   "JSON Threshold Test",
		DefaultIntervalSeconds: 10,
		MissedTimesThreshold:   3,
	})
	if err != nil {
		t.Fatal(err)
	}

	item, err := service.CreateItem(ctx, core.ItemInput{
		GroupID:         group.ID,
		Name:            "node-1",
		SourceType:      "passive",
		IntervalSeconds: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.UpsertField(ctx, core.FieldInput{
		ScopeType: "group",
		GroupID:   group.ID,
		FieldPath: "val1",
		ValueType: "float",
	}); err != nil {
		t.Fatal(err)
	}

	bTrue := true
	rule, err := store.UpsertRule(ctx, core.AlertRuleInput{
		Name:             "Dynamic Val1 High",
		ScopeType:        "item",
		GroupID:          &group.ID,
		ItemID:           &item.ID,
		SourceType:       "passive",
		RuleType:         "field_condition",
		FieldPath:        "val1",
		ValueType:        "float",
		Operator:         "gt",
		ThresholdValue:   "json:data.threshold_limit",
		ConsecutiveCount: 1,
		RecoveryCount:    1,
		Severity:         "warning",
		MessageTemplate:  "val1 is {{current}}",
		Enabled:          &bTrue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Send data where val1 (15.0) > threshold_limit (10.0) -> should trigger alarm
	_, _, err = service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "json_threshold_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 15.0,
			"threshold_limit": 10.0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _, err := service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].RuleID != rule.ID || events[0].EventType != "triggered" {
		t.Fatalf("expected 1 triggered event, got: %+v", events)
	}
	if events[0].ThresholdValue != "json:data.threshold_limit(10)" {
		t.Fatalf("expected ThresholdValue to be formatted as json:data.threshold_limit(10), got: %s", events[0].ThresholdValue)
	}

	// Clear events from DB for counting
	if _, err := db.ExecContext(ctx, `DELETE FROM alert_events`); err != nil {
		t.Fatal(err)
	}

	// 2. Send data where val1 (8.0) <= threshold_limit (10.0) -> should recover alarm
	_, _, err = service.ReceivePassive(ctx, core.PassivePayload{
		Group:    "json_threshold_test",
		Name:     "node-1",
		Interval: 10,
		Data: map[string]interface{}{
			"val1": 8.0,
			"threshold_limit": 10.0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _, err = service.Events(ctx, 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].RuleID != rule.ID || events[0].EventType != "recovered" {
		t.Fatalf("expected 1 recovered event, got: %+v", events)
	}
	if events[0].ThresholdValue != "json:data.threshold_limit(10)" {
		t.Fatalf("expected ThresholdValue to be formatted as json:data.threshold_limit(10), got: %s", events[0].ThresholdValue)
	}
}
