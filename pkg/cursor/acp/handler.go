package acp

import (
	"context"
	"encoding/json"
)

// Handler receives everything the agent pushes at the client during a session.
//
// Implementations must not block indefinitely: the read loop is serialized, so
// a slow handler stalls the whole stream. A nil Handler field on Options is
// replaced by a no-op that auto-cancels permission requests.
type Handler interface {
	// OnUpdate is called for every session/update notification, including
	// variants this package does not model; Update.Raw always carries the
	// original payload.
	OnUpdate(ctx context.Context, sessionID string, update Update)

	// OnPermission answers a session/request_permission call. Returning
	// CancelPermission() refuses the tool call.
	//
	// This is NOT a containment boundary. cursor-agent 2026.08.31 never sends
	// session/request_permission in its default configuration: it was observed
	// editing files, running shell commands, and writing outside the session
	// cwd without asking. Implement this for spec compliance and future CLI
	// versions, but confine the agent with OS-level isolation.
	OnPermission(ctx context.Context, req PermissionRequest) PermissionOutcome
}

// HandlerFuncs adapts plain functions to Handler. Nil fields are no-ops.
type HandlerFuncs struct {
	Update     func(ctx context.Context, sessionID string, update Update)
	Permission func(ctx context.Context, req PermissionRequest) PermissionOutcome
}

// OnUpdate implements Handler.
func (h HandlerFuncs) OnUpdate(ctx context.Context, sessionID string, update Update) {
	if h.Update != nil {
		h.Update(ctx, sessionID, update)
	}
}

// OnPermission implements Handler.
func (h HandlerFuncs) OnPermission(ctx context.Context, req PermissionRequest) PermissionOutcome {
	if h.Permission != nil {
		return h.Permission(ctx, req)
	}
	return CancelPermission()
}

// DenyAll is the default handler: it drops updates and refuses every
// permission request.
var DenyAll Handler = HandlerFuncs{}

// PolicyHandler answers every permission request with a fixed policy and
// forwards updates to Update when set.
func PolicyHandler(policy PermissionPolicy, update func(ctx context.Context, sessionID string, u Update)) Handler {
	return HandlerFuncs{
		Update: update,
		Permission: func(_ context.Context, req PermissionRequest) PermissionOutcome {
			return policy.Decide(req.Options)
		},
	}
}

type updateNotification struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

func (c *conn) serveNotification(msg rpcMessage, raw []byte) {
	if msg.Method != "session/update" {
		return
	}
	var note updateNotification
	if err := json.Unmarshal(msg.Params, &note); err != nil {
		return
	}
	var update Update
	if err := json.Unmarshal(note.Update, &update); err != nil {
		return
	}
	update.Raw = append(json.RawMessage(nil), note.Update...)
	ctx := context.Background()
	// Collectors for this session first, then the always-installed base
	// handler. The base handler never stops receiving updates.
	if col := c.collectorFor(note.SessionID); col != nil {
		col.collect(update)
	}
	c.currentHandler().OnUpdate(ctx, note.SessionID, update)
}

func (c *conn) serveRequest(msg rpcMessage) {
	id := *msg.ID
	switch msg.Method {
	case "session/request_permission":
		var req PermissionRequest
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			c.reply(id, nil, &rpcError{Code: -32602, Message: "invalid permission params"})
			return
		}
		outcome := c.currentHandler().OnPermission(context.Background(), req)
		c.reply(id, map[string]any{"outcome": outcome}, nil)
	default:
		c.reply(id, nil, &rpcError{Code: -32601, Message: "method not supported by client: " + msg.Method})
	}
}
