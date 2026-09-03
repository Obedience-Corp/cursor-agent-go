package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// serveACP impersonates "cursor-agent acp" over stdio using the same
// newline-delimited JSON-RPC framing as the real binary.
//
// Behaviour is driven by CURSOR_MOCK_ACP:
//
//	""            replay a normal turn: thought, two message chunks, a tool
//	              call whose arguments arrive on a later update, end_turn
//	"permission"  additionally request permission before the tool call, which
//	              the real CLI never does (see evidence/acp/README.md)
//	"cancel"      answer session/prompt with stopReason cancelled
//	"toolnewline" use a tool call id containing a literal newline
func serveACP() int {
	scenario := os.Getenv("CURSOR_MOCK_ACP")
	out := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	const sessionID = "00000000-0000-4000-8000-0000000000ac"
	toolCallID := "call_mock_1"
	if scenario == "toolnewline" {
		toolCallID = "call_mock_1\nctc_mock_2"
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params json.RawMessage  `json:"params"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		switch msg.Method {
		case "initialize":
			reply(out, msg.ID, map[string]any{
				"protocolVersion": 1,
				"agentCapabilities": map[string]any{
					"loadSession":        true,
					"promptCapabilities": map[string]any{"image": true},
				},
				"authMethods": []any{map[string]any{"id": "cursor_login", "name": "Cursor Login"}},
			})
		case "session/new":
			reply(out, msg.ID, map[string]any{
				"sessionId": sessionID,
				"modes": map[string]any{
					"currentModeId":  "agent",
					"availableModes": []any{map[string]any{"id": "agent", "name": "Agent"}},
				},
				"models": map[string]any{
					"currentModelId":  "mock-model",
					"availableModels": []any{map[string]any{"modelId": "mock-model", "name": "Mock"}},
				},
			})
		case "session/prompt":
			runPromptTurn(out, sessionID, toolCallID, scenario)
			stop := "end_turn"
			if scenario == "cancel" {
				stop = "cancelled"
			}
			reply(out, msg.ID, map[string]any{"stopReason": stop})
		case "session/cancel":
			// notification, nothing to answer
		default:
			if msg.ID != nil {
				replyError(out, msg.ID, -32601, "method not supported by mock: "+msg.Method)
			}
		}
	}
	return 0
}

func runPromptTurn(out *json.Encoder, sessionID, toolCallID, scenario string) {
	notify(out, sessionID, map[string]any{
		"sessionUpdate": "session_info_update", "title": "Mock Turn",
	})
	notify(out, sessionID, map[string]any{
		"sessionUpdate": "agent_thought_chunk",
		"content":       map[string]any{"type": "text", "text": "thinking"},
	})
	for _, chunk := range []string{"Hello ", "world"} {
		notify(out, sessionID, map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": chunk},
		})
	}
	if scenario == "permission" {
		// The real CLI does not send this; the mock does so the client's
		// permission path stays exercised.
		out.Encode(map[string]any{
			"jsonrpc": "2.0", "id": 9001, "method": "session/request_permission",
			"params": map[string]any{
				"sessionId": sessionID,
				"toolCall":  map[string]any{"sessionUpdate": "tool_call", "toolCallId": toolCallID},
				"options": []any{
					map[string]any{"optionId": "o-once", "kind": "allow_once", "name": "Allow once"},
					map[string]any{"optionId": "o-reject", "kind": "reject_once", "name": "Reject"},
				},
			},
		})
	}
	// Announce with empty rawInput, exactly as the real CLI does.
	notify(out, sessionID, map[string]any{
		"sessionUpdate": "tool_call", "toolCallId": toolCallID,
		"title": "Edit File", "kind": "edit", "status": "pending",
		"rawInput": map[string]any{},
	})
	// Arguments and locations only arrive here.
	notify(out, sessionID, map[string]any{
		"sessionUpdate": "tool_call_update", "toolCallId": toolCallID,
		"title":     "Edit `/tmp/mock.txt`",
		"status":    "in_progress",
		"rawInput":  map[string]any{"path": "/tmp/mock.txt"},
		"locations": []any{map[string]any{"path": "/tmp/mock.txt"}},
	})
}

func notify(out *json.Encoder, sessionID string, update map[string]any) {
	_ = out.Encode(map[string]any{
		"jsonrpc": "2.0", "method": "session/update",
		"params": map[string]any{"sessionId": sessionID, "update": update},
	})
}

func reply(out *json.Encoder, id *json.RawMessage, result any) {
	if id == nil {
		return
	}
	if err := out.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}); err != nil {
		fmt.Fprintln(os.Stderr, "cursor-agent-mock:", err)
	}
}

func replyError(out *json.Encoder, id *json.RawMessage, code int, message string) {
	_ = out.Encode(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": code, "message": message},
	})
}
