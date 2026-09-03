package acp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

// Two collections in flight on one connection must not see each other's
// updates, and unregistering one must not disturb the other.
func TestOverlappingCollectionsDoNotMixSessions(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)

	colA := newCollector(nil)
	colB := newCollector(nil)
	stopA := mustRegister(t, c, "session-a", colA)
	stopB := mustRegister(t, c, "session-b", colB)

	agent.notifyUpdate(t, "session-a", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "AAA"},
	})
	agent.notifyUpdate(t, "session-b", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "BBB"},
	})
	waitFor(t, func() bool {
		return colA.transcript("").Text != "" && colB.transcript("").Text != ""
	})

	if got := colA.transcript("").Text; got != "AAA" {
		t.Fatalf("collector A text = %q, want only its own session's update", got)
	}
	if got := colB.transcript("").Text; got != "BBB" {
		t.Fatalf("collector B text = %q, want only its own session's update", got)
	}

	// A finishing first must not detach B.
	stopA()
	agent.notifyUpdate(t, "session-b", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "CCC"},
	})
	waitFor(t, func() bool { return colB.transcript("").Text == "BBBCCC" })
	if got := colB.transcript("").Text; got != "BBBCCC" {
		t.Fatalf("collector B text = %q after A unregistered, want BBBCCC", got)
	}
	stopB()
}

// The base handler must keep receiving every update while collections run,
// and must still be installed once they finish.
func TestBaseHandlerSurvivesCollections(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	base := HandlerFuncs{Update: func(_ context.Context, sessionID string, _ Update) {
		mu.Lock()
		seen = append(seen, sessionID)
		mu.Unlock()
	}}
	c, agent := newFakeAgent(t, base)

	stop := mustRegister(t, c, "session-a", newCollector(nil))
	agent.notifyUpdate(t, "session-a", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "x"},
	})
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(seen) == 1 })
	stop()

	agent.notifyUpdate(t, "session-b", map[string]any{
		"sessionUpdate": UpdateAgentMessageChunk,
		"content":       map[string]any{"type": "text", "text": "y"},
	})
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(seen) == 2 })

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 || seen[0] != "session-a" || seen[1] != "session-b" {
		t.Fatalf("base handler saw %v, want both sessions", seen)
	}
}

// Drive two concurrent CollectPrompt calls for different sessions through the
// full client path. Run under -race, this covers both the data race and the
// cross-session mixing the connection-wide handler swap used to allow.
func TestConcurrentCollectPromptsAreIsolated(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)
	client := &Client{conn: c}

	// Answer each session's prompt after emitting only that session's text.
	go func() {
		pending := map[string]*rpcMessage{}
		for msg := range agent.requests {
			if msg.Method != "session/prompt" {
				continue
			}
			var params struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				continue
			}
			pending[params.SessionID] = &msg
			agent.notifyUpdate(t, params.SessionID, map[string]any{
				"sessionUpdate": UpdateAgentMessageChunk,
				"content":       map[string]any{"type": "text", "text": params.SessionID},
			})
			agent.respond(t, msg.ID, map[string]any{"stopReason": StopEndTurn})
		}
	}()

	var wg sync.WaitGroup
	results := make([]*Transcript, 2)
	errs := make([]error, 2)
	for i, sessionID := range []string{"session-a", "session-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = client.CollectAsk(t.Context(), sessionID, "go")
		}()
	}
	wg.Wait()

	for i, sessionID := range []string{"session-a", "session-b"} {
		if errs[i] != nil {
			t.Fatalf("%s: %v", sessionID, errs[i])
		}
		if results[i].Text != sessionID {
			t.Fatalf("%s collected %q, want only its own text", sessionID, results[i].Text)
		}
	}

	// Every collector must have detached.
	c.mu.Lock()
	remaining := len(c.collectors)
	c.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d collectors leaked after both collections finished", remaining)
	}
}

