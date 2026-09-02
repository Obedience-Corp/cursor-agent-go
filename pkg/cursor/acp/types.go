// Package acp is the Agent Client Protocol client for "cursor-agent acp".
//
// The transport is newline-delimited JSON-RPC 2.0 over the process's stdio. A
// session is long-lived: the client initializes once, opens a session, then
// issues prompt turns that stream session/update notifications back before the
// turn's result resolves.
package acp

import "encoding/json"

// ProtocolVersion is the version this client negotiates.
const ProtocolVersion = 1

// ContentBlock is a single piece of prompt or update content.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// TextBlock builds a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: text}
}

// FSCapabilities declares which filesystem calls the client can service.
type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

// ClientCapabilities is sent with initialize.
type ClientCapabilities struct {
	FS FSCapabilities `json:"fs"`
}

// AuthMethod is one authentication option the agent advertises.
type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// InitializeResult is the agent's reply to initialize.
type InitializeResult struct {
	ProtocolVersion   int             `json:"protocolVersion"`
	AgentCapabilities json.RawMessage `json:"agentCapabilities,omitempty"`
	AuthMethods       []AuthMethod    `json:"authMethods,omitempty"`
}

// Mode is one agent execution mode (agent, plan, ask).
type Mode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Modes is the mode block returned by session/new.
type Modes struct {
	CurrentModeID  string `json:"currentModeId"`
	AvailableModes []Mode `json:"availableModes"`
}

// Model is one selectable model.
type Model struct {
	ModelID string `json:"modelId"`
	Name    string `json:"name"`
}

// Models is the model block returned by session/new.
type Models struct {
	CurrentModelID  string  `json:"currentModelId"`
	AvailableModels []Model `json:"availableModels"`
}

// NewSessionResult is the agent's reply to session/new.
type NewSessionResult struct {
	SessionID string `json:"sessionId"`
	Modes     Modes  `json:"modes"`
	Models    Models `json:"models"`
}

// Command is one slash command the agent exposes.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToolCallLocation points at a file a tool call touches.
type ToolCallLocation struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

// Update is one session/update notification payload.
//
// The variant is carried in SessionUpdate. Fields not belonging to a variant
// are zero; RawInput and Locations only appear on tool-call variants.
type Update struct {
	SessionUpdate     string             `json:"sessionUpdate"`
	Content           *ContentBlock      `json:"content,omitempty"`
	Title             string             `json:"title,omitempty"`
	ToolCallID        string             `json:"toolCallId,omitempty"`
	Kind              string             `json:"kind,omitempty"`
	Status            string             `json:"status,omitempty"`
	RawInput          json.RawMessage    `json:"rawInput,omitempty"`
	Locations         []ToolCallLocation `json:"locations,omitempty"`
	AvailableCommands []Command          `json:"availableCommands,omitempty"`
	Raw               json.RawMessage    `json:"-"`
}

// Update variants observed on the wire. Unknown variants are still delivered.
const (
	UpdateAgentMessageChunk = "agent_message_chunk"
	UpdateAgentThoughtChunk = "agent_thought_chunk"
	UpdateToolCall          = "tool_call"
	UpdateToolCallUpdate    = "tool_call_update"
	UpdateSessionInfo       = "session_info_update"
	UpdateAvailableCommands = "available_commands_update"
)

// PromptResult is the reply to session/prompt.
type PromptResult struct {
	StopReason string `json:"stopReason"`
}

// Stop reasons returned by session/prompt.
const (
	StopEndTurn   = "end_turn"
	StopCancelled = "cancelled"
	StopMaxTokens = "max_tokens"
	StopRefusal   = "refusal"
)

// PermissionOption is one choice offered by session/request_permission.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"`
}

// PermissionRequest is the payload of a session/request_permission call.
//
// Shape follows the ACP specification. It was not exercised in the CA0005
// capture because the agent already held workspace permission; see
// evidence/acp/README.md.
type PermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  Update             `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// PermissionOutcome is the client's answer to a permission request.
type PermissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

// Permission outcome kinds.
const (
	OutcomeSelected  = "selected"
	OutcomeCancelled = "cancelled"
)

// AllowPermission selects an option by id.
func AllowPermission(optionID string) PermissionOutcome {
	return PermissionOutcome{Outcome: OutcomeSelected, OptionID: optionID}
}

// CancelPermission refuses the request.
func CancelPermission() PermissionOutcome {
	return PermissionOutcome{Outcome: OutcomeCancelled}
}
