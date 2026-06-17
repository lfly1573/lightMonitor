package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"lightmonitor/internal/domain/system"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
)

type Store interface {
	Authenticate(ctx context.Context, username string) (system.User, error)
	TouchLogin(ctx context.Context, userID int64) error
	ListSettings(ctx context.Context) ([]Setting, error)
	UpdateSettings(ctx context.Context, settings []Setting) error
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, input UserInput) (User, error)
	UpdateUser(ctx context.Context, id int64, input UserInput) (User, error)
	DeleteUser(ctx context.Context, id int64) error

	ListGroups(ctx context.Context) ([]Group, error)
	CreateGroup(ctx context.Context, input GroupInput) (Group, error)
	UpdateGroup(ctx context.Context, id int64, input GroupInput) (Group, error)
	DeleteGroup(ctx context.Context, id int64) error
	GetGroupByCode(ctx context.Context, code string) (Group, error)
	GetGroupByID(ctx context.Context, id int64) (Group, error)

	ListItems(ctx context.Context, groupID int64) ([]Item, error)
	GetItemByID(ctx context.Context, id int64) (Item, error)
	UpsertItem(ctx context.Context, input ItemInput) (Item, error)
	UpdateItem(ctx context.Context, id int64, input ItemInput) (Item, error)
	DeleteItem(ctx context.Context, id int64) error

	ListActiveRequests(ctx context.Context) ([]ActiveRequest, error)
	CreateActiveRequest(ctx context.Context, input ActiveRequestInput) (ActiveRequest, error)
	UpdateActiveRequest(ctx context.Context, id int64, input ActiveRequestInput) (ActiveRequest, error)
	DeleteActiveRequest(ctx context.Context, id int64) error

	ListFields(ctx context.Context, groupID, itemID int64) ([]FieldDefinition, error)
	UpsertField(ctx context.Context, input FieldInput) (FieldDefinition, error)
	DeleteField(ctx context.Context, id int64) error

	ListChannels(ctx context.Context) ([]Channel, error)
	UpsertChannel(ctx context.Context, input ChannelInput) (Channel, error)
	DeleteChannel(ctx context.Context, id int64) error

	ListRules(ctx context.Context) ([]AlertRule, error)
	UpsertRule(ctx context.Context, input AlertRuleInput) (AlertRule, error)
	DeleteRule(ctx context.Context, id int64) error

	SaveSample(ctx context.Context, input SaveSampleInput, values []SampleValue) (Sample, error)
	LastSample(ctx context.Context, itemID int64) (Sample, error)
	ListSamples(ctx context.Context, groupID, itemID int64, limit int) ([]Sample, error)
	Stats(ctx context.Context, groupID, itemID int64, fieldPath string, since time.Time) (StatResult, error)

	AlertRulesForSample(ctx context.Context, sample Sample) ([]AlertRule, error)
	WindowValues(ctx context.Context, itemID int64, fieldPath string, since time.Time, limit int) ([]float64, error)
	ApplyAlertEvaluation(ctx context.Context, rule AlertRule, sample Sample, matched bool, currentValue, threshold string) (*AlertEvent, error)
	ListEvents(ctx context.Context, limit int) ([]AlertEvent, error)
	ListEnabledChannelsForRule(ctx context.Context, ruleID int64) ([]Channel, error)
	CreateNotification(ctx context.Context, eventID, channelID int64, status, requestJSON, responseText, errorMessage string) error
	Dashboard(ctx context.Context) (Dashboard, error)
	Cleanup(ctx context.Context, before time.Time) error
}

type Service struct {
	store      Store
	httpClient *http.Client
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *Service) Authenticate(ctx context.Context, username, password string) (User, error) {
	u, err := s.store.Authenticate(ctx, strings.TrimSpace(username))
	if err != nil {
		return User{}, err
	}
	if !u.Enabled || !system.VerifyPassword(password, u.PasswordHash) {
		return User{}, ErrUnauthorized
	}
	if err := s.store.TouchLogin(ctx, u.ID); err != nil {
		return User{}, err
	}
	return User{ID: u.ID, Username: u.Username, Role: u.Role, DisplayName: u.DisplayName, Enabled: u.Enabled}, nil
}

func (s *Service) Settings(ctx context.Context) ([]Setting, error) {
	return s.store.ListSettings(ctx)
}

func (s *Service) UpdateSettings(ctx context.Context, settings []Setting) error {
	return s.store.UpdateSettings(ctx, settings)
}

func (s *Service) Users(ctx context.Context) ([]User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) CreateUser(ctx context.Context, input UserInput) (User, error) {
	return s.store.CreateUser(ctx, normalizeUserInput(input))
}

func (s *Service) UpdateUser(ctx context.Context, id int64, input UserInput) (User, error) {
	return s.store.UpdateUser(ctx, id, normalizeUserInput(input))
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.store.DeleteUser(ctx, id)
}

