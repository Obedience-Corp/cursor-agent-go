package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAgent speaks the agent half of the protocol over in-memory pipes.
type fakeAgent struct {
	toClient *io.PipeWriter
	enc      *json.Encoder
	requests chan rpcMessage
}

func newFakeAgent(t *testing.T, handler Handler) (*conn, *fakeAgent) {
	t.Helper()
	agentReader, clientWriter := io.Pipe()
	clientReader, agentWriter := io.Pipe()

	fa := &fakeAgent{
		toClient: agentWriter,
		enc:      json.NewEncoder(agentWriter),
		requests: make(chan rpcMessage, 16),
	}
	go func() {
		scanner := bufio.NewScanner(agentReader)
		for scanner.Scan() {
			var msg rpcMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			fa.requests <- msg
		}
		close(fa.requests)
	}()

	c := newConn(clientWriter, clientReader, handler)
	t.Cleanup(func() {
		_ = clientWriter.Close()
		_ = agentWriter.Close()
	})
	return c, fa
}

func (f *fakeAgent) next(t *testing.T) rpcMessage {
	t.Helper()
	select {
	case msg, ok := <-f.requests:
		if !ok {
			t.Fatal("agent stream closed while waiting for a request")
		}
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a client request")
		return rpcMessage{}
	}
}

func (f *fakeAgent) respond(t *testing.T, id *json.RawMessage, result any) {
	t.Helper()
	if err := f.enc.Encode(rpcMessage{JSONRPC: "2.0", ID: id, Result: mustRaw(result)}); err != nil {
		t.Fatalf("respond: %v", err)
	}
}

func (f *fakeAgent) notifyUpdate(t *testing.T, sessionID string, update any) {
	t.Helper()
	params := map[string]any{"sessionId": sessionID, "update": update}
	if err := f.enc.Encode(rpcMessage{JSONRPC: "2.0", Method: "session/update", Params: mustRaw(params)}); err != nil {
		t.Fatalf("notifyUpdate: %v", err)
	}
}

func TestCallResolvesResultByID(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)
	var out InitializeResult
	done := make(chan error, 1)
	go func() {
		done <- c.call(t.Context(), "initialize", map[string]any{"protocolVersion": 1}, &out)
	}()

	req := agent.next(t)
	if req.Method != "initialize" {
		t.Fatalf("method = %q, want initialize", req.Method)
	}
	agent.respond(t, req.ID, InitializeResult{ProtocolVersion: 1})

	if err := <-done; err != nil {
		t.Fatalf("call: %v", err)
	}
	if out.ProtocolVersion != 1 {
		t.Fatalf("protocolVersion = %d, want 1", out.ProtocolVersion)
	}
}

func TestCallPropagatesRPCError(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)
	done := make(chan error, 1)
	go func() { done <- c.call(t.Context(), "session/new", map[string]any{}, nil) }()

	req := agent.next(t)
	if err := agent.enc.Encode(rpcMessage{
		JSONRPC: "2.0", ID: req.ID,
		Error: &rpcError{Code: -32000, Message: "no workspace trust"},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "no workspace trust") {
		t.Fatalf("err = %v, want rpc error carrying the message", err)
	}
}

func TestCallHonorsContextCancellation(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.call(ctx, "session/prompt", map[string]any{}, nil) }()

	agent.next(t) // agent deliberately never answers
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("call did not observe context cancellation")
	}
}

func TestUpdatesReachHandlerAndPreserveUnknownVariants(t *testing.T) {
	var mu sync.Mutex
	var got []Update
	handler := HandlerFuncs{
		Update: func(_ context.Context, _ string, u Update) {
			mu.Lock()
			got = append(got, u)
			mu.Unlock()
		},
	}
	_, agent := newFakeAgent(t, handler)

	agent.notifyUpdate(t, "s1", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "Hello"},
	})
	agent.notifyUpdate(t, "s1", map[string]any{
		"sessionUpdate": "some_future_variant",
		"somethingNew":  42,
	})

	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 2 })

	mu.Lock()
	defer mu.Unlock()
	if got[0].Content == nil || got[0].Content.Text != "Hello" {
		t.Fatalf("first update content = %+v, want text Hello", got[0].Content)
	}
	if got[1].SessionUpdate != "some_future_variant" {
		t.Fatalf("unknown variant dropped: %+v", got[1])
	}
	if !strings.Contains(string(got[1].Raw), "somethingNew") {
		t.Fatalf("Raw lost the unmodeled field: %s", got[1].Raw)
	}
}

// The live capture returned a toolCallId containing a literal newline.
func TestToolCallIDWithNewlineSurvives(t *testing.T) {
	const id = "call_qBAP4Fl7IWVCo1TqD58ujyVi\nctc_0f67e1d54dd6c5ed016a9891f8d86887"
	var mu sync.Mutex
	var got Update
	handler := HandlerFuncs{Update: func(_ context.Context, _ string, u Update) {
		mu.Lock()
		got = u
		mu.Unlock()
	}}
	_, agent := newFakeAgent(t, handler)

	agent.notifyUpdate(t, "s1", map[string]any{
		"sessionUpdate": UpdateToolCall,
		"toolCallId":    id,
		"title":         "Edit File",
		"kind":          "edit",
		"status":        "pending",
	})
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return got.ToolCallID != "" })

	mu.Lock()
	defer mu.Unlock()
	if got.ToolCallID != id {
		t.Fatalf("toolCallId = %q, want %q", got.ToolCallID, id)
	}
}

func TestPermissionRequestIsAnswered(t *testing.T) {
	handler := HandlerFuncs{
		Permission: func(_ context.Context, req PermissionRequest) PermissionOutcome {
			if len(req.Options) == 0 {
				return CancelPermission()
			}
			return AllowPermission(req.Options[0].OptionID)
		},
	}
	_, agent := newFakeAgent(t, handler)

	params := map[string]any{
		"sessionId": "s1",
		"toolCall":  map[string]any{"sessionUpdate": UpdateToolCall, "toolCallId": "tc1"},
		"options":   []map[string]any{{"optionId": "allow-once", "name": "Allow once"}},
	}
	if err := agent.enc.Encode(rpcMessage{
		JSONRPC: "2.0", ID: rawID(99),
		Method: "session/request_permission", Params: mustRaw(params),
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	reply := agent.next(t)
	var body struct {
		Outcome PermissionOutcome `json:"outcome"`
	}
	if err := json.Unmarshal(reply.Result, &body); err != nil {
		t.Fatalf("decode reply %s: %v", reply.Result, err)
	}
	if body.Outcome.Outcome != OutcomeSelected || body.Outcome.OptionID != "allow-once" {
		t.Fatalf("outcome = %+v, want selected/allow-once", body.Outcome)
	}
}

func TestUnsupportedClientRequestGetsMethodNotFound(t *testing.T) {
	_, agent := newFakeAgent(t, DenyAll)
	if err := agent.enc.Encode(rpcMessage{
		JSONRPC: "2.0", ID: rawID(7), Method: "fs/readTextFile", Params: mustRaw(map[string]any{}),
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	reply := agent.next(t)
	if reply.Error == nil || reply.Error.Code != -32601 {
		t.Fatalf("error = %+v, want -32601", reply.Error)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
