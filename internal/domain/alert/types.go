package alert

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

type Channel struct {
	ID         int64
	Code       string
	Name       string
	Type       string
	ConfigJSON string
	Enabled    bool
	IsDefault  bool
}

type Rule struct {
	ID               int64
	Name             string
	ScopeType        string
	RuleType         string
	FieldPath        string
	Operator         string
	ThresholdValue   string
	ConsecutiveCount int
	RecoveryCount    int
	Severity         string
	Enabled          bool
}
