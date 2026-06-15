package monitor

import "time"

const (
	SourceTypePassive = "passive"
	SourceTypeActive  = "active"
)

type Group struct {
	ID                     int64
	Code                   string
	Name                   string
	Icon                   string
	DefaultIntervalSeconds int
	MissedTimesThreshold   int
	AlertEnabled           bool
	Enabled                bool
}

type Item struct {
	ID                   int64
	GroupID              int64
	SourceType           string
	Name                 string
	IntervalSeconds      int
	MissedTimesThreshold int
	AlertEnabled         bool
	Enabled              bool
	LastSeenAt           *time.Time
}

type FieldDefinition struct {
	ID          int64
	ScopeType   string
	GroupID     int64
	ItemID      *int64
	FieldPath   string
	DisplayName string
	ValueType   string
	Unit        string
	Required    bool
	Enabled     bool
}