func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	return s.store.ListGroups(ctx)
}

func (s *Service) CreateGroup(ctx context.Context, input GroupInput) (Group, error) {
	return s.store.CreateGroup(ctx, normalizeGroupInput(input))
}

func (s *Service) UpdateGroup(ctx context.Context, id int64, input GroupInput) (Group, error) {
	return s.store.UpdateGroup(ctx, id, normalizeGroupInput(input))
}

func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	return s.store.DeleteGroup(ctx, id)
}

func (s *Service) Items(ctx context.Context, groupID int64) ([]Item, error) {
	return s.store.ListItems(ctx, groupID)
}

func (s *Service) CreateItem(ctx context.Context, input ItemInput) (Item, error) {
	return s.store.UpsertItem(ctx, normalizeItemInput(input))
}

func (s *Service) UpdateItem(ctx context.Context, id int64, input ItemInput) (Item, error) {
	return s.store.UpdateItem(ctx, id, normalizeItemInput(input))
}

func (s *Service) DeleteItem(ctx context.Context, id int64) error {
	return s.store.DeleteItem(ctx, id)
}

func (s *Service) ActiveRequests(ctx context.Context) ([]ActiveRequest, error) {
	return s.store.ListActiveRequests(ctx)
}

func (s *Service) CreateActiveRequest(ctx context.Context, input ActiveRequestInput) (ActiveRequest, error) {
	return s.store.CreateActiveRequest(ctx, normalizeActiveRequestInput(input))
}

func (s *Service) UpdateActiveRequest(ctx context.Context, id int64, input ActiveRequestInput) (ActiveRequest, error) {
	return s.store.UpdateActiveRequest(ctx, id, normalizeActiveRequestInput(input))
}

func (s *Service) DeleteActiveRequest(ctx context.Context, id int64) error {
	return s.store.DeleteActiveRequest(ctx, id)
}

func (s *Service) Fields(ctx context.Context, groupID, itemID int64) ([]FieldDefinition, error) {
	return s.store.ListFields(ctx, groupID, itemID)
}

func (s *Service) UpsertField(ctx context.Context, input FieldInput) (FieldDefinition, error) {
	return s.store.UpsertField(ctx, normalizeFieldInput(input))
}

func (s *Service) DeleteField(ctx context.Context, id int64) error {
	return s.store.DeleteField(ctx, id)
}

func (s *Service) Channels(ctx context.Context) ([]Channel, error) {
	return s.store.ListChannels(ctx)
}

func (s *Service) UpsertChannel(ctx context.Context, input ChannelInput) (Channel, error) {
	return s.store.UpsertChannel(ctx, normalizeChannelInput(input))
}

func (s *Service) DeleteChannel(ctx context.Context, id int64) error {
	return s.store.DeleteChannel(ctx, id)
}

func (s *Service) Rules(ctx context.Context) ([]AlertRule, error) {
	return s.store.ListRules(ctx)
}

func (s *Service) UpsertRule(ctx context.Context, input AlertRuleInput) (AlertRule, error) {
	return s.store.UpsertRule(ctx, normalizeRuleInput(input))
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	return s.store.DeleteRule(ctx, id)
}

func isEmptyJSON(j string) bool {
	j = strings.TrimSpace(j)
	return j == "" || j == "{}"
}

