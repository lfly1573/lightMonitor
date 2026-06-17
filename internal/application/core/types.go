package core

import "time"

type Setting struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type"`
	Description string `json:"description"`
}

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
	LastLoginAt string `json:"last_login_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type UserInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Enabled     *bool  `json:"enabled"`
}

type Group struct {
	ID                     int64  `json:"id"`
	Code                   string `json:"code"`
	Name                   string `json:"name"`
	Icon                   string `json:"icon"`
	Description            string `json:"description"`
	DefaultIntervalSeconds int    `json:"default_interval_seconds"`
	MissedTimesThreshold   int    `json:"missed_times_threshold"`
	AlertEnabled           bool   `json:"alert_enabled"`
	Enabled                bool   `json:"enabled"`
	ResponseSettingsJSON   string `json:"response_settings_json"`
	SortOrder              int    `json:"sort_order"`
	CreatedAt              string `json:"created_at,omitempty"`
	UpdatedAt              string `json:"updated_at,omitempty"`
}

type GroupInput struct {
	Code                   string `json:"code"`
	Name                   string `json:"name"`
	Icon                   string `json:"icon"`
	Description            string `json:"description"`
	DefaultIntervalSeconds int    `json:"default_interval_seconds"`
	MissedTimesThreshold   int    `json:"missed_times_threshold"`
	AlertEnabled           *bool  `json:"alert_enabled"`
	Enabled                *bool  `json:"enabled"`
	ResponseSettingsJSON   string `json:"response_settings_json"`
	SortOrder              int    `json:"sort_order"`
}

type Item struct {
	ID                   int64  `json:"id"`
	GroupID              int64  `json:"group_id"`
	SourceType           string `json:"source_type"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	IntervalSeconds      int    `json:"interval_seconds"`
	MissedTimesThreshold int    `json:"missed_times_threshold"`
	AlertEnabled         bool   `json:"alert_enabled"`
	Enabled                bool   `json:"enabled"`
	ResponseSettingsJSON   string `json:"response_settings_json"`
	RefItemID            *int64 `json:"ref_item_id,omitempty"`
	RefItemName          string `json:"ref_item_name,omitempty"`
	LastSeenAt           string `json:"last_seen_at,omitempty"`
	CreatedAt            string `json:"created_at,omitempty"`
}

type ItemInput struct {
	GroupID              int64  `json:"group_id"`
	SourceType           string `json:"source_type"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	IntervalSeconds      int    `json:"interval_seconds"`
	MissedTimesThreshold int    `json:"missed_times_threshold"`
	AlertEnabled         *bool  `json:"alert_enabled"`
	Enabled              *bool  `json:"enabled"`
	ResponseSettingsJSON   string `json:"response_settings_json"`
	RefItemID            *int64 `json:"ref_item_id,omitempty"`
}

type ActiveRequest struct {
	ID                 int64  `json:"id"`
	GroupID            int64  `json:"group_id"`
	ItemID             int64  `json:"item_id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	Method             string `json:"method"`
	HeadersJSON        string `json:"headers_json"`
	BodyType           string `json:"body_type"`
	BodyJSON           string `json:"body_json"`
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	ExpectedStatusCode int    `json:"expected_status_code"`
	Enabled            bool   `json:"enabled"`
	LastSeenAt         string `json:"last_seen_at,omitempty"`
}

