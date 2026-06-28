package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type mockStore struct {
	Store
	settings []Setting
}

func (m *mockStore) ListSettings(ctx context.Context) ([]Setting, error) {
	return m.settings, nil
}

func TestSendChannel_FeishuNoSecret(t *testing.T) {
	// Create a mock server to receive the webhook
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(bodyBytes, &receivedBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	// Initialize Service
	store := &mockStore{
		settings: []Setting{
			{Key: "default_locale", Value: "zh-CN"},
		},
	}
	s := NewService(store)

	channel := Channel{
		ID:         1,
		Code:       "feishu_test",
		Name:       "Feishu Test",
		Type:       "feishu",
		ConfigJSON: fmt.Sprintf(`{"webhook": "%s"}`, server.URL),
		Enabled:    true,
	}

	event := AlertEvent{
		ID:         100,
		RuleID:     200,
		EventType:  "triggered",
		Severity:   "critical",
		Title:      "CPU High",
		Message:    "CPU is at 95%",
		OccurredAt: "2026-06-28 17:34:00",
	}

	reqJSON, respText, err := s.sendChannel(context.Background(), channel, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if respText != `{"code":0,"msg":"success"}` {
		t.Errorf("expected success response body, got %s", respText)
	}

	// Verify request JSON
	var sentBody map[string]interface{}
	if err := json.Unmarshal([]byte(reqJSON), &sentBody); err != nil {
		t.Fatalf("invalid reqJSON format: %v", err)
	}

	if sentBody["msg_type"] != "text" {
		t.Errorf("expected msg_type to be text, got %v", sentBody["msg_type"])
	}

	content, ok := sentBody["content"].(map[string]interface{})
	if !ok {
		t.Fatal("expected content to be a map")
	}

	expectedText := "[严重] CPU High\nCPU is at 95%"
	if content["text"] != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, content["text"])
	}

	if _, signed := sentBody["sign"]; signed {
		t.Error("expected no sign field when secret is not configured")
	}
}

func TestSendChannel_FeishuWithSecret(t *testing.T) {
	secretKey := "my_awesome_secret_key"
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(bodyBytes, &receivedBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	store := &mockStore{
		settings: []Setting{
			{Key: "default_locale", Value: "en-US"},
		},
	}
	s := NewService(store)

	channel := Channel{
		ID:         2,
		Code:       "feishu_test_secret",
		Name:       "Feishu Test Secret",
		Type:       "feishu",
		ConfigJSON: fmt.Sprintf(`{"webhook": "%s", "secret": "%s"}`, server.URL, secretKey),
		Enabled:    true,
	}

	event := AlertEvent{
		ID:         101,
		RuleID:     201,
		EventType:  "recovered",
		Severity:   "critical",
		Title:      "CPU High",
		Message:    "CPU is back to normal",
		OccurredAt: "2026-06-28 17:35:00",
	}

	_, _, err := s.sendChannel(context.Background(), channel, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify timestamp & signature in request body
	timestampStr, ok := receivedBody["timestamp"].(string)
	if !ok {
		t.Fatal("missing timestamp in request body")
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		t.Fatalf("invalid timestamp: %v", err)
	}

	// Check time is close to now
	timeDiff := time.Now().Unix() - timestamp
	if timeDiff < -5 || timeDiff > 5 {
		t.Errorf("timestamp is too far from now: %v", timeDiff)
	}

	signStr, ok := receivedBody["sign"].(string)
	if !ok {
		t.Fatal("missing sign in request body")
	}

	// Recalculate signature to verify
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secretKey)
	h := hmac.New(sha256.New, []byte(stringToSign))
	expectedSign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if signStr != expectedSign {
		t.Errorf("signature mismatch: expected %s, got %s", expectedSign, signStr)
	}

	// Verify msg content
	if receivedBody["msg_type"] != "text" {
		t.Errorf("expected msg_type text, got %v", receivedBody["msg_type"])
	}

	content := receivedBody["content"].(map[string]interface{})
	expectedText := "[Recovered] CPU High\nCPU is back to normal"
	if content["text"] != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, content["text"])
	}
}

func TestSendCombinedChannelText_Feishu(t *testing.T) {
	secretKey := "my_awesome_secret_key"
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(bodyBytes, &receivedBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewService(nil)
	channel := Channel{
		Type:       "feishu",
		ConfigJSON: fmt.Sprintf(`{"webhook": "%s", "secret": "%s"}`, server.URL, secretKey),
	}

	_, _, err := s.sendCombinedChannelText(context.Background(), channel, "Combined Alert Title", "This is line 1\nThis is line 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := receivedBody["content"].(map[string]interface{})
	expectedText := "Combined Alert Title\nThis is line 1\nThis is line 2"
	if content["text"] != expectedText {
		t.Errorf("expected combined text %q, got %q", expectedText, content["text"])
	}

	// Verify signature exists
	if _, signed := receivedBody["sign"]; !signed {
		t.Error("expected sign field in combined channel send")
	}
}