func (s *Service) processSinglePassive(ctx context.Context, groupCode string, name string, interval int, timestamp interface{}, data map[string]interface{}, refItemID *int64) (Item, Group, error) {
	group, err := s.store.GetGroupByCode(ctx, groupCode)
	if err != nil {
		return Item{}, Group{}, err
	}
	item, err := s.store.UpsertItem(ctx, ItemInput{
		GroupID:              group.ID,
		SourceType:           "passive",
		Name:                 name,
		IntervalSeconds:      interval,
		MissedTimesThreshold: group.MissedTimesThreshold,
		AlertEnabled:         boolPtr(true),
		Enabled:              boolPtr(true),
		RefItemID:            refItemID,
	})
	if err != nil {
		return Item{}, Group{}, err
	}

	reportedAt := parseTimestamp(timestamp)
	raw := map[string]interface{}{
		"group":     groupCode,
		"name":      name,
		"timestamp": timestamp,
		"interval":  interval,
		"data":      data,
	}
	values := s.extractValuesFor(ctx, group.ID, item.ID, data)
	sample, err := s.store.SaveSample(ctx, SaveSampleInput{
		GroupID:         group.ID,
		ItemID:          item.ID,
		SourceType:      "passive",
		Name:            name,
		ReportedAt:      reportedAt,
		IntervalSeconds: interval,
		Status:          "ok",
		Raw:             sampleRaw(values, raw),
	}, values)
	if err != nil {
		return Item{}, Group{}, err
	}
	_ = s.EvaluateSample(ctx, sample)

	// Process object_array fields
	allFields, err := s.store.ListFields(ctx, group.ID, item.ID)
	if err == nil {
		effFields := effectiveFields(allFields)
		for _, field := range effFields {
			if field.Enabled && field.ValueType == "object_array" && field.RefGroupID != nil && *field.RefGroupID > 0 && field.RefNamePath != "" {
				rawVal, ok := valueAtPath(data, field.FieldPath)
				if ok {
					var arr []interface{}
					switch v := rawVal.(type) {
					case []interface{}:
						arr = v
					case []map[string]interface{}:
						for _, itm := range v {
							arr = append(arr, itm)
						}
					default:
						if sVal, ok := rawVal.(string); ok {
							var parsed []interface{}
							if err := json.Unmarshal([]byte(sVal), &parsed); err == nil {
								arr = parsed
							}
						}
					}
					if len(arr) > 0 {
						targetGroup, err := s.store.GetGroupByID(ctx, *field.RefGroupID)
						if err == nil {
							for _, subItemRaw := range arr {
								if subItemMap, ok := subItemRaw.(map[string]interface{}); ok {
									nameVal, ok := valueAtPath(subItemMap, field.RefNamePath)
									if ok && nameVal != nil {
										subName := strings.TrimSpace(fmt.Sprint(nameVal))
										if subName != "" {
											_, _, _ = s.processSinglePassive(ctx, targetGroup.Code, subName, interval, timestamp, subItemMap, &item.ID)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return item, group, nil
}

func (s *Service) ReceivePassive(ctx context.Context, payload PassivePayload) (int, map[string]interface{}, error) {
	settings, err := s.store.ListSettings(ctx)
	if err != nil {
		return 0, nil, err
	}
	token := settingValue(settings, "upload_token")
	if token != "" && payload.Token != token {
		return 0, nil, ErrUnauthorized
	}

	hasRoot := payload.Group != "" && payload.Name != ""
	hasItems := len(payload.Items) > 0

	if !hasRoot && !hasItems {
		return 0, nil, ErrInvalidInput
	}

	if payload.Interval <= 0 {
		payload.Interval = 60
	}

	var rootInterval int
	var rootItem Item
	var rootGroup Group
	var rootErr error
	hasRootProcessed := false

	if hasRoot {
		item, group, err := s.processSinglePassive(ctx, payload.Group, payload.Name, payload.Interval, payload.Timestamp, payload.Data, nil)
		if err != nil {
			rootErr = err
		} else {
			hasRootProcessed = true
			rootItem = item
			rootGroup = group
			rootInterval = item.IntervalSeconds
		}
	}

	var lastItemInterval int
	hasItemProcessed := false
	var lastItemErr error

	for _, item := range payload.Items {
		if item.Group == "" || item.Name == "" {
			continue
		}
		itm, _, err := s.processSinglePassive(ctx, item.Group, item.Name, payload.Interval, payload.Timestamp, item.Data, nil)
		if err != nil {
			lastItemErr = err
		} else {
			hasItemProcessed = true
			lastItemInterval = itm.IntervalSeconds
		}
	}

	if !hasRootProcessed && !hasItemProcessed {
		if rootErr != nil {
			return 0, nil, rootErr
		}
		if lastItemErr != nil {
			return 0, nil, lastItemErr
		}
		return 0, nil, ErrInvalidInput
	}

	setting := map[string]interface{}{}
	if hasRootProcessed {
		var rawSettings string
		if !isEmptyJSON(rootItem.ResponseSettingsJSON) {
			rawSettings = rootItem.ResponseSettingsJSON
		} else if !isEmptyJSON(rootGroup.ResponseSettingsJSON) {
			rawSettings = rootGroup.ResponseSettingsJSON
		}
		if rawSettings != "" {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(rawSettings), &parsed); err == nil {
				setting = parsed
			}
		}
	}

	if hasRootProcessed {
		return rootInterval, setting, nil
	}
	return lastItemInterval, setting, nil
}

func (s *Service) Samples(ctx context.Context, groupID, itemID int64, limit int) ([]Sample, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return s.store.ListSamples(ctx, groupID, itemID, limit)
}

func (s *Service) Stats(ctx context.Context, groupID, itemID int64, fieldPath string, since time.Time) (StatResult, error) {
	if since.IsZero() {
		since = time.Now().Add(-24 * time.Hour)
	}
	return s.store.Stats(ctx, groupID, itemID, fieldPath, since)
}

func (s *Service) Events(ctx context.Context, limit int) ([]AlertEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.store.ListEvents(ctx, limit)
}

func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	return s.store.Dashboard(ctx)
}

func (s *Service) PollActiveRequest(ctx context.Context, req ActiveRequest) error {
	last, err := s.store.LastSample(ctx, req.ItemID)
	if err == nil && time.Since(parseStoredTime(last.ReceivedAt)) < time.Duration(req.IntervalSeconds)*time.Second {
		return nil
	}

	start := time.Now()
	raw, status, httpStatus, errMsg := s.executeActiveRequest(ctx, req)
	latency := time.Since(start).Milliseconds()
	sampleStatus := "ok"
	if status != req.ExpectedStatusCode || errMsg != "" {
		sampleStatus = "error"
		if errMsg == "" {
			errMsg = fmt.Sprintf("unexpected status code: %d", status)
		}
	}

	activeID := req.ID
	values := s.extractValuesFor(ctx, req.GroupID, req.ItemID, raw)
	sample, err := s.store.SaveSample(ctx, SaveSampleInput{
		GroupID:         req.GroupID,
		ItemID:          req.ItemID,
		SourceType:      "active",
		ActiveRequestID: &activeID,
		Name:            req.Name,
		IntervalSeconds: req.IntervalSeconds,
		Status:          sampleStatus,
		HTTPStatusCode:  httpStatus,
		LatencyMS:       latency,
		Raw:             sampleRaw(values, raw),
		ErrorMessage:    errMsg,
	}, values)
	if err != nil {
		return err
	}

	return s.EvaluateSample(ctx, sample)
}

func (s *Service) CheckMissing(ctx context.Context) error {
	groups, err := s.store.ListGroups(ctx)
	if err != nil {
		return err
	}
	groupMap := make(map[int64]Group, len(groups))
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	for _, group := range groups {
		items, err := s.store.ListItems(ctx, group.ID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if !item.Enabled || !item.AlertEnabled {
				continue
			}
			last, err := s.store.LastSample(ctx, item.ID)
			lastAt := parseStoredTime(item.CreatedAt)
			lastStatus := ""
			if err == nil {
				lastAt = parseStoredTime(last.ReceivedAt)
				lastStatus = last.Status
			}
			threshold := item.MissedTimesThreshold
			if threshold <= 0 {
				threshold = groupMap[item.GroupID].MissedTimesThreshold
			}
			if threshold <= 0 {
				threshold = 3
			}
			interval := item.IntervalSeconds
			if interval <= 0 {
				interval = groupMap[item.GroupID].DefaultIntervalSeconds
			}
			if time.Since(lastAt) < time.Duration(interval*threshold)*time.Second {
				continue
			}
			if lastStatus == "missing" && time.Since(lastAt) < time.Duration(interval)*time.Second {
				continue
			}
			sample, err := s.store.SaveSample(ctx, SaveSampleInput{
				GroupID:         item.GroupID,
				ItemID:          item.ID,
				SourceType:      item.SourceType,
				Name:            item.Name,
				IntervalSeconds: interval,
				Status:          "missing",
				Raw: map[string]interface{}{
					"message": "data missing",
				},
				ErrorMessage: "data missing",
			}, nil)
			if err != nil {
				return err
			}
			if err := s.EvaluateSample(ctx, sample); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Cleanup(ctx context.Context) error {
	settings, err := s.store.ListSettings(ctx)
	if err != nil {
		return err
	}
	days, _ := strconv.Atoi(settingValue(settings, "data_retention_days"))
	if days <= 0 {
		days = 30
	}
	return s.store.Cleanup(ctx, time.Now().AddDate(0, 0, -days))
}

func (s *Service) EvaluateSample(ctx context.Context, sample Sample) error {
	// If the item has alerts disabled, do not evaluate or notify any alert rules
	item, err := s.store.GetItemByID(ctx, sample.ItemID)
	if err == nil && !item.AlertEnabled {
		return nil
	}

	rules, err := s.store.AlertRulesForSample(ctx, sample)
	if err != nil {
		return err
	}
	valueMap := make(map[string]SampleValue, len(sample.Values))
	for _, value := range sample.Values {
		valueMap[value.FieldPath] = value
	}
	itemOverride := itemOverridesGroup(fieldsForSample(ctx, s.store, sample))

	for _, rule := range rules {
		if itemOverride && rule.ScopeType == "group" && ruleUsesField(rule) {
			continue
		}
		matched, currentValue := s.ruleMatched(ctx, rule, sample, valueMap)
		event, err := s.store.ApplyAlertEvaluation(ctx, rule, sample, matched, currentValue, rule.ThresholdValue)
		if err != nil {
			return err
		}
		if event != nil {
			if err := s.Notify(ctx, *event); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Notify(ctx context.Context, event AlertEvent) error {
	channels, err := s.store.ListEnabledChannelsForRule(ctx, event.RuleID)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		channels, _ = s.store.ListChannels(ctx)
	}

	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		reqJSON, respText, err := s.sendChannel(ctx, channel, event)
		status := "sent"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}
		if reqJSON == "" && status == "sent" {
			status = "skipped"
			respText = "channel type is not supported by sender"
		}
		if err := s.store.CreateNotification(ctx, event.ID, channel.ID, status, reqJSON, respText, errMsg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ruleMatched(ctx context.Context, rule AlertRule, sample Sample, values map[string]SampleValue) (bool, string) {
	switch rule.RuleType {
	case "missing_data":
		return sample.Status == "missing", sample.Status
	case "request_failed":
		return sample.Status == "error" && sample.SourceType == "active", sample.ErrorMessage
	case "field_condition":
		value, ok := values[rule.FieldPath]
		if !ok {
			return rule.Operator == "not_exists", ""
		}
		return compareValue(value, rule.Operator, rule.ThresholdValue), sampleValueString(value)
	case "aggregate_condition":
		window := 10 * time.Minute
		if rule.AggregateWindowSeconds != nil && *rule.AggregateWindowSeconds > 0 {
			window = time.Duration(*rule.AggregateWindowSeconds) * time.Second
		}
		vals, err := s.store.WindowValues(ctx, sample.ItemID, rule.FieldPath, time.Now().Add(-window), valueOrZero(rule.AggregateSampleCount))
		if err != nil || len(vals) == 0 {
			return false, ""
		}
		current := aggregate(vals, rule.AggregateFunc)
		return compareFloat(current, rule.Operator, rule.ThresholdValue), strconv.FormatFloat(current, 'f', -1, 64)
	default:
		return false, ""
	}
}

func (s *Service) executeActiveRequest(ctx context.Context, active ActiveRequest) (map[string]interface{}, int, int, string) {
	method := strings.ToUpper(active.Method)
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if method == http.MethodPost && active.BodyJSON != "" && active.BodyJSON != "{}" {
		switch active.BodyType {
		case "form-data":
			values := url.Values{}
			var data map[string]interface{}
			_ = json.Unmarshal([]byte(active.BodyJSON), &data)
			for key, value := range data {
				values.Set(key, fmt.Sprint(value))
			}
			body = strings.NewReader(values.Encode())
		default:
			body = bytes.NewBufferString(active.BodyJSON)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, active.URL, body)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, 0, 0, err.Error()
	}
	if active.BodyType == "form-data" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	var headers map[string]string
	if err := json.Unmarshal([]byte(active.HeadersJSON), &headers); err == nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	client := *s.httpClient
	if active.TimeoutSeconds > 0 {
		client.Timeout = time.Duration(active.TimeoutSeconds) * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, 0, 0, err.Error()
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, resp.StatusCode, resp.StatusCode, err.Error()
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		raw = map[string]interface{}{"body": string(bodyBytes)}
		return raw, resp.StatusCode, resp.StatusCode, "response is not json"
	}
	return raw, resp.StatusCode, resp.StatusCode, ""
}

func (s *Service) extractValues(data map[string]interface{}) []SampleValue {
	flat := map[string]interface{}{}
	flattenJSON("", data, flat)

	values := make([]SampleValue, 0, len(flat))
	for path, raw := range flat {
		value := sampleValue(path, raw)
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].FieldPath < values[j].FieldPath
	})
	return values
}

func (s *Service) extractValuesFor(ctx context.Context, groupID, itemID int64, data map[string]interface{}) []SampleValue {
	fields, err := s.store.ListFields(ctx, groupID, itemID)
	if err != nil || len(fields) == 0 {
		return nil
	}

	fields = effectiveFields(fields)
	if len(fields) == 0 {
		return nil
	}

	values := make([]SampleValue, 0, len(fields))
	for _, field := range fields {
		if !field.Enabled {
			continue
		}
		raw, ok := valueAtPath(data, field.FieldPath)
		if !ok {
			if field.Required {
				values = append(values, SampleValue{FieldPath: field.FieldPath, ValueType: field.ValueType, RawValue: nil})
			}
			continue
		}
		values = append(values, coerceSampleValue(field.FieldPath, field.ValueType, raw))
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].FieldPath < values[j].FieldPath
	})
	return values
}

func fieldsForSample(ctx context.Context, store Store, sample Sample) []FieldDefinition {
	fields, err := store.ListFields(ctx, sample.GroupID, sample.ItemID)
	if err != nil {
		return nil
	}
	return fields
}

func itemOverridesGroup(fields []FieldDefinition) bool {
	for _, field := range fields {
		if field.ScopeType == "item" {
			return true
		}
	}
	return false
}

func ruleUsesField(rule AlertRule) bool {
	return rule.RuleType == "field_condition" || rule.RuleType == "aggregate_condition"
}

func effectiveFields(fields []FieldDefinition) []FieldDefinition {
	var itemFields []FieldDefinition
	var groupFields []FieldDefinition
	for _, field := range fields {
		if field.ScopeType == "item" {
			itemFields = append(itemFields, field)
			continue
		}
		groupFields = append(groupFields, field)
	}
	if len(itemFields) > 0 {
		return itemFields
	}
	return groupFields
}

func sampleRaw(values []SampleValue, fallback map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		if errValue, ok := fallback["error"]; ok {
			return map[string]interface{}{"error": errValue}
		}
		return map[string]interface{}{}
	}
	fields := make(map[string]interface{}, len(values))
	for _, value := range values {
		fields[value.FieldPath] = value.RawValue
	}
	return map[string]interface{}{"fields": fields}
}

func (s *Service) sendChannel(ctx context.Context, channel Channel, event AlertEvent) (string, string, error) {
	var cfg map[string]interface{}
	_ = json.Unmarshal([]byte(channel.ConfigJSON), &cfg)
	locale := "zh-CN"
	if settings, err := s.store.ListSettings(ctx); err == nil {
		if value := settingValue(settings, "default_locale"); value != "" {
			locale = value
		}
	}
	severityStr := localizedSeverity(locale, event.Severity)
	if event.EventType == "recovered" {
		if locale == "zh-CN" {
			severityStr = "恢复"
		} else {
			severityStr = "Recovered"
		}
	}
	text := fmt.Sprintf("[%s] %s\n%s", severityStr, event.Title, event.Message)

	switch strings.ToLower(channel.Type) {
	case "dingding", "dingtalk":
		webhook := fmt.Sprint(cfg["webhook"])
		if webhook == "" || webhook == "<nil>" {
			return "", "", errors.New("missing dingding webhook")
		}
		secret := fmt.Sprint(cfg["secret"])
		if secret != "" && secret != "<nil>" {
			timestamp := time.Now().UnixNano() / int64(time.Millisecond)
			stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(stringToSign))
			sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

			u, err := url.Parse(webhook)
			if err != nil {
				return "", "", fmt.Errorf("parse dingding webhook URL: %w", err)
			}
			q := u.Query()
			q.Set("timestamp", fmt.Sprintf("%d", timestamp))
			q.Set("sign", sign)
			u.RawQuery = q.Encode()
			webhook = u.String()
		}
		body := map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": text,
			},
		}
		return s.postJSON(ctx, webhook, body)
	case "telegram":
		token := fmt.Sprint(cfg["bot_token"])
		chatID := fmt.Sprint(cfg["chat_id"])
		if token == "" || token == "<nil>" || chatID == "" || chatID == "<nil>" {
			return "", "", errors.New("missing telegram bot_token or chat_id")
		}
		body := map[string]interface{}{
			"chat_id": chatID,
			"text":    text,
		}
		return s.postJSON(ctx, "https://api.telegram.org/bot"+token+"/sendMessage", body)
	default:
		return "", "", nil
	}
}

func (s *Service) postJSON(ctx context.Context, target string, body map[string]interface{}) (string, string, error) {
	reqBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(reqBytes))
	if err != nil {
		return string(reqBytes), "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return string(reqBytes), "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(reqBytes), string(respBytes), fmt.Errorf("notification status: %d", resp.StatusCode)
	}
	return string(reqBytes), string(respBytes), nil
}

func compareValue(value SampleValue, operator, threshold string) bool {
	switch value.ValueType {
	case "string_array":
		var arr []string
		_ = json.Unmarshal([]byte(value.StringValue), &arr)
		arrLen := len(arr)
		switch operator {
		case "len_eq":
			tVal, err := strconv.Atoi(threshold)
			return err == nil && arrLen == tVal
		case "len_gt":
			tVal, err := strconv.Atoi(threshold)
			return err == nil && arrLen > tVal
		case "len_lt":
			tVal, err := strconv.Atoi(threshold)
			return err == nil && arrLen < tVal
		case "len_ne":
			tVal, err := strconv.Atoi(threshold)
			return err == nil && arrLen != tVal
		case "contains":
			return arrayContainsAny(arr, threshold)
		case "not_contains":
			return !arrayContainsAny(arr, threshold)
		case "exists":
			return true
		case "not_exists":
			return false
		default:
			return false
		}
	case "integer", "float":
		current := 0.0
		if value.NumericValue != nil {
			current = *value.NumericValue
		}
		return compareFloat(current, operator, threshold)
	case "boolean":
		want := threshold == "true" || threshold == "1"
		got := value.BooleanValue != nil && *value.BooleanValue
		switch operator {
		case "eq":
			return got == want
		case "ne":
			return got != want
		default:
			return false
		}
	default:
		current := sampleValueString(value)
		switch operator {
		case "eq":
			return current == threshold
		case "ne":
			return current != threshold
		case "contains":
			return containsAny(current, threshold)
		case "not_contains":
			return !containsAny(current, threshold)
		case "exists":
			return true
		case "not_exists":
			return false
		default:
			return false
		}
	}
}

func arrayContainsAny(arr []string, threshold string) bool {
	for _, part := range strings.Split(threshold, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, item := range arr {
			if strings.Contains(item, part) {
				return true
			}
		}
	}
	return false
}

func containsAny(current, threshold string) bool {
	for _, part := range strings.Split(threshold, ",") {
		part = strings.TrimSpace(part)
		if part != "" && strings.Contains(current, part) {
			return true
		}
	}
	return false
}

func localizedSeverity(locale, severity string) string {
	if locale == "zh-CN" {
		switch severity {
		case "info":
			return "信息"
		case "warning":
			return "警告"
		case "critical":
			return "严重"
		}
	}
	switch severity {
	case "info":
		return "Info"
	case "warning":
		return "Warning"
	case "critical":
		return "Critical"
	default:
		return severity
	}
}

func compareFloat(current float64, operator, threshold string) bool {
	want, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		return false
	}
	switch operator {
	case "gt":
		return current > want
	case "gte":
		return current >= want
	case "lt":
		return current < want
	case "lte":
		return current <= want
	case "eq":
		return current == want
	case "ne":
		return current != want
	default:
		return false
	}
}

func aggregate(vals []float64, fn string) float64 {
	if len(vals) == 0 {
		return 0
	}
	switch fn {
	case "max":
		v := vals[0]
		for _, x := range vals[1:] {
			v = math.Max(v, x)
		}
		return v
	case "min":
		v := vals[0]
		for _, x := range vals[1:] {
			v = math.Min(v, x)
		}
		return v
	case "median":
		cp := append([]float64(nil), vals...)
		sort.Float64s(cp)
		mid := len(cp) / 2
		if len(cp)%2 == 0 {
			return (cp[mid-1] + cp[mid]) / 2
		}
		return cp[mid]
	case "count":
		return float64(len(vals))
	default:
		sum := 0.0
		for _, x := range vals {
			sum += x
		}
		return sum / float64(len(vals))
	}
}

func sampleValue(path string, raw interface{}) SampleValue {
	value := SampleValue{FieldPath: path, RawValue: raw}
	switch v := raw.(type) {
	case bool:
		n := 0.0
		if v {
			n = 1
		}
		return SampleValue{FieldPath: path, ValueType: "boolean", BooleanValue: &v, NumericValue: &n, RawValue: raw}
	case float64:
		return SampleValue{FieldPath: path, ValueType: "float", FloatValue: &v, NumericValue: &v, RawValue: raw}
	case int:
		iv := int64(v)
		fv := float64(v)
		return SampleValue{FieldPath: path, ValueType: "integer", IntegerValue: &iv, NumericValue: &fv, RawValue: raw}
	case int64:
		fv := float64(v)
		return SampleValue{FieldPath: path, ValueType: "integer", IntegerValue: &v, NumericValue: &fv, RawValue: raw}
	case string:
		value.ValueType = "string"
		value.StringValue = v
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			value.NumericValue = &parsed
		}
		return value
	default:
		value.ValueType = "string"
		value.StringValue = fmt.Sprint(v)
		return value
	}
}

func coerceSampleValue(path, valueType string, raw interface{}) SampleValue {
	value := SampleValue{FieldPath: path, ValueType: valueType, RawValue: raw}
	switch valueType {
	case "integer":
		if n, ok := toFloat(raw); ok {
			iv := int64(n)
			fv := float64(iv)
			value.IntegerValue = &iv
			value.NumericValue = &fv
			return value
		}
	case "float":
		if n, ok := toFloat(raw); ok {
			value.FloatValue = &n
			value.NumericValue = &n
			return value
		}
	case "boolean":
		b := false
		switch v := raw.(type) {
		case bool:
			b = v
		case string:
			b = v == "true" || v == "1" || strings.EqualFold(v, "yes")
		case float64:
			b = v != 0
		case int:
			b = v != 0
		case int64:
			b = v != 0
		}
		n := 0.0
		if b {
			n = 1
		}
		value.BooleanValue = &b
		value.NumericValue = &n
		return value
	case "string_array":
		var strArr []string
		switch arr := raw.(type) {
		case []interface{}:
			for _, item := range arr {
				if item != nil {
					strArr = append(strArr, fmt.Sprint(item))
				}
			}
		case []string:
			strArr = arr
		default:
			if raw != nil {
				if s, ok := raw.(string); ok {
					var parsed []string
					if err := json.Unmarshal([]byte(s), &parsed); err == nil {
						strArr = parsed
					} else {
						strArr = []string{s}
					}
				} else {
					strArr = []string{fmt.Sprint(raw)}
				}
			}
		}
		if strArr == nil {
			strArr = []string{}
		}
		marshaled, _ := json.Marshal(strArr)
		value.StringValue = string(marshaled)
		return value
	case "object_array":
		marshaled, _ := json.Marshal(raw)
		value.StringValue = string(marshaled)
		return value
	}
	value.ValueType = "string"
	value.StringValue = fmt.Sprint(raw)
	return value
}

func toFloat(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func valueAtPath(data map[string]interface{}, path string) (interface{}, bool) {
	path = normalizeJSONFieldPath(path)
	if path == "" {
		return nil, false
	}
	current := interface{}(data)
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func normalizeJSONFieldPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, ".")
	path = strings.TrimPrefix(path, "data.")
	return path
}

func sampleValueString(value SampleValue) string {
	switch value.ValueType {
	case "integer":
		if value.IntegerValue != nil {
			return strconv.FormatInt(*value.IntegerValue, 10)
		}
	case "float":
		if value.FloatValue != nil {
			return strconv.FormatFloat(*value.FloatValue, 'f', -1, 64)
		}
	case "boolean":
		if value.BooleanValue != nil {
			return strconv.FormatBool(*value.BooleanValue)
		}
	}
	return value.StringValue
}

func flattenJSON(prefix string, input map[string]interface{}, out map[string]interface{}) {
	for key, value := range input {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := value.(map[string]interface{}); ok {
			flattenJSON(path, nested, out)
			continue
		}
		out[path] = value
	}
}

func parseTimestamp(value interface{}) *time.Time {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			return &ts
		}
		if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
			ts := time.Unix(unix, 0)
			return &ts
		}
	case float64:
		ts := time.Unix(int64(v), 0)
		return &ts
	case int64:
		ts := time.Unix(v, 0)
		return &ts
	}
	return nil
}