type ActiveRequestInput struct {
	GroupID            int64  `json:"group_id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	Method             string `json:"method"`
	HeadersJSON        string `json:"headers_json"`
	BodyType           string `json:"body_type"`
	BodyJSON           string `json:"body_json"`
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	ExpectedStatusCode int    `json:"expected_status_code"`
	Enabled            *bool  `json:"enabled"`
}

type FieldDefinition struct {
	ID          int64  `json:"id"`
	ScopeType   string `json:"scope_type"`
	GroupID     int64  `json:"group_id"`
	ItemID      *int64 `json:"item_id,omitempty"`
	FieldPath   string `json:"field_path"`
	DisplayName string `json:"display_name"`
	ValueType   string `json:"value_type"`
	Unit        string `json:"unit"`
	Required    bool   `json:"required"`
	Enabled     bool   `json:"enabled"`
	RefGroupID  *int64 `json:"ref_group_id,omitempty"`
	RefNamePath string `json:"ref_name_path,omitempty"`
}

type FieldInput struct {
	ID          int64  `json:"id"`
	ScopeType   string `json:"scope_type"`
	GroupID     int64  `json:"group_id"`
	ItemID      *int64 `json:"item_id"`
	FieldPath   string `json:"field_path"`
	DisplayName string `json:"display_name"`
	ValueType   string `json:"value_type"`
	Unit        string `json:"unit"`
	Required    *bool  `json:"required"`
	Enabled     *bool  `json:"enabled"`
	RefGroupID  *int64 `json:"ref_group_id"`
	RefNamePath string `json:"ref_name_path"`
}

type Channel struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Type       string `json:"channel_type"`
	ConfigJSON string `json:"config_json"`
	Enabled    bool   `json:"enabled"`
	IsDefault  bool   `json:"is_default"`
}

type ChannelInput struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Type       string `json:"channel_type"`
	ConfigJSON string `json:"config_json"`
	Enabled    *bool  `json:"enabled"`
	IsDefault  *bool  `json:"is_default"`
}

type AlertRule struct {
	ID                     int64   `json:"id"`
	Name                   string  `json:"name"`
	ScopeType              string  `json:"scope_type"`
	GroupID                *int64  `json:"group_id,omitempty"`
	ItemID                 *int64  `json:"item_id,omitempty"`
	FieldDefinitionID      *int64  `json:"field_definition_id,omitempty"`
	SourceType             string  `json:"source_type"`
	RuleType               string  `json:"rule_type"`
	FieldPath              string  `json:"field_path"`
	ValueType              string  `json:"value_type"`
	Operator               string  `json:"operator"`
	ThresholdValue         string  `json:"threshold_value"`
	AggregateFunc          string  `json:"aggregate_func"`
	AggregateWindowSeconds *int    `json:"aggregate_window_seconds,omitempty"`
	AggregateSampleCount   *int    `json:"aggregate_sample_count,omitempty"`
	ConsecutiveCount       int     `json:"consecutive_count"`
	RecoveryCount          int     `json:"recovery_count"`
	Severity               string  `json:"severity"`
	MessageTemplate        string  `json:"message_template"`
	Enabled                bool    `json:"enabled"`
	ChannelIDs             []int64 `json:"channel_ids,omitempty"`
}

type AlertRuleInput struct {
	ID                     int64   `json:"id"`
	Name                   string  `json:"name"`
	ScopeType              string  `json:"scope_type"`
	GroupID                *int64  `json:"group_id,omitempty"`
	ItemID                 *int64  `json:"item_id,omitempty"`
	FieldDefinitionID      *int64  `json:"field_definition_id,omitempty"`
	SourceType             string  `json:"source_type"`
	RuleType               string  `json:"rule_type"`
	FieldPath              string  `json:"field_path"`
	ValueType              string  `json:"value_type"`
	Operator               string  `json:"operator"`
	ThresholdValue         string  `json:"threshold_value"`
	AggregateFunc          string  `json:"aggregate_func"`
	AggregateWindowSeconds *int    `json:"aggregate_window_seconds,omitempty"`
	AggregateSampleCount   *int    `json:"aggregate_sample_count,omitempty"`
	ConsecutiveCount       int     `json:"consecutive_count"`
	RecoveryCount          int     `json:"recovery_count"`
	Severity               string  `json:"severity"`
	MessageTemplate        string  `json:"message_template"`
	Enabled                *bool   `json:"enabled"`
	ChannelIDs             []int64 `json:"channel_ids,omitempty"`
}

type PassiveSubItem struct {
	Group string                 `json:"group"`
	Name  string                 `json:"name"`
	Data  map[string]interface{} `json:"data"`
}

type PassivePayload struct {
	Group     string                 `json:"group"`
	Name      string                 `json:"name"`
	Token     string                 `json:"token"`
	Timestamp interface{}            `json:"timestamp"`
	Interval  int                    `json:"interval"`
	Data      map[string]interface{} `json:"data"`
	Items     []PassiveSubItem       `json:"items"`
}

type Sample struct {
	ID              int64                  `json:"id"`
	GroupID         int64                  `json:"group_id"`
	ItemID          int64                  `json:"item_id"`
	SourceType      string                 `json:"source_type"`
	Name            string                 `json:"name"`
	ReportedAt      string                 `json:"reported_at,omitempty"`
	ReceivedAt      string                 `json:"received_at"`
	IntervalSeconds int                    `json:"interval_seconds,omitempty"`
	Status          string                 `json:"status"`
	HTTPStatusCode  int                    `json:"http_status_code,omitempty"`
	LatencyMS       int64                  `json:"latency_ms,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Raw             map[string]interface{} `json:"raw,omitempty"`
	Values          []SampleValue          `json:"values,omitempty"`
}

