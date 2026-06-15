package core

import "testing"

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