// Collectors accumulate updates but must never intercept permission requests.
// Under the old handler-swap design a collector sat in front of the caller's
// handler and forwarded permissions to it; now it is out of that path
// entirely, so assert the caller still decides while a collection is running.
func TestPermissionReachesBaseHandlerDuringCollection(t *testing.T) {
	var mu sync.Mutex
	asked := 0
	base := HandlerFuncs{
		Permission: func(_ context.Context, req PermissionRequest) PermissionOutcome {
			mu.Lock()
			asked++
			mu.Unlock()
			return PolicyAllowOnce.Decide(req.Options)
		},
	}
	c, agent := newFakeAgent(t, base)

	stop := mustRegister(t, c, "session-a", newCollector(nil))
	defer stop()

	params := map[string]any{
		"sessionId": "session-a",
		"toolCall":  map[string]any{"sessionUpdate": UpdateToolCall, "toolCallId": "tc1"},
		"options": []map[string]any{
			{"optionId": "o-once", "kind": KindAllowOnce, "name": "Allow once"},
		},
	}
	if err := agent.enc.Encode(rpcMessage{
		JSONRPC: "2.0", ID: rawID(42),
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
	if body.Outcome.Outcome != OutcomeSelected || body.Outcome.OptionID != "o-once" {
		t.Fatalf("outcome = %+v, want the base handler's decision", body.Outcome)
	}
	mu.Lock()
	defer mu.Unlock()
	if asked != 1 {
		t.Fatalf("base handler asked %d times, want 1", asked)
	}
}

func mustRegister(t *testing.T, c *conn, sessionID string, col *collector) func() {
	t.Helper()
	stop, err := c.registerCollector(sessionID, col)
	if err != nil {
		t.Fatalf("registerCollector(%s): %v", sessionID, err)
	}
	return stop
}

// session/update carries only a sessionId, with no turn correlation, so two
// overlapping collections on one session cannot be separated. The second must
// be refused rather than handed a mixed transcript.
func TestSameSessionCollectionIsRefused(t *testing.T) {
	c, _ := newFakeAgent(t, DenyAll)

	stop := mustRegister(t, c, "session-a", newCollector(nil))

	_, err := c.registerCollector("session-a", newCollector(nil))
	if !errors.Is(err, ErrCollectionInProgress) {
		t.Fatalf("second registration err = %v, want ErrCollectionInProgress", err)
	}

	// A different session is unaffected.
	otherStop, err := c.registerCollector("session-b", newCollector(nil))
	if err != nil {
		t.Fatalf("different session refused: %v", err)
	}
	otherStop()

	// Once the first finishes the session is collectable again.
	stop()
	again, err := c.registerCollector("session-a", newCollector(nil))
	if err != nil {
		t.Fatalf("re-registration after release failed: %v", err)
	}
	again()
}

// The public API must surface the refusal, not just the internal registry.
func TestCollectPromptRefusesOverlappingSameSession(t *testing.T) {
	c, agent := newFakeAgent(t, DenyAll)
	client := &Client{conn: c}

	release := make(chan struct{})
	go func() {
		for msg := range agent.requests {
			if msg.Method != "session/prompt" {
				continue
			}
			<-release
			agent.respond(t, msg.ID, map[string]any{"stopReason": StopEndTurn})
		}
	}()

	first := make(chan error, 1)
	go func() {
		_, err := client.CollectAsk(t.Context(), "session-a", "one")
		first <- err
	}()

	// Wait until the first collection has actually registered.
	waitFor(t, func() bool { return c.collectorFor("session-a") != nil })

	if _, err := client.CollectAsk(t.Context(), "session-a", "two"); !errors.Is(err, ErrCollectionInProgress) {
		t.Fatalf("overlapping CollectAsk err = %v, want ErrCollectionInProgress", err)
	}

	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first collection: %v", err)
	}
	if c.collectorFor("session-a") != nil {
		t.Fatal("collector leaked after the first collection finished")
	}
}
