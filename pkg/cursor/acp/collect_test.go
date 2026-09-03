package acp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPermissionPolicyDecide(t *testing.T) {
	full := []PermissionOption{
		{OptionID: "o-reject", Kind: KindRejectOnce, Name: "Reject"},
		{OptionID: "o-once", Kind: KindAllowOnce, Name: "Allow once"},
		{OptionID: "o-always", Kind: KindAllowAlways, Name: "Always allow"},
	}
	tests := []struct {
		name        string
		policy      PermissionPolicy
		options     []PermissionOption
		wantOutcome string
		wantOption  string
	}{
		{"allow once", PolicyAllowOnce, full, OutcomeSelected, "o-once"},
		{"allow always", PolicyAllowAlways, full, OutcomeSelected, "o-always"},
		{"reject once", PolicyRejectOnce, full, OutcomeSelected, "o-reject"},
		{
			name:        "allow always falls back to allow once",
			policy:      PolicyAllowAlways,
			options:     []PermissionOption{{OptionID: "o-once", Kind: KindAllowOnce}},
			wantOutcome: OutcomeSelected, wantOption: "o-once",
		},
		{
			name:        "allow policy cancels rather than picking a reject option",
			policy:      PolicyAllowOnce,
			options:     []PermissionOption{{OptionID: "o-reject", Kind: KindRejectOnce}},
			wantOutcome: OutcomeCancelled,
		},
		{"no options cancels", PolicyAllowOnce, nil, OutcomeCancelled, ""},
		{"reject with no options still cancels", PolicyRejectOnce, nil, OutcomeCancelled, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.policy.Decide(tc.options)
			if got.Outcome != tc.wantOutcome || got.OptionID != tc.wantOption {
				t.Fatalf("Decide = %+v, want outcome %q option %q", got, tc.wantOutcome, tc.wantOption)
			}
		})
	}
}

func TestDefaultHandlerRejects(t *testing.T) {
	got := DenyAll.OnPermission(context.Background(), PermissionRequest{
		Options: []PermissionOption{{OptionID: "o-once", Kind: KindAllowOnce}},
	})
	if got.Outcome != OutcomeCancelled {
		t.Fatalf("DenyAll granted permission: %+v", got)
	}
}

func TestCollectorAggregatesTextAndMergesToolCalls(t *testing.T) {
	const id = "call_a\nctc_b" // the live capture really does contain a newline
	col := newCollector(nil)

	col.collect(Update{SessionUpdate: UpdateSessionInfo, Title: "File Creator"})
	col.collect(Update{SessionUpdate: UpdateAgentThoughtChunk, Content: &ContentBlock{Text: "planning"}})
	col.collect(Update{SessionUpdate: UpdateAgentMessageChunk, Content: &ContentBlock{Text: "Hello "}})
	col.collect(Update{SessionUpdate: UpdateAgentMessageChunk, Content: &ContentBlock{Text: "world"}})
	// tool_call announces with pending status and an empty rawInput...
	col.collect(Update{
		SessionUpdate: UpdateToolCall, ToolCallID: id,
		Title: "Edit File", Kind: "edit", Status: "pending",
		RawInput: json.RawMessage(`{}`),
	})
	// ...and the real arguments arrive later under the same id.
	col.collect(Update{
		SessionUpdate: UpdateToolCallUpdate, ToolCallID: id,
		Title:     "Edit `/tmp/hello.txt`",
		Status:    "completed",
		RawInput:  json.RawMessage(`{"path":"/tmp/hello.txt"}`),
		Locations: []ToolCallLocation{{Path: "/tmp/hello.txt"}},
	})

	got := col.transcript(StopEndTurn)
	if got.Text != "Hello world" {
		t.Fatalf("Text = %q", got.Text)
	}
	if got.Thoughts != "planning" || got.Title != "File Creator" {
		t.Fatalf("thoughts/title = %q / %q", got.Thoughts, got.Title)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1 merged: %+v", len(got.ToolCalls), got.ToolCalls)
	}
	call := got.ToolCalls[0]
	if call.ID != id {
		t.Fatalf("tool call id = %q, want the newline-bearing id", call.ID)
	}
	if call.Status != "completed" || call.Kind != "edit" {
		t.Fatalf("merge lost fields: %+v", call)
	}
	if !strings.Contains(string(call.RawInput), "/tmp/hello.txt") {
		t.Fatalf("rawInput not filled from the update frame: %s", call.RawInput)
	}
	if len(call.Locations) != 1 {
		t.Fatalf("locations = %+v", call.Locations)
	}
	if got.StopReason != StopEndTurn {
		t.Fatalf("stopReason = %q", got.StopReason)
	}
}