type SampleValue struct {
	FieldPath    string      `json:"field_path"`
	ValueType    string      `json:"value_type"`
	StringValue  string      `json:"string_value,omitempty"`
	IntegerValue *int64      `json:"integer_value,omitempty"`
	FloatValue   *float64    `json:"float_value,omitempty"`
	BooleanValue *bool       `json:"boolean_value,omitempty"`
	NumericValue *float64    `json:"numeric_value,omitempty"`
	RawValue     interface{} `json:"raw_value,omitempty"`
}

type SaveSampleInput struct {
	GroupID         int64
	ItemID          int64
	SourceType      string
	ActiveRequestID *int64
	Name            string
	ReportedAt      *time.Time
	IntervalSeconds int
	Status          string
	HTTPStatusCode  int
	LatencyMS       int64
	Raw             map[string]interface{}
	ErrorMessage    string
}

type AlertEvent struct {
	ID             int64  `json:"id"`
	RuleID         int64  `json:"rule_id"`
	GroupID        *int64 `json:"group_id,omitempty"`
	ItemID         *int64 `json:"item_id,omitempty"`
	SampleID       *int64 `json:"sample_id,omitempty"`
	EventType      string `json:"event_type"`
	Severity       string `json:"severity"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	FieldPath      string `json:"field_path"`
	CurrentValue   string `json:"current_value,omitempty"`
	ThresholdValue string `json:"threshold_value,omitempty"`
	OccurredAt     string `json:"occurred_at"`
}

type StatResult struct {
	GroupID     int64       `json:"group_id"`
	ItemID      int64       `json:"item_id"`
	FieldPath   string      `json:"field_path"`
	Count       int         `json:"count"`
	Avg         *float64    `json:"avg,omitempty"`
	Max         *float64    `json:"max,omitempty"`
	Min         *float64    `json:"min,omitempty"`
	Median      *float64    `json:"median,omitempty"`
	Latest      interface{} `json:"latest,omitempty"`
	LatestAt    string      `json:"latest_at,omitempty"`
	Series      []Point     `json:"series,omitempty"`
	GeneratedAt string      `json:"generated_at"`
}

type Point struct {
	Time  string   `json:"time"`
	Value *float64 `json:"value,omitempty"`
}

type Dashboard struct {
	Groups        int64        `json:"groups"`
	Items         int64        `json:"items"`
	Samples24h    int64        `json:"samples_24h"`
	AlertingRules int64        `json:"alerting_rules"`
	RecentEvents  []AlertEvent `json:"recent_events"`
}
