package core

import (
	"strings"
	"testing"
)

func TestCompareValueContainsMultipleThresholds(t *testing.T) {
	value := SampleValue{ValueType: "string", StringValue: "api timeout from upstream"}

	if !compareValue(value, "contains", "offline, timeout") {
		t.Fatal("contains should match any comma-separated threshold")
	}
	if compareValue(value, "not_contains", "offline, timeout") {
		t.Fatal("not_contains should fail when any threshold is present")
	}
	if !compareValue(value, "not_contains", "offline, refused") {
		t.Fatal("not_contains should pass when no threshold is present")
	}
}

func TestCoerceSampleValueStringArrayAndObjectArray(t *testing.T) {
	// 1. Test string_array with []interface{}
	val1 := coerceSampleValue("$.my_field", "string_array", []interface{}{"a", 123, true})
	if val1.ValueType != "string_array" {
		t.Errorf("expected value type string_array, got %s", val1.ValueType)
	}
	if val1.StringValue != `["a","123","true"]` {
		t.Errorf("expected string_array JSON, got %s", val1.StringValue)
	}

	// 2. Test string_array with string
	val2 := coerceSampleValue("$.my_field", "string_array", `["x","y"]`)
	if val2.StringValue != `["x","y"]` {
		t.Errorf("expected string_array JSON from JSON string, got %s", val2.StringValue)
	}

	// 3. Test string_array with plain string
	val3 := coerceSampleValue("$.my_field", "string_array", "hello")
	if val3.StringValue != `["hello"]` {
		t.Errorf("expected single-element string_array JSON from plain string, got %s", val3.StringValue)
	}

	// 4. Test object_array
	objs := []interface{}{
		map[string]interface{}{"name": "a", "val": 1},
		map[string]interface{}{"name": "b", "val": 2},
	}
	val4 := coerceSampleValue("$.my_field", "object_array", objs)
	if val4.ValueType != "object_array" {
		t.Errorf("expected value type object_array, got %s", val4.ValueType)
	}
	// Verify it marshals properly
	if !strings.Contains(val4.StringValue, `"name":"a"`) || !strings.Contains(val4.StringValue, `"val":2`) {
		t.Errorf("expected object_array JSON string, got %s", val4.StringValue)
	}
}

func TestCompareValueStringArray(t *testing.T) {
	tests := []struct {
		jsonVal   string
		operator  string
		threshold string
		want      bool
	}{
		// len_eq
		{`["a","b"]`, "len_eq", "2", true},
		{`["a","b"]`, "len_eq", "3", false},
		{`[]`, "len_eq", "0", true},
		// len_gt
		{`["a","b","c"]`, "len_gt", "2", true},
		{`["a","b","c"]`, "len_gt", "3", false},
		// len_lt
		{`["a"]`, "len_lt", "2", true},
		{`["a","b"]`, "len_lt", "2", false},
		// len_ne
		{`["a","b"]`, "len_ne", "3", true},
		{`["a","b"]`, "len_ne", "2", false},
		// contains
		{`["apple","banana"]`, "contains", "app", true},
		{`["apple","banana"]`, "contains", "nan", true},
		{`["apple","banana"]`, "contains", "orange, app", true}, // supports comma-separated
		{`["apple","banana"]`, "contains", "orange", false},
		// not_contains
		{`["apple","banana"]`, "not_contains", "orange", true},
		{`["apple","banana"]`, "not_contains", "apple", false},
		{`["apple","banana"]`, "not_contains", "orange, banana", false},
	}

	for _, tt := range tests {
		val := SampleValue{ValueType: "string_array", StringValue: tt.jsonVal}
		got := compareValue(val, tt.operator, tt.threshold)
		if got != tt.want {
			t.Errorf("compareValue(%s, %s, %s) = %v; want %v", tt.jsonVal, tt.operator, tt.threshold, got, tt.want)
		}
	}
}
