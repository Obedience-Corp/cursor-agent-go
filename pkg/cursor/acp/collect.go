package acp

import (
	"context"
	"strings"
	"sync"
)

// ToolCall is an aggregated view of one tool invocation.
//
// The CLI announces a call with an empty RawInput and fills the arguments and
// locations in on later tool_call_update frames sharing the same ID, so this
// merges every frame for an ID rather than keeping only the first.
type ToolCall struct {
	ID        string
	Title     string
	Kind      string
	Status    string
	RawInput  []byte
	Locations []ToolCallLocation
}

// Transcript is the aggregated outcome of one prompt turn.
//
// StopReason and Text are not evidence that the work happened. Across live
// runs of cursor-agent 2026.09.02 an edit tool call was never observed
// reaching a terminal status: the turn resolved end_turn while the last
// tool_call_update still read in_progress, and in one run of three the file
// the agent reported writing did not exist afterwards. Verify side effects
// yourself rather than trusting the reply text.
type Transcript struct {
	Text       string
	Thoughts   string
	ToolCalls  []ToolCall
	StopReason string
	Title      string
}

// collector accumulates the updates of a single turn on a single session.
//
// It is not a Handler: the connection routes updates to it by session id, so
// it never sees another session's traffic and never displaces the caller's
// own handler.
type collector struct {
	mu       sync.Mutex
	text     strings.Builder
	thoughts strings.Builder
	title    string
	order    []string
	calls    map[string]*ToolCall
}

func newCollector(_ Handler) *collector {
	return &collector{calls: make(map[string]*ToolCall)}
}

func (c *collector) collect(update Update) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch update.SessionUpdate {
	case UpdateAgentMessageChunk:
		if update.Content != nil {
			c.text.WriteString(update.Content.Text)
		}
	case UpdateAgentThoughtChunk:
		if update.Content != nil {
			c.thoughts.WriteString(update.Content.Text)
		}
	case UpdateSessionInfo:
		if update.Title != "" {
			c.title = update.Title
		}
	case UpdateToolCall, UpdateToolCallUpdate:
		c.mergeToolCall(update)
	}
}

// mergeToolCall requires c.mu held.
func (c *collector) mergeToolCall(update Update) {
	if update.ToolCallID == "" {
		return
	}
	call, seen := c.calls[update.ToolCallID]
	if !seen {
		call = &ToolCall{ID: update.ToolCallID}
		c.calls[update.ToolCallID] = call
		c.order = append(c.order, update.ToolCallID)
	}
	if update.Title != "" {
		call.Title = update.Title
	}
	if update.Kind != "" {
		call.Kind = update.Kind
	}
	if update.Status != "" {
		call.Status = update.Status
	}
	if len(update.RawInput) > 0 && string(update.RawInput) != "{}" {
		call.RawInput = append([]byte(nil), update.RawInput...)
	}
	if len(update.Locations) > 0 {
		call.Locations = update.Locations
	}
}

func (c *collector) transcript(stopReason string) *Transcript {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := &Transcript{
		Text:       c.text.String(),
		Thoughts:   c.thoughts.String(),
		StopReason: stopReason,
		Title:      c.title,
	}
	for _, id := range c.order {
		out.ToolCalls = append(out.ToolCalls, *c.calls[id])
	}
	return out
}

// CollectPrompt sends one turn and aggregates every update for that session
// until the turn resolves, returning the assembled text, thoughts, and merged
// tool calls.
//
// The client's own Handler still receives every update as it arrives; this
// aggregates alongside it rather than replacing it.
//
// Aggregation is scoped to sessionID. Collections on different sessions run
// concurrently and never observe each other's updates. Collections on the
// SAME session are serialized: the second call returns
// ErrCollectionInProgress rather than a mixed transcript, because
// session/update carries no turn correlation and the updates of two
// overlapping turns are indistinguishable on the wire.
func (c *Client) CollectPrompt(ctx context.Context, sessionID string, blocks ...ContentBlock) (*Transcript, error) {
	col := newCollector(nil)
	stop, err := c.conn.registerCollector(sessionID, col)
	if err != nil {
		return nil, err
	}
	defer stop()

	result, promptErr := c.Prompt(ctx, sessionID, blocks...)
	if promptErr != nil {
		return col.transcript(""), promptErr
	}
	return col.transcript(result.StopReason), nil
}

// CollectAsk is CollectPrompt with a single text block.
func (c *Client) CollectAsk(ctx context.Context, sessionID, text string) (*Transcript, error) {
	return c.CollectPrompt(ctx, sessionID, TextBlock(text))
}