func parseStoredTime(value string) time.Time {
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return ts
	}
	return time.Now()
}

func settingValue(settings []Setting, key string) string {
	for _, setting := range settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func normalizeUserInput(input UserInput) UserInput {
	input.Username = strings.TrimSpace(input.Username)
	input.Role = strings.TrimSpace(input.Role)
	if input.Role == "" {
		input.Role = "viewer"
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func normalizeGroupInput(input GroupInput) GroupInput {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Icon = strings.TrimSpace(input.Icon)
	if input.Icon == "" {
		input.Icon = "Monitor"
	}
	if input.DefaultIntervalSeconds <= 0 {
		input.DefaultIntervalSeconds = 60
	}
	if input.MissedTimesThreshold <= 0 {
		input.MissedTimesThreshold = 3
	}
	if input.AlertEnabled == nil {
		input.AlertEnabled = boolPtr(true)
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func normalizeItemInput(input ItemInput) ItemInput {
	input.Name = strings.TrimSpace(input.Name)
	if input.SourceType == "" {
		input.SourceType = "passive"
	}
	if input.IntervalSeconds <= 0 {
		input.IntervalSeconds = 60
	}
	if input.MissedTimesThreshold <= 0 {
		input.MissedTimesThreshold = 3
	}
	if input.AlertEnabled == nil {
		input.AlertEnabled = boolPtr(true)
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func normalizeActiveRequestInput(input ActiveRequestInput) ActiveRequestInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	if input.Method == "" {
		input.Method = "GET"
	}
	if input.BodyType == "" {
		input.BodyType = "none"
	}
	if input.HeadersJSON == "" {
		input.HeadersJSON = "{}"
	}
	if input.BodyJSON == "" {
		input.BodyJSON = "{}"
	}
	if input.IntervalSeconds <= 0 {
		input.IntervalSeconds = 60
	}
	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 10
	}
	if input.ExpectedStatusCode <= 0 {
		input.ExpectedStatusCode = 200
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func normalizeFieldInput(input FieldInput) FieldInput {
	input.ScopeType = strings.TrimSpace(input.ScopeType)
	if input.ScopeType == "" {
		input.ScopeType = "group"
	}
	input.FieldPath = strings.TrimSpace(input.FieldPath)
	if input.ValueType == "" {
		input.ValueType = "string"
	}
	if input.Required == nil {
		input.Required = boolPtr(false)
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func normalizeChannelInput(input ChannelInput) ChannelInput {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.TrimSpace(input.Type)
	if input.ConfigJSON == "" {
		input.ConfigJSON = "{}"
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	if input.IsDefault == nil {
		input.IsDefault = boolPtr(false)
	}
	return input
}

func normalizeRuleInput(input AlertRuleInput) AlertRuleInput {
	input.Name = strings.TrimSpace(input.Name)
	if input.ScopeType == "" {
		input.ScopeType = "global"
	}
	if input.SourceType == "" {
		input.SourceType = "any"
	}
	if input.ConsecutiveCount <= 0 {
		input.ConsecutiveCount = 1
	}
	if input.RecoveryCount <= 0 {
		input.RecoveryCount = 1
	}
	if input.Severity == "" {
		input.Severity = "warning"
	}
	if input.Enabled == nil {
		input.Enabled = boolPtr(true)
	}
	return input
}

func boolPtr(value bool) *bool {
	return &value
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
