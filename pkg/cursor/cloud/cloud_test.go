package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{APIKey: "test-key", BaseURL: srv.URL, HTTPClient: srv.Client()}
}

func TestCreateAgentSendsBearerAndBody(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"agent":{"id":"bc-1","status":"ACTIVE","url":"https://cursor.com/agents/bc-1"},"run":{"id":"run-1","agentId":"bc-1","status":"CREATING"}}`))
	})

	out, err := c.CreateAgent(context.Background(), CreateAgentRequest{
		Prompt: Prompt{Text: "Add a README"},
		Model:  &ModelRef{ID: "composer-2"},
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPath != "/v1/agents" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"text":"Add a README"`) {
		t.Fatalf("body missing prompt: %s", gotBody)
	}
	if out.Agent.ID != "bc-1" || out.Run.ID != "run-1" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestCreateAgentRejectsEmptyPrompt(t *testing.T) {
	c := New("k")
	if _, err := c.CreateAgent(context.Background(), CreateAgentRequest{}); err == nil {
		t.Fatal("expected validation error for empty prompt")
	}
}

func TestMissingAPIKeyIsRejectedBeforeRequest(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:0"}
	if _, err := c.Agent(context.Background(), "bc-1"); err == nil {
		t.Fatal("expected error for empty APIKey")
	}
}

func TestAPIErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		retryAfter    string
		wantRetryable bool
		wantNotFound  bool
		wantUnauth    bool
		wantCode      string
	}{
		{name: "not found", status: 404, body: `{"error":{"code":"agent_not_found","message":"no such agent"}}`, wantNotFound: true, wantCode: "agent_not_found"},
		{name: "unauthorized", status: 401, body: `{"error":{"code":"unauthorized","message":"bad key"}}`, wantUnauth: true, wantCode: "unauthorized"},
		{name: "rate limited", status: 429, body: `{"code":"rate_limited","message":"slow down"}`, retryAfter: "30", wantRetryable: true, wantCode: "rate_limited"},
		{name: "server error", status: 500, body: `not json at all`, wantRetryable: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := c.Agent(context.Background(), "bc-1")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if apiErr.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", apiErr.Code, tc.wantCode)
			}
			if apiErr.IsRetryable() != tc.wantRetryable {
				t.Fatalf("IsRetryable = %v, want %v", apiErr.IsRetryable(), tc.wantRetryable)
			}
			if apiErr.IsNotFound() != tc.wantNotFound {
				t.Fatalf("IsNotFound = %v", apiErr.IsNotFound())
			}
			if apiErr.IsUnauthorized() != tc.wantUnauth {
				t.Fatalf("IsUnauthorized = %v", apiErr.IsUnauthorized())
			}
			if tc.retryAfter != "" && apiErr.RetryAfter.Seconds() != 30 {
				t.Fatalf("RetryAfter = %v, want 30s", apiErr.RetryAfter)
			}
		})
	}
}

func TestCancelRunPostsToCancelPath(t *testing.T) {
	var gotMethod, gotPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.CancelRun(context.Background(), "bc-1", "run-1"); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/agents/bc-1/runs/run-1/cancel" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
}

func TestAgentUsageDecodes(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"agentId":"bc-1","inputTokens":1200,"outputTokens":340,"cacheReadTokens":90,"totalCents":7}`))
	})
	usage, err := c.AgentUsage(context.Background(), "bc-1")
	if err != nil {
		t.Fatalf("AgentUsage: %v", err)
	}
	if usage.InputTokens != 1200 || usage.OutputTokens != 340 || usage.TotalCents != 7 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestListRunsSendsPaging(t *testing.T) {
	var gotQuery string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"runs":[{"id":"run-1","agentId":"bc-1","status":"FINISHED"}],"nextCursor":"c2"}`))
	})
	out, err := c.ListRuns(context.Background(), "bc-1", 10, "c1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if !strings.Contains(gotQuery, "limit=10") || !strings.Contains(gotQuery, "cursor=c1") {
		t.Fatalf("query = %q", gotQuery)
	}
	if len(out.Runs) != 1 || out.NextCursor != "c2" {
		t.Fatalf("out = %+v", out)
	}
}

const sseBody = "" +
	"event: status\n" +
	"data: {\"runId\":\"run-1\",\"status\":\"RUNNING\"}\n" +
	"\n" +
	": this is a comment and must be ignored\n" +
	"id: evt-1\n" +
	"event: assistant\n" +
	"data: {\"text\":\"Hello \"}\n" +
	"\n" +
	"id: evt-2\n" +
	"event: assistant\n" +
	"data: {\"text\":\"world\"}\n" +
	"\n" +
	"id: evt-3\n" +
	"event: result\n" +
	"data: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"Hello world\",\"durationMs\":1234}\n" +
	"\n" +
	"event: done\n" +
	"data: {}\n" +
	"\n"

func TestStreamRunParsesEvents(t *testing.T) {
	var gotAccept, gotLastEventID string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotLastEventID = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Cursor-Stream-Retention-Seconds", "600")
		_, _ = w.Write([]byte(sseBody))
	})

	stream, err := c.StreamRun(context.Background(), "bc-1", "run-1", &StreamOptions{LastEventID: "evt-0"})
	if err != nil {
		t.Fatalf("StreamRun: %v", err)
	}
	defer func() { _ = stream.Close() }()

	if gotAccept != "text/event-stream" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotLastEventID != "evt-0" {
		t.Fatalf("Last-Event-ID = %q", gotLastEventID)
	}
	if stream.RetentionSeconds != 600 {
		t.Fatalf("RetentionSeconds = %d", stream.RetentionSeconds)
	}

	var types []string
	var text strings.Builder
	var result ResultData
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		types = append(types, event.Type)
		switch event.Type {
		case EventAssistant:
			var d TextData
			if err := json.Unmarshal(event.Data, &d); err != nil {
				t.Fatalf("assistant decode: %v", err)
			}
			text.WriteString(d.Text)
		case EventResult:
			if err := json.Unmarshal(event.Data, &result); err != nil {
				t.Fatalf("result decode: %v", err)
			}
		}
	}

	want := []string{EventStatus, EventAssistant, EventAssistant, EventResult, EventDone}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("types = %v, want %v", types, want)
	}
	if text.String() != "Hello world" {
		t.Fatalf("assembled text = %q", text.String())
	}
	if result.Status != RunFinished || result.DurationMs != 1234 {
		t.Fatalf("result = %+v", result)
	}
	// status and done carry no id, so the resume point stays the last real id.
	if stream.LastEventID() != "evt-3" {
		t.Fatalf("LastEventID = %q, want evt-3", stream.LastEventID())
	}
}

func TestStreamRunSurfacesHTTPError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":{"code":"stream_expired","message":"retention window passed"}}`))
	})
	_, err := c.StreamRun(context.Background(), "bc-1", "run-1", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stream_expired" {
		t.Fatalf("err = %v, want stream_expired APIError", err)
	}
}

func TestIsTerminal(t *testing.T) {
	for _, s := range []string{RunFinished, RunError, RunCancelled, RunExpired} {
		if !IsTerminal(s) {
			t.Fatalf("%s should be terminal", s)
		}
	}
	for _, s := range []string{RunCreating, RunRunning} {
		if IsTerminal(s) {
			t.Fatalf("%s should not be terminal", s)
		}
	}
}
